package twilio_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"service-platform/internal/api/v1/controllers"
	"service-platform/internal/api/v1/dto"
	"service-platform/internal/config"
	"service-platform/internal/middleware"

	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
)

// TestTwilioWebhookRateLimit ensures the Twilio webhook returns 429 once the configured limit is exceeded.
func TestTwilioWebhookRateLimit(t *testing.T) {
	gin.SetMode(gin.TestMode)

	config.ServicePlatform.MustInit("service-platform")
	if !config.ServicePlatform.IsLoaded() {
		t.Fatal("config must be loaded for Twilio rate limit test")
	}

	mini, err := miniredis.Run()
	if err != nil {
		t.Fatalf("failed to start miniredis: %v", err)
	}
	defer mini.Close()

	redisClient := redis.NewClient(&redis.Options{Addr: mini.Addr()})
	middleware.InitRateLimiter(redisClient)

	router := gin.New()
	twilioGroup := router.Group("", middleware.TwilioRateLimitMiddleware())
	twilioGroup.POST("/twilio_reply", controllers.HandleTwilioWhatsAppWebhook(nil))

	cfg := config.ServicePlatform.Get()
	limit := cfg.TwilioRateLimit.Requests
	if limit <= 0 {
		t.Fatalf("twilio rate limit not configured: %d", limit)
	}
	t.Logf("using Twilio webhook limit %d requests", limit)

	form := url.Values{
		"From":       {"whatsapp:+14155238886"},
		"To":         {"whatsapp:+6285173207755"},
		"Body":       {"Hello"},
		"MessageSid": {"SM123"},
	}
	payload := form.Encode()

	// Send enough requests until we encounter a 429 or exhaust a reasonable attempt count
	maxAttempts := limit + 5
	var seen429 bool
	for i := 0; i < maxAttempts; i++ {
		req, _ := http.NewRequest("POST", "/twilio_reply", strings.NewReader(payload))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)

		if w.Code == http.StatusTooManyRequests {
			seen429 = true
			if w.Header().Get("X-RateLimit-Remaining") != "0" {
				t.Fatalf("expected remaining header to be 0, got %s", w.Header().Get("X-RateLimit-Remaining"))
			}
			if w.Header().Get("Retry-After") == "" {
				t.Fatal("expected Retry-After header")
			}

			var apiErr dto.APIErrorResponse
			if err := json.Unmarshal(w.Body.Bytes(), &apiErr); err != nil {
				t.Fatalf("failed to decode error response: %v", err)
			}
			if apiErr.Error != "Rate limit exceeded" {
				t.Fatalf("unexpected error message: %s", apiErr.Error)
			}
			if apiErr.Code != "RATE_LIMIT_EXCEEDED" {
				t.Fatalf("unexpected error code: %s", apiErr.Code)
			}
			break
		}

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200 before limit, got %d", w.Code)
		}
	}

	if !seen429 {
		t.Fatalf("expected to hit rate limit within %d requests", maxAttempts)
	}
}
