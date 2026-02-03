package controllers

import (
	"context"
	"net/http"
	"os"
	"service-platform/cmd/technical_assistance/fun"
	"service-platform/cmd/technical_assistance/model"
	"strconv"
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
			"APP_NAME":         os.Getenv("APP_NAME"),
			"APP_LOGO":         os.Getenv("APP_LOGO"),
			"APP_VERSION":      os.Getenv("APP_VERSION"),
			"APP_VERSION_NO":   os.Getenv("APP_VERSION_NO"),
			"APP_VERSION_CODE": os.Getenv("APP_VERSION_CODE"),
			"APP_VERSION_NAME": os.Getenv("APP_VERSION_NAME"),
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
				<img src="` + os.Getenv("WEB_PUBLIC_URL") + os.Getenv("APP_LOGO") + `" alt="BP" width="180" height="101" style="display: block; margin: 0 auto;">
				<h1 style="color: #4287f5;">` + os.Getenv("APP_NAME") + ` Reset Password</h1>
				<p>Please click the button below to verify your email address:</p>
				<a href="` + os.Getenv("WEB_PUBLIC_URL") + `/reset-password/` + admin.Email + "/" + randomAccessToken + `" style="text-decoration: none;">
					<button style="background-color: #4287f5; color: #fff; padding: 10px 20px; border: none; border-radius: 4px; cursor: pointer;">
						Reset Password
					</button>
				</a>
			</div>
		</body>`

		mailer := gomail.NewMessage()
		mailer.SetHeader("From", "Email Verificator  <"+os.Getenv("CONFIG_SMTP_SENDER")+">")
		mailer.SetHeader("To", admin.Email)
		mailer.SetHeader("Subject", "[noreply] Here Reset Password link")
		mailer.SetBody("text/html", htmlMailTemplate)

		smtpPortStr := os.Getenv("CONFIG_SMTP_PORT")
		smtpPort, oops := strconv.Atoi(smtpPortStr)
		if oops != nil {
			smtpPort = 587
		}
		dialer := gomail.NewDialer(
			os.Getenv("CONFIG_SMTP_HOST"),
			smtpPort,
			os.Getenv("CONFIG_AUTH_EMAIL"),
			os.Getenv("CONFIG_AUTH_PASSWORD"),
		)

		errMailDialer := dialer.DialAndSend(mailer)

		if errMailDialer != nil {
			parameters["msg"] = "Failed to Send Email Reset Verification, Please Check Your Email Validity"
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
