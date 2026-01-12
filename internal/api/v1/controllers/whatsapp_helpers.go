package controllers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"service-platform/internal/api/v1/dto"
	"service-platform/internal/config"
	"service-platform/internal/core/model"
	whatsnyanmodel "service-platform/internal/core/model/whatsnyan_model"
	"service-platform/internal/pkg/fun"
	"service-platform/internal/whatsapp"
	pb "service-platform/proto"
	"strings"
	"time"

	"github.com/agnivade/levenshtein"
	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"github.com/sirupsen/logrus"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	"google.golang.org/protobuf/proto"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// LanguageTranslation: structure for multi-language text messages
type LanguageTranslation struct {
	LanguageCode string            // e.g. "en", "id"
	Texts        map[string]string // Langcode -> Text , e.g. "en" -> "Hello", "id" -> "Halo"
}

// FilePermissionRule: rules for file/document handling
type FilePermissionRule struct {
	AllowFunc         func(user *model.WAUsers) (bool, LanguageTranslation) // returns: (allowed, deny message)
	DenyMessage       LanguageTranslation
	MaxDailyQuota     int      // e.g. 10 files per day
	CooldownSeconds   int      // e.g. 30 sec cooldown between file uploads
	MaxFileSizeBytes  int64    // maximum file size in bytes (e.g. 10MB = 10*1024*1024)
	AllowedExtensions []string // allowed file extensions (e.g. []string{".pdf", ".jpg", ".png"})
	AllowedMimeTypes  []string // allowed MIME types (e.g. []string{"application/pdf", "image/jpeg"})
}

// FilePermissionResult represents the result of file permission check
type FilePermissionResult struct {
	Allowed      bool
	Message      LanguageTranslation // deny message or empty if allowed
	UsesLeft     int                 // how many times can still use today
	CooldownLeft int                 // seconds left before allowed again
	MaxFileSize  int64               // maximum allowed file size in bytes
}

// PromptPermissionResult represents the result of prompt permission check
type PromptPermissionResult struct {
	Allowed      bool
	Message      LanguageTranslation
	UsesLeft     int // how many times can still use today
	CooldownLeft int // seconds left before allowed again
}

// PromptPermissionRule: rules for prompt/command handling
type PromptPermissionRule struct {
	AllowFunc       func(user *model.WAUsers) (bool, LanguageTranslation) // returns: (allowed, deny message)
	DenyMessage     LanguageTranslation
	MaxDailyQuota   int // e.g. 5 times per day
	CooldownSeconds int // e.g. 10 sec cooldown
}

// DocumentFilterResult represents the result of document filtering
type DocumentFilterResult struct {
	Allowed      bool
	DocumentType string              // identified document type
	Reason       string              // reason for allow/deny
	Message      LanguageTranslation // message to user
}

// DocumentRule represents rules for specific document types
type DocumentRule struct {
	FilenamePrefixes []string                                                            // e.g., []string{"report pemasangan", "invoice"}
	FilenamePatterns []string                                                            // regex patterns for filename matching
	AllowedUserTypes []string                                                            // user types allowed to upload this document
	AllowedUserOf    []string                                                            // user organizations allowed
	RequiredPatterns []string                                                            // patterns that must exist in filename
	MonthPatterns    []string                                                            // month names to look for
	YearRequired     bool                                                                // whether year is required in filename
	Description      string                                                              // description of the document type
	ProcessFunc      func(v *events.Message, user *model.WAUsers, userLang string) error // function to process the uploaded file
}

// SaveWhatsAppMessage saves a WhatsApp message log to the database
func SaveWhatsAppMessage(db *gorm.DB, msg *whatsnyanmodel.WhatsAppMsg) error {
	return db.Create(msg).Error
}

// ValidateGroupJID validates and normalizes a WhatsApp group JID (ID), ensuring it is in the correct format,
// and checks if the bot is a participant of the group. It removes common suffixes, normalizes the input,
// and fetches group information using the WhatsappClient. If the group exists and the bot is a member,
// it returns the normalized JID; otherwise, it returns an error describing the issue.
//
// Parameters:
//   - groupJID: The group JID string to validate and normalize.
//
// Returns:
//   - types.JID: The normalized group JID if valid and the bot is a participant.
//   - error: An error if the JID is invalid, the group does not exist, or the bot is not a member.
func ValidateGroupJID(groupJID string) (types.JID, error) {
	// Normalize: remove spaces, lowercase
	groupJID = strings.TrimSpace(strings.ToLower(groupJID))

	// Check if it looks like a user JID
	if strings.Contains(groupJID, fmt.Sprintf("@%s", types.DefaultUserServer)) {
		return types.JID{}, fmt.Errorf("invalid group JID: looks like a user JID (ends in @%s)", types.DefaultUserServer)
	}

	// Remove any "@g.us" or "g.us" suffix if present
	groupJID = strings.TrimSuffix(groupJID, "@g.us")
	groupJID = strings.TrimSuffix(groupJID, "g.us")
	groupJID = strings.TrimSuffix(groupJID, "@") // Remove trailing @ if any

	if groupJID == "" {
		return types.JID{}, errors.New("group JID cannot be empty")
	}

	jid := types.NewJID(groupJID, "g.us")

	// Check if group exists and bot is a participant
	// Use a timeout to prevent hanging
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	groupInfoResp, err := whatsapp.Client.GetGroupInfo(ctx, &pb.GetGroupInfoRequest{
		GroupJid: jid.String(),
	})
	if err != nil {
		return types.JID{}, fmt.Errorf("failed to fetch group info: %v", err)
	}
	if !groupInfoResp.Success {
		return types.JID{}, fmt.Errorf("failed to fetch group info: %s", groupInfoResp.Message)
	}

	// Ensure the bot is part of the group
	meResp, err := whatsapp.Client.GetMe(ctx, &pb.GetMeRequest{})
	if err != nil {
		return types.JID{}, fmt.Errorf("failed to get bot info: %v", err)
	}
	if !meResp.Success {
		return types.JID{}, fmt.Errorf("failed to get bot info: %s", meResp.Message)
	}
	me := meResp.Jid

	isMember := false
	for _, p := range groupInfoResp.Participants {
		if p.Jid == me {
			isMember = true
			break
		}
	}

	if !isMember {
		botNumber := me
		// Check indonesian format
		if strings.HasPrefix(botNumber, config.GetConfig().Default.DialingCodeDefault) {
			botNumber = "+" + botNumber
		}
		return types.JID{}, fmt.Errorf("your bot number (%s) is not a member of group %s", botNumber, groupInfoResp.Name)
	}

	return jid, nil
}

// CheckValidWhatsappPhoneNumber sanitizes and validates a phone number to ensure it is registered on WhatsApp.
//
// Steps performed:
// - Checks if the phone number is provided and meets minimum length.
// - Sanitizes the number by removing invalid characters (via helper).
// - Constructs the WhatsApp JID and checks if it exists using WhatsAppClient.
//
// Parameters:
// - phoneNumber: Raw user input phone number (e.g., "08123456789").
// - client: The WhatsApp client to use for checking.
//
// Returns:
// - A sanitized version of the phone number without country code prefix (e.g., "8123456789").
// - An error if the number is empty, too short, invalid, or not registered on WhatsApp.
//
// This function ensures that outbound messages are not sent to non-WhatsApp users.
func CheckValidWhatsappPhoneNumber(phoneNumber string, client *whatsmeow.Client) (string, error) {
	if phoneNumber == "" {
		return "", errors.New("no phone number provided")
	}

	// Check minimum length
	if len(phoneNumber) < config.GetConfig().Default.MinLengthPhoneNumber {
		return "", fmt.Errorf("%s is too short, min. %d digits", phoneNumber, config.GetConfig().Default.MinLengthPhoneNumber)
	}

	// Sanitize the number (e.g., remove +, spaces, etc.)
	sanitizedPhone, err := fun.SanitizeIndonesiaPhoneNumber(phoneNumber)
	if err != nil {
		return "", fmt.Errorf("failed to sanitize phone number: %w", err)
	}

	// Append @types.DefaultUserServer for WhatsApp JID
	jid := config.GetConfig().Default.DialingCodeDefault + sanitizedPhone + "@" + types.DefaultUserServer

	// Check if the number is registered on WhatsApp
	ctx := context.Background()
	resp, err := client.IsOnWhatsApp(ctx, []string{jid})
	if err != nil {
		return "", fmt.Errorf("failed to check WhatsApp status: %w", err)
	}

	// No results returned
	if len(resp) == 0 {
		return "", errors.New("no WhatsApp result returned")
	}

	// Check contact status
	if !resp[0].IsIn {
		return "", fmt.Errorf("%s is not registered on WhatsApp", phoneNumber)
	}

	return sanitizedPhone, nil
}

// GetWhatsappGroup retrieves the list of WhatsApp groups that the client has joined using the WhatsappClient.
// It stores the group details and participants in the PostgreSQL database using the provided GORM DB instance.
// It performs an upsert operation: if the group exists, it updates the details; otherwise, it creates a new record.
// It also updates the participants list for each group, replacing existing entries to ensure accuracy.
func GetWhatsappGroup(db *gorm.DB) {
	if db == nil {
		logrus.Error("GetWhatsappGroup: database connection is nil")
		return
	}

	data, err := whatsapp.Client.GetJoinedGroups(context.Background(), &pb.GetJoinedGroupsRequest{})
	if err != nil {
		logrus.Errorf("GetWhatsappGroup: %v\n", err)
		return
	}

	for _, group := range data.Groups {
		// Map to WhatsAppGroup model
		groupModel := whatsnyanmodel.WhatsAppGroup{
			JID:               group.Jid,
			Name:              group.Name,
			OwnerJID:          group.OwnerJid,
			Topic:             group.Topic,
			TopicSetAt:        time.Unix(group.TopicSetAt, 0),
			TopicSetBy:        group.TopicSetBy,
			LinkedParentJID:   group.LinkedParentJid,
			IsDefaultSubGroup: group.IsDefaultSubGroup,
			IsParent:          group.IsParent,
		}

		// Upsert Group
		if err := db.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "jid"}},
			UpdateAll: true,
		}).Create(&groupModel).Error; err != nil {
			logrus.Errorf("Failed to upsert group %s: %v", group.Name, err)
			continue
		}

		// Update Participants
		// Transaction to ensure consistency
		err := db.Transaction(func(tx *gorm.DB) error {
			// Delete existing participants for this group
			if err := tx.Where("group_jid = ?", group.Jid).Delete(&whatsnyanmodel.WhatsAppGroupParticipant{}).Error; err != nil {
				return err
			}

			// Insert current participants
			if len(group.Participants) > 0 {
				participants := make([]whatsnyanmodel.WhatsAppGroupParticipant, len(group.Participants))
				for i, p := range group.Participants {
					participants[i] = whatsnyanmodel.WhatsAppGroupParticipant{
						GroupJID:     group.Jid,
						UserJID:      p.Jid,
						LID:          p.Lid,
						DisplayName:  p.DisplayName,
						IsAdmin:      p.IsAdmin,
						IsSuperAdmin: p.IsSuperAdmin,
						PhoneNumber:  p.PhoneNumber,
					}
				}
				if err := tx.Create(&participants).Error; err != nil {
					return err
				}
			}
			return nil
		})

		if err != nil {
			logrus.Errorf("Failed to update participants for group %s: %v", group.Name, err)
		}
	}

	logrus.Infof("Successfully synced %d WhatsApp groups to database", len(data.Groups))
}

// GetUserLang fetches a user's preferred language from Redis
// GetUserLang retrieves the language preference for a user identified by the given jid from Redis.
// It returns the language as a string and an error if the operation fails.
// If the language is not set for the user, it returns an empty string and a nil error.
func GetUserLang(jid string, rdb *redis.Client) (string, error) {
	val, err := rdb.Get(context.Background(), "user:lang:"+jid).Result()
	if err == redis.Nil {
		return "", nil // Not set yet
	}
	return val, err
}

// SetUserLang sets the language preference for a user identified by the given jid.
// The language is stored in Redis with a key formatted as "user:lang:<jid>" and an expiration of 24 hours.
// Returns an error if the operation fails.
func SetUserLang(jid, lang string, rdb *redis.Client) error {
	if rdb == nil {
		return errors.New("redis client is nil")
	}

	duration := config.GetConfig().Whatsnyan.LanguageExpiry
	if duration <= 0 {
		duration = 1 * 24 * 60 * 60 // default to 1 day
	}

	return rdb.Set(
		context.Background(),
		"user:lang:"+jid,
		lang,
		time.Duration(duration)*time.Second,
	).Err()
}

func NewLanguageMsgTranslation(code string) LanguageTranslation {
	return LanguageTranslation{
		LanguageCode: code,
		Texts:        make(map[string]string),
	}
}

