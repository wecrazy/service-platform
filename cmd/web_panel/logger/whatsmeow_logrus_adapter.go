package logger

import (
	"strings"

	"github.com/sirupsen/logrus"
	waLog "go.mau.fi/whatsmeow/util/log"
	"gopkg.in/natefinch/lumberjack.v2"
)

type LogrusAdapter struct {
	logger *logrus.Entry
}

// Implement waLog.Logger interface
func (a *LogrusAdapter) Debugf(msg string, args ...interface{}) { a.logger.Debugf(msg, args...) }
func (a *LogrusAdapter) Infof(msg string, args ...interface{})  { a.logger.Infof(msg, args...) }
func (a *LogrusAdapter) Warnf(msg string, args ...interface{})  { a.logger.Warnf(msg, args...) }
func (a *LogrusAdapter) Errorf(msg string, args ...interface{}) { a.logger.Errorf(msg, args...) }

// Sub returns a sub-logger with extra field
func (a *LogrusAdapter) Sub(module string) waLog.Logger {
	return &LogrusAdapter{
		logger: a.logger.WithField("submodule", module),
	}
}

// NewWhatsmeowLogger builds the LogrusAdapter
func NewWhatsmeowLogger(module string, logFile string, level logrus.Level) *LogrusAdapter {
	baseLogger := logrus.New()
	baseLogger.SetLevel(level)

	// Rotate log files
	fileWriter := &lumberjack.Logger{
		Filename:   logFile,
		MaxSize:    10, // MB
		MaxBackups: 5,
		MaxAge:     7, // days
		Compress:   true,
	}

	// Console + file
	// multiWriter := io.MultiWriter(os.Stdout, fileWriter)
	baseLogger.SetOutput(fileWriter)

	baseLogger.SetFormatter(&logrus.TextFormatter{
		FullTimestamp: true,
	})

	// Add module field
	return &LogrusAdapter{
		logger: baseLogger.WithField("module", module),
	}
}

// ParseWhatsmeowLogLevel converts string like "DEBUG" to logrus.Level.
// Defaults to INFO if invalid.
func ParseWhatsmeowLogLevel(levelStr string) logrus.Level {
	switch strings.ToUpper(levelStr) {
	case "DEBUG":
		return logrus.DebugLevel
	case "INFO":
		return logrus.InfoLevel
	case "WARN", "WARNING":
		return logrus.WarnLevel
	case "ERROR":
		return logrus.ErrorLevel
	case "FATAL":
		return logrus.FatalLevel
	case "PANIC":
		return logrus.PanicLevel
	default:
		return logrus.InfoLevel
	}
}
