package middleware

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"service-platform/internal/config"
	"service-platform/internal/core/model"
	"service-platform/pkg/fun"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"github.com/microcosm-cc/bluemonday"
	"github.com/mssola/user_agent"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// webSession is a global sync.Map used to store active web sessions. It allows concurrent access and modification of session data across different goroutines, ensuring thread safety when managing user sessions in the web application.
var webSession = &sync.Map{}

// bodyLogWriter is a custom ResponseWriter that captures the response body for logging purposes. It embeds gin.ResponseWriter and adds a bytes.Buffer to store the response body.
type bodyLogWriter struct {
	gin.ResponseWriter
	body *bytes.Buffer
}

// GetWebSession returns the global sync.Map used to store active web sessions.
func GetWebSession() *sync.Map {
	return webSession
}

// Write overrides the default Write method of the Gin ResponseWriter to capture the response body for logging purposes. It writes the response to both the original ResponseWriter and a buffer for later retrieval.
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

// extractSessionInfo extracts the username and session ID from the request cookies.
func extractSessionInfo(c *gin.Context) (username, sessionID string) {
	username = "UNKNOWN"
	sessionID = "NO_SESSION"
	session := GetWebSession()
	for _, cookie := range c.Request.Cookies() {
		if cookie.Name == "credentials" {
			sessionID = cookie.Value
			if value, ok := session.Load(cookie.Value); ok {
				if admin, ok2 := value.(model.Users); ok2 {
					username = admin.Username
				}
			}
			break
		}
	}
	return username, sessionID
}

// buildQueryString formats the URL query parameters as a single string.
func buildQueryString(c *gin.Context) string {
	var queryStr string
	for key, values := range c.Request.URL.Query() {
		for _, value := range values {
			queryStr += fmt.Sprintf("%s=%s ", key, value)
		}
	}
	queryStr = strings.TrimSpace(queryStr)
	if queryStr == "" {
		return "NONE"
	}
	return queryStr
}

// buildRouteParamString formats the route parameters as a single string.
func buildRouteParamString(c *gin.Context) string {
	var routeStr string
	for _, p := range c.Params {
		routeStr += fmt.Sprintf("%s=%s ", p.Key, p.Value)
	}
	routeStr = strings.TrimSpace(routeStr)
	if routeStr == "" {
		return "NONE"
	}
	return routeStr
}

// buildHeadersString formats HTTP request headers, masking sensitive ones.
func buildHeadersString(header http.Header) string {
	var headersString string
	for k, v := range header {
		for _, val := range v {
			if strings.ToLower(k) == "authorization" || strings.ToLower(k) == "cookie" {
				headersString += fmt.Sprintf("%s: [MASKED] -- ", k)
			} else {
				headersString += fmt.Sprintf("%s: %s -- ", k, val)
			}
		}
	}
	return strings.TrimSuffix(headersString, " -- ")
}

// buildCookiesString formats cookies, masking the credentials cookie value.
func buildCookiesString(cookies []*http.Cookie) string {
	var cookiesString string
	for _, cookie := range cookies {
		if cookie.Name == "credentials" {
			cookiesString += fmt.Sprintf("%s=[MASKED] ", cookie.Name)
		} else {
			cookiesString += fmt.Sprintf("%s=%s ", cookie.Name, cookie.Value)
		}
	}
	cookiesString = strings.TrimSpace(cookiesString)
	if cookiesString == "" {
		return "NONE"
	}
	return cookiesString
}

// buildResponseHeadersString formats HTTP response headers as a single string.
func buildResponseHeadersString(header http.Header) string {
	var responseHeadersString string
	for k, v := range header {
		for _, val := range v {
			responseHeadersString += fmt.Sprintf("%s: %s -- ", k, val)
		}
	}
	responseHeadersString = strings.TrimSuffix(responseHeadersString, " -- ")
	if responseHeadersString == "" {
		return "NONE"
	}
	return responseHeadersString
}

// LoggerMiddleware is a Gin middleware that logs HTTP request and response details.
func LoggerMiddleware(logWriter io.Writer) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Generate unique request ID for tracing
		requestID := generateRequestID()
		c.Set("requestID", requestID)

		acessUsername, sessionID := extractSessionInfo(c)
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

		queryStr := buildQueryString(c)
		routeStr := buildRouteParamString(c)
		headersString := buildHeadersString(c.Request.Header)
		cookiesString := buildCookiesString(c.Request.Cookies())
		responseHeadersString := buildResponseHeadersString(c.Writer.Header())

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