// SendLangWhatsAppTextMsg sends a WhatsApp text message to a user in their preferred language.
// It attempts to retrieve the user's language preference from Redis using the provided jid.
// If a preference is set, it uses that language code; otherwise, it falls back to the provided langCode,
// and if that's not available, to the default language code from configuration (typically "id").
// It selects the appropriate text from the LanguageTranslation map based on the determined language code.
// If the language code is not found in the map, it tries the default language code.
// The message is sent using the WhatsApp client. If stanzaID and v are provided, the message is quoted.
// The sent message is logged to the database asynchronously.
//
// Parameters:
//   - jid: The WhatsApp JID of the recipient.
//   - stanzaID: The stanza ID for quoting (optional, can be empty).
//   - v: The original WhatsApp message event for quoting (optional, can be nil).
//   - lang: A LanguageTranslation struct containing texts in various languages (map[string]string).
//   - langCode: The fallback language code if user's preference is not set.
//   - client: The WhatsApp client used to send the message.
//   - rdb: The Redis client used to fetch user language preferences.
//   - db: The GORM database instance used to log sent messages.
func SendLangWhatsAppTextMsg(jid, stanzaID string, v *events.Message, lang LanguageTranslation, langCode string, client *whatsmeow.Client, rdb *redis.Client, db *gorm.DB) {
	if client == nil {
		logrus.Error("SendLangWhatsAppMsg: WhatsApp client is nil")
		return
	}

	if rdb == nil {
		logrus.Error("SendLangWhatsAppMsg: Redis client is nil")
		return
	}

	userLang, err := GetUserLang(jid, rdb)
	if err != nil {
		logrus.Errorf("SendLangWhatsAppMsg: failed to get user lang for %s: %v", jid, err)
		langCode = fun.DefaultLang
	} else {
		if userLang != "" {
			langCode = userLang
		}
	}

	parseJID, err := types.ParseJID(jid)
	if err != nil {
		logrus.Errorf("SendLangWhatsAppMsg: invalid JID %s: %v", jid, err)
		return
	}

	userJID := types.JID{
		User:   parseJID.User,
		Server: parseJID.Server,
	}

	if v != nil {
		// Automatically mark incoming messages as read to trigger blue ticks on the sender's side
		if !v.Info.IsFromMe {
			err := client.MarkRead(context.Background(), []types.MessageID{types.MessageID(v.Info.ID)}, time.Now(), v.Info.Chat, v.Info.Sender)
			if err != nil {
				logrus.Errorf("Failed to mark message as read: %v", err)
			}
		}
		// Set chat presence to composing e.g. typing...
		client.SendChatPresence(context.Background(), v.Info.Chat, types.ChatPresenceComposing, types.ChatPresenceMediaText)
	}

	defaultLangCode := fun.DefaultLang

	text, exists := lang.Texts[langCode]
	if !exists {
		text, exists = lang.Texts[defaultLangCode]
		if !exists {
			logrus.Errorf("SendLangWhatsAppMsg: no text available for lang %s or default '%s'. Available keys: %v", langCode, defaultLangCode, getMapKeys(lang.Texts))
			return
		}
	}

	var resp whatsmeow.SendResponse
	var waErr error
	isBotNumber := false

	// Try find the jid that will send to and the store ID
	jidNormalized := NormalizeSenderJID(jid)
	jidNormalized = strings.ReplaceAll(jidNormalized, fmt.Sprintf("@%s", types.DefaultUserServer), "")
	storeIDNormalized := client.Store.ID.String()
	storeIDNormalized = strings.ReplaceAll(storeIDNormalized, fmt.Sprintf("@%s", types.DefaultUserServer), "")
	parts := strings.SplitN(storeIDNormalized, ":", 2)
	storeIDNormalized = parts[0]

	if jidNormalized == storeIDNormalized {
		logrus.Warnf("SendLangWhatsAppMsg: jid %s is the bot's own number, skipping message send", jid)
		isBotNumber = true
	}

	if stanzaID != "" && v != nil {
		quotedMsg := &waE2E.ContextInfo{
			StanzaID:      &stanzaID,
			Participant:   &jid,
			QuotedMessage: v.Message,
		}

		if !isBotNumber {
			resp, waErr = client.SendMessage(context.Background(), v.Info.Chat, &waE2E.Message{
				ExtendedTextMessage: &waE2E.ExtendedTextMessage{
					Text:        &text,
					ContextInfo: quotedMsg,
				},
			})
		} else {
			logrus.Warnf("SendLangWhatsAppMsg: skipping quoted message send because jid is bot's own number")
		}
	} else {
		if !isBotNumber {
			resp, waErr = client.SendMessage(context.Background(), userJID, &waE2E.Message{
				Conversation: proto.String(text),
			})
		} else {
			logrus.Warnf("SendLangWhatsAppMsg: skipping message send because jid is bot's own number")
			return
		}
	}

	if waErr != nil {
		logrus.Errorf("SendLangWhatsAppMsg: failed to send message to %s: %v", jid, waErr)
		return
	}

	go func() {
		// Parse timestamp
		var sentAt *time.Time
		if !resp.Timestamp.IsZero() {
			sentAt = &resp.Timestamp
		}

		if sentAt == nil {
			now := time.Now()
			sentAt = &now
		}

		// Log sent message to DB
		msg := whatsnyanmodel.WhatsAppMsg{
			WhatsappChatID:        resp.ID,
			WhatsappChatJID:       userJID.String(),
			WhatsappSenderJID:     resp.Sender.User + "@" + resp.Sender.Server,
			WhatsappMessageBody:   text,
			WhatsappMessageType:   "text",
			WhatsappIsGroup:       userJID.Server == types.GroupServer,
			WhatsappMsgStatus:     "sent",
			WhatsappMessageSentTo: userJID.String(),
			WhatsappSentAt:        sentAt,
		}
		_ = SaveWhatsAppMessage(db, &msg)
	}()
}

// CheckWAPhoneNumberIsRegistered godoc
// @Summary      Check WhatsApp Registration
// @Description  Checks if a phone number is registered on WhatsApp
// @Tags         WhatsApp
// @Produce      json
// @Param        request query dto.CheckWARegisteredRequest true "Query Params"
// @Success      200    {object}  map[string]interface{}
// @Failure      400    {object}  dto.APIErrorResponse
// @Failure      500    {object}  dto.APIErrorResponse
// @Router       /check_wa [get]
func CheckWAPhoneNumberIsRegistered() gin.HandlerFunc {
	return func(c *gin.Context) {
		digitNoTelp := config.GetConfig().Default.MinLengthPhoneNumber

		var req dto.CheckWARegisteredRequest
		if err := c.ShouldBindQuery(&req); err != nil {
			fun.HandleAPIErrorSimple(c, http.StatusBadRequest, err.Error())
			return
		}

		phoneNumber := req.Phone
		if phoneNumber == "" {
			fun.HandleAPIErrorSimple(c, http.StatusBadRequest, "Phone number is required")
			return
		}

		if len(phoneNumber) < digitNoTelp {
			fun.HandleAPIErrorSimple(c, http.StatusBadRequest, "Phone number too short")
			return
		}

		sanitizedPhoneNumber, err := fun.SanitizeIndonesiaPhoneNumber(phoneNumber)
		if err != nil {
			fun.HandleAPIErrorSimple(c, http.StatusBadRequest, err.Error())
			return
		} else {
			sanitizedPhoneNumber = config.GetConfig().Default.DialingCodeDefault + sanitizedPhoneNumber
		}

		jid := sanitizedPhoneNumber + "@" + types.DefaultUserServer
		resp, err := whatsapp.Client.IsOnWhatsApp(context.Background(), &pb.IsOnWhatsAppRequest{
			PhoneNumbers: []string{jid},
		})
		if err != nil {
			logrus.Errorf("got error while trying to check number phone is being registered in whatsapp: %v", err)
			errStr := strings.ToLower(err.Error())

			switch {
			case strings.Contains(errStr, "disconnected"),
				strings.Contains(errStr, "blocked"),
				strings.Contains(errStr, "timeout"),
				strings.Contains(errStr, "rate limited"),
				strings.Contains(errStr, "network error"),
				strings.Contains(errStr, "authentication failed"),
				strings.Contains(errStr, "1006"),
				strings.Contains(errStr, "unexpected eof"),
				strings.Contains(errStr, "websocket not connected"):
				c.Status(http.StatusInternalServerError)
			default:
				c.Status(http.StatusBadRequest)
			}
			return
		}

		if len(resp.Results) > 0 {
			contact := resp.Results[0]
			if !contact.IsRegistered {
				c.Status(http.StatusBadRequest)
				return
			} else {
				c.Status(http.StatusOK)
				return
			}
		} else {
			c.Status(http.StatusBadRequest)
			return
		}

	}
}

// NormalizeSenderJID normalizes a WhatsApp sender JID to a standard format.
// It ensures that the JID is in the form of user@types.DefaultUserServer for individual users
// and returns group JIDs as-is.
func NormalizeSenderJID(jid string) string {
	// First parse the JID properly
	parsed, err := types.ParseJID(jid)
	if err != nil {
		// Fallback for invalid JIDs
		if strings.Contains(jid, "@") {
			return strings.Split(jid, "@")[0] + "@" + types.DefaultUserServer
		}

		return jid + "@" + types.DefaultUserServer
	}

	// For groups, return the group JID as-is
	if parsed.Server == types.GroupServer {
		return parsed.String()
	}

	// For users, ensure standard format
	return parsed.User + "@" + types.DefaultUserServer
}

// getMapKeys returns the keys of a string map for logging/debugging purposes
func getMapKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// ResolveJIDToUserPhone attempts to resolve a JID to an actual user phone number JID.
// It first tries to look up the user ID part in the WhatsmeowLIDMap table to get the actual phone number,
// regardless of whether the JID comes as @lid, @s.whatsapp.net, or @g.us format.
// If not found in LIDMap, returns the original JID.
//
// Parameters:
//   - jid: The WhatsApp JID to resolve (can be "phone@s.whatsapp.net", "lidnumber@lid", or group JID)
//   - db: The GORM database instance for LID lookups
//
// Returns:
//   - string: The resolved user JID or original JID if resolution fails
//   - string: The JID type ("user", "group", "lid_resolved", "unknown")
func ResolveJIDToUserPhone(jid string, db *gorm.DB) (string, string) {
	if db == nil {
		return jid, "unknown"
	}

	// Extract the user ID part (before the @)
	var userID string
	if strings.Contains(jid, "@") {
		parts := strings.Split(jid, "@")
		userID = parts[0]
	} else {
		userID = jid
	}

	// First, always try to find the user ID in the LIDMap table
	// This works for @lid, @s.whatsapp.net, @g.us formats
	var lidMap whatsnyanmodel.WhatsmeowLIDMap
	if err := db.Where("lid = ?", userID).First(&lidMap).Error; err == nil {
		// Found in LIDMap! Return the actual phone number with user server
		phoneNumber := fmt.Sprintf("%s@%s", lidMap.PN, types.DefaultUserServer)
		return phoneNumber, "lid_resolved"
	}

	// Not found in LIDMap, check the JID format to determine type

	// Check if it's a group JID (ends with @g.us)
	if strings.Contains(jid, "@g.us") || strings.Contains(jid, types.GroupServer) {
		return jid, "group"
	}

	// Check if it's already a user JID (phone@s.whatsapp.net)
	if strings.Contains(jid, "@s.whatsapp.net") || strings.Contains(jid, types.DefaultUserServer) {
		return jid, "user"
	}

	// Check if it looks like a LID format but not found in database
	if strings.Contains(jid, "@lid") {
		logrus.Warnf("ResolveJIDToUserPhone: LID %s not found in database", userID)
		return jid, "lid"
	}

	// Unknown format
	return jid, "unknown"
}

// HandleLanguageChange handles the language change request for a user.
// It sets the user's preferred language in Redis and sends a confirmation message
// in the selected language if supported, otherwise skips sending the message.
//
// Parameters:
//   - jid: The WhatsApp JID of the user.
//   - langCode: The language code to set (e.g., "en", "id", "ja", "trj").
//   - client: The WhatsApp client instance.
//   - rdb: The Redis client for storing language preferences.
//   - db: The database client for logging.
func HandleLanguageChange(jid string, langCode string, client *whatsmeow.Client, rdb *redis.Client, db *gorm.DB) {
	if rdb == nil {
		logrus.Warnf("Redis client is nil, cannot handle language change for JID: %s", jid)
		return
	}

	if db == nil {
		logrus.Warnf("DB client is nil, cannot handle language change for JID: %s", jid)
		return
	}

	if client == nil {
		logrus.Warnf("Whatsmeow client is nil, cannot handle language change for JID: %s", jid)
		return
	}

	// Resolve JID to actual user phone number if it's a LID
	resolvedJID, jidType := ResolveJIDToUserPhone(jid, db)

	// ADD: skip jidType == 'group' if you don't want to allow language change in groups

	if jidType == "unknown" {
		logrus.Warnf("HandleLanguageChange: JID %s is unknown format, skipping language change", jid)
		return
	}

	// Sanitize input: trim whitespace, convert to lowercase
	langCode = strings.TrimSpace(strings.ToLower(langCode))
	if langCode == "" {
		return
	}

	// Check if it's a supported language using the official list
	if !fun.IsSupportedLanguage(langCode) {
		// logrus.Warnf("Unsupported language code '%s' for JID: %s", langCode, jid)
		return
	}

	// Normalize the language code (e.g., "ja" -> "jp", "zh" -> "cn")
	normalizedLangCode := fun.NormalizeLanguageCode(langCode)

	if err := SetUserLang(resolvedJID, normalizedLangCode, rdb); err != nil {
		langMsg := make(map[string]string)
		langMsg[fun.LangID] = fmt.Sprintf("Gagal mengatur bahasa = %s, terjadi kesalahan: %v", normalizedLangCode, err)
		langMsg[fun.LangEN] = fmt.Sprintf("Failed to set language = %s, got error: %v", normalizedLangCode, err)
		langMsg[fun.LangES] = fmt.Sprintf("No se pudo establecer el idioma = %s, ocurrió un error: %v", normalizedLangCode, err)
		langMsg[fun.LangFR] = fmt.Sprintf("Impossible de définir la langue = %s, erreur : %v", normalizedLangCode, err)
		langMsg[fun.LangDE] = fmt.Sprintf("Sprache konnte nicht gesetzt werden = %s, Fehler: %v", normalizedLangCode, err)
		langMsg[fun.LangPT] = fmt.Sprintf("Não foi possível definir o idioma = %s, ocorreu um erro: %v", normalizedLangCode, err)
		langMsg[fun.LangRU] = fmt.Sprintf("Не удалось установить язык = %s, ошибка: %v", normalizedLangCode, err)
		langMsg[fun.LangJP] = fmt.Sprintf("言語を設定できませんでした = %s、エラー: %v", normalizedLangCode, err)
		langMsg[fun.LangCN] = fmt.Sprintf("无法设置语言 = %s，发生错误：%v", normalizedLangCode, err)
		langMsg[fun.LangAR] = fmt.Sprintf("فشل في تعيين اللغة = %s، حدث خطأ: %v", normalizedLangCode, err)

		lang := NewLanguageMsgTranslation(normalizedLangCode)
		lang.Texts = langMsg
		SendLangWhatsAppTextMsg(resolvedJID, "", nil, lang, normalizedLangCode, client, rdb, db)
		return
	}

	// Success: send confirmation message in all languages
	langMsg := make(map[string]string)
	langMsg[fun.LangID] = "🇮🇩 Bahasa telah diatur ke *BAHASA INDONESIA*"
	langMsg[fun.LangEN] = "🇺🇸 Language has been set to *ENGLISH*"
	langMsg[fun.LangES] = "🇪🇸 El idioma se ha configurado en *ESPAÑOL*"
	langMsg[fun.LangFR] = "🇫🇷 La langue a été définie sur *FRANÇAIS*"
	langMsg[fun.LangPT] = "🇵🇹 O idioma foi definido para *PORTUGUÊS*"
	langMsg[fun.LangDE] = "🇩🇪 Die Sprache wurde auf *DEUTSCH* eingestellt"
	langMsg[fun.LangRU] = "🇷🇺 Язык был установлен на *РУССКИЙ*"
	langMsg[fun.LangJP] = "🇯🇵 言語が *日本語* に設定されました"
	langMsg[fun.LangCN] = "🇨🇳 语言已设置为 *中文*"
	langMsg[fun.LangAR] = "🇸🇦 تم تعيين اللغة إلى *العربية*"

	lang := NewLanguageMsgTranslation(normalizedLangCode)
	lang.Texts = langMsg

	// Send confirmation message using the normalized language code
	SendLangWhatsAppTextMsg(resolvedJID, "", nil, lang, normalizedLangCode, client, rdb, db)
	logrus.Infof("✅ Language changed to '%s' for JID: %s (resolved from %s)", normalizedLangCode, resolvedJID, jid)
}

