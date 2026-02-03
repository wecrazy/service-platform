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
	contracttechnicianmodel "service-platform/cmd/web_panel/model/contract_technician_model"
	"time"

	"github.com/sirupsen/logrus"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	"google.golang.org/protobuf/proto"
)

var (
	WhatsappEventsMsgForContractTechnician     *events.Message
	WhatsappEventsReceiptForContractTechnician *events.Receipt
)

func handleRepliesReactionsAndEditedMsgForContractTechnician() {
	e := WhatsappEventsMsgForContractTechnician
	dbWeb := gormdb.Databases.Web

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

	today := time.Now().Format("2006-01-02")
	uploadDir := filepath.Join(baseDir, "wa_reply", today)
	if err := os.MkdirAll(uploadDir, os.ModePerm); err != nil {
		logrus.Errorf("Failed to create upload directory: %v", err)
		return
	}

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

		var tech contracttechnicianmodel.ContractTechnicianODOO
		if err := dbWeb.
			Where(contracttechnicianmodel.ContractTechnicianODOO{WhatsappChatID: stanzaID}).
			First(&tech).Error; err != nil {
			// logrus.Errorf("No ContractTechnicianODOO matched for whatsapp id %s: %v", stanzaID, err)
			return
		}

		var updatedDBData contracttechnicianmodel.ContractTechnicianODOO
		var repliedAt *time.Time
		t := time.Now()
		repliedAt = &t

		updatedDBData = contracttechnicianmodel.ContractTechnicianODOO{
			WhatsappRepliedBy: e.Info.Sender.String(),
			WhatsappRepliedAt: repliedAt,
			WhatsappReplyText: replyText,
		}

		if err := dbWeb.Model(&contracttechnicianmodel.ContractTechnicianODOO{}).
			Where("whatsapp_chat_id = ?", stanzaID).
			Updates(updatedDBData).Error; err != nil {
			logrus.Errorf("Failed to update reply info: %v", err)
			return
		}
	}

	// 🤖 Handle reactions
	if r := e.Message.GetReactionMessage(); r != nil {
		stanzaID := r.GetKey().GetID()

		var tech contracttechnicianmodel.ContractTechnicianODOO
		if err := dbWeb.
			Where(contracttechnicianmodel.ContractTechnicianODOO{WhatsappChatID: stanzaID}).
			First(&tech).Error; err != nil {
			logrus.Errorf("No ContractTechnicianODOO matched for whatsapp id %s: %v", stanzaID, err)
			return
		}

		var updatedDBData contracttechnicianmodel.ContractTechnicianODOO
		var reactedAt *time.Time
		t := time.Now()
		reactedAt = &t

		updatedDBData = contracttechnicianmodel.ContractTechnicianODOO{
			WhatsappReactionEmoji: r.GetText(),
			WhatsappReactedBy:     e.Info.Sender.String(),
			WhatsappReactedAt:     reactedAt,
		}

		if err := dbWeb.Model(&contracttechnicianmodel.ContractTechnicianODOO{}).
			Where(contracttechnicianmodel.ContractTechnicianODOO{WhatsappChatID: stanzaID}).
			Updates(updatedDBData).Error; err != nil {
			logrus.Errorf("Failed to update reaction info: %v", err)
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
				// logrus.Println("📝 Edited reply text:", replyText)

				// ✅ This is the message ID you originally replied to
				if ctx := etm.GetContextInfo(); ctx != nil && ctx.GetStanzaID() != "" {
					messageIDToUpdate = ctx.GetStanzaID()
					logrus.Println("📝 Edited reply - using replied-to message ID:", messageIDToUpdate)
				} else {
					logrus.Println("❌ No ContextInfo or stanzaID in edited reply")
				}
			}

			// Case: plain edited message (no reply, just a text change)
			if replyText == "" && edited.GetConversation() != "" {
				replyText = edited.GetConversation()
				// For plain text edits, we need to use the original message ID (the one being edited)
				// This should be available from the protocol message key
				if pm.GetKey() != nil && pm.GetKey().GetID() != "" {
					messageIDToUpdate = pm.GetKey().GetID()
					logrus.Println("📝 Edited plain text - using original message ID:", messageIDToUpdate)
				} else {
					logrus.Println("❌ No message ID found in protocol message key for plain text edit")
				}
			}

			// 💾 Update quoted message with edited reply
			var tech contracttechnicianmodel.ContractTechnicianODOO

			if messageIDToUpdate == "" {
				logrus.Println("❌ No message ID to update - skipping edit processing")
				return
			}

			if err := dbWeb.
				Where(contracttechnicianmodel.ContractTechnicianODOO{WhatsappChatID: messageIDToUpdate}).
				First(&tech).Error; err != nil {
				logrus.Errorf("No ContractTechnicianODOO matched for whatsapp id %s: %v", messageIDToUpdate, err)
				return
			}

			var updatedDBData contracttechnicianmodel.ContractTechnicianODOO
			var repliedAt *time.Time
			t := time.Now()
			repliedAt = &t
			updatedDBData = contracttechnicianmodel.ContractTechnicianODOO{
				WhatsappReplyText: replyText,
				WhatsappRepliedBy: e.Info.Sender.String(),
				WhatsappRepliedAt: repliedAt,
			}
			if err := dbWeb.Model(&contracttechnicianmodel.ContractTechnicianODOO{}).
				Where(contracttechnicianmodel.ContractTechnicianODOO{WhatsappChatID: messageIDToUpdate}).
				Updates(updatedDBData).Error; err != nil {
				logrus.Errorf("Failed to update edited reply info: %v", err)
				return
			}
		}
	}
}

