package jsonmatch

import (
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strings"
)

// MutatorFunc is the signature for the callbacks provided to the Ref.Mutate methods
type MutatorFunc func(path string, value interface{}) (interface{}, error)

// MutateRegionsFunc is the signature for the callbacks provided to the ArrayRef.MutateAll
// methods that allow a selection of an array to be mutated as a whole. The selection is
// provided as an array of arrays (since selections might not be contiguous). Each sub-array
// representing a contiguous block of selected items. These may be modified directly by
// the mutator, or replaced with new ones.
type MutateRegionsFunc func(path string, current [][]interface{}) ([][]interface{}, error)

// A Ref is a reference to a selection of values, typically in a struct, map or an array that
// allow operations to be performed on said values. Main use case is to represent data that
// have been matched using jsonmatch specs and then allow a client to perform operations on that subset.
type Ref interface {
	// Values returns the value of all referenced variables in their original form
	Values() []interface{}
	// Vars returns VarRefs for all the referenced variable
	Vars() []*VarRef
	// Delete deletes the values from the referenced variables if possible (basically maps and arrays). If
	// the refs cannot be deleted (structs or primitive variables) theres an error
	Delete() error
	// Mutate changes the value of the refs by passing them through a mutator function
	Mutate(mutator MutatorFunc) error
	// Set sets the referenced values to the provided value
	Set(value interface{}) error
	// Depth returns a depth value for this ref. When applying operations to multiple refs
	// they are always performed on the refs with the highest depth first.
	Depth() int
	// Join this ref with another ref yielding what amounts to a union of the two
	// refs
	Union(ref ...Ref) Ref
	// Merge attempts to combine two refs into
	// one ref without resorting to the UnionRef. This is used to combine i.e. two
	// ArrayRefs that refer to the same underlying slice by just merging their
	// respective indicies.
	Merge(ref Ref) (Ref, bool)
	// Returns true if this Ref is not referring to any elements. I.e. after a Delete
	// operation
	IsEmpty() bool
	// Give an estimate of the number of values this Ref points to. Used for sizing
	// arrays so should err on the high side if it needs to err at all.
	EstimateSize() int
}

// ArrayRef is a ref to a subset of a slice
type ArrayRef struct {
	// Must be a VarRef to an indexable Go type
	variable  *VarRef
	selection Regions
}

// MapRef is a ref to a subset of a map type where the keys are strings
type MapRef struct {
	variable *VarRef
	keys     []string
}

// UnionRef is the union of a collection of refs
type UnionRef struct {
	refs []Ref
}

// LiteralRef is not a Ref at all, but just a container for a literal value
type LiteralRef struct {
	value interface{}
}

// // ValuesRef is a special ref that refers to a set of values with
// // no subsetting.
// type ValuesRef struct {
// 	values []reflect.Value
// 	depth  int
// }

// VarSetter is the signature for VarRef setters
type VarSetter func(value interface{})

// VarGetter is the signature for VarRef getters
type VarGetter func() interface{}

// VarRef is a reference to a variable with the capacity to replace
// the contents of said variable
type VarRef struct {
	// Method for setting the variable
	setter VarSetter
	// Method for getting the current value
	getter VarGetter
	// Scalar value indicating at what nesting depth the variable is from relative to root
	depth int
	// A unique name identifying the variable in the structure, in reality a jsonmatch string
	// locating exactly this value
	identity string
	// The key of this value in the event that this is a variable from an array or map
	key interface{}
}

// LatentMapRef is a reference to a keypath in a map that do not exist yet
// Given an object like {a: {}} and you match using the jsonmatch "a.b.c",
// the void map ref will refer to `a.b` as the root, (since b is a key)
// in a map that exists, but `c` has nowhere to go so a void map ref
// is formed in case the client wants to set that value. In that case
// the necessary maps will be created to realize the path. Every root
// ref in a VoidMapRef must be a MapRef. The LatentMapRef will report
// referring to absolutely no values, so cannot be mutated, only Set.
type LatentMapRef struct {
	root    Ref
	keyPath []string
}

