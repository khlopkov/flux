package universe_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/InfluxCommunity/flux"
	"github.com/InfluxCommunity/flux/dependencies/dependenciestest"
	"github.com/InfluxCommunity/flux/dependency"
	"github.com/InfluxCommunity/flux/execute"
	"github.com/InfluxCommunity/flux/execute/executetest"
	"github.com/InfluxCommunity/flux/internal/operation"
	"github.com/InfluxCommunity/flux/interpreter"
	"github.com/InfluxCommunity/flux/memory"
	"github.com/InfluxCommunity/flux/querytest"
	"github.com/InfluxCommunity/flux/runtime"
	"github.com/InfluxCommunity/flux/stdlib/influxdata/influxdb"
	"github.com/InfluxCommunity/flux/stdlib/universe"
	"github.com/InfluxCommunity/flux/values/valuestest"
)

func TestStateTracking_NewQuery(t *testing.T) {
	tests := []querytest.NewQueryTestCase{
		{
			Name: "from range count",
			Raw:  `from(bucket:"mydb") |> range(start:-1h) |> stateCount(fn: (r) => true)`,
			Want: &operation.Spec{
				Operations: []*operation.Node{
					{
						ID: "from0",
						Spec: &influxdb.FromOpSpec{
							Bucket: influxdb.NameOrID{Name: "mydb"},
						},
					},
					{
						ID: "range1",
						Spec: &universe.RangeOpSpec{
							Start: flux.Time{
								Relative:   -1 * time.Hour,
								IsRelative: true,
							},
							Stop:        flux.Now,
							TimeColumn:  "_time",
							StartColumn: "_start",
							StopColumn:  "_stop",
						},
					},
					{
						ID: "stateTracking2",
						Spec: &universe.StateTrackingOpSpec{
							CountColumn:    "stateCount",
							DurationColumn: "",
							DurationUnit:   flux.ConvertDuration(time.Second),
							TimeColumn:     "_time",
							Fn: interpreter.ResolvedFunction{
								Fn:    executetest.FunctionExpression(t, "(r) => true"),
								Scope: valuestest.Scope(),
							},
						},
					},
				},
				Edges: []operation.Edge{
					{Parent: "from0", Child: "range1"},
					{Parent: "range1", Child: "stateTracking2"},
				},
			},
		},
		{
			Name:    "from range count with time column",
			Raw:     `from(bucket:"mydb") |> range(start:-1h) |> stateCount(fn: (r) => true, timeColumn: "err")`,
			WantErr: true,
		},
		{
			Name: "from duration",
			Raw:  `from(bucket:"mydb") |> stateDuration(fn: (r) => true, timeColumn: "ts")`,
			Want: &operation.Spec{
				Operations: []*operation.Node{
					{
						ID: "from0",
						Spec: &influxdb.FromOpSpec{
							Bucket: influxdb.NameOrID{Name: "mydb"},
						},
					},
					{
						ID: "stateTracking1",
						Spec: &universe.StateTrackingOpSpec{
							CountColumn:    "",
							DurationColumn: "stateDuration",
							DurationUnit:   flux.ConvertDuration(time.Second),
							TimeColumn:     "ts",
							Fn: interpreter.ResolvedFunction{
								Fn:    executetest.FunctionExpression(t, "(r) => true"),
								Scope: valuestest.Scope(),
							},
						},
					},
				},
				Edges: []operation.Edge{
					{Parent: "from0", Child: "stateTracking1"},
				},
			},
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			querytest.NewQueryTestHelper(t, tc)
		})
	}
}