// ContainsJID checks whether a given JID exists in a list of group JID strings.
// It iterates over the provided groupList, converts each string to a types.JID
// with the server set to "g.us", and compares it to the target jid.
// Returns true if the jid is found in the list, otherwise returns false.
//
// Parameters:
//   - groupList: A slice of strings representing group JIDs (without the server part).
//   - jid: The types.JID to search for in the groupList.
//
// Returns:
//   - bool: true if the jid is present in the groupList, false otherwise.
func ContainsJID(groupList []string, jid types.JID) bool {
	for _, group := range groupList {
		groupJID := types.NewJID(group, "g.us") // Convert string to types.JID
		if groupJID == jid {
			return true
		}
	}
	return false
}

// ValidateUserToUseBotWhatsapp checks if a user with the given phone number is registered to use the WhatsApp bot service.
// It retrieves the user's preferred language from Redis and checks the database for the phone number.
// If the phone number is not registered, it prepares an error message in multiple languages
// and sets a Redis key to avoid spamming the user with repeated messages.
// If the phone number is registered, it returns the corresponding WAUsers record.
//
// Parameters:
//   - phoneNumber: The phone number to validate.
//   - jid: The WhatsApp JID of the user.
//   - isGroup: A boolean indicating if the context is a group (not used in this function).
//   - msgType: The type of message (not used in this function).
//   - rdb: The Redis client for caching.
//   - db: The GORM database instance for querying user records.
//
// Returns:
//   - *model.WAUsers: The WAUsers record if the phone number is registered.
//   - error: An error if the phone number is not registered or if any other issue occurs.
func ValidateUserToUseBotWhatsapp(phoneNumber, jid string, isGroup bool, msgType string, client *whatsmeow.Client, rdb *redis.Client, db *gorm.DB) (*model.WAUsers, map[string]string) {
	if rdb == nil {
		logrus.Error("ValidateUserToUseBotWhatsapp: redis client is nil")
		return nil, nil
	}

	if db == nil {
		logrus.Error("ValidateUserToUseBotWhatsapp: database connection is nil")
		return nil, nil
	}

	if client == nil {
		logrus.Error("ValidateUserToUseBotWhatsapp: WhatsApp client is nil")
		return nil, nil
	}

	errorInLanguages := make(map[string]string)

	var waUser model.WAUsers
	err := db.Where("phone_number = ?", phoneNumber).First(&waUser).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			errorInLanguages[fun.LangID] = fmt.Sprintf("Mohon maaf, nomor Anda belum terdaftar untuk menggunakan layanan ini. Silakan hubungi layanan bantuan teknis kami di +%s.", config.GetConfig().Whatsnyan.WATechnicalSupport)
			errorInLanguages[fun.LangEN] = fmt.Sprintf("Sorry, your number is not registered to use this service. Please contact our technical support at +%s.", config.GetConfig().Whatsnyan.WATechnicalSupport)
			errorInLanguages[fun.LangES] = fmt.Sprintf("Lo siento, su número no está registrado para usar este servicio. Por favor, contacte a nuestro soporte técnico en +%s.", config.GetConfig().Whatsnyan.WATechnicalSupport)
			errorInLanguages[fun.LangFR] = fmt.Sprintf("Désolé, votre numéro n'est pas enregistré pour utiliser ce service. Veuillez contacter notre support technique au +%s.", config.GetConfig().Whatsnyan.WATechnicalSupport)
			errorInLanguages[fun.LangDE] = fmt.Sprintf("Entschuldigung, Ihre Nummer ist nicht registriert, um diesen Dienst zu nutzen. Bitte kontaktieren Sie unseren technischen Support unter +%s.", config.GetConfig().Whatsnyan.WATechnicalSupport)
			errorInLanguages[fun.LangPT] = fmt.Sprintf("Desculpe, seu número não está registrado para usar este serviço. Por favor, entre em contato com nosso suporte técnico em +%s.", config.GetConfig().Whatsnyan.WATechnicalSupport)
			errorInLanguages[fun.LangRU] = fmt.Sprintf("Извините, ваш номер не зарегистрирован для использования этой услуги. Пожалуйста, свяжитесь с нашей технической поддержкой по телефону +%s.", config.GetConfig().Whatsnyan.WATechnicalSupport)
			errorInLanguages[fun.LangJP] = fmt.Sprintf("申し訳ありませんが、あなたの番号はこのサービスを利用するために登録されていません。+%s の技術サポートにお問い合わせください。", config.GetConfig().Whatsnyan.WATechnicalSupport)
			errorInLanguages[fun.LangCN] = fmt.Sprintf("抱歉，您的号码未注册使用此服务。请联系 +%s 的技术支持。", config.GetConfig().Whatsnyan.WATechnicalSupport)
			errorInLanguages[fun.LangAR] = fmt.Sprintf("عذرًا، رقمك غير مسجل لاستخدام هذه الخدمة. يرجى الاتصال بدعمنا الفني على +%s.", config.GetConfig().Whatsnyan.WATechnicalSupport)

			expireDuration := config.GetConfig().Whatsnyan.NotRegisteredPhoneExpiry
			if expireDuration <= 0 {
				expireDuration = 3600 // default to 1 hour
			}
			notRegisteredKey := "not_registered_phone_" + jid
			exists, err := rdb.Exists(context.Background(), notRegisteredKey).Result()
			if err != nil {
				logrus.Errorf("ValidateUserToUseBotWhatsapp: redis exists check failed for key %s: %v", notRegisteredKey, err)
			}

			if exists == 0 {
				rdb.Set(context.Background(), notRegisteredKey, "true", time.Duration(expireDuration)*time.Second)
				return nil, errorInLanguages
			} else {
				// Avoid spamming the user with the same error message
				return nil, nil
			}
		}

		errorInLanguages[fun.LangID] = fmt.Sprintf("Terjadi kesalahan saat mencoba untuk memperiksa nomor Anda. Detail error: %v", err)
		errorInLanguages[fun.LangEN] = fmt.Sprintf("An error occurred while trying to verify your number. Error details: %v", err)
		errorInLanguages[fun.LangES] = fmt.Sprintf("Se produjo un error al intentar verificar su número. Detalles del error: %v", err)
		errorInLanguages[fun.LangFR] = fmt.Sprintf("Une erreur s'est produite lors de la vérification de votre numéro. Détails de l'erreur : %v", err)
		errorInLanguages[fun.LangDE] = fmt.Sprintf("Beim Versuch, Ihre Nummer zu überprüfen, ist ein Fehler aufgetreten. Fehlerinformationen: %v", err)
		errorInLanguages[fun.LangPT] = fmt.Sprintf("Ocorreu um erro ao tentar verificar seu número. Detalhes do erro: %v", err)
		errorInLanguages[fun.LangRU] = fmt.Sprintf("Произошла ошибка при попытке проверить ваш номер. Подробности ошибки: %v", err)
		errorInLanguages[fun.LangJP] = fmt.Sprintf("番号を確認しようとしたときにエラーが発生しました。エラーの詳細：%v", err)
		errorInLanguages[fun.LangCN] = fmt.Sprintf("尝试验证您的号码时发生错误。错误详情：%v", err)
		errorInLanguages[fun.LangAR] = fmt.Sprintf("حدث خطأ أثناء محاولة التحقق من رقمك. تفاصيل الخطأ: %v", err)

		return nil, errorInLanguages
	}

	_, err = CheckValidWhatsappPhoneNumber(waUser.PhoneNumber, client)
	if err != nil {
		errorInLanguages[fun.LangID] = fmt.Sprintf("Nomor WhatsApp tidak valid: %v", err)
		errorInLanguages[fun.LangEN] = fmt.Sprintf("Invalid WhatsApp number: %v", err)
		errorInLanguages[fun.LangES] = fmt.Sprintf("Número de WhatsApp no válido: %v", err)
		errorInLanguages[fun.LangFR] = fmt.Sprintf("Numéro WhatsApp invalide : %v", err)
		errorInLanguages[fun.LangDE] = fmt.Sprintf("Ungültige WhatsApp-Nummer: %v", err)
		errorInLanguages[fun.LangPT] = fmt.Sprintf("Número do WhatsApp inválido: %v", err)
		errorInLanguages[fun.LangRU] = fmt.Sprintf("Недействительный номер WhatsApp: %v", err)
		errorInLanguages[fun.LangJP] = fmt.Sprintf("無効なWhatsApp番号：%v", err)
		errorInLanguages[fun.LangCN] = fmt.Sprintf("无效的WhatsApp号码：%v", err)
		errorInLanguages[fun.LangAR] = fmt.Sprintf("رقم واتساب غير صالح: %v", err)

		return nil, errorInLanguages
	}

	if waUser.IsBanned {
		errorInLanguages[fun.LangID] = fmt.Sprintf("⛔ Nomor Anda sudah diblokir untuk menggunakan layanan ini. Silakan hubungi layanan bantuan teknis kami di +%s untuk informasi lebih lanjut.", config.GetConfig().Whatsnyan.WATechnicalSupport)
		errorInLanguages[fun.LangEN] = fmt.Sprintf("⛔ Your number has been banned from using this service. Please contact our technical support at +%s for more information.", config.GetConfig().Whatsnyan.WATechnicalSupport)
		errorInLanguages[fun.LangES] = fmt.Sprintf("⛔ Su número ha sido bloqueado para usar este servicio. Por favor, contacte a nuestro soporte técnico en +%s para más información.", config.GetConfig().Whatsnyan.WATechnicalSupport)
		errorInLanguages[fun.LangFR] = fmt.Sprintf("⛔ Votre numéro a été bloqué pour utiliser ce service. Veuillez contacter notre support technique au +%s pour plus d'informations.", config.GetConfig().Whatsnyan.WATechnicalSupport)
		errorInLanguages[fun.LangDE] = fmt.Sprintf("⛔ Ihre Nummer wurde für die Nutzung dieses Dienstes gesperrt. Bitte kontaktieren Sie unseren technischen Support unter +%s für weitere Informationen.", config.GetConfig().Whatsnyan.WATechnicalSupport)
		errorInLanguages[fun.LangPT] = fmt.Sprintf("⛔ Seu número foi banido de usar este serviço. Por favor, entre em contato com nosso suporte técnico em +%s para mais informações.", config.GetConfig().Whatsnyan.WATechnicalSupport)
		errorInLanguages[fun.LangRU] = fmt.Sprintf("⛔ Ваш номер был заблокирован для использования этой услуги. Пожалуйста, свяжитесь с нашей технической поддержкой по телефону +%s для получения дополнительной информации.", config.GetConfig().Whatsnyan.WATechnicalSupport)
		errorInLanguages[fun.LangJP] = fmt.Sprintf("⛔ あなたの番号はこのサービスの利用を禁止されています。詳細については、+%s の技術サポートにお問い合わせください。", config.GetConfig().Whatsnyan.WATechnicalSupport)
		errorInLanguages[fun.LangCN] = fmt.Sprintf("⛔ 您的号码已被禁止使用此服务。请联系 +%s 的技术支持以获取更多信息。", config.GetConfig().Whatsnyan.WATechnicalSupport)
		errorInLanguages[fun.LangAR] = fmt.Sprintf("⛔ تم حظر رقمك من استخدام هذه الخدمة. يرجى الاتصال بدعمنا الفني على +%s لمزيد من المعلومات.", config.GetConfig().Whatsnyan.WATechnicalSupport)

		return nil, errorInLanguages
	}

	if !waUser.IsRegistered {
		errorInLanguages[fun.LangID] = fmt.Sprintf("Mohon maaf, nomor Anda belum terdaftar untuk menggunakan layanan ini. Silakan hubungi layanan bantuan teknis kami di +%s.", config.GetConfig().Whatsnyan.WATechnicalSupport)
		errorInLanguages[fun.LangEN] = fmt.Sprintf("Sorry, your number is not registered to use this service. Please contact our technical support at +%s.", config.GetConfig().Whatsnyan.WATechnicalSupport)
		errorInLanguages[fun.LangES] = fmt.Sprintf("Lo siento, su número no está registrado para usar este servicio. Por favor, contacte a nuestro soporte técnico en +%s.", config.GetConfig().Whatsnyan.WATechnicalSupport)
		errorInLanguages[fun.LangFR] = fmt.Sprintf("Désolé, votre numéro n'est pas enregistré pour utiliser ce service. Veuillez contacter notre support technique au +%s.", config.GetConfig().Whatsnyan.WATechnicalSupport)
		errorInLanguages[fun.LangDE] = fmt.Sprintf("Entschuldigung, Ihre Nummer ist nicht registriert, um diesen Dienst zu nutzen. Bitte kontaktieren Sie unseren technischen Support unter +%s.", config.GetConfig().Whatsnyan.WATechnicalSupport)
		errorInLanguages[fun.LangPT] = fmt.Sprintf("Desculpe, seu número não está registrado para usar este serviço. Por favor, entre em contato com nosso suporte técnico em +%s.", config.GetConfig().Whatsnyan.WATechnicalSupport)
		errorInLanguages[fun.LangRU] = fmt.Sprintf("Извините, ваш номер не зарегистрирован для использования этой услуги. Пожалуйста, свяжитесь с нашей технической поддержкой по телефону +%s.", config.GetConfig().Whatsnyan.WATechnicalSupport)
		errorInLanguages[fun.LangJP] = fmt.Sprintf("申し訳ありませんが、あなたの番号はこのサービスを利用するために登録されていません。+%s の技術サポートにお問い合わせください。", config.GetConfig().Whatsnyan.WATechnicalSupport)
		errorInLanguages[fun.LangCN] = fmt.Sprintf("抱歉，您的号码未注册使用此服务。请联系 +%s 的技术支持。", config.GetConfig().Whatsnyan.WATechnicalSupport)
		errorInLanguages[fun.LangAR] = fmt.Sprintf("عذرًا، رقمك غير مسجل لاستخدام هذه الخدمة. يرجى الاتصال بدعمنا الفني على +%s.", config.GetConfig().Whatsnyan.WATechnicalSupport)

		expireDuration := config.GetConfig().Whatsnyan.NotRegisteredPhoneExpiry
		if expireDuration <= 0 {
			expireDuration = 3600 // default to 1 hour
		}
		notRegisteredKey := "not_registered_phone_" + jid
		exists, err := rdb.Exists(context.Background(), notRegisteredKey).Result()
		if err != nil {
			logrus.Errorf("ValidateUserToUseBotWhatsapp: redis exists check failed for key %s: %v", notRegisteredKey, err)
		}

		if exists == 0 {
			rdb.Set(context.Background(), notRegisteredKey, "true", time.Duration(expireDuration)*time.Second)
			return nil, errorInLanguages
		} else {
			// Avoid spamming the user with the same error message
			return nil, nil
		}
	}

	switch waUser.AllowedChats {
	case model.DirectChat:
		if isGroup {
			errorInLanguages[fun.LangID] = "Maaf, Anda hanya diizinkan untuk menggunakan chat pribadi dengan bot ini."
			errorInLanguages[fun.LangEN] = "Sorry, you are only allowed to use direct chat with this bot."
			errorInLanguages[fun.LangES] = "Lo siento, solo se le permite usar el chat directo con este bot."
			errorInLanguages[fun.LangFR] = "Désolé, vous n'êtes autorisé à utiliser que le chat direct avec ce bot."
			errorInLanguages[fun.LangDE] = "Entschuldigung, Sie dürfen nur den Direktchat mit diesem Bot verwenden."
			errorInLanguages[fun.LangPT] = "Desculpe, você só tem permissão para usar o chat direto com este bot."
			errorInLanguages[fun.LangRU] = "Извините, вам разрешено использовать только прямой чат с этим ботом."
			errorInLanguages[fun.LangJP] = "申し訳ありませんが、このボットとの直接チャットのみが許可されています。"
			errorInLanguages[fun.LangCN] = "抱歉，您只能与此机器人进行直接聊天。"
			errorInLanguages[fun.LangAR] = "عذرًا، يُسمح لك فقط باستخدام الدردشة المباشرة مع هذا الروبوت."

			return nil, errorInLanguages
		}
	case model.GroupChat:
		// Allowed in groups only, no direct chat
		if !isGroup {
			errorInLanguages[fun.LangID] = "Maaf, Anda hanya diizinkan untuk menggunakan chat grup dengan bot ini."
			errorInLanguages[fun.LangEN] = "Sorry, you are only allowed to use group chat with this bot."
			errorInLanguages[fun.LangES] = "Lo siento, solo se le permite usar el chat grupal con este bot."
			errorInLanguages[fun.LangFR] = "Désolé, vous n'êtes autorisé à utiliser que le chat de groupe avec ce bot."
			errorInLanguages[fun.LangDE] = "Entschuldigung, Sie dürfen nur den Gruppenchat mit diesem Bot verwenden."
			errorInLanguages[fun.LangPT] = "Desculpe, você só tem permissão para usar o chat em grupo com este bot."
			errorInLanguages[fun.LangRU] = "Извините, вам разрешено использовать только групповой чат с этим ботом."
			errorInLanguages[fun.LangJP] = "申し訳ありませんが、このボットとのグループチャットのみが許可されています。"
			errorInLanguages[fun.LangCN] = "抱歉，您只能与此机器人进行群聊。"
			errorInLanguages[fun.LangAR] = "عذرًا، يُسمح لك فقط باستخدام الدردشة الجماعية مع هذا الروبوت."

			return nil, errorInLanguages
		}
	case model.BothChat:
		return &waUser, nil
	default:
		errorInLanguages[fun.LangID] = "Jenis obrolan yang diizinkan tidak valid."
		errorInLanguages[fun.LangEN] = "The allowed chat type is not valid."
		errorInLanguages[fun.LangES] = "El tipo de chat permitido no es válido."
		errorInLanguages[fun.LangFR] = "Le type de chat autorisé n'est pas valide."
		errorInLanguages[fun.LangDE] = "Der erlaubte Chat-Typ ist nicht gültig."
		errorInLanguages[fun.LangPT] = "O tipo de chat permitido não é válido."
		errorInLanguages[fun.LangRU] = "Разрешенный тип чата недействителен."
		errorInLanguages[fun.LangJP] = "許可されたチャットタイプが無効です。"
		errorInLanguages[fun.LangCN] = "允许的聊天类型无效。"
		errorInLanguages[fun.LangAR] = "نوع الدردشة المسموح به غير صالح."

		return nil, errorInLanguages
	}

	// Check message type restrictions
	var allowedMsgTypes []model.WAMessageType
	if len(waUser.AllowedTypes) > 0 {
		if err := json.Unmarshal(waUser.AllowedTypes, &allowedMsgTypes); err != nil {
			logrus.Errorf("ValidateUserToUseBotWhatsapp: failed to unmarshal allowed message types for user %s: %v", waUser.PhoneNumber, err)
			errorInLanguages[fun.LangID] = fmt.Sprintf("Terjadi kesalahan saat coba memeriksa jenis pesan yang diizinkan dari jenis %s: %v", msgType, err)
			errorInLanguages[fun.LangEN] = fmt.Sprintf("An error occurred while trying to verify allowed message types from type %s: %v", msgType, err)
			errorInLanguages[fun.LangES] = fmt.Sprintf("Se produjo un error al intentar verificar los tipos de mensajes permitidos del tipo %s: %v", msgType, err)
			errorInLanguages[fun.LangFR] = fmt.Sprintf("Une erreur s'est produite lors de la tentative de vérification des types de messages autorisés à partir du type %s : %v", msgType, err)
			errorInLanguages[fun.LangDE] = fmt.Sprintf("Beim Versuch, die zulässigen Nachrichtentypen vom Typ %s zu überprüfen, ist ein Fehler aufgetreten: %v", msgType, err)
			errorInLanguages[fun.LangPT] = fmt.Sprintf("Ocorreu um erro ao tentar verificar os tipos de mensagens permitidos do tipo %s: %v", msgType, err)
			errorInLanguages[fun.LangRU] = fmt.Sprintf("Произошла ошибка при попытке проверить разрешенные типы сообщений из типа %s: %v", msgType, err)
			errorInLanguages[fun.LangJP] = fmt.Sprintf("タイプ %s から許可されたメッセージタイプを確認しようとしたときにエラーが発生しました: %v", msgType, err)
			errorInLanguages[fun.LangCN] = fmt.Sprintf("尝试验证类型 %s 的允许消息类型时发生错误：%v", msgType, err)
			errorInLanguages[fun.LangAR] = fmt.Sprintf("حدث خطأ أثناء محاولة التحقق من أنواع الرسائل المسموح بها من النوع %s: %v", msgType, err)

			return nil, errorInLanguages
		}
	}

	allowedType := false
	for _, t := range allowedMsgTypes {
		if strings.EqualFold(string(t), msgType) {
			allowedType = true
			break
		}
	}
	if !allowedType && len(allowedMsgTypes) > 0 {
		errorInLanguages[fun.LangID] = fmt.Sprintf("Jenis pesan '%s' tidak diizinkan untuk digunakan.", msgType)
		errorInLanguages[fun.LangEN] = fmt.Sprintf("Message type '%s' is not allowed to be used.", msgType)
		errorInLanguages[fun.LangES] = fmt.Sprintf("El tipo de mensaje '%s' no está permitido para su uso.", msgType)
		errorInLanguages[fun.LangFR] = fmt.Sprintf("Le type de message '%s' n'est pas autorisé à être utilisé.", msgType)
		errorInLanguages[fun.LangDE] = fmt.Sprintf("Der Nachrichtentyp '%s' ist nicht zur Verwendung erlaubt.", msgType)
		errorInLanguages[fun.LangPT] = fmt.Sprintf("O tipo de mensagem '%s' não é permitido para uso.", msgType)
		errorInLanguages[fun.LangRU] = fmt.Sprintf("Тип сообщения '%s' не разрешен для использования.", msgType)
		errorInLanguages[fun.LangJP] = fmt.Sprintf("メッセージタイプ '%s' は使用できません。", msgType)
		errorInLanguages[fun.LangCN] = fmt.Sprintf("不允许使用消息类型 '%s'。", msgType)
		errorInLanguages[fun.LangAR] = fmt.Sprintf("نوع الرسالة '%s' غير مسموح به للاستخدام.", msgType)

		return nil, errorInLanguages
	}

	return &waUser, nil
}

