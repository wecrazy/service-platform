package fun

import "strings"

// JoinStringSlice concatenates the elements of a string slice to create a single string, separated by the specified delimiter
func JoinStringSlice(slice []string, delimiter string) string {
	return strings.Join(slice, delimiter)
}