// CacheControlMiddleware is a Gin middleware that sets HTTP cache control headers.
func CacheControlMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Server", "SWS")

		hasPrefix := false

		if c.Request.Method == "GET" {
			requestURI := c.Request.RequestURI

			if !strings.HasPrefix(requestURI, config.GlobalURL+"web/") {

				prefixes := []string{
					config.GlobalURL + "assets/",
					config.GlobalURL + "dist/",
					config.GlobalURL + "fonts/",
					config.GlobalURL + "js/",
					config.GlobalURL + "libs/",
					config.GlobalURL + "scss/",
				}

				for _, prefix := range prefixes {
					if strings.HasPrefix(requestURI, prefix) {
						c.Header("Cache-Control", fmt.Sprintf("public, max-age=%d", config.CacheMaxAge))
						c.Next()
						hasPrefix = true
						break
					}
				}
			}
		}
		if !hasPrefix {
			c.Header("Cache-Control", "no-store")
		}
		c.Next()
	}
}

// SanitizeMiddleware is a Gin middleware that sanitizes request body inputs to prevent injection attacks.
func SanitizeMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		switch c.Request.Method {
		case "POST", "PUT", "PATCH":
			// fmt.Println("ONLY ACCEPT body RAW JSON & FORM DATA")
		}
		p := bluemonday.UGCPolicy()

		// Sanitize query parameters if they exist
		query := c.Request.URL.Query()
		for key, values := range query {
			for i, value := range values {
				query[key][i] = p.Sanitize(value)
			}
		}
		c.Request.URL.RawQuery = query.Encode()

		// Sanitize form data if the request has form data
		if strings.Contains(c.ContentType(), "application/x-www-form-urlencoded") || strings.Contains(c.ContentType(), "multipart/form-data") {
			c.Request.ParseForm()
			for key, values := range c.Request.PostForm {
				for i, value := range values {
					c.Request.PostForm[key][i] = fun.SanitizeString(value)
				}
			}
		}
		bodyBytes, err := io.ReadAll(c.Request.Body)
		if err == nil && len(bodyBytes) > 0 {
			var jsonData interface{}
			if err := json.Unmarshal(bodyBytes, &jsonData); err == nil {
				// Sanitize string values
				sanitizedData := fun.SanitizeJSONStrings(jsonData, p)

				// Convert sanitized data back to JSON
				sanitizedBodyBytes, err := json.Marshal(sanitizedData)
				if err == nil {
					// Replace request body with sanitized JSON
					c.Request.Body = io.NopCloser(bytes.NewBuffer(sanitizedBodyBytes))
				}
			} else {
				// If not a valid JSON, restore original body
				c.Request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
			}
		}

		c.Next()
	}
}

// SanitizeCsvMiddleware is a Gin middleware that sanitizes CSV inputs in requests.
func SanitizeCsvMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		switch c.Request.Method {
		case "POST", "PUT", "PATCH":
			// Sanitize query parameters if they exist
			query := c.Request.URL.Query()
			for key, values := range query {
				for i, value := range values {
					query[key][i] = fun.SanitizeString(value)
				}
			}
			c.Request.URL.RawQuery = query.Encode()

			// Sanitize form data if the request has form data
			if strings.Contains(c.ContentType(), "application/x-www-form-urlencoded") || strings.Contains(c.ContentType(), "multipart/form-data") {
				c.Request.ParseForm()
				for key, values := range c.Request.PostForm {
					for i, value := range values {
						c.Request.PostForm[key][i] = fun.SanitizeString(value)
					}
				}
			}

			// Sanitize JSON body if present
			bodyBytes, err := io.ReadAll(c.Request.Body)
			if err == nil && len(bodyBytes) > 0 {
				var jsonData interface{}
				if err := json.Unmarshal(bodyBytes, &jsonData); err == nil {
					// Sanitize string values
					sanitizedData := fun.SanitizeJSONCsvStrings(jsonData)

					// Convert sanitized data back to JSON
					sanitizedBodyBytes, err := json.Marshal(sanitizedData)
					if err == nil {
						// Replace request body with sanitized JSON
						c.Request.Body = io.NopCloser(bytes.NewBuffer(sanitizedBodyBytes))
					}
				} else {
					// If not a valid JSON, restore original body
					c.Request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
				}
			}
		}

		c.Next()
	}
}

