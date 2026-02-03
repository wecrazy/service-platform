package fun

import "fmt"

func BytesToMB(size int64) string {
	mb := float64(size) / (1024 * 1024)
	return fmt.Sprintf("%.2f MB", mb)
}
