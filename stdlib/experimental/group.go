package experimental

import (
	"fmt"
	"sort"

	"github.com/InfluxCommunity/flux"
	"github.com/InfluxCommunity/flux/codes"
	"github.com/InfluxCommunity/flux/execute"
	"github.com/InfluxCommunity/flux/internal/errors"
	"github.com/InfluxCommunity/flux/interpreter"
	"github.com/InfluxCommunity/flux/plan"
	"github.com/InfluxCommunity/flux/runtime"
	"github.com/InfluxCommunity/flux/semantic"
)

const ExperimentalGroupKind = "experimental-group"

const (
	groupModeExtend = "extend"
)

// GroupOpSpec in package experimental defines a special group() function
// that has just one mode called "extend", which adds additional columns to the group key.
// This is a workaround until schema introspection is implemented:
//
//	https://github.com/InfluxCommunity/flux/issues/27
//
// Most of this code has simply been copied from stdlib/universe/group.go
type GroupOpSpec struct {
	Mode    string   `json:"mode"`
	Columns []string `json:"columns"`
}

func init() {
	groupSignature := runtime.MustLookupBuiltinType("experimental", "group")
	runtime.RegisterPackageValue("experimental", "group", flux.MustValue(flux.FunctionValue("group", createGroupOpSpec, groupSignature)))
	plan.RegisterProcedureSpec(ExperimentalGroupKind, newGroupProcedure, ExperimentalGroupKind)
	execute.RegisterTransformation(ExperimentalGroupKind, createGroupTransformation)
}

func createGroupOpSpec(args flux.Arguments, a *flux.Administration) (flux.OperationSpec, error) {
	if err := a.AddParentFromArgs(args); err != nil {
		return nil, err
	}

	spec := new(GroupOpSpec)

	if mode, err := args.GetRequiredString("mode"); err != nil {
		return nil, err
	} else if mode != groupModeExtend {
		return nil, errors.New(
			codes.Invalid,
			fmt.Sprintf(`invalid group mode %q; experimental.group requires mode %q`, mode, groupModeExtend),
		)
	} else {
		spec.Mode = mode
	}

	if columns, ok, err := args.GetArray("columns", semantic.String); err != nil {
		return nil, err
	} else if ok {
		spec.Columns, err = interpreter.ToStringArray(columns)
		if err != nil {
			return nil, err
		}
	} else {
		spec.Columns = []string{}
	}

	return spec, nil
}

func (s *GroupOpSpec) Kind() flux.OperationKind {
	return ExperimentalGroupKind
}

type GroupProcedureSpec struct {
	plan.DefaultCost
	GroupKeys []string
}

func newGroupProcedure(qs flux.OperationSpec, pa plan.Administration) (plan.ProcedureSpec, error) {
	spec, ok := qs.(*GroupOpSpec)
	if !ok {
		return nil, errors.Newf(codes.Internal, "invalid spec type %T", qs)
	}

	p := &GroupProcedureSpec{
		GroupKeys: spec.Columns,
	}
	return p, nil
}

func (s *GroupProcedureSpec) Kind() plan.ProcedureKind {
	return ExperimentalGroupKind
}
func (s *GroupProcedureSpec) Copy() plan.ProcedureSpec {
	ns := new(GroupProcedureSpec)

	ns.GroupKeys = make([]string, len(s.GroupKeys))
	copy(ns.GroupKeys, s.GroupKeys)

	return ns
}

func createGroupTransformation(id execute.DatasetID, mode execute.AccumulationMode, spec plan.ProcedureSpec, a execute.Administration) (execute.Transformation, execute.Dataset, error) {
	s, ok := spec.(*GroupProcedureSpec)
	if !ok {
		return nil, nil, errors.Newf(codes.Internal, "invalid spec type %T", spec)
	}
	cache := execute.NewTableBuilderCache(a.Allocator())
	d := execute.NewDataset(id, mode, cache)
	t := NewGroupTransformation(d, cache, s)
	return t, d, nil
}

type groupTransformation struct {
	execute.ExecutionNode
	d     execute.Dataset
	cache execute.TableBuilderCache

	keys []string
}

func NewGroupTransformation(d execute.Dataset, cache execute.TableBuilderCache, spec *GroupProcedureSpec) *groupTransformation {
	t := &groupTransformation{
		d:     d,
		cache: cache,
		keys:  spec.GroupKeys,
	}
	sort.Strings(t.keys)
	return t
}

func (t *groupTransformation) RetractTable(id execute.DatasetID, key flux.GroupKey) (err error) {
	panic("not implemented")
}

func (t *groupTransformation) Process(id execute.DatasetID, tbl flux.Table) error {
	cols := tbl.Cols()
	on := make(map[string]bool, len(cols))
	for _, k := range tbl.Key().Cols() {
		on[k.Label] = true
	}
	for _, c := range t.keys {
		if execute.ColIdx(c, cols) < 0 {
			return errors.New(codes.Invalid, fmt.Sprintf("unknown column %q", c))
		}
		on[c] = true
	}

	colMap := make([]int, 0, len(tbl.Cols()))
	return tbl.Do(func(cr flux.ColReader) error {
		l := cr.Len()
		for i := 0; i < l; i++ {
			key := execute.GroupKeyForRowOn(i, cr, on)
			builder, _ := t.cache.TableBuilder(key)

			colMap, err := execute.AddNewTableCols(tbl, builder, colMap)
			if err != nil {
				return err
			}

			err = execute.AppendMappedRecordWithNulls(i, cr, builder, colMap)
			if err != nil {
				return err
			}
		}
		return nil
	})
}

func (t *groupTransformation) UpdateWatermark(id execute.DatasetID, mark execute.Time) error {
	return t.d.UpdateWatermark(mark)
}
func (t *groupTransformation) UpdateProcessingTime(id execute.DatasetID, pt execute.Time) error {
	return t.d.UpdateProcessingTime(pt)
}
func (t *groupTransformation) Finish(id execute.DatasetID, err error) {
	t.d.Finish(err)
}
