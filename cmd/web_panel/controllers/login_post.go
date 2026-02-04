package controllers

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"net/url"
	"service-platform/cmd/web_panel/fun"
	"service-platform/cmd/web_panel/model"
	"service-platform/internal/config"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"gorm.io/gorm"
)

func PostWebLogin(db *gorm.DB, redisDB *redis.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		userAgent := c.GetHeader("User-Agent")
		accept := c.GetHeader("Accept")
		// acceptLanguage := c.GetHeader("Accept-Language")
		// referer := c.GetHeader("Referer")
		// host := c.GetHeader("Host")

		if userAgent == "" || accept == "" {
			fmt.Println("Blocked Because No this aspect ", userAgent, "|", accept, "|")
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Wrong Username or Password"})
			return
		}

		var loginForm struct {
			EmailUsername string   `form:"email-username" binding:"required"`
			Password      string   `form:"password" binding:"required"`
			Captcha       string   `form:"captcha"`
			RememberMe    bool     `form:"remember-me"`
			TimeEvent     []int64  `form:"-"`
			CaptchaEvent  []string `form:"-"`
		}

		// Bind form data to the LoginForm struct
		if err := c.ShouldBind(&loginForm); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		// captchaID, err := c.Cookie("halo")
		// if err != nil {
		// 	c.Redirect(http.StatusSeeOther, fun.GLOBAL_URL+"login")
		// 	return
		// }
		// if !captcha.VerifyString(captchaID, loginForm.Captcha) {
		// 	// Captcha solution is incorrect
		// 	c.JSON(http.StatusUnauthorized, gin.H{"error": "Wrong Username or Password or captcha."})
		// 	return
		// }

		// timeData := c.PostForm("time-event")
		// textData := c.PostForm("captcha-event")
		// // Unmarshal timeData into timeArray
		// if err := json.Unmarshal([]byte(timeData), &loginForm.TimeEvent); err != nil {
		// 	c.JSON(http.StatusInternalServerError, gin.H{"error": "Error unmarshaling timeData:" + err.Error()})
		// 	return
		// }
		// if err := json.Unmarshal([]byte(textData), &loginForm.CaptchaEvent); err != nil {
		// 	c.JSON(http.StatusInternalServerError, gin.H{"error": "Error unmarshaling timeData:" + err.Error()})
		// 	return
		// }

		// if loginForm.CaptchaEvent[len(loginForm.CaptchaEvent)-1] == loginForm.Captcha {
		// 	fmt.Println(loginForm.CaptchaEvent[len(loginForm.CaptchaEvent)-1])
		// 	fmt.Println(loginForm.Captcha)
		// 	// Loop through the array backward
		// 	for i := len(loginForm.CaptchaEvent) - 1; i >= 0; i-- {
		// 		if i != 0 {
		// 			timeGap := loginForm.TimeEvent[i] - loginForm.TimeEvent[i-1]
		// 			fmt.Println(timeGap)
		// 			if timeGap < 30 {
		// 				fmt.Println("Login Failed Time Gap Below 40 ms")
		// 				c.JSON(http.StatusUnauthorized, gin.H{"error": "Wrong Username or Password or captcha.."})
		// 				return
		// 			}
		// 		}
		// 	}
		// 	if len(loginForm.CaptchaEvent) < 6 {
		// 		c.JSON(http.StatusUnauthorized, gin.H{"error": "Wrong Username or Password or Captcha..."})
		// 		return
		// 	}
		// } else {
		// 	c.JSON(http.StatusUnauthorized, gin.H{"error": "Wrong Username or Password or Captcha...."})
		// 	return
		// }

		var admin model.Admin

		// Check if the login is attempted with an email or username
		whereQuery := ""
		if strings.Contains(loginForm.EmailUsername, "@") {
			whereQuery = "Email = ? "
		} else {
			whereQuery = "Username = ? "

		}
		if err := db.Where(whereQuery, loginForm.EmailUsername).First(&admin).Error; err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Wrong Username or Password."})
			return
		}

		// Reset attempts if lockout period has passed
		if admin.LockUntil != nil && admin.LockUntil.Before(time.Now()) {
			admin.LoginAttempts = 0
			admin.LockUntil = nil
			db.Save(&admin)
		}

		// Check if account is currently locked
		if admin.LockUntil != nil && admin.LockUntil.After(time.Now()) {
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error": "Account locked. Try again at " + admin.LockUntil.Format("2006-01-02 15:04:05"),
			})
			return
		}

		if !fun.IsPasswordMatched(loginForm.Password, admin.Password) {
			admin.LoginAttempts++
			now := time.Now()
			admin.LastFailedLogin = &now

			// Lock the account if over the retry limit
			if admin.LoginAttempts >= admin.MaxRetry {
				lockUntil := now.Add(time.Duration(config.WebPanel.Get().App.LoginLockUntil) * time.Minute)
				admin.LockUntil = &lockUntil
			}

			db.Save(&admin)

			locked := ""
			if admin.LockUntil != nil && admin.LockUntil.After(time.Now()) {
				locked = fmt.Sprintf(" Account locked until %s.", admin.LockUntil.Format("2006-01-02 15:04:05"))
			}

			errorMessage := fmt.Sprintf(
				"Wrong Username or Password or Captcha. Attempt %d of %d.%s",
				admin.LoginAttempts, admin.MaxRetry, locked,
			)

			c.JSON(http.StatusUnauthorized, gin.H{
				"error": errorMessage,
			})
			return
		}

		// LINE AFTER USER VERIFIED LOGIN...__________________________________________________________________________

		// Reset failed login attempts and lock
		admin.LoginAttempts = 0
		admin.LockUntil = nil

		errSet := redisDB.Set(context.Background(), "last_activity_time:"+admin.Email, time.Now().UnixMilli(), 30*time.Minute).Err()
		if errSet != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Internal Server Error, Error Saving to Memory : " + errSet.Error()})
			return
		}

		// Check if its being activated
		var adminStatuses []model.AdminStatus
		db.Find(&adminStatuses)
		for _, adminStatus := range adminStatuses {
			if adminStatus.ID == uint(admin.Status) {
				if adminStatus.Title != "ACTIVE" {
					c.JSON(http.StatusUnauthorized, gin.H{"error": fmt.Sprintf("Please Contact Admin / Technical Support To Activate your Account @+%s", config.WebPanel.Get().Whatsmeow.WaTechnicalSupport)})
					return
				}
			}
		}

		// Set session expiration time (e.g., 7 days in the future)
		currentUnixTime := time.Now().Unix() * 1000               // Convert to milliseconds
		futureTime := currentUnixTime + (7 * 24 * 60 * 60 * 1000) // 7 days in milliseconds
		admin.SessionExpired = futureTime
		admin.Session = fun.GenerateRandomString(40)
		admin.UpdatedAt = time.Now()

		// SAVE LOGIN SESSION
		if err := db.Save(&admin).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update member"})
			return
		}
		// ws.CloseWebsocketConnection(admin.Email)

		var user_roles []struct {
			model.RolePrivilege
			Path string `json:"path" gorm:"column:path"`
		}

		if err := db.
			Table("role_privileges rp").
			Unscoped(). // Disable soft deletes for this query
			Select("rp.*,f.path").
			Joins("LEFT JOIN features f ON f.id = rp.feature_id").
			Where("rp.role_id = ?", admin.Role).
			// Offset(0).
			// Limit(1).
			Find(&user_roles).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "Error querying database",
			})
			return
		}

		// Initialize the dynamic map
		roleData := make(map[string]interface{})

		// Populate the map with path and privilege string
		for _, role := range user_roles {
			// fmt.Printf("RolePrivilege: %+v, Path: %s\n", role.RolePrivilege, role.Path)
			privilege := strconv.Itoa(int(role.Create)) + strconv.Itoa(int(role.Read)) + strconv.Itoa(int(role.Update)) + strconv.Itoa(int(role.Delete))
			roleData[role.Path] = privilege
		}

		var roles model.Role
		if err := db.Where("id = ?", admin.Role).First(&roles).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal Server Error, Saving To DB : " + err.Error()})
			return
		}
		authToken := fun.GenerateRandomString(40 + rand.Intn(25) + 1)
		randomToken := fun.GenerateRandomString(40 + rand.Intn(25) + 1)

		created_at_str := admin.CreatedAt.Format(fun.T_DD_MMMM_YYYY)
		profile_image := "/assets/img/avatars/default.jpg"
		if admin.ProfileImage != "" {
			profile_image = admin.ProfileImage
		}
		imageMaps := map[string]interface{}{
			"t":  fun.GenerateRandomString(3),
			"id": admin.ID,
		}
		pathString, err := fun.GetAESEcryptedURLfromJSON(imageMaps)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not encripting image " + err.Error()})
			return
		}
		// profile_image = "/profile/default.jpg?f=" + pathString
		// Only override profile_image if the admin doesn't have a custom one
		if admin.ProfileImage == "" {
			profile_image = "/profile/default.jpg?f=" + pathString
		}
		// Create jwt.MapClaims and merge with roleData
		claims := map[string]interface{}{
			"id":              admin.ID,
			"fullname":        admin.Fullname,
			"username":        admin.Username,
			"phone":           admin.Phone,
			"email":           admin.Email,
			"password":        admin.Password,
			"type":            admin.Type,
			"role":            admin.Role,
			"role_name":       roles.RoleName,
			"profile_image":   profile_image,
			"status":          admin.Status,
			"status_name":     "",
			"created_at_str":  created_at_str,
			"last_login":      time.Now().Format(fun.T_YYYYMMDD_HHmmss),
			"session":         admin.Session,
			"session_expired": admin.SessionExpired,
			"random":          randomToken,
			"auth":            authToken,
			"ip":              admin.IP,
		}

		// Merge roleData into claims
		for k, v := range roleData {
			claims[k] = v
		}

		jsonText, err := json.Marshal(claims)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not string token " + err.Error()})
			return
		}
		tokenString, err := fun.GetAESEncrypted(string(jsonText))
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not encripting token " + err.Error()})
			return
		}

		loginTimeStr := config.WebPanel.Get().App.LoginTimeM

		// Parse the login time as an integer
		loginExpiredMinutes, err := strconv.Atoi(loginTimeStr)
		if err != nil {
			loginExpiredMinutes = 60
		}

		// Calculate the expiration time by adding loginExpiredMinutes to the current time
		expiration := time.Now().Add(time.Duration(loginExpiredMinutes) * time.Minute)
		// Set random token as cookie
		auth := &http.Cookie{
			Name:     "auth",
			Value:    authToken,
			Expires:  expiration,
			Path:     fun.GLOBAL_URL,
			Domain:   config.WebPanel.Get().App.CookieLoginDomain,
			SameSite: http.SameSiteStrictMode,
			Secure:   config.WebPanel.Get().App.CookieLoginSecure,
			HttpOnly: true,
		}
		http.SetCookie(c.Writer, auth)

		// Set random token as cookie
		random := &http.Cookie{
			Name:     "random",
			Value:    randomToken,
			Expires:  expiration,
			Path:     fun.GLOBAL_URL,
			Domain:   config.WebPanel.Get().App.CookieLoginDomain,
			SameSite: http.SameSiteStrictMode,
			Secure:   config.WebPanel.Get().App.CookieLoginSecure,
			HttpOnly: true,
		}
		http.SetCookie(c.Writer, random)

		// Set JWT token as cookie
		tokenCookie := &http.Cookie{
			Name:     "token",
			Value:    tokenString,
			Expires:  expiration,
			Path:     fun.GLOBAL_URL,
			Domain:   config.WebPanel.Get().App.CookieLoginDomain,
			SameSite: http.SameSiteStrictMode,
			Secure:   config.WebPanel.Get().App.CookieLoginSecure,
			HttpOnly: true,
		}
		http.SetCookie(c.Writer, tokenCookie)

		// Create and set the "credentials" cookie
		credentialsCookie := &http.Cookie{
			Name:     "credentials",
			Value:    url.QueryEscape(admin.Session),
			Expires:  expiration,
			Path:     fun.GLOBAL_URL,
			Domain:   config.WebPanel.Get().App.CookieLoginDomain,
			SameSite: http.SameSiteStrictMode,
			Secure:   config.WebPanel.Get().App.CookieLoginSecure,
			HttpOnly: true,
		}
		http.SetCookie(c.Writer, credentialsCookie)

		// syncCookie := &http.Cookie{
		// 	Name:     "jm_id",
		// 	Value:    url.QueryEscape(admin.Session),
		// 	Expires:  expiration,
		// 	Path:     fun.GLOBAL_URL,
		// 	Domain:   config.WebPanel.Get().App.CookieLoginDomain,
		// 	SameSite: http.SameSiteLaxMode,
		// 	Secure:   config.WebPanel.Get().App.CookieLoginSecure,
		// 	HttpOnly: false,
		// }
		// http.SetCookie(c.Writer, syncCookie)
		// c.SecureJSON(http.StatusOK, gin.H{
		// 	"status": "01",
		// })

		// Save the newLog to the database
		db.Create(&model.LogActivity{
			AdminID:   admin.ID,
			FullName:  admin.Fullname,
			Action:    "LOGIN",
			Status:    "Success",
			Log:       "User logged in successfully",
			IP:        c.ClientIP(),
			UserAgent: c.Request.UserAgent(),
			ReqMethod: c.Request.Method,
			ReqUri:    c.Request.RequestURI,
		})

		c.Redirect(http.StatusSeeOther, fun.GLOBAL_URL+"page")
		c.Abort()
	}
}
