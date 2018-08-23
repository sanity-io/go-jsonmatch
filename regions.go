// Regions model areas of slices and their behavior as items are deleted or inserted
// into the array. The main use case is to support splice-style mutations. See
// MatchSet.MutateRegions.

package jsonmatch

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// Region represents a region of a slice and support operations that model
// the behavior of a region of array indicies as an array is being manipulated
type Region struct {
	Start int
	End   int
}

// Len returns the number of items in the region
func (r Region) Len() int {
	return r.End - r.Start
}

// Empty is true if the region cover no items
func (r Region) Empty() bool {
	return r.Len() == 0
}

// ContainsIndex is true if the index provided is covered by the region
func (r Region) ContainsIndex(index int) bool {
	return index >= r.Start && index < r.End
}

// IsBefore is true if the index is before the start of the region
func (r Region) IsBefore(index int) bool {
	return index < r.Start
}

// IsAfter is true if the index is after the end of the region
func (r Region) IsAfter(index int) bool {
	return r.Start >= index
}

// Clone returns a fresh copy of the region
func (r Region) Clone() Region {
	return Region{
		Start: r.Start,
		End:   r.End,
	}
}

// Grow extends the region by growing it to the right
func (r Region) Grow(count int) Region {
	return Region{
		Start: r.Start,
		End:   r.End + count,
	}
}

// Adjacent returns true if the regions are exactly adjacent in either order
func (r Region) Adjacent(other Region) bool {
	return r.End == other.Start || other.End == r.Start
}

// ContainsRegion returns true if the reciever covers the entire other region (or more)
func (r Region) ContainsRegion(other Region) bool {
	return other.Start >= r.Start && other.End <= r.End
}

// Overlap is true if the two regions cover at least one shared element
func (r Region) Overlap(other Region) bool {
	return (other.Start >= r.Start && other.Start < r.End) ||
		(other.End > r.Start && other.End < r.End) ||
		(other.Start < r.Start && other.End > r.Start) ||
		(r.Start < other.Start && r.End > other.Start)
}

// Equal is true if the two regions are exactly the same
func (r Region) Equal(other Region) bool {
	return r.Start == other.Start && r.End == other.End
}

// OverlapOrAdjacent is true if the two regions are adjacent or overlapping
func (r Region) OverlapOrAdjacent(other Region) bool {
	return r.Adjacent(other) || r.Overlap(other)
}

// Join returns a range that cover the entire area of both regions. If the regions
// are not adjacent, the resulting region will also span the void between them
func (r Region) Join(other Region) Region {
	start := other.Start
	if r.Start < other.Start {
		start = r.Start
	}
	end := other.End
	if r.End > other.End {
		end = r.End
	}
	return Region{
		Start: start,
		End:   end,
	}
}

// Shrink region, but never smaller than nothing
func (r Region) Shrink(count int) Region {
	if count >= r.Len() {
		return Region{
			Start: r.Start,
			End:   r.Start,
		}
	}
	return Region{
		Start: r.Start,
		End:   r.End - count,
	}
}

// Shift moves the entire region by the numbre of indicies indicated
func (r Region) Shift(diff int) Region {
	return Region{
		Start: r.Start + diff,
		End:   r.End + diff,
	}
}

// Cut returns the new position and size of this region as if the provided region was
// cut from the underlying array.
func (r Region) Cut(other Region) Region {
	if other.End < r.Start {
		// Cut from start of region, this moves down
		return r.Shift(-other.Len())
	} else if other.Start >= r.End {
		// Other is after this region, so no change
		return r
	}
	if other.Start >= r.Start && other.End >= r.End {
		return Region{
			Start: r.Start,
			End:   other.Start,
		}
	}
	if other.Start <= r.Start && other.End > r.End {
		return Region{
			Start: other.Start,
			End:   other.Start,
		}
	}
	if other.Start <= r.Start && other.End > r.Start {
		return Region{
			Start: other.Start,
			End:   other.Start + r.Len() - (other.End - r.Start),
		}
	}
	// Region is fully inside this region, so it just shrinks
	return r.Shrink(other.Len())
}

