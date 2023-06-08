// Generated by tmpl
// https://github.com/benbjohnson/tmpl
//
// DO NOT EDIT!
// Source: compare.gen.go.tmpl

package arrowutil

import (
	"fmt"

	"github.com/InfluxCommunity/flux/array"
)

// CompareFunc defines the interface for a comparison function.
// The comparison function should return 0 for equivalent values,
// -1 if x[i] is before y[j], and +1 if x[i] is after y[j].
type CompareFunc func(x, y array.Array, i, j int) int

// Compare will compare two values in the various arrays.
// The result will be 0 if x[i] == y[j], -1 if x[i] < y[j], and +1 if x[i] > y[j].
// A null value is always less than every non-null value.
func Compare(x, y array.Array, i, j int) int {
	switch x := x.(type) {

	case *array.Int:
		return IntCompare(x, y.(*array.Int), i, j)

	case *array.Uint:
		return UintCompare(x, y.(*array.Uint), i, j)

	case *array.Float:
		return FloatCompare(x, y.(*array.Float), i, j)

	case *array.Boolean:
		return BooleanCompare(x, y.(*array.Boolean), i, j)

	case *array.String:
		return StringCompare(x, y.(*array.String), i, j)

	default:
		panic(fmt.Errorf("unsupported array data type: %s", x.DataType()))
	}
}

// CompareDesc will compare two values in the various arrays.
// The result will be 0 if x[i] == y[j], -1 if x[i] > y[j], and +1 if x[i] < y[j].
// A null value is always greater than every non-null value.
func CompareDesc(x, y array.Array, i, j int) int {
	switch x := x.(type) {

	case *array.Int:
		return IntCompareDesc(x, y.(*array.Int), i, j)

	case *array.Uint:
		return UintCompareDesc(x, y.(*array.Uint), i, j)

	case *array.Float:
		return FloatCompareDesc(x, y.(*array.Float), i, j)

	case *array.Boolean:
		return BooleanCompareDesc(x, y.(*array.Boolean), i, j)

	case *array.String:
		return StringCompareDesc(x, y.(*array.String), i, j)

	default:
		panic(fmt.Errorf("unsupported array data type: %s", x.DataType()))
	}
}

func IntCompare(x, y *array.Int, i, j int) int {
	if x.IsNull(i) {
		if y.IsNull(j) {
			return 0
		}
		return -1
	} else if y.IsNull(j) {
		return 1
	}

	if l, r := x.Value(i), y.Value(j); l < r {
		return -1
	} else if l == r {
		return 0
	}
	return 1

}

func IntCompareDesc(x, y *array.Int, i, j int) int {
	if x.IsNull(i) {
		if y.IsNull(j) {
			return 0
		}
		return -1
	} else if y.IsNull(j) {
		return 1
	}

	if l, r := x.Value(i), y.Value(j); l > r {
		return -1
	} else if l == r {
		return 0
	}
	return 1

}

func UintCompare(x, y *array.Uint, i, j int) int {
	if x.IsNull(i) {
		if y.IsNull(j) {
			return 0
		}
		return -1
	} else if y.IsNull(j) {
		return 1
	}

	if l, r := x.Value(i), y.Value(j); l < r {
		return -1
	} else if l == r {
		return 0
	}
	return 1

}

func UintCompareDesc(x, y *array.Uint, i, j int) int {
	if x.IsNull(i) {
		if y.IsNull(j) {
			return 0
		}
		return -1
	} else if y.IsNull(j) {
		return 1
	}

	if l, r := x.Value(i), y.Value(j); l > r {
		return -1
	} else if l == r {
		return 0
	}
	return 1

}

func FloatCompare(x, y *array.Float, i, j int) int {
	if x.IsNull(i) {
		if y.IsNull(j) {
			return 0
		}
		return -1
	} else if y.IsNull(j) {
		return 1
	}

	if l, r := x.Value(i), y.Value(j); l < r {
		return -1
	} else if l == r {
		return 0
	}
	return 1

}

func FloatCompareDesc(x, y *array.Float, i, j int) int {
	if x.IsNull(i) {
		if y.IsNull(j) {
			return 0
		}
		return -1
	} else if y.IsNull(j) {
		return 1
	}

	if l, r := x.Value(i), y.Value(j); l > r {
		return -1
	} else if l == r {
		return 0
	}
	return 1

}

func BooleanCompare(x, y *array.Boolean, i, j int) int {
	if x.IsNull(i) {
		if y.IsNull(j) {
			return 0
		}
		return -1
	} else if y.IsNull(j) {
		return 1
	}

	if x.Value(i) {
		if y.Value(j) {
			return 0
		}
		return 1
	} else if y.Value(j) {
		return -1
	}
	return 0

}

func BooleanCompareDesc(x, y *array.Boolean, i, j int) int {
	if x.IsNull(i) {
		if y.IsNull(j) {
			return 0
		}
		return -1
	} else if y.IsNull(j) {
		return 1
	}

	if x.Value(i) {
		if y.Value(j) {
			return 0
		}
		return -1
	} else if y.Value(j) {
		return 1
	}
	return 0

}

func StringCompare(x, y *array.String, i, j int) int {
	if x.IsNull(i) {
		if y.IsNull(j) {
			return 0
		}
		return -1
	} else if y.IsNull(j) {
		return 1
	}

	if l, r := x.Value(i), y.Value(j); l < r {
		return -1
	} else if l == r {
		return 0
	}
	return 1

}

func StringCompareDesc(x, y *array.String, i, j int) int {
	if x.IsNull(i) {
		if y.IsNull(j) {
			return 0
		}
		return -1
	} else if y.IsNull(j) {
		return 1
	}

	if l, r := x.Value(i), y.Value(j); l > r {
		return -1
	} else if l == r {
		return 0
	}
	return 1

}
