// Package controllers provides HTTP handler functions for the web GUI,
// API endpoints, and service integrations used by the service-platform application.
package controllers

import (
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"math/rand"
	"net/http"
	"net/url"
	"service-platform/internal/api/v1/dto"
	"service-platform/internal/config"
	"service-platform/internal/core/model"
	"service-platform/pkg/fun"
	"service-platform/pkg/webguibuilder"
	"strconv"
	"strings"
	"time"

	"github.com/dchest/captcha"
	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"github.com/sirupsen/logrus"
	"gopkg.in/gomail.v2"
	"gorm.io/gorm"
)

// GetWebLogin godoc
// @Summary      Get Login Page
// @Description  Renders the login page
// @Tags         Web
// @Produce      html
// @Success      200  {string}   string "HTML Content"
// @Router       /login [get]
func GetWebLogin(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Retrieve cookies from the request
		cookies := c.Request.Cookies()

		// Check if the "credentials" cookie exists
		var credentialsCookie *http.Cookie
		for _, cookie := range cookies {
			if cookie.Name == "credentials" {
				credentialsCookie = cookie
				break
			}
		}

		yamlCfg := config.ServicePlatform.Get()

		parameters := gin.H{
			"APP_NAME":         yamlCfg.App.Name,
			"APP_LOGO":         yamlCfg.App.Logo,
			"APP_VERSION":      yamlCfg.App.Version,
			"APP_VERSION_NO":   yamlCfg.App.VersionNo,
			"APP_VERSION_CODE": yamlCfg.App.VersionCode,
			"APP_VERSION_NAME": yamlCfg.App.VersionName,
			"CAPTCHA_ID":       captcha.New(),
			"DEBUG":            yamlCfg.App.Debug,
		}

		// Check if the credentials cookie is not nil before accessing its value
		if credentialsCookie != nil && credentialsCookie.Value != "" {
			var user model.Users
			if err := db.Where("session = ?", credentialsCookie.Value).First(&user).Error; err != nil {
				for _, cookie := range cookies {
					cookie.Expires = time.Now().AddDate(0, 0, -1)
					http.SetCookie(c.Writer, cookie)
				}
				c.HTML(http.StatusOK, "login.html", parameters)
				return
			}
			c.Redirect(http.StatusFound, config.GlobalURL+"page")
		} else {
			for _, cookie := range cookies {
				cookie.Expires = time.Now().AddDate(0, 0, -1)
				http.SetCookie(c.Writer, cookie)
			}
			c.HTML(http.StatusOK, "login.html", parameters)
		}
	}
}

// PostWebLogin godoc
// @Summary      Process Login
// @Description  Processes user login credentials
// @Tags         Web
// @Accept       x-www-form-urlencoded
// @Produce      json
// @Param        request formData dto.LoginRequest true "Login Request"
// @Success      200  {object}   map[string]interface{}
// @Failure      400  {object}   dto.APIErrorResponse
// @Failure      401  {object}   dto.APIErrorResponse
// @Failure      429  {object}   dto.APIErrorResponse
// @Failure      500  {object}   dto.APIErrorResponse
// @Failure      503  {object}   dto.APIErrorResponse
// @Router       /login [post]
func PostWebLogin(db *gorm.DB, redisDB *redis.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		userAgent := c.GetHeader("User-Agent")
		accept := c.GetHeader("Accept")

		if userAgent == "" || accept == "" {
			logrus.Errorf("Blocked Because No this aspect %s | %s |", userAgent, accept)
			fun.HandleAPIErrorSimple(c, http.StatusUnauthorized, "Wrong Username or Password")
			return
		}

		var loginForm dto.LoginRequest
		if err := c.ShouldBind(&loginForm); err != nil {
			fun.HandleAPIErrorSimple(c, http.StatusBadRequest, err.Error())
			return
		}

		if clientCaptcha := c.PostForm("client_captcha_valid"); clientCaptcha != "1" {
			if !captcha.VerifyString(loginForm.CaptchaID, loginForm.Captcha) {
				fun.HandleAPIErrorSimple(c, http.StatusUnauthorized, "Invalid captcha")
				return
			}
		}

		user, ok := lookupLoginUser(db, loginForm.EmailUsername)
		if !ok {
			fun.HandleAPIErrorSimple(c, http.StatusUnauthorized, "Wrong Username or Password")
			return
		}

		if locked, msg := checkAccountLockout(db, &user); locked {
			fun.HandleAPIErrorSimple(c, http.StatusTooManyRequests, msg)
			return
		}

		if !fun.IsPasswordMatched(loginForm.Password, user.Password) {
			msg := recordFailedLoginAttempt(db, &user)
			fun.HandleAPIErrorSimple(c, http.StatusUnauthorized, msg)
			return
		}

		// User verified — reset lockout state
		user.LoginAttempts = 0
		user.LockUntil = nil

		if err := redisDB.Set(context.Background(), "last_activity_time:"+user.Email, time.Now().UnixMilli(), 30*time.Minute).Err(); err != nil {
			fun.HandleAPIErrorSimple(c, http.StatusInternalServerError, "Internal Server Error, Error Saving to Memory : "+err.Error())
			return
		}

		if !isUserAccountActive(c, db, user) {
			return
		}

		// Refresh session
		currentUnixTime := time.Now().Unix() * 1000
		user.SessionExpired = currentUnixTime + (7 * 24 * 60 * 60 * 1000)
		user.Session = fun.GenerateRandomString(40)
		user.UpdatedAt = time.Now()
		if err := db.Save(&user).Error; err != nil {
			fun.HandleAPIErrorSimple(c, http.StatusInternalServerError, "Failed to update user: "+err.Error())
			return
		}

		tokenString, authToken, randomToken, err := buildLoginToken(c, db, &user)
		if err != nil {
			fun.HandleAPIErrorSimple(c, http.StatusInternalServerError, err.Error())
			return
		}

		setLoginCookies(c, authToken, randomToken, tokenString, user.Session)

		db.Create(&model.LogActivity{
			UserID:    user.ID,
			FullName:  user.Fullname,
			Action:    "LOGIN",
			Status:    "Success",
			Log:       "User logged in successfully",
			IP:        c.ClientIP(),
			UserAgent: c.Request.UserAgent(),
			ReqMethod: c.Request.Method,
			ReqURI:    c.Request.RequestURI,
		})

		c.Redirect(http.StatusSeeOther, config.GlobalURL+"page")
		c.Abort()
	}
}