// Intersect returns the intersection of the two regions. Removing a chunk from the middle
// of the receiver splits the region, so since this may result in two or no regions at all
// the result is a region set
func (r Region) Intersect(other Region) Regions {
	if other.ContainsRegion(r) {
		// Completely obliterated
		return Regions{}
	}
	if !r.Overlap(other) {
		// The other region is completely outside the reciever, so no cutting
		return Regions{r}
	}
	if other.Start <= r.Start {
		// The cut is from the left
		return Regions{Region{
			Start: other.End,
			End:   r.End,
		}}
	}
	if other.End >= r.End {
		// The cut is from the right
		return Regions{Region{
			Start: r.Start,
			End:   other.Start,
		}}
	}
	// The cut is in the middle
	return Regions{
		Region{r.Start, other.Start},
		Region{other.End, r.End},
	}
}

// Regions model a set of Regions as in ranges of array indicies and model the
// movement of these ranges as the underlying array is being manipulated.
type Regions []Region

// InsertAt returns a new region set moving regions as if something was inserted
// at the specfied region. If the new region is inside an existing region, this region is
// grown accordingly, all regions after the index is moved up by count indicies. The
// provided region is not actually inserted into the region set.
func (rs Regions) InsertAt(other Region) Regions {
	result := make(Regions, 0, len(rs))
	if other.Len() < 0 {
		panic("Insert only makes sense when inserting a positive amount of items")
	}
	for _, r := range rs {
		if r.IsAfter(other.Start) {
			result = append(result, r.Shift(other.Len()))
		} else if r.ContainsIndex(other.Start) {
			result = append(result, r.Grow(other.Len()))
		} else {
			result = append(result, r)
		}
	}
	return result
}

// Cut one region from the other regions. Shrinking or moving down the regions
// correspondingly. The number of resulting regions is always the same, regions
// that end up being empty will just get a lengt of 0. Call Compact to have them removed,
// if you need it.
func (rs Regions) Cut(other Region) Regions {
	result := make(Regions, 0, len(rs))
	for _, r := range rs {
		result = append(result, r.Cut(other))
	}
	return result
}

// Compact removes any empty regions from the set
func (rs Regions) Compact() Regions {
	result := make(Regions, 0, len(rs))
	for _, r := range rs {
		if !r.Empty() {
			result = append(result, r)
		}
	}
	return result
}

// Simplify removes any empty regions and joins overlapping regions
func (rs Regions) Simplify() Regions {
	if !rs.CheckSorted() {
		panic("Simplify require that the regions are sorted")
	}
	result := make(Regions, 0, len(rs))
	for _, r := range rs.Compact() {
		if len(result) > 0 {
			if result[len(result)-1].Overlap(r) {
				// Replace last region in the result set with the union of this region and the last
				result[len(result)-1] = result[len(result)-1].Join(r)
				continue
			}
		}
		if r.Len() > 0 {
			result = append(result, r)
		}
	}
	return result
}

// Clean takes a jumble of unsorted, potentially overlapping set of regions
// and return a sorted set of non-overlapping regions without joining
// any regions, meaning we preserve any split-points and zero-length regions
// in the selection.
func (rs Regions) Clean() Regions {
	rs = rs.Sort()
	result := make(Regions, 0, len(rs))
	var next *Region
	for i, r := range rs {
		if next == nil {
			next = &rs[i]
			continue
		}
		if r == *next {
			// Duplicate, discard
			continue
		}
		if r.Start >= (*next).End {
			result = append(result, *next)
			next = &rs[i]
			continue
		}
		if r.End > (*next).End {
			// r overlapping next starting inside next, ending outside to the right
			result = append(result, Region{(*next).Start, r.Start}, Region{r.Start, (*next).End})
			next = &Region{(*next).End, r.End}
		} else {
			// r is inside next
			result = append(result, Region{(*next).Start, r.Start}, r)
			next = &Region{r.End, (*next).End}
		}
	}
	if next != nil {
		result = append(result, *next)
	}
	return result
}

