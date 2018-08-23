package jsonmatch_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sanity-io/jsonmatch"
)

func r(start int, end int) jsonmatch.Region {
	return jsonmatch.Region{Start: start, End: end}
}

func rs(regions ...jsonmatch.Region) jsonmatch.Regions {
	if len(regions) == 0 {
		return jsonmatch.Regions{}
	}
	return jsonmatch.Regions(regions)
}

func parseRs(selector string) jsonmatch.Regions {
	rs, err := jsonmatch.NewRegionsFromSliceSelector(selector)
	if err != nil {
		panic(err)
	}
	return rs
}

func TestEmptyRegion(t *testing.T) {
	empty := jsonmatch.Region{
		Start: 5,
		End:   5,
	}
	assert.Equal(t, 0, empty.Len())
	assert.True(t, empty.Empty())
	assert.True(t, empty.Adjacent(jsonmatch.Region{
		Start: 5,
		End:   8,
	}))
	assert.True(t, empty.Adjacent(jsonmatch.Region{
		Start: 5,
		End:   5,
	}))

	assert.False(t, empty.ContainsIndex(5))
	assert.False(t, empty.IsBefore(5))
	assert.True(t, empty.IsBefore(4))
	assert.True(t, empty.IsAfter(5))
	assert.True(t, empty.IsAfter(4))
	assert.False(t, empty.IsAfter(6))

	grown := empty.Grow(3)
	assert.Equal(t, 3, grown.Len())
	assert.Equal(t, 5, grown.Start)
	assert.Equal(t, 8, grown.End)

	shrunk := empty.Shrink(3)
	assert.Equal(t, 5, shrunk.Start)
	assert.Equal(t, 5, shrunk.End)

	shifted := empty.Shift(3)
	assert.Equal(t, 0, shifted.Len())
	assert.Equal(t, 8, shifted.Start)
	assert.Equal(t, 8, shifted.End)
}

func TestOverlap(t *testing.T) {
	assert.False(t, r(2, 3).Overlap(r(3, 4)))
	assert.False(t, r(3, 4).Overlap(r(2, 3)))
	assert.True(t, r(2, 4).Overlap(r(3, 4)))
	assert.True(t, r(3, 4).Overlap(r(2, 4)))
	assert.True(t, r(1, 8).Overlap(r(3, 4)))
	assert.True(t, r(3, 4).Overlap(r(1, 8)))
	assert.False(t, r(5, 5).Overlap(r(5, 6)))
	assert.False(t, r(5, 5).Overlap(r(5, 5)))
	assert.True(t, r(4, 6).Overlap(r(5, 5)))
	assert.False(t, r(0, 5).Overlap(r(5, 5)))
	assert.False(t, r(1, 1).Overlap(r(4, 8)))
}

func TestJoin(t *testing.T) {
	joined := r(2, 5).Join(r(5, 7))
	assert.Equal(t, 2, joined.Start)
	assert.Equal(t, 7, joined.End)

	joined = r(2, 2).Join(r(7, 7))
	assert.Equal(t, 2, joined.Start)
	assert.Equal(t, 7, joined.End)
}

func TestCut(t *testing.T) {
	master := r(5, 8)
	// Cut from start
	assert.Equal(t, master.Cut(r(2, 7)), r(2, 3))
	// Cut from end
	assert.Equal(t, master.Cut(r(7, 12)), r(5, 7))
	// Cut inside
	assert.Equal(t, master.Cut(r(6, 7)), r(5, 7))
	// Cut to the bone
	assert.Equal(t, master.Cut(r(5, 8)), r(5, 5))
	// Cut to left
	assert.Equal(t, master.Cut(r(1, 3)), r(3, 6))
	// Cut to the right
	assert.Equal(t, master.Cut(r(12, 36)), r(5, 8))
}

func TestToSliceSelector(t *testing.T) {
	assert.Equal(t, "1,4:9,12:12",
		rs(
			r(1, 2),
			r(4, 9),
			r(12, 12),
		).ToSliceSelector())
	assert.Equal(t, "", rs().ToSliceSelector())
}

