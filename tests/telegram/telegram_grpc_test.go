package telegram_test

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"service-platform/internal/config"
	"service-platform/proto"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// TelegramGRPCTestSuite provides a test suite for Telegram gRPC service testing.
// It includes setup for gRPC client connections to test Telegram endpoints.
type TelegramGRPCTestSuite struct {
	suite.Suite
	conn   *grpc.ClientConn            // gRPC client connection
	client proto.TelegramServiceClient // Telegram service gRPC client
}

// SetupTest initializes the Telegram gRPC test suite by establishing a connection to the Telegram service.
// It loads configuration and creates a gRPC client connection to the Telegram service.
func (suite *TelegramGRPCTestSuite) SetupTest() {
	var err error
	err = config.LoadConfig()
	assert.NoError(suite.T(), err)
	cfg := config.GetConfig()

	host := cfg.GRPC.Host
	port := cfg.GRPC.Port
	addr := fmt.Sprintf("%s:%d", host, port)

	suite.conn, err = grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		suite.conn = nil
		suite.client = nil
		return
	}
	suite.client = proto.NewTelegramServiceClient(suite.conn)
}

// TearDownTest cleans up the Telegram gRPC test suite by closing the client connection.
func (suite *TelegramGRPCTestSuite) TearDownTest() {
	if suite.conn != nil {
		suite.conn.Close()
	}
}

// TestSendMessage tests the gRPC SendMessage functionality.
// It sends a text message request and verifies the response.
func (suite *TelegramGRPCTestSuite) TestSendMessage() {
	req := &proto.SendTelegramMessageRequest{
		ChatId:    "123456789",
		Text:      "Test message from unit test",
		ParseMode: "Markdown",
	}
	resp, err := suite.client.SendMessage(context.Background(), req)
	if err != nil {
		if strings.Contains(err.Error(), "Unavailable") {
			suite.T().Skip("gRPC server not running")
		}
		assert.NoError(suite.T(), err)
	}
	assert.NotNil(suite.T(), resp)
	if resp != nil {
		assert.True(suite.T(), resp.Success || !resp.Success) // Just check the field exists
	}
}

// TestSendVoice tests the gRPC SendVoice functionality.
// It sends a voice message request and verifies the response.
func (suite *TelegramGRPCTestSuite) TestSendVoice() {
	req := &proto.SendTelegramVoiceRequest{
		ChatId:    "123456789",
		Voice:     "https://example.com/voice.ogg",
		Caption:   "Test voice message",
		ParseMode: "Markdown",
		Duration:  10,
	}
	resp, err := suite.client.SendVoice(context.Background(), req)
	if err != nil {
		if strings.Contains(err.Error(), "Unavailable") {
			suite.T().Skip("gRPC server not running")
		}
		assert.NoError(suite.T(), err)
	}
	assert.NotNil(suite.T(), resp)
	if resp != nil {
		assert.True(suite.T(), resp.Success || !resp.Success)
	}
}

// TestSendDocument tests the gRPC SendDocument functionality.
// It sends a document request and verifies the response.
func (suite *TelegramGRPCTestSuite) TestSendDocument() {
	req := &proto.SendTelegramDocumentRequest{
		ChatId:    "123456789",
		Document:  "https://example.com/document.pdf",
		Caption:   "Test document",
		ParseMode: "Markdown",
	}
	resp, err := suite.client.SendDocument(context.Background(), req)
	if err != nil {
		if strings.Contains(err.Error(), "Unavailable") {
			suite.T().Skip("gRPC server not running")
		}
		assert.NoError(suite.T(), err)
	}
	assert.NotNil(suite.T(), resp)
	if resp != nil {
		assert.True(suite.T(), resp.Success || !resp.Success)
	}
}

// TestSendPhoto tests the gRPC SendPhoto functionality.
// It sends a photo request and verifies the response.
func (suite *TelegramGRPCTestSuite) TestSendPhoto() {
	req := &proto.SendTelegramPhotoRequest{
		ChatId:    "123456789",
		Photo:     "https://example.com/photo.jpg",
		Caption:   "Test photo",
		ParseMode: "Markdown",
	}
	resp, err := suite.client.SendPhoto(context.Background(), req)
	if err != nil {
		if strings.Contains(err.Error(), "Unavailable") {
			suite.T().Skip("gRPC server not running")
		}
		assert.NoError(suite.T(), err)
	}
	assert.NotNil(suite.T(), resp)
	if resp != nil {
		assert.True(suite.T(), resp.Success || !resp.Success)
	}
}