// CheckAndNotifyQuotaLimit checks if a user has exceeded their daily message quota.
// If the user is over the quota, it sends a notification message in multiple languages
// informing them of the limit and when it will reset. The function uses Redis to track
// message counts and warnings to avoid spamming the user.
//
// Parameters:
//   - userID: The ID of the user.
//   - useBot: A boolean indicating if the bot service is being used.
//   - jid: The WhatsApp JID of the user.
//   - maxQuota: The maximum allowed messages per day.
//   - client: The WhatsApp client instance.
//   - rdb: The Redis client for tracking quotas.
//   - db: The database client for logging.
//
// Returns:
//   - bool: true if under quota and processing can continue, false if over quota.
//   - error: An error if any issues occur during processing.
func CheckAndNotifyQuotaLimit(userID uint, useBot bool, jid string, maxQuota int, client *whatsmeow.Client, rdb *redis.Client, db *gorm.DB) (bool, error) {
	if useBot {
		quotaMsgKey := fmt.Sprintf("wa_msg_quota:%d:%s", userID, time.Now().Format(config.DATE_YYYY_MM_DD))
		// Increment counter
		quotaMsgCount, err := rdb.Incr(context.Background(), quotaMsgKey).Result()
		if err != nil {
			logrus.Errorf("Failed to increment quota counter for user %d: %v", userID, err)
			// Fail-safe: block processing
			return false, err
		}

		duration := config.GetConfig().Whatsnyan.QuotaLimitExpiry
		if duration <= 0 {
			duration = 86400 // default to 24 hours
		}

		// On first increment, set expiry so it resets daily
		if quotaMsgCount == 1 {
			rdb.Expire(context.Background(), quotaMsgKey, time.Duration(duration)*time.Second)
		}

		// Over quota?
		if int(quotaMsgCount) > maxQuota {
			// Check if we've already warned today
			warnKey := fmt.Sprintf("quota_warned:%d:%s", userID, time.Now().Format(config.DATE_YYYY_MM_DD))
			isWarned, err := rdb.Exists(context.Background(), warnKey).Result()
			if err != nil {
				logrus.Errorf("Failed to check warnKey: %v", err)
				// fallback: warn anyway
				isWarned = 0
			}

			if isWarned == 0 {
				// Get TTL to tell user when quota resets
				ttl, err := rdb.TTL(context.Background(), quotaMsgKey).Result()
				if err != nil || ttl <= 0 {
					ttl = time.Duration(config.GetConfig().Whatsnyan.QuotaLimitExpiry) * time.Second
				}

				lang := NewLanguageMsgTranslation(fun.DefaultLang)
				langMessages := make(map[string]string)

				// Prepare messages in multiple languages
				langMessages[fun.LangID] = fmt.Sprintf("Anda telah mencapai batas kuota harian (%d) pesan.\nMohon tunggu *%s* hingga batas kuota direset.", maxQuota, fun.FormatDurationHumanReadable(ttl))
				langMessages[fun.LangEN] = fmt.Sprintf("You have reached your daily quota of %d messages.\nPlease wait *%s* until the quota resets.", maxQuota, fun.FormatDurationHumanReadable(ttl))
				langMessages[fun.LangES] = fmt.Sprintf("Ha alcanzado su cuota diaria de %d mensajes.\nPor favor, espere *%s* hasta que se restablezca la cuota.", maxQuota, fun.FormatDurationHumanReadable(ttl))
				langMessages[fun.LangFR] = fmt.Sprintf("Vous avez atteint votre quota quotidien de %d messages.\nVeuillez attendre *%s* jusqu'à ce que le quota se réinitialise.", maxQuota, fun.FormatDurationHumanReadable(ttl))
				langMessages[fun.LangDE] = fmt.Sprintf("Sie haben Ihr tägliches Kontingent von %d Nachrichten erreicht.\nBitte warten Sie *%s*, bis das Kontingent zurückgesetzt wird.", maxQuota, fun.FormatDurationHumanReadable(ttl))
				langMessages[fun.LangPT] = fmt.Sprintf("Você atingiu sua cota diária de %d mensagens.\nPor favor, aguarde *%s* até que a cota seja redefinida.", maxQuota, fun.FormatDurationHumanReadable(ttl))
				langMessages[fun.LangRU] = fmt.Sprintf("Вы достигли дневного лимита в %d сообщений.\nПожалуйста, подождите *%s*, пока квота не сбросится.", maxQuota, fun.FormatDurationHumanReadable(ttl))
				langMessages[fun.LangJP] = fmt.Sprintf("1日のメッセージクォータ (%d) に達しました。\nクォータがリセットされるまで *%s* お待ちください。", maxQuota, fun.FormatDurationHumanReadable(ttl))
				langMessages[fun.LangCN] = fmt.Sprintf("您已达到每日消息配额 (%d)。\n请等待 *%s* 直到配额重置。", maxQuota, fun.FormatDurationHumanReadable(ttl))
				langMessages[fun.LangAR] = fmt.Sprintf("لقد وصلت إلى الحد اليومي للحصة (%d) من الرسائل.\nيرجى الانتظار *%s* حتى يتم إعادة تعيين الحصة.", maxQuota, fun.FormatDurationHumanReadable(ttl))

				lang.Texts = langMessages

				// Send the message
				SendLangWhatsAppTextMsg(jid, "", nil, lang, fun.DefaultLang, client, rdb, db)

				// Mark as warned today (with same TTL as quota key)
				rdb.Set(context.Background(), warnKey, "true", ttl)
			}

			// Over quota: do not process further
			return false, nil
		}

		// Under quota: proceed
		return true, nil
	}

	return true, nil
}

