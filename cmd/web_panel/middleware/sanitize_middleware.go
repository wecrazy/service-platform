package middleware

import (
	"bytes"
	"encoding/json"
	"io"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/microcosm-cc/bluemonday"
)

func sanitizeString(value string) string {
	var sanitized string
	for _, char := range value {
		switch char {
		case '<':
			sanitized += "&lt;"
		case '>':
			sanitized += "&gt;"
		default:
			sanitized += string(char)
		}
	}
	return sanitized
}

// SanitizeJSONStrings recursively sanitizes all string values in a map
func SanitizeJSONStrings(data interface{}, p *bluemonday.Policy) interface{} {
	switch v := data.(type) {
	case map[string]interface{}:
		for key, value := range v {
			v[key] = SanitizeJSONStrings(value, p)
		}
	case []interface{}:
		for i, value := range v {
			v[i] = SanitizeJSONStrings(value, p)
		}
	case string:
		v = sanitizeString(v)
		return p.Sanitize(v)
	}
	return data
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
					c.Request.PostForm[key][i] = sanitizeString(value)
				}
			}
		}
		bodyBytes, err := io.ReadAll(c.Request.Body)
		if err == nil && len(bodyBytes) > 0 {
			var jsonData interface{}
			if err := json.Unmarshal(bodyBytes, &jsonData); err == nil {
				// Sanitize string values
				sanitizedData := SanitizeJSONStrings(jsonData, p)

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
