package middleware

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"
	"service-platform/cmd/technical_assistance/model"
	"service-platform/cmd/technical_assistance/shared"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/mssola/user_agent"
)

func LoggerMiddleware(logFile *os.File) gin.HandlerFunc {
	return func(c *gin.Context) {

		// GET USER ACCESS
		webSession := shared.GetWebSession()
		acessUsername := "UNKNOWN"
		cookies := c.Request.Cookies()
		var credentialsCookie *http.Cookie
		for _, cookie := range cookies {
			if cookie.Name == "credentials" {
				credentialsCookie = cookie
				break
			}
		}

		if credentialsCookie != nil {
			if value, ok := webSession.Load(credentialsCookie.Value); ok {
				acessUsername = value.(model.Admin).Username
			}
		}

		// Start timer
		start := time.Now()

		// Process request
		c.Next()

		// Calculate latency
		latency := time.Since(start)

		// Get status code
		status := c.Writer.Status()

		// Get request and response details
		requestMethod := c.Request.Method
		requestURI := c.Request.RequestURI
		clientIP := c.ClientIP()
		userAgent := c.Request.UserAgent()

		ua := user_agent.New(userAgent)
		// Parsing browser information
		browser, version := ua.Browser()
		os := ua.OS()

		// Get the request body
		bodyBytes, err := c.GetRawData()
		if err != nil {
			fmt.Fprintf(logFile, "Error reading request body: %v\n", err)
			c.AbortWithStatus(http.StatusBadRequest)
			return
		}
		bodyString := string(bodyBytes)

		// Restore the io.ReadCloser to its original state
		c.Request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

		// Get the request headers
		headers := c.Request.Header
		headersString := ""
		if requestURI == "/api/merchants/login" {
			for key, values := range headers {
				for _, value := range values {
					headersString += fmt.Sprintf("%s: %s --", key, value)
				}
			}
		}

		// Log the data in your preferred format
		fmt.Fprintf(logFile, "[LOG] %v | %-7s | %3d | %13v | %15s | %10s | %-7s %-9s | %s | %s | H:\n%s | B: %s\n",
			start.Format("2006/01/02 - 15:04:05"),
			requestMethod,
			status,
			latency,
			clientIP,
			os,
			browser,
			version,
			acessUsername,
			requestURI,
			headersString, // the headers string
			bodyString,    // the body string
		)
	}
}