func getDocumentRules() map[string]DocumentRule {
	return map[string]DocumentRule{
		"general_document": {
			Description: "General Document",
			AllowedUserTypes: []string{
				string(model.CommonUser),
				string(model.SuperUser),
				string(model.ClientUser),
				string(model.AdministratorUser),
			},
		},
		// ADD: specific document rules here e.g. "invoices", "reports", etc.
	}
}

func checkUserPermissionForDocument(user *model.WAUsers, rule DocumentRule) DocumentFilterResult {
	// Check user type
	allowedType := false
	if len(rule.AllowedUserTypes) == 0 {
		allowedType = true
	} else {
		for _, t := range rule.AllowedUserTypes {
			if string(user.UserType) == t {
				allowedType = true
				break
			}
		}
	}

	if !allowedType {
		return DocumentFilterResult{
			Allowed: false,
			Reason:  "User type not allowed",
			Message: LanguageTranslation{
				Texts: map[string]string{
					fun.LangID: "❌ Anda tidak memiliki izin untuk mengirim jenis dokumen ini.",
					fun.LangEN: "❌ You don't have permission to upload this type of document.",
					fun.LangES: "❌ No tiene permiso para subir este tipo de documento.",
					fun.LangFR: "❌ Vous n'avez pas la permission de télécharger ce type de document.",
					fun.LangDE: "❌ Sie haben keine Berechtigung zum Hochladen dieses Dokumenttyps.",
					fun.LangPT: "❌ Você não tem permissão para fazer upload deste tipo de documento.",
					fun.LangRU: "❌ У вас нет разрешения на загрузку этого типа документа.",
					fun.LangJP: "❌ このタイプのドキュメントをアップロードする権限がありません。",
					fun.LangCN: "❌ 您没有权限上传此类型的文档。",
					fun.LangAR: "❌ ليس لديك إذن لتحميل هذا النوع من المستندات.",
				},
			},
		}
	}

	// Check user organization
	allowedOrg := false
	if len(rule.AllowedUserOf) == 0 {
		allowedOrg = true
	} else {
		for _, o := range rule.AllowedUserOf {
			if string(user.UserOf) == o {
				allowedOrg = true
				break
			}
		}
	}

	if !allowedOrg {
		return DocumentFilterResult{
			Allowed: false,
			Reason:  "User organization not allowed",
			Message: LanguageTranslation{
				Texts: map[string]string{
					fun.LangID: "❌ Organisasi Anda tidak memiliki izin untuk mengirim jenis dokumen ini.",
					fun.LangEN: "❌ Your organization doesn't have permission to upload this type of document.",
					fun.LangES: "❌ Su organización no tiene permiso para subir este tipo de documento.",
					fun.LangFR: "❌ Votre organisation n'a pas la permission de télécharger ce type de document.",
					fun.LangDE: "❌ Ihre Organisation hat keine Berechtigung zum Hochladen dieses Dokumenttyps.",
					fun.LangPT: "❌ Sua organização não tem permissão para fazer upload deste tipo de documento.",
					fun.LangRU: "❌ Ваша организация не имеет разрешения на загрузку этого типа документа.",
					fun.LangJP: "❌ あなたの組織には、このタイプのドキュメントをアップロードする権限がありません。",
					fun.LangCN: "❌ 您的组织没有权限上传此类型的文档。",
					fun.LangAR: "❌ مؤسستك ليس لديها إذن لتحميل هذا النوع من المستندات.",
				},
			},
		}
	}

	return DocumentFilterResult{Allowed: true}
}

func validateDocumentPatterns(filename string, rule DocumentRule) struct {
	Valid   bool
	Reason  string
	Message LanguageTranslation
} {
	// Check required patterns
	for _, pattern := range rule.RequiredPatterns {
		if !strings.Contains(filename, pattern) {
			return struct {
				Valid   bool
				Reason  string
				Message LanguageTranslation
			}{
				Valid:  false,
				Reason: fmt.Sprintf("Missing required pattern: %s", pattern),
				Message: LanguageTranslation{
					Texts: map[string]string{
						fun.LangID: fmt.Sprintf("❌ Nama file harus mengandung: %s", pattern),
						fun.LangEN: fmt.Sprintf("❌ Filename must contain: %s", pattern),
						fun.LangES: fmt.Sprintf("❌ El nombre del archivo debe contener: %s", pattern),
						fun.LangFR: fmt.Sprintf("❌ Le nom du fichier doit contenir : %s", pattern),
						fun.LangDE: fmt.Sprintf("❌ Dateiname muss enthalten: %s", pattern),
						fun.LangPT: fmt.Sprintf("❌ O nome do arquivo deve conter: %s", pattern),
						fun.LangRU: fmt.Sprintf("❌ Имя файла должно содержать: %s", pattern),
						fun.LangJP: fmt.Sprintf("❌ ファイル名には以下を含める必要があります: %s", pattern),
						fun.LangCN: fmt.Sprintf("❌ 文件名必须包含: %s", pattern),
						fun.LangAR: fmt.Sprintf("❌ يجب أن يحتوي اسم الملف على: %s", pattern),
					},
				},
			}
		}
	}

	// Check year requirement
	if rule.YearRequired {
		// Simple check for 4 digits
		matched, _ := regexp.MatchString(`\d{4}`, filename)
		if !matched {
			return struct {
				Valid   bool
				Reason  string
				Message LanguageTranslation
			}{
				Valid:  false,
				Reason: "Missing year in filename",
				Message: LanguageTranslation{
					Texts: map[string]string{
						fun.LangID: "❌ Nama file harus mengandung tahun (YYYY).",
						fun.LangEN: "❌ Filename must contain year (YYYY).",
						fun.LangES: "❌ El nombre del archivo debe contener el año (YYYY).",
						fun.LangFR: "❌ Le nom du fichier doit contenir l'année (YYYY).",
						fun.LangDE: "❌ Dateiname muss Jahr enthalten (YYYY).",
						fun.LangPT: "❌ O nome do arquivo deve conter o ano (YYYY).",
						fun.LangRU: "❌ Имя файла должно содержать год (YYYY).",
						fun.LangJP: "❌ ファイル名には年 (YYYY) を含める必要があります。",
						fun.LangCN: "❌ 文件名必須包含年份 (YYYY)。",
						fun.LangAR: "❌ يجب أن يحتوي اسم الملف على السنة (YYYY).",
					},
				},
			}
		}
	}

	return struct {
		Valid   bool
		Reason  string
		Message LanguageTranslation
	}{Valid: true}
}

// SanitizeAndFilterDocument checks if the document is allowed based on content/type
func SanitizeAndFilterDocument(v *events.Message, user *model.WAUsers, userLang string) (bool, LanguageTranslation) {
	if v == nil || v.Message.DocumentMessage == nil || v.Message.DocumentMessage.FileName == nil {
		return false, LanguageTranslation{
			Texts: map[string]string{
				fun.LangID: "❌ Upload dokumen tidak valid.",
				fun.LangEN: "❌ Invalid document upload.",
				fun.LangES: "❌ Carga de documento inválida.",
				fun.LangFR: "❌ Téléchargement de document invalide.",
				fun.LangDE: "❌ Ungültiger Dokument-Upload.",
				fun.LangPT: "❌ Upload de documento inválido.",
				fun.LangRU: "❌ Недопустимая загрузка документа.",
				fun.LangJP: "❌ 無効なドキュメントのアップロード。",
				fun.LangCN: "❌ 无效的文档上传。",
				fun.LangAR: "❌ تحميل مستند غير صالح.",
			},
		}
	}

	filename := strings.ToLower(*v.Message.DocumentMessage.FileName)
	rules := getDocumentRules()

	// Try to match document type based on filename
	var matchedRule DocumentRule
	var matched bool

	// Check each rule to find a match
	for ruleType, rule := range rules {
		if ruleType == "general_document" {
			continue // Skip general rule for now
		}

		for _, prefix := range rule.FilenamePrefixes {
			if strings.HasPrefix(filename, prefix) {
				matchedRule = rule
				matched = true
				break
			}
		}
		if matched {
			break
		}
	}

	// If no specific rule matched, use general document rule
	if !matched {
		matchedRule = rules["general_document"]
	}

	// Check user permissions
	userAllowed := checkUserPermissionForDocument(user, matchedRule)
	if !userAllowed.Allowed {
		return false, userAllowed.Message
	}

	// Check filename patterns for specific document types
	patternCheck := validateDocumentPatterns(filename, matchedRule)
	if !patternCheck.Valid {
		return false, patternCheck.Message
	}

	// Process the document if processing function is defined
	if matchedRule.ProcessFunc != nil {
		if err := matchedRule.ProcessFunc(v, user, userLang); err != nil {
			logrus.Errorf("Failed to process document: %v", err)
			// Don't fail the permission check if processing fails, just log it
		}
	}

	return true, NewLanguageMsgTranslation(userLang)
}

// getCooldownKey generates a Redis key for tracking cooldowns based on prefix and userID.
func getCooldownKey(prefix string, userID uint) string {
	return fmt.Sprintf("perm:cooldown:%s:%d", prefix, userID)
}

// getUsageKey generates a Redis key for tracking daily usage based on prefix, userID, and current date.
func getUsageKey(prefix string, userID uint) string {
	return fmt.Sprintf("perm:usage:%s:%d:%s", prefix, userID, time.Now().Format(config.DATE_YYYY_MM_DD))
}

// ValidateFileProperties validates file properties like size, extension, and MIME type
func ValidateFileProperties(filename string, fileSize int64, mimeType string, rule FilePermissionRule, userLang string) (bool, string) {
	// Check file size
	if fileSize > rule.MaxFileSizeBytes {
		maxSizeMB := rule.MaxFileSizeBytes / (1024 * 1024)

		messages := map[string]string{
			fun.LangID: fmt.Sprintf("📁 File terlalu besar! Maksimal %dMB diizinkan.", maxSizeMB),
			fun.LangEN: fmt.Sprintf("📁 File too large! Maximum %dMB allowed.", maxSizeMB),
			fun.LangES: fmt.Sprintf("📁 ¡Archivo demasiado grande! Se permite un máximo de %dMB.", maxSizeMB),
			fun.LangFR: fmt.Sprintf("📁 Fichier trop volumineux ! Maximum %dMo autorisé.", maxSizeMB),
			fun.LangDE: fmt.Sprintf("📁 Datei zu groß! Maximal %dMB erlaubt.", maxSizeMB),
			fun.LangPT: fmt.Sprintf("📁 Arquivo muito grande! Máximo de %dMB permitido.", maxSizeMB),
			fun.LangRU: fmt.Sprintf("📁 Файл слишком большой! Разрешено максимум %dМБ.", maxSizeMB),
			fun.LangJP: fmt.Sprintf("📁 ファイルが大きすぎます！最大 %dMB まで許可されています。", maxSizeMB),
			fun.LangCN: fmt.Sprintf("📁 文件过大！最大允许 %dMB。", maxSizeMB),
			fun.LangAR: fmt.Sprintf("📁 الملف كبير جدًا! الحد الأقصى المسموح به هو %d ميجابايت.", maxSizeMB),
		}

		msg, ok := messages[userLang]
		if !ok {
			msg = messages[fun.DefaultLang]
		}
		return false, msg
	}

	// Check file extension
	if len(rule.AllowedExtensions) > 0 {
		validExt := false
		filename = strings.ToLower(filename)
		for _, ext := range rule.AllowedExtensions {
			if strings.HasSuffix(filename, ext) {
				validExt = true
				break
			}
		}
		if !validExt {
			allowed := strings.Join(rule.AllowedExtensions, ", ")
			messages := map[string]string{
				fun.LangID: fmt.Sprintf("📁 Format file tidak didukung! Format yang diizinkan: %s", allowed),
				fun.LangEN: fmt.Sprintf("📁 Unsupported file format! Allowed formats: %s", allowed),
				fun.LangES: fmt.Sprintf("📁 ¡Formato de archivo no soportado! Formatos permitidos: %s", allowed),
				fun.LangFR: fmt.Sprintf("📁 Format de fichier non pris en charge ! Formats autorisés : %s", allowed),
				fun.LangDE: fmt.Sprintf("📁 Dateiformat nicht unterstützt! Erlaubte Formate: %s", allowed),
				fun.LangPT: fmt.Sprintf("📁 Formato de arquivo não suportado! Formatos permitidos: %s", allowed),
				fun.LangRU: fmt.Sprintf("📁 Неподдерживаемый формат файла! Разрешенные форматы: %s", allowed),
				fun.LangJP: fmt.Sprintf("📁 サポートされていないファイル形式です！許可された形式: %s", allowed),
				fun.LangCN: fmt.Sprintf("📁 不支持的文件格式！允许的格式: %s", allowed),
				fun.LangAR: fmt.Sprintf("📁 تنسيق الملف غير مدعوم! التنسيقات المسموح بها: %s", allowed),
			}

			msg, ok := messages[userLang]
			if !ok {
				msg = messages[fun.DefaultLang]
			}
			return false, msg
		}
	}

	// Check MIME type
	if len(rule.AllowedMimeTypes) > 0 {
		validMime := false
		for _, mime := range rule.AllowedMimeTypes {
			if mimeType == mime {
				validMime = true
				break
			}
		}
		if !validMime {
			allowed := strings.Join(rule.AllowedMimeTypes, ", ")
			messages := map[string]string{
				fun.LangID: fmt.Sprintf("📁 Tipe file _%s_ tidak didukung! Tipe yang diizinkan: %s", mimeType, allowed),
				fun.LangEN: fmt.Sprintf("📁 Unsupported file type: _%s_! Allowed types: %s", mimeType, allowed),
				fun.LangES: fmt.Sprintf("📁 ¡Tipo de archivo _%s_ no soportado! Tipos permitidos: %s", mimeType, allowed),
				fun.LangFR: fmt.Sprintf("📁 Type de fichier _%s_ non pris en charge ! Types autorisés : %s", mimeType, allowed),
				fun.LangDE: fmt.Sprintf("📁 Dateityp _%s_ nicht unterstützt! Erlaubte Typen: %s", mimeType, allowed),
				fun.LangPT: fmt.Sprintf("📁 Tipo de arquivo _%s_ não suportado! Tipos permitidos: %s", mimeType, allowed),
				fun.LangRU: fmt.Sprintf("📁 Тип файла _%s_ не поддерживается! Разрешенные типы: %s", mimeType, allowed),
				fun.LangJP: fmt.Sprintf("📁 ファイルタイプ _%s_ はサポートされていません！許可されたタイプ: %s", mimeType, allowed),
				fun.LangCN: fmt.Sprintf("📁 不支持的文件类型 _%s_！允许的类型: %s", mimeType, allowed),
				fun.LangAR: fmt.Sprintf("📁 نوع الملف _%s_ غير مدعوم! الأنواع المسموح بها: %s", mimeType, allowed),
			}

			msg, ok := messages[userLang]
			if !ok {
				msg = messages[fun.DefaultLang]
			}
			return false, msg
		}
	}

	return true, ""
}