func handleReceiptForContractTechnician() {
	e := WhatsappEventsReceiptForContractTechnician
	dbWeb := gormdb.Databases.Web

	for _, msgID := range e.MessageIDs {
		if string(e.Type) != "" {
			var tech contracttechnicianmodel.ContractTechnicianODOO
			if err := dbWeb.
				Where(contracttechnicianmodel.ContractTechnicianODOO{WhatsappChatID: msgID}).
				First(&tech).Error; err != nil {
				// Skip if no record found
				// logrus.Infof("No ContractTechnicianODOO record found for whatsapp_chat_id %s: %v", msgID, err)
				continue
			}

			if err := dbWeb.Model(&contracttechnicianmodel.ContractTechnicianODOO{}).
				Where(contracttechnicianmodel.ContractTechnicianODOO{WhatsappChatID: msgID}).
				Update("whatsapp_msg_status", string(e.Type)).Error; err != nil {
				logrus.Errorf("Failed to update whatsapp_msg_status for technician %s: %v", tech.Technician, err)
				continue
			}
		}
	}
}

func sendLangDocumentMessageForContractTechnician(forProject, technician, jid, idmsg, enmsg, lang, filePath string) {
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
	sendDocumentViaBotForContractTechnician(forProject, technician, jid, msg, filePath)
}

func sendDocumentViaBotForContractTechnician(forProject, technician, jid, message, filePath string) {
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
	}

	go func() {
		var sentAt *time.Time
		if !resp.Timestamp.IsZero() {
			sentAt = &resp.Timestamp
		} else {
			t := time.Now()
			sentAt = &t
		}

		dbData := contracttechnicianmodel.ContractTechnicianODOO{
			WhatsappChatID:      resp.ID,
			WhatsappChatJID:     userJID.String(),
			WhatsappSenderJID:   resp.Sender.User + "@" + resp.Sender.Server,
			WhatsappMessageBody: message,
			WhatsappMessageType: "text",
			WhatsappIsGroup:     userJID.Server == "g.us",
			WhatsappMsgStatus:   "sent",
			WhatsappSentAt:      sentAt,

			IsContractSent: true,
			ContractSendAt: sentAt,
		}

		if err := dbWeb.
			Where(contracttechnicianmodel.ContractTechnicianODOO{ForProject: forProject}).
			Where(contracttechnicianmodel.ContractTechnicianODOO{Technician: technician}).
			Updates(&dbData).
			Error; err != nil {
			logrus.Errorf("Failed to update whatsapp message data for technician %s: %v", technician, err)
			return
		}
	}()
}
