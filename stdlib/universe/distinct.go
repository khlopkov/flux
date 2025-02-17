package universe

import (
	"github.com/InfluxCommunity/flux"
	"github.com/InfluxCommunity/flux/codes"
	"github.com/InfluxCommunity/flux/execute"
	"github.com/InfluxCommunity/flux/internal/errors"
	"github.com/InfluxCommunity/flux/plan"
	"github.com/InfluxCommunity/flux/runtime"
	"github.com/InfluxCommunity/flux/values"
)

const DistinctKind = "distinct"

type DistinctOpSpec struct {
	Column string `json:"column"`
}

func init() {
	distinctSignature := runtime.MustLookupBuiltinType("universe", "distinct")

	runtime.RegisterPackageValue("universe", DistinctKind, flux.MustValue(flux.FunctionValue(DistinctKind, CreateDistinctOpSpec, distinctSignature)))
	plan.RegisterProcedureSpec(DistinctKind, newDistinctProcedure, DistinctKind)
	execute.RegisterTransformation(DistinctKind, createDistinctTransformation)
}

func CreateDistinctOpSpec(args flux.Arguments, a *flux.Administration) (flux.OperationSpec, error) {
	if err := a.AddParentFromArgs(args); err != nil {
		return nil, err
	}

	spec := new(DistinctOpSpec)

	if col, ok, err := args.GetString("column"); err != nil {
		return nil, err
	} else if ok {
		spec.Column = col
	} else {
		spec.Column = execute.DefaultValueColLabel
	}

	return spec, nil
}

func (s *DistinctOpSpec) Kind() flux.OperationKind {
	return DistinctKind
}

type DistinctProcedureSpec struct {
	plan.DefaultCost
	Column string
}

func newDistinctProcedure(qs flux.OperationSpec, pa plan.Administration) (plan.ProcedureSpec, error) {
	spec, ok := qs.(*DistinctOpSpec)
	if !ok {
		return nil, errors.Newf(codes.Internal, "invalid spec type %T", qs)
	}

	return &DistinctProcedureSpec{
		Column: spec.Column,
	}, nil
}

func (s *DistinctProcedureSpec) Kind() plan.ProcedureKind {
	return DistinctKind
}
func (s *DistinctProcedureSpec) Copy() plan.ProcedureSpec {
	ns := new(DistinctProcedureSpec)

	*ns = *s

	return ns
}

// TriggerSpec implements plan.TriggerAwareProcedureSpec
func (s *DistinctProcedureSpec) TriggerSpec() plan.TriggerSpec {
	return plan.NarrowTransformationTriggerSpec{}
}

func createDistinctTransformation(id execute.DatasetID, mode execute.AccumulationMode, spec plan.ProcedureSpec, a execute.Administration) (execute.Transformation, execute.Dataset, error) {
	s, ok := spec.(*DistinctProcedureSpec)
	if !ok {
		return nil, nil, errors.Newf(codes.Internal, "invalid spec type %T", spec)
	}
	cache := execute.NewTableBuilderCache(a.Allocator())
	d := execute.NewDataset(id, mode, cache)
	t := NewDistinctTransformation(d, cache, s)
	return t, d, nil
}

type distinctTransformation struct {
	execute.ExecutionNode
	d     execute.Dataset
	cache execute.TableBuilderCache

	column string
}

func NewDistinctTransformation(d execute.Dataset, cache execute.TableBuilderCache, spec *DistinctProcedureSpec) *distinctTransformation {
	return &distinctTransformation{
		d:      d,
		cache:  cache,
		column: spec.Column,
	}
}

func (t *distinctTransformation) RetractTable(id execute.DatasetID, key flux.GroupKey) error {
	return t.d.RetractTable(key)
}

