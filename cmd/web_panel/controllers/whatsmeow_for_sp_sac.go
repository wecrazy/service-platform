package controllers

import (
	"context"
	"fmt"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"service-platform/cmd/web_panel/config"
	"service-platform/cmd/web_panel/internal/gormdb"
	"service-platform/cmd/web_panel/model"
	sptechnicianmodel "service-platform/cmd/web_panel/model/sp_technician_model"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	"google.golang.org/protobuf/proto"
	"gorm.io/gorm"
)

var (
	WhatsappEventsMsgForSPSAC     *events.Message
	WhatsappEventsReceiptForSPSAC *events.Receipt
)

func handleRepliesReactionsAndEditedMsgForSPSAC(forProject string) {
	e := WhatsappEventsMsgForSPSAC
	dbWeb := gormdb.Databases.Web

	dataHRD := config.GetConfig().Default.PTHRD

	cwd, err := os.Getwd()
	if err != nil {
		logrus.Errorf("Failed to get working directory: %v", err)
		return
	}

	baseDir := filepath.Join(cwd, "web", "file")
	if _, err := os.Stat(baseDir); os.IsNotExist(err) {
		logrus.Errorf("Directory does not exist: %s\n", baseDir)
		return
	}

	loc, _ := time.LoadLocation(config.GetConfig().Default.Timezone)
	now := time.Now().In(loc)

	today := now.Format("2006-01-02")
	uploadDir := filepath.Join(baseDir, "wa_reply", today)
	if err := os.MkdirAll(uploadDir, os.ModePerm); err != nil {
		logrus.Errorf("Failed to create upload directory: %v", err)
		return
	}

	whatSP := "SP_SAC"

	// 🔧 Extract context info from any type
	var ctxInfo *waE2E.ContextInfo

	switch {
	case e.Message.ExtendedTextMessage != nil:
		ctxInfo = e.Message.ExtendedTextMessage.GetContextInfo()

	case e.Message.ImageMessage != nil:
		ctxInfo = e.Message.ImageMessage.GetContextInfo()

	case e.Message.VideoMessage != nil:
		ctxInfo = e.Message.VideoMessage.GetContextInfo()

	case e.Message.DocumentMessage != nil:
		ctxInfo = e.Message.DocumentMessage.GetContextInfo()

	case e.Message.AudioMessage != nil:
		ctxInfo = e.Message.AudioMessage.GetContextInfo()

	case e.Message.StickerMessage != nil:
		ctxInfo = e.Message.StickerMessage.GetContextInfo()
	}

	// ✅ Handle replies
	if ctxInfo != nil && ctxInfo.QuotedMessage != nil && ctxInfo.StanzaID != nil && *ctxInfo.StanzaID != "" {
		var replyText string
		waReplyPublicURL := config.GetConfig().Whatsmeow.WAReplyPublicURL + "/" + time.Now().Format("2006-01-02")

		switch {

		case e.Message.Conversation != nil:
			replyText = *e.Message.Conversation

		case e.Message.ExtendedTextMessage != nil && e.Message.ExtendedTextMessage.Text != nil:
			replyText = *e.Message.ExtendedTextMessage.Text

		case e.Message.ImageMessage != nil:
			msg := e.Message.ImageMessage
			data, err := WhatsappClient.Download(context.Background(), msg)
			if err != nil {
				logrus.Error("Failed to download image:", err)
				break
			}
			mimeType := getSafeString(msg.Mimetype)
			ext := getFileExtension(mimeType)
			filename := fmt.Sprintf("img_%d%s", time.Now().UnixNano(), ext)
			savePath := filepath.Join(uploadDir, filename)
			os.WriteFile(savePath, data, 0644)
			publicURL := fmt.Sprintf("%s/%s", waReplyPublicURL, filename)
			caption := getSafeString(msg.Caption)
			replyText = fmt.Sprintf("📷 %s %s", caption, publicURL)

		case e.Message.VideoMessage != nil:
			msg := e.Message.VideoMessage
			data, err := WhatsappClient.Download(context.Background(), msg)
			if err != nil {
				logrus.Info("Failed to download video:", err)
				break
			}
			mimeType := getSafeString(msg.Mimetype)
			ext := getFileExtension(mimeType)
			filename := fmt.Sprintf("vid_%d%s", time.Now().UnixNano(), ext)
			savePath := filepath.Join(uploadDir, filename)
			os.WriteFile(savePath, data, 0644)
			publicURL := fmt.Sprintf("%s/%s", waReplyPublicURL, filename)
			caption := getSafeString(msg.Caption)
			replyText = fmt.Sprintf("🎥 %s %s", caption, publicURL)

		case e.Message.AudioMessage != nil:
			msg := e.Message.AudioMessage
			data, err := WhatsappClient.Download(context.Background(), msg)
			if err != nil {
				logrus.Error("Failed to download audio:", err)
				break
			}
			mimeType := getSafeString(msg.Mimetype)
			ext := getFileExtension(mimeType)
			filename := fmt.Sprintf("aud_%d%s", time.Now().UnixNano(), ext)
			savePath := filepath.Join(uploadDir, filename)
			os.WriteFile(savePath, data, 0644)
			publicURL := fmt.Sprintf("%s/%s", waReplyPublicURL, filename)
			replyText = fmt.Sprintf("🎧 Audio message: %s", publicURL)

		case e.Message.DocumentMessage != nil:
			msg := e.Message.DocumentMessage
			data, err := WhatsappClient.Download(context.Background(), msg)
			if err != nil {
				logrus.Error("Failed to download document:", err)
				break
			}
			mimeType := getSafeString(msg.Mimetype)
			ext := getFileExtension(mimeType)
			filename := fmt.Sprintf("doc_%d%s", time.Now().UnixNano(), ext)
			savePath := filepath.Join(uploadDir, filename)
			os.WriteFile(savePath, data, 0644)
			publicURL := fmt.Sprintf("%s/%s", waReplyPublicURL, filename)
			caption := getSafeString(msg.Caption)
			replyText = fmt.Sprintf("📄 %s %s", caption, publicURL)

		case e.Message.StickerMessage != nil:
			msg := e.Message.StickerMessage
			data, err := WhatsappClient.Download(context.Background(), msg)
			if err != nil {
				logrus.Error("Failed to download sticker:", err)
				break
			}
			mimeType := getSafeString(msg.Mimetype)
			ext := getFileExtension(mimeType)
			filename := fmt.Sprintf("stk_%d%s", time.Now().UnixNano(), ext)
			savePath := filepath.Join(uploadDir, filename)
			os.WriteFile(savePath, data, 0644)
			publicURL := fmt.Sprintf("%s/%s", waReplyPublicURL, filename)
			replyText = fmt.Sprintf("🖼️ Sticker: %s", publicURL)

		default:
			replyText = "(non-text or unknown reply)"
		}

		stanzaID := *ctxInfo.StanzaID
		stanzaID = strings.TrimSpace(stanzaID)

		var spMessage sptechnicianmodel.SPWhatsAppMessage
		result := dbWeb.Where("whatsapp_chat_id = ? AND for_project = ? AND what_sp = ?", stanzaID, forProject, whatSP).First(&spMessage)
		if result.Error != nil {
			if result.Error == gorm.ErrRecordNotFound {
				// No matching record found, skip processing
				return
			}
			logrus.Errorf("Database error while fetching SPWhatsAppMessage for whatsapp_chat_id '%s', project '%s', type '%s': %v", stanzaID, forProject, whatSP, result.Error)
			return
		}

		// We found the message, now update it
		t := time.Now()
		spMessage.WhatsappRepliedBy = e.Info.Sender.String()
		spMessage.WhatsappRepliedAt = &t
		spMessage.WhatsappReplyText = replyText

		if err := dbWeb.Save(&spMessage).Error; err != nil {
			logrus.Errorf("Failed to update SP reply info for message ID %d: %v", spMessage.ID, err)
			return
		}

		// For the notification, we need details from the parent SACGotSP record
		var dbData sptechnicianmodel.SACGotSP
		if spMessage.SACGotSPID != nil {
			if err := dbWeb.First(&dbData, *spMessage.SACGotSPID).Error; err == nil {
				sentAt := "N/A"
				if spMessage.WhatsappSentAt != nil {
					sentAt = spMessage.WhatsappSentAt.Format("02 Jan 2006 15:04")
				}

				var userReplyData model.WAPhoneUser
				var userReplied string
				if err := dbWeb.Model(&model.WAPhoneUser{}).
					Where("phone_number = ?", e.Info.Sender.User).
					First(&userReplyData).Error; err != nil {
					logrus.Errorf("Failed to find WAPhoneUser for phone number %s: %v", e.Info.Sender.User, err)
					userReplied = e.Info.Sender.String()
				} else {
					userReplied = fmt.Sprintf("_%s_ (%s)", userReplyData.FullName, userReplyData.PhoneNumber)
				}

				// Send replied text to Whatsapp HRD
				idText := fmt.Sprintf("SP-%d untuk %s yang dikirim pada %v mendapatkan respon dari %s, yakni %s",
					spMessage.NumberOfSP,
					dbData.SAC,
					sentAt,
					userReplied,
					UnboldedLinkWAMsg(replyText),
				)
				enText := fmt.Sprintf("SP-%d for %s sent at %v received a reply from %s, which is %s",
					spMessage.NumberOfSP,
					dbData.SAC,
					sentAt,
					userReplied,
					UnboldedLinkWAMsg(replyText),
				)

				for _, hrd := range dataHRD {
					jid := fmt.Sprintf("%s@%s", hrd.PhoneNumber, "s.whatsapp.net")
					SendLangMessage(jid, idText, enText, "id")
				}

				// Send thank you reply to replier
				textID := fmt.Sprintf("🙏🏽 Terima kasih atas respon Anda untuk SP-%d.", spMessage.NumberOfSP)
				textEN := fmt.Sprintf("🙏🏽 Thank you for your reply to SP-%d.", spMessage.NumberOfSP)
				SendLangMessage(e.Info.Sender.String(), textID, textEN, "id")
			}
		}
	}

	// 🤖 Handle reactions
	if r := e.Message.GetReactionMessage(); r != nil {
		stanzaID := r.GetKey().GetID()
		stanzaID = strings.TrimSpace(stanzaID)

		var spMessage sptechnicianmodel.SPWhatsAppMessage
		if err := dbWeb.Where("whatsapp_chat_id = ? AND for_project = ? AND what_sp = ?", stanzaID, forProject, whatSP).First(&spMessage).Error; err != nil {
			logrus.Warnf("No SPWhatsAppMessage matched for reaction on whatsapp_chat_id '%s', project '%s', type '%s': %v", stanzaID, forProject, whatSP, err)
			return
		}

		t := time.Now()
		spMessage.WhatsappReactionEmoji = r.GetText()
		spMessage.WhatsappRepliedBy = e.Info.Sender.String() // Using RepliedBy for reactions as well
		spMessage.WhatsappReactedAt = &t

		if err := dbWeb.Save(&spMessage).Error; err != nil {
			logrus.Errorf("Failed to update SP reaction info for message ID %d: %v", spMessage.ID, err)
			return
		}
	}

	// ✏️ Edited Message
	if pm := e.Message.GetProtocolMessage(); pm != nil {
		if pm.GetType() == waE2E.ProtocolMessage_MESSAGE_EDIT {
			var replyText string
			var messageIDToUpdate string
			edited := pm.GetEditedMessage()

			// Case: edited reply (ExtendedTextMessage with context)
			if etm := edited.GetExtendedTextMessage(); etm != nil {
				replyText = etm.GetText()
				if ctx := etm.GetContextInfo(); ctx != nil && ctx.GetStanzaID() != "" {
					messageIDToUpdate = ctx.GetStanzaID()
				}
			}

			// Case: plain edited message (no reply, just a text change)
			if replyText == "" && edited.GetConversation() != "" {
				replyText = edited.GetConversation()
				if pm.GetKey() != nil && pm.GetKey().GetID() != "" {
					messageIDToUpdate = pm.GetKey().GetID()
				}
			}

			if messageIDToUpdate == "" {
				logrus.Println("❌ No message ID to update - skipping edit processing")
				return
			}

			var spMessage sptechnicianmodel.SPWhatsAppMessage
			if err := dbWeb.Where("whatsapp_chat_id = ? AND for_project = ? AND what_sp = ?", messageIDToUpdate, forProject, whatSP).First(&spMessage).Error; err != nil {
				logrus.Errorf("No SPWhatsAppMessage matched for edited message on whatsapp_chat_id '%s', project '%s', type '%s': %v", messageIDToUpdate, forProject, whatSP, err)
				return
			}

			t := time.Now()
			spMessage.WhatsappReplyText = replyText
			spMessage.WhatsappRepliedBy = e.Info.Sender.String()
			spMessage.WhatsappRepliedAt = &t // Using RepliedAt for edits

			if err := dbWeb.Save(&spMessage).Error; err != nil {
				logrus.Errorf("Failed to update SP edited message reply for message ID %d: %v", spMessage.ID, err)
				return
			}

			// For the notification, we need details from the parent SACGotSP record
			var dbData sptechnicianmodel.SACGotSP
			if spMessage.SACGotSPID != nil {
				if err := dbWeb.First(&dbData, *spMessage.SACGotSPID).Error; err == nil {
					sentAt := "N/A"
					if spMessage.WhatsappSentAt != nil {
						sentAt = spMessage.WhatsappSentAt.Format("02 Jan 2006 15:04")
					}

					var userReplyData model.WAPhoneUser
					var userReplied string
					if err := dbWeb.Model(&model.WAPhoneUser{}).
						Where("phone_number = ?", e.Info.Sender.User).
						First(&userReplyData).Error; err != nil {
						logrus.Errorf("Failed to find WAPhoneUser for phone number %s: %v", e.Info.Sender.User, err)
						userReplied = e.Info.Sender.String()
					} else {
						userReplied = fmt.Sprintf("_%s_ (%s)", userReplyData.FullName, userReplyData.PhoneNumber)
					}

					// Send edited text to Whatsapp HRD
					idText := fmt.Sprintf("SP-%d untuk %s yang dikirim pada %v mendapatkan pesan (yang diedit) dari %s, yakni %s",
						spMessage.NumberOfSP,
						dbData.SAC,
						sentAt,
						userReplied,
						UnboldedLinkWAMsg(replyText),
					)
					enText := fmt.Sprintf("SP-%d for %s sent at %v received an (edited) message from %s, which is %s",
						spMessage.NumberOfSP,
						dbData.SAC,
						sentAt,
						userReplied,
						UnboldedLinkWAMsg(replyText),
					)
					for _, hrd := range dataHRD {
						jid := fmt.Sprintf("%s@%s", hrd.PhoneNumber, "s.whatsapp.net")
						SendLangMessage(jid, idText, enText, "id")
					}
				}
			}
		}
	}
}