// Union computes the union of two sets of Regions. Any overlapping regions are combined,
// while adjacent regions are kept as they are.
func (rs Regions) Union(otherRegions Regions) Regions {
	// Short circuit if any of the region sets are empty
	if len(rs) == 0 {
		return otherRegions
	}
	if len(otherRegions) == 0 {
		return rs
	}

	// The two sides of the union
	leftSet := rs
	rightSet := otherRegions

	result := make(Regions, 0, len(leftSet)+len(rightSet))

	// The region we are building, the next to be added to the result
	var next *Region

	for len(leftSet) > 0 || len(rightSet) > 0 {
		var left *Region
		var right *Region
		if len(leftSet) > 0 {
			left = &leftSet[0]
		} else {
			left = nil
		}
		if len(rightSet) > 0 {
			right = &rightSet[0]
		} else {
			right = nil
		}
		if next == nil {
			if left == nil {
				if right == nil {
					// We're done
					break
				}
				// If next was nil, and also left is nil, just add the rest of the rightSet
				result = append(result, rightSet...)
				break
			}
			if right == nil {
				// left is !nil, but next is nil, so just flush the leftSet
				result = append(result, leftSet...)
				break
			}
			if left.Start < right.Start {
				clone := (*left).Clone()
				next = &clone
				leftSet = leftSet[1:]
			} else {
				clone := (*right).Clone()
				next = &clone
				rightSet = rightSet[1:]
			}
			continue
		}
		// next is not nil, we need to see if we can build on it
		if left != nil {
			if (*next).Overlap(*left) {
				*next = (*next).Join(*left)
				leftSet = leftSet[1:]
				continue
			}
			// Make sure 0 length regions (that by definition never overlap) are not represented more than once
			if (*next).Equal(*left) {
				leftSet = leftSet[1:]
				continue
			}
		}
		if right != nil {
			if (*next).Overlap(*right) {
				*next = (*next).Join(*right)
				rightSet = rightSet[1:]
				continue
			}
			// Make sure 0 length regions (that by definition never overlap) are not represented more than once
			if (*next).Equal(*right) {
				rightSet = rightSet[1:]
				continue
			}
		}
		// next is not nil, and neither left nor right overlap next
		// time to add next to the result, and move on
		result = append(result, *next)
		next = nil
	}
	if next != nil {
		result = append(result, *next)
	}
	return result
}

func (rs Regions) Intersect(other Regions) Regions {
	// Short circuit for empty sets
	if len(other) == 0 || len(rs) == 0 {
		return rs
	}
	leftSet := rs
	rightSet := other
	result := make(Regions, 0, len(leftSet))

	var next *Region

	for len(leftSet) > 0 || len(rightSet) > 0 {
		var right *Region
		if len(leftSet) == 0 && next == nil {
			// When the left set is empty, and there is no residue in *next we're done
			break
		}
		if next == nil {
			// There is room in the next-buffer. Consume one from the leftSet and place it in the
			// hot set
			next = &leftSet[0]
			leftSet = leftSet[1:]
		}
		// Consume from the right set until we catch up with *next
		for len(rightSet) > 0 && rightSet[0].End <= (*next).Start {
			rightSet = rightSet[1:]
		}
		if len(rightSet) == 0 {
			// Nothing more to intersect away, just flush out the rest of the left set
			// and be done
			result = append(result, *next)
			next = nil
			result = append(result, leftSet...)
			break
		}
		right = &rightSet[0]
		if right.Overlap(*next) {
			intersection := (*next).Intersect(*right)
			if len(intersection) == 0 {
				// Next was just chomped up completely by the right
				next = nil
			} else if len(intersection) == 1 {
				// A clean cut leaving a single new region
				next = &intersection[0]
			} else { // len == 2
				// A cut from the middle of next leaving two regions. The leftmost we add to the result, while the
				// rightmost is or next up in the hotseat
				result = append(result, intersection[0])
				next = &intersection[1]
			}
		} else {
			result = append(result, *next)
			next = nil
		}
	}
	if next != nil {
		result = append(result, *next)
	}
	return result
}

