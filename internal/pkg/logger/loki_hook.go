package logger

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"service-platform/internal/config"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

// LokiHook sends logs to Grafana Loki
type LokiHook struct {
	url        string
	labels     map[string]string
	batch      []*LokiLogEntry
	batchMutex sync.Mutex
	batchSize  int
	batchTimer *time.Ticker
	done       chan struct{}
	client     *http.Client
}

// LokiLogEntry represents a single log entry
type LokiLogEntry struct {
	Timestamp int64
	Line      string
}

var lokiHookInstance *LokiHook
var lokiHookMutex sync.Mutex

// InitLokiHook initializes the Loki hook with the provided configuration
// Returns nil if Loki is disabled, allowing for graceful degradation
func InitLokiHook() (*LokiHook, error) {
	lokiHookMutex.Lock()
	defer lokiHookMutex.Unlock()

	cfg := config.GetConfig()

	// Check if Loki is enabled
	if !cfg.Observability.Loki.Enabled {
		return nil, nil
	}

	// Skip if already initialized
	if lokiHookInstance != nil {
		return lokiHookInstance, nil
	}

	lokiCfg := cfg.Observability.Loki

	// Validate URL
	if lokiCfg.URL == "" {
		log.Printf("⚠️ Loki URL is empty (logging to Loki disabled)")
		return nil, fmt.Errorf("loki URL is empty")
	}

	hook := &LokiHook{
		url:       lokiCfg.URL,
		labels:    lokiCfg.Labels,
		batch:     make([]*LokiLogEntry, 0, lokiCfg.BatchSize),
		batchSize: lokiCfg.BatchSize,
		done:      make(chan struct{}),
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}

	// Start batch flush timer
	hook.batchTimer = time.NewTicker(time.Duration(lokiCfg.BatchTimeoutMs) * time.Millisecond)
	log.Printf("⏰ Created Loki flush timer: interval=%dms", lokiCfg.BatchTimeoutMs)
	go hook.flushBatchRoutine()

	lokiHookInstance = hook
	log.Printf("✅ Loki hook initialized successfully: %s", lokiCfg.URL)

	return hook, nil
}

// GetLokiHook returns the Loki hook instance
func GetLokiHook() *LokiHook {
	lokiHookMutex.Lock()
	defer lokiHookMutex.Unlock()
	return lokiHookInstance
}

// Fire is called when a log entry is written
func (h *LokiHook) Fire(entry *logrus.Entry) error {
	if h == nil {
		return nil
	}

	// Format the log message
	logLine := formatLogForLoki(entry)
	logEntry := &LokiLogEntry{
		Timestamp: entry.Time.UnixNano(),
		Line:      logLine,
	}

	// Add to batch
	h.batchMutex.Lock()
	h.batch = append(h.batch, logEntry)
	batchLen := len(h.batch)
	shouldFlush := batchLen >= h.batchSize
	h.batchMutex.Unlock()

	// Flush if batch is full
	if shouldFlush {
		_ = h.Flush()
	}

	return nil
}

// Levels returns the log levels this hook handles
func (h *LokiHook) Levels() []logrus.Level {
	return logrus.AllLevels
}

// Flush sends all buffered logs to Loki
func (h *LokiHook) Flush() error {
	if h == nil {
		return nil
	}

	h.batchMutex.Lock()
	batchLen := len(h.batch)
	if batchLen == 0 {
		h.batchMutex.Unlock()
		return nil
	}

	batch := h.batch
	h.batch = make([]*LokiLogEntry, 0, h.batchSize)
	h.batchMutex.Unlock()

	// Send batch to Loki
	go h.sendBatchToLoki(batch)

	return nil
}

// sendBatchToLoki sends a batch of logs to Loki
func (h *LokiHook) sendBatchToLoki(entries []*LokiLogEntry) {
	if len(entries) == 0 {
		return
	}

	// Build Loki push request
	request := h.buildLokiPushRequest(entries)
	if request == nil {
		log.Printf("⚠️ Failed to build Loki push request")
		return
	}

	// Create HTTP request
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	httpReq, err := http.NewRequestWithContext(ctx, "POST", h.url+"/loki/api/v1/push", request)
	if err != nil {
		log.Printf("⚠️ Failed to create Loki request: %v", err)
		return
	}

	httpReq.Header.Set("Content-Type", "application/json")

	// Send request
	resp, err := h.client.Do(httpReq)
	if err != nil {
		log.Printf("⚠️ Failed to send logs to Loki: %v", err)
		return
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		log.Printf("⚠️ Loki returned status %d: %s", resp.StatusCode, string(body))
		return
	}
}

// buildLokiPushRequest builds a request body for Loki push API
func (h *LokiHook) buildLokiPushRequest(entries []*LokiLogEntry) io.Reader {
	// Build Loki push request in JSON format
	// Format: {streams: [{stream: {...}, values: [["ts_nanoseconds", "line"]]}]}

	var buf bytes.Buffer
	buf.WriteString(`{"streams":[{"stream":{`)

	// Add labels as JSON object
	first := true
	for k, v := range h.labels {
		if !first {
			buf.WriteString(",")
		}
		fmt.Fprintf(&buf, `"%s":"%s"`, k, v)
		first = false
	}

	buf.WriteString(`},"values":[`)

	// Add entries (values format: [timestamp_ns, line])
	first = true
	for _, entry := range entries {
		if !first {
			buf.WriteString(",")
		}
		// Loki expects nanosecond timestamp
		fmt.Fprintf(&buf, `["%d","%s"]`, entry.Timestamp, escapeJSON(entry.Line))
		first = false
	}

	buf.WriteString(`]}]}`)

	return &buf
}

// escapeJSON escapes special characters for JSON
func escapeJSON(s string) string {
	var buf bytes.Buffer
	for _, r := range s {
		switch r {
		case '"':
			buf.WriteString(`\"`)
		case '\\':
			buf.WriteString(`\\`)
		case '\n':
			buf.WriteString(`\n`)
		case '\r':
			buf.WriteString(`\r`)
		case '\t':
			buf.WriteString(`\t`)
		default:
			buf.WriteRune(r)
		}
	}
	return buf.String()
}

// flushBatchRoutine periodically flushes the batch
func (h *LokiHook) flushBatchRoutine() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			h.Flush()
		case <-h.done:
			h.Flush()
			return
		}
	}
}

// Close gracefully closes the Loki hook
func (h *LokiHook) Close() error {
	if h == nil {
		return nil
	}

	if h.batchTimer != nil {
		h.batchTimer.Stop()
	}

	close(h.done)
	_ = h.Flush()

	return nil
}

// formatLogForLoki formats a logrus entry for Loki
func formatLogForLoki(entry *logrus.Entry) string {
	timestamp := entry.Time.Format("2006-01-02T15:04:05.000Z07:00")
	level := entry.Level.String()
	caller := ""

	if entry.HasCaller() {
		caller = fmt.Sprintf("%s:%d", entry.Caller.Function, entry.Caller.Line)
	}

	// Build the log message with fields
	message := entry.Message
	if len(entry.Data) > 0 {
		message = fmt.Sprintf("%s %+v", message, entry.Data)
	}

	return fmt.Sprintf("[%s] %s %s - %s", timestamp, level, caller, message)
}