// Unwraps a ref into an array of refs
func individualRefs(ref Ref) []Ref {
	var refs []Ref
	switch t := ref.(type) {
	case *UnionRef:
		refs = t.refs
	default:
		refs = []Ref{t}
	}
	return refs
}

// latentMapRefs extracts the latent map refs from the provided ref
// either creating them from maps containing "missing" keys, or passing
// though any pre-existing LatentMapRefs.
func latentMapRefs(ref Ref) []*LatentMapRef {
	result := []*LatentMapRef{}
	for _, r := range individualRefs(ref) {
		switch t := r.(type) {
		case *MapRef:
			latent := t.GetLatentMapRef()
			if latent != nil {
				result = append(result, latent)
			}
		case *LatentMapRef:
			result = append(result, t)
		}
	}
	return result
}

// NewEmptyRef returns an empty Ref
func NewEmptyRef() Ref {
	return &UnionRef{[]Ref{}}
}

// GetLatentMapRef generates a latent map ref for any keys that are missing
// or not Maps in the underlying value. If no such values, nil is returned
func (r *MapRef) GetLatentMapRef() *LatentMapRef {
	if value := r.variable.CanonicalValue().(map[string]interface{}); value != nil {
		var missingKeys []string
		for _, key := range r.keys {
			content, keyPresent := value[key]
			if !keyPresent || content == nil || reflect.TypeOf(content).Kind() != reflect.Map {
				missingKeys = append(missingKeys, key)
			}
		}
		if len(missingKeys) > 0 {
			return NewLatentMapRef(NewMapRef(r.variable, missingKeys), []string{})
		}
	}
	return nil
}

// NewArrayRef creates a new ArrayRef
func NewArrayRef(variable *VarRef, selection Regions) *ArrayRef {
	if _, ok := variable.CanonicalValue().([]interface{}); !ok {
		panic("Variables wrapped by ArrayRefs must be compatible with type []interface{}")
	}
	return &ArrayRef{
		variable:  variable,
		selection: selection,
	}
}

// Values implements the Values method
func (r *ArrayRef) Values() []interface{} {
	indicies := r.selection.ToIndicies()
	result := make([]interface{}, len(indicies))
	array := r.variable.CanonicalValue().([]interface{})
	for i := range indicies {
		result[i] = array[indicies[i]]
	}
	return result
}

// Vars returns VarRefs for all the referenced variable
func (r *ArrayRef) Vars() []*VarRef {
	indicies := r.selection.ToIndicies()
	result := make([]*VarRef, len(indicies))
	for i := range indicies {
		index := indicies[i]
		identity := fmt.Sprintf("%s[%06d]", r.variable.identity, index)
		getter := func() interface{} {
			return r.variable.CanonicalValue().([]interface{})[index]
		}
		setter := func(value interface{}) { // Setter
			// Clone the array, insert the new value and update the
			// value of our array
			current := r.variable.CanonicalValue().([]interface{})
			mutated := make([]interface{}, len(current))
			copy(mutated, current)
			mutated[index] = value
			if err := r.variable.Set(mutated); err != nil {
				// FIXME: Return error
				panic(err)
			}
		}
		result[i] = &VarRef{
			identity: identity,
			setter:   setter,
			getter:   getter,
			depth:    r.Depth() + 1,
			key:      i,
		}
	}
	return result
}

// indexIncluded Returns true if the provided index is in the ref set
func (r *ArrayRef) indexIncluded(index int) bool {
	return r.selection.ContainsIndex(index)
}

// Delete implements the Ref.Delete method
func (r *ArrayRef) Delete() error {
	current := r.variable.CanonicalValue().([]interface{})
	regionsToKeep := Regions{Region{0, len(current)}}.Intersect(r.selection)
	capacity := regionsToKeep.IndiciesCount()
	modified := make([]interface{}, 0, capacity)
	for _, region := range regionsToKeep {
		for i := region.Start; i < region.End; i++ {
			modified = append(modified, current[i])
		}
	}
	// Replace the value with the subset effectively deleting the indicies pointed to
	// by this ref
	if err := r.variable.SetWithMatchedType(modified); err != nil {
		return err
	}
	// Clear indicies set since they are now gone from the underlying set
	r.selection = Regions{}
	return nil
}

