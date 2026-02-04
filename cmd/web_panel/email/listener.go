package email

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"regexp"
	"service-platform/cmd/web_panel/controllers"
	"service-platform/cmd/web_panel/fun"
	"service-platform/cmd/web_panel/model"
	"service-platform/internal/config"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/emersion/go-imap"
	idle "github.com/emersion/go-imap-idle"
	"github.com/emersion/go-imap/client"
	"github.com/emersion/go-message/mail"
	"github.com/microcosm-cc/bluemonday"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

var IsYahooMailListenerRunning bool

type ParsedEmailBody struct {
	IsHTML     bool
	PlainText  string
	HTMLText   string
	Links      []string
	Emails     []string
	RawContent string
}

func parseEmailBody(r io.Reader) (*ParsedEmailBody, error) {
	mr, err := mail.CreateReader(r)
	if err != nil {
		return nil, fmt.Errorf("failed to create mail reader: %w", err)
	}

	var result ParsedEmailBody
	var htmlBuf, textBuf bytes.Buffer

	// Loop through all parts
	for {
		part, err := mr.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("error reading MIME part: %w", err)
		}

		body, err := io.ReadAll(part.Body)
		if err != nil {
			return nil, fmt.Errorf("error reading part body: %w", err)
		}

		switch h := part.Header.(type) {
		case *mail.InlineHeader:
			ct, _, _ := h.ContentType()
			switch {
			case strings.Contains(ct, "text/html"):
				result.IsHTML = true
				htmlBuf.Write(body)
			case strings.Contains(ct, "text/plain"):
				textBuf.Write(body)
			}
		}
	}

	result.HTMLText = strings.TrimSpace(htmlBuf.String())
	result.PlainText = strings.TrimSpace(textBuf.String())
	result.RawContent = result.HTMLText
	if !result.IsHTML {
		result.RawContent = result.PlainText
	}

	// Parse HTML body for links and emails
	if result.IsHTML && result.HTMLText != "" {
		doc, err := goquery.NewDocumentFromReader(strings.NewReader(result.HTMLText))
		if err == nil {
			// Extract visible text (strip HTML)
			result.PlainText = doc.Text()

			// Extract links
			doc.Find("a").Each(func(i int, s *goquery.Selection) {
				if href, ok := s.Attr("href"); ok {
					result.Links = append(result.Links, href)
				}
			})

			// Extract visible emails
			doc.Find("body").Each(func(i int, s *goquery.Selection) {
				text := s.Text()
				for _, word := range strings.Fields(text) {
					if strings.Contains(word, "@") && strings.Contains(word, ".") {
						result.Emails = append(result.Emails, word)
					}
				}
			})
		}
	}

	return &result, nil
}

func StartYahooMailListener(ctx context.Context, db *gorm.DB) {
	go func() {
		for {
			if !IsYahooMailListenerRunning {
				logrus.Println("📧 Starting Yahoo Mail listener...")
				IsYahooMailListenerRunning = true
			}

			err := connectAndIdle(ctx, db)
			if err != nil {
				logrus.Println("❌ Email listener error: ", err)
			}

			select {
			case <-ctx.Done():
				logrus.Println("🛑 Context cancelled. Stopping email listener.")
				return
			case <-time.After(10 * time.Second):
				// logrus.Println("🔁 Retrying Yahoo Mail connection...")
			}
		}
	}()
}

