package controllers

import (
	"context"
	"errors"
	"fmt"
	whatsappmodel "service-platform/cmd/web_panel/model/whatsapp_model"
	"time"

	"github.com/sirupsen/logrus"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/types"
	"gorm.io/gorm"
)

// SyncWhatsappContactsToDatabase syncs real WhatsApp contacts to database
func SyncWhatsappContactsToDatabase(client *whatsmeow.Client, userID uint, db *gorm.DB) error {
	if client == nil || !client.IsConnected() {
		return errors.New("WhatsApp client not connected")
	}

	logrus.Infof("Starting contact sync for user %d", userID)

	// Get all contacts from WhatsApp Store
	contacts, err := client.Store.Contacts.GetAllContacts(context.Background())
	if err != nil {
		logrus.WithError(err).Error("Failed to get contacts from WhatsApp store")
		return err
	}

	logrus.Infof("Found %d contacts in WhatsApp store for user %d", len(contacts), userID)

	// Sync each contact to database
	var syncedCount int
	for jid, contact := range contacts {
		// Skip if this is the user's own number
		if jid.User == client.Store.ID.User {
			continue
		}

		// Normalize JID to ensure consistency
		normalizedJID := normalizeJID(jid.String())

		// Create or update contact in database
		waContact := whatsappmodel.WAContactInfo{
			UserID:      userID,
			ContactJID:  normalizedJID,
			ContactName: contact.FullName,
			PhoneNumber: formatPhoneDisplay(jid.User),
			PushName:    contact.PushName,
			IsOnline:    false, // We'll update this later from presence
		}

		// Use GORM's FirstOrCreate to avoid duplicates
		var existingContact whatsappmodel.WAContactInfo
		result := db.Where("user_id = ? AND contact_jid = ?", userID, normalizedJID).FirstOrCreate(&existingContact, waContact)

		if result.Error != nil {
			logrus.WithError(result.Error).Errorf("Failed to sync contact %s", normalizedJID)
			continue
		}

		// Update existing contact with latest info
		if result.RowsAffected == 0 {
			updates := map[string]interface{}{
				"contact_name": contact.FullName,
				"push_name":    contact.PushName,
				"updated_at":   time.Now(),
			}
			db.Model(&existingContact).Updates(updates)
		}

		syncedCount++
	}

	logrus.Infof("Successfully synced %d contacts for user %d", syncedCount, userID)

	// Create conversations for contacts that don't have them yet
	err = createConversationsForContacts(userID, db)
	if err != nil {
		logrus.WithError(err).Error("Failed to create conversations for contacts")
	}

	return nil
}

// createConversationsForContacts creates conversation records for contacts without them
func createConversationsForContacts(userID uint, db *gorm.DB) error {
	// Get all contacts that don't have conversations yet
	var contacts []whatsappmodel.WAContactInfo
	err := db.Where(`user_id = ? AND contact_jid NOT IN (
		SELECT DISTINCT contact_jid FROM wa_conversations 
		WHERE user_id = ? AND deleted_at IS NULL
	)`, userID, userID).Find(&contacts).Error

	if err != nil {
		return err
	}

	logrus.Infof("Creating conversations for %d contacts without existing conversations", len(contacts))

	// Create conversation for each contact
	for _, contact := range contacts {
		conversation := whatsappmodel.WAConversation{
			UserID:       userID,
			ContactJID:   contact.ContactJID,
			ContactName:  contact.GetDisplayName(),
			ContactPhone: contact.PhoneNumber,
			IsGroup:      false, // Individual contacts are not groups
			UnreadCount:  0,
		}

		result := db.Create(&conversation)
		if result.Error != nil {
			logrus.WithError(result.Error).Errorf("Failed to create conversation for contact %s", contact.ContactJID)
			continue
		}
	}

	return nil
}

