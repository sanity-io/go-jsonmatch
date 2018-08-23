package jsonmatch

import (
	"errors"
	"reflect"
)

// Canonicalization of types: For slices and maps the jsonmatch system uses
// []interface{} and map[string]interface{} respectively. The client may
// use any type alias they want for these types, but we need to convert them
// to their canoncial types while processing avoid reflection-fireworks in our code.
var canonicalMapType = reflect.TypeOf(map[string]interface{}{})
var canonicalSliceType = reflect.TypeOf([]interface{}{})

func isCanonicalType(t reflect.Type) bool {
	return t == canonicalMapType || t == canonicalSliceType
}

// Converts maps and arrays to their canonical types
func toCanonicalType(value interface{}) (interface{}, bool, error) {
	valueType := reflect.TypeOf(value)
	if valueType.Kind() == reflect.Map && valueType != canonicalMapType {
		if !valueType.ConvertibleTo(canonicalMapType) {
			return nil, false, errors.New("Maps used with jsonmatch must be convertible to map[string]interface{}")
		}
		return reflect.ValueOf(value).Convert(canonicalMapType).Interface(), true, nil
	} else if valueType.Kind() == reflect.Slice && valueType != canonicalSliceType {
		if !valueType.ConvertibleTo(canonicalSliceType) {
			slice, _ := intoInterfaceSlice(value)
			return slice, true, nil
		}
		return reflect.ValueOf(value).Convert(canonicalSliceType).Interface(), true, nil
	} else {
		return value, false, nil
	}
}

// Will attempt to convert the newValue to the same type as the oldValue if
// the new value is of a canonical type and the underlying old type is compatible
func matchType(newValue interface{}, oldValue interface{}) interface{} {
	newValueType := reflect.TypeOf(newValue)
	if !isCanonicalType(newValueType) {
		return newValue
	}
	oldValueType := reflect.TypeOf(oldValue)
	if newValueType == oldValueType {
		return newValue
	}
	if newValueType.ConvertibleTo(oldValueType) {
		return reflect.ValueOf(newValue).Convert(oldValueType).Interface()
	}
	return newValue
}

// Checks that a type is compatible with the refs system
func assertIsCompatible(value interface{}) error {
	_, _, err := toCanonicalType(value)
	return err
}
