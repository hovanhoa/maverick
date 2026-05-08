package collections_test

import (
	"strconv"
	"testing"

	"github.com/hovanhoa/llmgateway/pkg/core/collections"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMap_StringToStringWithAppendFunction(t *testing.T) {
	t.Parallel()
	test := []string{"one", "two", "three"}
	result := collections.Map(test, func(s string) string { return s + " added" })
	assert.Equal(t, []string{"one added", "two added", "three added"}, result)
}

func TestMap_EmptySlice(t *testing.T) {
	t.Parallel()
	test := []string{}
	result := collections.Map(test, func(s string) string { return s + " added" })
	assert.Equal(t, []string{}, result)
}

func TestMap_NilSlice(t *testing.T) {
	t.Parallel()
	var test []string
	result := collections.Map(test, func(s string) string { return s + " added" })
	assert.Equal(t, []string{}, result)
}

type TestType struct {
	content string
}

func TestMap_ConvertType(t *testing.T) {
	t.Parallel()
	test := []string{"one", "two", "three"}
	result := collections.Map(test, func(s string) TestType { return TestType{content: s} })

	for index, testable := range test {
		assert.Equal(t, testable, result[index].content)
	}
}

func TestMapIndex(t *testing.T) {
	t.Parallel()
	j := 0
	test := []string{"a", "bb", "ccc"}
	result := collections.MapIndex(test, func(i int, v string) int {
		assert.Equal(t, j, i)
		assert.Len(t, v, j+1)
		j++
		return len(v)
	})
	assert.Equal(t, []int{1, 2, 3}, result)
}

func TestMapIndex_EmptySlice(t *testing.T) {
	t.Parallel()
	test := []string{}
	result := collections.MapIndex(test, func(i int, v string) int {
		require.Fail(t, "Function should not be called for empty slice")
		return 0
	})
	assert.Equal(t, []int{}, result)
}

func TestMapIndex_NilSlice(t *testing.T) {
	t.Parallel()
	var test []string
	result := collections.MapIndex(test, func(i int, v string) int {
		require.Fail(t, "Function should not be called for nil slice")
		return 0
	})
	assert.Equal(t, []int{}, result)
}

func TestMax(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		test     []int
		expected int
	}{
		{
			name:     "empty",
			test:     []int{},
			expected: 0,
		},
		{
			name:     "single",
			test:     []int{1},
			expected: 1,
		},
		{
			name:     "multiple",
			test:     []int{3, 6, 4, 2},
			expected: 6,
		},
		{
			name:     "negative numbers",
			test:     []int{-5, -2, -10, -1},
			expected: -1,
		},
		{
			name:     "mixed positive negative",
			test:     []int{-5, 2, -10, 8, -1},
			expected: 8,
		},
		{
			name:     "all same values",
			test:     []int{5, 5, 5, 5},
			expected: 5,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.expected, collections.Max(test.test))
		})
	}
}

func TestMax_Float64(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		test     []float64
		expected float64
	}{
		{
			name:     "empty",
			test:     []float64{},
			expected: 0,
		},
		{
			name:     "decimals",
			test:     []float64{3.14, 2.71, 1.41},
			expected: 3.14,
		},
		{
			name:     "with large numbers",
			test:     []float64{1.0, 2.0, 3.0, 1e308},
			expected: 1e308,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.expected, collections.Max(test.test))
		})
	}
}

func TestMax_String(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		test     []string
		expected string
	}{
		{
			name:     "empty",
			test:     []string{},
			expected: "",
		},
		{
			name:     "lexicographic order",
			test:     []string{"apple", "banana", "cherry"},
			expected: "cherry",
		},
		{
			name:     "case sensitive",
			test:     []string{"Apple", "apple", "BANANA"},
			expected: "apple",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.expected, collections.Max(test.test))
		})
	}
}

func TestFilter(t *testing.T) {
	t.Parallel()
	filter := func(i int) bool { return i%2 == 0 }
	tests := []struct {
		name     string
		test     []int
		expected []int
	}{
		{
			name:     "empty",
			test:     []int{},
			expected: nil,
		},
		{
			name:     "odds",
			test:     []int{1, 3, 5},
			expected: nil,
		},
		{
			name:     "evens",
			test:     []int{2, 4, 6},
			expected: []int{2, 4, 6},
		},
		{
			name:     "mixed",
			test:     []int{2, 3, 6, 5, 1, 4},
			expected: []int{2, 6, 4},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.expected, collections.Filter(test.test, filter))
		})
	}
}

