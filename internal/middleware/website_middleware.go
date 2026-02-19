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
	"service-platform/internal/pkg/fun"
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

var webSession = &sync.Map{}

type bodyLogWriter struct {
	gin.ResponseWriter
	body *bytes.Buffer
}

func GetWebSession() *sync.Map {
	return webSession
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
		webSession := GetWebSession()
		acessUsername := "UNKNOWN"
		sessionID := "NO_SESSION"
		for _, cookie := range c.Request.Cookies() {
			if cookie.Name == "credentials" {
				sessionID = cookie.Value
				if value, ok := webSession.Load(cookie.Value); ok {
					if admin, ok := value.(model.Users); ok {
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

func CacheControlMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Server`", "SWS")

		hasPrefix := false

		if c.Request.Method == "GET" {
			requestURI := c.Request.RequestURI

			if !strings.HasPrefix(requestURI, config.GLOBAL_URL+"web/") {

				prefixes := []string{
					config.GLOBAL_URL + "assets/",
					config.GLOBAL_URL + "dist/",
					config.GLOBAL_URL + "fonts/",
					config.GLOBAL_URL + "js/",
					config.GLOBAL_URL + "libs/",
					config.GLOBAL_URL + "scss/",
				}

				for _, prefix := range prefixes {
					if strings.HasPrefix(requestURI, prefix) {
						c.Header("Cache-Control", fmt.Sprintf("public, max-age=%d", config.CACHE_MAX_AGE))
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

func AuthMiddleware(db *gorm.DB, redisDB *redis.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		userAgent := c.GetHeader("User-Agent")

		if userAgent == "" {
			logrus.Warn("Blocked Because No User-Agent")
			fun.HandleAPIErrorSimple(c, http.StatusUnauthorized, "Unauthorized")
			return
		}

		cookies := c.Request.Cookies()

		// Parse JWT token from cookie
		tokenString, err := c.Cookie("token")
		if err != nil {
			fun.ClearCookiesAndRedirect(c, cookies)
			return
		}
		tokenString = strings.ReplaceAll(tokenString, " ", "+")

		decrypted, err := fun.GetAESDecrypted(tokenString)
		if err != nil {
			logrus.Warn("Error during decryption", err)
			fun.ClearCookiesAndRedirect(c, cookies)
			return
		}
		var claims map[string]interface{}
		err = json.Unmarshal(decrypted, &claims)
		if err != nil {
			logrus.Warn("Error during unmarshalling", err)
			fun.ClearCookiesAndRedirect(c, cookies)
			return
		}
		emailToken := claims["email"].(string)
		if emailToken == "" {
			fun.ClearCookiesAndRedirect(c, cookies)
			return
		}

		loginTime := config.ServicePlatform.Get().App.LoginTimeM
		if loginTime == 0 {
			loginTime = 15 // Default to 15 minutes if parsing or env is not set
		}

		// Retrieve the last activity time from Redis
		lastActivityTimeStr, err := redisDB.Get(context.Background(), "last_activity_time:"+emailToken).Result()
		if err != nil {
			// Handle missing or erroneous last activity time, default to expired
			lastActivityTimeStr = "0"
		}

		// Convert the last activity time to int64 (assuming it's stored as Unix milliseconds)
		lastActivityTime, err := strconv.ParseInt(lastActivityTimeStr, 10, 64)
		if err != nil {
			// If conversion fails, assume the session expired
			lastActivityTime = 0
		}

		// Get the current time in Unix milliseconds
		currentTime := time.Now().UnixMilli()

		// Check if the time difference exceeds the login expiration threshold
		if currentTime-lastActivityTime > int64(loginTime*60*1000) {
			sessEmpty := map[string]any{
				"session":    "",
				"last_login": nil,
			}

			// Invalidate the user session
			result := db.Model(&model.Users{}).Where("email = ?", emailToken).Updates(sessEmpty)

			if result.Error != nil {
				fun.HandleAPIErrorSimple(c, http.StatusInternalServerError, "Internal Server Error")
				return
			}

			// Close WebSocket connection
			// ws.CloseWebsocketConnection(emailToken)
			return
		}
		errSet := redisDB.Set(context.Background(), "last_activity_time:"+emailToken, time.Now().UnixMilli(), 30*time.Minute).Err()
		if errSet != nil {
			fun.HandleAPIErrorSimple(c, http.StatusInternalServerError, "Internal Server Error")
			return
		}

		// Validate additional cookies
		if !fun.ValidateCookie(c, "credentials", claims["session"]) ||
			!fun.ValidateCookie(c, "auth", claims["auth"]) ||
			!fun.ValidateCookie(c, "random", claims["random"]) {
			fun.ClearCookiesAndRedirect(c, cookies)
			return
		}

		var user model.Users
		if err := db.Where("id = ? AND session = ?", claims["id"], claims["session"]).First(&user).Error; err != nil {
			fun.ClearCookiesAndRedirect(c, cookies)
			fun.HandleAPIErrorSimple(c, http.StatusInternalServerError, "Error querying database: "+err.Error())
			return

			// Handle other errors
		}
		if user.ID == 0 || user.Session == "" {
			fun.ClearCookiesAndRedirect(c, cookies)
			for _, cookie := range cookies {
				cookie.Expires = time.Now().AddDate(0, 0, -1)
				http.SetCookie(c.Writer, cookie)
			}
			return
		}

		// Get the credentials from cookies
		credentials, err := c.Cookie("credentials")
		if err != nil {
			fun.HandleAPIErrorSimple(c, http.StatusBadRequest, "Missing credentials cookie")
			c.Abort()
			return
		}

		// Build the Redis key
		redisKey := "web:" + credentials

		// Check if there is data with the key in Redis
		data, err := redisDB.Get(context.Background(), redisKey).Result()
		if err == redis.Nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "No data found for the given credentials"})
			c.Abort()
			return
		} else if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error retrieving data from Redis, details: " + err.Error()})
			c.Abort()
			return
		}

		// Parse the access from the path
		access := c.Param("access")
		access = strings.ReplaceAll(access, "/", "")
		access = strings.ReplaceAll(access, "..", "")

		// Compare the access value with the data from Redis
		if data != access {
			if config.ServicePlatform.Get().App.Debug {
				c.JSON(http.StatusUnauthorized, gin.H{"error": fmt.Sprintf("Access not allowed coz %s != %s", data, access)})
			} else {
				c.JSON(http.StatusUnauthorized, gin.H{"error": "Access not allowed"})
			}
			c.Abort()
			return
		}
		paths := strings.Split(c.Request.URL.Path, "/")

		// Print the paths
		for _, part := range paths {
			// fmt.Printf("Part %d: %s\n", i, part)
			if strings.Contains(part, "tab-") {

				// fmt.Println("method :", c.Request.Method, "Part :", part)
				path, ok := claims[part].(string)
				if !ok {
					c.JSON(http.StatusNotFound, gin.H{"error": "access tab not found, try to check your permissions or path exists"})
					c.Abort()
					return
				}
				if path == "" { // check if parsing error
					c.JSON(http.StatusForbidden, gin.H{"error": "Forbidden"})
					return
				}
				index := 0
				switch c.Request.Method {
				case http.MethodGet:
					index = 1
				case http.MethodPost:
					if strings.Contains(c.Request.URL.Path, "/create") {
						index = 0
					} else {
						index = 1
					}
				case http.MethodPut, http.MethodPatch:
					index = 2
				case http.MethodDelete:
					index = 3
				default:
					c.JSON(http.StatusMethodNotAllowed, gin.H{"error": "Method not allowed"})
					c.Abort()
					return
				}

				if string(path[index]) != "1" {
					c.Abort()
					c.JSON(http.StatusForbidden, gin.H{"error": "Forbidden"})
					return
				}
				break
			}
		}
		// If everything matches, proceed with the request
		c.Next()
	}
}
