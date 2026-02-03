package logger

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"service-platform/cmd/web_panel/config"
	"strings"
	"sync"

	"github.com/sirupsen/logrus"
	"gopkg.in/natefinch/lumberjack.v2"
)

var (
	loggers      = make(map[string]*logrus.Logger)
	mu           sync.RWMutex
	fileLoggers  = make(map[string]*lumberjack.Logger)
	fileLoggerMu sync.RWMutex
)

// DynamicFileHook routes logs to different files based on caller
type DynamicFileHook struct{}

func (hook *DynamicFileHook) Fire(entry *logrus.Entry) error {
	if !entry.HasCaller() {
		return nil
	}

	// Get the caller's filename
	callerFile := entry.Caller.File
	baseName := filepath.Base(callerFile)
	fileName := strings.TrimSuffix(baseName, filepath.Ext(baseName))
	fileName = strings.ReplaceAll(fileName, " ", "_")
	logFileName := fileName + ".log"

	// Get or create file logger for this file
	fileLogger := getOrCreateFileLogger(logFileName)

	// Format the entry
	formatter := &CSVFormatter{
		TimestampFormat: "2006-01-02 15:04:05.000 MST",
	}
	serialized, err := formatter.Format(entry)
	if err != nil {
		return err
	}

	// Write to the file-specific logger
	_, err = fileLogger.Write(serialized)
	return err
}

func (hook *DynamicFileHook) Levels() []logrus.Level {
	return logrus.AllLevels
}

func getOrCreateFileLogger(logFileName string) *lumberjack.Logger {
	fileLoggerMu.RLock()
	if logger, exists := fileLoggers[logFileName]; exists {
		fileLoggerMu.RUnlock()
		return logger
	}
	fileLoggerMu.RUnlock()

	fileLoggerMu.Lock()
	defer fileLoggerMu.Unlock()

	// Double-check after acquiring write lock
	if logger, exists := fileLoggers[logFileName]; exists {
		return logger
	}

	appLogDir := config.GetConfig().App.LogDir
	if err := os.MkdirAll(appLogDir, os.ModePerm); err != nil {
		log.Printf("Failed to create log directory: %v", err)
		return nil
	}

	logPath := filepath.Join(appLogDir, logFileName)
	fileLogger := &lumberjack.Logger{
		Filename:   logPath,
		MaxSize:    10,
		MaxAge:     30,
		MaxBackups: 30,
		Compress:   true,
	}

	fileLoggers[logFileName] = fileLogger
	return fileLogger
}

type CSVFormatter struct {
	IncludeHeader   bool
	TimestampFormat string
	once            bool
}

func (f *CSVFormatter) Format(entry *logrus.Entry) ([]byte, error) {
	var b bytes.Buffer

	if f.TimestampFormat == "" {
		f.TimestampFormat = "2006-01-02T15:04:05.000Z07:00"
	}

	// Add header row once
	if f.IncludeHeader && !f.once {
		header := []string{"level", "time", "msg", "caller"}
		for k := range entry.Data {
			header = append(header, k)
		}
		b.WriteString(strings.Join(header, ",") + "\n")
		f.once = true
	}

	// Format log fields
	csvFields := []string{
		entry.Level.String(),
		entry.Time.Format(f.TimestampFormat),
		strings.ReplaceAll(entry.Message, ",", ";"), // sanitize commas
	}

	// Add shortened caller (remove current working dir)
	caller := ""
	if entry.HasCaller() {
		wd, err := os.Getwd()
		if err == nil {
			cleanPath := strings.TrimPrefix(entry.Caller.File, wd+"/")
			caller = fmt.Sprintf("%s:%d", cleanPath, entry.Caller.Line)
		} else {
			caller = fmt.Sprintf("%s:%d", entry.Caller.File, entry.Caller.Line)
		}
	}
	csvFields = append(csvFields, caller)

	// Add extra fields
	for _, v := range entry.Data {
		csvFields = append(csvFields, fmt.Sprint(v))
	}

	b.WriteString(strings.Join(csvFields, ",") + "\n")
	return b.Bytes(), nil
}

