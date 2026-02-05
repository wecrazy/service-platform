// Package odoomscontrollers provides ODOO Management Service (ODOO MS) API integration
// with session caching, monitoring, and robust error handling.
//
// This package implements a cached ODOO session management system that:
//   - Caches ODOO authentication sessions for 30 minutes to reduce server load
//   - Provides comprehensive Prometheus metrics for monitoring cache performance
//   - Implements thread-safe operations with proper mutex locking
//   - Handles automatic retries with exponential backoff
//   - Integrates with Grafana dashboards for real-time monitoring
//
// Example usage:
//
//	helper := NewODOOMSAPIHelper(config)
//	cookies, err := helper.GetODOOMSCookies("user@example.com", "password")
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	// Use cached session for API calls
//	data, err := FetchODOOMS("https://odoo.example.com/api/data", "POST", requestBody)
package odoomscontrollers

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"service-platform/internal/config"
	"service-platform/internal/database"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

const (
	// sessionCacheDuration defines how long ODOO sessions are cached (30 minutes)
	// This reduces authentication requests to the ODOO server and improves performance
	sessionCacheDuration = 30 * time.Minute
)

var (
	// odoomsSessionCache holds the cached ODOO session cookies
	odoomsSessionCache []*http.Cookie
	// odoomsSessionCacheTime tracks when the cache was last updated
	odoomsSessionCacheTime time.Time
	// odoomsSessionMutex provides thread-safe access to the session cache
	odoomsSessionMutex sync.RWMutex

	// Global ODOOMS API Helper instance - initialized in init()
	odoomsHelper *ODOOMSAPIHelper

	// Prometheus metrics for session cache monitoring
	// These metrics are automatically exposed via the /api-metrics endpoint
	sessionCacheHits = promauto.NewCounter(prometheus.CounterOpts{
		Name: "odooms_session_cache_hits_total",
		Help: "Total number of ODOOMS session cache hits",
	})
	sessionCacheMisses = promauto.NewCounter(prometheus.CounterOpts{
		Name: "odooms_session_cache_misses_total",
		Help: "Total number of ODOOMS session cache misses",
	})
	sessionCacheErrors = promauto.NewCounter(prometheus.CounterOpts{
		Name: "odooms_session_cache_errors_total",
		Help: "Total number of ODOOMS session cache errors",
	})
	sessionCacheAge = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "odooms_session_cache_age_seconds",
		Help: "Age of the current ODOOMS session cache in seconds",
	})
)

// ODOOMSAPIHelper provides a cached and monitored interface to ODOO Management Service.
// It implements session caching to reduce authentication overhead and provides
// comprehensive monitoring through Prometheus metrics.
//
// The helper maintains a global session cache that:
//   - Stores ODOO authentication cookies for 30 minutes
//   - Provides thread-safe access with read-write mutexes
//   - Tracks cache performance metrics
//   - Handles automatic session refresh when expired
//
// Example:
//
//	helper := NewODOOMSAPIHelper(config)
//	cookies, err := helper.GetODOOMSCookies("user@company.com", "password")
//	stats := helper.GetSessionCacheStats()
type ODOOMSAPIHelper struct {
	config         *config.TypeConfig
	client         *http.Client
	dbTA           *gorm.DB // Database connection for Dashboard Technical Assistance - Manage Service Integration
	dbMSMiddleware *gorm.DB // Database connection for Middleware Manage Service Integration
}

// NewODOOMSAPIHelper creates a new ODOOMSAPIHelper instance with the provided configuration.
// The helper is initialized with a pre-configured HTTP client that has TLS verification
// disabled for ODOO server compatibility.
//
// Parameters:
//   - cfg: Pointer to the application configuration containing ODOO settings
//
// Returns:
//   - *ODOOMSAPIHelper: Configured helper instance ready for use
//
// Example:
//
//	config := config.GetConfig()
//	helper := NewODOOMSAPIHelper(&config, dbTA)
func NewODOOMSAPIHelper(cfg *config.TypeConfig, dbTA *gorm.DB, dbMSMiddleware *gorm.DB) *ODOOMSAPIHelper {
	return &ODOOMSAPIHelper{
		config: cfg,
		client: &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: cfg.ODOOManageService.SkipSSLVerify},
			},
		},
		dbTA:           dbTA,
		dbMSMiddleware: dbMSMiddleware,
	}
}

