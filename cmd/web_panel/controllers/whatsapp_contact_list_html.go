package controllers

import (
	"fmt"
	"net/http"
	whatsappmodel "service-platform/cmd/web_panel/model/whatsapp_model"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// GetWhatsappContactListHTML returns contact list as HTML with real data
func GetWhatsappContactListHTML(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		userIDStr := c.Param("userid")
		if userIDStr == "" {
			c.HTML(http.StatusBadRequest, "", `<li class="chat-contact-list-item"><div class="text-danger">User ID is required</div></li>`)
			return
		}

		userID, err := strconv.ParseUint(userIDStr, 10, 32)
		if err != nil {
			c.HTML(http.StatusBadRequest, "", `<li class="chat-contact-list-item"><div class="text-danger">Invalid user ID</div></li>`)
			return
		}

		// Get conversations for contact list
		var conversations []whatsappmodel.WAConversation
		err = db.Where("user_id = ? AND deleted_at IS NULL", userID).
			Order("is_pinned DESC, CASE WHEN last_message_time IS NULL THEN 1 ELSE 0 END, last_message_time DESC, updated_at DESC").
			Find(&conversations).Error

		if err != nil {
			c.HTML(http.StatusInternalServerError, "", `<li class="chat-contact-list-item"><div class="text-danger">Failed to load conversations</div></li>`)
			return
		}

		// Generate HTML for each conversation
		var html strings.Builder

		if len(conversations) == 0 {
			html.WriteString(`
				<li class="chat-contact-list-item">
					<div class="d-flex justify-content-center align-items-center p-3">
						<div class="text-center">
							<i class="bx bx-message-dots bx-lg text-muted mb-2"></i>
							<p class="text-muted mb-0">No conversations yet</p>
							<small class="text-muted">Start by sending a message</small>
						</div>
					</div>
				</li>
			`)
		}

		for _, conv := range conversations {
			// Calculate time ago
			var timeAgo string
			if conv.LastMessageTime != nil {
				diff := time.Since(*conv.LastMessageTime)
				if diff < time.Minute {
					timeAgo = "now"
				} else if diff < time.Hour {
					timeAgo = fmt.Sprintf("%dm", int(diff.Minutes()))
				} else if diff < 24*time.Hour {
					timeAgo = fmt.Sprintf("%dh", int(diff.Hours()))
				} else {
					timeAgo = conv.LastMessageTime.Format("01/02")
				}
			} else {
				timeAgo = ""
			}

			// Determine display name
			displayName := conv.GetDisplayName()
			if displayName == "" {
				if conv.IsGroup {
					displayName = "Group Chat"
				} else {
					// Extract phone number from JID
					if strings.Contains(conv.ContactJID, "@") {
						phone := strings.Split(conv.ContactJID, "@")[0]
						// Remove any :xx suffix (like :21)
						if strings.Contains(phone, ":") {
							phone = strings.Split(phone, ":")[0]
						}
						displayName = phone
					} else {
						displayName = "Unknown Contact"
					}
				}
			}

			// Format last message
			lastMessage := conv.LastMessage
			if len(lastMessage) > 50 {
				lastMessage = lastMessage[:47] + "..."
			}
			if lastMessage == "" {
				lastMessage = "No messages yet"
			}

			// Generate contact item HTML
			html.WriteString(fmt.Sprintf(`
				<li class="chat-contact-list-item" data-contact-id="%s">
					<div class="d-flex">
						<div class="flex-shrink-0 avatar %s">
							%s
						</div>
						<div class="chat-contact-info flex-grow-1 ms-3">
							<div class="d-flex justify-content-between align-items-center">
								<h6 class="chat-contact-name mb-0">%s</h6>
								<small class="text-muted chat-time">%s</small>
							</div>
							<div class="d-flex justify-content-between align-items-center">
								<small class="chat-contact-status text-muted">%s</small>
								%s
							</div>
						</div>
					</div>
				</li>
			`,
				conv.ContactJID,
				func() string {
					if conv.IsGroup {
						return "avatar-group"
					}
					return ""
				}(),
				func() string {
					if conv.IsGroup {
						return `<span class="avatar-initial rounded-circle bg-primary">G</span>`
					}
					return `<img src="/assets/img/avatars/default-avatar.jpeg" alt="Avatar" class="rounded-circle" />`
				}(),
				displayName,
				timeAgo,
				lastMessage,
				func() string {
					if conv.UnreadCount > 0 {
						return fmt.Sprintf(`<span class="badge bg-danger rounded-pill badge-notifications">%d</span>`, conv.UnreadCount)
					}
					return ""
				}(),
			))
		}

		// Return the HTML content directly
		c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(html.String()))
	}
}
