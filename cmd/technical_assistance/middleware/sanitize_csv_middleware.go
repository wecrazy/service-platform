package middleware

import (
	"bytes"
	"encoding/json"
	"io"
	"strings"

	"github.com/gin-gonic/gin"
)

// sanitizeCsvString applies a simple prefix strategy to avoid CSV injection
func sanitizeCsvString(value string) string {
	if strings.HasPrefix(value, "=") || strings.HasPrefix(value, "+") || strings.HasPrefix(value, "-") || strings.HasPrefix(value, "@") {
		return "'" + value
	}
	return value
}

// SanitizeJSONCsvStrings recursively sanitizes all string values in a map
func SanitizeJSONCsvStrings(data interface{}) interface{} {
	switch v := data.(type) {
	case map[string]interface{}:
		for key, value := range v {
			v[key] = SanitizeJSONCsvStrings(value)
		}
	case []interface{}:
		for i, value := range v {
			v[i] = SanitizeJSONCsvStrings(value)
		}
	case string:
		return sanitizeCsvString(v)
	}
	return data
}

// SanitizeCsvMiddleware returns a Gin middleware function for CSV injection protection
func SanitizeCsvMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		switch c.Request.Method {
		case "POST", "PUT", "PATCH":
			// Sanitize query parameters if they exist
			query := c.Request.URL.Query()
			for key, values := range query {
				for i, value := range values {
					query[key][i] = sanitizeCsvString(value)
				}
			}
			c.Request.URL.RawQuery = query.Encode()

			// Sanitize form data if the request has form data
			if strings.Contains(c.ContentType(), "application/x-www-form-urlencoded") || strings.Contains(c.ContentType(), "multipart/form-data") {
				c.Request.ParseForm()
				for key, values := range c.Request.PostForm {
					for i, value := range values {
						c.Request.PostForm[key][i] = sanitizeCsvString(value)
					}
				}
			}

			// Sanitize JSON body if present
			bodyBytes, err := io.ReadAll(c.Request.Body)
			if err == nil && len(bodyBytes) > 0 {
				var jsonData interface{}
				if err := json.Unmarshal(bodyBytes, &jsonData); err == nil {
					// Sanitize string values
					sanitizedData := SanitizeJSONCsvStrings(jsonData)

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
