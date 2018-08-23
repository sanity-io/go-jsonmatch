package jsonmatch_test

import (
	"reflect"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sanity-io/jsonmatch"
)

var nextIdentity = 1

func varRef(value interface{}, depth int) *jsonmatch.VarRef {
	identity := "$" + strconv.Itoa(nextIdentity)
	nextIdentity++
	getter := func() interface{} {
		return value
	}
	setter := func(newValue interface{}) {
		value = newValue
	}
	return jsonmatch.NewVarRef(identity, getter, setter, depth)
}

func arrayRefFromIndicies(variable *jsonmatch.VarRef, indicies []int) *jsonmatch.ArrayRef {
	return jsonmatch.NewArrayRef(variable, jsonmatch.NewRegionsFromIndicies(indicies))
}

func TestArrayRef_Value(t *testing.T) {
	val := varRef([]interface{}{"zero", "one", "two", "three", "four"}, 0)
	ref := arrayRefFromIndicies(val, []int{3, 1, 4})
	selection := ref.Values()
	assert.Equal(t, []interface{}{"one", "three", "four"}, selection)
}

func TestArrayRef_Delete(t *testing.T) {
	base := varRef([]interface{}{"zero", "one", "two", "three", "four"}, 0)
	ref := arrayRefFromIndicies(base, []int{3, 1, 4})
	err := ref.Delete()
	assert.NoError(t, err)
	assert.Equal(t, []interface{}{"zero", "two"}, base.Value())
}

func TestArrayRef_Set(t *testing.T) {
	base := varRef([]interface{}{"zero", "one", "two", "three", "four"}, 0)
	ref := arrayRefFromIndicies(base, []int{3, 1, 4})
	err := ref.Set("waka")
	assert.NoError(t, err)
	assert.Equal(t, []interface{}{"zero", "waka", "two", "waka", "waka"}, base.Value())
}

func TestArrayRef_Mutate(t *testing.T) {
	base := varRef([]interface{}{2, 4, 6, 8}, 0)
	ref := arrayRefFromIndicies(base, []int{1, 3})
	err := ref.Mutate(func(path string, value interface{}) (interface{}, error) {
		return value.(int) / 2, nil
	})
	assert.NoError(t, err)
	assert.Equal(t, []interface{}{2, 2, 6, 4}, base.Value())
}

func TestArrayRef_MutateRegions(t *testing.T) {
	base := varRef([]interface{}{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}, 0)
	ref := arrayRefFromIndicies(base, []int{1, 2, 3, 8})
	err := ref.MutateRegions(func(path string, current [][]interface{}) ([][]interface{}, error) {
		return [][]interface{}{
			[]interface{}{"foo"},
			[]interface{}{"bar", "baz", "pow", "kapling"},
		}, nil
	})
	assert.NoError(t, err)
	assert.Equal(t, []interface{}{0, "foo", 4, 5, 6, 7, "bar", "baz", "pow", "kapling", 9}, base.Value())
}

func TestUnionRef_ArrayRefCanonicalization(t *testing.T) {
	base := varRef([]interface{}{2, 4, 6, 8}, 0)
	ref1 := arrayRefFromIndicies(base, []int{1, 3})
	ref2 := arrayRefFromIndicies(base, []int{2, 3})
	union := ref1.Union(ref2)
	_, ok := union.(*jsonmatch.ArrayRef)
	assert.True(t, ok, "Union of two ArrayRefs with same base must be an *ArrayRef")
	assert.Equal(t, []interface{}{4, 6, 8}, union.Values())
}

func TestUnionRef_ArrayRefSelectiveCanonicalization(t *testing.T) {
	base1 := varRef([]interface{}{2, 4, 6, 8}, 1)
	base2 := varRef([]interface{}{10, 20, 30, 40}, 0)
	ref1 := arrayRefFromIndicies(base1, []int{1, 3})
	ref2 := arrayRefFromIndicies(base2, []int{2, 3})
	union := ref1.Union(ref2)
	_, ok := union.(*jsonmatch.ArrayRef)
	assert.False(t, ok, "Union of two ArrayRefs with different base must be a *UnionRef")
	assert.Equal(t, []interface{}{4, 8, 30, 40}, union.Values())
}

func TestMapRef_Value(t *testing.T) {
	ref := jsonmatch.NewMapRef(
		varRef(map[string]interface{}{
			"one":   1,
			"two":   2,
			"three": 3,
			"four":  4,
		}, 0), []string{"two", "four"})
	assert.Equal(t, []interface{}{4, 2}, ref.Values())
}

