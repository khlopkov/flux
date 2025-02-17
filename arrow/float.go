package arrow

import (
	"github.com/InfluxCommunity/flux/array"
	"github.com/InfluxCommunity/flux/memory"
)

func NewFloat(vs []float64, alloc memory.Allocator) *array.Float {
	b := NewFloatBuilder(alloc)
	b.Resize(len(vs))
	for _, v := range vs {
		b.UnsafeAppend(v)
	}
	a := b.NewFloatArray()
	b.Release()
	return a
}

func FloatSlice(arr *array.Float, i, j int) *array.Float {
	return Slice(arr, int64(i), int64(j)).(*array.Float)
}

func NewFloatBuilder(a memory.Allocator) *array.FloatBuilder {
	if a == nil {
		a = memory.DefaultAllocator
	}
	return array.NewFloatBuilder(a)
}
