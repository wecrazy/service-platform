package controllers

import (
	"crypto/tls"
	"errors"
	"fmt"
	"log"
	"os"
	"service-platform/cmd/technical_assistance/config"
	"time"

	"github.com/gin-gonic/gin"
	"gopkg.in/gomail.v2"
	"gorm.io/gorm"
)

type EmailAttachment struct {
	FilePath    string
	NewFileName string
}

type TechnicianInfo struct {
	SPL  string
	Head string
}

// di controller
func SendReportHandler(db *gorm.DB, dbWeb *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		success, err := GenerateDailyReportTAActivity(db, dbWeb)
		if err != nil {
			c.JSON(500, gin.H{
				"message": "Gagal generate report",
				"error":   err.Error(),
			})
			return
		}

		if success {
			c.JSON(200, gin.H{
				"message": fmt.Sprintf("Berhasil kirim report TA @ %s", time.Now()),
			})
		} else {
			c.JSON(400, gin.H{
				"message": "Gagal generate report, unknown reason",
			})
		}
	}
}

func GenerateDailyReportTAActivity(db *gorm.DB, dbWeb *gorm.DB) (bool, error) {
	excelFileName, excelFilePath, err := GenerateTAExcelReport(db, dbWeb)
	if err != nil {
		return false, err
	}

	if excelFileName == "" && excelFilePath == "" {
		return false, errors.New("no excel found")
	}

	excelFileName2, excelFilePath2, err := GenerateTAMonthlyExcelReport(db)
	if err != nil {
		return false, err
	}

	if excelFileName2 == "" && excelFilePath2 == "" {
		return false, errors.New("no excel monthly found")
	}

	// Send report to email
	emailAttachments := []EmailAttachment{
		{
			FilePath:    excelFilePath,
			NewFileName: excelFileName,
		},
		{
			FilePath:    excelFilePath2,
			NewFileName: excelFileName2,
		},
	}
	config := config.GetConfig()

	emailSubject := fmt.Sprintf("Technical Assistance Log Activity @%v", time.Now().Add(7*time.Hour).Format("02 January 2006"))
	emailMsg := `
		<html>
			<body>
				<i>Dear All,</i><br><br>
				We would like to attach the report regarding the report of ta log activity.<br><br><br>
				Best Regards,<br><br>
				<b><i>PT. Cyber Smart Network Asia</i></b>
			</body>
		</html>`
	err = SendMail(config.Report.To, config.Report.Cc, emailSubject, emailMsg, emailAttachments)
	if err != nil {
		errMsg := fmt.Sprintf("got error while try to send mailer daily ta report :%v", err)
		log.Print(errMsg)
		return false, errors.New(errMsg)
	}

	log.Printf("%v successfully generated and send via email!", excelFileName)
	return true, nil
}

func SendMail(to []string, cc []string, subject string, message string, attachments []EmailAttachment) error {
	config := config.GetConfig()

	m := gomail.NewMessage()

	m.SetHeader("From", fmt.Sprintf("\"%s\" <%s>", "Service Report", config.Email.Username))
	m.SetHeader("To", to...)
	m.SetHeader("Cc", cc...)
	m.SetHeader("Subject", subject)

	m.SetBody("text/html", message)

	for _, attachment := range attachments {
		if _, err := os.Stat(attachment.FilePath); err == nil {
			m.Attach(attachment.FilePath, gomail.Rename(attachment.NewFileName))
		} else {
			log.Printf("File does not exist: %s", attachment.FilePath)
		}
	}

	d := gomail.NewDialer(config.Email.Host, config.Email.Port, config.Email.Username, config.Email.Password)
	d.TLSConfig = &tls.Config{InsecureSkipVerify: true}

	var err error
	for i := 0; i < config.Email.MaxRetry; i++ {
		err = d.DialAndSend(m)
		if err == nil {
			// If no error, email is sent successfully, break out of the loop
			// here u add log to log the mail send !!
			return nil
		}

		log.Printf("Attempt %d/%d failed to send email: %v", i+1, config.Email.MaxRetry, err)
		if i < config.Email.MaxRetry-1 {
			time.Sleep(time.Duration(config.Email.RetryDelay) * time.Second)
		}
	}

	return err
}
