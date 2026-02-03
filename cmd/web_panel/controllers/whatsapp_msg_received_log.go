package controllers

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"service-platform/cmd/web_panel/config"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	"go.mau.fi/whatsmeow/types/events"
	"gopkg.in/natefinch/lumberjack.v2"
)

// WhatsappMsgReceivedCSVFormatter - as defined previously
type WhatsappMsgReceivedCSVFormatter struct {
	IncludeHeader   bool
	TimestampFormat string
	once            bool
	FieldOrder      []string
}

func (f *WhatsappMsgReceivedCSVFormatter) Format(entry *logrus.Entry) ([]byte, error) {
	var b bytes.Buffer
	if f.TimestampFormat == "" {
		f.TimestampFormat = "2006-01-02 15:04:05.000 MST"
	}
	if f.IncludeHeader && !f.once {
		header := []string{"level", "time", "msg"}
		header = append(header, f.FieldOrder...)
		b.WriteString(strings.Join(header, ",") + "\n")
		f.once = false
	}
	fields := []string{
		entry.Level.String(),
		entry.Time.Format(f.TimestampFormat),
		strings.ReplaceAll(entry.Message, ",", ";"),
	}
	for _, key := range f.FieldOrder {
		if val, ok := entry.Data[key]; ok {
			fields = append(fields, fmt.Sprint(val))
		} else {
			fields = append(fields, "")
		}
	}
	b.WriteString(strings.Join(fields, ",") + "\n")
	return b.Bytes(), nil
}

func LogIncomingWhatsAppMessage(e *events.Message, uploadDir string) {
	logPath := config.GetConfig().Whatsmeow.MsgReceivedLogFile
	if logPath == "" {
		logrus.Warn("WhatsApp message log path is empty")
		return
	}

	// Ensure log directory exists
	dir := filepath.Dir(logPath)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		if err := os.MkdirAll(dir, os.ModePerm); err != nil {
			logrus.Errorf("Failed to create log directory: %v", err)
			return
		}
	}

	// Ensure file exists
	if _, err := os.Stat(logPath); os.IsNotExist(err) {
		if f, err := os.Create(logPath); err != nil {
			logrus.Errorf("Failed to create log file: %v", err)
			return
		} else {
			f.Close()
		}
	}

	// Create message logger
	logger := logrus.New()
	logger.SetReportCaller(false)
	logger.SetFormatter(&WhatsappMsgReceivedCSVFormatter{
		TimestampFormat: "2006-01-02 15:04:05.000 MST",
		IncludeHeader:   false,
		FieldOrder: []string{
			"event_id",
			"from",
			"push_name",
			"sender_jid",
			"timestamp",
			"is_group",
			"message_type",
		},
	})
	logger.SetOutput(&lumberjack.Logger{
		Filename:   logPath,
		MaxSize:    50,
		MaxAge:     30,
		MaxBackups: 5,
		Compress:   false,
	})
	logger.SetLevel(logrus.InfoLevel)

	// Extract content
	content := extractMessageContent(e, uploadDir)

	// Prepare log fields
	fields := logrus.Fields{
		"event_id":     e.Info.ID,
		"from":         e.Info.SourceString(),
		"push_name":    e.Info.PushName,
		"sender_jid":   e.Info.Sender.String(),
		"timestamp":    e.Info.Timestamp.Format("2006-01-02 15:04:05.000 MST"),
		"is_group":     e.Info.IsGroup,
		"message_type": content,
	}

	logger.WithFields(fields).Info("Incoming WhatsApp message")
}