// SecurityControlMiddleware is a Gin middleware that sets security-related HTTP response headers.
func SecurityControlMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Content-Security-Policy", "frame-ancestors 'none'")
		c.Header("X-Frame-Options", "DENY")
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("Strict-Transport-Security", "max-age=16070400; includeSubDomains")
		c.Header("Referrer-Policy", "no-referrer")
		c.Header("X-XSS-Protection", "1; mode=block")
		c.Next()
	}
}

// resolveAuthClaims extracts and decrypts the auth token cookie, returning the JWT claims.
func resolveAuthClaims(c *gin.Context, cookies []*http.Cookie) (map[string]interface{}, bool) {
	tokenString, err := c.Cookie("token")
	if err != nil {
		fun.ClearCookiesAndRedirect(c, cookies)
		return nil, false
	}
	tokenString = strings.ReplaceAll(tokenString, " ", "+")
	decrypted, err := fun.GetAESDecrypted(tokenString)
	if err != nil {
		logrus.Warn("Error during decryption", err)
		fun.ClearCookiesAndRedirect(c, cookies)
		return nil, false
	}
	var claims map[string]interface{}
	if err = json.Unmarshal(decrypted, &claims); err != nil {
		logrus.Warn("Error during unmarshalling", err)
		fun.ClearCookiesAndRedirect(c, cookies)
		return nil, false
	}
	return claims, true
}

// handleActivityTimeout checks if the session has timed out and invalidates it if so.
// Returns true if the session is expired (caller should abort).
func handleActivityTimeout(c *gin.Context, db *gorm.DB, redisDB *redis.Client, emailToken string, loginTime int) bool {
	lastActivityTimeStr, err := redisDB.Get(context.Background(), "last_activity_time:"+emailToken).Result()
	if err != nil {
		lastActivityTimeStr = "0"
	}
	lastActivityTime, err := strconv.ParseInt(lastActivityTimeStr, 10, 64)
	if err != nil {
		lastActivityTime = 0
	}
	if time.Now().UnixMilli()-lastActivityTime <= int64(loginTime*60*1000) {
		return false
	}
	sessEmpty := map[string]any{"session": "", "last_login": nil}
	result := db.Model(&model.Users{}).Where("email = ?", emailToken).Updates(sessEmpty)
	if result.Error != nil {
		fun.HandleAPIErrorSimple(c, http.StatusInternalServerError, "Internal Server Error")
	}
	return true
}

// validateAuthCookies ensures that the session, auth, and random cookies match the JWT claims.
func validateAuthCookies(c *gin.Context, claims map[string]interface{}, cookies []*http.Cookie) bool {
	if !fun.ValidateCookie(c, "credentials", claims["session"]) ||
		!fun.ValidateCookie(c, "auth", claims["auth"]) ||
		!fun.ValidateCookie(c, "random", claims["random"]) {
		fun.ClearCookiesAndRedirect(c, cookies)
		return false
	}
	return true
}

// loadAndValidateUser fetches the authenticated user from the database and validates the session.
func loadAndValidateUser(c *gin.Context, db *gorm.DB, claims map[string]interface{}, cookies []*http.Cookie) (model.Users, bool) {
	var user model.Users
	if err := db.Where("id = ? AND session = ?", claims["id"], claims["session"]).First(&user).Error; err != nil {
		logrus.WithError(err).Warn("WebSession: error querying user")
		fun.ClearCookiesAndRedirect(c, cookies)
		fun.HandleAPIErrorSimple(c, http.StatusInternalServerError, "Internal Server Error")
		return user, false
	}
	if user.ID == 0 || user.Session == "" {
		fun.ClearCookiesAndRedirect(c, cookies)
		for _, cookie := range cookies {
			cookie.Expires = time.Now().AddDate(0, 0, -1)
			http.SetCookie(c.Writer, cookie)
		}
		return user, false
	}
	return user, true
}

