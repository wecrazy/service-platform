package api_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"service-platform/internal/api/v1/routes"
	"service-platform/internal/config"
	"service-platform/internal/core/model"
	"service-platform/internal/pkg/fun"

	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type APITestSuite struct {
	suite.Suite
	router        *gin.Engine
	db            *gorm.DB
	redisClient   *redis.Client
	systemMonitor *fun.SystemResourceMonitor
	miniRedis     *miniredis.Miniredis
}

func (suite *APITestSuite) SetupTest() {
	gin.SetMode(gin.TestMode)

	// Load config from conf.yaml
	var err error
	err = config.LoadConfig()
	assert.NoError(suite.T(), err)

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

	// Setup system monitor
	suite.systemMonitor = &fun.SystemResourceMonitor{}

	// Setup router
	suite.router = gin.New()
	routes.StaticFile(suite.router)
	routes.HtmlRoutes(suite.db, suite.router, suite.redisClient, suite.systemMonitor)
}

func (suite *APITestSuite) TearDownTest() {
	if suite.db != nil {
		sqlDB, _ := suite.db.DB()
		sqlDB.Close()
	}
	if suite.miniRedis != nil {
		suite.miniRedis.Close()
	}
}

func (suite *APITestSuite) TestGetHello() {
	req, _ := http.NewRequest("GET", "/hello", nil)
	w := httptest.NewRecorder()
	suite.router.ServeHTTP(w, req)

	assert.Equal(suite.T(), http.StatusOK, w.Code)
	assert.Contains(suite.T(), w.Body.String(), `"message":"Hello, World!"`)
}

func (suite *APITestSuite) TestHealthCheck() {
	req, _ := http.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()
	suite.router.ServeHTTP(w, req)

	assert.Equal(suite.T(), http.StatusOK, w.Code)
	assert.Contains(suite.T(), w.Body.String(), `"status"`)
}

func TestAPISuite(t *testing.T) {
	suite.Run(t, new(APITestSuite))
}
