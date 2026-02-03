package middleware

import (
	"service-platform/cmd/web_panel/fun"
	"strings"

	"github.com/gin-gonic/gin"
)

func CacheControlMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Server`", "SWS")

		hasPrefix := false

		if c.Request.Method == "GET" {
			requestURI := c.Request.RequestURI

			if !strings.HasPrefix(requestURI, fun.GLOBAL_URL+"web/") {

				prefixes := []string{fun.GLOBAL_URL + "assets/", fun.GLOBAL_URL + "dist/", fun.GLOBAL_URL + "fonts/", fun.GLOBAL_URL + "js/", fun.GLOBAL_URL + "libs/", fun.GLOBAL_URL + "scss/"}
				for _, prefix := range prefixes {
					if strings.HasPrefix(requestURI, prefix) {
						c.Header("Cache-Control", "public, max-age=31536000") // 1 year
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