// Sort the regions by the start index
func (rs Regions) Sort() Regions {
	result := make(Regions, len(rs))
	copy(result, rs)
	sort.Sort(result)
	return result
}

// Check validates that the regions set is fully compliant, meaning its sorted
// and non-overlapping.
func (rs Regions) Check() bool {
	watermark := 0
	for _, r := range rs {
		if r.Start < watermark {
			return false
		}
		watermark = r.End
	}
	return true
}

// CheckSorted validates that the regions are sorted
func (rs Regions) CheckSorted() bool {
	watermark := 0
	for _, r := range rs {
		if r.Start < watermark {
			return false
		}
		watermark = r.Start
	}
	return true
}

// ToIndicies converts a set of regions to individual indicies
func (rs Regions) ToIndicies() []int {
	result := []int{}
	for _, r := range rs {
		for i := r.Start; i < r.End; i++ {
			result = append(result, i)
		}
	}
	return result
}

// ForEachIndexFunc is the type of the callback provided to ForEachIndex.
// The callback must return true as long as it wants the process to continue.
// If an error is returned, ForEachIndex returns with the error.
// When there are no more indicies, the process is of course terminated regardless.
type ForEachIndexFunc func(i int) (bool, error)

// ForEachIndex calls the callback for each index in the region set
func (rs Regions) ForEachIndex(callback ForEachIndexFunc) error {
	for _, r := range rs {
		for i := r.Start; i < r.End; i++ {
			keepGoing, err := callback(i)
			if err != nil {
				return err
			}
			if !keepGoing {
				return nil
			}
		}
	}
	return nil
}

// ContainsIndex checks if any of the regions in the set contains the index
func (rs Regions) ContainsIndex(i int) bool {
	for _, r := range rs {
		// Short circuit if Start is > i because regions are sorted
		if r.Start > i {
			return false
		}
		if r.ContainsIndex(i) {
			return true
		}
	}
	return false
}

// NewRegionsFromIndicies takes an array of ints and convert them to a set of regions, describing
// any contiguous series of indicies as single regions
func NewRegionsFromIndicies(indicies []int) Regions {
	result := Regions{}
	nextRegion := Region{
		Start: 0,
		End:   0,
	}
	sort.Ints(indicies)
	for _, index := range indicies {
		if nextRegion.Empty() {
			nextRegion.Start = index
			nextRegion.End = index + 1
			continue
		}
		if nextRegion.End == index {
			nextRegion.End = index + 1
			continue
		}
		result = append(result, nextRegion)
		nextRegion = Region{
			Start: index,
			End:   index + 1,
		}
	}
	if !nextRegion.Empty() {
		result = append(result, nextRegion)
	}
	return result
}

// NewRegionForEachIndex takes an array of ints and convert them to a set of regions,
// having an individual region for each index
func NewRegionForEachIndex(indicies []int) Regions {
	result := make(Regions, len(indicies))
	for i, index := range indicies {
		result[i] = Region{index, index + 1}
	}
	return result
}

// ExtractItems extracts the items in the regions and returns
// them as an array of arrays with one sub-array with the items of each region
func (rs Regions) ExtractItems(source []interface{}) [][]interface{} {
	result := make([][]interface{}, 0, len(rs))
	for _, r := range rs {
		items := make([]interface{}, 0, r.Len())
		for i := r.Start; i < r.End; i++ {
			items = append(items, source[i])
		}
		result = append(result, items)
	}
	return result
}

