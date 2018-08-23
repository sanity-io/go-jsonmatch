package jsonmatch

import (
	"errors"
	"fmt"
)

// MatchSet represents one jsonmatch extract and provides functions for
// mutating or extracting the matched values. Setting selected values can
// be done until you have performed a mutation. You can only perform exactly
// one mutation.
type MatchSet struct {
	// The variable reference containing the root value of the extract
	root *VarRef
	// The ref describing the matched values of the extract
	ref Ref
	// True if the underlying value has been mutated
	mutated bool
}

// Values returns an array of all the values selected by the jsonmatch
func (e *MatchSet) Values() []interface{} {
	if e.mutated {
		panic("Values are not availible after extract has been mutated")
	}
	return e.ref.Values()
}

// Set updates all selected values to the provided value
func (e *MatchSet) Set(value interface{}) (interface{}, error) {
	if e.mutated {
		return nil, errors.New("This extract has allready mutated once")
	}
	err := e.ref.Set(value)
	if err != nil {
		return nil, err
	}
	e.mutated = true
	return e.root.Value(), nil
}

// Delete deletes all selected values from the underlying data
func (e *MatchSet) Delete() (interface{}, error) {
	if e.mutated {
		return nil, errors.New("This extract has allready mutated once")
	}
	err := e.ref.Delete()
	if err != nil {
		return nil, err
	}
	e.mutated = true
	return e.root.Value(), nil
}

// Mutate passes all selected values through the mutator and updates them
// in the underlying value
func (e *MatchSet) Mutate(mutator MutatorFunc) (interface{}, error) {
	if e.mutated {
		return nil, errors.New("This extract has allready mutated once")
	}
	err := e.ref.Mutate(mutator)
	if err != nil {
		return nil, err
	}
	e.mutated = true
	return e.root.Value(), nil
}

// MutateRegions mutates the elements of each selected array as single operations. The
// original valuas are provided as an array of arrays, and returns an array of arrays
// of the same shape. Each sub-array is substituted for its corresponding original,
// and it is okay if the length of these sub arrays differ. The result is grown or
// shrunk accordingly. Main use case is to support slice-style operations like append,
// replace etc. If the method is used on selections that also include non-array
// members, an error is returned.
func (e *MatchSet) MutateRegions(mutator MutateRegionsFunc) (interface{}, error) {
	if e.mutated {
		return nil, errors.New("This extract has allready mutated once")
	}

	// Extract all array references to be mutated
	var arrayRefs []*ArrayRef
	switch t := e.ref.(type) {
	case *ArrayRef:
		arrayRefs = []*ArrayRef{t}
	case *UnionRef:
		arrayRefs = make([]*ArrayRef, 0, len(t.refs))
		for _, r := range t.refs {
			arrayRef, ok := r.(*ArrayRef)
			if !ok {
				return nil, fmt.Errorf("Cannot mutate regions of a %T ref. All selected values must be array members", r)
			}
			arrayRefs = append(arrayRefs, arrayRef)
		}
	}

	// Perform the mutations
	for _, ref := range arrayRefs {
		if err := ref.MutateRegions(mutator); err != nil {
			return nil, err
		}
	}
	e.mutated = true
	return e.root.Value(), nil
}