// getRedisAccess retrieves the access value associated with the given credentials from Redis.
func getRedisAccess(c *gin.Context, redisDB *redis.Client, credentials string) (string, bool) {
	data, err := redisDB.Get(context.Background(), "web:"+credentials).Result()
	if err == redis.Nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "No data found for the given credentials"})
		c.Abort()
		return "", false
	} else if err != nil {
		logrus.WithError(err).Warn("WebSession: error retrieving data from Redis")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal Server Error"})
		c.Abort()
		return "", false
	}
	return data, true
}

// httpMethodPermIndex returns the permission bit index for the given HTTP method.
func httpMethodPermIndex(c *gin.Context) (int, bool) {
	switch c.Request.Method {
	case http.MethodGet:
		return 1, true
	case http.MethodPost:
		if strings.Contains(c.Request.URL.Path, "/create") {
			return 0, true
		}
		return 1, true
	case http.MethodPut, http.MethodPatch:
		return 2, true
	case http.MethodDelete:
		return 3, true
	default:
		c.JSON(http.StatusMethodNotAllowed, gin.H{"error": "Method not allowed"})
		c.Abort()
		return 0, false
	}
}

// authorizeTabAccess checks that the request has permission to access the tab segment in its URL path.
func authorizeTabAccess(c *gin.Context, claims map[string]interface{}) bool {
	for _, part := range strings.Split(c.Request.URL.Path, "/") {
		if !strings.Contains(part, "tab-") {
			continue
		}
		path, ok := claims[part].(string)
		if !ok {
			c.JSON(http.StatusNotFound, gin.H{"error": "access tab not found, try to check your permissions or path exists"})
			c.Abort()
			return false
		}
		if path == "" {
			c.JSON(http.StatusForbidden, gin.H{"error": "Forbidden"})
			return false
		}
		index, valid := httpMethodPermIndex(c)
		if !valid {
			return false
		}
		if string(path[index]) != "1" {
			c.Abort()
			c.JSON(http.StatusForbidden, gin.H{"error": "Forbidden"})
			return false
		}
		break
	}
	return true
}

// AuthMiddleware is a Gin middleware that validates session-based authentication and authorization.
func AuthMiddleware(db *gorm.DB, redisDB *redis.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		if userAgent := c.GetHeader("User-Agent"); userAgent == "" {
			logrus.Warn("Blocked Because No User-Agent")
			fun.HandleAPIErrorSimple(c, http.StatusUnauthorized, "Unauthorized")
			return
		}

		cookies := c.Request.Cookies()

		claims, ok := resolveAuthClaims(c, cookies)
		if !ok {
			return
		}

		emailToken := claims["email"].(string)
		if emailToken == "" {
			fun.ClearCookiesAndRedirect(c, cookies)
			return
		}

		loginTime := config.ServicePlatform.Get().App.LoginTimeM
		if loginTime == 0 {
			loginTime = 15
		}

		if handleActivityTimeout(c, db, redisDB, emailToken, loginTime) {
			return
		}

		if err := redisDB.Set(context.Background(), "last_activity_time:"+emailToken, time.Now().UnixMilli(), 30*time.Minute).Err(); err != nil {
			fun.HandleAPIErrorSimple(c, http.StatusInternalServerError, "Internal Server Error")
			return
		}

		if !validateAuthCookies(c, claims, cookies) {
			return
		}

		if _, valid := loadAndValidateUser(c, db, claims, cookies); !valid {
			return
		}

		credentials, err := c.Cookie("credentials")
		if err != nil {
			fun.HandleAPIErrorSimple(c, http.StatusBadRequest, "Missing credentials cookie")
			c.Abort()
			return
		}

		data, dataOK := getRedisAccess(c, redisDB, credentials)
		if !dataOK {
			return
		}

		access := strings.ReplaceAll(strings.ReplaceAll(c.Param("access"), "/", ""), "..", "")
		if data != access {
			if config.ServicePlatform.Get().App.Debug {
				c.JSON(http.StatusUnauthorized, gin.H{"error": fmt.Sprintf("Access not allowed coz %s != %s", data, access)})
			} else {
				c.JSON(http.StatusUnauthorized, gin.H{"error": "Access not allowed"})
			}
			c.Abort()
			return
		}

		if !authorizeTabAccess(c, claims) {
			return
		}

		c.Next()
	}
}
