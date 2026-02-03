package fun

// MarkerSymbolUnicode returns a Unicode symbol for the given marker name
func MarkerSymbolUnicode(symbol string) string {
	switch symbol {
	case "circle":
		return "\u25CF" // ●
	case "square":
		return "\u25A0" // ■
	case "diamond":
		return "\u25C6" // ◆
	case "triangle":
		return "\u25B2" // ▲
	case "plus":
		return "\u002B" // +
	case "cross":
		return "\u2715" // ✕
	case "star":
		return "\u2605" // ★
	default:
		return "\u2022" // •
	}
}