// Mutate implements the Ref.Mutate method
func (r *ArrayRef) Mutate(mutator MutatorFunc) error {
	current := r.variable.CanonicalValue().([]interface{})
	modified := make([]interface{}, len(current))
	copy(modified, current)
	for _, region := range r.selection {
		for i := region.Start; i < region.End; i++ {
			if r.indexIncluded(i) {
				newValue, err := mutator(fmt.Sprintf("%s[%06d]", r.variable.identity, i), current[i])
				if err != nil {
					return err
				}
				modified[i] = newValue
			}
		}
	}
	return r.variable.SetWithMatchedType(modified)
}

// MutateRegions lets you mutate the parts of an array as a whole. The mutator
// gets an array of arrays corresponding to all contiguous items selected. It returns
// an array of arrays where each sub array will be substituted for each corresponding
// region in the source. The length of each region may change as required and the
// length of the result will be adjusted accordingly. Also the selected indicies
// of the ArrayRef are updated to reflect the new positions so this method may be
// combined with other mutations afterwards.
func (r *ArrayRef) MutateRegions(mutator MutateRegionsFunc) error {
	original := r.variable.CanonicalValue().([]interface{})
	extract := r.selection.ExtractItems(original)
	modifiedExtract, err := mutator(
		fmt.Sprintf("%s[%s]", r.variable.identity, r.selection.ToSliceSelector()), extract)
	if err != nil {
		return err
	}
	modified, updatedRegions := r.selection.MergeItems(original, modifiedExtract)
	if e := r.variable.SetWithMatchedType(modified); e != nil {
		return e
	}

	// Update the positions of the matched indicies
	r.selection = updatedRegions

	return nil
}

// Set implements the Ref.Set method
func (r *ArrayRef) Set(value interface{}) error {
	current := r.variable.CanonicalValue().([]interface{})
	modified := make([]interface{}, len(current))
	copy(modified, current)
	for i := 0; i < len(current); i++ {
		if r.indexIncluded(i) {
			modified[i] = value
		}
	}
	return r.variable.SetWithMatchedType(modified)
}

// Depth implements the Ref.Depth method
func (r *ArrayRef) Depth() int {
	return r.variable.depth + 1
}

// IsEmpty implements the Ref.IsEmpty method
func (r *ArrayRef) IsEmpty() bool {
	return r.selection.IndiciesCount() == 0
}

// EstimateSize implements the Ref.EstimateSize method
func (r *ArrayRef) EstimateSize() int {
	return r.selection.IndiciesCount()
}

// Merge implements Ref.Merge
func (r *ArrayRef) Merge(ref Ref) (Ref, bool) {
	if other, ok := ref.(*ArrayRef); ok {
		if r.variable.identity == other.variable.identity {
			// Can be merged!
			return &ArrayRef{
				variable:  r.variable,
				selection: r.selection.Union(other.selection),
			}, true
		}
	}
	// Not same kind of ref or different underlying array
	return nil, false
}

// Union implements the Ref.Union method
func (r *ArrayRef) Union(refs ...Ref) Ref {
	return NewUnionRef(append(refs, r)...)
}

// NewMapRef creates a new MapRef
func NewMapRef(variable *VarRef, keys []string) *MapRef {
	if _, ok := variable.CanonicalValue().(map[string]interface{}); !ok {
		panic("Variables wrapped by MapRefs must be of type map[string]interface{}")
	}
	sort.Strings(keys)
	return &MapRef{
		variable: variable,
		keys:     keys,
	}
}

// Values implements the Ref.Values method
func (r *MapRef) Values() []interface{} {
	current := r.variable.CanonicalValue().(map[string]interface{})
	result := make([]interface{}, 0, len(r.keys))
	for i := range r.keys {
		val := current[r.keys[i]]
		if val != nil {
			result = append(result, val)
		}
	}
	return result
}

func (r *MapRef) cloneMap() map[string]interface{} {
	result := make(map[string]interface{})
	current := r.variable.CanonicalValue().(map[string]interface{})
	for k, v := range current {
		result[k] = v
	}
	return result
}