// CheckFilePermission checks if a user is allowed to send a specific file type
func CheckFilePermission(ctx context.Context, v *events.Message, msgType string, waUser *model.WAUsers, userLang string, rdb *redis.Client) FilePermissionResult {
	// Define file permission rules for different message types
	fileRules := map[string]FilePermissionRule{
		"image": {
			AllowFunc: func(u *model.WAUsers) (bool, LanguageTranslation) {
				if u.IsBanned {
					lang := NewLanguageMsgTranslation(userLang)
					lang.Texts = map[string]string{
						fun.LangID: "🚫 Akun Anda diblokir dan tidak dapat mengirim gambar.",
						fun.LangEN: "🚫 Your account is banned and cannot upload images.",
						fun.LangES: "🚫 Su cuenta está bloqueada y no puede subir imágenes.",
						fun.LangFR: "🚫 Votre compte est banni et ne peut pas télécharger d'images.",
						fun.LangDE: "🚫 Ihr Konto ist gesperrt und kann keine Bilder hochladen.",
						fun.LangPT: "🚫 Sua conta está banida e não pode fazer upload de imagens.",
						fun.LangRU: "🚫 Ваш аккаунт заблокирован и не может загружать изображения.",
						fun.LangJP: "🚫 アカウントが禁止されており、画像をアップロードできません。",
						fun.LangCN: "🚫 您的账户已被禁止，无法上传图片。",
						fun.LangAR: "🚫 حسابك محظور ولا يمكنه تحميل الصور.",
					}
					return false, lang
				}
				// Allow all registered users to send images
				return true, NewLanguageMsgTranslation(userLang)
			},
			MaxDailyQuota:     config.GetConfig().Whatsnyan.Files.Image.MaxDailyQuota,
			CooldownSeconds:   config.GetConfig().Whatsnyan.Files.Image.CoolDownSeconds,
			MaxFileSizeBytes:  config.GetConfig().Whatsnyan.Files.Image.MaxSize * 1024 * 1024, // max size in MB converted to bytes
			AllowedExtensions: config.GetConfig().Whatsnyan.Files.Image.AllowedExtensions,     // e.g. []string{".jpg", ".jpeg", ".png", ".gif", ".webp"}
			AllowedMimeTypes:  config.GetConfig().Whatsnyan.Files.Image.AllowedMimeTypes,      // e.g. []string{"image/jpeg", "image/png", "image/gif", "image/webp"}
			DenyMessage: LanguageTranslation{
				LanguageCode: userLang,
				Texts: map[string]string{
					fun.LangID: "❌ Gagal mengirim gambar: quota habis, file terlalu besar, atau format tidak didukung.",
					fun.LangEN: "❌ Image upload failed: quota exceeded, file too large, or unsupported format.",
					fun.LangES: "❌ Error al subir imagen: cuota excedida, archivo demasiado grande o formato no soportado.",
					fun.LangFR: "❌ Échec du téléchargement de l'image : quota dépassé, fichier trop volumineux ou format non pris en charge.",
					fun.LangDE: "❌ Bild-Upload fehlgeschlagen: Kontingent überschritten, Datei zu groß oder nicht unterstütztes Format.",
					fun.LangPT: "❌ Falha no upload da imagem: cota excedida, arquivo muito grande ou formato não suportado.",
					fun.LangRU: "❌ Ошибка загрузки изображения: превышена квота, файл слишком большой или неподдерживаемый формат.",
					fun.LangJP: "❌ 画像のアップロードに失敗しました：クォータ超過、ファイルが大きすぎる、またはサポートされていない形式です。",
					fun.LangCN: "❌ 图片上传失败：配额超出，文件过大或格式不支持。",
					fun.LangAR: "❌ فشل تحميل الصورة: تم تجاوز الحصة، الملف كبير جدًا، أو التنسيق غير مدعوم.",
				},
			},
		},
		"video": {
			AllowFunc: func(u *model.WAUsers) (bool, LanguageTranslation) {
				if u.IsBanned {
					lang := NewLanguageMsgTranslation(userLang)
					lang.Texts = map[string]string{
						fun.LangID: "🚫 Akun Anda diblokir dan tidak dapat mengirim video.",
						fun.LangEN: "🚫 Your account is banned and cannot upload videos.",
						fun.LangES: "🚫 Su cuenta está bloqueada y no puede subir videos.",
						fun.LangFR: "🚫 Votre compte est banni et ne peut pas télécharger de vidéos.",
						fun.LangDE: "🚫 Ihr Konto ist gesperrt und kann keine Videos hochladen.",
						fun.LangPT: "🚫 Sua conta está banida e não pode fazer upload de vídeos.",
						fun.LangRU: "🚫 Ваш аккаунт заблокирован и не может загружать видео.",
						fun.LangJP: "🚫 アカウントが禁止されており、動画をアップロードできません。",
						fun.LangCN: "🚫 您的账户已被禁止，无法上传视频。",
						fun.LangAR: "🚫 حسابك محظور ولا يمكنه تحميل مقاطع الفيديو.",
					}
					return false, lang
				}
				// Only allow certain user types to send videos due to bandwidth concerns
				if u.UserType == model.SuperUser ||
					u.UserType == model.AdministratorUser ||
					u.PhoneNumber == config.GetConfig().Default.SuperUserPhone {
					return true, NewLanguageMsgTranslation(userLang)
				}
				lang := NewLanguageMsgTranslation(userLang)
				lang.Texts = map[string]string{
					fun.LangID: "❌ Anda tidak memiliki izin untuk mengirim video.",
					fun.LangEN: "❌ You don't have permission to upload videos.",
					fun.LangES: "❌ No tiene permiso para subir videos.",
					fun.LangFR: "❌ Vous n'avez pas la permission de télécharger des vidéos.",
					fun.LangDE: "❌ Sie haben keine Berechtigung zum Hochladen von Videos.",
					fun.LangPT: "❌ Você não tem permissão para fazer upload de vídeos.",
					fun.LangRU: "❌ У вас нет разрешения на загрузку видео.",
					fun.LangJP: "❌ 動画をアップロードする権限がありません。",
					fun.LangCN: "❌ 您没有权限上传视频。",
					fun.LangAR: "❌ ليس لديك إذن لتحميل مقاطع الفيديو.",
				}
				return false, lang
			},
			MaxDailyQuota:     config.GetConfig().Whatsnyan.Files.Video.MaxDailyQuota,
			CooldownSeconds:   config.GetConfig().Whatsnyan.Files.Video.CoolDownSeconds,
			MaxFileSizeBytes:  config.GetConfig().Whatsnyan.Files.Video.MaxSize * 1024 * 1024, // max size in MB converted to bytes
			AllowedExtensions: config.GetConfig().Whatsnyan.Files.Video.AllowedExtensions,     // e.g. []string{".mp4", ".avi", ".mov", ".3gp"}
			AllowedMimeTypes:  config.GetConfig().Whatsnyan.Files.Video.AllowedMimeTypes,      // e.g. []string{"video/mp4", "video/x-msvideo", "video/quicktime", "video/3gpp"}
			DenyMessage: LanguageTranslation{
				LanguageCode: userLang,
				Texts: map[string]string{
					fun.LangID: "❌ Gagal mengirim video: tidak ada izin, quota habis, file terlalu besar, atau format tidak didukung.",
					fun.LangEN: "❌ Video upload failed: no permission, quota exceeded, file too large, or unsupported format.",
					fun.LangES: "❌ Error al subir video: sin permiso, cuota excedida, archivo demasiado grande o formato no soportado.",
					fun.LangFR: "❌ Échec du téléchargement de la vidéo : pas de permission, quota dépassé, fichier trop volumineux ou format non pris en charge.",
					fun.LangDE: "❌ Video-Upload fehlgeschlagen: keine Berechtigung, Kontingent überschritten, Datei zu groß oder nicht unterstütztes Format.",
					fun.LangPT: "❌ Falha no upload de vídeo: sem permissão, cota excedida, arquivo muito grande ou formato não suportado.",
					fun.LangRU: "❌ Ошибка загрузки видео: нет разрешения, превышена квота, файл слишком большой или неподдерживаемый формат.",
					fun.LangJP: "❌ 動画のアップロードに失敗しました：権限なし、クォータ超過、ファイルが大きすぎる、またはサポートされていない形式です。",
					fun.LangCN: "❌ 视频上传失败：无权限，配额超出，文件过大或格式不支持。",
					fun.LangAR: "❌ فشل تحميل الفيديو: لا يوجد إذن، تم تجاوز الحصة، الملف كبير جدًا، أو التنسيق غير مدعوم.",
				},
			},
		},
		"document": {
			AllowFunc: func(u *model.WAUsers) (bool, LanguageTranslation) {
				// Permission check is done later in SanitizeAndFilterDocument
				return true, LanguageTranslation{}
			},
			MaxDailyQuota:     config.GetConfig().Whatsnyan.Files.Document.MaxDailyQuota,
			CooldownSeconds:   config.GetConfig().Whatsnyan.Files.Document.CoolDownSeconds,
			MaxFileSizeBytes:  config.GetConfig().Whatsnyan.Files.Document.MaxSize * 1024 * 1024, // max size in MB converted to bytes
			AllowedExtensions: config.GetConfig().Whatsnyan.Files.Document.AllowedExtensions,     // e.g. []string{".pdf", ".doc", ".docx", ".txt", ".zip"}
			AllowedMimeTypes:  config.GetConfig().Whatsnyan.Files.Document.AllowedMimeTypes,      // e.g. []string{"application/pdf", "application/msword", "application/vnd.openxmlformats-officedocument.wordprocessingml.document", "text/plain", "application/zip"}
			DenyMessage: LanguageTranslation{
				LanguageCode: userLang,
				Texts: map[string]string{
					fun.LangID: "❌ Gagal mengirim dokumen: tidak ada izin, quota habis, file terlalu besar, atau format tidak didukung.",
					fun.LangEN: "❌ Document upload failed: no permission, quota exceeded, file too large, or unsupported format.",
					fun.LangES: "❌ Error al subir documento: sin permiso, cuota excedida, archivo demasiado grande o formato no soportado.",
					fun.LangFR: "❌ Échec du téléchargement du document : pas de permission, quota dépassé, fichier trop volumineux ou format non pris en charge.",
					fun.LangDE: "❌ Dokument-Upload fehlgeschlagen: keine Berechtigung, Kontingent überschritten, Datei zu groß oder nicht unterstütztes Format.",
					fun.LangPT: "❌ Falha no upload do documento: sem permissão, cota excedida, arquivo muito grande ou formato não suportado.",
					fun.LangRU: "❌ Ошибка загрузки документа: нет разрешения, превышена квота, файл слишком большой или неподдерживаемый формат.",
					fun.LangJP: "❌ ドキュメントのアップロードに失敗しました：権限なし、クォータ超過、ファイルが大きすぎる、またはサポートされていない形式です。",
					fun.LangCN: "❌ 文档上传失败：无权限，配额超出，文件过大或格式不支持。",
					fun.LangAR: "❌ فشل تحميل المستند: لا يوجد إذن، تم تجاوز الحصة، الملف كبير جدًا، أو التنسيق غير مدعوم.",
				},
			},
		},
		"audio": {
			AllowFunc: func(u *model.WAUsers) (bool, LanguageTranslation) {
				if u.IsBanned {
					lang := NewLanguageMsgTranslation(userLang)
					lang.Texts = map[string]string{
						fun.LangID: "🚫 Akun Anda diblokir dan tidak dapat mengirim audio.",
						fun.LangEN: "🚫 Your account is banned and cannot upload audio files.",
						fun.LangES: "🚫 Su cuenta está bloqueada y no puede subir archivos de audio.",
						fun.LangFR: "🚫 Votre compte est banni et ne peut pas télécharger de fichiers audio.",
						fun.LangDE: "🚫 Ihr Konto ist gesperrt und kann keine Audiodateien hochladen.",
						fun.LangPT: "🚫 Sua conta está banida e não pode fazer upload de arquivos de áudio.",
						fun.LangRU: "🚫 Ваш аккаунт заблокирован и не может загружать аудиофайлы.",
						fun.LangJP: "🚫 アカウントが禁止されており、オーディオファイルをアップロードできません。",
						fun.LangCN: "🚫 您的账户已被禁止，无法上传音频文件。",
						fun.LangAR: "🚫 حسابك محظور ولا يمكنه تحميل ملفات الصوت.",
					}
					return false, lang
				}
				// Allow all registered users to send voice messages/audio
				return true, NewLanguageMsgTranslation(userLang)
			},
			MaxDailyQuota:     config.GetConfig().Whatsnyan.Files.Audio.MaxDailyQuota,
			CooldownSeconds:   config.GetConfig().Whatsnyan.Files.Audio.CoolDownSeconds,
			MaxFileSizeBytes:  config.GetConfig().Whatsnyan.Files.Audio.MaxSize * 1024 * 1024, // max size in MB converted to bytes
			AllowedExtensions: config.GetConfig().Whatsnyan.Files.Audio.AllowedExtensions,     // e.g. []string{".mp3", ".wav", ".ogg", ".m4a", ".aac"}
			AllowedMimeTypes:  config.GetConfig().Whatsnyan.Files.Audio.AllowedMimeTypes,      // e.g. []string{"audio/mpeg", "audio/wav", "audio/ogg", "audio/mp4", "audio/aac"}
			DenyMessage: LanguageTranslation{
				LanguageCode: userLang,
				Texts: map[string]string{
					fun.LangID: "❌ Gagal mengirim audio: quota habis, file terlalu besar, atau format tidak didukung.",
					fun.LangEN: "❌ Audio upload failed: quota exceeded, file too large, or unsupported format.",
					fun.LangES: "❌ Error al subir audio: cuota excedida, archivo demasiado grande o formato no soportado.",
					fun.LangFR: "❌ Échec du téléchargement audio : quota dépassé, fichier trop volumineux ou format non pris en charge.",
					fun.LangDE: "❌ Audio-Upload fehlgeschlagen: Kontingent überschritten, Datei zu groß oder nicht unterstütztes Format.",
					fun.LangPT: "❌ Falha no upload de áudio: cota excedida, arquivo muito grande ou formato não suportado.",
					fun.LangRU: "❌ Ошибка загрузки аудио: превышена квота, файл слишком большой или неподдерживаемый формат.",
					fun.LangJP: "❌ オーディオのアップロードに失敗しました：クォータ超過、ファイルが大きすぎる、またはサポートされていない形式です。",
					fun.LangCN: "❌ 音频上传失败：配额超出，文件过大或格式不支持。",
					fun.LangAR: "❌ فشل تحميل الصوت: تم تجاوز الحصة، الملف كبير جدًا، أو التنسيق غير مدعوم.",
				},
			},
		},
	}

	rule, ok := fileRules[msgType]
	if !ok {
		// Unsupported file type
		unsupportedMessages := map[string]string{
			fun.LangID: "❌ Tipe file tidak didukung.",
			fun.LangEN: "❌ Unsupported file type.",
			fun.LangES: "❌ Tipo de archivo no soportado.",
			fun.LangFR: "❌ Type de fichier non pris en charge.",
			fun.LangDE: "❌ Dateityp nicht unterstützt.",
			fun.LangPT: "❌ Tipo de arquivo não suportado.",
			fun.LangRU: "❌ Неподдерживаемый тип файла.",
			fun.LangJP: "❌ サポートされていないファイルタイプです。",
			fun.LangCN: "❌ 不支持的文件类型。",
			fun.LangAR: "❌ نوع الملف غير مدعوم.",
		}

		return FilePermissionResult{
			Allowed: false,
			Message: LanguageTranslation{Texts: unsupportedMessages},
		}
	}

	userID := waUser.ID
	usesLeft := rule.MaxDailyQuota
	cooldownLeft := 0

	// FIRST: Check cooldown before doing any processing
	if rule.CooldownSeconds > 0 {
		cooldownKey := getCooldownKey("file_"+msgType, userID)
		ttl, _ := rdb.TTL(ctx, cooldownKey).Result()
		if ttl > 0 {
			cooldownLeft = int(ttl.Seconds())

			// Dynamic language support for cooldown message
			cooldownMessages := map[string]string{
				fun.LangID: "⏱ Tunggu %d detik sebelum mengirim %s lagi.",
				fun.LangEN: "⏱ Please wait %d seconds before uploading another %s.",
				fun.LangES: "⏱ Por favor espere %d segundos antes de subir otro %s.",
				fun.LangFR: "⏱ Veuillez attendre %d secondes avant de télécharger un autre %s.",
				fun.LangDE: "⏱ Bitte warten Sie %d Sekunden, bevor Sie eine weitere %s hochladen.",
				fun.LangPT: "⏱ Por favor, aguarde %d segundos antes de enviar outro %s.",
				fun.LangRU: "⏱ Пожалуйста, подождите %d секунд перед загрузкой еще одного %s.",
				fun.LangJP: "⏱ もう一度 %[2]s をアップロードする前に %[1]d 秒お待ちください。",
				fun.LangCN: "⏱ 请等待 %d 秒后再上传另一个 %s。",
				fun.LangAR: "⏱ يرجى الانتظار %d ثانية قبل تحميل %s آخر.",
			}

			formattedMessages := make(map[string]string)
			for code, format := range cooldownMessages {
				formattedMessages[code] = fmt.Sprintf(format, cooldownLeft, msgType)
			}

			return FilePermissionResult{
				Allowed:      false,
				Message:      LanguageTranslation{Texts: formattedMessages},
				CooldownLeft: cooldownLeft,
			}
		}
	}

	// SECOND: Check quota before doing any processing
	if rule.MaxDailyQuota > 0 {
		usageKey := getUsageKey("file_"+msgType, userID)
		count, _ := rdb.Get(ctx, usageKey).Int()
		usesLeft = rule.MaxDailyQuota - count
		if count >= rule.MaxDailyQuota {
			ttl, _ := rdb.TTL(ctx, usageKey).Result()
			hours := int(ttl.Hours())
			minutes := int(ttl.Minutes()) % 60

			// Dynamic language support for quota message
			quotaMessages := map[string]string{
				fun.LangID: "🚫 Anda telah mencapai batas harian untuk mengirim %s. Coba lagi dalam %dj %dm.",
				fun.LangEN: "🚫 You have reached your daily limit for %s uploads. Try again in %dh %dm.",
				fun.LangES: "🚫 Ha alcanzado su límite diario de subidas de %s. Inténtelo de nuevo en %dh %dm.",
				fun.LangFR: "🚫 Vous avez atteint votre limite quotidienne de téléchargements de %s. Réessayez dans %dh %dm.",
				fun.LangDE: "🚫 Sie haben Ihr tägliches Limit für %s-Uploads erreicht. Versuchen Sie es in %dh %dm erneut.",
				fun.LangPT: "🚫 Você atingiu seu limite diário de uploads de %s. Tente novamente em %dh %dm.",
				fun.LangRU: "🚫 Вы достигли дневного лимита загрузок %s. Попробуйте снова через %dч %dм.",
				fun.LangJP: "🚫 %s のアップロードの1日の制限に達しました。%d時間 %d分後に再試行してください。",
				fun.LangCN: "🚫 您已达到 %s 上传的每日限制。请在 %d小时 %d分钟后重试。",
				fun.LangAR: "🚫 لقد وصلت إلى الحد اليومي لتحميلات %s. حاول مرة أخرى خلال %d ساعة و %d دقيقة.",
			}

			formattedMessages := make(map[string]string)
			for code, format := range quotaMessages {
				formattedMessages[code] = fmt.Sprintf(format, msgType, hours, minutes)
			}

			return FilePermissionResult{
				Allowed:  false,
				Message:  LanguageTranslation{Texts: formattedMessages},
				UsesLeft: 0,
			}
		}
	}

	// THIRD: Only now check user permission and process files (after quota/cooldown validation)
	allowed, denyLang := rule.AllowFunc(waUser)
	if !allowed {
		// If denyLang is empty (which shouldn't happen if properly configured), fallback to rule.DenyMessage
		if len(denyLang.Texts) == 0 {
			denyLang = rule.DenyMessage
		}
		return FilePermissionResult{
			Allowed: false,
			Message: denyLang,
		}
	}

	// File size validation (would need actual file info from WhatsApp message)
	// This is a placeholder - you'd need to extract file size from the message
	// For now, we'll just pass the max allowed size for reference
	maxFileSize := rule.MaxFileSizeBytes

	// FINAL: All checks passed - increment usage & set cooldown
	pipe := rdb.TxPipeline()
	if rule.MaxDailyQuota > 0 {
		usageKey := getUsageKey("file_"+msgType, userID)
		pipe.Incr(ctx, usageKey)
		pipe.Expire(ctx, usageKey, time.Duration(config.GetConfig().Whatsnyan.QuotaLimitExpiry)*time.Second)
	}
	if rule.CooldownSeconds > 0 {
		cooldownKey := getCooldownKey("file_"+msgType, userID)
		pipe.Set(ctx, cooldownKey, "1", time.Duration(rule.CooldownSeconds)*time.Second)
	}
	_, _ = pipe.Exec(ctx)

	return FilePermissionResult{
		Allowed:      true,
		UsesLeft:     usesLeft - 1, // after this use
		CooldownLeft: 0,
		MaxFileSize:  maxFileSize,
	}
}

