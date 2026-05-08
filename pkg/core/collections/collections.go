package collections

import "golang.org/x/exp/constraints"

func Map[T, R any](array []T, function func(T) R) []R {
	result := []R{}
	for _, item := range array {
		result = append(result, function(item))
	}

	return result
}

func MapIndex[T, R any](array []T, function func(int, T) R) []R {
	result := []R{}
	for i, item := range array {
		result = append(result, function(i, item))
	}

	return result
}

func Max[T constraints.Ordered](vals []T) (max T) {
	if len(vals) == 0 {
		return
	}
	max = vals[0]
	for _, v := range vals[1:] {
		if v > max {
			max = v
		}
	}
	return
}

// Filter returns the elements of the array for which the passed
// `filter` function returns true.
func Filter[T any](array []T, filter func(T) bool) (result []T) {
	for _, item := range array {
		if filter(item) {
			result = append(result, item)
		}
	}

	return
}

// LinearSearch finds the address to the first element of the array
// for which the match function returns true, or nil otherwise.
func LinearSearch[T any](array []T, match func(T) bool) *T {
	for i, item := range array {
		if match(item) {
			return &array[i]
		}
	}

	return nil
}

func Find[T comparable](array []T, val ...T) *T {
	return LinearSearch(array, func(v T) bool {
		return LinearSearch(val, func(dv T) bool {
			return v == dv
		}) != nil
	})
}

func Contains[T comparable](array []T, val ...T) bool {
	return Find(array, val...) != nil
}

// Partition splits an array into two subarrays, one containing
// items that match the partitioning function and one containing
// the rest.
func Partition[T any](array []T, match func(T) bool) (matched []T, unmatched []T) {
	for i, item := range array {
		if match(item) {
			matched = append(matched, array[i])
		} else {
			unmatched = append(unmatched, array[i])
		}
	}

	return
}

func Values[K comparable, T any](m map[K]T) []T {
	var result []T
	for _, v := range m {
		result = append(result, v)
	}

	return result
}

func Set[V comparable](values []V) map[V]struct{} {
	set := make(map[V]struct{})
	for _, v := range values {
		set[v] = struct{}{}
	}

	return set
}

func GroupBy[T any, K comparable](array []T, groupBy func(T) K) map[K][]T {
	groups := make(map[K][]T)
	for _, item := range array {
		group := groupBy(item)
		groups[group] = append(groups[group], item)
	}

	return groups
}

func Keys[K comparable, V any](m map[K]V) []K {
	keys := make([]K, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// IsEmptyValue returns true if v is nil, an empty string, zero, or an empty slice/map.
// This is useful for detecting missing or unset values after unmarshaling JSON into
// a map[string]interface{}, where Go's encoder produces these zero values.
func IsEmptyValue(v interface{}) bool {
	if v == nil {
		return true
	}
	switch val := v.(type) {
	case string:
		return val == ""
	case float64:
		return val == 0
	case int:
		return val == 0
	case []interface{}:
		return len(val) == 0
	case map[string]interface{}:
		return len(val) == 0
	}
	return false
}
