package experimental_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/InfluxCommunity/flux"
	"github.com/InfluxCommunity/flux/execute"
	"github.com/InfluxCommunity/flux/execute/executetest"
	_ "github.com/InfluxCommunity/flux/fluxinit/static"
	"github.com/InfluxCommunity/flux/internal/operation"
	"github.com/InfluxCommunity/flux/querytest"
	"github.com/InfluxCommunity/flux/stdlib/experimental"
	"github.com/InfluxCommunity/flux/stdlib/influxdata/influxdb"
	"github.com/InfluxCommunity/flux/stdlib/universe"
)

func TestGroup_NewQuery(t *testing.T) {
	tests := []querytest.NewQueryTestCase{
		{
			Name: "experimental group extend",
			Raw: `import "experimental"
from(bucket: "telegraf") |> range(start: -1m) |> experimental.group(mode: "extend", columns: ["a"])`,
			Want: &operation.Spec{
				Operations: []*operation.Node{
					{
						ID: "from0",
						Spec: &influxdb.FromOpSpec{
							Bucket: influxdb.NameOrID{Name: "telegraf"},
						},
					},
					{
						ID: "range1",
						Spec: &universe.RangeOpSpec{
							Start: flux.Time{
								Relative:   -1 * time.Minute,
								IsRelative: true,
							},
							Stop:        flux.Time{IsRelative: true},
							TimeColumn:  "_time",
							StartColumn: "_start",
							StopColumn:  "_stop",
						},
					},
					{
						ID:   "experimental-group2",
						Spec: &experimental.GroupOpSpec{Mode: "extend", Columns: []string{"a"}},
					},
				},
				Edges: []operation.Edge{
					{Parent: "from0", Child: "range1"},
					{Parent: "range1", Child: "experimental-group2"},
				},
			},
		},
		{
			// only "extend" is supportd for experimental.group()
			Name: "experimental group invalid mode",
			Raw: `import "experimental"
from(bucket: "telegraf") |> range(start: -1m) |> experimental.group(mode: "by", columns: ["a"])`,
			WantErr: true,
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

func TestGroup_Process(t *testing.T) {
	testCases := []struct {
		name    string
		spec    *experimental.GroupProcedureSpec
		data    []flux.Table
		want    []*executetest.Table
		wantErr error
	}{
		{
			name: "extend",
			spec: &experimental.GroupProcedureSpec{
				GroupKeys: []string{"t2"},
			},
			data: []flux.Table{
				&executetest.Table{
					KeyCols: []string{"t1"},
					ColMeta: []flux.ColMeta{
						{Label: "_time", Type: flux.TTime},
						{Label: "_value", Type: flux.TFloat},
						{Label: "t1", Type: flux.TString},
						{Label: "t2", Type: flux.TString},
					},
					Data: [][]interface{}{
						{execute.Time(1), 2.0, "a", "x"},
						{execute.Time(2), 1.0, "a", "y"},
					},
				},
				&executetest.Table{
					KeyCols: []string{"t1"},
					ColMeta: []flux.ColMeta{
						{Label: "_time", Type: flux.TTime},
						{Label: "_value", Type: flux.TFloat},
						{Label: "t1", Type: flux.TString},
						{Label: "t2", Type: flux.TString},
					},
					Data: [][]interface{}{
						{execute.Time(1), 4.0, "b", "x"},
						{execute.Time(2), 7.0, "b", "y"},
					},
				},
			},
			want: []*executetest.Table{
				{
					KeyCols: []string{"t1", "t2"},
					ColMeta: []flux.ColMeta{
						{Label: "_time", Type: flux.TTime},
						{Label: "_value", Type: flux.TFloat},
						{Label: "t1", Type: flux.TString},
						{Label: "t2", Type: flux.TString},
					},
					Data: [][]interface{}{
						{execute.Time(1), 2.0, "a", "x"},
					},
				},
				{
					KeyCols: []string{"t1", "t2"},
					ColMeta: []flux.ColMeta{
						{Label: "_time", Type: flux.TTime},
						{Label: "_value", Type: flux.TFloat},
						{Label: "t1", Type: flux.TString},
						{Label: "t2", Type: flux.TString},
					},
					Data: [][]interface{}{
						{execute.Time(2), 1.0, "a", "y"},
					},
				},
				{
					KeyCols: []string{"t1", "t2"},
					ColMeta: []flux.ColMeta{
						{Label: "_time", Type: flux.TTime},
						{Label: "_value", Type: flux.TFloat},
						{Label: "t1", Type: flux.TString},
						{Label: "t2", Type: flux.TString},
					},
					Data: [][]interface{}{
						{execute.Time(1), 4.0, "b", "x"},
					},
				},
				{
					KeyCols: []string{"t1", "t2"},
					ColMeta: []flux.ColMeta{
						{Label: "_time", Type: flux.TTime},
						{Label: "_value", Type: flux.TFloat},
						{Label: "t1", Type: flux.TString},
						{Label: "t2", Type: flux.TString},
					},
					Data: [][]interface{}{
						{execute.Time(2), 7.0, "b", "y"},
					},
				},
			},
		},
		{
			name: "nonexistent column",
			spec: &experimental.GroupProcedureSpec{
				GroupKeys: []string{"t3"},
			},
			data: []flux.Table{
				&executetest.Table{
					KeyCols: []string{"t1"},
					ColMeta: []flux.ColMeta{
						{Label: "_time", Type: flux.TTime},
						{Label: "_value", Type: flux.TFloat},
						{Label: "t1", Type: flux.TString},
						{Label: "t2", Type: flux.TString},
					},
					Data: [][]interface{}{
						{execute.Time(1), 2.0, "a", "x"},
						{execute.Time(2), 1.0, "a", "y"},
					},
				},
			},
			wantErr: fmt.Errorf(`unknown column "t3"`),
		},
	}
	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			executetest.ProcessTestHelper(
				t,
				tc.data,
				tc.want,
				tc.wantErr,
				func(d execute.Dataset, c execute.TableBuilderCache) execute.Transformation {
					return experimental.NewGroupTransformation(d, c, tc.spec)
				},
			)
		})
	}
}