func TestFilter_NilSlice(t *testing.T) {
	t.Parallel()
	var test []int
	filter := func(i int) bool { return i%2 == 0 }
	result := collections.Filter(test, filter)
	assert.Equal(t, []int(nil), result)
}

func TestFilter_AllMatch(t *testing.T) {
	t.Parallel()
	test := []int{2, 4, 6, 8}
	filter := func(i int) bool { return i%2 == 0 }
	result := collections.Filter(test, filter)
	assert.Equal(t, test, result)
}

func TestFilter_NoneMatch(t *testing.T) {
	t.Parallel()
	test := []int{1, 3, 5, 7}
	filter := func(i int) bool { return i%2 == 0 }
	result := collections.Filter(test, filter)
	assert.Equal(t, []int(nil), result)
}

func TestLinearSearch(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		test     []int
		match    func(int) bool
		expected *int
	}{
		{
			name:     "empty slice",
			test:     []int{},
			match:    func(i int) bool { return i == 5 },
			expected: nil,
		},
		{
			name:     "nil slice",
			test:     nil,
			match:    func(i int) bool { return i == 5 },
			expected: nil,
		},
		{
			name:     "found first",
			test:     []int{5, 3, 7, 1},
			match:    func(i int) bool { return i == 5 },
			expected: &[]int{5, 3, 7, 1}[0],
		},
		{
			name:     "found middle",
			test:     []int{1, 3, 5, 7},
			match:    func(i int) bool { return i == 5 },
			expected: &[]int{1, 3, 5, 7}[2],
		},
		{
			name:     "found last",
			test:     []int{1, 3, 7, 5},
			match:    func(i int) bool { return i == 5 },
			expected: &[]int{1, 3, 7, 5}[3],
		},
		{
			name:     "not found",
			test:     []int{1, 3, 7, 9},
			match:    func(i int) bool { return i == 5 },
			expected: nil,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := collections.LinearSearch(test.test, test.match)
			assert.Equal(t, test.expected, result)
		})
	}
}

func TestLinearSearch_String(t *testing.T) {
	t.Parallel()
	test := []string{"apple", "banana", "cherry"}
	match := func(s string) bool { return s == "banana" }
	result := collections.LinearSearch(test, match)
	assert.Equal(t, &test[1], result)
}

func TestFind(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		test     []int
		values   []int
		expected *int
	}{
		{
			name:     "empty slice",
			test:     []int{},
			values:   []int{5},
			expected: nil,
		},
		{
			name:     "nil slice",
			test:     nil,
			values:   []int{5},
			expected: nil,
		},
		{
			name:     "single value found",
			test:     []int{1, 3, 5, 7},
			values:   []int{5},
			expected: &[]int{1, 3, 5, 7}[2],
		},
		{
			name:     "single value not found",
			test:     []int{1, 3, 7, 9},
			values:   []int{5},
			expected: nil,
		},
		{
			name:     "multiple values found first",
			test:     []int{1, 3, 5, 7},
			values:   []int{5, 7, 9},
			expected: &[]int{1, 3, 5, 7}[2],
		},
		{
			name:     "multiple values found last",
			test:     []int{1, 3, 7, 5},
			values:   []int{5, 7, 9},
			expected: &[]int{1, 3, 7, 5}[2], // Should find 7 first (index 2)
		},
		{
			name:     "multiple values not found",
			test:     []int{1, 3, 7, 9},
			values:   []int{2, 4, 6},
			expected: nil,
		},
		{
			name:     "no values to search",
			test:     []int{1, 3, 5, 7},
			values:   []int{},
			expected: nil,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := collections.Find(test.test, test.values...)
			assert.Equal(t, test.expected, result)
		})
	}
}

func TestFind_String(t *testing.T) {
	t.Parallel()
	test := []string{"apple", "banana", "cherry"}
	result := collections.Find(test, "banana", "grape")
	assert.Equal(t, &test[1], result)
}

