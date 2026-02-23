package unit

import (
	"testing"
)

// BenchmarkStringOperations benchmarks various string operations
// Run with: make benchstat
// Compare results: benchstat old.txt new.txt
func BenchmarkStringConcat(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = "hello" + " " + "world"
	}
}

// BenchmarkStringRepeat benchmarks string repetition
func BenchmarkStringRepeat(b *testing.B) {
	str := "go"
	for i := 0; i < b.N; i++ {
		_ = str + str + str
	}
}

// BenchmarkStringToLower benchmarks string case conversion
func BenchmarkStringToLower(b *testing.B) {
	str := "SERVICE_PLATFORM_API_V1_ENDPOINT_HANDLER"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = toLower(str)
	}
}

// Helper function for benchmarking
func toLower(s string) string {
	result := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			result[i] = c + 32
		} else {
			result[i] = c
		}
	}
	return string(result)
}