func TestStateTracking_Process(t *testing.T) {
	gt5 := interpreter.ResolvedFunction{
		Fn:    executetest.FunctionExpression(t, "(r) => r._value > 5.0"),
		Scope: runtime.Prelude(),
	}

	testCases := []struct {
		name    string
		spec    *universe.StateTrackingProcedureSpec
		data    []flux.Table
		want    []*executetest.Table
		wantErr error
	}{
		{
			name: "only duration",
			spec: &universe.StateTrackingProcedureSpec{
				DurationColumn: "duration",
				DurationUnit:   flux.ConvertDuration(1),
				Fn:             gt5,
				TimeCol:        "_time",
			},
			data: []flux.Table{&executetest.Table{
				ColMeta: []flux.ColMeta{
					{Label: "_time", Type: flux.TTime},
					{Label: "_value", Type: flux.TFloat},
				},
				Data: [][]interface{}{
					{execute.Time(1), 2.0},
					{execute.Time(2), 1.0},
					{execute.Time(3), 6.0},
					{execute.Time(4), 7.0},
					{execute.Time(5), 8.0},
					{execute.Time(6), 1.0},
				},
			}},
			want: []*executetest.Table{{
				ColMeta: []flux.ColMeta{
					{Label: "_time", Type: flux.TTime},
					{Label: "_value", Type: flux.TFloat},
					{Label: "duration", Type: flux.TInt},
				},
				Data: [][]interface{}{
					{execute.Time(1), 2.0, int64(-1)},
					{execute.Time(2), 1.0, int64(-1)},
					{execute.Time(3), 6.0, int64(0)},
					{execute.Time(4), 7.0, int64(1)},
					{execute.Time(5), 8.0, int64(2)},
					{execute.Time(6), 1.0, int64(-1)},
				},
			}},
		},
		{
			name: "only duration, null timestamps",
			spec: &universe.StateTrackingProcedureSpec{
				DurationColumn: "duration",
				DurationUnit:   flux.ConvertDuration(1),
				Fn:             gt5,
				TimeCol:        "_time",
			},
			data: []flux.Table{&executetest.Table{
				ColMeta: []flux.ColMeta{
					{Label: "_time", Type: flux.TTime},
					{Label: "_value", Type: flux.TFloat},
				},
				Data: [][]interface{}{
					{execute.Time(1), 2.0},
					{execute.Time(2), 1.0},
					{execute.Time(3), 6.0},
					{nil, 7.0},
					{execute.Time(5), 8.0},
					{nil, 1.0},
				},
			}},
			wantErr: errors.New("got a null timestamp"),
		},
		{
			name: "only duration, out of order timestamps",
			spec: &universe.StateTrackingProcedureSpec{
				DurationColumn: "duration",
				DurationUnit:   flux.ConvertDuration(1),
				Fn:             gt5,
				TimeCol:        "_time",
			},
			data: []flux.Table{&executetest.Table{
				ColMeta: []flux.ColMeta{
					{Label: "_time", Type: flux.TTime},
					{Label: "_value", Type: flux.TFloat},
				},
				Data: [][]interface{}{
					{execute.Time(2), 1.0},
					{execute.Time(4), 7.0},
					{execute.Time(1), 2.0},
					{execute.Time(5), 8.0},
					{execute.Time(6), 1.0},
					{execute.Time(3), 6.0},
				},
			}},
			wantErr: errors.New("got an out-of-order timestamp"),
		},
		{
			name: "only count",
			spec: &universe.StateTrackingProcedureSpec{
				CountColumn: "count",
				Fn:          gt5,
				TimeCol:     "_time",
			},
			data: []flux.Table{&executetest.Table{
				ColMeta: []flux.ColMeta{
					{Label: "_time", Type: flux.TTime},
					{Label: "_value", Type: flux.TFloat},
				},
				Data: [][]interface{}{
					{execute.Time(1), 2.0},
					{execute.Time(2), 1.0},
					{execute.Time(3), 6.0},
					{execute.Time(4), 7.0},
					{execute.Time(5), 8.0},
					{execute.Time(6), 1.0},
				},
			}},
			want: []*executetest.Table{{
				ColMeta: []flux.ColMeta{
					{Label: "_time", Type: flux.TTime},
					{Label: "_value", Type: flux.TFloat},
					{Label: "count", Type: flux.TInt},
				},
				Data: [][]interface{}{
					{execute.Time(1), 2.0, int64(-1)},
					{execute.Time(2), 1.0, int64(-1)},
					{execute.Time(3), 6.0, int64(1)},
					{execute.Time(4), 7.0, int64(2)},
					{execute.Time(5), 8.0, int64(3)},
					{execute.Time(6), 1.0, int64(-1)},
				},
			}},
		},
		{
			name: "only count, out of order and null timestamps",
			spec: &universe.StateTrackingProcedureSpec{
				CountColumn: "count",
				Fn:          gt5,
				TimeCol:     "_time",
			},
			data: []flux.Table{&executetest.Table{
				ColMeta: []flux.ColMeta{
					{Label: "_time", Type: flux.TTime},
					{Label: "_value", Type: flux.TFloat},
				},
				Data: [][]interface{}{
					{execute.Time(3), 6.0},
					{nil, 2.0},
					{execute.Time(5), 8.0},
					{nil, 1.0},
					{execute.Time(2), 7.0},
					{execute.Time(4), 10.0},
				},
			}},
			want: []*executetest.Table{{
				ColMeta: []flux.ColMeta{
					{Label: "_time", Type: flux.TTime},
					{Label: "_value", Type: flux.TFloat},
					{Label: "count", Type: flux.TInt},
				},
				Data: [][]interface{}{
					{execute.Time(3), 6.0, int64(1)},
					{nil, 2.0, int64(-1)},
					{execute.Time(5), 8.0, int64(1)},
					{nil, 1.0, int64(-1)},
					{execute.Time(2), 7.0, int64(1)},
					{execute.Time(4), 10.0, int64(2)},
				},
			}},
		},
		{
			name: "one table",
			spec: &universe.StateTrackingProcedureSpec{
				CountColumn:    "count",
				DurationColumn: "duration",
				DurationUnit:   flux.ConvertDuration(1),
				Fn:             gt5,
				TimeCol:        "_time",
			},
			data: []flux.Table{&executetest.Table{
				ColMeta: []flux.ColMeta{
					{Label: "_time", Type: flux.TTime},
					{Label: "_value", Type: flux.TFloat},
				},
				Data: [][]interface{}{
					{execute.Time(1), 2.0},
					{execute.Time(2), 1.0},
					{execute.Time(3), 6.0},
					{execute.Time(4), 7.0},
					{execute.Time(5), 8.0},
					{execute.Time(6), 1.0},
				},
			}},
			want: []*executetest.Table{{
				ColMeta: []flux.ColMeta{
					{Label: "_time", Type: flux.TTime},
					{Label: "_value", Type: flux.TFloat},
					{Label: "count", Type: flux.TInt},
					{Label: "duration", Type: flux.TInt},
				},
				Data: [][]interface{}{
					{execute.Time(1), 2.0, int64(-1), int64(-1)},
					{execute.Time(2), 1.0, int64(-1), int64(-1)},
					{execute.Time(3), 6.0, int64(1), int64(0)},
					{execute.Time(4), 7.0, int64(2), int64(1)},
					{execute.Time(5), 8.0, int64(3), int64(2)},
					{execute.Time(6), 1.0, int64(-1), int64(-1)},
				},
			}},
		},
		{
			name: "empty table",
			spec: &universe.StateTrackingProcedureSpec{
				CountColumn:    "count",
				DurationColumn: "duration",
				DurationUnit:   flux.ConvertDuration(1),
				Fn:             gt5,
				TimeCol:        "_time",
			},
			data: []flux.Table{&executetest.Table{
				ColMeta: []flux.ColMeta{
					{Label: "_time", Type: flux.TTime},
					{Label: "_start", Type: flux.TTime},
					{Label: "_stop", Type: flux.TTime},
					{Label: "_value", Type: flux.TFloat},
				},
				Data: [][]interface{}{},
			}},
			want: []*executetest.Table{{
				ColMeta: []flux.ColMeta{
					{Label: "_time", Type: flux.TTime},
					{Label: "_start", Type: flux.TTime},
					{Label: "_stop", Type: flux.TTime},
					{Label: "_value", Type: flux.TFloat},
					{Label: "count", Type: flux.TInt},
					{Label: "duration", Type: flux.TInt},
				},
				Data: nil,
			}},
		},
	}
	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			executetest.ProcessTestHelper2(
				t,
				tc.data,
				tc.want,
				tc.wantErr,
				func(id execute.DatasetID, alloc memory.Allocator) (execute.Transformation, execute.Dataset) {
					ctx, deps := dependency.Inject(context.Background(), dependenciestest.Default())
					defer deps.Finish()

					ntx, nd, err := universe.NewStateTrackingTransformation(ctx, tc.spec, id, alloc)
					if err != nil {
						t.Fatal(err)
					}
					return ntx, nd
				},
			)
		})
	}
}