func TestContains(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		test     []int
		values   []int
		expected bool
	}{
		{
			name:     "empty slice",
			test:     []int{},
			values:   []int{5},
			expected: false,
		},
		{
			name:     "nil slice",
			test:     nil,
			values:   []int{5},
			expected: false,
		},
		{
			name:     "single value found",
			test:     []int{1, 3, 5, 7},
			values:   []int{5},
			expected: true,
		},
		{
			name:     "single value not found",
			test:     []int{1, 3, 7, 9},
			values:   []int{5},
			expected: false,
		},
		{
			name:     "multiple values found",
			test:     []int{1, 3, 5, 7},
			values:   []int{5, 7, 9},
			expected: true,
		},
		{
			name:     "multiple values not found",
			test:     []int{1, 3, 7, 9},
			values:   []int{2, 4, 6},
			expected: false,
		},
		{
			name:     "no values to search",
			test:     []int{1, 3, 5, 7},
			values:   []int{},
			expected: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := collections.Contains(test.test, test.values...)
			assert.Equal(t, test.expected, result)
		})
	}
}

func TestContains_String(t *testing.T) {
	t.Parallel()
	test := []string{"apple", "banana", "cherry"}
	assert.True(t, collections.Contains(test, "banana"))
	assert.True(t, collections.Contains(test, "banana", "grape"))
	assert.False(t, collections.Contains(test, "grape"))
	assert.False(t, collections.Contains(test, "grape", "orange"))
}

func TestPartition(t *testing.T) {
	t.Parallel()
	evens, odds := collections.Partition([]int{1, 2, 3, 4, 5}, func(i int) bool { return i%2 == 0 })
	assert.Equal(t, []int{2, 4}, evens)
	assert.Equal(t, []int{1, 3, 5}, odds)
}

func TestPartition_EmptySlice(t *testing.T) {
	t.Parallel()
	matched, unmatched := collections.Partition([]int{}, func(i int) bool { return i%2 == 0 })
	assert.Equal(t, []int(nil), matched)
	assert.Equal(t, []int(nil), unmatched)
}

func TestPartition_NilSlice(t *testing.T) {
	t.Parallel()
	var test []int
	matched, unmatched := collections.Partition(test, func(i int) bool { return i%2 == 0 })
	assert.Equal(t, []int(nil), matched)
	assert.Equal(t, []int(nil), unmatched)
}

func TestPartition_AllMatch(t *testing.T) {
	t.Parallel()
	test := []int{2, 4, 6, 8}
	matched, unmatched := collections.Partition(test, func(i int) bool { return i%2 == 0 })
	assert.Equal(t, test, matched)
	assert.Equal(t, []int(nil), unmatched)
}

func TestPartition_NoneMatch(t *testing.T) {
	t.Parallel()
	test := []int{1, 3, 5, 7}
	matched, unmatched := collections.Partition(test, func(i int) bool { return i%2 == 0 })
	assert.Equal(t, []int(nil), matched)
	assert.Equal(t, test, unmatched)
}

func TestPartition_String(t *testing.T) {
	t.Parallel()
	test := []string{"apple", "banana", "cherry", "date"}
	short, long := collections.Partition(test, func(s string) bool { return len(s) <= 5 })
	assert.Equal(t, []string{"apple", "date"}, short)
	assert.Equal(t, []string{"banana", "cherry"}, long)
}

func TestValues(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		input    map[string]int
		expected []int
	}{
		{
			name:     "empty map",
			input:    map[string]int{},
			expected: []int{},
		},
		{
			name:     "nil map",
			input:    nil,
			expected: []int{},
		},
		{
			name:     "single entry",
			input:    map[string]int{"a": 1},
			expected: []int{1},
		},
		{
			name:     "multiple entries",
			input:    map[string]int{"a": 1, "b": 2, "c": 3},
			expected: []int{1, 2, 3}, // Order may vary
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := collections.Values(test.input)
			// Sort results for consistent comparison since map iteration order is not guaranteed
			assert.ElementsMatch(t, test.expected, result)
		})
	}
}

func TestValues_StringMap(t *testing.T) {
	t.Parallel()
	input := map[int]string{1: "one", 2: "two", 3: "three"}
	result := collections.Values(input)
	assert.ElementsMatch(t, []string{"one", "two", "three"}, result)
}