// CheckPromptPermission checks if a user is allowed to use a specific command/prompt
func CheckPromptPermission(ctx context.Context, v *events.Message, cmd string, waUser *model.WAUsers, userLang string, rdb *redis.Client, db *gorm.DB) PromptPermissionResult {
	rules := map[string]PromptPermissionRule{
		"ping": {
			AllowFunc: func(u *model.WAUsers) (bool, LanguageTranslation) {
				return true, NewLanguageMsgTranslation(userLang)
			},
			MaxDailyQuota:   50,
			CooldownSeconds: 5,
			DenyMessage: LanguageTranslation{
				LanguageCode: userLang,
				Texts: map[string]string{
					fun.LangID: "❌ Anda tidak memiliki izin, kuota habis, atau terlalu cepat.",
					fun.LangEN: "❌ You're not allowed, quota exceeded, or too fast.",
					fun.LangES: "❌ No tienes permiso, cuota excedida o demasiado rápido.",
					fun.LangFR: "❌ Vous n'êtes pas autorisé, quota dépassé ou trop rapide.",
					fun.LangDE: "❌ Sie sind nicht berechtigt, Kontingent überschritten oder zu schnell.",
					fun.LangPT: "❌ Você não tem permissão, cota excedida ou muito rápido.",
					fun.LangRU: "❌ Вам не разрешено, квота превышена или слишком быстро.",
					fun.LangJP: "❌ 許可されていません、クォータ超過、または速すぎます。",
					fun.LangCN: "❌ 您未被允许，配额超出或太快。",
					fun.LangAR: "❌ غير مسموح لك، تم تجاوز الحصة، أو سريع جدًا.",
				},
			},
		},
		"get pprof": {
			AllowFunc: func(u *model.WAUsers) (bool, LanguageTranslation) {
				if u.UserType == model.SuperUser {
					return true, NewLanguageMsgTranslation(userLang)
				}
				return false, LanguageTranslation{
					LanguageCode: userLang,
					Texts: map[string]string{
						fun.LangID: "❌ Anda tidak memiliki izin untuk menggunakan perintah ini.",
						fun.LangEN: "❌ You do not have permission to use this command.",
						fun.LangES: "❌ No tienes permiso para usar este comando.",
						fun.LangFR: "❌ Vous n'avez pas la permission d'utiliser cette commande.",
						fun.LangDE: "❌ Sie haben keine Berechtigung, diesen Befehl zu verwenden.",
						fun.LangPT: "❌ Você não tem permissão para usar este comando.",
						fun.LangRU: "❌ У вас нет разрешения на использование этой команды.",
						fun.LangJP: "❌ このコマンドを使用する権限がありません。",
						fun.LangCN: "❌ 您没有权限使用此命令。",
						fun.LangAR: "❌ ليس لديك إذن لاستخدام هذا الأمر.",
					},
				}
			},
			MaxDailyQuota:   20,
			CooldownSeconds: 60,
			DenyMessage: LanguageTranslation{
				LanguageCode: userLang,
				Texts: map[string]string{
					fun.LangID: "❌ Anda tidak memiliki izin, kuota habis, atau terlalu cepat.",
					fun.LangEN: "❌ You're not allowed, quota exceeded, or too fast.",
					fun.LangES: "❌ No tienes permiso, cuota excedida o demasiado rápido.",
					fun.LangFR: "❌ Vous n'êtes pas autorisé, quota dépassé ou trop rapide.",
					fun.LangDE: "❌ Sie sind nicht berechtigt, Kontingent überschritten oder zu schnell.",
					fun.LangPT: "❌ Você não tem permissão, cota excedida ou muito rápido.",
					fun.LangRU: "❌ Вам не разрешено, квота превышена или слишком быстро.",
					fun.LangJP: "❌ 許可されていません、クォータ超過、または速すぎます。",
					fun.LangCN: "❌ 您未被允许，配额超出或太快。",
					fun.LangAR: "❌ غير مسموح لك، تم تجاوز الحصة، أو سريع جدًا.",
				},
			},
		},
		"get metrics": {
			AllowFunc: func(u *model.WAUsers) (bool, LanguageTranslation) {
				if u.UserType == model.SuperUser {
					return true, NewLanguageMsgTranslation(userLang)
				}
				return false, LanguageTranslation{
					LanguageCode: userLang,
					Texts: map[string]string{
						fun.LangID: "❌ Anda tidak memiliki izin untuk melihat metrik server.",
						fun.LangEN: "❌ You do not have permission to view server metrics.",
						fun.LangES: "❌ No tienes permiso para ver las métricas del servidor.",
						fun.LangFR: "❌ Vous n'avez pas la permission de voir les métriques du serveur.",
						fun.LangDE: "❌ Sie haben keine Berechtigung, Servermetriken anzuzeigen.",
						fun.LangPT: "❌ Você não tem permissão para ver as métricas do servidor.",
						fun.LangRU: "❌ У вас нет разрешения на просмотр метрик сервера.",
						fun.LangJP: "❌ サーバーメトリクスを表示する権限がありません。",
						fun.LangCN: "❌ 您没有权限查看服务器指标。",
						fun.LangAR: "❌ ليس لديك إذن لعرض مقاييس الخادم.",
					},
				}
			},
			MaxDailyQuota:   50,
			CooldownSeconds: 10,
			DenyMessage: LanguageTranslation{
				LanguageCode: userLang,
				Texts: map[string]string{
					fun.LangID: "❌ Anda tidak memiliki izin, kuota habis, atau terlalu cepat.",
					fun.LangEN: "❌ You're not allowed, quota exceeded, or too fast.",
					fun.LangES: "❌ No tienes permiso, cuota excedida o demasiado rápido.",
					fun.LangFR: "❌ Vous n'êtes pas autorisé, quota dépassé ou trop rapide.",
					fun.LangDE: "❌ Sie sind nicht berechtigt, Kontingent überschritten oder zu schnell.",
					fun.LangPT: "❌ Você não tem permissão, cota excedida ou muito rápido.",
					fun.LangRU: "❌ Вам не разрешено, квота превышена или слишком быстро.",
					fun.LangJP: "❌ 許可されていません、クォータ超過、または速すぎます。",
					fun.LangCN: "❌ 您未被允许，配额超出或太快。",
					fun.LangAR: "❌ غير مسموح لك، تم تجاوز الحصة، أو سريع جدًا.",
				},
			},
		},
	}

	var matchedRule string
	if strings.Contains(cmd, "get pprof") {
		matchedRule = "get pprof"
	} else if strings.Contains(cmd, "get metrics") {
		matchedRule = "get metrics"
	}

	var rule PromptPermissionRule
	var ok bool
	if matchedRule != "" {
		rule, ok = rules[matchedRule]
	} else {
		rule, ok = rules[cmd]
	}
	if !ok {
		// Check bad words
		if result := CheckBadWords(ctx, cmd, waUser, userLang, rdb, db); result != nil {
			return *result
		}

		// Command not found in rules
		return PromptPermissionResult{Allowed: true} // Allow unknown commands to pass through (e.g. for AI processing)
	}

	userID := waUser.ID
	usesLeft := rule.MaxDailyQuota
	cooldownLeft := 0

	// FIRST: Check cooldown
	if rule.CooldownSeconds > 0 {
		cooldownKey := getCooldownKey("cmd_"+cmd, userID)
		ttl, _ := rdb.TTL(ctx, cooldownKey).Result()
		if ttl > 0 {
			cooldownLeft = int(ttl.Seconds())

			cooldownMessages := map[string]string{
				fun.LangID: "⏱ Tunggu %d detik sebelum menggunakan perintah *%s* lagi.",
				fun.LangEN: "⏱ Please wait %d seconds before using *%s* command again.",
				fun.LangES: "⏱ Por favor espere %d segundos antes de usar el comando *%s* nuevamente.",
				fun.LangFR: "⏱ Veuillez attendre %d secondes avant d'utiliser à nouveau la commande *%s*.",
				fun.LangDE: "⏱ Bitte warten Sie %d Sekunden, bevor Sie den Befehl *%s* erneut verwenden.",
				fun.LangPT: "⏱ Por favor, aguarde %d segundos antes de usar o comando *%s* novamente.",
				fun.LangRU: "⏱ Пожалуйста, подождите %d секунд перед повторным использованием команды *%s*.",
				fun.LangJP: "⏱ コマンド *%[2]s* を再度使用する前に %[1]d 秒お待ちください。",
				fun.LangCN: "⏱ 请等待 %d 秒后再使用命令 *%s*。",
				fun.LangAR: "⏱ يرجى الانتظار %d ثانية قبل استخدام الأمر *%s* مرة أخرى.",
			}

			formattedMessages := make(map[string]string)
			for code, format := range cooldownMessages {
				formattedMessages[code] = fmt.Sprintf(format, cooldownLeft, cmd)
			}

			return PromptPermissionResult{
				Allowed:      false,
				Message:      LanguageTranslation{Texts: formattedMessages},
				CooldownLeft: cooldownLeft,
			}
		}
	}

	// SECOND: Check quota
	if rule.MaxDailyQuota > 0 {
		usageKey := getUsageKey("cmd_"+cmd, userID)
		count, _ := rdb.Get(ctx, usageKey).Int()
		usesLeft = rule.MaxDailyQuota - count
		if count >= rule.MaxDailyQuota {
			ttl, _ := rdb.TTL(ctx, usageKey).Result()
			hours := int(ttl.Hours())
			minutes := int(ttl.Minutes()) % 60

			quotaMessages := map[string]string{
				fun.LangID: "🚫 Anda telah mencapai batas harian untuk perintah *%s*. Coba lagi dalam %dj %dm.",
				fun.LangEN: "🚫 You have reached your daily limit for command *%s*. Try again in %dh %dm.",
				fun.LangES: "🚫 Ha alcanzado su límite diario para el comando *%s*. Inténtelo de nuevo en %dh %dm.",
				fun.LangFR: "🚫 Vous avez atteint votre limite quotidienne pour la commande *%s*. Réessayez dans %dh %dm.",
				fun.LangDE: "🚫 Sie haben Ihr tägliches Limit für den Befehl *%s* erreicht. Versuchen Sie es in %dh %dm erneut.",
				fun.LangPT: "🚫 Você atingiu seu limite diário para o comando *%s*. Tente novamente em %dh %dm.",
				fun.LangRU: "🚫 Вы достигли дневного лимита для команды *%s*. Попробуйте снова через %dч %dм.",
				fun.LangJP: "🚫 コマンド *%s* の1日の制限に達しました。%d時間 %d分後に再試行してください。",
				fun.LangCN: "🚫 您已达到命令 *%s* 的每日限制。请在 %d小时 %d分钟后重试。",
				fun.LangAR: "🚫 لقد وصلت إلى الحد اليومي للأمر *%s*. حاول مرة أخرى خلال %d ساعة و %d دقيقة.",
			}

			formattedMessages := make(map[string]string)
			for code, format := range quotaMessages {
				formattedMessages[code] = fmt.Sprintf(format, cmd, hours, minutes)
			}

			return PromptPermissionResult{
				Allowed:  false,
				Message:  LanguageTranslation{Texts: formattedMessages},
				UsesLeft: 0,
			}
		}
	}

	// THIRD: Check user permission
	allowed, denyLang := rule.AllowFunc(waUser)
	if !allowed {
		if len(denyLang.Texts) == 0 {
			denyLang = rule.DenyMessage
		}
		return PromptPermissionResult{
			Allowed: false,
			Message: denyLang,
		}
	}

	// FINAL: Increment usage & set cooldown
	pipe := rdb.TxPipeline()
	if rule.MaxDailyQuota > 0 {
		usageKey := getUsageKey("cmd_"+cmd, userID)
		pipe.Incr(ctx, usageKey)
		pipe.Expire(ctx, usageKey, time.Duration(config.GetConfig().Whatsnyan.QuotaLimitExpiry)*time.Second)
	}
	if rule.CooldownSeconds > 0 {
		cooldownKey := getCooldownKey("cmd_"+cmd, userID)
		pipe.Set(ctx, cooldownKey, "1", time.Duration(rule.CooldownSeconds)*time.Second)
	}
	_, _ = pipe.Exec(ctx)

	return PromptPermissionResult{
		Allowed:      true,
		UsesLeft:     usesLeft - 1,
		CooldownLeft: 0,
	}
}

