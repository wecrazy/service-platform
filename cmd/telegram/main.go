package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/go-redis/redis/v8"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
	"gorm.io/gorm"

	"service-platform/internal/api/v1/controllers"
	"service-platform/internal/config"
	telegrammodel "service-platform/internal/core/model/telegram_model"
	"service-platform/internal/database"
	"service-platform/internal/pkg/fun"
	"service-platform/internal/pkg/logger"
	pb "service-platform/proto"
)

// server implements the TelegramServiceServer interface.
type server struct {
	pb.UnimplementedTelegramServiceServer
	bot         *tgbotapi.BotAPI
	redis       *redis.Client
	db          *gorm.DB
	defaultLang string
	helper      *controllers.TelegramHelper
}

// SendMessage sends a text message to a specified chat.
func (s *server) SendMessage(ctx context.Context, req *pb.SendTelegramMessageRequest) (*pb.SendTelegramMessageResponse, error) {
	msg := tgbotapi.NewMessageToChannel(req.ChatId, req.Text)
	if req.ParseMode != "" {
		msg.ParseMode = req.ParseMode
	}

	sentMsg, err := s.bot.Send(msg)
	if err != nil {
		logrus.WithError(err).Error("Failed to send Telegram message")
		return &pb.SendTelegramMessageResponse{
			Success: false,
			Message: err.Error(),
		}, nil
	}

	// Store sent message in database
	sentTime := sentMsg.Time()
	isGroup := sentMsg.Chat.IsGroup() || sentMsg.Chat.IsSuperGroup()
	telegramMsg := telegrammodel.TelegramMsg{
		TelegramChatID:        req.ChatId,
		TelegramMessageSentTo: req.ChatId,
		TelegramSentAt:        &sentTime,
		TelegramSenderID:      fmt.Sprintf("%d", s.bot.Self.ID),
		TelegramMessageBody:   req.Text,
		TelegramMessageType:   telegrammodel.TelegramTextMessage,
		TelegramIsGroup:       isGroup,
		TelegramMsgStatus:     "sent",
		TelegramMessageID:     int64(sentMsg.MessageID),
	}

	if err := s.db.Create(&telegramMsg).Error; err != nil {
		logrus.WithError(err).Error("Failed to store sent Telegram message")
		// Don't return error, message was sent successfully
	}

	return &pb.SendTelegramMessageResponse{
		Success:   true,
		Message:   "Message sent successfully",
		MessageId: int64(sentMsg.MessageID),
	}, nil
}

// SendMessageWithKeyboard sends a text message with an inline keyboard to a specified chat.
func (s *server) SendMessageWithKeyboard(ctx context.Context, req *pb.SendTelegramMessageWithKeyboardRequest) (*pb.SendTelegramMessageWithKeyboardResponse, error) {
	msg := tgbotapi.NewMessageToChannel(req.ChatId, req.Text)
	if req.ParseMode != "" {
		msg.ParseMode = req.ParseMode
	}

	if req.Keyboard != nil {
		keyboard := s.helper.BuildInlineKeyboard(req.Keyboard)
		msg.ReplyMarkup = keyboard
	}

	sentMsg, err := s.bot.Send(msg)
	if err != nil {
		logrus.WithError(err).Error("Failed to send Telegram message with keyboard")
		return &pb.SendTelegramMessageWithKeyboardResponse{
			Success: false,
			Message: err.Error(),
		}, nil
	}

	// Store sent message in database
	sentTime := sentMsg.Time()
	isGroup := sentMsg.Chat.IsGroup() || sentMsg.Chat.IsSuperGroup()
	telegramMsg := telegrammodel.TelegramMsg{
		TelegramChatID:        req.ChatId,
		TelegramMessageSentTo: req.ChatId,
		TelegramSentAt:        &sentTime,
		TelegramSenderID:      fmt.Sprintf("%d", s.bot.Self.ID),
		TelegramMessageBody:   req.Text,
		TelegramMessageType:   telegrammodel.TelegramTextMessage,
		TelegramIsGroup:       isGroup,
		TelegramMsgStatus:     "sent",
		TelegramMessageID:     int64(sentMsg.MessageID),
	}

	if err := s.db.Create(&telegramMsg).Error; err != nil {
		logrus.WithError(err).Error("Failed to store sent Telegram message with keyboard")
		// Don't return error, message was sent successfully
	}

	return &pb.SendTelegramMessageWithKeyboardResponse{
		Success:   true,
		Message:   "Message with keyboard sent successfully",
		MessageId: int64(sentMsg.MessageID),
	}, nil
}

