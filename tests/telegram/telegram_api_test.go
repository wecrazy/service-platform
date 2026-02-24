package telegram_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"service-platform/internal/api/v1/dto"
	"service-platform/internal/api/v1/routes"
	"service-platform/internal/config"
	"service-platform/pkg/fun"

	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

const (
	// TelegramTabAccess is the test access token used for Telegram API endpoint testing
	TelegramTabAccess = "testaccess"
)

// TelegramAPITestSuite provides a test suite for Telegram API endpoint testing.
// It includes setup for HTTP router and test server to test REST endpoints.
type TelegramAPITestSuite struct {
	suite.Suite
	router        *gin.Engine                // Gin router for testing endpoints
	db            *gorm.DB                   // Database connection for testing
	redisClient   *redis.Client              // Redis client for caching and rate limiting tests
	systemMonitor *fun.SystemResourceMonitor // System monitor for resource tracking
	miniRedis     *miniredis.Miniredis       // Mini Redis instance for testing
}

// SetupTest initializes the Telegram API test suite by setting up the Gin router
// with Telegram routes and required dependencies.
func (suite *TelegramAPITestSuite) SetupTest() {
	gin.SetMode(gin.TestMode)

	config.ServicePlatform.MustInit("service-platform") // Load config with name "service-platform.%s.yaml"
	var err error
	if !config.ServicePlatform.IsLoaded() {
		err = errors.New("failed to load configuration")
		assert.NoError(suite.T(), err, "Config should be loaded successfully")
	}

	// Setup test database (SQLite)
	suite.db, err = gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	assert.NoError(suite.T(), err)

	// Setup test Redis
	suite.miniRedis, err = miniredis.Run()
	assert.NoError(suite.T(), err)
	suite.redisClient = redis.NewClient(&redis.Options{
		Addr: suite.miniRedis.Addr(),
	})

	// Setup system monitor
	suite.systemMonitor = &fun.SystemResourceMonitor{}

	// Setup router with all routes including Telegram
	suite.router = gin.New()
	routes.StaticFile(suite.router)
	routes.HTMLRoutes(suite.db, suite.router, suite.redisClient, suite.systemMonitor)
}

// TearDownTest cleans up the Telegram API test suite.
func (suite *TelegramAPITestSuite) TearDownTest() {
	if suite.db != nil {
		sqlDB, _ := suite.db.DB()
		sqlDB.Close()
	}
	if suite.miniRedis != nil {
		suite.miniRedis.Close()
	}
}