// InitLogrus initializes the global Logrus logger with dynamic file routing
// Logs from each file automatically go to their own log file
// Example: logs from scheduler.go → scheduler.log, main.go → main.log
func InitLogrus() {
	logLevel := config.GetConfig().App.LogLevel
	switch strings.ToLower(logLevel) {
	case "panic":
		logrus.SetLevel(logrus.PanicLevel)
	case "fatal":
		logrus.SetLevel(logrus.FatalLevel)
	case "error":
		logrus.SetLevel(logrus.ErrorLevel)
	case "warn", "warning":
		logrus.SetLevel(logrus.WarnLevel)
	case "info":
		logrus.SetLevel(logrus.InfoLevel)
	case "debug":
		logrus.SetLevel(logrus.DebugLevel)
	case "trace":
		logrus.SetLevel(logrus.TraceLevel)
	default:
		logrus.SetLevel(logrus.TraceLevel)
	}

	// Set report caller to true so we can detect the calling file
	logrus.SetReportCaller(true)

	// Add the dynamic file hook
	logrus.AddHook(&DynamicFileHook{})

	// Set a no-op formatter since the hook handles formatting
	logrus.SetFormatter(&logrus.TextFormatter{
		DisableColors:          true,
		DisableTimestamp:       true,
		DisableLevelTruncation: true,
	})

	// Discard default output since we're writing via hooks
	// logrus.SetOutput(os.Stderr) // Keep stderr for fallback
	logrus.SetOutput(io.Discard) // Keep stderr for fallback

	logrus.Info("🟢 Dynamic logger initialized successfully")
}

// GetLoggerForFile returns a logger instance configured for a specific file
// This creates a separate logger instance that writes to its own log file
func GetLoggerForFile(logFileName string) *logrus.Logger {
	// Check if logger already exists
	mu.RLock()
	if logger, exists := loggers[logFileName]; exists {
		mu.RUnlock()
		return logger
	}
	mu.RUnlock()

	// Create new logger
	mu.Lock()
	defer mu.Unlock()

	// Double-check after acquiring write lock
	if logger, exists := loggers[logFileName]; exists {
		return logger
	}

	logger := logrus.New()

	appLogDir := config.GetConfig().App.LogDir
	if err := os.MkdirAll(appLogDir, os.ModePerm); err != nil {
		log.Fatal(err)
	}

	logPath := filepath.Join(appLogDir, logFileName)

	// Apply same configuration as main logger
	logLevel := config.GetConfig().App.LogLevel
	switch strings.ToLower(logLevel) {
	case "panic":
		logger.SetLevel(logrus.PanicLevel)
	case "fatal":
		logger.SetLevel(logrus.FatalLevel)
	case "error":
		logger.SetLevel(logrus.ErrorLevel)
	case "warn", "warning":
		logger.SetLevel(logrus.WarnLevel)
	case "info":
		logger.SetLevel(logrus.InfoLevel)
	case "debug":
		logger.SetLevel(logrus.DebugLevel)
	case "trace":
		logger.SetLevel(logrus.TraceLevel)
	default:
		logger.SetLevel(logrus.TraceLevel)
	}

	logFormat := config.GetConfig().App.LogFormat
	switch strings.ToLower(logFormat) {
	case "text":
		logger.SetFormatter(&logrus.TextFormatter{
			FullTimestamp: true,
		})
	case "json":
		logger.SetFormatter(&logrus.JSONFormatter{})
	default:
		logger.SetFormatter(&CSVFormatter{
			TimestampFormat: "2006-01-02 15:04:05.000 MST",
		})
	}

	logger.SetOutput(&lumberjack.Logger{
		Filename:   logPath,
		MaxSize:    10,
		MaxAge:     30,
		MaxBackups: 30,
		Compress:   true,
	})
	logger.SetReportCaller(true)

	// Cache the logger
	loggers[logFileName] = logger

	return logger
}

// GetLogger returns a logger for a specific name/module
// Example: GetLogger("main") returns logger writing to main.log
func GetLogger(name string) *logrus.Logger {
	if name == "" {
		name = strings.TrimSuffix(config.GetConfig().App.SystemLogFilename, ".log")
	}

	logFileName := name
	if !strings.HasSuffix(logFileName, ".log") {
		logFileName = name + ".log"
	}

	return GetLoggerForFile(logFileName)
}