func extractMessageContent(e *events.Message, uploadDir string) string {
	var msgReceived string
	waReplyPublicURL := config.GetConfig().Whatsmeow.WAReplyPublicURL + "/" + time.Now().Format("2006-01-02")

	switch {

	case e.Message.Conversation != nil:
		msgReceived = *e.Message.Conversation

	case e.Message.ExtendedTextMessage != nil && e.Message.ExtendedTextMessage.Text != nil:
		msgReceived = *e.Message.ExtendedTextMessage.Text

	case e.Message.ImageMessage != nil:
		msg := e.Message.ImageMessage
		data, err := WhatsappClient.Download(context.Background(), msg)
		if err != nil {
			logrus.Error("Failed to download image:", err)
			break
		}
		mimeType := getSafeString(msg.Mimetype)
		ext := getFileExtension(mimeType)
		filename := fmt.Sprintf("img_%d%s", time.Now().UnixNano(), ext)
		savePath := filepath.Join(uploadDir, filename)
		os.WriteFile(savePath, data, 0644)
		publicURL := fmt.Sprintf("%s/%s", waReplyPublicURL, filename)
		caption := getSafeString(msg.Caption)
		msgReceived = fmt.Sprintf("📷 %s %s", caption, publicURL)

	case e.Message.VideoMessage != nil:
		msg := e.Message.VideoMessage
		data, err := WhatsappClient.Download(context.Background(), msg)
		if err != nil {
			logrus.Info("Failed to download video:", err)
			break
		}
		mimeType := getSafeString(msg.Mimetype)
		ext := getFileExtension(mimeType)
		filename := fmt.Sprintf("vid_%d%s", time.Now().UnixNano(), ext)
		savePath := filepath.Join(uploadDir, filename)
		os.WriteFile(savePath, data, 0644)
		publicURL := fmt.Sprintf("%s/%s", waReplyPublicURL, filename)
		caption := getSafeString(msg.Caption)
		msgReceived = fmt.Sprintf("🎥 %s %s", caption, publicURL)

	case e.Message.AudioMessage != nil:
		msg := e.Message.AudioMessage
		data, err := WhatsappClient.Download(context.Background(), msg)
		if err != nil {
			logrus.Error("Failed to download audio:", err)
			break
		}
		mimeType := getSafeString(msg.Mimetype)
		ext := getFileExtension(mimeType)
		filename := fmt.Sprintf("aud_%d%s", time.Now().UnixNano(), ext)
		savePath := filepath.Join(uploadDir, filename)
		os.WriteFile(savePath, data, 0644)
		publicURL := fmt.Sprintf("%s/%s", waReplyPublicURL, filename)
		msgReceived = fmt.Sprintf("🎧 Audio message: %s", publicURL)

	case e.Message.DocumentMessage != nil:
		msg := e.Message.DocumentMessage
		data, err := WhatsappClient.Download(context.Background(), msg)
		if err != nil {
			logrus.Error("Failed to download document:", err)
			break
		}
		mimeType := getSafeString(msg.Mimetype)
		ext := getFileExtension(mimeType)
		filename := fmt.Sprintf("doc_%d%s", time.Now().UnixNano(), ext)
		savePath := filepath.Join(uploadDir, filename)
		os.WriteFile(savePath, data, 0644)
		publicURL := fmt.Sprintf("%s/%s", waReplyPublicURL, filename)
		caption := getSafeString(msg.Caption)
		msgReceived = fmt.Sprintf("📄 %s %s", caption, publicURL)

	case e.Message.StickerMessage != nil:
		msg := e.Message.StickerMessage
		data, err := WhatsappClient.Download(context.Background(), msg)
		if err != nil {
			logrus.Error("Failed to download sticker:", err)
			break
		}
		mimeType := getSafeString(msg.Mimetype)
		ext := getFileExtension(mimeType)
		filename := fmt.Sprintf("stk_%d%s", time.Now().UnixNano(), ext)
		savePath := filepath.Join(uploadDir, filename)
		os.WriteFile(savePath, data, 0644)
		publicURL := fmt.Sprintf("%s/%s", waReplyPublicURL, filename)
		msgReceived = fmt.Sprintf("🖼️ Sticker: %s", publicURL)

	default:
		msgReceived = "(non-text or unknown reply)"
	}

	// Remove comma coz using CSV Formatter
	msgReceived = strings.ReplaceAll(msgReceived, ",", " ")
	return msgReceived
}