// Vars returns VarRefs for all the referenced variable
func (r *MapRef) Vars() []*VarRef {
	result := make([]*VarRef, len(r.keys))
	for i := range r.keys {
		key := r.keys[i]
		identity := fmt.Sprintf("%s.%s", r.variable.identity, key)
		getter := func() interface{} { // getter
			return r.variable.CanonicalValue().(map[string]interface{})[key]
		}
		setter := func(value interface{}) { // setter
			// Make a shallow copy of the contained map, then replace the value
			modified := r.cloneMap()
			modified[key] = value
			if err := r.variable.Set(modified); err != nil {
				// FIXME: Return error
				panic(err)
			}
		}
		result[i] = &VarRef{
			identity: identity,
			setter:   setter,
			getter:   getter,
			depth:    r.Depth() + 1,
			key:      key,
		}
	}
	return result
}

// Mutate implements the Ref.Mutate method
func (r *MapRef) Mutate(mutator MutatorFunc) error {
	current := r.variable.CanonicalValue().(map[string]interface{})
	modified := r.cloneMap()
	for _, key := range r.keys {
		newValue, err := mutator(fmt.Sprintf("%s.%s", r.variable.identity, key), current[key])
		if err != nil {
			return err
		}
		modified[key] = newValue
	}
	return r.variable.SetWithMatchedType(modified)
}

// Set implements the Ref.Set method
func (r *MapRef) Set(value interface{}) error {
	modified := r.cloneMap()
	for _, key := range r.keys {
		modified[key] = value
	}
	return r.variable.SetWithMatchedType(modified)
}

// Delete implements the Ref.Delete method
func (r *MapRef) Delete() error {
	modified := r.cloneMap()
	for _, key := range r.keys {
		delete(modified, key)
	}
	return r.variable.SetWithMatchedType(modified)
}

// Depth implements the Ref.Depth method
func (r *MapRef) Depth() int {
	return r.variable.depth + 1
}

// EstimateSize implements the Ref.EstimateSize method
func (r *MapRef) EstimateSize() int {
	return len(r.keys)
}

// IsEmpty implements the Ref.IsEmpty method
func (r *MapRef) IsEmpty() bool {
	return len(r.keys) == 0
}

// Merge implements Ref.Merge
func (r *MapRef) Merge(ref Ref) (Ref, bool) {
	if other, ok := ref.(*MapRef); ok {
		if other.variable.identity == r.variable.identity {
			// Can be merged!
			return &MapRef{
				variable: r.variable,
				keys:     unionKeys(r.keys, other.keys),
			}, true
		}
	}
	// Not same kind of ref, or not same underlying map
	return nil, false
}

// Union implements the Ref.Union method
func (r *MapRef) Union(refs ...Ref) Ref {
	return NewUnionRef(append(refs, r)...)
}

// NewUnionRef creates a new UnionRef
func NewUnionRef(refs ...Ref) Ref {
	return (&UnionRef{}).Union(refs...)
}

// Delete implements the Ref.Delete method
func (r *UnionRef) Delete() error {
	// TODO: Sort refs by depth, deepest first before deleting
	for _, ref := range r.refs {
		err := ref.Delete()
		if err != nil {
			return err
		}
	}
	return nil
}

// Mutate implements the Ref.Mutate method
func (r *UnionRef) Mutate(mutator MutatorFunc) error {
	// TODO: Sort refs by depth, deepest first before mutating
	for _, ref := range r.refs {
		err := ref.Mutate(mutator)
		if err != nil {
			return err
		}
	}
	return nil
}

// Set implements the Ref.Set method
func (r *UnionRef) Set(value interface{}) error {
	for _, ref := range r.refs {
		err := ref.Set(value)
		if err != nil {
			return err
		}
	}
	return nil
}

// Depth implements the Ref.Depth methd
func (r *UnionRef) Depth() int {
	return 0
}

// EstimateSize implements the Ref.EstimateSize method
func (r *UnionRef) EstimateSize() int {
	result := 0
	for _, r := range r.refs {
		result += r.EstimateSize()
	}
	return result
}

// IsEmpty implements the Ref.IsEmpty method
func (r *UnionRef) IsEmpty() bool {
	for _, ref := range r.refs {
		if !ref.IsEmpty() {
			return false
		}
	}
	return true
}

