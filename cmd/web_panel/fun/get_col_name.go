package fun

// Generate Excel-style column names: A, B, ..., Z, AA, AB, ...
func GetColName(n int) string {
	name := ""
	for n >= 0 {
		// name = string('A'+(n%26)) + name
		name = string(rune('A'+(n%26))) + name
		n = n/26 - 1
	}
	return name
}
