package middleware

import (
	"net/http"
	"strconv"
	"time"

	"service-platform/internal/config"
	"service-platform/internal/pkg/fun"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"github.com/go-redis/redis_rate/v9"
	"github.com/sirupsen/logrus"
)

// rateLimiter is a global variable that holds the Redis rate limiter instance, which is initialized in the InitRateLimiter function. It is used to enforce rate limits on incoming requests based on the configuration defined in the application's settings. The middleware functions use this rate limiter to check if a request exceeds the allowed limits and respond accordingly with appropriate headers and status codes.
var rateLimiter *redis_rate.Limiter

// InitRateLimiter initializes the Redis rate limiter
func InitRateLimiter(redisClient *redis.Client) {
	rateLimiter = redis_rate.NewLimiter(redisClient)
}

// RateLimitMiddleware applies rate limiting to requests
func RateLimitMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		cfg := config.ServicePlatform.Get().RateLimit
		if !cfg.Enabled || rateLimiter == nil {
			c.Next()
			return
		}

		limit := buildLimit(cfg.Requests, cfg.Period, cfg.Burst)
		if !enforceRateLimit(c, limit, "") {
			return
		}

		c.Next()
	}
}

// TwilioRateLimitMiddleware enforces the Twilio-specific rate limit configuration.
func TwilioRateLimitMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		cfg := config.ServicePlatform.Get().TwilioRateLimit
		if !cfg.Enabled || rateLimiter == nil {
			c.Next()
			return
		}

		limit := buildLimit(cfg.Requests, cfg.Period, cfg.Burst)
		if !enforceRateLimit(c, limit, "twilio") {
			return
		}

		c.Next()
	}
}

// buildLimit constructs a redis_rate.Limit based on the provided parameters, ensuring that default values are set for requests, periodSeconds, and burst if they are not provided or are invalid. This function is used to create the rate limit configuration for both general and Twilio-specific rate limiting.
func buildLimit(requests, periodSeconds, burst int) redis_rate.Limit {
	if requests <= 0 {
		requests = 1
	}
	if periodSeconds <= 0 {
		periodSeconds = 1
	}
	if burst <= 0 {
		burst = requests
	}

	return redis_rate.Limit{
		Rate:   requests,
		Period: time.Duration(periodSeconds) * time.Second,
		Burst:  burst,
	}
}

// enforceRateLimit checks if the incoming request exceeds the specified rate limit and responds with appropriate headers and status codes if the limit is exceeded. It uses the global rateLimiter to check the request count for the client's IP address, and it can apply a prefix to differentiate between general and specific rate limits (e.g., for Twilio). If the request is allowed, it sets headers to indicate the remaining requests and reset time; if not, it sets headers to indicate when the client can retry and responds with a 429 status code.
func enforceRateLimit(c *gin.Context, limit redis_rate.Limit, prefix string) bool {
	if limit.IsZero() {
		return true
	}

	ip := c.ClientIP()
	key := ip
	if prefix != "" {
		key = prefix + ":" + key
	}

	result, err := rateLimiter.Allow(c.Request.Context(), key, limit)
	if err != nil {
		logrus.WithError(err).WithField("prefix", prefix).Error("Rate limiter error")
		fun.HandleAPIErrorSimple(c, http.StatusInternalServerError, "Rate limiting service unavailable")
		c.Abort()
		return false
	}

	if result.Allowed == 0 {
		retryAfter := int(result.RetryAfter.Seconds())
		if retryAfter < 1 {
			retryAfter = 1
		}

		c.Header("Retry-After", strconv.Itoa(retryAfter))
		c.Header("X-RateLimit-Remaining", "0")
		c.Header("X-RateLimit-Reset", strconv.FormatInt(time.Now().Add(result.RetryAfter).Unix(), 10))

		logrus.WithFields(logrus.Fields{
			"ip":          ip,
			"prefix":      prefix,
			"retry_after": retryAfter,
		}).Warn("Rate limit exceeded")

		fun.HandleAPIError(c, http.StatusTooManyRequests, "Rate limit exceeded", "Too many requests", "RATE_LIMIT_EXCEEDED")
		c.Abort()
		return false
	}

	c.Header("X-RateLimit-Remaining", strconv.Itoa(result.Remaining))
	c.Header("X-RateLimit-Reset", strconv.FormatInt(time.Now().Add(result.RetryAfter).Unix(), 10))

	return true
}
