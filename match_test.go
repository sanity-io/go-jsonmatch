package jsonmatch_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sanity-io/jsonmatch"
)

func testRecord() map[string]interface{} {
	return map[string]interface{}{
		"some": map[string]interface{}{
			"path":      "hello there",
			"wrongPath": "hardyharhar!",
		},
		"array":      []interface{}{0, 10, 20, 30, 40},
		"otherArray": []interface{}{100, 200},
		"ghosts": []interface{}{
			map[string]interface{}{"name": "Blinky", "color": "red"},
			map[string]interface{}{"name": "Pinky", "color": "pink"},
			map[string]interface{}{"name": "Inky", "color": "cyan"},
			map[string]interface{}{"name": "Clyde", "color": "orange"},
		},
		"products": []interface{}{
			map[string]interface{}{"newPrice": 12.4, "oldPrice": 25.2, "title": "Deck Chair"},
			map[string]interface{}{"newPrice": 52.2, "oldPrice": 10.0, "title": "Malt Keg"},
		},
		"name": "root",
	}
}

func match(src string, data interface{}) (*jsonmatch.MatchSet, error) {
	expr, err := jsonmatch.Parse(src)
	if err != nil {
		return nil, err
	}
	return expr.Match(data)
}

func extractValues(t *testing.T, src string, data interface{}) interface{} {
	ms, err := match(src, data)
	require.NoError(t, err)
	return ms.Values()
}

// func TestMatch_blah(t *testing.T) {
// 	ms, err := match("/[x?]", []interface{}{
// 		map[string]interface{}{"x": 42},
// 	})
// 	require.NoError(t, err)
// 	assert.Equal(t, []interface{}{42}, ms.Values())
// }

// func TestMatch_set(t *testing.T) {
// 	ms, err := match("some/thing/nice", map[string]interface{}{})
// 	assert.NoError(t, err)
// 	newValue, err := ms.Set(42)
// 	require.NoError(t, err)
// 	assert.Equal(t, map[string]interface{}{
// 		"some": map[string]interface{}{
// 			"thing": map[string]interface{}{
// 				"nice": 42,
// 			},
// 		},
// 	}, newValue)
// }

func TestMatch_simpleFieldExtraction(t *testing.T) {
	ms, err := match("some.path", testRecord())
	assert.NoError(t, err)
	assert.Equal(t, []interface{}{"hello there"}, ms.Values())

	record := testRecord()
	ms, err = match("some.nonExistantPath", record)
	assert.NoError(t, err)
	assert.Equal(t, 0, len(ms.Values()), "Should refer to no value")

	mutated, err := ms.Set("is here")
	assert.NoError(t, err)
	assert.Equal(t, "is here", mutated.(map[string]interface{})["some"].(map[string]interface{})["nonExistantPath"],
		"Should be able to assign a value til a previously unassigned key")
	assert.Equal(t, nil, record["some"].(map[string]interface{})["nonExistantPath"],
		"Mutating should not mutate the underlying data")
}

func TestMatch_simpleArrayExtractionAndMutation(t *testing.T) {
	record := testRecord()
	ms, err := match("array[-1]", record)
	assert.NoError(t, err)
	assert.Equal(t, []interface{}{40}, ms.Values())

	ms, err = match("array[1]", record)
	assert.NoError(t, err)
	assert.Equal(t, []interface{}{10}, ms.Values())

	mutated, err := ms.Delete()
	assert.NoError(t, err)
	assert.Equal(t, 20, mutated.(map[string]interface{})["array"].([]interface{})[1])
}

func TestMatch_fullArrayValues(t *testing.T) {
	ms, err := match("array", testRecord())
	assert.NoError(t, err)
	assert.Equal(t, []interface{}{[]interface{}{0, 10, 20, 30, 40}}, ms.Values())

	ms, err = match("[array,otherArray]", testRecord())
	assert.NoError(t, err)
	assert.Equal(t, []interface{}{[]interface{}{0, 10, 20, 30, 40}, []interface{}{100, 200}}, ms.Values())
}

func TestMatch_simpleArrayRangeExtraction(t *testing.T) {
	record := testRecord()

	ms, err := match("array[1:2]", record)
	assert.NoError(t, err)
	assert.Equal(t, []interface{}{10}, ms.Values())

	ms, err = match("array[3:]", record)
	assert.NoError(t, err)
	assert.Equal(t, []interface{}{30, 40}, ms.Values())

	ms, err = match("array[:3]", record)
	assert.NoError(t, err)
	assert.Equal(t, []interface{}{0, 10, 20}, ms.Values())
}

func TestMatch_wildcard(t *testing.T) {
	ms, err := match("some.*", testRecord())
	assert.NoError(t, err)
	assert.Equal(t, []interface{}{"hello there", "hardyharhar!"}, ms.Values())
}

func TestMatch_recursive(t *testing.T) {
	ms, err := match("..name", testRecord())
	assert.NoError(t, err)
	assert.Equal(t, []interface{}{"Blinky", "Pinky", "Inky", "Clyde", "root"}, ms.Values())
}