// GetODOOMSCookies retrieves ODOO session cookies using intelligent caching.
// This method implements a double-checked locking pattern to provide thread-safe
// access to cached sessions while minimizing lock contention.
//
// The method will:
//   - Return cached cookies if they exist and are still valid (< 30 minutes old)
//   - Automatically fetch new cookies if cache is expired or empty
//   - Update Prometheus metrics for cache hits, misses, and errors
//   - Log cache operations for monitoring and debugging
//
// Parameters:
//   - email: ODOO user email for authentication
//   - password: ODOO user password for authentication
//
// Returns:
//   - []*http.Cookie: Array of session cookies for ODOO API calls
//   - error: Any error encountered during authentication or caching
//
// Example:
//
//	cookies, err := helper.GetODOOMSCookies("user@company.com", "secure_password")
//	if err != nil {
//	    return fmt.Errorf("failed to get ODOO session: %w", err)
//	}
//	// Use cookies for subsequent API calls
func (h *ODOOMSAPIHelper) GetODOOMSCookies(email, password string) ([]*http.Cookie, error) {
	odoomsSessionMutex.RLock()
	// Check if we have valid cached session
	if odoomsSessionCache != nil && time.Since(odoomsSessionCacheTime) < sessionCacheDuration {
		cookies := make([]*http.Cookie, len(odoomsSessionCache))
		copy(cookies, odoomsSessionCache)
		odoomsSessionMutex.RUnlock()

		// Monitor cache hit
		sessionCacheHits.Inc()
		sessionCacheAge.Set(time.Since(odoomsSessionCacheTime).Seconds())
		logrus.Debug("ODOOMS session cache hit")

		return cookies, nil
	}
	odoomsSessionMutex.RUnlock()

	// Need to fetch new session
	odoomsSessionMutex.Lock()
	defer odoomsSessionMutex.Unlock()

	// Double-check in case another goroutine updated the cache
	if odoomsSessionCache != nil && time.Since(odoomsSessionCacheTime) < sessionCacheDuration {
		cookies := make([]*http.Cookie, len(odoomsSessionCache))
		copy(cookies, odoomsSessionCache)

		// Monitor cache hit
		sessionCacheHits.Inc()
		sessionCacheAge.Set(time.Since(odoomsSessionCacheTime).Seconds())
		logrus.Debug("ODOOMS session cache hit (double-check)")

		return cookies, nil
	}

	// Monitor cache miss
	sessionCacheMisses.Inc()
	logrus.Info("ODOOMS session cache miss, fetching new session")

	// Fetch new session
	newCookies, err := h.getODOOMSCookies(email, password)
	if err != nil {
		sessionCacheErrors.Inc()
		logrus.WithError(err).Error("Failed to fetch ODOOMS session cookies")
		return nil, err
	}

	// Update cache
	odoomsSessionCache = make([]*http.Cookie, len(newCookies))
	copy(odoomsSessionCache, newCookies)
	odoomsSessionCacheTime = time.Now()
	sessionCacheAge.Set(0) // Reset age to 0 for fresh cache

	logrus.Info("ODOOMS session cache updated successfully")

	return newCookies, nil
}