// lookupLoginUser finds a user by email or username.
func lookupLoginUser(db *gorm.DB, emailUsername string) (model.Users, bool) {
	whereQuery := "Username = ? "
	if strings.Contains(emailUsername, "@") {
		whereQuery = "Email = ? "
	}
	var user model.Users
	if err := db.Where(whereQuery, emailUsername).First(&user).Error; err != nil {
		return user, false
	}
	return user, true
}

// checkAccountLockout resets expired lockouts and returns (true, message) when account is still locked.
func checkAccountLockout(db *gorm.DB, user *model.Users) (bool, string) {
	if user.LockUntil != nil && user.LockUntil.Before(time.Now()) {
		user.LoginAttempts = 0
		user.LockUntil = nil
		db.Save(user)
	}
	if user.LockUntil != nil && user.LockUntil.After(time.Now()) {
		return true, "Account locked. Try again at " + user.LockUntil.Format(config.DateYYYYMMDDHHMMSS)
	}
	return false, ""
}

// recordFailedLoginAttempt increments the attempt counter, locks if needed, and returns the error message.
func recordFailedLoginAttempt(db *gorm.DB, user *model.Users) string {
	user.LoginAttempts++
	now := time.Now()
	user.LastFailedLogin = &now
	if user.LoginAttempts >= user.MaxRetry {
		lockUntil := now.Add(time.Duration(config.ServicePlatform.Get().App.LoginLockUntil) * time.Minute)
		user.LockUntil = &lockUntil
	}
	db.Save(user)
	locked := ""
	if user.LockUntil != nil && user.LockUntil.After(time.Now()) {
		locked = fmt.Sprintf(" Account locked until %s.", user.LockUntil.Format(config.DateYYYYMMDDHHMMSS))
	}
	return fmt.Sprintf("Wrong Username or Password or Captcha. Attempt %d of %d.%s", user.LoginAttempts, user.MaxRetry, locked)
}

// isUserAccountActive returns true when the user has ACTIVE status; false + responds with error when not.
func isUserAccountActive(c *gin.Context, db *gorm.DB, user model.Users) bool {
	var userStatuses []model.UserStatus
	db.Find(&userStatuses)
	for _, us := range userStatuses {
		if us.ID == uint(user.Status) && us.Title != "ACTIVE" {
			fun.HandleAPIErrorSimple(c, http.StatusUnauthorized,
				fmt.Sprintf("Please Contact Our Technical Support To Activate your Account @+%s", config.ServicePlatform.Get().Whatsnyan.WATechnicalSupport))
			return false
		}
	}
	return true
}

// buildLoginToken builds the encrypted token string plus auth and random tokens.
func buildLoginToken(c *gin.Context, db *gorm.DB, user *model.Users) (tokenString, authToken, randomToken string, err error) {
	roleData, err := loadRoleData(db, user.Role)
	if err != nil {
		return "", "", "", fmt.Errorf("Error querying database: %w", err)
	}
	var roles model.Role
	if err := db.Where("id = ?", user.Role).First(&roles).Error; err != nil {
		return "", "", "", fmt.Errorf("Error querying database: %w", err)
	}
	authToken = fun.GenerateRandomString(40 + rand.Intn(25) + 1)
	randomToken = fun.GenerateRandomString(40 + rand.Intn(25) + 1)

	profileImage, err := resolveLoginProfileImage(user)
	if err != nil {
		return "", "", "", fmt.Errorf("Could not encripting image %w", err)
	}

	claims := map[string]interface{}{
		"id": user.ID, "fullname": user.Fullname, "username": user.Username,
		"phone": user.Phone, "email": user.Email, "password": user.Password,
		"type": user.Type, "role": user.Role, "role_name": roles.RoleName,
		"profile_image": profileImage, "status": user.Status, "status_name": "",
		"last_login": time.Now(), "session": user.Session, "session_expired": user.SessionExpired,
		"random": randomToken, "auth": authToken, "ip": user.IP,
	}
	for k, v := range roleData {
		claims[k] = v
	}
	jsonText, err := json.Marshal(claims)
	if err != nil {
		return "", "", "", fmt.Errorf("Could not string token %w", err)
	}
	tokenString, err = fun.GetAESEncrypted(string(jsonText))
	if err != nil {
		return "", "", "", fmt.Errorf("Could not encripting token %w", err)
	}
	_ = c
	return tokenString, authToken, randomToken, nil
}

// resolveLoginProfileImage returns a profile image URL for the user.
func resolveLoginProfileImage(user *model.Users) (string, error) {
	if user.ProfileImage != "" {
		return user.ProfileImage, nil
	}
	imageMaps := map[string]interface{}{"t": fun.GenerateRandomString(3), "id": user.ID}
	pathString, err := fun.GetAESEcryptedURLfromJSON(imageMaps)
	if err != nil {
		return "", err
	}
	return "/profile/default.jpg?f=" + pathString, nil
}