// Values implements the Ref.Values method
func (r *UnionRef) Values() []interface{} {
	result := make([]interface{}, 0, r.EstimateSize())
	for _, ref := range r.refs {
		result = append(result, ref.Values()...)
	}
	return result
}

// Vars returns VarRefs for all the referenced variable
func (r *UnionRef) Vars() []*VarRef {
	result := make([]*VarRef, 0, r.EstimateSize())
	for _, ref := range r.refs {
		result = append(result, ref.Vars()...)
	}
	return result
}

// Union implements the Ref.Union method
func (r *UnionRef) Union(refs ...Ref) Ref {
	// TODO: If two underlying refs are referencing the same base object, join them
	// by substituting both with one ref where the selection is the union of the two
	var result Ref = r
	for _, ref := range refs {
		switch t := ref.(type) {
		case *UnionRef:
			// Unwrap secondary unions in order to avoid recursion
			result = result.Union(t.refs...)
		default:
			// First see if this can be merged into an existing ref
			if merged, didMerge := result.Merge(ref); didMerge {
				result = merged
			} else {
				// Resort to adding it to the end of the existing union, if no merge
				// was possible
				result = &UnionRef{refs: append(result.(*UnionRef).refs, ref)}
			}
		}
	}

	if union, ok := result.(*UnionRef); ok {
		// Unwrap unions with only one member, no reason to keep the UnionRef wrapper
		if len(union.refs) == 1 {
			return union.refs[0]
		}
		// Call sort to make sure the union is sorted depth first at all times
		sort.Sort(union)
	} else {
		panic("BUG: Result of a union with more than one member must be *UnionRef")
	}

	return result
}

func (r *UnionRef) Len() int {
	return len(r.refs)
}

func (r *UnionRef) Less(i, j int) bool {
	if r.refs[i].Depth() == r.refs[j].Depth() {
		a, aOk := r.refs[i].(PathedRef)
		b, bOk := r.refs[j].(PathedRef)
		if aOk && !bOk {
			return true
		}
		if aOk && bOk {
			return strings.Compare(a.GetPath(), b.GetPath()) < 0
		}
	}
	return r.refs[i].Depth() > r.refs[j].Depth()
}

func (r *UnionRef) Swap(i, j int) {
	hold := r.refs[i]
	r.refs[i] = r.refs[j]
	r.refs[j] = hold
}

// Merge implements the Ref.Merge method
func (r *UnionRef) Merge(ref Ref) (Ref, bool) {
	if _, ok := ref.(*UnionRef); ok {
		// Never able to merge one union with another union, for that there is Union()
		return nil, false
	}
	// Tries to merge the new ref into one of the existing refs in the union
	for i, old := range r.refs {
		if merged, ok := old.Merge(ref); ok {
			// It worked!
			result := make([]Ref, len(r.refs))
			copy(result, r.refs)
			result[i] = merged
			return &UnionRef{refs: result}, true
		}
	}
	return r, false
}

// NewVarRef has a complicated signature and should not be used except in tests
func NewVarRef(identity string, getter VarGetter, setter VarSetter, depth int) *VarRef {
	return &VarRef{
		identity: identity,
		setter:   setter,
		getter:   getter,
		depth:    depth,
	}
}

// Values gets the value wrapped in an array for compatibility
func (r *VarRef) Values() []interface{} {
	return []interface{}{r.Value()}
}

// Vars gets this VarRef wrapped in an array for your iteration convenience
func (r *VarRef) Vars() []*VarRef {
	return []*VarRef{r}
}

// Set sets the value of the contained variable
func (r *VarRef) Set(value interface{}) error {
	err := assertIsCompatible(value)
	if err != nil {
		return err
	}
	r.setter(value)
	return nil
}

// SetWithMatchedType updates the value, but attempts to avoid changing
// the underlying type if the new value is one of the canonical types
func (r *VarRef) SetWithMatchedType(value interface{}) error {
	return r.Set(matchType(value, r.Value()))
}

// CanonicalValue gets the current value of the variable converted to canonical type
func (r *VarRef) CanonicalValue() interface{} {
	canonical, _, err := toCanonicalType(r.getter())
	if err != nil {
		panic(err) // TODO: Maybe not panic here
	}
	return canonical
}

