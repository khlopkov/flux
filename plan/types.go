package plan

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/InfluxCommunity/flux"
	"github.com/InfluxCommunity/flux/internal/operation"
	"github.com/InfluxCommunity/flux/interpreter"
	"github.com/InfluxCommunity/flux/interval"
)

type Planner interface {
	Plan(context.Context, *operation.Spec) (*Spec, error)
}

// Node defines the common interface for interacting with
// logical and physical plan nodes.
type Node interface {
	// ID returns an identifier for this plan node.
	ID() NodeID

	// Bounds returns the time bounds for this plan node.
	Bounds() *Bounds

	// Predecessors returns plan nodes executed immediately before this node.
	Predecessors() []Node

	// Successors returns plan nodes executed immediately after this node.
	Successors() []Node

	// ProcedureSpec returns the specification of the procedure represented by this node.
	ProcedureSpec() ProcedureSpec

	// ReplaceSpec replaces the procedure spec of this node with another.
	ReplaceSpec(ProcedureSpec) error

	// Kind returns the type of procedure represented by this node.
	Kind() ProcedureKind

	// CallStack returns the list of StackEntry values that created this
	// Node. A Node may have no associated call stack. This happens
	// when a Node is constructed from a planner rule and not from a
	// source location.
	CallStack() []interpreter.StackEntry

	// Helper methods for manipulating a plan
	// These methods are used during planning
	SetBounds(bounds *Bounds)
	AddSuccessors(...Node)
	AddPredecessors(...Node)
	ClearSuccessors()
	ClearPredecessors()

	ShallowCopy() Node
}

type NodeID string

// Spec holds the result nodes of a query plan with associated metadata
type Spec struct {
	// Roots are the "sink" nodes in the plan, which have no successors.
	Roots     map[Node]struct{}
	Resources flux.ResourceManagement
	Now       time.Time
}

// NewPlanSpec initializes a new query plan
func NewPlanSpec() *Spec {
	return &Spec{
		Roots: make(map[Node]struct{}),
	}
}

// Replace replaces one of the root nodes of the query plan
func (plan *Spec) Replace(root, with Node) {
	delete(plan.Roots, root)
	plan.Roots[with] = struct{}{}
}

// CheckIntegrity checks the integrity of the plan, i.e.:
//   - node A is predecessor of B iff B is successor of A;
//   - there is no cycle.
//
// This check only detects this problem (N2 is predecessor of R, but not viceversa):
//
//	N1 <----> R
//	          |
//	N2 <-------
//
// And this one (R is successor of N2, but not viceversa):
//
//	N1 <-------
//	|         |--> R
//	N2 --------
//
// But not this one, because N2 is not reachable from R (root):
//
//	N1 <-------
//	          |--> R
//	N2 --------
func (plan *Spec) CheckIntegrity() error {
	sinks := make([]Node, 0, len(plan.Roots))
	for root := range plan.Roots {
		sinks = append(sinks, root)
	}
	sources := make([]Node, 0)

	fn := func(node Node) error {
		if len(node.Predecessors()) == 0 {
			sources = append(sources, node)
		}

		return symmetryCheck(node)
	}

	err := WalkPredecessors(sinks, fn)
	if err != nil {
		return err
	}

	return WalkSuccessors(sources, symmetryCheck)
}

func symmetryCheck(node Node) error {
	for _, pred := range node.Predecessors() {
		if idx := IndexOfNode(node, pred.Successors()); idx == -1 {
			return fmt.Errorf("integrity violated: %s is predecessor of %s, "+
				"but %s is not successor of %s", pred.ID(), node.ID(), node.ID(), pred.ID())
		}
	}

	for _, succ := range node.Successors() {
		if idx := IndexOfNode(node, succ.Predecessors()); idx == -1 {
			return fmt.Errorf("integrity violated: %s is successor of %s, "+
				"but %s is not predecessor of %s`", succ.ID(), node.ID(), node.ID(), succ.ID())
		}
	}

	return nil
}

// IndexOfNode is a utility function that will return the offset
// of the given node in the slice of nodes.
// This is useful to determine whether a node is the 1st or 2nd predecessor
// of some other node for example.
// Returns -1 if node not found.
func IndexOfNode(node Node, nodes []Node) int {
	for i, n := range nodes {
		if n == node {
			return i
		}
	}

	return -1
}

// ProcedureSpec specifies a query operation
type ProcedureSpec interface {
	Kind() ProcedureKind
	Copy() ProcedureSpec
}

// ProcedureKind denotes the kind of operation
type ProcedureKind string

type bounds struct {
	value *Bounds
}

func (b *bounds) SetBounds(bounds *Bounds) {
	b.value = bounds
}

func (b *bounds) Bounds() *Bounds {
	return b.value
}

type edges struct {
	predecessors []Node
	successors   []Node
}

func (e *edges) Predecessors() []Node {
	return e.predecessors
}

func (e *edges) Successors() []Node {
	return e.successors
}

func (e *edges) AddSuccessors(nodes ...Node) {
	e.successors = append(e.successors, nodes...)
}

func (e *edges) AddPredecessors(nodes ...Node) {
	e.predecessors = append(e.predecessors, nodes...)
}

func (e *edges) ClearSuccessors() {
	e.successors = e.successors[0:0]
}

func (e *edges) ClearPredecessors() {
	e.predecessors = e.predecessors[0:0]
}

func (e *edges) shallowCopy() edges {
	newEdges := new(edges)
	copy(newEdges.predecessors, e.predecessors)
	copy(newEdges.successors, e.successors)
	return *newEdges
}