func TestSet(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		input    []int
		expected map[int]struct{}
	}{
		{
			name:     "empty slice",
			input:    []int{},
			expected: map[int]struct{}{},
		},
		{
			name:     "nil slice",
			input:    nil,
			expected: map[int]struct{}{},
		},
		{
			name:     "unique values",
			input:    []int{1, 2, 3},
			expected: map[int]struct{}{1: {}, 2: {}, 3: {}},
		},
		{
			name:     "duplicate values",
			input:    []int{1, 2, 2, 3, 1},
			expected: map[int]struct{}{1: {}, 2: {}, 3: {}},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := collections.Set(test.input)
			assert.Equal(t, test.expected, result)
		})
	}
}

func TestSet_String(t *testing.T) {
	t.Parallel()
	input := []string{"apple", "banana", "apple", "cherry"}
	result := collections.Set(input)
	expected := map[string]struct{}{"apple": {}, "banana": {}, "cherry": {}}
	assert.Equal(t, expected, result)
}

func TestGroupBy(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		input    []int
		groupBy  func(int) string
		expected map[string][]int
	}{
		{
			name:     "empty slice",
			input:    []int{},
			groupBy:  func(i int) string { return "group" },
			expected: map[string][]int{},
		},
		{
			name:     "nil slice",
			input:    nil,
			groupBy:  func(i int) string { return "group" },
			expected: map[string][]int{},
		},
		{
			name:  "group by even/odd",
			input: []int{1, 2, 3, 4, 5, 6},
			groupBy: func(i int) string {
				if i%2 == 0 {
					return "even"
				} else {
					return "odd"
				}
			},
			expected: map[string][]int{"even": {2, 4, 6}, "odd": {1, 3, 5}},
		},
		{
			name:  "group by length",
			input: []int{1, 12, 123, 1234},
			groupBy: func(i int) string {
				return strconv.Itoa(len(strconv.Itoa(i)))
			},
			expected: map[string][]int{"1": {1}, "2": {12}, "3": {123}, "4": {1234}},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := collections.GroupBy(test.input, test.groupBy)
			assert.Equal(t, test.expected, result)
		})
	}
}

func TestGroupBy_String(t *testing.T) {
	t.Parallel()
	input := []string{"apple", "banana", "cherry", "date"}
	result := collections.GroupBy(input, func(s string) int { return len(s) })
	expected := map[int][]string{
		5: {"apple"},
		6: {"banana", "cherry"},
		4: {"date"},
	}
	assert.Equal(t, expected, result)
}

func TestKeys(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		input    map[string]int
		expected []string
	}{
		{
			name:     "empty map",
			input:    map[string]int{},
			expected: []string{},
		},
		{
			name:     "nil map",
			input:    nil,
			expected: []string{},
		},
		{
			name:     "single entry",
			input:    map[string]int{"a": 1},
			expected: []string{"a"},
		},
		{
			name:     "multiple entries",
			input:    map[string]int{"a": 1, "b": 2, "c": 3},
			expected: []string{"a", "b", "c"}, // Order may vary
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := collections.Keys(test.input)
			// Sort results for consistent comparison since map iteration order is not guaranteed
			assert.ElementsMatch(t, test.expected, result)
		})
	}
}

func TestKeys_IntMap(t *testing.T) {
	t.Parallel()
	input := map[int]string{1: "one", 2: "two", 3: "three"}
	result := collections.Keys(input)
	assert.ElementsMatch(t, []int{1, 2, 3}, result)
}

func TestIsEmptyValue(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		value    interface{}
		expected bool
	}{
		{name: "nil", value: nil, expected: true},
		{name: "empty string", value: "", expected: true},
		{name: "non-empty string", value: "hello", expected: false},
		{name: "zero float64", value: float64(0), expected: true},
		{name: "non-zero float64", value: float64(1.5), expected: false},
		{name: "zero int", value: 0, expected: true},
		{name: "non-zero int", value: 42, expected: false},
		{name: "false bool returns false", value: false, expected: false},
		{name: "true bool returns false", value: true, expected: false},
		{name: "empty slice", value: []interface{}{}, expected: true},
		{name: "non-empty slice", value: []interface{}{"a"}, expected: false},
		{name: "empty map", value: map[string]interface{}{}, expected: true},
		{name: "non-empty map", value: map[string]interface{}{"k": "v"}, expected: false},
		{name: "non-empty nested map", value: map[string]interface{}{"k": map[string]interface{}{}}, expected: false},
		{name: "struct returns false", value: struct{}{}, expected: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, collections.IsEmptyValue(tt.value))
		})
	}
}
