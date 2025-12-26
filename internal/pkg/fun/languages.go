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