func (t *distinctTransformation) Process(id execute.DatasetID, tbl flux.Table) error {
	builder, created := t.cache.TableBuilder(tbl.Key())
	if !created {
		return errors.Newf(codes.FailedPrecondition, "distinct found duplicate table with key: %v", tbl.Key())
	}

	colIdx := execute.ColIdx(t.column, tbl.Cols())
	if colIdx < 0 {
		// doesn't exist in this table, so add an empty value
		if err := execute.AddTableKeyCols(tbl.Key(), builder); err != nil {
			return err
		}
		colIdx, err := builder.AddCol(flux.ColMeta{
			Label: execute.DefaultValueColLabel,
			Type:  flux.TString,
		})
		if err != nil {
			return err
		}

		if err := builder.AppendString(colIdx, ""); err != nil {
			return err
		}
		if err := execute.AppendKeyValues(tbl.Key(), builder); err != nil {
			return err
		}
		// TODO: hack required to ensure data flows downstream
		return tbl.Do(func(flux.ColReader) error {
			return nil
		})
	}

	col := tbl.Cols()[colIdx]

	if err := execute.AddTableKeyCols(tbl.Key(), builder); err != nil {
		return err
	}
	colIdx, err := builder.AddCol(flux.ColMeta{
		Label: execute.DefaultValueColLabel,
		Type:  col.Type,
	})
	if err != nil {
		return err
	}

	if tbl.Key().HasCol(t.column) {
		j := execute.ColIdx(t.column, tbl.Key().Cols())
		switch col.Type {
		case flux.TBool:
			if err := builder.AppendBool(colIdx, tbl.Key().ValueBool(j)); err != nil {
				return err
			}
		case flux.TInt:
			if err := builder.AppendInt(colIdx, tbl.Key().ValueInt(j)); err != nil {
				return err
			}
		case flux.TUInt:
			if err := builder.AppendUInt(colIdx, tbl.Key().ValueUInt(j)); err != nil {
				return err
			}
		case flux.TFloat:
			if err := builder.AppendFloat(colIdx, tbl.Key().ValueFloat(j)); err != nil {
				return err
			}
		case flux.TString:
			if err := builder.AppendString(colIdx, tbl.Key().ValueString(j)); err != nil {
				return err
			}
		case flux.TTime:
			if err := builder.AppendTime(colIdx, tbl.Key().ValueTime(j)); err != nil {
				return err
			}
		}

		if err := execute.AppendKeyValues(tbl.Key(), builder); err != nil {
			return err
		}
		// TODO: hack required to ensure data flows downstream
		return tbl.Do(func(flux.ColReader) error {
			return nil
		})
	}

	var (
		nullDistinct   bool
		boolDistinct   map[bool]bool
		intDistinct    map[int64]bool
		uintDistinct   map[uint64]bool
		floatDistinct  map[float64]bool
		stringDistinct map[string]bool
		timeDistinct   map[execute.Time]bool
	)
	switch col.Type {
	case flux.TBool:
		boolDistinct = make(map[bool]bool)
	case flux.TInt:
		intDistinct = make(map[int64]bool)
	case flux.TUInt:
		uintDistinct = make(map[uint64]bool)
	case flux.TFloat:
		floatDistinct = make(map[float64]bool)
	case flux.TString:
		stringDistinct = make(map[string]bool)
	case flux.TTime:
		timeDistinct = make(map[execute.Time]bool)
	}

	j := execute.ColIdx(t.column, tbl.Cols())
	return tbl.Do(func(cr flux.ColReader) error {
		l := cr.Len()

		for i := 0; i < l; i++ {
			// Check distinct
			switch col.Type {
			case flux.TBool:
				if cr.Bools(j).IsNull(i) {
					if nullDistinct {
						continue
					}
					if err := builder.AppendNil(colIdx); err != nil {
						return err
					}
					nullDistinct = true
				} else {
					v := cr.Bools(j).Value(i)
					if boolDistinct[v] {
						continue
					}
					boolDistinct[v] = true
					if err := builder.AppendBool(colIdx, v); err != nil {
						return err
					}
				}
			case flux.TInt:
				if cr.Ints(j).IsNull(i) {
					if nullDistinct {
						continue
					}
					if err := builder.AppendNil(colIdx); err != nil {
						return err
					}
					nullDistinct = true
				} else {
					v := cr.Ints(j).Value(i)
					if intDistinct[v] {
						continue
					}
					intDistinct[v] = true
					if err := builder.AppendInt(colIdx, v); err != nil {
						return err
					}
				}
			case flux.TUInt:
				if cr.UInts(j).IsNull(i) {
					if nullDistinct {
						continue
					}
					if err := builder.AppendNil(colIdx); err != nil {
						return err
					}
					nullDistinct = true
				} else {
					v := cr.UInts(j).Value(i)
					if uintDistinct[v] {
						continue
					}
					uintDistinct[v] = true
					if err := builder.AppendUInt(colIdx, v); err != nil {
						return err
					}
				}
			case flux.TFloat:
				if cr.Floats(j).IsNull(i) {
					if nullDistinct {
						continue
					}
					if err := builder.AppendNil(colIdx); err != nil {
						return err
					}
					nullDistinct = true
				} else {
					v := cr.Floats(j).Value(i)
					if floatDistinct[v] {
						continue
					}
					floatDistinct[v] = true
					if err := builder.AppendFloat(colIdx, v); err != nil {
						return err
					}
				}
			case flux.TString:
				if cr.Strings(j).IsNull(i) {
					if nullDistinct {
						continue
					}
					if err := builder.AppendNil(colIdx); err != nil {
						return err
					}
					nullDistinct = true
				} else {
					v := cr.Strings(j).Value(i)
					if stringDistinct[v] {
						continue
					}
					stringDistinct[v] = true
					if err := builder.AppendString(colIdx, v); err != nil {
						return err
					}
				}
			case flux.TTime:
				if cr.Times(j).IsNull(i) {
					if nullDistinct {
						continue
					}
					if err := builder.AppendNil(colIdx); err != nil {
						return err
					}
					nullDistinct = true
				} else {
					v := values.Time(cr.Times(j).Value(i))
					if timeDistinct[v] {
						continue
					}
					timeDistinct[v] = true
					if err := builder.AppendTime(colIdx, v); err != nil {
						return err
					}
				}
			}

			if err := execute.AppendKeyValues(tbl.Key(), builder); err != nil {
				return err
			}
		}
		return nil
	})
}

func (t *distinctTransformation) UpdateWatermark(id execute.DatasetID, mark execute.Time) error {
	return t.d.UpdateWatermark(mark)
}
func (t *distinctTransformation) UpdateProcessingTime(id execute.DatasetID, pt execute.Time) error {
	return t.d.UpdateProcessingTime(pt)
}
func (t *distinctTransformation) Finish(id execute.DatasetID, err error) {
	t.d.Finish(err)
}
