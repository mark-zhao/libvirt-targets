// Package slices contains functions to operate on slices treated as sets.
package slices // import "github.com/influxdata/influxdb/pkg/slices"

import "strings"

// Union combines two string sets.
func Union(setA, setB []string, ignoreCase bool) []string {
	for _, b := range setB {
		if ignoreCase {
			if !ExistsIgnoreCase(setA, b) {
				setA = append(setA, b)
			}
			continue
		}
		if !Exists(setA, b) {
			setA = append(setA, b)
		}
	}
	return setA
}

// Exists checks if a string is in a set.
func Exists(set []string, find string) bool {
	for _, s := range set {
		if s == find {
			return true
		}
	}
	return false
}

// ExistsIgnoreCase checks if a string is in a set but ignores its case.
func ExistsIgnoreCase(set []string, find string) bool {
	for _, s := range set {
		if strings.EqualFold(s, find) {
			return true
		}
	}
	return false
}

// StringsToBytes converts a variable number of strings into a slice of []byte.
func StringsToBytes(s ...string) [][]byte {
	a := make([][]byte, 0, len(s))
	for _, v := range s {
		a = append(a, []byte(v))
	}
	return a
}