// connectAndIdle connects to the IMAP server and enters IDLE mode to listen for new emails
func connectAndIdle(ctx context.Context, db *gorm.DB) error {
	cfg := config.WebPanel.Get()

	c, err := client.DialTLS(fmt.Sprintf("%s:%d", cfg.Email.ListenerHost, cfg.Email.ListenerPort), nil)
	if err != nil {
		return fmt.Errorf("failed to dial IMAP: %w", err)
	}
	defer func() {
		_ = c.Logout()
		// logrus.Println("🔌 Logged out from Yahoo IMAP server.")
	}()

	if err := c.Login(cfg.Email.ListenerEmail, cfg.Email.ListenerPassword); err != nil {
		return fmt.Errorf("login failed: %w", err)
	}
	// logrus.Println("✅ Logged in to Yahoo IMAP.")

	c.Updates = make(chan client.Update, 10)

	if _, err := c.Select("INBOX", false); err != nil {
		return fmt.Errorf("failed to select INBOX: %w", err)
	}
	// logrus.Println("📂 INBOX selected.")

	checkUnseen := func() {
		criteria := imap.NewSearchCriteria()
		criteria.WithoutFlags = []string{imap.SeenFlag}

		now := time.Now()
		yesterday := now.AddDate(0, 0, -1)
		tomorrow := now.AddDate(0, 0, 1)

		criteria.Since = yesterday
		criteria.Before = tomorrow

		ids, err := c.Search(criteria)
		if err != nil {
			skippedErrMsg := []string{
				"no mailbox selected",
				"imap: connection closed",
			}
			errMsg := strings.ToLower(err.Error())
			for _, skip := range skippedErrMsg {
				if strings.Contains(errMsg, skip) {
					return
				}
			}
			logrus.Println("🔍 Search error: ", err)
			return
		}

		if len(ids) == 0 {
			return
		}

		seqset := new(imap.SeqSet)
		seqset.AddNum(ids...)
		messages := make(chan *imap.Message, 10)
		done := make(chan error, 1)

		go func() {
			done <- c.Fetch(seqset, []imap.FetchItem{imap.FetchEnvelope, imap.FetchBodyStructure, imap.FetchRFC822}, messages)
		}()

		for msg := range messages {
			if msg.Envelope != nil {
				// fmt.Printf("📩 New UNREAD email: %s\n", msg.Envelope.Subject)
				// fmt.Printf("📨 Subject: %s\n", msg.Envelope.Subject)
				// fmt.Printf("📅 Date: %v\n", msg.Envelope.Date)
				// fmt.Printf("📧 From: %s\n", formatAddresses(msg.Envelope.From))
				// fmt.Printf("👥 To: %s\n", formatAddresses(msg.Envelope.To))

				section := &imap.BodySectionName{}
				r := msg.GetBody(section)
				if r == nil {
					fmt.Println("⚠️ No message body found")
					continue
				}

				parsed, err := parseEmailBody(r)
				if err != nil {
					fmt.Printf("❌ Error parsing body: %v\n", err)
					return
				}

				// fmt.Println("📃 Plain Text:\n", parsed.PlainText)
				// fmt.Println("🌐 Links Found:", parsed.Links)
				// fmt.Println("📧 Emails Found:", parsed.Emails)

				linksJSON, _ := json.Marshal(parsed.Links)
				emailsJSON, _ := json.Marshal(parsed.Emails)

				unreadMail := model.IncomingEmail{
					MessageID:  msg.Envelope.MessageId,
					Subject:    msg.Envelope.Subject,
					FromEmail:  formatAddresses(msg.Envelope.From),
					ToEmail:    formatAddresses(msg.Envelope.To),
					DateEmail:  msg.Envelope.Date,
					PlainText:  parsed.PlainText,
					HTMLText:   parsed.HTMLText,
					Links:      string(linksJSON),
					Emails:     string(emailsJSON),
					IsHTML:     parsed.IsHTML,
					RawContent: parsed.RawContent,
					IsRead:     false,
				}

				var existing model.IncomingEmail
				errCheck := db.Where("message_id = ?", unreadMail.MessageID).First(&existing).Error

				if errCheck != nil {
					if errCheck == gorm.ErrRecordNotFound {
						// Not found, insert new
						if err := db.Create(&unreadMail).Error; err != nil {
							logrus.Errorf("Failed to insert new email: %v", err)
						} else {
							// logrus.Infof("✅ New email inserted: %s", unreadMail.Subject)
						}
					} else {
						logrus.Errorf("Failed to check existing email: %v", errCheck)
					}
				} else {
					// Found, update
					if err := db.Model(&existing).Updates(unreadMail).Error; err != nil {
						logrus.Errorf("Failed to update existing email: %v", err)
					} else {
						// logrus.Infof("🔁 Existing email updated: %s", unreadMail.Subject)
					}
				}

				// Process incoming customer email for potential complaint ticket creation
				go handleIncomingPotentialComplaintTicket(unreadMail.MessageID, db)
			}
		}

		if err := <-done; err != nil {
			fmt.Printf("⚠️ Fetch error: %v\n", err)
		}
	}

	checkUnseen() // initial scan

	// 🕐 Start fallback polling every 60s
	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				// logrus.Println("🔄 Periodic check for unseen emails...")
				checkUnseen()
			}
		}
	}()

	idleClient := idle.NewClient(c)

	for {
		select {
		case <-ctx.Done():
			return nil
		default:
			// logrus.Println("🕒 Entering IDLE mode...")
			stop := make(chan struct{})
			done := make(chan error, 1)

			go func() {
				done <- idleClient.IdleWithFallback(stop, 10*time.Second)
			}()

			select {
			case <-ctx.Done():
				close(stop)
				<-done
				return nil
			case err := <-done:
				close(stop)

				if err != nil {
					skippedErrMsg := []string{
						"idle invalid arguments",
					}
					errMsg := strings.ToLower(err.Error())
					for _, skip := range skippedErrMsg {
						if strings.Contains(errMsg, skip) {
							return nil
						}
					}

					logrus.Println("⚠️ IDLE error: ", err)
					return err
				}
				checkUnseen() // fallback check after IDLE finishes
			}
		}
	}
}

