package universe_test

import (
	"testing"

	"github.com/InfluxCommunity/flux"
	"github.com/InfluxCommunity/flux/execute"
	"github.com/InfluxCommunity/flux/execute/executetest"
	"github.com/InfluxCommunity/flux/memory"
	"github.com/InfluxCommunity/flux/stdlib/universe"
)

func TestCumulativeSum_Process(t *testing.T) {
	testCases := []struct {
		name string
		spec *universe.CumulativeSumProcedureSpec
		data []flux.Table
		want []*executetest.Table
	}{
		{
			name: "float",
			spec: &universe.CumulativeSumProcedureSpec{
				Columns: []string{execute.DefaultValueColLabel},
			},
			data: []flux.Table{&executetest.Table{
				ColMeta: []flux.ColMeta{
					{Label: "_time", Type: flux.TTime},
					{Label: "_value", Type: flux.TFloat},
				},
				Data: [][]interface{}{
					{execute.Time(0), 2.0},
					{execute.Time(1), 1.0},
					{execute.Time(2), 3.0},
					{execute.Time(3), 4.0},
					{execute.Time(4), 2.0},
					{execute.Time(5), 6.0},
					{execute.Time(6), 2.0},
					{execute.Time(7), 7.0},
					{execute.Time(8), 3.0},
					{execute.Time(9), 8.0},
				},
			}},
			want: []*executetest.Table{{
				ColMeta: []flux.ColMeta{
					{Label: "_time", Type: flux.TTime},
					{Label: "_value", Type: flux.TFloat},
				},
				Data: [][]interface{}{
					{execute.Time(0), 2.0},
					{execute.Time(1), 3.0},
					{execute.Time(2), 6.0},
					{execute.Time(3), 10.0},
					{execute.Time(4), 12.0},
					{execute.Time(5), 18.0},
					{execute.Time(6), 20.0},
					{execute.Time(7), 27.0},
					{execute.Time(8), 30.0},
					{execute.Time(9), 38.0},
				},
			}},
		},
		{
			name: "with null",
			spec: &universe.CumulativeSumProcedureSpec{
				Columns: []string{execute.DefaultValueColLabel},
			},
			data: []flux.Table{&executetest.Table{
				ColMeta: []flux.ColMeta{
					{Label: "_time", Type: flux.TTime},
					{Label: "_value", Type: flux.TFloat},
				},
				Data: [][]interface{}{
					{execute.Time(0), 2.0},
					{execute.Time(1), 1.0},
					{execute.Time(2), nil},
					{execute.Time(3), 4.0},
					{execute.Time(4), nil},
					{execute.Time(5), 6.0},
					{execute.Time(6), 2.0},
					{execute.Time(7), nil},
					{execute.Time(8), nil},
					{execute.Time(9), 8.0},
				},
			}},
			want: []*executetest.Table{{
				ColMeta: []flux.ColMeta{
					{Label: "_time", Type: flux.TTime},
					{Label: "_value", Type: flux.TFloat},
				},
				Data: [][]interface{}{
					{execute.Time(0), 2.0},
					{execute.Time(1), 3.0},
					{execute.Time(2), 3.0},
					{execute.Time(3), 7.0},
					{execute.Time(4), 7.0},
					{execute.Time(5), 13.0},
					{execute.Time(6), 15.0},
					{execute.Time(7), 15.0},
					{execute.Time(8), 15.0},
					{execute.Time(9), 23.0},
				},
			}},
		},
		{
			name: "multiple value columns",
			spec: &universe.CumulativeSumProcedureSpec{
				Columns: []string{"int", "uint", "float"},
			},
			data: []flux.Table{&executetest.Table{
				ColMeta: []flux.ColMeta{
					{Label: "_time", Type: flux.TTime},
					{Label: "int", Type: flux.TInt},
					{Label: "uint", Type: flux.TUInt},
					{Label: "float", Type: flux.TFloat},
				},
				Data: [][]interface{}{
					{execute.Time(0), int64(2), uint64(1), 1.0},
					{execute.Time(1), int64(1), uint64(2), 1.0},
					{execute.Time(2), int64(3), uint64(3), 2.0},
					{execute.Time(3), int64(4), uint64(4), 13.0},
					{execute.Time(4), int64(2), uint64(5), 4.0},
					{execute.Time(5), int64(6), uint64(6), 5.0},
					{execute.Time(6), int64(2), uint64(7), -7.0},
					{execute.Time(7), int64(-7), uint64(8), 2.0},
					{execute.Time(8), int64(3), uint64(9), -6.0},
					{execute.Time(9), int64(8), uint64(11), 3.0},
				},
			}},
			want: []*executetest.Table{{
				ColMeta: []flux.ColMeta{
					{Label: "_time", Type: flux.TTime},
					{Label: "int", Type: flux.TInt},
					{Label: "uint", Type: flux.TUInt},
					{Label: "float", Type: flux.TFloat},
				},
				Data: [][]interface{}{
					{execute.Time(0), int64(2), uint64(1), 1.0},
					{execute.Time(1), int64(3), uint64(3), 2.0},
					{execute.Time(2), int64(6), uint64(6), 4.0},
					{execute.Time(3), int64(10), uint64(10), 17.0},
					{execute.Time(4), int64(12), uint64(15), 21.0},
					{execute.Time(5), int64(18), uint64(21), 26.0},
					{execute.Time(6), int64(20), uint64(28), 19.0},
					{execute.Time(7), int64(13), uint64(36), 21.0},
					{execute.Time(8), int64(16), uint64(45), 15.0},
					{execute.Time(9), int64(24), uint64(56), 18.0},
				},
			}},
		},
		{
			name: "multiple time columns",
			spec: &universe.CumulativeSumProcedureSpec{
				Columns: []string{"int", "float"},
			},
			data: []flux.Table{&executetest.Table{
				ColMeta: []flux.ColMeta{
					{Label: "_time", Type: flux.TTime},
					{Label: "time2", Type: flux.TTime},
					{Label: "int", Type: flux.TInt},
					{Label: "float", Type: flux.TFloat},
				},
				Data: [][]interface{}{
					{execute.Time(0), execute.Time(0), int64(2), 1.0},
					{execute.Time(1), execute.Time(1), int64(1), 1.0},
					{execute.Time(2), execute.Time(2), int64(3), 2.0},
					{execute.Time(3), execute.Time(3), int64(4), 13.0},
					{execute.Time(4), execute.Time(4), int64(2), 4.0},
					{execute.Time(5), execute.Time(5), int64(6), 5.0},
					{execute.Time(6), execute.Time(6), int64(2), -7.0},
					{execute.Time(7), execute.Time(7), int64(7), 2.0},
					{execute.Time(8), execute.Time(8), int64(3), -6.0},
					{execute.Time(9), execute.Time(9), int64(8), 3.0},
				},
			}},
			want: []*executetest.Table{{
				ColMeta: []flux.ColMeta{
					{Label: "_time", Type: flux.TTime},
					{Label: "time2", Type: flux.TTime},
					{Label: "int", Type: flux.TInt},
					{Label: "float", Type: flux.TFloat},
				},
				Data: [][]interface{}{
					{execute.Time(0), execute.Time(0), int64(2), 1.0},
					{execute.Time(1), execute.Time(1), int64(3), 2.0},
					{execute.Time(2), execute.Time(2), int64(6), 4.0},
					{execute.Time(3), execute.Time(3), int64(10), 17.0},
					{execute.Time(4), execute.Time(4), int64(12), 21.0},
					{execute.Time(5), execute.Time(5), int64(18), 26.0},
					{execute.Time(6), execute.Time(6), int64(20), 19.0},
					{execute.Time(7), execute.Time(7), int64(27), 21.0},
					{execute.Time(8), execute.Time(8), int64(30), 15.0},
					{execute.Time(9), execute.Time(9), int64(38), 18.0},
				},
			}},
		},
		{
			name: "tag columns",
			spec: &universe.CumulativeSumProcedureSpec{
				Columns: []string{"int"},
			},
			data: []flux.Table{&executetest.Table{
				ColMeta: []flux.ColMeta{
					{Label: "_time", Type: flux.TTime},
					{Label: "int", Type: flux.TInt},
					{Label: "t", Type: flux.TString},
				},
				Data: [][]interface{}{
					{execute.Time(0), int64(2), "tag0"},
					{execute.Time(1), int64(1), "tag0"},
					{execute.Time(2), int64(3), "tag1"},
					{execute.Time(3), int64(4), "tag1"},
					{execute.Time(4), int64(2), "tag0"},
					{execute.Time(5), int64(6), "tag0"},
					{execute.Time(6), int64(2), "tag1"},
					{execute.Time(7), int64(7), "tag1"},
					{execute.Time(8), int64(3), "tag0"},
					{execute.Time(9), int64(8), "tag0"},
				},
			}},
			want: []*executetest.Table{{
				ColMeta: []flux.ColMeta{
					{Label: "_time", Type: flux.TTime},
					{Label: "int", Type: flux.TInt},
					{Label: "t", Type: flux.TString},
				},
				Data: [][]interface{}{
					{execute.Time(0), int64(2), "tag0"},
					{execute.Time(1), int64(3), "tag0"},
					{execute.Time(2), int64(6), "tag1"},
					{execute.Time(3), int64(10), "tag1"},
					{execute.Time(4), int64(12), "tag0"},
					{execute.Time(5), int64(18), "tag0"},
					{execute.Time(6), int64(20), "tag1"},
					{execute.Time(7), int64(27), "tag1"},
					{execute.Time(8), int64(30), "tag0"},
					{execute.Time(9), int64(38), "tag0"},
				},
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
				nil,
				func(id execute.DatasetID, alloc memory.Allocator) (execute.Transformation, execute.Dataset) {
					tr, d, err := universe.NewCumulativeSumTransformation(id, tc.spec, alloc)
					if err != nil {
						t.Fatal(err)
					}
					return tr, d
				},
			)
		})
	}
}
