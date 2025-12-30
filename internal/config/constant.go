package config

const (
	GLOBAL_URL    = "/"
	CACHE_MAX_AGE = 31536000 // seconds (1 year)
	API_URL       = "api/v1/"
)

var (
	ACTIVE_DEBUG = false || GetConfig().App.Debug
)
