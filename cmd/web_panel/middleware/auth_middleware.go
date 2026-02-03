package middleware

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"service-platform/cmd/web_panel/config"
	"service-platform/cmd/web_panel/fun"
	"service-platform/cmd/web_panel/model"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"gorm.io/gorm"
)

func AuthMiddleware(db *gorm.DB, redisDB *redis.Client) gin.HandlerFunc {
	return func(c *gin.Context) {

		userAgent := c.GetHeader("User-Agent")
		// accept := c.GetHeader("Accept")
		// acceptLanguage := c.GetHeader("Accept-Language")
		// referer := c.GetHeader("Referer")
		// host := c.GetHeader("Host")

		if userAgent == "" {
			fmt.Println("Blocked Because No this aspect \n", userAgent, "\n|")
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
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
			fmt.Println("Error during decryption", err)
			fun.ClearCookiesAndRedirect(c, cookies)
			return
		}
		var claims map[string]interface{}
		err = json.Unmarshal(decrypted, &claims)
		if err != nil {
			fmt.Printf("Error converting JSON to map: %v", err)
			fun.ClearCookiesAndRedirect(c, cookies)
			return
		}
		emailToken := claims["email"].(string)
		if emailToken == "" {
			fun.ClearCookiesAndRedirect(c, cookies)
			return
		}

		loginTimeStr := config.GetConfig().App.LoginTimeM
		loginExpiredMinutes, errConv := strconv.ParseInt(loginTimeStr, 10, 64)
		if errConv != nil {
			loginExpiredMinutes = 15 // Default to 15 minutes if parsing or env is not set
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
		if currentTime-lastActivityTime > loginExpiredMinutes*60*1000 {
			// Invalidate the user session
			result := db.Model(&model.Admin{}).Where("email = ?", emailToken).Updates(model.Admin{Session: ""})

			if result.Error != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal Server Error"})
				return
			}

			// fmt.Printf("Rows affected: %d\n", result.RowsAffected)

			// Close WebSocket connection
			// ws.CloseWebsocketConnection(emailToken)
			return
		}
		errSet := redisDB.Set(context.Background(), "last_activity_time:"+emailToken, time.Now().UnixMilli(), 30*time.Minute).Err()
		if errSet != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal Server Error"})
			return
		}

		// Validate additional cookies
		if !fun.ValidateCookie(c, "credentials", claims["session"]) ||
			!fun.ValidateCookie(c, "auth", claims["auth"]) ||
			!fun.ValidateCookie(c, "random", claims["random"]) {
			fun.ClearCookiesAndRedirect(c, cookies)
			return
		}
		var admin model.Admin
		if err := db.Where("id = ? AND session = ?", claims["id"], claims["session"]).First(&admin).Error; err != nil {
			fun.ClearCookiesAndRedirect(c, cookies)
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "Error querying database" + err.Error(),
			})
			return

			// Handle other errors
		}
		if admin.ID == 0 || admin.Session == "" {
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
			c.JSON(http.StatusBadRequest, gin.H{"error": "Missing credentials cookie"})
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
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error retrieving data from Redis"})
			c.Abort()
			return
		}

		// Parse the access from the path
		access := c.Param("access")
		access = strings.ReplaceAll(access, "/", "")
		access = strings.ReplaceAll(access, "..", "")

		// Compare the access value with the data from Redis
		if data != access {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Access not allowed"})
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
					c.JSON(http.StatusNotFound, gin.H{"error": "access tab not found"})
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