// MergeItems is a companion to ExtractItems that takes the arrays of the replace parameter
// and replaces them for each region described in rs returning the updated array and
// regions updated to cover the newly inserted items
func (rs Regions) MergeItems(source []interface{}, replace [][]interface{}) ([]interface{}, Regions) {
	merged := make([]interface{}, 0, len(source))
	outRegions := make(Regions, 0, len(rs))
	sourceIndex := 0
	for regionIndex, r := range rs {
		// Add any elements from source up until the next replacement region
		for ; sourceIndex < r.Start && sourceIndex < len(source); sourceIndex++ {
			merged = append(merged, source[sourceIndex])
		}
		// Move source index to the first item after the replacement region
		sourceIndex = r.End
		// Check that we have items to insert in this region
		var insert []interface{}
		if len(replace) > regionIndex {
			insert = replace[regionIndex]
		} else {
			// Just make an empty replacement array since there was no data in the in-params
			insert = []interface{}{}
		}
		// Add a region describing the segment we are just about to add
		outRegions = append(outRegions, Region{
			Start: len(merged),
			End:   len(merged) + len(insert),
		})
		// Add the new items
		merged = append(merged, insert...)
	}
	// Add any items beyond the end of the regions
	if sourceIndex < len(source) {
		merged = append(merged, source[sourceIndex:]...)
	}
	return merged, outRegions
}

// ToSliceSelector returns a jsonmatch slice selector for the regions on the form
// "2:5,7:9" that would fit in a selector as "my.array[2:5,7:9]".
func (rs Regions) ToSliceSelector() string {
	selectors := make([]string, 0, len(rs))
	for _, r := range rs {
		if r.Len() > 1 || r.Len() == 0 {
			selectors = append(selectors, fmt.Sprintf("%d:%d", r.Start, r.End))
		} else if r.Len() == 1 {
			selectors = append(selectors, fmt.Sprintf("%d", r.Start))
		}
		// The empty regions is just ignored
	}
	return strings.Join(selectors, ",")
}

// NewRegionsFromSliceSelector creates a region set from a jsonmatch slice selector
// (only supporting literal indicies, no matching etc obviously since we don't have
// any data at this junction)
func NewRegionsFromSliceSelector(selector string) (Regions, error) {
	if strings.Trim(selector, " ") == "" {
		return Regions{}, nil
	}
	sliceSpecs := strings.Split(selector, ",")
	result := make(Regions, 0, len(sliceSpecs))
	for _, spec := range sliceSpecs {
		m := strings.Split(spec, ":")
		if len(m) == 1 {
			start, err := strconv.Atoi(strings.Trim(m[0], " "))
			if err != nil {
				return Regions{}, fmt.Errorf("Invalid slice spec %q", spec)
			}
			result = append(result, Region{
				Start: start,
				End:   start + 1,
			})
		} else if len(m) == 2 {
			start, err1 := strconv.Atoi(strings.Trim(m[0], " "))
			end, err2 := strconv.Atoi(strings.Trim(m[1], " "))
			if err1 != nil || err2 != nil {
				return Regions{}, fmt.Errorf("Invalid slice spec %q", spec)
			}
			result = append(result, Region{
				Start: start,
				End:   end,
			})
		} else {
			return Regions{}, fmt.Errorf("Invalid slice spec %q", spec)
		}
	}
	return result.Clean(), nil
}

// IndiciesCount counts the total number of indicies covered by the regions in this set
func (rs Regions) IndiciesCount() int {
	result := 0
	for _, r := range rs {
		result += r.Len()
	}
	return result
}

// Len implements the sortable interface
func (rs Regions) Len() int { return len(rs) }

// Swap implements the sortable interface
func (rs Regions) Swap(i, j int) { rs[i], rs[j] = rs[j], rs[i] }

// Less implements the sortable interface
func (rs Regions) Less(i, j int) bool { return rs[i].Start < rs[j].Start }