// MergeToLogicalNode merges top and bottom plan nodes into a new plan node, with the
// given procedure spec.
//
//	V1     V2       V1            V2       <-- successors
//	  \   /
//	   top             mergedNode
//	    |      ==>         |
//	  bottom               W
//	    |
//	    W
//
// The returned node will have its predecessors set to the predecessors
// of "bottom", however, it's successors will not be set---it will be the responsibility of
// the plan to attach the merged node to its successors.
func MergeToLogicalNode(top, bottom Node, procSpec ProcedureSpec) (Node, error) {
	merged := &LogicalNode{
		id:   mergeIDs(top.ID(), bottom.ID()),
		Spec: procSpec,
	}

	return mergePlanNodes(top, bottom, merged)
}

func MergeToPhysicalNode(top, bottom Node, procSpec PhysicalProcedureSpec) (Node, error) {
	merged := &PhysicalPlanNode{
		id:   mergeIDs(top.ID(), bottom.ID()),
		Spec: procSpec,
	}

	return mergePlanNodes(top, bottom, merged)
}

func mergeIDs(top, bottom NodeID) NodeID {
	if strings.HasPrefix(string(top), "merged_") {
		top = top[7:]
	}
	if strings.HasPrefix(string(bottom), "merged_") {
		bottom = bottom[7:]
	}

	return "merged_" + bottom + "_" + top

}

func mergePlanNodes(top, bottom, merged Node) (Node, error) {
	if len(top.Predecessors()) != 1 ||
		len(bottom.Successors()) != 1 ||
		top.Predecessors()[0] != bottom {
		return nil, fmt.Errorf("cannot merge %s and %s due to topological issues", top.ID(), bottom.ID())
	}

	merged.AddPredecessors(bottom.Predecessors()...)
	for _, pred := range merged.Predecessors() {
		for i, succ := range pred.Successors() {
			if succ == bottom {
				pred.Successors()[i] = merged
			}
		}
	}

	return merged, nil

}

// SwapPlanNodes swaps two plan nodes and returns an equivalent sub-plan with the nodes swapped.
//
//	V1   V2        V1   V2
//	  \ /
//	   A              B
//	   |     ==>      |
//	   B          copy of A
//	   |              |
//	   W              W
//
// Note that successors of the original top node will not be updated, and the returned
// plan node will have no successors.  It will be the responsibility of the plan to
// attach the swapped nodes to successors.
func SwapPlanNodes(top, bottom Node) (Node, error) {
	if len(top.Predecessors()) != 1 ||
		len(bottom.Successors()) != 1 ||
		len(bottom.Predecessors()) != 1 {
		return nil, fmt.Errorf("cannot swap nodes %v and %v due to topological issue", top.ID(), bottom.ID())
	}

	newBottom := top.ShallowCopy()
	newBottom.ClearSuccessors()
	newBottom.ClearPredecessors()
	newBottom.AddSuccessors(bottom)
	newBottom.AddPredecessors(bottom.Predecessors()[0])
	for i, bottomPredSucc := range bottom.Predecessors()[0].Successors() {
		if bottomPredSucc == bottom {
			bottom.Predecessors()[0].Successors()[i] = newBottom
			break
		}
	}

	bottom.ClearPredecessors()
	bottom.AddPredecessors(newBottom)
	bottom.ClearSuccessors()
	return bottom, nil
}

// ReplaceNode accepts two nodes and attaches
// all the predecessors of the old node to the new node.
//
//	S1   S2        S1   S2
//	  \ /
//	oldNode   =>   newNode
//	  / \            / \
//	P1   P2        P1   P2
//
// As is convention, newNode will not have any successors attached.
// The planner will take care of this.
func ReplaceNode(oldNode, newNode Node) {
	newNode.ClearPredecessors()
	newNode.ClearSuccessors()

	newNode.AddPredecessors(oldNode.Predecessors()...)
	for _, pred := range newNode.Predecessors() {
		for i, predSucc := range pred.Successors() {
			if predSucc == oldNode {
				pred.Successors()[i] = newNode
			}
		}
	}

	oldNode.ClearPredecessors()
}

// ReplacePhysicalNodes accepts a connected group of nodes that has a single output and
// a single input, and replaces them with a single node with the predecessors of the old input node.
// Note that the planner has a convention of connecting successors itself
// (rather than having the rules doing it) so the old output's successors
// remain unconnected.
func ReplacePhysicalNodes(ctx context.Context, oldOutputNode, oldInputNode Node, name string, newSpec PhysicalProcedureSpec) Node {
	newNode := CreateUniquePhysicalNode(ctx, name, newSpec)

	newNode.AddPredecessors(oldInputNode.Predecessors()...)
	for _, pred := range oldInputNode.Predecessors() {
		i := IndexOfNode(oldInputNode, pred.Successors())
		pred.Successors()[i] = newNode
	}

	return newNode
}

type WindowSpec struct {
	Every    flux.Duration
	Period   flux.Duration
	Offset   flux.Duration
	Location Location
}

func (w WindowSpec) LoadLocation() (interval.Location, error) {
	return w.Location.Load()
}

type Location struct {
	Name   string
	Offset flux.Duration
}

func (l Location) IsUTC() bool {
	name := l.Name
	if name == "" {
		name = "UTC"
	}
	return name == "UTC" && l.Offset.IsZero()
}

func (l Location) Load() (interval.Location, error) {
	name := l.Name
	if name == "" {
		name = "UTC"
	}
	loc, err := interval.LoadLocation(name)
	if err != nil {
		return interval.Location{}, err
	}
	loc.Offset = l.Offset
	return loc, nil
}