// setLoginCookies writes auth, random, token, and credentials cookies to the response.
func setLoginCookies(c *gin.Context, authToken, randomToken, tokenString, session string) {
	cfg := config.ServicePlatform.Get().App
	loginExpiredMinutes := cfg.LoginTimeM
	if loginExpiredMinutes <= 0 {
		loginExpiredMinutes = 30
	}
	expiration := time.Now().Add(time.Duration(loginExpiredMinutes) * time.Minute)
	makeCookie := func(name, value string, httpOnly bool) *http.Cookie {
		return &http.Cookie{
			Name: name, Value: value, Expires: expiration,
			Path: config.GlobalURL, Domain: cfg.CookieLoginDomain,
			SameSite: http.SameSiteStrictMode, Secure: cfg.CookieLoginSecure,
			HttpOnly: httpOnly,
		}
	}
	http.SetCookie(c.Writer, makeCookie("auth", authToken, true))
	http.SetCookie(c.Writer, makeCookie("random", randomToken, true))
	http.SetCookie(c.Writer, makeCookie("token", tokenString, true))
	http.SetCookie(c.Writer, makeCookie("credentials", url.QueryEscape(session), true))
}

// VerifyCaptcha godoc
// @Summary      Verify CAPTCHA
// @Description  Verifies the CAPTCHA input
// @Tags         Web
// @Accept       x-www-form-urlencoded
// @Produce      json
// @Param        request formData dto.CaptchaRequest true "Captcha Request"
// @Success      200  {object}   map[string]bool
// @Failure      400  {object}   map[string]string
// @Router       /verify-captcha [post]
func VerifyCaptcha(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		_ = db // Currently not used, but kept for potential future use or consistency with other handlers

		var captchaForm dto.CaptchaRequest

		if err := c.ShouldBind(&captchaForm); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"valid": false, "error": err.Error()})
			return
		}

		valid := captcha.VerifyString(captchaForm.CaptchaID, captchaForm.Captcha) ||
			captcha.VerifyString(captchaForm.CaptchaID, strings.ToLower(captchaForm.Captcha)) ||
			captcha.VerifyString(captchaForm.CaptchaID, strings.ToUpper(captchaForm.Captcha))
		c.JSON(http.StatusOK, gin.H{"valid": valid})
	}
}

// GetWebForgotPassword godoc
// @Summary      Get Forgot Password Page
// @Description  Renders the forgot password page
// @Tags         Web
// @Produce      html
// @Success      200  {string}   string "HTML Content"
// @Router       /forgot-password [get]
func GetWebForgotPassword(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Retrieve cookies from the request
		cookies := c.Request.Cookies()

		// Check if the "credentials" cookie exists
		var credentialsCookie *http.Cookie
		for _, cookie := range cookies {
			if cookie.Name == "credentials" {
				credentialsCookie = cookie
				break
			}
		}

		yamlCfg := config.ServicePlatform.Get()

		parameters := gin.H{
			"APP_NAME":         yamlCfg.App.Name,
			"APP_LOGO":         yamlCfg.App.Logo,
			"APP_VERSION":      yamlCfg.App.Version,
			"APP_VERSION_NO":   yamlCfg.App.VersionNo,
			"APP_VERSION_CODE": yamlCfg.App.VersionCode,
			"APP_VERSION_NAME": yamlCfg.App.VersionName,
		}
		if credentialsCookie != nil {
			var user model.Users
			if err := db.Where("session = ?", credentialsCookie.Value).First(&user).Error; err != nil {
				c.HTML(http.StatusOK, "forgot-password.html", parameters)
				return
			}
			c.Redirect(http.StatusFound, config.GlobalURL+"page")
		} else {
			c.HTML(http.StatusOK, "forgot-password.html", parameters)
		}
	}
}