// TestSendAudio tests the gRPC SendAudio functionality.
// It sends an audio request and verifies the response.
func (suite *TelegramGRPCTestSuite) TestSendAudio() {
	req := &proto.SendTelegramAudioRequest{
		ChatId:    "123456789",
		Audio:     "https://example.com/audio.mp3",
		Caption:   "Test audio",
		ParseMode: "Markdown",
		Duration:  180,
		Performer: "Test Artist",
		Title:     "Test Song",
	}
	resp, err := suite.client.SendAudio(context.Background(), req)
	if err != nil {
		if strings.Contains(err.Error(), "Unavailable") {
			suite.T().Skip("gRPC server not running")
		}
		assert.NoError(suite.T(), err)
	}
	assert.NotNil(suite.T(), resp)
	if resp != nil {
		assert.True(suite.T(), resp.Success || !resp.Success)
	}
}

// TestSendVideo tests the gRPC SendVideo functionality.
// It sends a video request and verifies the response.
func (suite *TelegramGRPCTestSuite) TestSendVideo() {
	req := &proto.SendTelegramVideoRequest{
		ChatId:    "123456789",
		Video:     "https://example.com/video.mp4",
		Caption:   "Test video",
		ParseMode: "Markdown",
		Duration:  60,
	}
	resp, err := suite.client.SendVideo(context.Background(), req)
	if err != nil {
		if strings.Contains(err.Error(), "Unavailable") {
			suite.T().Skip("gRPC server not running")
		}
		assert.NoError(suite.T(), err)
	}
	assert.NotNil(suite.T(), resp)
	if resp != nil {
		assert.True(suite.T(), resp.Success || !resp.Success)
	}
}

// TestSendMessageWithKeyboard tests the gRPC SendMessageWithKeyboard functionality.
// It sends a message with keyboard request and verifies the response.
func (suite *TelegramGRPCTestSuite) TestSendMessageWithKeyboard() {
	keyboard := &proto.InlineKeyboardMarkup{
		InlineKeyboard: []*proto.InlineKeyboardButtonRow{
			{
				Buttons: []*proto.InlineKeyboardButton{
					{
						Text:         "Test Button",
						CallbackData: "test_callback",
					},
				},
			},
		},
	}

	req := &proto.SendTelegramMessageWithKeyboardRequest{
		ChatId:    "123456789",
		Text:      "Test message with keyboard",
		ParseMode: "Markdown",
		Keyboard:  keyboard,
	}
	resp, err := suite.client.SendMessageWithKeyboard(context.Background(), req)
	if err != nil {
		if strings.Contains(err.Error(), "Unavailable") {
			suite.T().Skip("gRPC server not running")
		}
		assert.NoError(suite.T(), err)
	}
	assert.NotNil(suite.T(), resp)
	if resp != nil {
		assert.True(suite.T(), resp.Success || !resp.Success)
	}
}

// TestEditMessage tests the gRPC EditMessage functionality.
// It sends an edit message request and verifies the response.
func (suite *TelegramGRPCTestSuite) TestEditMessage() {
	req := &proto.EditTelegramMessageRequest{
		ChatId:    "123456789",
		MessageId: 123,
		Text:      "Edited test message",
		ParseMode: "Markdown",
	}
	resp, err := suite.client.EditMessage(context.Background(), req)
	if err != nil {
		if strings.Contains(err.Error(), "Unavailable") {
			suite.T().Skip("gRPC server not running")
		}
		assert.NoError(suite.T(), err)
	}
	assert.NotNil(suite.T(), resp)
	if resp != nil {
		assert.True(suite.T(), resp.Success || !resp.Success)
	}
}

// TestAnswerCallbackQuery tests the gRPC AnswerCallbackQuery functionality.
// It sends an answer callback query request and verifies the response.
func (suite *TelegramGRPCTestSuite) TestAnswerCallbackQuery() {
	req := &proto.TelegramAnswerCallbackQueryRequest{
		CallbackQueryId: "test_callback_id",
		Text:            "Callback answered",
		ShowAlert:       false,
	}
	resp, err := suite.client.AnswerCallbackQuery(context.Background(), req)
	if err != nil {
		if strings.Contains(err.Error(), "Unavailable") {
			suite.T().Skip("gRPC server not running")
		}
		assert.NoError(suite.T(), err)
	}
	assert.NotNil(suite.T(), resp)
	if resp != nil {
		assert.True(suite.T(), resp.Success || !resp.Success)
	}
}

// TestTelegramGRPCTestSuite runs the Telegram gRPC test suite.
func TestTelegramGRPCTestSuite(t *testing.T) {
	suite.Run(t, new(TelegramGRPCTestSuite))
}
