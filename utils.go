package jsonmatch

import (
	"encoding/json"
	"reflect"
	"sort"
)

func findInKeys(value string, set []string) (int, bool) {
	for i, v := range set {
		if value == v {
			return i, true
		}
	}
	return -1, false
}

func unionKeys(a, b []string) []string {
	// TODO: Optimize by sorting superset and using the same "last trick" as in
	// unionIndicies
	superset := append(a, b...)
	result := make([]string, 0, len(superset))
	for _, candidate := range superset {
		if _, found := findInKeys(candidate, result); !found {
			result = append(result, candidate)
		}
	}
	sort.Strings(result)
	return result
}

func allKeysOfMap(m map[string]interface{}) []string {
	result := make([]string, 0, len(m))
	for key := range m {
		result = append(result, key)
	}
	return result
}

// Returns a Ref to all values on the first sub level of the supplied ref
func matchAllChildren(varRef *VarRef) Ref {
	result := NewEmptyRef()
	if varRef.IsMap() {
		result = result.Union(NewMapRef(varRef, allKeysOfMap(varRef.CanonicalValue().(map[string]interface{}))))
	} else if varRef.IsSlice() {
		result = result.Union(NewArrayRef(varRef, Regions{Region{0, len(varRef.CanonicalValue().([]interface{}))}}))
	}
	return result
}

// floatFromValue converts any numeric value to a float64.
func floatFromValue(v interface{}) (float64, bool) {
	switch t := v.(type) {
	case float32:
		return float64(t), true
	case float64:
		return t, true
	case int:
		return float64(t), true
	case int8:
		return float64(t), true
	case int16:
		return float64(t), true
	case int32:
		return float64(t), true
	case int64:
		return float64(t), true
	case uint:
		return float64(t), true
	case uint8:
		return float64(t), true
	case uint16:
		return float64(t), true
	case uint32:
		return float64(t), true
	case uint64:
		return float64(t), true
	case json.Number:
		if f, err := t.Float64(); err == nil {
			return f, true
		} else if i, err := t.Int64(); err == nil {
			return float64(i), true
		}
	}
	return 0, false
}

// intoInterfaceSlice takes a slice and returns a neutral slice of interfaces.
// If the input value is not a slice, it returns false as the second return
// value. If the input value is already []interface{}, the value is returned
// as-is.
func intoInterfaceSlice(value interface{}) ([]interface{}, bool) {
	v := reflect.ValueOf(value)
	if v.Kind() != reflect.Slice {
		return nil, false
	}

	if v.Type() == intfSliceType {
		return v.Interface().([]interface{}), true
	}

	length := v.Len()
	slice := make([]interface{}, length)
	for i := 0; i < length; i++ {
		slice[i] = v.Index(i).Interface()
	}
	return slice, true
}

var intfSliceType = reflect.TypeOf([]interface{}{})