// SyncWhatsappGroupsToDatabase syncs real WhatsApp groups to database
func SyncWhatsappGroupsToDatabase(client *whatsmeow.Client, userID uint, db *gorm.DB) error {
	if client == nil || !client.IsConnected() {
		return errors.New("WhatsApp client not connected")
	}

	// Get groups that the user is part of
	groups, err := client.GetJoinedGroups(context.Background())
	if err != nil {
		logrus.WithError(err).Error("Failed to get joined groups")
		return err
	}

	logrus.Infof("Found %d groups for user %d", len(groups), userID)

	for _, groupInfo := range groups {
		// Normalize group JID
		normalizedGroupJID := normalizeJID(groupInfo.JID.String())

		// Create or update group conversation
		groupConversation := whatsappmodel.WAConversation{
			UserID:       userID,
			ContactJID:   normalizedGroupJID,
			ContactName:  "",
			IsGroup:      true,
			GroupSubject: groupInfo.Name,
			UnreadCount:  0,
		}

		var existingConv whatsappmodel.WAConversation
		result := db.Where("user_id = ? AND contact_jid = ?", userID, normalizedGroupJID).FirstOrCreate(&existingConv, groupConversation)

		if result.Error != nil {
			logrus.WithError(result.Error).Errorf("Failed to sync group %s", normalizedGroupJID)
			continue
		}

		// Update group subject if it changed
		if result.RowsAffected == 0 && existingConv.GroupSubject != groupInfo.Name {
			db.Model(&existingConv).Update("group_subject", groupInfo.Name)
		}

		// Sync group participants
		err = syncGroupParticipants(client, userID, groupInfo.JID, existingConv.ID, db)
		if err != nil {
			logrus.WithError(err).Errorf("Failed to sync participants for group %s", normalizedGroupJID)
		}
	}

	return nil
} // syncGroupParticipants syncs group participants to database
func syncGroupParticipants(client *whatsmeow.Client, userID uint, groupJID types.JID, conversationID uint, db *gorm.DB) error {
	// Get group info with participants
	groupInfo, err := client.GetGroupInfo(context.Background(), groupJID)
	if err != nil {
		return err
	}

	// Clear existing participants
	db.Where("conversation_id = ?", conversationID).Delete(&whatsappmodel.WAGroupParticipant{})

	// Add current participants
	for _, participant := range groupInfo.Participants {
		// Determine participant role
		role := "member"
		if participant.IsSuperAdmin {
			role = "superadmin"
		} else if participant.IsAdmin {
			role = "admin"
		}

		groupParticipant := whatsappmodel.WAGroupParticipant{
			ConversationID:  conversationID,
			UserID:          userID,
			ParticipantJID:  participant.JID.String(),
			ParticipantRole: role,
			IsActive:        true,
			JoinedAt:        time.Now(), // We don't have exact join time from API
		}

		result := db.Create(&groupParticipant)
		if result.Error != nil {
			logrus.WithError(result.Error).Errorf("Failed to add group participant %s", participant.JID.String())
		}
	}

	return nil
}

// RefreshWhatsappContactsForUser refreshes contacts for a specific user
func RefreshWhatsappContactsForUser(userID uint, db *gorm.DB) error {
	manager := GetUserClientManager()
	client, exists := manager.GetClient(userID)
	if !exists {
		return fmt.Errorf("WhatsApp client not found for user %d", userID)
	}

	if !client.IsConnected || !client.IsAuthenticated {
		return fmt.Errorf("WhatsApp client not connected or authenticated for user %d", userID)
	}

	// Get the whatsmeow client from user client
	whatsClient := client.Client
	if whatsClient == nil {
		return fmt.Errorf("WhatsApp client is nil for user %d", userID)
	}

	// Sync contacts and groups
	err := SyncWhatsappContactsToDatabase(whatsClient, userID, db)
	if err != nil {
		return fmt.Errorf("failed to sync contacts: %v", err)
	}

	err = SyncWhatsappGroupsToDatabase(whatsClient, userID, db)
	if err != nil {
		return fmt.Errorf("failed to sync groups: %v", err)
	}

	return nil
}