// PostForgotPassword godoc
// @Summary      Process Forgot Password
// @Description  Handles forgot password request
// @Tags         Web
// @Accept       x-www-form-urlencoded
// @Produce      html
// @Param        request formData dto.ForgotPasswordRequest true "Forgot Password Request"
// @Success      200  {string}   string "HTML Content"
// @Failure      400  {object}   dto.APIErrorResponse
// @Router       /forgot-password [post]
func PostForgotPassword(db *gorm.DB, redisDB *redis.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req dto.ForgotPasswordRequest
		if err := c.ShouldBind(&req); err != nil {
			fun.HandleAPIErrorSimple(c, http.StatusBadRequest, err.Error())
			return
		}

		// Extract form data
		email := req.Email
		captchaText := req.Captcha

		captchaID, err := c.Cookie("halo")
		if err != nil {
			c.Redirect(http.StatusSeeOther, "/login")
			return
		}

		yamlCfg := config.ServicePlatform.Get()
		parameters := gin.H{
			"APP_NAME":         yamlCfg.App.Name,
			"APP_LOGO":         yamlCfg.App.Logo,
			"APP_VERSION":      yamlCfg.App.Version,
			"APP_VERSION_NO":   yamlCfg.App.VersionNo,
			"APP_VERSION_CODE": yamlCfg.App.VersionCode,
			"APP_VERSION_NAME": yamlCfg.App.VersionName,
			"MSG_HEADER":       "Please, Contact Our Technical Support",
			"EMAIL_DOMAIN":     "",
			"EMAIL":            email,
			"DISABLED":         "disabled",
			"msg":              "",
		}
		// Perform necessary actions (e.g., send a reset link, validate email, etc.)
		if email == "" {
			parameters["msg"] = "Please provide a valid email"
			c.HTML(http.StatusOK, "verify-email.html", parameters)
			return
		}
		if !captcha.VerifyString(captchaID, captchaText) {
			parameters["msg"] = "Invalid email or captcha"
			c.HTML(http.StatusOK, "verify-email.html", parameters)
			return
		}
		var user model.Users
		if err := db.Where("email = ?", email).First(&user).Error; err != nil {
			parameters["msg"] = "Invalid email or captcha, details: " + err.Error()
			c.HTML(http.StatusOK, "verify-email.html", parameters)
			return
		}

		parts := strings.Split(user.Email, "@")
		if len(parts) != 2 {
			parts[1] = ""
		}

		parameters["MSG_HEADER"] = "Please, Check Your Email"
		parameters["DISABLED"] = ""
		parameters["EMAIL_DOMAIN"] = parts[1]
		parameters["EMAIL"] = user.Email
		parameters["msg"] = "Please Verify Your Email Address Link Sended To Your Email Address "
		val, _ := redisDB.Get(context.Background(), "reset_pwd:"+user.Email).Result()
		if val != "" {
			c.HTML(http.StatusOK, "verify-email.html", parameters)
			return
		}
		randomAccessToken := fun.GenerateRandomString(100)

		//SEND EMAIL VERIFICATION
		// Now you can send the email with the verification link.
		htmlMailTemplate := `<body style="font-family: Arial, sans-serif; text-align: center;">
			<div style="background-color: #f4f4f4; padding: 20px;">
				<img src="` + yamlCfg.App.WebPublicURL + yamlCfg.App.Logo + `" alt="logo" width="180" height="101" style="display: block; margin: 0 auto;">
				<h1 style="color: #4287f5;">` + yamlCfg.App.Name + ` Reset Password</h1>
				<p>Please click the button below to verify your email address:</p>
				<a href="` + yamlCfg.App.WebPublicURL + `/reset-password/` + user.Email + "/" + randomAccessToken + `" style="text-decoration: none;">
					<button style="background-color: #4287f5; color: #fff; padding: 10px 20px; border: none; border-radius: 4px; cursor: pointer;">
						Reset Password
					</button>
				</a>
			</div>
		</body>`

		mailer := gomail.NewMessage()
		mailer.SetHeader("From", fmt.Sprintf("Email Verificator  <%s>", yamlCfg.Email.Username))
		mailer.SetHeader("To", user.Email)
		mailer.SetHeader("Subject", "[noreply] Here Reset Password link")
		mailer.SetBody("text/html", htmlMailTemplate)

		dialer := gomail.NewDialer(
			yamlCfg.Email.Host,
			yamlCfg.Email.Port,
			yamlCfg.Email.Username,
			yamlCfg.Email.Password,
		)

		errMailDialer := dialer.DialAndSend(mailer)

		if errMailDialer != nil {
			parameters["msg"] = fmt.Sprintf("Error Sending Email, Please Contact Admin : %v", errMailDialer)
			c.HTML(http.StatusOK, "verify-email.html", parameters)
		} else {
			errSet := redisDB.Set(context.Background(), "reset_pwd:"+user.Email, randomAccessToken, 60*time.Minute).Err()
			if errSet != nil {
				fun.HandleAPIErrorSimple(c, http.StatusInternalServerError, "Internal Server Error, Error Created Random Token Cache : "+errSet.Error())
				return
			}
			c.HTML(http.StatusOK, "verify-email.html", parameters)
		}
	}
}

// GetWebResetPassword godoc
// @Summary      Get Reset Password Page
// @Description  Renders the reset password page
// @Tags         Web
// @Produce      html
// @Param        email       path      string  true  "Email"
// @Param        token_data  path      string  true  "Token Data"
// @Success      200  {string}   string "HTML Content"
// @Failure      404  {string}   string "Not Found"
// @Router       /reset-password/{email}/{token_data} [get]
func GetWebResetPassword(db *gorm.DB, redisDB *redis.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		_ = db // Currently not used, but kept for potential future use or consistency with other handlers

		// Extract email and token_data from URL parameters
		email := c.Param("email")
		tokenData := c.Param("token_data")

		// Create Redis key
		redisKey := "reset_pwd:" + email

		// Fetch the token from Redis
		val, err := redisDB.Get(context.Background(), redisKey).Result()
		if err == redis.Nil {
			// Key does not exist
			c.HTML(http.StatusNotFound, "misc-error-page.html", gin.H{})
			return
		} else if err != nil {
			// Some other Redis error
			c.HTML(http.StatusInternalServerError, "misc-error-page.html", gin.H{})
			return
		}

		// Check if the token matches
		if val != tokenData {
			c.HTML(http.StatusNotFound, "misc-error-page.html", gin.H{})
			return
		}

		yamlCfg := config.ServicePlatform.Get()

		parameters := gin.H{
			"APP_NAME":         yamlCfg.App.Name,
			"APP_LOGO":         yamlCfg.App.Logo,
			"APP_VERSION":      yamlCfg.App.Version,
			"APP_VERSION_NO":   yamlCfg.App.VersionNo,
			"APP_VERSION_CODE": yamlCfg.App.VersionCode,
			"APP_VERSION_NAME": yamlCfg.App.VersionName,
			"EMAIL":            email,
			"TOKEN":            tokenData,
		}
		// If the token matches, render the verification page
		c.HTML(http.StatusOK, "reset-password.html", parameters)
	}
}