func TestNewRegionsFromSliceSelector(t *testing.T) {
	regions, err := jsonmatch.NewRegionsFromSliceSelector("1,4:9")
	assert.NoError(t, err)
	assert.Equal(t, "1,4:9", regions.ToSliceSelector())

	regions, err = jsonmatch.NewRegionsFromSliceSelector("7:13 ,1:6  , 4:9")
	assert.NoError(t, err)
	assert.Equal(t, "1:4,4:6,6,7:9,9:13", regions.ToSliceSelector())

	regions, err = jsonmatch.NewRegionsFromSliceSelector("1,2, 3, 4,7, 8,9")
	assert.NoError(t, err)
	assert.Equal(t, "1,2,3,4,7,8,9", regions.ToSliceSelector())

	regions, err = jsonmatch.NewRegionsFromSliceSelector("")
	assert.NoError(t, err)
	assert.Equal(t, "", regions.ToSliceSelector())

	regions, err = jsonmatch.NewRegionsFromSliceSelector("1,2,invalid:selector")
	assert.Error(t, err)

	regions, err = jsonmatch.NewRegionsFromSliceSelector("@")
	assert.Error(t, err)

	regions, err = jsonmatch.NewRegionsFromSliceSelector("2,3,4:6,7")
	assert.NoError(t, err)
	assert.Equal(t, "2,3,4:6,7", regions.ToSliceSelector())

	regions, err = jsonmatch.NewRegionsFromSliceSelector("1:5,5:8")
	assert.NoError(t, err)
	assert.Equal(t, "1:5,5:8", regions.ToSliceSelector())

	assert.Equal(t, parseRs("1,3,5,7,9"), rs(r(1, 2), r(3, 4), r(5, 6), r(7, 8), r(9, 10)))

	regions, err = jsonmatch.NewRegionsFromSliceSelector("5:5")
	assert.NoError(t, err)
	assert.Equal(t, "5:5", regions.ToSliceSelector())

}

func TestContainsIndex(t *testing.T) {
	assert.False(t, r(3, 4).ContainsIndex(1))
	assert.False(t, r(3, 4).ContainsIndex(4))
	assert.True(t, r(3, 4).ContainsIndex(3))
	assert.False(t, r(3, 3).ContainsIndex(3))
}

func TestIsAfter(t *testing.T) {
	assert.True(t, r(3, 4).IsAfter(1))
	assert.True(t, r(3, 4).IsAfter(3))
	assert.False(t, r(3, 4).IsAfter(4))
}

func TestInsertAt(t *testing.T) {
	regions, _ := jsonmatch.NewRegionsFromSliceSelector("1,4:9,12:16")
	regions = regions.InsertAt(r(5, 6))
	assert.Equal(t, "1,4:10,13:17", regions.ToSliceSelector())

	regions, _ = jsonmatch.NewRegionsFromSliceSelector("1,4:9,12:16")
	regions = regions.InsertAt(r(4, 14))
	assert.Equal(t, "1,14:19,22:26", regions.ToSliceSelector())

	regions, _ = jsonmatch.NewRegionsFromSliceSelector("1,4:9,12:16")
	regions = regions.InsertAt(r(0, 2))
	assert.Equal(t, "3,6:11,14:18", regions.ToSliceSelector())

	regions, _ = jsonmatch.NewRegionsFromSliceSelector("1,4:9,12:16")
	regions = regions.InsertAt(r(20, 22))
	assert.Equal(t, "1,4:9,12:16", regions.ToSliceSelector())
}

func TestRegionsCut(t *testing.T) {
	regions, _ := jsonmatch.NewRegionsFromSliceSelector("1,4:9,12:16")
	regions = regions.Cut(r(5, 6))
	assert.Equal(t, "1,4:8,11:15", regions.ToSliceSelector())

	regions, _ = jsonmatch.NewRegionsFromSliceSelector("1,4:9,12:16")
	regions = regions.Cut(r(4, 14))
	assert.Equal(t, "1,4:4,4:6", regions.ToSliceSelector())

	regions, _ = jsonmatch.NewRegionsFromSliceSelector("1,4:9,12:16")
	regions = regions.Cut(r(0, 2))
	assert.Equal(t, "0:0,2:7,10:14", regions.ToSliceSelector())

	regions, _ = jsonmatch.NewRegionsFromSliceSelector("1,4:9,12:16")
	regions = regions.Cut(r(20, 22))
	assert.Equal(t, "1,4:9,12:16", regions.ToSliceSelector())
}

