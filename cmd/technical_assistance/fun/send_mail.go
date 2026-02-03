package fun

import (
	"os"
	"strconv"

	"gopkg.in/gomail.v2"
)

func SendEmail(to, body string) error {
	mailer := gomail.NewMessage()
	mailer.SetHeader("From", "Email Verificator  <"+os.Getenv("CONFIG_SMTP_SENDER")+">")
	mailer.SetHeader("To", to)
	mailer.SetHeader("Subject", "[noreply] Here Reset Password link")
	mailer.SetBody("text/html", body)

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

	return dialer.DialAndSend(mailer)
}