func CheckBadWords(ctx context.Context, cmd string, waUser *model.WAUsers, userLang string, rdb *redis.Client, db *gorm.DB) *PromptPermissionResult {
	var badWords []model.BadWord
	if err := db.Where("is_enabled = ?", true).Find(&badWords).Error; err == nil {
		lowerCmd := strings.ToLower(cmd)
		for _, bw := range badWords {
			if userLang == bw.Language && strings.Contains(lowerCmd, strings.ToLower(bw.Word)) {
				strikeKey := fmt.Sprintf("bad_word_strikes:%d", waUser.ID)
				strikes, err := rdb.Incr(ctx, strikeKey).Result()
				if err != nil {
					logrus.Errorf("Failed to increment bad word strikes: %v", err)
				}

				maxStrikes := config.GetConfig().Default.MaxBadWordStrikes
				if maxStrikes <= 0 {
					maxStrikes = 3 // default to 3 if not set
				}

				if strikes < maxStrikes {
					return &PromptPermissionResult{
						Allowed: false,
						Message: LanguageTranslation{
							LanguageCode: userLang,
							Texts: map[string]string{
								fun.LangID: fmt.Sprintf("⚠️ Kata kasar terdeteksi! Peringatan %d/%d.", strikes, maxStrikes),
								fun.LangEN: fmt.Sprintf("⚠️ Bad word detected! Warning %d/%d.", strikes, maxStrikes),
								fun.LangES: fmt.Sprintf("⚠️ ¡Palabra malsonante detectada! Advertencia %d/%d.", strikes, maxStrikes),
								fun.LangFR: fmt.Sprintf("⚠️ Gros mot détecté ! Avertissement %d/%d.", strikes, maxStrikes),
								fun.LangDE: fmt.Sprintf("⚠️ Schimpfwort erkannt! Warnung %d/%d.", strikes, maxStrikes),
								fun.LangPT: fmt.Sprintf("⚠️ Palavrão detectado! Aviso %d/%d.", strikes, maxStrikes),
								fun.LangRU: fmt.Sprintf("⚠️ Обнаружено ругательство! Предупреждение %d/%d.", strikes, maxStrikes),
								fun.LangJP: fmt.Sprintf("⚠️ 不適切な言葉が検出されました！警告 %d/%d。", strikes, maxStrikes),
								fun.LangCN: fmt.Sprintf("⚠️ 检测到脏话！警告 %d/%d。", strikes, maxStrikes),
								fun.LangAR: fmt.Sprintf("⚠️ تم اكتشاف كلمة سيئة! تحذير %d/%d.", strikes, maxStrikes),
							},
						},
					}
				} else {
					waUser.IsBanned = true
					waUser.AllowedToCall = false
					if err := db.Save(waUser).Error; err != nil {
						logrus.Errorf("Failed to ban user %d: %v", waUser.ID, err)
					}
					return &PromptPermissionResult{
						Allowed: false,
						Message: LanguageTranslation{
							LanguageCode: userLang,
							Texts: map[string]string{
								fun.LangID: "🚫 Akun Anda telah diblokir karena penggunaan kata-kata kasar yang berlebihan.",
								fun.LangEN: "🚫 Your account has been banned due to excessive use of bad words.",
								fun.LangES: "🚫 Su cuenta ha sido bloqueada debido al uso excesivo de malas palabras.",
								fun.LangFR: "🚫 Votre compte a été banni en raison de l'utilisation excessive de gros mots.",
								fun.LangDE: "🚫 Ihr Konto wurde wegen übermäßiger Verwendung von Schimpfwörtern gesperrt.",
								fun.LangPT: "🚫 Sua conta foi banida devido ao uso excessivo de palavrões.",
								fun.LangRU: "🚫 Ваша учетная запись была заблокирована из-за чрезмерного использования ругательств.",
								fun.LangJP: "🚫 不適切な言葉の過度な使用により、アカウントが停止されました。",
								fun.LangCN: "🚫 由于过度使用脏话，您的帐户已被封禁。",
								fun.LangAR: "🚫 تم حظر حسابك بسبب الاستخدام المفرط للكلمات السيئة.",
							},
						},
					}
				}
			}
		}
	}
	return nil
}

// HandleKeywordSearch handles the keyword search logic for auto-replies
func HandleKeywordSearch(ctx context.Context, v *events.Message, stanzaID string, lowerMsgText, originalSenderJID, userLang string, userSanitizeResult *model.WAUsers, client *whatsmeow.Client, rdb *redis.Client, db *gorm.DB) {
	var lang model.Language
	if err := db.Where("code = ?", userLang).First(&lang).Error; err != nil {
		db.Where("code = ?", fun.DefaultLang).First(&lang)
	}

	if lang.ID != 0 {
		var userType model.WAUserType = model.CommonUser
		var userOf model.WAUserOf = model.ClientCompanyEmployee

		if userSanitizeResult != nil {
			userType = userSanitizeResult.UserType
			userOf = userSanitizeResult.UserOf
		}

		var autoReplies []model.WhatsappMessageAutoReply
		if err := db.Where("language_id = ? AND user_of = ? AND (for_user_type = ? OR for_user_type = ?)",
			lang.ID, userOf, model.CommonUser, userType).Find(&autoReplies).Error; err == nil {

			dataSeparator := config.GetConfig().Default.DataSeparator
			if dataSeparator == "" {
				dataSeparator = "|"
			}

			var matchedReply *model.WhatsappMessageAutoReply
			var closestKeyword string
			minDistance := 100 // Initialize with a high value

			for _, reply := range autoReplies {
				keywords := strings.Split(reply.Keywords, dataSeparator)
				for _, k := range keywords {
					k = strings.TrimSpace(strings.ToLower(k))
					if k != "" {
						if k == lowerMsgText {
							matchedReply = &reply
							break
						}

						// Calculate Levenshtein distance
						dist := levenshtein.ComputeDistance(lowerMsgText, k)
						if dist < minDistance {
							minDistance = dist
							closestKeyword = k
						}
					}
				}
				if matchedReply != nil {
					break
				}
			}

			if matchedReply != nil {
				replyMsg := LanguageTranslation{
					LanguageCode: userLang,
					Texts: map[string]string{
						userLang: matchedReply.ReplyText,
					},
				}
				SendLangWhatsAppTextMsg(originalSenderJID, stanzaID, v, replyMsg, userLang, client, rdb, db)
				return
			} else {
				// Suggest closest keyword if distance is reasonable
				// For short words (len <= 3), allow max 1 edit.
				// For medium words (4 <= len <= 5), allow max 2 edits.
				// For longer words, allow max 3 edits.
				threshold := 3
				if len(lowerMsgText) <= 3 {
					threshold = 1
				} else if len(lowerMsgText) <= 5 {
					threshold = 2
				}

				if closestKeyword != "" && minDistance <= threshold {
					prefixMap := map[string]string{
						fun.LangID: "Mungkin maksud Anda:",
						fun.LangEN: "Did you mean:",
						fun.LangES: "¿Quiso decir:",
						fun.LangFR: "Vouliez-vous dire :",
						fun.LangDE: "Meinten Sie:",
						fun.LangPT: "Você quis dizer:",
						fun.LangRU: "Вы имели в виду:",
						fun.LangJP: "もしかして：",
						fun.LangCN: "你的意思是：",
						fun.LangAR: "هل تقصد:",
					}

					prefix, ok := prefixMap[userLang]
					if !ok {
						prefix = prefixMap[fun.DefaultLang]
					}

					msg := fmt.Sprintf("%s *%s* ?", prefix, closestKeyword)

					suggestionMsg := LanguageTranslation{
						LanguageCode: userLang,
						Texts: map[string]string{
							userLang: msg,
						},
					}
					SendLangWhatsAppTextMsg(originalSenderJID, stanzaID, v, suggestionMsg, userLang, client, rdb, db)
					return
				}
			}
		}
	}
}