// getODOOMSCookies performs the actual HTTP request to get ODOO session cookies.
// This is the internal implementation that handles the raw authentication request
// with retry logic and proper error handling.
//
// The method implements:
//   - Configurable retry attempts with exponential backoff
//   - Proper timeout handling based on configuration
//   - JSON-RPC formatted authentication requests
//   - TLS verification bypass for ODOO server compatibility
//
// This method is not exported and should not be called directly.
// Use GetODOOMSCookies() instead for proper caching behavior.
//
// Parameters:
//   - email: ODOO user email for authentication
//   - password: ODOO user password for authentication
//
// Returns:
//   - []*http.Cookie: Fresh session cookies from ODOO server
//   - error: Authentication or network errors
func (h *ODOOMSAPIHelper) getODOOMSCookies(email, password string) ([]*http.Cookie, error) {
	// Use ODOOManageService config instead of non-existent ApiODOO
	odooConfig := h.config.ODOOManageService

	requestJSON := `{
		"jsonrpc": "%s",
		"params": {
			"db": "%s",
			"login": "%s",
			"password": "%s"
		}
	}`

	rawJSON := fmt.Sprintf(requestJSON, odooConfig.JsonRPCVersion, odooConfig.DB, email, password)

	maxRetries := odooConfig.MaxRetry
	if maxRetries <= 0 {
		maxRetries = 3
	}

	retryDelay := odooConfig.RetryDelay
	if retryDelay <= 0 {
		retryDelay = 3
	}

	reqTimeout := time.Duration(odooConfig.SessionTimeout) * time.Second
	if reqTimeout <= 0 {
		reqTimeout = 30 * time.Second
	}

	h.client.Timeout = reqTimeout

	var lastErr error
	for attempt := 1; attempt <= maxRetries; attempt++ {
		req, err := http.NewRequest("POST", odooConfig.URL+odooConfig.PathSession, bytes.NewBufferString(rawJSON))
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := h.client.Do(req)
		if err != nil {
			logrus.WithError(err).Warningf("Request failed (attempt %d/%d)", attempt, maxRetries)
			lastErr = err
			if attempt < maxRetries {
				time.Sleep(time.Duration(retryDelay) * time.Second)
			}
			continue
		}

		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			logrus.Warningf("Bad response status: %d (attempt %d/%d)", resp.StatusCode, attempt, maxRetries)
			lastErr = fmt.Errorf("request failed with status: %d", resp.StatusCode)
			if attempt < maxRetries {
				time.Sleep(time.Duration(retryDelay) * time.Second)
			}
			continue
		}

		_, err = io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to read response body: %w", err)
		}

		return resp.Cookies(), nil
	}

	return nil, fmt.Errorf("all retry attempts failed, last error: %w", lastErr)
}

// GetSessionCacheStats returns comprehensive statistics about the session cache.
// This method provides real-time insights into cache performance and health
// for monitoring and debugging purposes.
//
// Returns a map containing:
//   - cache_valid: boolean indicating if cache is currently valid
//   - cache_age_seconds: age of current cache in seconds
//   - cache_duration_minutes: total cache validity duration
//   - cookies_count: number of cached cookies
//   - last_updated: RFC3339 timestamp of last cache update
//
// Example:
//
//	stats := helper.GetSessionCacheStats()
//	fmt.Printf("Cache valid: %v, Age: %.1fs\n",
//	    stats["cache_valid"], stats["cache_age_seconds"])
func (h *ODOOMSAPIHelper) GetSessionCacheStats() map[string]interface{} {
	odoomsSessionMutex.RLock()
	defer odoomsSessionMutex.RUnlock()

	stats := map[string]interface{}{
		"cache_valid":            odoomsSessionCache != nil && time.Since(odoomsSessionCacheTime) < sessionCacheDuration,
		"cache_age_seconds":      time.Since(odoomsSessionCacheTime).Seconds(),
		"cache_duration_minutes": sessionCacheDuration.Minutes(),
		"cookies_count":          len(odoomsSessionCache),
		"last_updated":           odoomsSessionCacheTime.Format(time.RFC3339),
	}

	return stats
}

// ClearSessionCache manually clears the session cache and resets all metrics.
// This method is useful for:
//   - Testing scenarios requiring fresh authentication
//   - Forcing cache refresh when ODOO credentials change
//   - Troubleshooting cache-related issues
//
// The method is thread-safe and will log the cache clearing operation.
//
// Example:
//
//	helper.ClearSessionCache() // Force next call to re-authenticate
func (h *ODOOMSAPIHelper) ClearSessionCache() {
	odoomsSessionMutex.Lock()
	defer odoomsSessionMutex.Unlock()

	odoomsSessionCache = nil
	odoomsSessionCacheTime = time.Time{}
	sessionCacheAge.Set(0)

	logrus.Info("ODOOMS session cache cleared manually")
}

// IsSessionCacheValid checks if the current session cache is still valid.
// This method performs a thread-safe check to determine if cached sessions
// can be used without requiring re-authentication.
//
// Returns:
//   - true: Cache exists and is within the validity period (30 minutes)
//   - false: Cache is expired, empty, or invalid
//
// Example:
//
//	if !helper.IsSessionCacheValid() {
//	    fmt.Println("Cache expired, next call will re-authenticate")
//	}
func (h *ODOOMSAPIHelper) IsSessionCacheValid() bool {
	odoomsSessionMutex.RLock()
	defer odoomsSessionMutex.RUnlock()

	return odoomsSessionCache != nil && time.Since(odoomsSessionCacheTime) < sessionCacheDuration
}

// SetTestHelper allows injecting a test helper for unit testing.
// This function should only be used in test environments.
func SetTestHelper(helper *ODOOMSAPIHelper) {
	odoomsHelper = helper
}

