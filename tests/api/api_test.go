package api_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"service-platform/internal/api/v1/routes"
	"service-platform/internal/config"
	"service-platform/internal/core/model"
	"service-platform/internal/middleware"
	"service-platform/pkg/fun"

	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// APITestSuite provides a test suite for API endpoints testing.
// It includes setup for database, Redis, and Gin router for comprehensive API testing.
type APITestSuite struct {
	suite.Suite
	router        *gin.Engine                // Gin router for handling HTTP requests
	db            *gorm.DB                   // Database connection for testing
	redisClient   *redis.Client              // Redis client for caching and rate limiting tests
	systemMonitor *fun.SystemResourceMonitor // System monitor for resource tracking
	miniRedis     *miniredis.Miniredis       // Mini Redis instance for testing
}

// SetupTest initializes the test suite with database, Redis, and router setup.
// It creates an in-memory SQLite database, mini Redis instance, and configures the Gin router.
func (suite *APITestSuite) SetupTest() {
	gin.SetMode(gin.TestMode)

	// Load config from conf.yaml
	var err error
	config.ServicePlatform.MustInit("service-platform") // Load config with name "service-platform.%s.yaml"
	if !config.ServicePlatform.IsLoaded() {
		err = errors.New("failed to load configuration")
		assert.NoError(suite.T(), err, "Config should be loaded successfully")
	}

	// Setup test database (SQLite)
	suite.db, err = gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	assert.NoError(suite.T(), err)

	// Migrate models
	err = suite.db.AutoMigrate(
		&model.Users{},
		&model.UserStatus{},
		&model.UserPasswordChangeLog{},
		&model.Role{},
		&model.RolePrivilege{},
		&model.Feature{},
		&model.LogActivity{},
		&model.Language{},
		&model.BadWord{},
		&model.AppConfig{},
		&model.WAUsers{},
		&model.WhatsappMessageAutoReply{},
		// Skip whatsnyan models for now due to migration issues
		// &whatsnyanmodel.WhatsAppGroup{},
		// &whatsnyanmodel.WhatsAppGroupParticipant{},
		// &whatsnyanmodel.WhatsAppMsg{},
		// &whatsnyanmodel.WhatsAppIncomingMsg{},
	)
	assert.NoError(suite.T(), err)

	// Setup test Redis
	suite.miniRedis, err = miniredis.Run()
	assert.NoError(suite.T(), err)
	suite.redisClient = redis.NewClient(&redis.Options{
		Addr: suite.miniRedis.Addr(),
	})

	// Initialize rate limiter
	middleware.InitRateLimiter(suite.redisClient)

	// Setup system monitor
	suite.systemMonitor = &fun.SystemResourceMonitor{}

	// Setup router
	suite.router = gin.New()
	routes.StaticFile(suite.router)
	routes.HTMLRoutes(suite.db, suite.router, suite.redisClient, suite.systemMonitor)
}

// TearDownTest cleans up test resources including database connections and Redis instances.
func (suite *APITestSuite) TearDownTest() {
	if suite.db != nil {
		sqlDB, _ := suite.db.DB()
		sqlDB.Close()
	}
	if suite.miniRedis != nil {
		suite.miniRedis.Close()
	}
}

// TestGetHello tests the /hello endpoint to ensure it returns a proper greeting message.
func (suite *APITestSuite) TestGetHello() {
	req, _ := http.NewRequest("GET", "/hello", nil)
	w := httptest.NewRecorder()
	suite.router.ServeHTTP(w, req)

	assert.Equal(suite.T(), http.StatusOK, w.Code)
	assert.Contains(suite.T(), w.Body.String(), `"message":"Hello, World!"`)
}

// TestHealthCheck tests the /health endpoint to ensure the service is running properly.
func (suite *APITestSuite) TestHealthCheck() {
	req, _ := http.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()
	suite.router.ServeHTTP(w, req)

	assert.Equal(suite.T(), http.StatusOK, w.Code)
	assert.Contains(suite.T(), w.Body.String(), `"status"`)
}

// TestRateLimitBelowLimit tests that requests within the rate limit are processed normally.
// It verifies that rate limit headers are present and requests succeed.
func (suite *APITestSuite) TestRateLimitBelowLimit() {
	// Make requests up to the limit (100 per minute)
	for i := 0; i < 10; i++ { // Test with fewer to keep test fast
		req, _ := http.NewRequest("GET", "/health", nil)
		w := httptest.NewRecorder()
		suite.router.ServeHTTP(w, req)

		assert.Equal(suite.T(), http.StatusOK, w.Code)
		assert.Contains(suite.T(), w.Body.String(), `"status"`)
		// Check rate limit headers
		assert.NotEmpty(suite.T(), w.Header().Get("X-RateLimit-Remaining"))
		assert.NotEmpty(suite.T(), w.Header().Get("X-RateLimit-Reset"))
	}
}

// TestRateLimitOverLimit tests that requests exceeding the rate limit are properly rejected.
// It verifies that rate limiting returns 429 status with appropriate error messages and headers.
func (suite *APITestSuite) TestRateLimitOverLimit() {
	// First, exhaust the rate limit by making many requests quickly
	for i := 0; i < 110; i++ { // Exceed the 100 limit
		req, _ := http.NewRequest("GET", "/health", nil)
		w := httptest.NewRecorder()
		suite.router.ServeHTTP(w, req)
	}

	// Now test that the next request is rate limited
	req, _ := http.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()
	suite.router.ServeHTTP(w, req)

	assert.Equal(suite.T(), http.StatusTooManyRequests, w.Code)
	assert.Contains(suite.T(), w.Body.String(), `"error":"Rate limit exceeded"`)
	assert.Equal(suite.T(), "0", w.Header().Get("X-RateLimit-Remaining"))
	assert.NotEmpty(suite.T(), w.Header().Get("Retry-After"))
}

// TestAPISuite runs the API test suite.
func TestAPISuite(t *testing.T) {
	suite.Run(t, new(APITestSuite))
}
