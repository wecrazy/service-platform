package fun

// // Map for translating weather descriptions
// var weatherTranslations = map[string]string{
// 	"clear sky":                       "Langit cerah",
// 	"few clouds":                      "Sedikit berawan",
// 	"scattered clouds":                "Berawan tipis",
// 	"broken clouds":                   "Berawan tebal",
// 	"overcast clouds":                 "Mendung",
// 	"rain":                            "Hujan",
// 	"light rain":                      "Hujan rintik-rintik",
// 	"moderate rain":                   "Hujan sedang",
// 	"heavy intensity rain":            "Hujan lebat",
// 	"very heavy rain":                 "Hujan sangat lebat",
// 	"extreme rain":                    "Hujan ekstrem",
// 	"freezing rain":                   "Hujan beku",
// 	"light intensity shower rain":     "Gerimis ringan",
// 	"shower rain":                     "Hujan deras sesekali",
// 	"heavy intensity shower rain":     "Hujan deras",
// 	"ragged shower rain":              "Hujan deras tidak merata",
// 	"thunderstorm":                    "Badai petir",
// 	"thunderstorm with light rain":    "Badai petir dengan hujan ringan",
// 	"thunderstorm with rain":          "Badai petir dengan hujan",
// 	"thunderstorm with heavy rain":    "Badai petir dengan hujan lebat",
// 	"light thunderstorm":              "Badai petir ringan",
// 	"heavy thunderstorm":              "Badai petir kuat",
// 	"ragged thunderstorm":             "Badai petir tidak merata",
// 	"thunderstorm with drizzle":       "Badai petir dengan gerimis",
// 	"thunderstorm with heavy drizzle": "Badai petir dengan gerimis lebat",
// 	"snow":                            "Salju",
// 	"light snow":                      "Salju ringan",
// 	"heavy snow":                      "Salju lebat",
// 	"sleet":                           "Hujan es",
// 	"light shower sleet":              "Gerimis hujan es",
// 	"shower sleet":                    "Hujan es deras",
// 	"light rain and snow":             "Hujan rintik-rintik dan salju",
// 	"rain and snow":                   "Hujan dan salju",
// 	"light shower snow":               "Hujan salju ringan",
// 	"shower snow":                     "Hujan salju",
// 	"heavy shower snow":               "Hujan salju lebat",
// 	"mist":                            "Berkabut",
// 	"smoke":                           "Berasap",
// 	"haze":                            "Kabut asap",
// 	"sand/dust whirls":                "Pusaran pasir/debu",
// 	"fog":                             "Kabut",
// 	"sand":                            "Badai pasir",
// 	"dust":                            "Badai debu",
// 	"volcanic ash":                    "Abu vulkanik",
// 	"squalls":                         "Angin kencang",
// 	"tornado":                         "Tornado",
// }

// ADD if needed
// func getWeatherEmoji(description string) string {
// 	emojiMap := map[string]string{
// 		"clear sky":                    "☀️", // Langit cerah
// 		"few clouds":                   "🌤️", // Sedikit berawan
// 		"scattered clouds":             "⛅",  // Berawan tipis
// 		"broken clouds":                "🌥️", // Berawan tebal
// 		"overcast clouds":              "☁️", // Mendung
// 		"light rain":                   "🌦️", // Hujan rintik-rintik
// 		"moderate rain":                "🌧️", // Hujan sedang
// 		"heavy intensity rain":         "🌧️", // Hujan lebat
// 		"very heavy rain":              "⛈️", // Hujan sangat lebat
// 		"extreme rain":                 "🌊",  // Hujan ekstrem
// 		"freezing rain":                "❄️", // Hujan beku
// 		"shower rain":                  "🌦️", // Hujan deras sesekali
// 		"thunderstorm":                 "⛈️", // Badai petir
// 		"thunderstorm with light rain": "⛈️",
// 		"thunderstorm with rain":       "⛈️",
// 		"thunderstorm with heavy rain": "⛈️",
// 		"snow":                         "❄️", // Salju
// 		"light snow":                   "🌨️", // Salju ringan
// 		"heavy snow":                   "❄️", // Salju lebat
// 		"mist":                         "🌫️", // Berkabut
// 		"smoke":                        "💨",  // Berasap
// 		"haze":                         "🌫️", // Kabut asap
// 		"fog":                          "🌫️", // Kabut
// 		"sand":                         "🏜️", // Badai pasir
// 		"dust":                         "💨",  // Badai debu
// 		"volcanic ash":                 "🌋",  // Abu vulkanik
// 		"squalls":                      "💨",  // Angin kencang
// 		"tornado":                      "🌪️", // Tornado
// 	}

// 	// Default emoji if the description is not found
// 	if emoji, exists := emojiMap[description]; exists {
// 		return emoji
// 	}
// 	return "🌎" // Default emoji if condition is unknown
// }

// func translateWeather(desc string) string {
// 	if indonesianDesc, found := weatherTranslations[desc]; found {
// 		return indonesianDesc
// 	}
// 	return desc // Default: return original if no translation is found
// }