func handleReceiptForSPSAC(forProject string) {
	e := WhatsappEventsReceiptForSPSAC
	dbWeb := gormdb.Databases.Web
	whatSP := "SP_SAC"

	for _, msgID := range e.MessageIDs {
		if string(e.Type) != "" {
			var spMessage sptechnicianmodel.SPWhatsAppMessage
			if err := dbWeb.Where("whatsapp_chat_id = ? AND for_project = ? AND what_sp = ?", msgID, forProject, whatSP).First(&spMessage).Error; err != nil {
				// Skip logging to reduce noise for messages not related to SP
				continue
			}

			// Update the status and save
			spMessage.WhatsappMsgStatus = string(e.Type)
			if err := dbWeb.Save(&spMessage).Error; err != nil {
				logrus.Errorf("Failed to update receipt status for message ID %d: %v", spMessage.ID, err)
			}
		}
	}
}

func sendLangDocumentMessageForSPSAC(forProject, sac, jid, idmsg, enmsg, lang, filePath string, spNumber int, spSentTo string) {
	// Trace language of user is using now
	userLang, err := GetUserLang(jid)
	if err != nil {
		userLang = lang
	}
	lang = userLang

	var msg string
	switch lang {
	case "id":
		msg = idmsg
	case "en":
		msg = enmsg
	default:
		msg = idmsg // Default to Indonesian if language not recognized
	}
	sendDocumentViaBotForSPSAC(forProject, sac, jid, msg, filePath, spNumber, spSentTo)
}