// Value gets the current value of the variable in original underlying type
func (r *VarRef) Value() interface{} {
	return r.getter()
}

// Mutate mutates
func (r *VarRef) Mutate(mutator MutatorFunc) error {
	newValue, err := mutator(r.identity, r.getter())
	if err != nil {
		return err
	}
	r.setter(newValue)
	return nil
}

// Delete is not supported for VarRefs
func (r *VarRef) Delete() error {
	return errors.New("Delete not supported for VarRefs")
}

// IsEmpty is never true for VarRefs
func (r *VarRef) IsEmpty() bool {
	return false
}

// EstimateSize is always 1 for VarRefs
func (r *VarRef) EstimateSize() int {
	return 1
}

// Merge only "merges" two identical VarRefs
func (r *VarRef) Merge(ref Ref) (Ref, bool) {
	if varRef, ok := ref.(*VarRef); ok {
		if varRef.identity == r.identity {
			return r, true
		}
	}
	return nil, false
}

// Union creates a union with the VarRef
func (r *VarRef) Union(refs ...Ref) Ref {
	return NewUnionRef(append(refs, r)...)
}

// Depth returns the depth of the variable
func (r *VarRef) Depth() int {
	return r.depth
}

// TypeKind returns the kind of the type contained by the VarRef
func (r *VarRef) TypeKind() reflect.Kind {
	value := r.getter()
	if value == nil {
		return reflect.Invalid
	}
	return reflect.TypeOf(value).Kind()
}

// IsMap is true if the underlying value is a kind of map.
func (r *VarRef) IsMap() bool {
	return r.TypeKind() == reflect.Map
}

// IsSlice is true if the underlying value is a kind of slice or array.
func (r *VarRef) IsSlice() bool {
	switch r.TypeKind() {
	case reflect.Slice, reflect.Array:
		return true
	}
	return false
}

// IsContainer is true if this VarRef wraps a map or slice or array.
func (r *VarRef) IsContainer() bool {
	switch r.TypeKind() {
	case reflect.Slice, reflect.Array, reflect.Map:
		return true
	}
	return false
}

// Index returns the int-index of this VarRef if it is originally from a slice
func (r *VarRef) Index() int {
	return r.key.(int)
}

// Key returns the key of this VarRef if it was originally from a map
func (r *VarRef) Key() string {
	return r.key.(string)
}

// NewLiteralRef returns a new *LiteralRef
func NewLiteralRef(value interface{}) Ref {
	return &LiteralRef{value}
}

// Values gets the value wrapped in an array for compatibility
func (r *LiteralRef) Values() []interface{} {
	return []interface{}{r.value}
}

// Vars gets this VarRef wrapped in an array for your iteration convenience
func (r *LiteralRef) Vars() []*VarRef {
	getter := func() interface{} {
		return r.value
	}
	setter := func(v interface{}) {
		panic("Attempt to set value of LiteralRef")
	}
	return []*VarRef{NewVarRef("[literal]", getter, setter, -1)}
}

// Set sets the value of the contained variable
func (r *LiteralRef) Set(value interface{}) error {
	panic("Attempt to set value of LiteralRef")
}

// SetWithMatchedType updates the value, but attempts to avoid changing
// the underlying type if the new value is one of the canonical types
func (r *LiteralRef) SetWithMatchedType(value interface{}) error {
	panic("Attempt to set value of LiteralRef")
}

// CanonicalValue gets the current value of the variable converted to canonical type
func (r *LiteralRef) CanonicalValue() interface{} {
	return r.value
}

// Value gets the current value of the variable in original underlying type
func (r *LiteralRef) Value() interface{} {
	return r.value
}

// Mutate mutates
func (r *LiteralRef) Mutate(mutator MutatorFunc) error {
	panic("Attempt to mutate value of LiteralRef")
}

// Delete is not supported for VarRefs
func (r *LiteralRef) Delete() error {
	panic("Attempt to delete LiteralRef")
}

// IsEmpty is never true for VarRefs
func (r *LiteralRef) IsEmpty() bool {
	return false
}

// EstimateSize is always 1 for VarRefs
func (r *LiteralRef) EstimateSize() int {
	return 1
}