func formatAddresses(addrs []*imap.Address) string {
	var result []string
	for _, addr := range addrs {
		// Format: "Name <local@domain>"
		name := addr.PersonalName
		email := addr.MailboxName + "@" + addr.HostName

		if name != "" {
			result = append(result, fmt.Sprintf("%s <%s>", name, email))
		} else {
			result = append(result, email)
		}
	}
	return strings.Join(result, ", ")
}

func handleIncomingPotentialComplaintTicket(messageID string, db *gorm.DB) {
	var dataEmail model.IncomingEmail
	if err := db.Where("message_id = ?", messageID).First(&dataEmail).Error; err != nil {
		logrus.Errorf("got error while trying to find email for process its request: %v", err)
		return
	}

	// ADD verify check for trusted client
	// e.g. ⮯
	// var clientData model.TrustedClient
	// if err := db.Where("FIND_IN_SET(?, emails) > 0", dataEmail.FromEmail).First(&clientData).Error; err != nil {
	// 	logrus.Errorf("trusted client not found for email %s: %v", dataEmail.FromEmail, err)
	// 	return
	// }

	// Parsing body email
	var bodyEmailToUse string
	if dataEmail.IsHTML {
		if len(dataEmail.HTMLText) > 0 {
			bodyEmailToUse = dataEmail.HTMLText
		}
	} else {
		if len(dataEmail.PlainText) > 0 {
			bodyEmailToUse = dataEmail.PlainText
		}
	}
	if bodyEmailToUse == "" {
		logrus.Errorf("No usable email body found for message ID: %s", dataEmail.MessageID)
		return
	}

	// ADD needed keywords
	keyWords := []string{
		"keterangan",
		"ket:",

		"kendala",

		"phone:",
		"no hp",
		"no. hp",
		"no_hp",
		"telepon",
	}
	found := FindAllKeyValueMatches(bodyEmailToUse, keyWords)

	// for k, v := range found {
	// 	fmt.Printf("Key: %s\n", k)
	// 	for i, val := range v {
	// 		fmt.Printf("  Match %d: %s\n", i+1, val)
	// 	}
	// }

	// Example: search for a specific key, e.g. "keterangan"
	var emailKeterangan, emailPhoneNumber string
	// Find "keterangan"
	if values, ok := found["keterangan"]; ok && len(values) > 0 {
		emailKeterangan = values[0]
	} else {
		logrus.Errorf("no values found for key 'keterangan'")
	}

	// Find "no. hp" or "no hp"
	if values, ok := found["no. hp"]; ok && len(values) > 0 {
		emailPhoneNumber = values[0]
	} else if values, ok := found["no hp"]; ok && len(values) > 0 {
		emailPhoneNumber = values[0]
	} else {
		logrus.Errorf("no values found for key 'no. hp' or 'no hp'")
	}

	// Create new ticket in ODOO if enabled
	if config.WebPanel.Get().HommyPayCCData.EmailListenerCreateNewTicketInODOO {
		// Ticket Subject e.g. HPY/dd/mm/yyyy/random // FIX: ticket subject so its not random
		ticketSubject := fmt.Sprintf("HPY/%s/%s", time.Now().Format("02/01/2006"), fun.GenerateRandomString(30))

		// Insert to DB ticket first
		newTicket := model.TicketHommyPayCC{
			TicketNumber:  ticketSubject,
			Description:   emailKeterangan,
			CustomerPhone: emailPhoneNumber,
			StatusInOdoo:  "Draft",
			Priority:      "0", // ADD priority of the case
		}

		if errCheck := db.First(&model.TicketHommyPayCC{}, "ticket_number = ?", newTicket.TicketNumber).Error; errCheck != nil {
			if errCheck == gorm.ErrRecordNotFound {
				if err := db.Create(&newTicket).Error; err != nil {
					logrus.Errorf("Failed to create new ticket: %v", err)
				} else {
					// logrus.Infof("✅ New ticket created: %s", newTicket.TicketNumber)
					// newTicket.ID is now populated with the auto-incremented primary key
				}
			} else {
				logrus.Errorf("Failed to check if ticket exists: %v", errCheck)
			}
		} else {
			logrus.Warnf("Ticket Number %s already exists, skipping creation", newTicket.TicketNumber)
		}

		// Create new ticket in ODOO
		controllers.TriggerInsertDatatoODOO <- controllers.InsertedDataTriggerItem{
			Database: db,
			IDinDB:   newTicket.ID,
		}
	}

}

