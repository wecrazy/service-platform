package fun

import (
	"service-platform/internal/config"
	"strconv"

	"gopkg.in/gomail.v2"
)

func SendEmail(to, body string) error {
	mailer := gomail.NewMessage()
	mailer.SetHeader("From", "Email Verificator  <"+config.TechnicalAssistance.Get().CONFIG_SMTP_SENDER+">")
	mailer.SetHeader("To", to)
	mailer.SetHeader("Subject", "[noreply] Here Reset Password link")
	mailer.SetBody("text/html", body)

	smtpPortStr := config.TechnicalAssistance.Get().CONFIG_SMTP_PORT
	smtpPort, oops := strconv.Atoi(smtpPortStr)
	if oops != nil {
		smtpPort = 587
	}
	dialer := gomail.NewDialer(
		config.TechnicalAssistance.Get().CONFIG_SMTP_HOST,
		smtpPort,
		config.TechnicalAssistance.Get().CONFIG_AUTH_EMAIL,
		config.TechnicalAssistance.Get().CONFIG_AUTH_PASSWORD,
	)

	return dialer.DialAndSend(mailer)
}