// EditMessage edits an existing message in a specified chat.
func (s *server) EditMessage(ctx context.Context, req *pb.EditTelegramMessageRequest) (*pb.EditTelegramMessageResponse, error) {
	// Parse chat ID - it should be numeric for editing messages
	chatID, err := strconv.ParseInt(req.ChatId, 10, 64)
	if err != nil {
		return &pb.EditTelegramMessageResponse{
			Success: false,
			Message: "Invalid chat ID: must be numeric for message editing",
		}, nil
	}

	editMsg := tgbotapi.NewEditMessageText(chatID, int(req.MessageId), req.Text)
	if req.ParseMode != "" {
		editMsg.ParseMode = req.ParseMode
	}

	if req.Keyboard != nil {
		keyboard := s.helper.BuildInlineKeyboard(req.Keyboard)
		editMsg.ReplyMarkup = &keyboard
	}

	_, err = s.bot.Send(editMsg)
	if err != nil {
		logrus.WithError(err).Error("Failed to edit Telegram message")
		return &pb.EditTelegramMessageResponse{
			Success: false,
			Message: err.Error(),
		}, nil
	}

	// Store edited message in database
	currentTime := time.Now()

	// Get chat info to determine if it's a group
	chatInfoCfg := tgbotapi.ChatInfoConfig{
		ChatConfig: tgbotapi.ChatConfig{
			ChatID: chatID,
		},
	}
	chat, err := s.bot.GetChat(chatInfoCfg)
	isGroup := false
	if err != nil {
		logrus.WithError(err).Error("Failed to get chat info for group detection")
	} else {
		isGroup = chat.IsGroup() || chat.IsSuperGroup()
	}

	telegramMsg := telegrammodel.TelegramMsg{
		TelegramChatID:        req.ChatId,
		TelegramMessageSentTo: req.ChatId,
		TelegramSentAt:        &currentTime,
		TelegramSenderID:      fmt.Sprintf("%d", s.bot.Self.ID),
		TelegramMessageBody:   req.Text,
		TelegramMessageType:   telegrammodel.TelegramTextMessage,
		TelegramIsGroup:       isGroup,
		TelegramMsgStatus:     "edited",
		TelegramMessageID:     req.MessageId,
	}

	if err := s.db.Create(&telegramMsg).Error; err != nil {
		logrus.WithError(err).Error("Failed to store edited Telegram message")
		// Don't return error, message was edited successfully
	}

	return &pb.EditTelegramMessageResponse{
		Success: true,
		Message: "Message edited successfully",
	}, nil
}

// AnswerCallbackQuery answers a callback query from an inline keyboard button.
func (s *server) AnswerCallbackQuery(ctx context.Context, req *pb.TelegramAnswerCallbackQueryRequest) (*pb.TelegramAnswerCallbackQueryResponse, error) {
	callback := tgbotapi.NewCallback(req.CallbackQueryId, req.Text)
	if req.ShowAlert {
		callback.ShowAlert = true
	}

	_, err := s.bot.Request(callback)
	if err != nil {
		logrus.WithError(err).Error("Failed to answer callback query")
		return &pb.TelegramAnswerCallbackQueryResponse{
			Success: false,
			Message: err.Error(),
		}, nil
	}

	return &pb.TelegramAnswerCallbackQueryResponse{
		Success: true,
		Message: "Callback query answered successfully",
	}, nil
}