// CleanHTML removes HTML tags and scripts, returns plain text
func CleanHTML(input string) string {
	p := bluemonday.StrictPolicy()
	return p.Sanitize(input)
}

// FindAllKeyValueMatches scans text and returns a map of all key-value pairs matching the pattern
func FindAllKeyValueMatches(text string, keys []string) map[string][]string {
	// Replace <br> with newline BEFORE sanitizing
	text = strings.ReplaceAll(text, "<br>", "\n")
	text = strings.ReplaceAll(text, "<br/>", "\n")
	text = strings.ReplaceAll(text, "<br />", "\n")

	cleanText := CleanHTML(text)
	result := make(map[string][]string)

	lines := strings.Split(cleanText, "\n")

	for _, line := range lines {
		lowerLine := strings.ToLower(strings.TrimSpace(line))

		for _, key := range keys {
			normalizedKey := strings.ToLower(key)
			normalizedKey = strings.ReplaceAll(normalizedKey, "_", "[\\s_\\-]*")
			re := regexp.MustCompile(fmt.Sprintf(`(?i)%s\s*[:\-]?\s*(.+?)$`, normalizedKey)) // non-greedy match

			match := re.FindStringSubmatch(lowerLine)
			if len(match) >= 2 {
				val := strings.TrimSpace(match[1])
				if val != "" {
					result[key] = append(result[key], val)
				}
			}
		}
	}

	return result
}

// func FindAllKeyValueMatches(text string, keys []string) map[string][]string {
// 	cleanText := CleanHTML(text)
// 	result := make(map[string][]string)

// 	lines := strings.Split(cleanText, "\n")

// 	for _, line := range lines {
// 		lowerLine := strings.ToLower(line)

// 		for _, key := range keys {
// 			normalizedKey := strings.ToLower(key)
// 			normalizedKey = strings.ReplaceAll(normalizedKey, "_", "[\\s_\\-]*")
// 			re := regexp.MustCompile(fmt.Sprintf(`(?i)%s\s*[:\-]?\s*(.+)`, normalizedKey))

// 			match := re.FindStringSubmatch(lowerLine)
// 			if len(match) >= 2 {
// 				val := strings.TrimSpace(match[1])
// 				// Truncate at punctuation if needed
// 				if idx := strings.IndexAny(val, ".!?"); idx != -1 {
// 					val = strings.TrimSpace(val[:idx])
// 				}
// 				if val != "" {
// 					result[key] = append(result[key], val)
// 				}
// 			}
// 		}
// 	}

// 	return result
// }
