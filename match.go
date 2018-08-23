package jsonmatch

import (
	"fmt"

	"github.com/sanity-io/jsonmatch/template"
)

// Match runs the jsonmatch query on the data and returns a MatchSet
// referencing all matches
func (expr *Expression) Match(data interface{}) (*MatchSet, error) {
	// Setup the root VarRef
	getter := func() interface{} {
		return data
	}
	setter := func(value interface{}) {
		data = value
	}
	rootVar := &VarRef{
		identity: "$",
		setter:   setter,
		getter:   getter,
	}

	ref, err := process(rootVar, expr.root)
	if err != nil {
		return nil, err
	}

	return &MatchSet{
		root:    rootVar,
		ref:     ref,
		mutated: false,
	}, nil
}

func process(input Ref, root node) (Ref, error) {
	switch n := root.(type) {
	case *pathNode:
		return processPath(input, n)
	case *stringNode:
		return NewLiteralRef(n.value), nil
	case *fieldNode:
		return processField(input, n)
	case *existingFieldNode:
		return processExistingField(input, n)
	case *indexNode:
		return processIndex(input, n)
	case *sliceNode:
		return processSlice(input, n)
	case *filterNode:
		return processFilter(input, n)
	case *intNode:
		return NewLiteralRef(n.value), nil
	case *floatNode:
		return NewLiteralRef(n.value), nil
	case *wildcardNode:
		return processWildcard(input, n)
	case *recursiveNode:
		return processRecursive(input, n)
	case *unionNode:
		return processUnion(input, n)
	case *selfNode:
		return input, nil
	// case *IdentifierNode:
	// 	return j.evalIdentifier(value, node)
	default:
		return nil, fmt.Errorf("unexpected node %v", root)
	}
}

// processPath evaluates pathNode
func processPath(input Ref, list *pathNode) (Ref, error) {
	var err error
	result := input
	for _, n := range list.nodes {
		result, err = process(result, n)
		if err != nil {
			return nil, err
		}
	}
	return result, nil
}

// processField evaluates field of struct or key of map.
func processFieldSelection(input Ref, name string, requireFieldToExist bool) (Ref, error) {
	results := NewEmptyRef()
	for _, varRef := range input.Vars() {
		if varRef.IsMap() {
			// If field must be pre-existing, we check that here and potantially reject the match
			if requireFieldToExist {
				value := varRef.CanonicalValue().(map[string]interface{})
				if _, hasField := value[name]; !hasField {
					continue
				}
			}
			results = results.Union(NewMapRef(varRef, []string{name}))
		}
	}
	if !requireFieldToExist {
		for _, latent := range latentMapRefs(input) {
			latent.AddKey(name)
			results = results.Union(latent)
		}
	}
	return results, nil
}

func processField(input Ref, node *fieldNode) (Ref, error) {
	return processFieldSelection(input, node.name, false)
}

func processExistingField(input Ref, node *existingFieldNode) (Ref, error) {
	return processFieldSelection(input, node.name, true)
}

// evalArray evaluates sliceNode
func processSlice(input Ref, node *sliceNode) (Ref, error) {
	result := NewEmptyRef()
	for _, varRef := range input.Vars() {
		if varRef.IsSlice() {
			value := varRef.CanonicalValue().([]interface{})
			// Extract the start, end and step values
			start := 0
			if node.startSpecified {
				start = node.start
			}
			if start < 0 {
				start += len(value)
			}
			end := len(value)
			if node.endSpecified {
				end = node.end
			}
			if end < 0 {
				end += len(value)
			}

			// Clamp start/end to edges of array
			if start < 0 {
				start = 0
			}
			if end > len(value) {
				end = len(value)
			}

			step := 1
			if node.stepSpecified {
				step = node.step
				if step == 0 {
					// She didn't mean that, right? We'll try to make sense of it
					if end != start {
						end = start + 1
					}
					step = 1
				}
			}

			if step == 1 {
				// This is a continuous range e.g. "4:7" "5:"
				result = result.Union(NewArrayRef(varRef, Regions{Region{start, end}}))
			} else {
				// This is discontinuous, or reverse, so need to make individual indicies
				if step < 0 {
					start, end = end, start
				}
				// Build array of indicies
				indicies := make([]int, 0, end-start)
				for i := start; i < end; i += step {
					indicies = append(indicies, i)
				}
				result = result.Union(NewArrayRef(varRef, NewRegionForEachIndex(indicies)))
			}
		}
	}
	return result, nil
}

