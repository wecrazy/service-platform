package fun

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"
	"service-platform/cmd/web_panel/config"

	"github.com/Boostport/mjml-go"
	"github.com/sirupsen/logrus"
	"gopkg.in/gomail.v2"
)

type EmailAttachment struct {
	FilePath    string
	NewFileName string
}

func Sendmail(to, body string) error {
	mailer := gomail.NewMessage()
	mailer.SetHeader("From", "Email Verificator  <"+config.GetConfig().Email.Sender+">")
	mailer.SetHeader("To", to)
	mailer.SetHeader("Subject", "[noreply] Here Reset Password link")
	mailer.SetBody("text/html", body)

	dialer := gomail.NewDialer(
		config.GetConfig().Email.Host,
		config.GetConfig().Email.Port,
		config.GetConfig().Email.Username,
		config.GetConfig().Email.Password,
	)

	return dialer.DialAndSend(mailer)
}

// ConvertMJMLToHTML converts MJML content to HTML using mjml-go
func ConvertMJMLToHTML(mjmlContent string) (string, error) {
	// Correctly call mjml.ToHTML
	output, err := mjml.ToHTML(context.Background(), mjmlContent, mjml.WithMinify(true))
	if err != nil {
		var mjmlError mjml.Error
		if errors.As(err, &mjmlError) {
			// Convert error details to a readable string format
			detailsStr := formatMJMLErrorDetails(mjmlError.Details)
			return "", fmt.Errorf("MJML conversion error: %s - %s", mjmlError.Message, detailsStr)
		}
		return "", err
	}
	return output, nil
}

// Helper function to format MJML error details
func formatMJMLErrorDetails(details []struct {
	Line    int    `json:"line"`
	Message string `json:"message"`
	TagName string `json:"tagName"`
}) string {
	var formattedDetails []string
	for _, detail := range details {
		formattedDetails = append(formattedDetails, fmt.Sprintf("Line %d: %s (Tag: %s)", detail.Line, detail.Message, detail.TagName))
	}
	return strings.Join(formattedDetails, "; ")
}

// Send email
func TrySendEmail(to, cc, bcc []string, subject string, mjmlBody string, attachments []EmailAttachment) error {
	if len(to) == 0 {
		return errors.New("no recipients specified")
	}

	// Convert MJML to HTML
	htmlBody, err := ConvertMJMLToHTML(mjmlBody)
	if err != nil {
		fmt.Printf("❌ Failed to convert MJML to HTML: %v\n", err)
		return err
	}

	config := config.GetConfig().Email

	d := gomail.NewDialer(config.Host, config.Port, config.Username, config.Password)
	d.TLSConfig = &tls.Config{InsecureSkipVerify: true}

	// Create email message
	m := gomail.NewMessage()
	m.SetHeader("From", fmt.Sprintf("\"%s\" <%s>", config.Sender, config.Username))
	m.SetHeader("To", to...)
	if len(cc) > 0 {
		m.SetHeader("Cc", cc...)
	}

	if len(bcc) > 0 {
		m.SetHeader("Bcc", bcc...)
	}
	m.SetHeader("Subject", subject)
	m.SetBody("text/html", htmlBody)

	// Attachments
	if len(attachments) > 0 {
		for _, attachment := range attachments {
			if _, err := os.Stat(attachment.FilePath); err == nil {
				m.Attach(attachment.FilePath, gomail.Rename(attachment.NewFileName))
			} else {
				logrus.Warnf("File does not exist: %s", attachment.FilePath)
			}
		}
	}

	// Retry logic for sending emails
	for i := 0; i < config.MaxRetry; i++ {
		err = d.DialAndSend(m)
		if err == nil {
			logrus.Infof("Email sent successfully - Subject: '%s', To: [%s], CC: [%s], BCC: [%s], Attempt: %d/%d",
				subject,
				strings.Join(to, ", "),
				strings.Join(cc, ", "),
				strings.Join(bcc, ", "),
				i+1,
				config.MaxRetry)

			return nil
		}

		logrus.Errorf("⚠ Attempt %d/%d failed to send email: %v\n", i+1, config.MaxRetry, err)
		time.Sleep(time.Duration(config.RetryDelay) * time.Second)
	}

	return err
}