func main() {
	// Load config
	if err := config.LoadConfig(); err != nil {
		log.Fatalf("Error loading .yaml conf :%v", err)
	}
	go config.WatchConfig()
	cfg := config.GetConfig()

	// Initialize logger
	logger.InitLogrus()

	// Initialize database
	db, err := database.InitAndCheckDB(
		cfg.Database.Type,
		cfg.Database.Username,
		cfg.Database.Password,
		cfg.Database.Host,
		cfg.Database.Port,
		cfg.Database.Name,
		cfg.Database.SSLMode,
	)
	if err != nil {
		logrus.WithError(err).Fatal("Failed to initialize database")
	}
	logrus.Info("✅ Connected to database")

	// Auto-migrate Telegram models
	// Drop old unique indexes on telegram_chat_id if they exist
	indexesToDrop := []string{
		"idx_telegram_incoming_messages_telegram_chat_id",
		"idx_telegram_messages_telegram_chat_id", // Assuming GORM generates this for TelegramMsg
	}
	for _, idx := range indexesToDrop {
		if db.Migrator().HasIndex(&telegrammodel.TelegramIncomingMsg{}, idx) {
			if err := db.Migrator().DropIndex(&telegrammodel.TelegramIncomingMsg{}, idx); err != nil {
				logrus.WithError(err).Warn("Failed to drop old index on incoming messages, continuing")
			}
		}
		if db.Migrator().HasIndex(&telegrammodel.TelegramMsg{}, idx) {
			if err := db.Migrator().DropIndex(&telegrammodel.TelegramMsg{}, idx); err != nil {
				logrus.WithError(err).Warn("Failed to drop old index on sent messages, continuing")
			}
		}
	}
	if err := db.AutoMigrate(&telegrammodel.TelegramMsg{}, &telegrammodel.TelegramIncomingMsg{}); err != nil {
		logrus.WithError(err).Fatal("Failed to migrate Telegram models")
	}
	logrus.Info("✅ Migrated Telegram models")

	// Initialize Redis
	redisClient := redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%d", cfg.Redis.Host, cfg.Redis.Port),
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.Db,
	})
	defer redisClient.Close()

	// Test Redis connection
	_, err = redisClient.Ping(context.Background()).Result()
	if err != nil {
		logrus.WithError(err).Fatal("Failed to connect to Redis")
	}
	logrus.Info("✅ Connected to Redis")

	// Initialize Telegram bot
	bot, err := tgbotapi.NewBotAPI(cfg.Telegram.BotToken)
	if err != nil {
		log.Fatal("Failed to initialize Telegram bot: ", err)
	}

	bot.Debug = cfg.Telegram.Debug

	logrus.Info("✅ Telegram bot authorized on account ", bot.Self.UserName)

	// Create gRPC server
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", cfg.Telegram.GRPCPort))
	if err != nil {
		logrus.WithError(err).Fatal("Failed to listen on port")
	}

	s := grpc.NewServer()
	serverInstance := &server{
		bot:         bot,
		redis:       redisClient,
		db:          db,
		defaultLang: fun.DefaultLang,
	}
	serverInstance.helper = controllers.NewTelegramHelper(bot, redisClient, db, fun.DefaultLang)
	pb.RegisterTelegramServiceServer(s, serverInstance)
	reflection.Register(s)

	// Start update listener
	u := tgbotapi.NewUpdate(0) // Offset 0 = start from current updates
	u.Timeout = 60
	updates := bot.GetUpdatesChan(u)

	go func() {
		for update := range updates {
			serverInstance.helper.HandleUpdate(update)
		}
	}()

	logrus.Info("✅ Telegram update listener started")

	// Start metrics server
	go func() {
		http.Handle("/telegram-metrics", promhttp.Handler())
		logrus.Info("✅ Starting metrics server on port ", cfg.Metrics.TelegramPort)
		if err := http.ListenAndServe(fmt.Sprintf(":%d", cfg.Metrics.TelegramPort), nil); err != nil {
			logrus.WithError(err).Fatal("Failed to start metrics server")
		}
	}()

	// Handle graceful shutdown
	go func() {
		c := make(chan os.Signal, 1)
		signal.Notify(c, os.Interrupt, syscall.SIGTERM)
		<-c
		logrus.Info("❌ Shutting down Telegram service...")
		s.GracefulStop()
	}()

	fmt.Printf("🤖 Telegram gRPC server listening on port %d\n", cfg.Telegram.GRPCPort)
	if err := s.Serve(lis); err != nil {
		log.Fatal("Failed to serve gRPC server: ", err)
	}
}
