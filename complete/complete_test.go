package complete_test

import (
	"context"
	"testing"

	"github.com/InfluxCommunity/flux/complete"
	"github.com/InfluxCommunity/flux/semantic"
	"github.com/InfluxCommunity/flux/values"
	"github.com/google/go-cmp/cmp"
)

func TestNames(t *testing.T) {
	s := values.NewScope()
	v := values.NewInt(0)
	s.Set("boom", v)
	s.Set("tick", v)

	c := complete.NewCompleter(s)

	results := c.Names()
	expected := []string{
		"boom",
		"tick",
	}

	if !cmp.Equal(results, expected) {
		t.Error(cmp.Diff(results, expected), "unexpected names from declarations")
	}
}

func TestValue(t *testing.T) {
	name := "foo"
	scope := values.NewScope()
	value := values.NewInt(5)
	scope.Set(name, value)

	v, _ := complete.NewCompleter(scope).Value(name)

	if !cmp.Equal(value, v) {
		t.Error(cmp.Diff(value, v), "unexpected value for name")
	}
}

func TestFunctionNames(t *testing.T) {
	boom := values.NewFunction(
		"boom",
		semantic.NewFunctionType(semantic.BasicInt, nil),
		func(context.Context, values.Object) (values.Value, error) {
			return values.NewInt(5), nil
		},
		false,
	)
	s := values.NewScope()
	s.Set("boom", boom)
	c := complete.NewCompleter(s)
	results := c.FunctionNames()

	expected := []string{
		"boom",
	}

	if !cmp.Equal(results, expected) {
		t.Error(cmp.Diff(results, expected), "unexpected function names")
	}
}

func TestFunctionSuggestion(t *testing.T) {
	name := "bar"
	bar := values.NewFunction(
		name,
		semantic.NewFunctionType(semantic.BasicInt, []semantic.ArgumentType{
			{
				Name: []byte("start"),
				Type: semantic.BasicTime,
			},
			{
				Name: []byte("stop"),
				Type: semantic.BasicTime,
			},
		}),
		func(context.Context, values.Object) (values.Value, error) {
			return values.NewInt(5), nil
		},
		false,
	)
	s := values.NewScope()
	s.Set(name, bar)
	result, _ := complete.NewCompleter(s).FunctionSuggestion(name)

	expected := complete.FunctionSuggestion{
		Params: map[string]string{
			"start": semantic.Time.String(),
			"stop":  semantic.Time.String(),
		},
	}

	if !cmp.Equal(result, expected) {
		t.Error(cmp.Diff(result, expected), "does not match expected suggestion")
	}
}