// PostResetPassword godoc
// @Summary      Process Reset Password
// @Description  Handles password reset
// @Tags         Web
// @Accept       x-www-form-urlencoded
// @Produce      json
// @Param        email             path      string  true  "Email"
// @Param        request formData dto.ResetPasswordRequest true "Reset Password Request"
// @Success      200  {object}   map[string]string
// @Failure      400  {object}   dto.APIErrorResponse
// @Failure      500  {object}   dto.APIErrorResponse
// @Router       /reset-password/{email}/{token_data} [post]
func PostResetPassword(db *gorm.DB, redisDB *redis.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req dto.ResetPasswordRequest
		if err := c.ShouldBind(&req); err != nil {
			fun.HandleAPIErrorSimple(c, http.StatusBadRequest, err.Error())
			return
		}

		// Extract email, token_data, password, and confirm-password from the form data
		email := req.Email
		tokenData := req.TokenData
		password := req.Password
		confirmPwd := req.ConfirmPassword

		// Validate passwords
		if password != confirmPwd {
			fun.HandleAPIErrorSimple(c, http.StatusBadRequest, "Passwords do not match")
			return
		}
		// Validate the password
		if err := fun.ValidatePassword(password); err != nil {
			fun.HandleAPIErrorSimple(c, http.StatusBadRequest, err.Error())
			return
		}

		numberOfPreviousPasswordsToCheck := 4
		var checkUserPasswordChangelogs []model.UserPasswordChangeLog
		db.Where("email = ?", email).Order("created_at desc").Find(&checkUserPasswordChangelogs)
		for _, data := range checkUserPasswordChangelogs {
			if fun.IsPasswordMatched(password, data.Password) {
				fun.HandleAPIErrorSimple(c, http.StatusBadRequest, fmt.Sprintf("You cannot reuse one of your last %d passwords. Please choose a different password", numberOfPreviousPasswordsToCheck))
				return
			}
		}

		// Create Redis key
		redisKey := "reset_pwd:" + email

		// Fetch the token from Redis
		val, err := redisDB.Get(context.Background(), redisKey).Result()
		if err == redis.Nil {
			fun.HandleAPIErrorSimple(c, http.StatusBadRequest, "Link expired or invalid : "+err.Error())
			return
		} else if err != nil {
			fun.HandleAPIErrorSimple(c, http.StatusInternalServerError, "Error accessing Redis")
			return
		}

		// Check if the token matches
		if val != tokenData {
			fun.HandleAPIErrorSimple(c, http.StatusBadRequest, "Invalid reset link")
			return
		}
		// Update the password in the database
		var user model.Users
		if err := db.Where("email = ?", email).First(&user).Error; err != nil {
			fun.HandleAPIErrorSimple(c, http.StatusBadRequest, "Email not found : "+err.Error())
			return
		}

		var lastLoginTime *time.Time
		now := time.Now()
		lastLoginTime = &now

		user.Password = fun.GenerateSaltedPassword(password)
		user.LastLogin = lastLoginTime
		if err := db.Save(&user).Error; err != nil {
			fun.HandleAPIErrorSimple(c, http.StatusInternalServerError, "Error updating password "+err.Error())
			return
		}

		if err := savePasswordChangelog(db, user, numberOfPreviousPasswordsToCheck); err != nil {
			fun.HandleAPIErrorSimple(c, http.StatusInternalServerError, err.Error())
			return
		}
		// Remove the token from Redis
		if err := redisDB.Del(context.Background(), redisKey).Err(); err != nil {
			fun.HandleAPIErrorSimple(c, http.StatusInternalServerError, "Error removing Redis key: "+err.Error())
			return
		}

		// Respond with success message
		c.JSON(http.StatusOK, gin.H{"msg": "Password reset successful"})
	}
}

// MainPage godoc
// @Summary      Get Main Page
// @Description  Renders the main dashboard page
// @Tags         Web
// @Produce      html
// @Success      200  {string}   string "HTML Content"
// @Failure      302  {string}   string "Redirect to Login Page"
// @Failure      500  {object}   dto.APIErrorResponse
// @Router       /page [get]
func MainPage(db *gorm.DB, redisDB *redis.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		cookies := c.Request.Cookies()
		user, session, claims, ok := validateMainPageSession(c, db, cookies)
		if !ok {
			return
		}

		yamlCfg := config.ServicePlatform.Get()
		featuresPrivileges, ok := loadMainPagePrivileges(c, db, emailToken(claims), cookies, claims["role"])
		if !ok {
			return
		}

		fileContent, fileContentTab := buildMainPageMenuHTML(featuresPrivileges)

		randomAccessToken := fun.GenerateRandomString(20 + rand.Intn(30) + 1)
		if err := redisDB.Set(context.Background(), "web:"+session, randomAccessToken, 0).Err(); err != nil {
			fun.RemoveEmailSession(db, emailToken(claims))
			fun.ClearCookiesAndRedirect(c, cookies)
			return
		}

		imageMaps := map[string]interface{}{"t": fun.GenerateRandomString(3), "id": user.ID}
		pathString, err := fun.GetAESEcryptedURLfromJSON(imageMaps)
		if err != nil {
			fun.HandleAPIErrorSimple(c, http.StatusInternalServerError, "Could not encripting image "+err.Error())
			return
		}
		profileImage := config.GlobalURL + "profile/default.jpg?f=" + pathString

		appName, appLogo, appVersion, appVersionNo, appVersionCode, appVersionName := resolveAppConfig(db, user.Role, yamlCfg)

		c.HTML(http.StatusOK, "index.html", gin.H{
			"APP_NAME":         appName,
			"APP_LOGO":         appLogo,
			"APP_VERSION":      appVersion,
			"APP_VERSION_NO":   appVersionNo,
			"APP_VERSION_CODE": appVersionCode,
			"APP_VERSION_NAME": appVersionName,
			"ACCESS":           config.APIURL + randomAccessToken,
			"username":         claims["username"],
			"role":             claims["role_name"],
			"fullname":         claims["fullname"],
			"email":            claims["email"],
			"profile_image":    profileImage,
			"GLOBAL_URL":       config.GlobalURL,
			"sidebar":          template.HTML(fileContent),
			"contents":         template.HTML(fileContentTab),
			"IsEnableDebug":    yamlCfg.App.Debug,
		})
	}
}

