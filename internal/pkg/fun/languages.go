package fun

// Language codes
const (
	LangID = "id" // Indonesian
	LangEN = "en" // English
	LangES = "es" // Spanish
	LangFR = "fr" // French
	LangDE = "de" // German
	LangPT = "pt" // Portuguese
	LangRU = "ru" // Russian
	LangJP = "jp" // Japanese
	LangCN = "cn" // Chinese
	LangAR = "ar" // Arabic
)

// DefaultLang is the default language code that used if no language is specified
const DefaultLang = LangID

// LanguageNameMap maps language codes to their display names
var LanguageNameMap = map[string]string{
	LangID: "Bahasa Indonesia",
	LangEN: "English",
	LangES: "Spanish",
	LangFR: "French",
	LangDE: "German",
	LangPT: "Portuguese",
	LangRU: "Russian",
	LangJP: "Japanese",
	LangCN: "Chinese",
	LangAR: "Arabic",
}

// LanguageAliasMap maps alternative language codes to their canonical codes
// This allows users to use ISO 639-1 or other common codes
var LanguageAliasMap = map[string]string{
	// Indonesian
	"in": LangID, // Sometimes reported as "in" instead of "id"

	// English variants
	"en-US": LangEN,
	"en-GB": LangEN,
	"en-CA": LangEN,

	// Spanish variants
	"es-ES": LangES,
	"es-MX": LangES,
	"es-AR": LangES,
	"es-CO": LangES,

	// French variants
	"fr-FR": LangFR,
	"fr-CA": LangFR,
	"fr-BE": LangFR,

	// German variants
	"de-DE": LangDE,
	"de-AT": LangDE,
	"de-CH": LangDE,

	// Portuguese variants
	"pt-BR": LangPT,
	"pt-PT": LangPT,

	// Russian variants
	"ru-RU": LangRU,
	"ru-UA": LangRU,

	// Japanese
	"ja": LangJP, // ISO 639-1 code "ja" maps to our "jp"

	// Chinese variants
	"zh":    LangCN, // General Chinese
	"zh-CN": LangCN, // Simplified Chinese
	"zh-TW": LangCN, // Traditional Chinese (still map to our "cn")
	"zh-HK": LangCN,

	// Arabic variants
	"ar-SA": LangAR, // Saudi Arabia
	"ar-EG": LangAR, // Egypt
	"ar-AE": LangAR, // UAE
}

// GetSupportedLanguages returns a list of all supported language codes
func GetSupportedLanguages() []string {
	return []string{
		LangID,
		LangEN,
		LangES,
		LangFR,
		LangDE,
		LangPT,
		LangRU,
		LangJP,
		LangCN,
		LangAR,
	}
}

// IsSupportedLanguage checks if a language code is supported
func IsSupportedLanguage(code string) bool {
	for _, lang := range GetSupportedLanguages() {
		if lang == code {
			return true
		}
	}
	return false
}

// NormalizeLanguageCode converts an alias or alternative language code to the canonical code
// For example, "ja" is converted to "jp", "zh" is converted to "cn"
// If the code is already canonical or not recognized, it returns the original code
func NormalizeLanguageCode(code string) string {
	if canonical, exists := LanguageAliasMap[code]; exists {
		return canonical
	}
	return code
}