func processIndex(input Ref, node *indexNode) (Ref, error) {
	result := NewEmptyRef()
	for _, varRef := range input.Vars() {
		if varRef.IsSlice() {
			value := varRef.CanonicalValue().([]interface{})
			index := node.value
			if len(value) == 0 && (node.value == 0 || node.value == -1) {
				// Special handling of index 0 (start of array) and -1 (end of array)
				// when length is 0. We return a zero length selection in order to support
				// seamless appending/prepending to arrays even when they are empty.
				result = result.Union(NewArrayRef(varRef, Regions{Region{0, 0}}))
			} else {
				if index < 0 {
					index += len(value)
				}
				// ignore indicies outside the range of the array
				if index >= 0 && index < len(value) {
					result = result.Union(NewArrayRef(varRef, Regions{Region{index, index + 1}}))
				}
			}
		}
	}
	return result, nil
}

func processWildcard(input Ref, node *wildcardNode) (Ref, error) {
	result := NewEmptyRef()
	for _, varRef := range input.Vars() {
		result = result.Union(matchAllChildren(varRef))
	}
	return result, nil
}

func processRecursive(input Ref, node *recursiveNode) (Ref, error) {
	result := input
	for _, varRef := range input.Vars() {
		children := matchAllChildren(varRef)
		descendants, err := processRecursive(children, node)
		if err != nil {
			return nil, err
		}
		result = result.Union(descendants)
	}
	return result, nil
}

func processUnion(input Ref, n *unionNode) (Ref, error) {
	result := NewEmptyRef()
	for _, pathNode := range n.nodes {
		subset, err := process(input, pathNode)
		if err != nil {
			return nil, err
		}
		result = result.Union(subset)
	}
	return result, nil
}

func coerceComparisionValue(value interface{}) interface{} {
	if floatValue, wasNumber := floatFromValue(value); wasNumber {
		return floatValue
	}
	return value
}

// Applies the filter to the lhs deferenced value supplied
func applyFilter(lhs Ref, rhs Ref, node *filterNode) (bool, error) {
	if lhs.IsEmpty() {
		return false, nil
	}
	lhsValues := lhs.Values()
	if len(lhsValues) == 0 {
		return false, nil
	}
	if node.operator == Exists {
		return true, nil
	}
	rhsValues := rhs.Values()
	if len(rhsValues) == 0 {
		return false, nil
	}

	// Add any operators that would work on collectios here

	if len(lhsValues) > 1 || len(rhsValues) > 1 {
		// None of the following operators are valid for collections
		return false, nil
	}
	left := coerceComparisionValue(lhsValues[0])
	right := coerceComparisionValue(rhsValues[0])
	var err error
	var result bool
	switch node.operator {
	case LT:
		result, err = template.Less(left, right)
	case GT:
		result, err = template.Greater(left, right)
	case Equals:
		result, err = template.Equal(left, right)
	case NEQ:
		result, err = template.NotEqual(left, right)
	case LTE:
		result, err = template.LessEqual(left, right)
	case GTE:
		result, err = template.GreaterEqual(left, right)
	default:
		return false, fmt.Errorf("unrecognized filter operator %s", node.operator)
	}
	if err == template.ErrBadComparisonType || err == template.ErrBadComparison {
		return false, nil
	}
	return result, err
}

func processFilter(input Ref, node *filterNode) (Ref, error) {
	result := NewEmptyRef()
	// Now go through each entry in the result and check conditions
	for _, varRef := range input.Vars() {
		if varRef.IsMap() {
			mapRef := matchAllChildren(varRef).(*MapRef)
			matches := make([]string, 0, len(mapRef.keys))
			for _, key := range mapRef.keys {
				// Get the lhs for this key
				lhs, err := process(NewMapRef(mapRef.variable, []string{key}), node.lhs)
				if err != nil {
					return nil, err
				}
				var rhs Ref
				// unary operators have no rhs
				if node.rhs != nil {
					rhs, err = process(NewMapRef(mapRef.variable, []string{key}), node.rhs)
					if err != nil {
						return nil, err
					}
				}
				isMatch, err := applyFilter(lhs, rhs, node)
				if err != nil {
					return nil, err
				}
				if isMatch {
					matches = append(matches, key)
				}
			}
			result = result.Union(NewMapRef(mapRef.variable, matches))
		} else if varRef.IsSlice() {
			arrayRef := matchAllChildren(varRef).(*ArrayRef)
			matches := make([]int, 0, arrayRef.EstimateSize())
			for _, index := range arrayRef.selection.ToIndicies() {
				// Get the lhs for this key
				lhs, err := process(NewArrayRef(arrayRef.variable, NewRegionForEachIndex([]int{index})), node.lhs)
				if err != nil {
					return nil, err
				}
				var rhs Ref
				// unary operators have no rhs
				if node.rhs != nil {
					rhs, err = process(NewArrayRef(arrayRef.variable, NewRegionForEachIndex([]int{index})), node.rhs)
					if err != nil {
						return nil, err
					}
				}
				isMatch, err := applyFilter(lhs, rhs, node)
				if err != nil {
					return nil, err
				}
				if isMatch {
					matches = append(matches, index)
				}
			}
			result = result.Union(NewArrayRef(arrayRef.variable, NewRegionForEachIndex(matches)))
		}
	}
	return result, nil
}