// menuPrivilege is a flattened row of role privilege joined with feature metadata.
type menuPrivilege struct {
	model.RolePrivilege
	ParentID  uint   `json:"parent_id" gorm:"column:parent_id"`
	Title     string `json:"title" gorm:"column:title"`
	Path      string `json:"path" gorm:"column:path"`
	MenuOrder uint   `json:"menu_order" gorm:"column:menu_order"`
	Status    uint   `json:"status" gorm:"column:status"`
	Level     uint   `json:"level" gorm:"column:level"`
	Icon      string `json:"icon" gorm:"column:icon"`
}

func emailToken(claims map[string]interface{}) string {
	if v, ok := claims["email"].(string); ok {
		return v
	}
	return ""
}

// validateMainPageSession validates the token cookie, additional cookies and DB session.
// Returns (user, session, claims, true) on success and responds+redirects on failure.
func validateMainPageSession(c *gin.Context, db *gorm.DB, cookies []*http.Cookie) (model.Users, string, map[string]interface{}, bool) {
	var empty model.Users
	tokenString, err := c.Cookie("token")
	if err != nil {
		fun.ClearCookiesAndRedirect(c, cookies)
		return empty, "", nil, false
	}
	tokenString = strings.ReplaceAll(tokenString, " ", "+")

	decrypted, err := fun.GetAESDecrypted(tokenString)
	if err != nil {
		fmt.Println("Error during decryption", err)
		fun.ClearCookiesAndRedirect(c, cookies)
		return empty, "", nil, false
	}
	var claims map[string]interface{}
	if err = json.Unmarshal(decrypted, &claims); err != nil {
		fmt.Printf("Error converting JSON to map: %v", err)
		fun.ClearCookiesAndRedirect(c, cookies)
		return empty, "", nil, false
	}
	email := emailToken(claims)
	if email == "" {
		fun.ClearCookiesAndRedirect(c, cookies)
		return empty, "", nil, false
	}
	if !fun.ValidateCookie(c, "credentials", claims["session"]) ||
		!fun.ValidateCookie(c, "auth", claims["auth"]) ||
		!fun.ValidateCookie(c, "random", claims["random"]) {
		fun.RemoveEmailSession(db, email)
		fun.ClearCookiesAndRedirect(c, cookies)
		return empty, "", nil, false
	}
	session, ok := claims["session"].(string)
	if !ok {
		fun.RemoveEmailSession(db, email)
		fun.ClearCookiesAndRedirect(c, cookies)
		return empty, "", nil, false
	}
	var user model.Users
	if err := db.Where("id = ?", claims["id"]).First(&user).Error; err != nil {
		fun.RemoveEmailSession(db, email)
		fun.ClearCookiesAndRedirect(c, cookies)
		return empty, "", nil, false
	}
	if user.Session != session {
		fun.RemoveEmailSession(db, email)
		fun.ClearCookiesAndRedirect(c, cookies)
		return empty, "", nil, false
	}
	return user, session, claims, true
}

// loadMainPagePrivileges queries role privileges for the main page menu.
func loadMainPagePrivileges(c *gin.Context, db *gorm.DB, email string, cookies []*http.Cookie, roleID interface{}) ([]menuPrivilege, bool) {
	yamlCfg := config.ServicePlatform.Get()
	var featuresPrivileges []menuPrivilege
	err := db.
		Table(fmt.Sprintf("%s a", yamlCfg.Database.TbRolePrivilege)).
		Unscoped().
		Select("a.*, b.parent_id, b.title, b.path, b.menu_order, b.status, b.level, b.icon").
		Joins(fmt.Sprintf("LEFT JOIN %s b ON a.feature_id = b.id", yamlCfg.Database.TbFeature)).
		Where("a.role_id = ?", roleID).
		Order("b.menu_order").
		Find(&featuresPrivileges).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			fun.RemoveEmailSession(db, email)
			fun.ClearCookiesAndRedirect(c, cookies)
			return nil, false
		}
		fun.HandleAPIErrorSimple(c, http.StatusInternalServerError, "Error querying database: "+err.Error())
		fun.RemoveEmailSession(db, email)
		fun.ClearCookiesAndRedirect(c, cookies)
		return nil, false
	}
	return featuresPrivileges, true
}

// menuI18nKey derives a stable i18n key from a feature path or title.
func menuI18nKey(path, title string) string {
	key := "menu." + strings.ReplaceAll(strings.Trim(path, "/ "), "/", ".")
	if strings.TrimSpace(key) == "menu." {
		key = "menu." + strings.ToLower(strings.ReplaceAll(title, " ", "_"))
	}
	return key
}

// hasMenuPermission returns true when at least one CRUD bit is set.
func hasMenuPermission(p menuPrivilege) bool {
	return p.CanCreate == 1 || p.CanRead == 1 || p.CanUpdate == 1 || p.CanDelete == 1
}

// buildChildMenuItems builds the <ul class="menu-sub"> HTML for children of a parent menu entry.
// Returns (html, hasChildren).
func buildChildMenuItems(all []menuPrivilege, parentMenuOrder uint) (string, bool) {
	hasAccessible := false
	for _, fp := range all {
		if fp.ParentID == parentMenuOrder && hasMenuPermission(fp) {
			hasAccessible = true
			break
		}
	}
	if !hasAccessible {
		return "", false
	}
	childHTML := ""
	for _, fp := range all {
		if fp.ParentID != parentMenuOrder || !hasMenuPermission(fp) {
			continue
		}
		privKey := menuI18nKey(fp.Path, fp.Title)
		childHTML += `        
						<li class="menu-item">
							<a href="#` + fp.Path + `" 
							class="menu-link" 
							data-bs-toggle="tooltip" 
							data-bs-placement="right" 
							data-bs-original-title="` + fp.Title + `">
								<div class="text-truncate" data-i18n="` + privKey + `">` + fp.Title + `</div>
							</a>
						</li>`
	}
	return `<ul class="menu-sub">` + childHTML + `</ul>`, true
}

