package config

const (
	GLOBAL_URL    = "/"
	CACHE_MAX_AGE = 31536000 // seconds (1 year)
	API_URL       = "api/v1/"

	// Date formats
	// Use Go's reference time "Mon Jan 2 15:04:05 MST 2006" to define layouts
	DATE_YYYY_MM_DD                = "2006-01-02"                  // YYYY-MM-DD
	DATE_YYYY_MM_DD_HH_MM_SS       = "2006-01-02 15:04:05"         // YYYY-MM-DD HH:MM:SS
	DATE_YYYY_MM_DD_HH_MM_SS_MS_TZ = "2006-01-02 15:04:05.000 MST" // YYYY-MM-DD HH:MM:SS.sss TZ
)

var (
	ACTIVE_DEBUG = false || GetConfig().App.Debug
)
