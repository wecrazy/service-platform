package controllers

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"service-platform/cmd/web_panel/fun"
	"service-platform/cmd/web_panel/model"
	"service-platform/internal/config"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types/events"
)

// Custom event handler for WhatsApp events
// This handler processes various WhatsApp events such as connection, disconnection,
// message receipts, incoming messages, reactions, and message edits.
// It manages file uploads, updates message statuses in the database, and logs relevant information.
// The handler also extracts context information from messages to handle replies and attachments.
func HandleWhatsappEvent(evt interface{}) {
	cwd, err := os.Getwd()
	if err != nil {
		logrus.Errorf("Failed to get working directory: %v", err)
	}

	baseDir := filepath.Join(cwd, "web", "file")
	if _, err := os.Stat(baseDir); os.IsNotExist(err) {
		logrus.Errorf("Directory does not exist: %s\n", baseDir)
	}

	today := time.Now().Format("2006-01-02")
	uploadDir := filepath.Join(baseDir, "wa_reply", today)
	if err := os.MkdirAll(uploadDir, os.ModePerm); err != nil {
		logrus.Errorf("Failed to create upload directory: %v", err)
	}

	// Handle events
	switch e := evt.(type) {
	case *events.Connected:
		jid := WhatsappClient.Store.ID.User
		logrus.Infof("✅ WhatsApp %v connected", jid)
		GetWhatsappGroup()
	case *events.Disconnected:
		var jid string
		if WhatsappClient != nil && WhatsappClient.Store != nil && WhatsappClient.Store.ID != nil {
			jid = WhatsappClient.Store.ID.User
		} else {
			jid = "(unknown)"
		}
		logrus.Warnf("❌ Whatsapp %v disconnected", jid)
		// maybe retry logic or alert
	case *events.LoggedOut:
		jid := WhatsappClient.Store.ID.User
		logrus.Warnf("🏃🏻 Whatsapp %v logged out", jid)
	case *events.Receipt:
		for _, msgID := range e.MessageIDs {
			if string(e.Type) != "" {
				if err := dbWeb.Model(&model.WAMessage{}).Where("id = ?", msgID).Update("Status", string(e.Type)).Error; err != nil {
					logrus.Errorf("got error while trying to update wa msg status in db: %v", err)
				}
			}
		}

		// SP Technician WhatsApp message receipt handling
		go func() {
			forProject := "ODOO MS"
			WhatsappEventsReceiptForSPTechnician = e
			handleReceiptForSPTechnician(forProject)
		}()

		// SP SPL WhatsApp message receipt handling
		go func() {
			forProject := "ODOO MS"
			WhatsappEventsReceiptForSPSPL = e
			handleReceiptForSPSPL(forProject)
		}()

		// SP SAC WhatsApp message receipt handling
		go func() {
			forProject := "ODOO MS"
			WhatsappEventsReceiptForSPSAC = e
			handleReceiptForSPSAC(forProject)
		}()

		// Contract Technician WhatsApp message receipt handling
		go func() {
			// forProject := "ODOO MS"
			WhatsappEventsReceiptForContractTechnician = e
			handleReceiptForContractTechnician()
		}()

		// Payslip Technician WhatsApp message receipt handling -> Technician EDC & ATM
		go func() {
			WhatsappEventsReceiptForPayslipTechnician = e
			handleReceiptForPayslipTechnician()
		}()

		// SP Technician Stock Opname
		go func() {
			WhatsappEventsReceiptForSPStockOpname = e
			handleReceiptForSPStockOpname()
		}()

	case *events.CallOffer:
		caller := e.CallCreator.String()
		callerPhoneNumber := strings.Split(caller, ":")[0]
		callerPhoneNumber = strings.ReplaceAll(callerPhoneNumber, "@s.whatsapp.net", "")
		var sanitizedPhoneNumber string
		sanitizedPhoneNumber, err := fun.SanitizePhoneNumber(callerPhoneNumber)
		if err != nil {
			logrus.Errorf("failed to get sanitized phone number: %v", err)
		}

		rejectCall := true
		var waUserData model.WAPhoneUser
		if err := dbWeb.Where("phone_number LIKE ?", "%"+sanitizedPhoneNumber+"%").First(&waUserData).Error; err != nil {
			logrus.Errorf("cannot parse wa user data: %v", err)
		}
		if waUserData.AllowedToCall {
			rejectCall = false
		}

		if rejectCall {
			logrus.Infof("Incoming call of %s from %s, try to rejecting...", e.From, callerPhoneNumber)
			if err := WhatsappClient.RejectCall(context.Background(), e.From, e.CallID); err != nil {
				logrus.Errorf("Failed RejectCall for %s: %v", callerPhoneNumber, err)
			}

			// Send message to user who called
			idText := fmt.Sprintf("📞❌ Maaf, nomor %s tidak diizinkan untuk melakukan panggilan ke WhatsApp ini.\n\nApabila ada kendala, Anda bisa menghubungi layanan bantuan teknis kami di nomor berikut: +%s. Terima kasih. 🙏",
				callerPhoneNumber, config.WebPanel.Get().Whatsmeow.WaTechnicalSupport)
			enText := fmt.Sprintf("📞❌ Sorry, the number %s is not allowed to make calls to this WhatsApp.\n\nIf you have any issues, you can contact our technical support service at the following number: +%s. Thank you. 🙏",
				callerPhoneNumber, config.WebPanel.Get().Whatsmeow.WaTechnicalSupport)
			jidStr := callerPhoneNumber + "@s.whatsapp.net"
			userLang, err := GetUserLang(jidStr)
			if err != nil {
				logrus.Errorf("failed to get user language: %v", err)
				userLang = "id" // default to Indonesian
			}
			SendLangMessage(jidStr, idText, enText, userLang)
		}

	case *events.Message:
		go ProcessMessage(e) // use goroutine to prevent blocking

		// SP Technician WhatsApp message event handling
		go func() {
			// Store the incoming WhatsApp message event for later handling
			// (e.g. replies, reactions, or edits) without blocking the main event loop.
			forProject := "ODOO MS"
			WhatsappEventsMsgForSPTechnician = e
			handleRepliesReactionsAndEditedMsgForSPTechnician(forProject)
		}()

		// SP SPL WhatsApp message event handling
		go func() {
			forProject := "ODOO MS"
			WhatsappEventsMsgForSPSPL = e
			handleRepliesReactionsAndEditedMsgForSPSPL(forProject)
		}()

		// SP SAC WhatsApp message event handling
		go func() {
			forProject := "ODOO MS"
			WhatsappEventsMsgForSPSAC = e
			handleRepliesReactionsAndEditedMsgForSPSAC(forProject)
		}()

		// Contract Technician WhatsApp message event handling
		go func() {
			// forProject := "ODOO MS"
			WhatsappEventsMsgForContractTechnician = e
			handleRepliesReactionsAndEditedMsgForContractTechnician()
		}()

		// Payslip Technician WhatsApp message event handling -> Technician EDC & ATM
		go func() {
			WhatsappEventMsgForPayslipTechnician = e
			handleRepliesReactionsAndEditedMsgForPayslipTechnician()
		}()

		// SP Technician Stock Opname

		go func() {
			WhatsappEventMsgForSPStockOpname = e
			handleRepliesReactionsAndEditedMsgForSPStockOpname()
		}()

		// // ==== [DEBUG] ==================================
		// fmt.Println("📥 New message event received")
		// spew.Dump(e.Message)
		LogIncomingWhatsAppMessage(e, uploadDir)
		// // ===============================================

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
			waReplyPublicURL := config.WebPanel.Get().Whatsmeow.WAReplyPublicURL + "/" + time.Now().Format("2006-01-02")

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

			err := dbWeb.Model(&model.WAMessage{}).
				Where("id = ?", *ctxInfo.StanzaID).
				Updates(map[string]interface{}{
					"RepliedBy": e.Info.Sender.String(),
					"RepliedAt": time.Now(),
					"ReplyText": replyText,
				}).Error

			if err != nil {
				logrus.Printf("Failed to update reply info: %v", err)
			}
		}

		// 🤖 Handle reactions
		// fmt.Printf("[DEBUG] Received Message with sub-messages: %+v\n", e.Message)
		if r := e.Message.GetReactionMessage(); r != nil {
			if err := dbWeb.Model(&model.WAMessage{}).
				Where("id = ?", r.GetKey().GetID()).
				Updates(map[string]interface{}{
					"ReactionEmoji": r.GetText(),
					"ReactedBy":     e.Info.Sender.String(),
					"ReactedAt":     time.Now(),
				}).Error; err != nil {
				logrus.Errorf("error while try to update reaction for wa msg: %v", err)
			}
		}

		// ✏️ Edited Message
		if pm := e.Message.GetProtocolMessage(); pm != nil {
			if pm.GetType() == waE2E.ProtocolMessage_MESSAGE_EDIT {
				var replyText string
				var repliedToMsgID string
				edited := pm.GetEditedMessage()

				// Case: edited reply (ExtendedTextMessage with context)
				if etm := edited.GetExtendedTextMessage(); etm != nil {
					replyText = etm.GetText()
					// fmt.Println("📝 Edited reply text:", replyText)

					// ✅ This is the message ID you originally replied to
					if ctx := etm.GetContextInfo(); ctx != nil && ctx.GetStanzaID() != "" {
						repliedToMsgID = ctx.GetStanzaID()
					} else {
						logrus.Println("❌ No ContextInfo or stanzaID in edited reply")
					}
				}

				// Case: plain edited message (no reply, just a text change)
				if replyText == "" && edited.GetConversation() != "" {
					replyText = edited.GetConversation()
					// Not a reply, just text coz whatsapp desktop didnt show the replied msg destination only edited reply
					logrus.Println("📝 Edited plain text (not a reply)")
				}

				// 💾 Update quoted message with edited reply
				if repliedToMsgID != "" && replyText != "" {
					err := dbWeb.Model(&model.WAMessage{}).
						Where("id = ?", repliedToMsgID).
						Updates(map[string]interface{}{
							"ReplyText": replyText,
							"RepliedAt": time.Now(),
							"RepliedBy": e.Info.Sender.String(),
						}).Error

					if err != nil {
						logrus.Errorf("Failed to update quoted message (reply edit): %v", err)
					}
				}
			}
		}

	}
}
