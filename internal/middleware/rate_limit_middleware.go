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

var rateLimiter *redis_rate.Limiter

// InitRateLimiter initializes the Redis rate limiter
func InitRateLimiter(redisClient *redis.Client) {
	rateLimiter = redis_rate.NewLimiter(redisClient)
}

// RateLimitMiddleware applies rate limiting to requests
func RateLimitMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !config.ServicePlatform.Get().RateLimit.Enabled || rateLimiter == nil {
			c.Next()
			return
		}

		// Use client IP as key for rate limiting
		key := c.ClientIP()

		// Apply rate limit
		result, err := rateLimiter.Allow(c.Request.Context(), key, redis_rate.PerSecond(config.ServicePlatform.Get().RateLimit.Requests))
		if err != nil {
			logrus.WithError(err).Error("Rate limiter error")
			fun.HandleAPIErrorSimple(c, http.StatusInternalServerError, "Rate limiting service unavailable")
			c.Abort()
			return
		}

		// Check if limit exceeded
		if result.Allowed == 0 {
			// Calculate retry-after time
			retryAfter := int(result.RetryAfter.Seconds())
			if retryAfter < 1 {
				retryAfter = 1
			}

			c.Header("Retry-After", strconv.Itoa(retryAfter))
			c.Header("X-RateLimit-Remaining", "0")
			c.Header("X-RateLimit-Reset", strconv.FormatInt(time.Now().Add(result.RetryAfter).Unix(), 10))

			logrus.WithFields(logrus.Fields{
				"ip":          key,
				"retry_after": retryAfter,
			}).Warn("Rate limit exceeded")

			fun.HandleAPIError(c, http.StatusTooManyRequests, "Rate limit exceeded", "Too many requests", "RATE_LIMIT_EXCEEDED")
			c.Abort()
			return
		}

		// Set headers for remaining requests
		c.Header("X-RateLimit-Remaining", strconv.Itoa(result.Remaining))
		c.Header("X-RateLimit-Reset", strconv.FormatInt(time.Now().Add(result.RetryAfter).Unix(), 10))

		c.Next()
	}
}