func TestMapRef_Delete(t *testing.T) {
	base := varRef(map[string]interface{}{
		"one":   1,
		"two":   2,
		"three": 3,
		"four":  4,
	}, 0)
	ref := jsonmatch.NewMapRef(base, []string{"two", "four"})
	assert.NoError(t, ref.Delete())
	assert.Equal(t, map[string]interface{}{
		"one":   1,
		"three": 3,
	}, base.Value())
}

func TestMapRef_Set(t *testing.T) {
	base := varRef(map[string]interface{}{
		"one":   1,
		"two":   2,
		"three": 3,
		"four":  4,
	}, 0)
	ref := jsonmatch.NewMapRef(base, []string{"two", "four"})
	assert.NoError(t, ref.Set(100))
	assert.Equal(t, map[string]interface{}{
		"one":   1,
		"two":   100,
		"three": 3,
		"four":  100,
	}, base.Value())
}

func TestMapRef_Mutate(t *testing.T) {
	base := varRef(map[string]interface{}{
		"one":   1,
		"two":   2,
		"three": 3,
		"four":  4,
	}, 0)
	ref := jsonmatch.NewMapRef(base, []string{"two", "four"})
	err := ref.Mutate(func(path string, value interface{}) (interface{}, error) {
		return value.(int) * 100, nil
	})
	assert.NoError(t, err)
	assert.Equal(t, map[string]interface{}{
		"one":   1,
		"two":   200,
		"three": 3,
		"four":  400,
	}, base.Value())
}

func TestUnionRef_MapRefCanonicalization(t *testing.T) {
	base := varRef(map[string]interface{}{
		"one":   1,
		"two":   2,
		"three": 3,
		"four":  4,
	}, 0)
	ref1 := jsonmatch.NewMapRef(base, []string{"one", "three"})
	ref2 := jsonmatch.NewMapRef(base, []string{"two", "three"})
	union := ref1.Union(ref2)
	_, ok := union.(*jsonmatch.MapRef)
	assert.True(t, ok, "Union of two MapRefs with same base must be a *MapRef")
	assert.Equal(t, []interface{}{1, 3, 2}, union.Values())
}

func TestUnionRef_MapRefSelectiveCanonicalization(t *testing.T) {
	base1 := varRef(map[string]interface{}{
		"one":   1,
		"two":   2,
		"three": 3,
		"four":  4,
	}, 1)
	base2 := varRef(map[string]interface{}{
		"ten":    10,
		"twenty": 20,
	}, 0)
	ref1 := jsonmatch.NewMapRef(base1, []string{"one", "three"})
	ref2 := jsonmatch.NewMapRef(base2, []string{"ten"})
	union := ref1.Union(ref2)
	_, ok := union.(*jsonmatch.UnionRef)
	assert.True(t, ok, "Union of two MapRefs with different base must be a *UnionRef")
	assert.Equal(t, []interface{}{1, 3, 10}, union.Values())
}

func TestVarRef_typePreservation(t *testing.T) {
	type mapAlias map[string]interface{}
	base := varRef(mapAlias{"one": 1}, 0)
	assert.Equal(t, "jsonmatch_test.mapAlias", reflect.TypeOf(base.Value()).String(), "Value should not be the canonical type")
	assert.Equal(t, "map[string]interface {}", reflect.TypeOf(base.CanonicalValue()).String(), "Value should not be the canonical type")
	assert.NoError(t, base.SetWithMatchedType(map[string]interface{}{"banana": 2}))
	assert.Equal(t, "jsonmatch_test.mapAlias", reflect.TypeOf(base.Value()).String(),
		"Type of original value should be preserved after SetWithMatchedType using a canonical type")
	assert.NoError(t, base.Set(map[string]interface{}{"bandana": 3}))
	assert.Equal(t, "map[string]interface {}", reflect.TypeOf(base.Value()).String(),
		"Type of original value should be overwritten after straight Set even using a canonical type")
}

func TestUnionRef_MutateMissingKey(t *testing.T) {
	base := varRef(map[string]interface{}{}, 0)
	ref := jsonmatch.NewLatentMapRef(
		jsonmatch.NewMapRef(base, []string{"one"}), []string{"two"})
	err := ref.Mutate(func(_ string, value interface{}) (interface{}, error) {
		return "three", nil
	})
	assert.NoError(t, err)
	mutated := base.Value()
	ms, err := jsonmatch.Match("one.two", mutated)
	require.NoError(t, err)
	assert.Equal(t, []interface{}{"three"}, ms.Values())
}
