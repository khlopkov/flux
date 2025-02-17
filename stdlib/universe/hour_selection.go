package universe

import (
	"github.com/InfluxCommunity/flux"
	"github.com/InfluxCommunity/flux/codes"
	"github.com/InfluxCommunity/flux/execute"
	"github.com/InfluxCommunity/flux/internal/date"
	"github.com/InfluxCommunity/flux/internal/errors"
	"github.com/InfluxCommunity/flux/plan"
	"github.com/InfluxCommunity/flux/runtime"
	"github.com/InfluxCommunity/flux/values"
)

const HourSelectionKind = "_hourSelection"

type HourSelectionOpSpec struct {
	Start      int64           `json:"start"`
	Stop       int64           `json:"stop"`
	Location   string          `json:"location"`
	Offset     values.Duration `json:"offset"`
	TimeColumn string          `json:"timeColumn"`
}

func init() {
	hourSelectionSignature := runtime.MustLookupBuiltinType("universe", "_hourSelection")

	runtime.RegisterPackageValue("universe", HourSelectionKind, flux.MustValue(flux.FunctionValue(HourSelectionKind, createHourSelectionOpSpec, hourSelectionSignature)))
	plan.RegisterProcedureSpec(HourSelectionKind, newHourSelectionProcedure, HourSelectionKind)
	execute.RegisterTransformation(HourSelectionKind, createHourSelectionTransformation)
}

func createHourSelectionOpSpec(args flux.Arguments, a *flux.Administration) (flux.OperationSpec, error) {
	if err := a.AddParentFromArgs(args); err != nil {
		return nil, err
	}

	spec := new(HourSelectionOpSpec)

	start, err := args.GetRequiredInt("start")
	if err != nil {
		return nil, err
	}
	spec.Start = start

	stop, err := args.GetRequiredInt("stop")
	if err != nil {
		return nil, err
	}
	spec.Stop = stop

	location, offset, err := date.GetLocationFromFluxArgs(args)
	if err != nil {
		return nil, err
	}
	spec.Location = location
	spec.Offset = offset

	if label, ok, err := args.GetString("timeColumn"); err != nil {
		return nil, err
	} else if ok {
		spec.TimeColumn = label
	} else {
		spec.TimeColumn = execute.DefaultTimeColLabel
	}

	return spec, nil
}

func (s *HourSelectionOpSpec) Kind() flux.OperationKind {
	return HourSelectionKind
}

type HourSelectionProcedureSpec struct {
	plan.DefaultCost
	Start      int64           `json:"start"`
	Stop       int64           `json:"stop"`
	Location   string          `json:"location"`
	Offset     values.Duration `json:"offset"`
	TimeColumn string          `json:"timeColumn"`
}

func newHourSelectionProcedure(qs flux.OperationSpec, pa plan.Administration) (plan.ProcedureSpec, error) {
	spec, ok := qs.(*HourSelectionOpSpec)
	if !ok {
		return nil, errors.Newf(codes.Internal, "invalid spec type %T", qs)
	}

	return &HourSelectionProcedureSpec{
		Start:      spec.Start,
		Stop:       spec.Stop,
		Location:   spec.Location,
		Offset:     spec.Offset,
		TimeColumn: spec.TimeColumn,
	}, nil
}

func (s *HourSelectionProcedureSpec) Kind() plan.ProcedureKind {
	return HourSelectionKind
}

func (s *HourSelectionProcedureSpec) Copy() plan.ProcedureSpec {
	ns := new(HourSelectionProcedureSpec)

	*ns = *s

	return ns
}

func (s *HourSelectionProcedureSpec) TriggerSpec() plan.TriggerSpec {
	return plan.NarrowTransformationTriggerSpec{}
}

func createHourSelectionTransformation(id execute.DatasetID, mode execute.AccumulationMode, spec plan.ProcedureSpec, a execute.Administration) (execute.Transformation, execute.Dataset, error) {
	s, ok := spec.(*HourSelectionProcedureSpec)
	if !ok {
		return nil, nil, errors.Newf(codes.Internal, "invalid spec type %T", spec)
	}
	cache := execute.NewTableBuilderCache(a.Allocator())
	d := execute.NewDataset(id, mode, cache)
	t := NewHourSelectionTransformation(d, cache, s)
	return t, d, nil
}

type hourSelectionTransformation struct {
	execute.ExecutionNode
	d     execute.Dataset
	cache execute.TableBuilderCache

	start    int64
	stop     int64
	location string
	offset   values.Duration
	timeCol  string
}

func NewHourSelectionTransformation(d execute.Dataset, cache execute.TableBuilderCache, spec *HourSelectionProcedureSpec) *hourSelectionTransformation {
	return &hourSelectionTransformation{
		d:        d,
		cache:    cache,
		start:    spec.Start,
		stop:     spec.Stop,
		location: spec.Location,
		offset:   spec.Offset,
		timeCol:  spec.TimeColumn,
	}
}

func (t *hourSelectionTransformation) RetractTable(id execute.DatasetID, key flux.GroupKey) error {
	return t.d.RetractTable(key)
}

func (t *hourSelectionTransformation) Process(id execute.DatasetID, tbl flux.Table) error {
	builder, created := t.cache.TableBuilder(tbl.Key())
	if !created {
		return errors.Newf(codes.FailedPrecondition, "hour selection found duplicate table with key: %v", tbl.Key())
	}
	if err := execute.AddTableCols(tbl, builder); err != nil {
		return err
	}

	colIdx := execute.ColIdx(t.timeCol, tbl.Cols())
	if colIdx < 0 {
		return errors.Newf(codes.FailedPrecondition, "invalid time column")
	}

	if t.start < 0 || t.start > 23 {
		return errors.Newf(codes.Invalid, "start must be between 0 and 23")
	}
	if t.stop < 0 || t.stop > 23 {
		return errors.Newf(codes.Invalid, "stop must be between 0 and 23")
	}

	return tbl.Do(func(cr flux.ColReader) error {
		l := cr.Len()
		for i := 0; i < l; i++ {
			if nullCheck := cr.Times(colIdx); nullCheck.IsNull(i) {
				continue
			}
			lTime, err := date.GetTimeInLocation(execute.Time(cr.Times(colIdx).Value(i)), t.location, t.offset)
			if err != nil {
				return nil
			}
			lHour := int64(lTime.Time().Time().Hour())
			if (lHour >= t.start && lHour <= t.stop) || (t.start > t.stop && (lHour >= t.start || lHour <= t.stop)) {
				for k := range cr.Cols() {
					if err := builder.AppendValue(k, execute.ValueForRow(cr, i, k)); err != nil {
						return err
					}
				}
			}
		}
		return nil
	})
}

func (t *hourSelectionTransformation) UpdateWatermark(id execute.DatasetID, mark execute.Time) error {
	return t.d.UpdateWatermark(mark)
}
func (t *hourSelectionTransformation) UpdateProcessingTime(id execute.DatasetID, pt execute.Time) error {
	return t.d.UpdateProcessingTime(pt)
}
func (t *hourSelectionTransformation) Finish(id execute.DatasetID, err error) {
	t.d.Finish(err)
}