// buildMainPageMenuHTML iterates over privileges and builds sidebar + tab HTML.
func buildMainPageMenuHTML(all []menuPrivilege) (sidebar, tabs string) {
	for _, parent := range all {
		childHTML := ""
		menuToggle := ""
		if strings.TrimSpace(parent.Path) == "" {
			var hasChildren bool
			childHTML, hasChildren = buildChildMenuItems(all, parent.MenuOrder)
			if !hasChildren {
				continue
			}
			if childHTML != "" {
				menuToggle = "menu-toggle"
			}
		}

		if parent.Level != 0 || parent.Status != 1 {
			if parent.Path != "" {
				tabs += `<div id="` + parent.Path + `" class="tab-content flex-grow-1 container-p-y d-none h-100"></div>`
			}
			continue
		}

		hrefPath := ""
		if parent.Path != "" {
			if !hasMenuPermission(parent) {
				continue
			}
			hrefPath = `href="#` + parent.Path + `"`
		} else if childHTML == "" && !hasMenuPermission(parent) {
			continue
		}

		parentKey := menuI18nKey(parent.Path, parent.Title)
		sidebar += `
					<li class="menu-item ">
						<a ` + hrefPath + ` 
						class="menu-link ` + menuToggle + `"
						data-bs-toggle="tooltip" 
						data-bs-placement="right" 
						data-bs-original-title="` + parent.Title + `">
							<i class="menu-icon tf-icons ` + parent.Icon + `"></i>
							<div class="text-truncate" data-i18n="` + parentKey + `">` + parent.Title + `</div>
						</a>
						` + childHTML + `
					</li>`

		if parent.Path != "" {
			tabs += `<div id="` + parent.Path + `" class="tab-content flex-grow-1 container-p-y d-none h-100"></div>`
		}
	}
	return sidebar, tabs
}

// GetWebLogout godoc
// @Summary      Logout
// @Description  Logs out the user and clears cookies
// @Tags         Web
// @Produce      html
// @Success      302  {string}   string "Redirect to login"
// @Router       /logout [get]
func GetWebLogout(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		cookies := c.Request.Cookies()

		tokenString, err := c.Cookie("token")
		if err != nil {
			fun.ClearCookiesAndRedirect(c, cookies)
			return
		}
		tokenString = strings.ReplaceAll(tokenString, " ", "+")

		decrypted, err := fun.GetAESDecrypted(tokenString)
		if err != nil {
			logrus.Errorf("failed to decrypt token %s: %v", tokenString, err)
			fun.ClearCookiesAndRedirect(c, cookies)
			return
		}
		var claims map[string]interface{}
		err = json.Unmarshal(decrypted, &claims)
		if err != nil {
			logrus.Errorf("failed to unmarshal decrypted token %s: %v", string(decrypted), err)
			fun.ClearCookiesAndRedirect(c, cookies)
			return
		}

		id := uint(claims["id"].(float64))

		userToUpdate := map[string]any{
			"session":         "",
			"session_expired": 0,
		}

		if err := db.Model(&model.Users{}).Where(&model.Users{ID: id}).Updates(userToUpdate).Error; err != nil {
			logrus.Errorf("failed to update user session for user ID %d: %v", id, err)
			fun.ClearCookiesAndRedirect(c, cookies)
			return
		}

		// Redirect to the login page
		c.Redirect(http.StatusFound, config.GlobalURL+"login")

		// Save the newLog to the database
		db.Create(&model.LogActivity{
			UserID:    uint(claims["id"].(float64)),
			FullName:  claims["fullname"].(string),
			Action:    "LOGOUT",
			Status:    "Success",
			Log:       "Logout by button",
			IP:        c.ClientIP(),
			UserAgent: c.Request.UserAgent(),
			ReqMethod: c.Request.Method,
			ReqURI:    c.Request.RequestURI,
		})
	}
}

// htmlEscape escapes special HTML characters in a string.
func htmlEscape(s string) string {
	s = strings.ReplaceAll(s, `"`, "&quot;")
	s = strings.ReplaceAll(s, `<`, "&lt;")
	s = strings.ReplaceAll(s, `>`, "&gt;")
	return s
}

