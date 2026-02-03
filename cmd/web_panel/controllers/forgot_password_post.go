package controllers

import (
	"context"
	"fmt"
	"net/http"
	"service-platform/cmd/web_panel/config"
	"service-platform/cmd/web_panel/fun"
	"service-platform/cmd/web_panel/model"
	"strings"
	"time"

	"github.com/dchest/captcha"
	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"gopkg.in/gomail.v2"
	"gorm.io/gorm"
)

func PostForgotPassword(db *gorm.DB, redisDB *redis.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Extract form data
		email := c.PostForm("email")
		captchaText := c.PostForm("captcha")

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
			"MSG_HEADER":       "Please, Contact Admin",
			"EMAIL_DOMAIN":     "",
			"EMAIL":            email,
			"DISABLED":         "disabled",
			"msg":              "",
		}
		// Perform necessary actions (e.g., send a reset link, validate email, etc.)
		if email == "" {
			parameters["msg"] = "Please Fill The Email"
			c.HTML(http.StatusOK, "verify-email.html", parameters)
			return
		}
		if !captcha.VerifyString(captchaID, captchaText) {
			// c.JSON(http.StatusUnauthorized, gin.H{"error": "Wrong Username or Password or captcha"})
			parameters["msg"] = "INVALID EMAIL or CAPTCHA"
			c.HTML(http.StatusOK, "verify-email.html", parameters)
			return
		}
		var admin model.Admin
		if err := db.Where("email = ?", email).First(&admin).Error; err != nil {
			parameters["msg"] = "INVALID EMAIL or CAPTCHA :"
			c.HTML(http.StatusOK, "verify-email.html", parameters)
			return
		}
		// DATA _________________________________
		parts := strings.Split(admin.Email, "@")
		if len(parts) != 2 {
			parts[1] = ""
		}
		// DATA _____________________________________

		parameters["MSG_HEADER"] = "Please, Check Your Email"
		parameters["DISABLED"] = ""
		parameters["EMAIL_DOMAIN"] = parts[1]
		parameters["EMAIL"] = admin.Email
		parameters["msg"] = "Please Verify Your Email Address Link Sended To Your Email Address "
		val, _ := redisDB.Get(context.Background(), "reset_pwd:"+admin.Email).Result()
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
				<a href="` + config.GetConfig().App.WebPublicURL + `/reset-password/` + admin.Email + "/" + randomAccessToken + `" style="text-decoration: none;">
					<button style="background-color: #4287f5; color: #fff; padding: 10px 20px; border: none; border-radius: 4px; cursor: pointer;">
						Reset Password
					</button>
				</a>
			</div>
		</body>`

		mailer := gomail.NewMessage()
		mailer.SetHeader("From", fmt.Sprintf("Email Verificator  <%s>", config.GetConfig().Email.Username))
		mailer.SetHeader("To", admin.Email)
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
			errSet := redisDB.Set(context.Background(), "reset_pwd:"+admin.Email, randomAccessToken, 60*time.Minute).Err()
			if errSet != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Error Created Random Token Cache"})
				return
			}
			c.HTML(http.StatusOK, "verify-email.html", parameters)
		}
	}
}