func TestToIndicies(t *testing.T) {
	regions, _ := jsonmatch.NewRegionsFromSliceSelector("1,4:9,12:16")
	assert.Equal(t, []int{1, 4, 5, 6, 7, 8, 12, 13, 14, 15}, regions.ToIndicies())
}

func TestNewFromIndicies(t *testing.T) {
	regions := jsonmatch.NewRegionsFromIndicies([]int{1, 4, 5, 6, 7, 8, 12, 13, 14, 15})
	assert.Equal(t, "1,4:9,12:16", regions.ToSliceSelector())
}

func TestExtractItems(t *testing.T) {
	regions, _ := jsonmatch.NewRegionsFromSliceSelector("1,4:9")
	data := []interface{}{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	extract := regions.ExtractItems(data)
	assert.Equal(t, [][]interface{}{[]interface{}{1}, []interface{}{4, 5, 6, 7, 8}}, extract)
}

func TestMergeItems(t *testing.T) {
	regions, _ := jsonmatch.NewRegionsFromSliceSelector("1,4:9")
	data := []interface{}{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	merged, regions := regions.MergeItems(data, [][]interface{}{[]interface{}{"huba", "hopp"}, []interface{}{"foo"}})
	assert.Equal(t, []interface{}{0, "huba", "hopp", 2, 3, "foo", 9, 10}, merged)
	assert.Equal(t, "1:3,5", regions.ToSliceSelector())

	regions, _ = jsonmatch.NewRegionsFromSliceSelector("1,4:9")
	merged, regions = regions.MergeItems(data, [][]interface{}{[]interface{}{}, []interface{}{}})
	assert.Equal(t, []interface{}{0, 2, 3, 9, 10}, merged)
	assert.Equal(t, "", regions.Simplify().ToSliceSelector())

	regions, _ = jsonmatch.NewRegionsFromSliceSelector("0:100")
	merged, regions = regions.MergeItems(data, [][]interface{}{[]interface{}{}})
	assert.Equal(t, []interface{}{}, merged)
	assert.Equal(t, "", regions.Simplify().ToSliceSelector())

	regions, _ = jsonmatch.NewRegionsFromSliceSelector("0:10")
	merged, regions = regions.MergeItems(data, [][]interface{}{[]interface{}{"hello", "mutant"}})
	assert.Equal(t, []interface{}{"hello", "mutant", 10}, merged)
	assert.Equal(t, "0:2", regions.ToSliceSelector())

	regions, _ = jsonmatch.NewRegionsFromSliceSelector("0:0")
	merged, regions = regions.MergeItems([]interface{}{}, [][]interface{}{[]interface{}{"hello", "mutant"}})
	assert.Equal(t, []interface{}{"hello", "mutant"}, merged)
	assert.Equal(t, "0:2", regions.ToSliceSelector())
}

func runCleanScenario(t *testing.T, jumble jsonmatch.Regions, expect jsonmatch.Regions, description string) {
	cleaned := jumble.Clean()
	assert.Equal(t, expect, cleaned, description)
}

func TestClean(t *testing.T) {
	runCleanScenario(t, rs(r(2, 3), r(1, 10)), rs(r(1, 2), r(2, 3), r(3, 10)), "A region interrupted by another region")
	runCleanScenario(t, rs(r(1, 10), r(2, 2)), rs(r(1, 2), r(2, 2), r(2, 10)), "A region interrupted by a zero-length region")
	runCleanScenario(t, rs(r(20, 30), r(1, 10)), rs(r(1, 10), r(20, 30)), "Non-overlapping gets sorted")
	runCleanScenario(t, rs(r(20, 30), r(20, 30)), rs(r(20, 30)), "De-dupe")
}

func assertUnion(t *testing.T, l string, r string, expect string, description string) {
	left, err := jsonmatch.NewRegionsFromSliceSelector(l)
	require.NoError(t, err)
	right, err := jsonmatch.NewRegionsFromSliceSelector(r)
	require.NoError(t, err)
	union := left.Union(right)
	assert.Equal(t, expect, union.ToSliceSelector(), description)
	reverseUnion := right.Union(left)
	assert.Equal(t, expect, reverseUnion.ToSliceSelector(), description+" (reverse)")
}

func TestUnion(t *testing.T) {
	assertUnion(t, "1, 4:9", "2", "1,2,4:9", "Non overlapping, but adjacent must not be joined")
	assertUnion(t, "12:20", "15:25", "12:25", "Partially overlapping must be joined")
	assertUnion(t, "12:20", "0:25", "0:25", "Totally overlapping must be joined")
	assertUnion(t, "1:5", "0:3,3:5", "0:5", "Serially overlapping must be joined")
	assertUnion(t, "1,3,4,5,6", "2,3,4:6", "1,2,3,4:6,6", "Adjacent multiple must not be joined")
	assertUnion(t, "1:9, 15", "", "1:9,15", "Empty sets are no problem")
}

func TestUnionInfinitesimals(t *testing.T) {
	union := rs(r(5, 5)).Union(rs(r(5, 5), r(5, 6)))
	assert.Equal(t, rs(r(5, 5), r(5, 6)), union, "Infinitesimal slices must be preserved when adjacent in a union")

	union = rs(r(6, 6)).Union(rs(r(5, 5), r(5, 6)))
	assert.Equal(t, rs(r(5, 5), r(5, 6), r(6, 6)), union, "Infinitesimal slices must be preserved when adjacent in a union")

	union = rs(r(5, 5)).Union(rs(r(3, 6)))
	assert.Equal(t, rs(r(3, 6)), union, "Infinitesimal slices must be merged when overlapping")
}

func TestIntersectRegionVsRegion(t *testing.T) {
	assert.Equal(t, rs(r(3, 4)), r(3, 5).Intersect(r(4, 9)))
	assert.Equal(t, rs(r(4, 5)), r(3, 5).Intersect(r(1, 4)))
	assert.Equal(t, rs(), r(3, 5).Intersect(r(3, 5)))
	assert.Equal(t, rs(), r(3, 5).Intersect(r(1, 10)))
	assert.Equal(t, rs(r(3, 4), r(6, 10)), r(3, 10).Intersect(r(4, 6)))
	assert.Equal(t, rs(r(1, 10)), r(1, 10).Intersect(r(100, 1000)))
}

func runRegionIntersectionScenario(t *testing.T, l string, r string, expect string, description string) {
	left, err := jsonmatch.NewRegionsFromSliceSelector(l)
	require.NoError(t, err)
	right, err := jsonmatch.NewRegionsFromSliceSelector(r)
	require.NoError(t, err)
	intersection := left.Intersect(right)
	require.NoError(t, err)
	assert.Equal(t, expect, intersection.ToSliceSelector(), description)
}

func TestIntersectRegionsVsRegions(t *testing.T) {
	runRegionIntersectionScenario(t, "0:50", "25:30", "0:25,30:50", "A cut from the middle should work")
	runRegionIntersectionScenario(t, "0:50", "0:5, 25:30", "5:25,30:50", "Two cuts in the same chunk, one from the left, one in the middle should work")
	runRegionIntersectionScenario(t, "0:25,30:50", "20:40", "0:20,40:50", "One cut from the middle of two chunks should work")
	runRegionIntersectionScenario(t, "0:10", "1,3,5,7,9", "0,2,4,6,8", "Several single item cuts in a single chunk should work")
	runRegionIntersectionScenario(t, "", "1,3,5,7,9", "", "Slicing through thin air should work fine")
	runRegionIntersectionScenario(t, "0:10", "5:5", "0:5,5:10", "Slicing with infinitesimals should work")
}