func sendDocumentViaBotForSPSAC(forProject, sac, jid, message, filePath string, spNumber int, spSentTo string) {
	dbWeb := gormdb.Databases.Web

	parsedJID, err := types.ParseJID(jid)
	if err != nil {
		logrus.Errorf("Failed to parse JID: %v\n", err)
		return
	}

	// Remove device part if present
	userJID := types.JID{
		User:   parsedJID.User,
		Server: parsedJID.Server,
	}

	fileData, err := os.ReadFile(filePath)
	if err != nil {
		logrus.Errorf("Failed to read file %s: %v\n", filePath, err)
		return
	}

	fileInfo, err := os.Stat(filePath)
	if err != nil {
		logrus.Errorf("Failed to get file info for %s: %v\n", filePath, err)
		return
	}

	if fileInfo.Size() == 0 {
		logrus.Errorf("File %s is empty, cannot send message", filePath)
		return
	}

	// Detect MIME type from file data
	mimeType := http.DetectContentType(fileData)
	if mimeType == "" {
		// Fallback to extension-based detection
		ext := filepath.Ext(filePath)
		mimeType = mime.TypeByExtension(ext)
		if mimeType == "" {
			mimeType = "application/octet-stream" // Default fallback
		}
	}

	// Get filename from path
	fileName := filepath.Base(filePath)

	uploaded, err := WhatsappClient.Upload(
		context.Background(),
		fileData,
		whatsmeow.MediaDocument)
	if err != nil {
		logrus.Errorf("Failed to upload file %s: %v\n", filePath, err)
		return
	}
	if uploaded.URL == "" || uploaded.DirectPath == "" {
		logrus.Errorf("Upload response is missing URL or DirectPath for file %s", filePath)
		return
	}

	resp, err := WhatsappClient.SendMessage(context.Background(), userJID, &waE2E.Message{
		DocumentMessage: &waE2E.DocumentMessage{
			URL:           proto.String(uploaded.URL),
			DirectPath:    proto.String(uploaded.DirectPath),
			MediaKey:      uploaded.MediaKey,
			Mimetype:      proto.String(mimeType),
			FileEncSHA256: uploaded.FileEncSHA256,
			FileSHA256:    uploaded.FileSHA256,
			FileLength:    proto.Uint64(uint64(len(fileData))),
			FileName:      proto.String(fileName),
			Caption:       proto.String(message), // Your message becomes the caption
		},
	})

	if err != nil {
		logrus.Errorf("Failed to send message to %s: %v\n", userJID.String(), err)
		// Do not return here, as we still want to log attempt if needed,
		// but the resp object will be nil, so handle that below.
	}

	go func() {
		// 1. Find the parent SACGotSP record
		var dbData sptechnicianmodel.SACGotSP
		if err := dbWeb.
			Where("for_project = ? AND sac = ?", forProject, sac).
			First(&dbData).
			Error; err != nil {
			logrus.Errorf("Failed to find SACGotSP for %s/%s: %v", forProject, sac, err)
			return
		}

		whatSP := "SP_SAC"

		// 2. Create a new SPWhatsAppMessage record
		var respSentAt *time.Time
		if !resp.Timestamp.IsZero() {
			respSentAt = &resp.Timestamp
		} else {
			t := time.Now()
			respSentAt = &t
		}

		newSPMsg := sptechnicianmodel.SPWhatsAppMessage{
			SACGotSPID:            &dbData.ID,
			WhatSP:                whatSP,
			ForProject:            forProject,
			NumberOfSP:            spNumber,
			WhatsappMessageSentTo: spSentTo,
			WhatsappChatID:        resp.ID,
			WhatsappSentAt:        respSentAt,
			WhatsappChatJID:       userJID.String(),
			WhatsappSenderJID:     resp.Sender.String(),
			WhatsappMessageBody:   message,
			WhatsappMessageType:   "document",
			WhatsappIsGroup:       userJID.Server == "g.us",
			WhatsappMsgStatus:     "sent",
		}

		// 3. Save the new msg record to the database
		if err := dbWeb.Create(&newSPMsg).Error; err != nil {
			logrus.Errorf("Failed to create SPWhatsAppMessage record for %s/%s: %v", forProject, sac, err)
			return
		}

		logrus.Infof("Sucessfully saved SP %d WhatsApp message (that sent to %s) record for %s/%s", spNumber, spSentTo, forProject, sac)
	}()
}
