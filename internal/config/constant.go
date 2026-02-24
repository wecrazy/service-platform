package config

// GlobalURL is the base URL path for the application.
const (
	GlobalURL   = "/"
	CacheMaxAge = 31536000 // seconds (1 year)
	APIURL      = "api/v1/"

	// Date format layouts using Go's reference time "Mon Jan 2 15:04:05 MST 2006".
	DateYYYYMMDD           = "2006-01-02"                  // YYYY-MM-DD
	DateYYYYMMDDHHMMSS     = "2006-01-02 15:04:05"         // YYYY-MM-DD HH:MM:SS
	DateYYYYMMDDHHMMSSMSTZ = "2006-01-02 15:04:05.000 MST" // YYYY-MM-DD HH:MM:SS.sss TZ
)

// ActiveDebug reflects the current debug mode from config.
var ActiveDebug = false || ServicePlatform.Get().App.Debug