// ComponentPage godoc
// @Summary      Get Component Page
// @Description  Renders a specific component page
// @Tags         Web
// @Produce      html
// @Param        access     path      string  true  "Access Token"
// @Param        component  path      string  true  "Component Name"
// @Success      200  {string}   string "HTML Content"
// @Router       /api/v1/{access}/components/{component} [get]
func ComponentPage(db *gorm.DB, redisDB *redis.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		globalURL := config.GlobalURL
		if globalURL == "" {
			logrus.Fatal("no global URL set in config")
		}

		cookies := c.Request.Cookies()

		// Parse JWT token from cookie
		tokenString, err := c.Cookie("token")
		if err != nil {
			logrus.Error("Error retrieving token cookie:", err)
			fun.ClearCookiesAndRedirect(c, cookies)
			return
		}
		tokenString = strings.ReplaceAll(tokenString, " ", "+")

		decrypted, err := fun.GetAESDecrypted(tokenString)
		if err != nil {
			logrus.Error("Error during decryption:", err)
			fun.ClearCookiesAndRedirect(c, cookies)
			return
		}
		var claims map[string]interface{}
		err = json.Unmarshal(decrypted, &claims)
		if err != nil {
			logrus.Error("Error converting JSON to map:", err)
			fun.ClearCookiesAndRedirect(c, cookies)
			return
		}
		componentID := c.Param("component")
		componentID = strings.ReplaceAll(componentID, "/", "")
		componentID = strings.ReplaceAll(componentID, "..", "")
		componentPrv, ok := claims[componentID]
		if !ok {
			fun.ClearCookiesAndRedirect(c, cookies)
			return
		}
		componentPrvStr, ok := componentPrv.(string)
		if !ok || componentPrvStr == "" {
			fun.ClearCookiesAndRedirect(c, cookies)
			return
		}
		if string(componentPrvStr[1:2]) != "1" {
			fun.ClearCookiesAndRedirect(c, cookies)
			return
		}

		var user model.Users
		db.Where(&model.Users{ID: uint(claims["id"].(float64))}).Find(&user)

		imageMaps := map[string]interface{}{
			"t":  fun.GenerateRandomString(3),
			"id": user.ID,
		}
		pathString, err := fun.GetAESEcryptedURLfromJSON(imageMaps)
		if err != nil {
			logrus.Errorf("failed to encrypt image data: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not encripting image " + err.Error()})
			return
		}
		profileImage := "/profile/default.jpg?f=" + pathString

		var userStatusData model.UserStatus
		if err := db.First(&userStatusData, user.Status).Error; err != nil {
			logrus.Errorf("failed to parse data status for user: %v", err)
		}
		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("<div class='%s'>", userStatusData.ClassName))
		sb.WriteString(userStatusData.Title)
		sb.WriteString("</div>")
		userStatusHTML := sb.String()

		yamlCfg := config.ServicePlatform.Get()
		appName, appLogo, appVersion, appVersionNo, appVersionCode, appVersionName := resolveAppConfig(db, user.Role, yamlCfg)

		randAccessToken := fun.GetRedis("web:"+user.Session, redisDB)

		replacements := map[string]any{
			"APP_NAME":         appName,
			"APP_LOGO":         appLogo,
			"APP_VERSION":      appVersion,
			"APP_VERSION_NO":   appVersionNo,
			"APP_VERSION_CODE": appVersionCode,
			"APP_VERSION_NAME": appVersionName,
			"fullname":         user.Fullname,
			"username":         user.Username,
			"userid":           user.ID,
			"phone":            user.Phone,
			"email":            user.Email,
			"role_name":        claims["role_name"].(string),
			"status_name":      template.HTML(userStatusHTML),
			"last_login":       claims["last_login"].(string),
			"profile_image":    profileImage,
			"ip":               user.IP,
			"GLOBAL_URL":       globalURL,
			"RANDOM_ACCESS":    randAccessToken,

			/* App Config */
			"TableAppConfiguration": webguibuilder.TableAppConfiguration(user.Session, redisDB),
		}
		c.HTML(http.StatusOK, componentID+".html", replacements)
	}
}

// loadRoleData queries role privileges for the given roleID and returns a map of path -> privilege string.
func loadRoleData(db *gorm.DB, roleID interface{}) (map[string]interface{}, error) {
	type roleRow struct {
		model.RolePrivilege
		Path string `json:"path" gorm:"column:path"`
	}
	var rows []roleRow
	cfg := config.ServicePlatform.Get()
	if err := db.
		Table(fmt.Sprintf("%s rp", cfg.Database.TbRolePrivilege)).
		Unscoped().
		Select("rp.*,f.path").
		Joins(fmt.Sprintf("LEFT JOIN %s f ON f.id = rp.feature_id", cfg.Database.TbFeature)).
		Where("rp.role_id = ?", roleID).
		Find(&rows).Error; err != nil {
		return nil, err
	}
	roleData := make(map[string]interface{}, len(rows))
	for _, r := range rows {
		privilege := strconv.Itoa(int(r.CanCreate)) + strconv.Itoa(int(r.CanRead)) + strconv.Itoa(int(r.CanUpdate)) + strconv.Itoa(int(r.CanDelete))
		roleData[r.Path] = privilege
	}
	return roleData, nil
}

// resolveAppConfig returns app display values, preferring a role-specific AppConfig when one exists.
func resolveAppConfig(db *gorm.DB, roleID interface{}, yamlCfg config.TypeServicePlatform) (appName, appLogo, appVersion, appVersionNo, appVersionCode, appVersionName string) {
	var appConfig model.AppConfig
	if err := db.Where("role_id = ? AND is_active = ?", roleID, true).First(&appConfig).Error; err == nil {
		return appConfig.AppName, appConfig.AppLogo, appConfig.AppVersion, appConfig.VersionNo, appConfig.VersionCode, appConfig.VersionName
	}
	return yamlCfg.App.Name, yamlCfg.App.Logo, yamlCfg.App.Version, strconv.Itoa(yamlCfg.App.VersionNo), yamlCfg.App.VersionCode, yamlCfg.App.VersionName
}

// savePasswordChangelog creates a new password changelog entry and prunes old ones,
// keeping at most numberOfPrevious entries per email.
func savePasswordChangelog(db *gorm.DB, user model.Users, numberOfPrevious int) error {
	userPasswordChangelog := model.UserPasswordChangeLog{
		Email:    user.Email,
		Password: user.Password,
	}
	if err := db.Create(&userPasswordChangelog).Error; err != nil {
		return fmt.Errorf("error updating password changelog: %w", err)
	}

	var userPasswordChangelogs []model.UserPasswordChangeLog
	if err := db.Where("email = ?", user.Email).
		Order("created_at asc").
		Find(&userPasswordChangelogs).Error; err != nil {
		return fmt.Errorf("email not found: %w", err)
	}

	if len(userPasswordChangelogs) > numberOfPrevious {
		for i := 0; i < len(userPasswordChangelogs)-numberOfPrevious; i++ {
			db.Delete(&userPasswordChangelogs[i])
		}
	}
	return nil
}