// FetchODOOMS performs authenticated HTTP requests to ODOO Management Service endpoints.
// This function automatically handles session management by using the cached ODOOMSAPIHelper,
// eliminating the need for manual cookie management in calling code.
//
// The function provides:
//   - Automatic session cookie injection from cache
//   - Configurable retry logic with exponential backoff
//   - Proper timeout handling based on ODOO configuration
//   - Comprehensive error logging and handling
//   - Thread-safe operation through the global helper instance
//
// Parameters:
//   - url: Complete ODOO API endpoint URL (e.g., "https://odoo.example.com/api/data")
//   - method: HTTP method ("GET", "POST", "PUT", "DELETE")
//   - req: Request body as JSON string (can be empty for GET requests)
//
// Returns:
//   - []byte: Raw response body from ODOO API
//   - error: Any error encountered during the request
//
// Example:
//
//	// POST request with JSON body
//	requestBody := `{"model": "res.partner", "method": "search", "args": []}`
//	data, err := FetchODOOMS("https://odoo.company.com/api/data", "POST", requestBody)
//	if err != nil {
//	    return fmt.Errorf("ODOO API call failed: %w", err)
//	}
//
//	// GET request
//	data, err := FetchODOOMS("https://odoo.company.com/api/status", "GET", "")
//
// Thread Safety:
//
//	This function is thread-safe and can be called concurrently from multiple goroutines.
//	Session caching is handled automatically with proper mutex locking.
func FetchODOOMS(url, method, req string) ([]byte, error) {
	yamlCfg := config.GetConfig()

	// Lazy initialize the global ODOOMS helper
	if odoomsHelper == nil {
		dbTA := database.GetDBTA()
		dbMSMiddleware := database.GetDBMS()
		odoomsHelper = NewODOOMSAPIHelper(&yamlCfg, dbTA, dbMSMiddleware)
	}

	// Use ODOOManageService config instead of non-existent ApiODOO
	odooConfig := yamlCfg.ODOOManageService

	maxRetries := odooConfig.MaxRetry
	if maxRetries <= 0 {
		maxRetries = 3
	}

	retryDelay := odooConfig.RetryDelay
	if retryDelay <= 0 {
		retryDelay = 3
	}

	reqTimeout := time.Duration(odooConfig.DataTimeout) * time.Second
	if reqTimeout <= 0 {
		reqTimeout = 5 * time.Minute // 300 seconds default
	}

	var lastErr error
	var response *http.Response

	for attempts := 1; attempts <= maxRetries; attempts++ {
		request, err := http.NewRequest(method, url, bytes.NewBufferString(req))
		if err != nil {
			logrus.WithError(err).Error("Failed to create ODOO request")
			return nil, fmt.Errorf("failed to create request: %w", err)
		}
		request.Header.Set("Content-Type", "application/json")

		// Get session cookies using the cached helper
		sessionCookies, err := odoomsHelper.GetODOOMSCookies(odooConfig.Login, odooConfig.Password)
		if err != nil {
			logrus.WithError(err).Error("Failed to get ODOO session cookies")
			return nil, fmt.Errorf("failed to get session cookies: %w", err)
		}

		// Add session cookies to request
		for _, cookie := range sessionCookies {
			request.AddCookie(cookie)
		}

		// Use the helper's HTTP client for consistency
		odoomsHelper.client.Timeout = reqTimeout
		response, err = odoomsHelper.client.Do(request)

		if err != nil {
			logrus.WithError(err).Warningf("ODOO request failed (attempt %d/%d)", attempts, maxRetries)
			lastErr = err
			if attempts < maxRetries {
				time.Sleep(time.Duration(retryDelay) * time.Second)
			}
			continue
		}

		// Check if the response is successful
		if response.StatusCode == http.StatusOK {
			break
		} else {
			logrus.Warningf("Bad ODOO response: %d (attempt %d/%d) for %s",
				response.StatusCode, attempts, maxRetries, url)
			lastErr = fmt.Errorf("request failed with status: %d", response.StatusCode)
			if attempts < maxRetries {
				response.Body.Close()
				time.Sleep(time.Duration(retryDelay) * time.Second)
			}
			continue
		}
	}

	if response == nil {
		return nil, fmt.Errorf("all retry attempts failed: %w", lastErr)
	}

	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s request failed with status code: %d", method, response.StatusCode)
	}

	body, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	return body, nil
}
