// Package stringutil provides common string utility functions.
package stringutil

import "strings"

// ContainsCaseInsensitive checks if a string exists in a slice (case-insensitive).
func ContainsCaseInsensitive(slice []string, target string) bool {
	targetLower := strings.ToLower(target)

	for _, s := range slice {
		if strings.ToLower(s) == targetLower {
			return true
		}
	}

	return false
}