// Merge only "merges" two identical VarRefs
func (r *LiteralRef) Merge(ref Ref) (Ref, bool) {
	if literalRef, ok := ref.(*LiteralRef); ok {
		if r.value == literalRef.value {
			return r, true
		}
	}
	return nil, false
}

// Union creates a union with the VarRef
func (r *LiteralRef) Union(refs ...Ref) Ref {
	return NewUnionRef(append(refs, r)...)
}

// Depth returns the depth of the variable
func (r *LiteralRef) Depth() int {
	return -1
}

// NewLatentMapRef returns a new *LatentMapRef
func NewLatentMapRef(rootRef Ref, keyPath []string) *LatentMapRef {
	return &LatentMapRef{rootRef, keyPath}
}

// Values gets the value wrapped in an array for compatibility
func (r *LatentMapRef) Values() []interface{} {
	return []interface{}{}
}

// Vars gets this VarRef wrapped in an array for your iteration convenience
func (r *LatentMapRef) Vars() []*VarRef {
	return []*VarRef{}
}

// Wraps the value in onionskins of maps according to the keypath. I.e. if the
// value is 4 and the keypath is ['a', 'b'] the result is {"a": {"b": 4}}
func buildMapBabushka(keyPath []string, value interface{}) map[string]interface{} {
	result := map[string]interface{}{}
	current := result
	for i, key := range keyPath {
		if i < len(keyPath)-1 {
			next := map[string]interface{}{}
			current[key] = next
			current = next
		} else {
			// last element
			current[key] = value
		}
	}
	return result
}

// Set sets the value of the contained variable
func (r *LatentMapRef) Set(value interface{}) error {
	for _, ref := range individualRefs(r.root) {
		if err := ref.Set(buildMapBabushka(r.keyPath, value)); err != nil {
			return err
		}
	}
	return nil
}

// SetWithMatchedType updates the value, but attempts to avoid changing
// the underlying type if the new value is one of the canonical types
func (r *LatentMapRef) SetWithMatchedType(value interface{}) error {
	// By definition there is no underlying type to refer to for a LatentMapRef,
	// so just call Set
	return r.Set(value)
}

// Mutate mutates values that by definition is non existant. The mutator will
// recieve the initial value nil
func (r *LatentMapRef) Mutate(mutator MutatorFunc) error {
	newPathIdentity := strings.Join(r.keyPath, ".")
	for _, ref := range individualRefs(r.root) {
		baseIdentity := ref.(*MapRef).variable.identity
		identity := fmt.Sprintf("%s.*.%s", baseIdentity, newPathIdentity)
		value, err := mutator(identity, nil)
		if err != nil {
			return err
		}
		err = ref.Set(buildMapBabushka(r.keyPath, value))
		if err != nil {
			return err
		}
	}
	return nil
}

// Delete is a noop for LatentMapRef
func (r *LatentMapRef) Delete() error {
	return nil
}

// IsEmpty is always true for a LatentMapRef
func (r *LatentMapRef) IsEmpty() bool {
	return true
}

// EstimateSize is always 0 for LatentMapRefs
func (r *LatentMapRef) EstimateSize() int {
	return 0
}

// Merge only merges with itself
func (r *LatentMapRef) Merge(ref Ref) (Ref, bool) {
	return nil, ref == r
}

// Union creates a union
func (r *LatentMapRef) Union(refs ...Ref) Ref {
	return NewUnionRef(append(refs, r)...)
}

// Depth returns the depth of the variable. Since a Set for this should
// always execute first, it pretends to be veeery deep.
func (r *LatentMapRef) Depth() int {
	return 100000
}

// AddKey adds a key to the keypath of the LatentMapRef
func (r *LatentMapRef) AddKey(key string) {
	r.keyPath = append(r.keyPath, key)
}

// PathedRef is an interface for refs that have a definitive, resolvable paths, which is effectively Array and Map ref
type PathedRef interface {
	GetPath() string
}

func (r *MapRef) GetPath() string {
	return r.variable.identity
}

func (r *ArrayRef) GetPath() string {
	return r.variable.identity
}

func (r *VarRef) GetPath() string {
	return r.identity
}