func TestMatch_recursiveAssignment(t *testing.T) {
	ms, err := match("..name", testRecord())
	assert.NoError(t, err)
	mutated, err := ms.Set("Ghost")
	assert.NoError(t, err)
	ms2, err := match("..name", mutated)
	assert.NoError(t, err)
	// Check that we only set the existing name properties on objects that have name properties,
	// no adding fields to every object when using the .. operator
	assert.Equal(t, []interface{}{"Ghost", "Ghost", "Ghost", "Ghost", "Ghost"}, ms2.Values())
}

func TestMatch_union(t *testing.T) {
	ms, err := match("ghosts..['name', 'color']", testRecord())
	assert.NoError(t, err)
	assert.Equal(t, []interface{}{"red", "Blinky", "pink", "Pinky", "cyan", "Inky", "orange", "Clyde"}, ms.Values())
}

func TestMatch_filterSliceUsingDocumentValues(t *testing.T) {
	ms, err := match("products[?(newPrice > oldPrice)].title", testRecord())
	assert.NoError(t, err)
	assert.Equal(t, []interface{}{"Malt Keg"}, ms.Values())

	ms, err = match("products[?(newPrice != missing)].title", testRecord())
	assert.NoError(t, err)
	assert.Equal(t, []interface{}{}, ms.Values())
}

func TestMatch_filterSliceUsingLiteralValues(t *testing.T) {
	ms, err := match("ghosts[?(name == \"Clyde\")].color", testRecord())
	assert.NoError(t, err)
	assert.Equal(t, []interface{}{"orange"}, ms.Values())
}

func TestMatch_filterSliceUsingLiteralValuesQuoted(t *testing.T) {
	tr := testRecord()
	tr["ghosts"] = append([]interface{}{tr["ghosts"]}, map[string]interface{}{
		"name": `Ghost "Ghosty" McGhostface`, "color": "blue",
	})
	ms, err := match(`ghosts[?(name == "Ghost \"Ghosty\" McGhostface")].color`, tr)
	if assert.NoError(t, err) {
		assert.Equal(t, []interface{}{"blue"}, ms.Values())
	}
}

func TestMatch_filterMapUsingLiteralValues(t *testing.T) {
	ms, err := match("products[0][?(@ > 25.1)]", testRecord())
	assert.NoError(t, err)
	assert.Equal(t, []interface{}{25.2}, ms.Values())
}

func TestMatch_listAtRoot(t *testing.T) {
	ms, err := match("[ghosts[0].name,products[0].title]", testRecord())
	assert.NoError(t, err)
	assert.Equal(t, []interface{}{"Blinky", "Deck Chair"}, ms.Values())
}

// Test support for strange characters in selectors
func TestMatch_StrangeCharacters(t *testing.T) {
	assert.Equal(t, []interface{}{"an-id"},
		extractValues(t, "_ref", map[string]interface{}{"_ref": "an-id"}))
	assert.Equal(t, []interface{}{"an-id"},
		extractValues(t, "_ref", map[string]interface{}{"_ref": "an-id"}))
}

// Tests ability to set deep, non-existant keys
func TestMatch_LatentMapRef(t *testing.T) {
	ms, err := match("a['c','b','array'].d.e",
		map[string]interface{}{
			"a": map[string]interface{}{
				"b":     map[string]interface{}{},
				"array": []interface{}{1, 2, 3},
			},
			"doNotTouch": map[string]interface{}{"is": "untouched"},
		})
	assert.NoError(t, err)
	modified, err := ms.Set("touched")

	assert.NoError(t, err)
	assert.Equal(t, []interface{}{"untouched"}, extractValues(t, "doNotTouch.is", modified))
	assert.Equal(t, []interface{}{"touched"}, extractValues(t, "a.b.d.e", modified))
	assert.Equal(t, []interface{}{"touched"}, extractValues(t, "a.c.d.e", modified))
	assert.Equal(t, []interface{}{"touched"}, extractValues(t, "a.array.d.e", modified))
	assert.Equal(t, 3, len(extractValues(t, "..[?(@ == \"touched\")]", modified).([]interface{})))

	// New jsonpath2 exists syntax
	ms, err = match("..[@.name?].marked.thing", testRecord())
	require.NoError(t, err)
	modified, err = ms.Set(true)
	require.NoError(t, err)

	// Check marked things
	ms, err = match("..[?(@.marked.thing)].name", modified)
	assert.NoError(t, err)
	assert.Equal(t, []interface{}{"Blinky", "Pinky", "Inky", "Clyde"}, ms.Values())
}

func TestMatch_IllegalToken(t *testing.T) {
	_, err := match("milestones.0.date._type", 0)
	assert.EqualError(t, err, "Syntax error. (Illegal token \"0.\")")
}

func TestMatch_TrailingNakedInteger(t *testing.T) {
	_, err := match("milestones.0", 0)
	assert.EqualError(t, err, "Wrap numbers in brackets when used in dotted path expressions ([0] or [\"0\"] depending on what you mean)")

	_, err = match("milestones[0]", 0)
	assert.NoError(t, err)
}
