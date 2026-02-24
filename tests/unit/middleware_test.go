package unit

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"service-platform/internal/config"
	"service-platform/internal/middleware"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// ── SecurityControlMiddleware ───────────────────────────────────────────────

func TestSecurityControlMiddleware(t *testing.T) {
	r := gin.New()
	r.Use(middleware.SecurityControlMiddleware())
	r.GET("/test", func(c *gin.Context) { c.String(http.StatusOK, "ok") })

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/test", nil)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	checks := map[string]string{
		"Content-Security-Policy":   "frame-ancestors 'none'",
		"X-Frame-Options":           "DENY",
		"X-Content-Type-Options":    "nosniff",
		"Strict-Transport-Security": "max-age=16070400; includeSubDomains",
		"Referrer-Policy":           "no-referrer",
		"X-Xss-Protection":          "1; mode=block",
	}
	for header, want := range checks {
		assert.Equal(t, want, w.Header().Get(header), "header: %s", header)
	}
}

// ── CacheControlMiddleware ──────────────────────────────────────────────────

func TestCacheControlMiddleware_StaticAsset(t *testing.T) {
	r := gin.New()
	r.Use(middleware.CacheControlMiddleware())
	r.GET("/assets/*path", func(c *gin.Context) { c.String(http.StatusOK, "asset") })

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/assets/app.js", nil)
	r.ServeHTTP(w, req)

	want := fmt.Sprintf("public, max-age=%d", config.CacheMaxAge)
	assert.Equal(t, want, w.Header().Get("Cache-Control"))
	assert.Equal(t, "SWS", w.Header().Get("Server"))
}

func TestCacheControlMiddleware_NonStaticPath(t *testing.T) {
	r := gin.New()
	r.Use(middleware.CacheControlMiddleware())
	r.GET("/api/users", func(c *gin.Context) { c.String(http.StatusOK, "users") })

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/users", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, "no-store", w.Header().Get("Cache-Control"))
}

func TestCacheControlMiddleware_PostIgnored(t *testing.T) {
	r := gin.New()
	r.Use(middleware.CacheControlMiddleware())
	r.POST("/assets/upload", func(c *gin.Context) { c.String(http.StatusOK, "ok") })

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/assets/upload", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, "no-store", w.Header().Get("Cache-Control"))
}

func TestCacheControlMiddleware_WebPathExcluded(t *testing.T) {
	r := gin.New()
	r.Use(middleware.CacheControlMiddleware())
	r.GET("/web/*path", func(c *gin.Context) { c.String(http.StatusOK, "web") })

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/web/index.html", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, "no-store", w.Header().Get("Cache-Control"))
}

// ── SanitizeMiddleware ──────────────────────────────────────────────────────

func TestSanitizeMiddleware_QueryParams(t *testing.T) {
	r := gin.New()
	r.Use(middleware.SanitizeMiddleware())
	r.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, c.Query("q"))
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/test?q=<script>alert(1)</script>", nil)
	r.ServeHTTP(w, req)

	assert.NotContains(t, w.Body.String(), "<script>", "XSS should be sanitized")
}

func TestSanitizeMiddleware_JSONBody(t *testing.T) {
	r := gin.New()
	r.Use(middleware.SanitizeMiddleware())
	r.POST("/test", func(c *gin.Context) {
		bodyBytes, _ := io.ReadAll(c.Request.Body)
		c.String(http.StatusOK, string(bodyBytes))
	})

	payload := map[string]string{"name": "<img src=x onerror=alert(1)>"}
	payloadBytes, _ := json.Marshal(payload)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/test", bytes.NewReader(payloadBytes))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.NotContains(t, w.Body.String(), "<img", "raw <img should be sanitized")
}

func TestSanitizeMiddleware_SafeContentPasses(t *testing.T) {
	r := gin.New()
	r.Use(middleware.SanitizeMiddleware())
	r.POST("/test", func(c *gin.Context) {
		bodyBytes, _ := io.ReadAll(c.Request.Body)
		c.String(http.StatusOK, string(bodyBytes))
	})

	payload := map[string]string{"name": "John Doe"}
	payloadBytes, _ := json.Marshal(payload)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/test", bytes.NewReader(payloadBytes))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Contains(t, w.Body.String(), "John Doe")
}

// ── SanitizeCsvMiddleware ───────────────────────────────────────────────────

func TestSanitizeCsvMiddleware_FormulaInjection(t *testing.T) {
	r := gin.New()
	r.Use(middleware.SanitizeCsvMiddleware())
	r.POST("/test", func(c *gin.Context) {
		bodyBytes, _ := io.ReadAll(c.Request.Body)
		c.String(http.StatusOK, string(bodyBytes))
	})

	payload := map[string]string{"cell": "=cmd|'/C calc'!A0"}
	payloadBytes, _ := json.Marshal(payload)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/test", bytes.NewReader(payloadBytes))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Contains(t, w.Body.String(), "'=", "CSV injection should be prefixed")
}

func TestSanitizeCsvMiddleware_GetPassesThrough(t *testing.T) {
	r := gin.New()
	r.Use(middleware.SanitizeCsvMiddleware())
	r.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/test?q==evil", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

// ── RateLimitMiddleware (without Redis) ─────────────────────────────────────

func TestRateLimitMiddleware_DisabledWithoutInit(t *testing.T) {
	// Without InitRateLimiter, the middleware should pass through
	r := gin.New()
	r.Use(middleware.RateLimitMiddleware())
	r.GET("/test", func(c *gin.Context) { c.String(http.StatusOK, "ok") })

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/test", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestTwilioRateLimitMiddleware_DisabledWithoutInit(t *testing.T) {
	r := gin.New()
	r.Use(middleware.TwilioRateLimitMiddleware())
	r.GET("/test", func(c *gin.Context) { c.String(http.StatusOK, "ok") })

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/test", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

// ── WebSession ──────────────────────────────────────────────────────────────

func TestGetWebSession_NotNil(t *testing.T) {
	ws := middleware.GetWebSession()
	assert.NotNil(t, ws)

	// Store and load roundtrip
	ws.Store("test-key", "test-value")
	val, ok := ws.Load("test-key")
	assert.True(t, ok)
	assert.Equal(t, "test-value", val)

	// Cleanup
	ws.Delete("test-key")
	_, ok = ws.Load("test-key")
	assert.False(t, ok)
}