// TestSendMessageAPI tests the REST API endpoint for sending text messages.
func (suite *TelegramAPITestSuite) TestSendMessageAPI() {
	reqBody := dto.SendTelegramMessageRequest{
		ChatID:    "123456789",
		Text:      "Test message from API test",
		ParseMode: "Markdown",
	}

	jsonData, _ := json.Marshal(reqBody)
	req, _ := http.NewRequest("POST", "/"+config.APIURL+TelegramTabAccess+"/tab-telegram/send_message", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	suite.router.ServeHTTP(w, req)

	// Since the service may not be running, we just check that the endpoint exists
	// and returns a valid HTTP status (could be 200 or error status)
	assert.True(suite.T(), w.Code != http.StatusNotFound, "Endpoint should exist (may return 401 for auth required)")
}

// TestSendVoiceAPI tests the REST API endpoint for sending voice messages.
func (suite *TelegramAPITestSuite) TestSendVoiceAPI() {
	reqBody := dto.SendTelegramVoiceRequest{
		ChatID:    "123456789",
		Voice:     "https://example.com/voice.ogg",
		Caption:   "Test voice message",
		ParseMode: "Markdown",
		Duration:  10,
	}

	jsonData, _ := json.Marshal(reqBody)
	req, _ := http.NewRequest("POST", "/"+config.APIURL+TelegramTabAccess+"/tab-telegram/send_voice", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	suite.router.ServeHTTP(w, req)

	assert.True(suite.T(), w.Code != http.StatusNotFound, "Endpoint should exist (may return 401 for auth required)")
}

// TestSendDocumentAPI tests the REST API endpoint for sending documents.
func (suite *TelegramAPITestSuite) TestSendDocumentAPI() {
	reqBody := dto.SendTelegramDocumentRequest{
		ChatID:    "123456789",
		Document:  "https://example.com/document.pdf",
		Caption:   "Test document",
		ParseMode: "Markdown",
	}

	jsonData, _ := json.Marshal(reqBody)
	req, _ := http.NewRequest("POST", "/"+config.APIURL+TelegramTabAccess+"/tab-telegram/send_document", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	suite.router.ServeHTTP(w, req)

	assert.True(suite.T(), w.Code != http.StatusNotFound, "Endpoint should exist (may return 401 for auth required)")
}

// TestSendPhotoAPI tests the REST API endpoint for sending photos.
func (suite *TelegramAPITestSuite) TestSendPhotoAPI() {
	reqBody := dto.SendTelegramPhotoRequest{
		ChatID:    "123456789",
		Photo:     "https://example.com/photo.jpg",
		Caption:   "Test photo",
		ParseMode: "Markdown",
	}

	jsonData, _ := json.Marshal(reqBody)
	req, _ := http.NewRequest("POST", "/"+config.APIURL+TelegramTabAccess+"/tab-telegram/send_photo", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	suite.router.ServeHTTP(w, req)

	assert.True(suite.T(), w.Code != http.StatusNotFound, "Endpoint should exist (may return 401 for auth required)")
}

// TestSendAudioAPI tests the REST API endpoint for sending audio files.
func (suite *TelegramAPITestSuite) TestSendAudioAPI() {
	reqBody := dto.SendTelegramAudioRequest{
		ChatID:    "123456789",
		Audio:     "https://example.com/audio.mp3",
		Caption:   "Test audio",
		ParseMode: "Markdown",
		Duration:  180,
		Performer: "Test Artist",
		Title:     "Test Song",
	}

	jsonData, _ := json.Marshal(reqBody)
	req, _ := http.NewRequest("POST", "/"+config.APIURL+TelegramTabAccess+"/tab-telegram/send_audio", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	suite.router.ServeHTTP(w, req)

	assert.True(suite.T(), w.Code != http.StatusNotFound, "Endpoint should exist (may return 401 for auth required)")
}

// TestSendVideoAPI tests the REST API endpoint for sending videos.
func (suite *TelegramAPITestSuite) TestSendVideoAPI() {
	reqBody := dto.SendTelegramVideoRequest{
		ChatID:    "123456789",
		Video:     "https://example.com/video.mp4",
		Caption:   "Test video",
		ParseMode: "Markdown",
		Duration:  60,
	}

	jsonData, _ := json.Marshal(reqBody)
	req, _ := http.NewRequest("POST", "/"+config.APIURL+TelegramTabAccess+"/tab-telegram/send_video", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	suite.router.ServeHTTP(w, req)

	assert.True(suite.T(), w.Code != http.StatusNotFound, "Endpoint should exist (may return 401 for auth required)")
}

// TestSendMessageWithKeyboardAPI tests the REST API endpoint for sending messages with keyboards.
func (suite *TelegramAPITestSuite) TestSendMessageWithKeyboardAPI() {
	keyboard := dto.InlineKeyboardMarkup{
		InlineKeyboard: []dto.InlineKeyboardButtonRow{
			{
				Buttons: []dto.InlineKeyboardButton{
					{
						Text:         "Test Button",
						CallbackData: "test_callback",
					},
				},
			},
		},
	}

	reqBody := dto.SendTelegramMessageWithKeyboardRequest{
		ChatID:    "123456789",
		Text:      "Test message with keyboard",
		ParseMode: "Markdown",
		Keyboard:  &keyboard,
	}

	jsonData, _ := json.Marshal(reqBody)
	req, _ := http.NewRequest("POST", "/"+config.APIURL+TelegramTabAccess+"/tab-telegram/send_message_with_keyboard", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	suite.router.ServeHTTP(w, req)

	assert.True(suite.T(), w.Code != http.StatusNotFound, "Endpoint should exist (may return 401 for auth required)")
}

// TestEditMessageAPI tests the REST API endpoint for editing messages.
func (suite *TelegramAPITestSuite) TestEditMessageAPI() {
	reqBody := dto.EditTelegramMessageRequest{
		ChatID:    "123456789",
		MessageID: 123,
		Text:      "Edited test message",
		ParseMode: "Markdown",
	}

	jsonData, _ := json.Marshal(reqBody)
	req, _ := http.NewRequest("POST", "/"+config.APIURL+TelegramTabAccess+"/tab-telegram/edit_message", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	suite.router.ServeHTTP(w, req)

	assert.True(suite.T(), w.Code != http.StatusNotFound, "Endpoint should exist (may return 401 for auth required)")
}

// TestAnswerCallbackQueryAPI tests the REST API endpoint for answering callback queries.
func (suite *TelegramAPITestSuite) TestAnswerCallbackQueryAPI() {
	reqBody := dto.TelegramAnswerCallbackQueryRequest{
		CallbackQueryID: "test_callback_id",
		Text:            "Callback answered",
		ShowAlert:       false,
	}

	jsonData, _ := json.Marshal(reqBody)
	req, _ := http.NewRequest("POST", "/"+config.APIURL+TelegramTabAccess+"/tab-telegram/answer_callback_query", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	suite.router.ServeHTTP(w, req)

	assert.True(suite.T(), w.Code != http.StatusNotFound, "Endpoint should exist (may return 401 for auth required)")
}

// TestTelegramAPITestSuite runs the Telegram API test suite.
func TestTelegramAPITestSuite(t *testing.T) {
	suite.Run(t, new(TelegramAPITestSuite))
}
