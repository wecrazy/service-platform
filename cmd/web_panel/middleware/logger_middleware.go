package middleware

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"service-platform/cmd/web_panel/model"
	"service-platform/cmd/web_panel/shared"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/mssola/user_agent"
)

type bodyLogWriter struct {
	gin.ResponseWriter
	body *bytes.Buffer
}

func (w bodyLogWriter) Write(b []byte) (int, error) {
	w.body.Write(b) // Save copy for log
	return w.ResponseWriter.Write(b)
}

// generateRequestID creates a unique request ID for tracing
func generateRequestID() string {
	bytes := make([]byte, 8)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)
}

func LoggerMiddleware(logWriter io.Writer) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Generate unique request ID for tracing
		requestID := generateRequestID()
		c.Set("requestID", requestID)

		// Extract user from session (as you already do)
		webSession := shared.GetWebSession()
		acessUsername := "UNKNOWN"
		sessionID := "NO_SESSION"
		for _, cookie := range c.Request.Cookies() {
			if cookie.Name == "credentials" {
				sessionID = cookie.Value
				if value, ok := webSession.Load(cookie.Value); ok {
					if admin, ok := value.(model.Admin); ok {
						acessUsername = admin.Username
					}
				}
				break
			}
		}

		start := time.Now()

		// Capture request body
		bodyBytes, _ := io.ReadAll(c.Request.Body)
		c.Request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
		bodyString := string(bodyBytes)
		if len(bodyString) > 1000 {
			bodyString = bodyString[:1000] + "... [TRUNCATED]"
		}

		// Capture response body
		blw := &bodyLogWriter{body: bytes.NewBufferString(""), ResponseWriter: c.Writer}
		c.Writer = blw

		c.Next()

		// After request is processed
		latency := time.Since(start)
		status := c.Writer.Status()
		clientIP := c.ClientIP()
		method := c.Request.Method
		uri := c.Request.RequestURI
		contentType := c.ContentType()

		// Get content lengths
		requestContentLength := c.Request.ContentLength
		responseContentLength := int64(blw.body.Len())

		// User agent parsing
		ua := user_agent.New(c.Request.UserAgent())
		browser, version := ua.Browser()
		os := ua.OS()
		userAgent := c.Request.UserAgent()

		// Protocol and scheme info
		protocol := c.Request.Proto
		scheme := "http"
		if c.Request.TLS != nil {
			scheme = "https"
		}
		host := c.Request.Host

		// Referrer
		referrer := c.Request.Referer()
		if referrer == "" {
			referrer = "NONE"
		}

		// Query parameters
		var queryStr string
		for key, values := range c.Request.URL.Query() {
			for _, value := range values {
				queryStr += fmt.Sprintf("%s=%s ", key, value)
			}
		}
		queryStr = strings.TrimSpace(queryStr)
		if queryStr == "" {
			queryStr = "NONE"
		}

		// Format route params
		var routeStr string
		for _, p := range c.Params {
			routeStr += fmt.Sprintf("%s=%s ", p.Key, p.Value)
		}
		routeStr = strings.TrimSpace(routeStr)
		if routeStr == "" {
			routeStr = "NONE"
		}

		// All request headers
		var headersString string
		for k, v := range c.Request.Header {
			for _, val := range v {
				// Mask sensitive headers
				if strings.ToLower(k) == "authorization" || strings.ToLower(k) == "cookie" {
					headersString += fmt.Sprintf("%s: [MASKED] -- ", k)
				} else {
					headersString += fmt.Sprintf("%s: %s -- ", k, val)
				}
			}
		}
		headersString = strings.TrimSuffix(headersString, " -- ")

		// All cookies (masked values for security)
		var cookiesString string
		for _, cookie := range c.Request.Cookies() {
			if cookie.Name == "credentials" {
				cookiesString += fmt.Sprintf("%s=[MASKED] ", cookie.Name)
			} else {
				cookiesString += fmt.Sprintf("%s=%s ", cookie.Name, cookie.Value)
			}
		}
		cookiesString = strings.TrimSpace(cookiesString)
		if cookiesString == "" {
			cookiesString = "NONE"
		}

		// Response headers
		var responseHeadersString string
		for k, v := range c.Writer.Header() {
			for _, val := range v {
				responseHeadersString += fmt.Sprintf("%s: %s -- ", k, val)
			}
		}
		responseHeadersString = strings.TrimSuffix(responseHeadersString, " -- ")
		if responseHeadersString == "" {
			responseHeadersString = "NONE"
		}

		// Response body (truncated for readability)
		responseBody := blw.body.String()
		if len(responseBody) > 500 {
			responseBody = responseBody[:500] + "... [TRUNCATED]"
		}
		if responseBody == "" {
			responseBody = "EMPTY"
		}

		// Errors from Gin context
		ginErrors := c.Errors.String()
		if ginErrors == "" {
			ginErrors = "NONE"
		}

		// Final comprehensive log print
		fmt.Fprintf(logWriter,
			`[LOG] %v | RequestID: %s | %-7s | %3d | %13v | %15s | %s://%s | %s | %-10s | %-7s %-9s | %s | %s
SessionID: %s
Content-Type: %s | ReqSize: %d bytes | RespSize: %d bytes
Referrer: %s
UserAgent: %s
Query: %s
RouteParams: %s
RequestHeaders: %s
Cookies: %s
ResponseHeaders: %s
Errors: %s
RequestBody:
%s
ResponseBody:
%s
────────────────────────────────────────────────────────────────────────────────
`,
			start.Format("2006/01/02 - 15:04:05"),
			requestID,
			method,
			status,
			latency,
			clientIP,
			scheme,
			host,
			protocol,
			os,
			browser,
			version,
			acessUsername,
			uri,
			sessionID,
			contentType,
			requestContentLength,
			responseContentLength,
			referrer,
			userAgent,
			queryStr,
			routeStr,
			headersString,
			cookiesString,
			responseHeadersString,
			ginErrors,
			bodyString,
			responseBody,
		)
	}
}
