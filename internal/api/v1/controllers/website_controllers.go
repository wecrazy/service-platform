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
	"service-platform/internal/pkg/fun"
	"service-platform/internal/pkg/webguibuilder"
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

		parameters := gin.H{
			"APP_NAME":         config.GetConfig().App.Name,
			"APP_LOGO":         config.GetConfig().App.Logo,
			"APP_VERSION":      config.GetConfig().App.Version,
			"APP_VERSION_NO":   config.GetConfig().App.VersionNo,
			"APP_VERSION_CODE": config.GetConfig().App.VersionCode,
			"APP_VERSION_NAME": config.GetConfig().App.VersionName,
			"CAPTCHA_ID":       captcha.New(),
			"DEBUG":            config.GetConfig().App.Debug,
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
			c.Redirect(http.StatusFound, config.GLOBAL_URL+"page")
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

		// Bind form data to the LoginForm struct
		if err := c.ShouldBind(&loginForm); err != nil {
			fun.HandleAPIErrorSimple(c, http.StatusBadRequest, err.Error())
			return
		}

		// Validate captcha
		// Allow client-side verified CAPTCHA when frontend sends client_captcha_valid=1
		clientCaptcha := c.PostForm("client_captcha_valid")
		if clientCaptcha != "1" {
			if !captcha.VerifyString(loginForm.CaptchaID, loginForm.Captcha) {
				fun.HandleAPIErrorSimple(c, http.StatusUnauthorized, "Invalid captcha")
				return
			}
		}

		var user model.Users

		// Check if the login is attempted with an email or username
		whereQuery := ""
		if strings.Contains(loginForm.EmailUsername, "@") {
			whereQuery = "Email = ? "
		} else {
			whereQuery = "Username = ? "

		}
		if err := db.Where(whereQuery, loginForm.EmailUsername).First(&user).Error; err != nil {
			fun.HandleAPIErrorSimple(c, http.StatusUnauthorized, "Wrong Username or Password")
			return
		}

		// Reset attempts if lockout period has passed
		if user.LockUntil != nil && user.LockUntil.Before(time.Now()) {
			user.LoginAttempts = 0
			user.LockUntil = nil
			db.Save(&user)
		}

		// Check if account is currently locked
		if user.LockUntil != nil && user.LockUntil.After(time.Now()) {
			fun.HandleAPIErrorSimple(c, http.StatusTooManyRequests, "Account locked. Try again at "+user.LockUntil.Format(config.DATE_YYYY_MM_DD_HH_MM_SS))
			return
		}

		if !fun.IsPasswordMatched(loginForm.Password, user.Password) {
			user.LoginAttempts++
			now := time.Now()
			user.LastFailedLogin = &now

			// Lock the account if over the retry limit
			if user.LoginAttempts >= user.MaxRetry {
				lockUntil := now.Add(time.Duration(config.GetConfig().App.LoginLockUntil) * time.Minute)
				user.LockUntil = &lockUntil
			}

			db.Save(&user)

			locked := ""
			if user.LockUntil != nil && user.LockUntil.After(time.Now()) {
				locked = fmt.Sprintf(" Account locked until %s.", user.LockUntil.Format(config.DATE_YYYY_MM_DD_HH_MM_SS))
			}

			errorMessage := fmt.Sprintf(
				"Wrong Username or Password or Captcha. Attempt %d of %d.%s",
				user.LoginAttempts, user.MaxRetry, locked,
			)

			fun.HandleAPIErrorSimple(c, http.StatusUnauthorized, errorMessage)
			return
		}

		// LINE AFTER USER VERIFIED LOGIN...__________________________________________________________________________

		// Reset failed login attempts and lock
		user.LoginAttempts = 0
		user.LockUntil = nil

		errSet := redisDB.Set(context.Background(), "last_activity_time:"+user.Email, time.Now().UnixMilli(), 30*time.Minute).Err()
		if errSet != nil {
			fun.HandleAPIErrorSimple(c, http.StatusInternalServerError, "Internal Server Error, Error Saving to Memory : "+errSet.Error())
			return
		}

		// Check if its being activated
		var userStatuses []model.UserStatus
		db.Find(&userStatuses)
		for _, userStatus := range userStatuses {
			if userStatus.ID == uint(user.Status) {
				if userStatus.Title != "ACTIVE" {
					fun.HandleAPIErrorSimple(c, http.StatusUnauthorized, fmt.Sprintf("Please Contact Our Technical Support To Activate your Account @+%s", config.GetConfig().Whatsnyan.WATechnicalSupport))
					return
				}
			}
		}

		// Set session expiration time (e.g., 7 days in the future)
		currentUnixTime := time.Now().Unix() * 1000               // Convert to milliseconds
		futureTime := currentUnixTime + (7 * 24 * 60 * 60 * 1000) // 7 days in milliseconds
		user.SessionExpired = futureTime
		user.Session = fun.GenerateRandomString(40)
		user.UpdatedAt = time.Now()

		// SAVE LOGIN SESSION
		if err := db.Save(&user).Error; err != nil {
			fun.HandleAPIErrorSimple(c, http.StatusInternalServerError, "Failed to update user: "+err.Error())
			return
		}
		// ws.CloseWebsocketConnection(user.Email)

		var user_roles []struct {
			model.RolePrivilege
			Path string `json:"path" gorm:"column:path"`
		}

		if err := db.
			Table(fmt.Sprintf("%s rp", config.GetConfig().Database.TbRolePrivilege)).
			Unscoped(). // Disable soft deletes for this query
			Select("rp.*,f.path").
			Joins(fmt.Sprintf("LEFT JOIN %s f ON f.id = rp.feature_id", config.GetConfig().Database.TbFeature)).
			Where("rp.role_id = ?", user.Role).
			// Offset(0).
			// Limit(1).
			Find(&user_roles).Error; err != nil {
			fun.HandleAPIErrorSimple(c, http.StatusInternalServerError, "Error querying database: "+err.Error())
			return
		}

		// Initialize the dynamic map
		roleData := make(map[string]interface{})

		// Populate the map with path and privilege string
		for _, role := range user_roles {
			privilege := strconv.Itoa(int(role.CanCreate)) + strconv.Itoa(int(role.CanRead)) + strconv.Itoa(int(role.CanUpdate)) + strconv.Itoa(int(role.CanDelete))
			roleData[role.Path] = privilege
		}

		var roles model.Role
		if err := db.Where("id = ?", user.Role).First(&roles).Error; err != nil {
			fun.HandleAPIErrorSimple(c, http.StatusInternalServerError, "Error querying database: "+err.Error())
			return
		}
		authToken := fun.GenerateRandomString(40 + rand.Intn(25) + 1)
		randomToken := fun.GenerateRandomString(40 + rand.Intn(25) + 1)

		profile_image := "/assets/img/avatars/default.jpg"
		if user.ProfileImage != "" {
			profile_image = user.ProfileImage
		}
		imageMaps := map[string]interface{}{
			"t":  fun.GenerateRandomString(3),
			"id": user.ID,
		}
		pathString, err := fun.GetAESEcryptedURLfromJSON(imageMaps)
		if err != nil {
			fun.HandleAPIErrorSimple(c, http.StatusInternalServerError, "Could not encripting image "+err.Error())
			return
		}

		// Only override profile_image if the user doesn't have a custom one
		if user.ProfileImage == "" {
			profile_image = "/profile/default.jpg?f=" + pathString
		}

		// Create jwt.MapClaims and merge with roleData
		claims := map[string]interface{}{
			"id":              user.ID,
			"fullname":        user.Fullname,
			"username":        user.Username,
			"phone":           user.Phone,
			"email":           user.Email,
			"password":        user.Password,
			"type":            user.Type,
			"role":            user.Role,
			"role_name":       roles.RoleName,
			"profile_image":   profile_image,
			"status":          user.Status,
			"status_name":     "",
			"last_login":      time.Now(),
			"session":         user.Session,
			"session_expired": user.SessionExpired,
			"random":          randomToken,
			"auth":            authToken,
			"ip":              user.IP,
		}

		// Merge roleData into claims
		for k, v := range roleData {
			claims[k] = v
		}

		jsonText, err := json.Marshal(claims)
		if err != nil {
			fun.HandleAPIErrorSimple(c, http.StatusInternalServerError, "Could not string token "+err.Error())
			return
		}
		tokenString, err := fun.GetAESEncrypted(string(jsonText))
		if err != nil {
			fun.HandleAPIErrorSimple(c, http.StatusInternalServerError, "Could not encripting token "+err.Error())
			return
		}

		// Parse the login time as an integer
		loginExpiredMinutes := config.GetConfig().App.LoginTimeM
		if loginExpiredMinutes <= 0 {
			loginExpiredMinutes = 30
		}

		// Calculate the expiration time by adding loginExpiredMinutes to the current time
		expiration := time.Now().Add(time.Duration(loginExpiredMinutes) * time.Minute)
		// Set random token as cookie
		auth := &http.Cookie{
			Name:     "auth",
			Value:    authToken,
			Expires:  expiration,
			Path:     config.GLOBAL_URL,
			Domain:   config.GetConfig().App.CookieLoginDomain,
			SameSite: http.SameSiteStrictMode,
			Secure:   config.GetConfig().App.CookieLoginSecure,
			HttpOnly: true,
		}
		http.SetCookie(c.Writer, auth)

		// Set random token as cookie
		random := &http.Cookie{
			Name:     "random",
			Value:    randomToken,
			Expires:  expiration,
			Path:     config.GLOBAL_URL,
			Domain:   config.GetConfig().App.CookieLoginDomain,
			SameSite: http.SameSiteStrictMode,
			Secure:   config.GetConfig().App.CookieLoginSecure,
			HttpOnly: true,
		}
		http.SetCookie(c.Writer, random)

		// Set JWT token as cookie
		tokenCookie := &http.Cookie{
			Name:     "token",
			Value:    tokenString,
			Expires:  expiration,
			Path:     config.GLOBAL_URL,
			Domain:   config.GetConfig().App.CookieLoginDomain,
			SameSite: http.SameSiteStrictMode,
			Secure:   config.GetConfig().App.CookieLoginSecure,
			HttpOnly: true,
		}
		http.SetCookie(c.Writer, tokenCookie)

		// Create and set the "credentials" cookie
		credentialsCookie := &http.Cookie{
			Name:     "credentials",
			Value:    url.QueryEscape(user.Session),
			Expires:  expiration,
			Path:     config.GLOBAL_URL,
			Domain:   config.GetConfig().App.CookieLoginDomain,
			SameSite: http.SameSiteStrictMode,
			Secure:   config.GetConfig().App.CookieLoginSecure,
			HttpOnly: true,
		}
		http.SetCookie(c.Writer, credentialsCookie)

		// syncCookie := &http.Cookie{
		// 	Name:     "jm_id",
		// 	Value:    url.QueryEscape(user.Session),
		// 	Expires:  expiration,
		// 	Path:     config.GLOBAL_URL,
		// 	Domain:   config.GetConfig().App.CookieLoginDomain,
		// 	SameSite: http.SameSiteLaxMode,
		// 	Secure:   config.GetConfig().App.CookieLoginSecure,
		// 	HttpOnly: false,
		// }
		// http.SetCookie(c.Writer, syncCookie)
		// c.SecureJSON(http.StatusOK, gin.H{
		// 	"status": "01",
		// })

		// Save the newLog to the database
		db.Create(&model.LogActivity{
			UserID:    user.ID,
			FullName:  user.Fullname,
			Action:    "LOGIN",
			Status:    "Success",
			Log:       "User logged in successfully",
			IP:        c.ClientIP(),
			UserAgent: c.Request.UserAgent(),
			ReqMethod: c.Request.Method,
			ReqUri:    c.Request.RequestURI,
		})

		c.Redirect(http.StatusSeeOther, config.GLOBAL_URL+"page")
		c.Abort()
	}
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

		parameters := gin.H{
			"APP_NAME":         config.GetConfig().App.Name,
			"APP_LOGO":         config.GetConfig().App.Logo,
			"APP_VERSION":      config.GetConfig().App.Version,
			"APP_VERSION_NO":   config.GetConfig().App.VersionNo,
			"APP_VERSION_CODE": config.GetConfig().App.VersionCode,
			"APP_VERSION_NAME": config.GetConfig().App.VersionName,
		}
		if credentialsCookie != nil {
			var user model.Users
			if err := db.Where("session = ?", credentialsCookie.Value).First(&user).Error; err != nil {
				c.HTML(http.StatusOK, "forgot-password.html", parameters)
				return
			}
			c.Redirect(http.StatusFound, config.GLOBAL_URL+"page")
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

		parameters := gin.H{
			"APP_NAME":         config.GetConfig().App.Name,
			"APP_LOGO":         config.GetConfig().App.Logo,
			"APP_VERSION":      config.GetConfig().App.Version,
			"APP_VERSION_NO":   config.GetConfig().App.VersionNo,
			"APP_VERSION_CODE": config.GetConfig().App.VersionCode,
			"APP_VERSION_NAME": config.GetConfig().App.VersionName,
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
				<img src="` + config.GetConfig().App.WebPublicURL + config.GetConfig().App.Logo + `" alt="logo" width="180" height="101" style="display: block; margin: 0 auto;">
				<h1 style="color: #4287f5;">` + config.GetConfig().App.Name + ` Reset Password</h1>
				<p>Please click the button below to verify your email address:</p>
				<a href="` + config.GetConfig().App.WebPublicURL + `/reset-password/` + user.Email + "/" + randomAccessToken + `" style="text-decoration: none;">
					<button style="background-color: #4287f5; color: #fff; padding: 10px 20px; border: none; border-radius: 4px; cursor: pointer;">
						Reset Password
					</button>
				</a>
			</div>
		</body>`

		mailer := gomail.NewMessage()
		mailer.SetHeader("From", fmt.Sprintf("Email Verificator  <%s>", config.GetConfig().Email.Username))
		mailer.SetHeader("To", user.Email)
		mailer.SetHeader("Subject", "[noreply] Here Reset Password link")
		mailer.SetBody("text/html", htmlMailTemplate)

		dialer := gomail.NewDialer(
			config.GetConfig().Email.Host,
			config.GetConfig().Email.Port,
			config.GetConfig().Email.Username,
			config.GetConfig().Email.Password,
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

		parameters := gin.H{
			"APP_NAME":         config.GetConfig().App.Name,
			"APP_LOGO":         config.GetConfig().App.Logo,
			"APP_VERSION":      config.GetConfig().App.Version,
			"APP_VERSION_NO":   config.GetConfig().App.VersionNo,
			"APP_VERSION_CODE": config.GetConfig().App.VersionCode,
			"APP_VERSION_NAME": config.GetConfig().App.VersionName,
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
		var check_user_password_changelogs []model.UserPasswordChangeLog
		db.Where("email = ?", email).Order("created_at desc").Find(&check_user_password_changelogs)
		for _, data := range check_user_password_changelogs {
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

		var user_password_changelog model.UserPasswordChangeLog
		user_password_changelog.Email = user.Email
		user_password_changelog.Password = user.Password
		if err := db.Create(&user_password_changelog).Error; err != nil {
			fun.HandleAPIErrorSimple(c, http.StatusInternalServerError, "Error updating password changelog"+err.Error())
			return
		}
		var user_password_changelogs []model.UserPasswordChangeLog

		// Fetch the password change logs sorted by CreatedAt in ascending order
		if err := db.Where("email = ?", user.Email).
			Order("created_at asc").
			Find(&user_password_changelogs).Error; err != nil {
			fun.HandleAPIErrorSimple(c, http.StatusBadRequest, "Email not found")
			return
		}

		// If there are more than %d records, delete the oldest ones
		if len(user_password_changelogs) > numberOfPreviousPasswordsToCheck {
			for i := 0; i < len(user_password_changelogs)-numberOfPreviousPasswordsToCheck; i++ {
				db.Delete(&user_password_changelogs[i])
			}
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
		// Validate additional cookies
		if !fun.ValidateCookie(c, "credentials", claims["session"]) ||
			!fun.ValidateCookie(c, "auth", claims["auth"]) ||
			!fun.ValidateCookie(c, "random", claims["random"]) {
			fun.RemoveEmailSession(db, emailToken)
			fun.ClearCookiesAndRedirect(c, cookies)
			return
		}
		session, ok := claims["session"].(string)
		if !ok {
			fun.RemoveEmailSession(db, emailToken)
			fun.ClearCookiesAndRedirect(c, cookies)
			return
		}
		var user model.Users
		resultAdmin := db.Where("id = ?", claims["id"]).First(&user)
		if resultAdmin.Error != nil {
			fun.RemoveEmailSession(db, emailToken)
			fun.ClearCookiesAndRedirect(c, cookies)
			return
		}

		if user.Session != session {
			fun.RemoveEmailSession(db, emailToken)
			fun.ClearCookiesAndRedirect(c, cookies)
			return
		}

		var featuresPrivileges []struct {
			model.RolePrivilege
			ParentID  uint   `json:"parent_id" gorm:"column:parent_id"`
			Title     string `json:"title" gorm:"column:title"`
			Path      string `json:"path" gorm:"column:path"`
			MenuOrder uint   `json:"menu_order" gorm:"column:menu_order"`
			Status    uint   `json:"status" gorm:"column:status"`
			Level     uint   `json:"level" gorm:"column:level"`
			Icon      string `json:"icon" gorm:"column:icon"`
		}

		if err := db.
			Table(fmt.Sprintf("%s a", config.GetConfig().Database.TbRolePrivilege)).
			Unscoped(). // Disable soft deletes for this query
			Select("a.*, b.parent_id , b.title , b.path , b.menu_order , b.status , b.level , b.icon").
			Joins(fmt.Sprintf("LEFT JOIN %s b ON a.feature_id = b.id", config.GetConfig().Database.TbFeature)).
			Where("a.role_id = ?", claims["role"]).
			Order("b.menu_order").
			Find(&featuresPrivileges).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				fun.RemoveEmailSession(db, emailToken)
				fun.ClearCookiesAndRedirect(c, cookies)
				return
			}

			// Handle other errors
			fun.HandleAPIErrorSimple(c, http.StatusInternalServerError, "Error querying database: "+err.Error())
			fun.RemoveEmailSession(db, emailToken)
			fun.ClearCookiesAndRedirect(c, cookies)
			return
		}

		fileContent := ""

		fileContentTab := ""
		for _, featurePrivilegeParent := range featuresPrivileges {
			fileContentChild := ""
			menuToggle := ""

			if len(strings.TrimSpace(featurePrivilegeParent.Path)) == 0 {
				// Check if this parent has any accessible children before processing
				hasAccessibleChildren := false
				for _, featurePrivilege := range featuresPrivileges {
					if featurePrivilege.ParentID == featurePrivilegeParent.MenuOrder {
						// Check if the child has any permissions (at least read access)
						if featurePrivilege.CanCreate == 1 ||
							featurePrivilege.CanRead == 1 ||
							featurePrivilege.CanUpdate == 1 ||
							featurePrivilege.CanDelete == 1 {
							hasAccessibleChildren = true
							break
						}
					}
				}

				// If no accessible children, skip this parent menu entirely
				if !hasAccessibleChildren {
					continue
				}

				// Build child menu items only for accessible children
				for _, featurePrivilege := range featuresPrivileges {
					if featurePrivilege.ParentID == featurePrivilegeParent.MenuOrder {
						// Check if the child has any permissions (at least read access)
						if featurePrivilege.CanCreate == 0 &&
							featurePrivilege.CanRead == 0 &&
							featurePrivilege.CanUpdate == 0 &&
							featurePrivilege.CanDelete == 0 {
							continue // Skip this child menu if no permissions
						}
						// Use a stable i18n key for dynamic menu entries (derived from path when available)
						// Keep the original title as the visible fallback text so pages still render
						privKey := "menu." + strings.ReplaceAll(strings.Trim(featurePrivilege.Path, "/ "), "/", ".")
						if strings.TrimSpace(privKey) == "menu." {
							// Fallback: slugify the title if path is empty
							privKey = "menu." + strings.ToLower(strings.ReplaceAll(featurePrivilege.Title, " ", "_"))
						}
						fileContentChild += `        
						<li class="menu-item">
							<a href="#` + featurePrivilege.Path + `" 
							class="menu-link" 
							data-bs-toggle="tooltip" 
							data-bs-placement="right" 
							data-bs-original-title="` + featurePrivilege.Title + `">
								<div class="text-truncate" data-i18n="` + privKey + `">` + featurePrivilege.Title + `</div>
							</a>
						</li>`
					}
				}

				if len(fileContentChild) > 0 {
					fileContentChild = `<ul class="menu-sub">` + fileContentChild + `</ul>`
					menuToggle = "menu-toggle"
				}
			}

			if featurePrivilegeParent.Level == 0 && featurePrivilegeParent.Status == 1 {
				hrefPath := ""
				if len(featurePrivilegeParent.Path) != 0 {
					if featurePrivilegeParent.CanCreate == 0 &&
						featurePrivilegeParent.CanRead == 0 &&
						featurePrivilegeParent.CanUpdate == 0 &&
						featurePrivilegeParent.CanDelete == 0 {
						continue
					}
					hrefPath = `href="#` + featurePrivilegeParent.Path + `"`
				} else {
					if len(fileContentChild) == 0 {
						if featurePrivilegeParent.CanCreate == 0 &&
							featurePrivilegeParent.CanRead == 0 &&
							featurePrivilegeParent.CanUpdate == 0 &&
							featurePrivilegeParent.CanDelete == 0 {
							continue
						}
					}
				}

				fileContent += `
					<li class="menu-item ">
						<a ` + hrefPath + ` 
						class="menu-link ` + menuToggle + `"
						data-bs-toggle="tooltip" 
						data-bs-placement="right" 
						data-bs-original-title="` + featurePrivilegeParent.Title + `">
							<i class="menu-icon tf-icons ` + featurePrivilegeParent.Icon + `"></i>
							` + func() string {
					// Derive an i18n key for parent menu entries as well
					parentKey := "menu." + strings.ReplaceAll(strings.Trim(featurePrivilegeParent.Path, "/ "), "/", ".")
					if strings.TrimSpace(parentKey) == "menu." {
						parentKey = "menu." + strings.ToLower(strings.ReplaceAll(featurePrivilegeParent.Title, " ", "_"))
					}
					return `<div class="text-truncate" data-i18n="` + parentKey + `">` + featurePrivilegeParent.Title + `</div>`
				}() + `
						</a>
						` + fileContentChild + `
					</li>`

			}

			if len(featurePrivilegeParent.Path) > 0 {
				fileContentTab += `<div id="` + featurePrivilegeParent.Path + `" class="tab-content flex-grow-1 container-p-y d-none h-100"></div>` //` + string(fileContent) + `
			}
		}

		randomAccessToken := fun.GenerateRandomString(20 + rand.Intn(30) + 1)
		err = redisDB.Set(context.Background(), "web:"+session, randomAccessToken, 0).Err()
		if err != nil {
			fun.RemoveEmailSession(db, emailToken)
			fun.ClearCookiesAndRedirect(c, cookies)
			return
		}
		imageMaps := map[string]interface{}{
			"t":  fun.GenerateRandomString(3),
			"id": user.ID,
		}
		pathString, err := fun.GetAESEcryptedURLfromJSON(imageMaps)
		if err != nil {
			fun.HandleAPIErrorSimple(c, http.StatusInternalServerError, "Could not encripting image "+err.Error())
			return
		}
		profile_image := config.GLOBAL_URL + "profile/default.jpg?f=" + pathString

		// Check if user role has specific app configuration
		var appConfig model.AppConfig
		var appName, appLogo, appVersion, appVersionNo, appVersionCode, appVersionName string

		if err := db.Where("role_id = ? AND is_active = ?", user.Role, true).First(&appConfig).Error; err == nil {
			// Use role-specific app configuration
			appName = appConfig.AppName
			appLogo = appConfig.AppLogo
			appVersion = appConfig.AppVersion
			appVersionNo = appConfig.VersionNo
			appVersionCode = appConfig.VersionCode
			appVersionName = appConfig.VersionName
			// logrus.Infof("Using role-specific app config for role %d: %s", user.Role, appConfig.AppName)
		} else {
			// Fallback to default config
			appName = config.GetConfig().App.Name
			appLogo = config.GetConfig().App.Logo
			appVersion = config.GetConfig().App.Version
			appVersionNo = strconv.Itoa(config.GetConfig().App.VersionNo)
			appVersionCode = config.GetConfig().App.VersionCode
			appVersionName = config.GetConfig().App.VersionName
			// logrus.Infof("Using default app config for role %d (no specific config found)", user.Role)
		}

		c.HTML(http.StatusOK, "index.html", gin.H{
			"APP_NAME":         appName,
			"APP_LOGO":         appLogo,
			"APP_VERSION":      appVersion,
			"APP_VERSION_NO":   appVersionNo,
			"APP_VERSION_CODE": appVersionCode,
			"APP_VERSION_NAME": appVersionName,
			"ACCESS":           config.API_URL + randomAccessToken,
			"username":         claims["username"],
			"role":             claims["role_name"],
			"fullname":         claims["fullname"],
			"email":            claims["email"],
			"profile_image":    profile_image,
			"GLOBAL_URL":       config.GLOBAL_URL,
			"sidebar":          template.HTML(string(fileContent)),
			"contents":         template.HTML(string(fileContentTab)),
			"IsEnableDebug":    config.GetConfig().App.Debug,
		})

	}
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
		c.Redirect(http.StatusFound, config.GLOBAL_URL+"login")

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
			ReqUri:    c.Request.RequestURI,
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
		globalURL := config.GLOBAL_URL
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
		profile_image := "/profile/default.jpg?f=" + pathString

		var userStatusData model.UserStatus
		if err := db.First(&userStatusData, user.Status).Error; err != nil {
			logrus.Errorf("failed to parse data status for user: %v", err)
		}
		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("<div class='%s'>", userStatusData.ClassName))
		sb.WriteString(userStatusData.Title)
		sb.WriteString("</div>")
		userStatusHTML := sb.String()

		// Check if user role has specific app configuration
		var appConfig model.AppConfig
		var appName, appLogo, appVersion, appVersionNo, appVersionCode, appVersionName string

		if err := db.Where("role_id = ? AND is_active = ?", user.Role, true).First(&appConfig).Error; err == nil {
			// Use role-specific app configuration
			appName = appConfig.AppName
			appLogo = appConfig.AppLogo
			appVersion = appConfig.AppVersion
			appVersionNo = appConfig.VersionNo
			appVersionCode = appConfig.VersionCode
			appVersionName = appConfig.VersionName
			// logrus.Infof("Using role-specific app config for role %d: %s", user.Role, appConfig.AppName)
		} else {
			// Fallback to default config
			appName = config.GetConfig().App.Name
			appLogo = config.GetConfig().App.Logo
			appVersion = config.GetConfig().App.Version
			appVersionNo = strconv.Itoa(config.GetConfig().App.VersionNo)
			appVersionCode = config.GetConfig().App.VersionCode
			appVersionName = config.GetConfig().App.VersionName
			// logrus.Infof("Using default app config for role %d (no specific config found)", user.Role)
		}

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
			"profile_image":    profile_image,
			"ip":               user.IP,
			"GLOBAL_URL":       globalURL,
			"RANDOM_ACCESS":    randAccessToken,

			/* App Config */
			"TABLE_APP_CONFIGURATION": webguibuilder.TABLE_APP_CONFIGURATION(user.Session, redisDB),
		}
		c.HTML(http.StatusOK, componentID+".html", replacements)
	}
}
