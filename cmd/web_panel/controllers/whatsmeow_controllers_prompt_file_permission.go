package controllers

import (
	"fmt"
	"regexp"
	"service-platform/cmd/web_panel/model"
	"service-platform/internal/config"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	"go.mau.fi/whatsmeow/types/events"
)

// FilePermissionRule: rules for file/document handling
type FilePermissionRule struct {
	AllowFunc         func(user *model.WAPhoneUser) (bool, string, string) // returns: (allowed, customMsgEN, customMsgID)
	DenyMessageID     string
	DenyMessageEN     string
	MaxDailyQuota     int      // e.g. 10 files per day
	CooldownSeconds   int      // e.g. 30 sec cooldown between file uploads
	MaxFileSizeBytes  int64    // maximum file size in bytes (e.g. 10MB = 10*1024*1024)
	AllowedExtensions []string // allowed file extensions (e.g. []string{".pdf", ".jpg", ".png"})
	AllowedMimeTypes  []string // allowed MIME types (e.g. []string{"application/pdf", "image/jpeg"})
}

// FilePermissionResult represents the result of file permission check
type FilePermissionResult struct {
	Allowed      bool
	Message      string // deny message or empty if allowed
	UsesLeft     int    // how many times can still use today
	CooldownLeft int    // seconds left before allowed again
	MaxFileSize  int64  // maximum allowed file size in bytes
}

// DocumentRule represents rules for specific document types
type DocumentRule struct {
	FilenamePrefixes []string                                                                // e.g., []string{"report pemasangan", "invoice"}
	FilenamePatterns []string                                                                // regex patterns for filename matching
	AllowedUserTypes []string                                                                // user types allowed to upload this document
	AllowedUserOf    []string                                                                // user organizations allowed
	RequiredPatterns []string                                                                // patterns that must exist in filename
	MonthPatterns    []string                                                                // month names to look for
	YearRequired     bool                                                                    // whether year is required in filename
	Description      string                                                                  // description of the document type
	ProcessFunc      func(v *events.Message, user *model.WAPhoneUser, userLang string) error // function to process the uploaded file
}

// DocumentFilterResult represents the result of document filtering
type DocumentFilterResult struct {
	Allowed      bool
	DocumentType string // identified document type
	Reason       string // reason for allow/deny
	MessageEN    string // English message
	MessageID    string // Indonesian message
}

// // processGeneralDocumentFile processes general document uploads
// func processGeneralDocumentFile(v *events.Message, user *model.WAPhoneUser, userLang string) error {
// 	if v == nil || v.Message.DocumentMessage == nil || v.Message.DocumentMessage.FileName == nil {
// 		return errors.New("no document or filename found")
// 	}

// 	filename := *v.Message.DocumentMessage.FileName
// 	logrus.Infof("Processing general document: %s from user: %s", filename, user.PhoneNumber)

// 	// Simple acknowledgment for general documents
// 	var ackMsg string
// 	if userLang == "id" {
// 		ackMsg = fmt.Sprintf("📄 Dokumen '%s' berhasil diterima dan akan diproses.", filename)
// 	} else {
// 		ackMsg = fmt.Sprintf("📄 Document '%s' received successfully and will be processed.", filename)
// 	}

// 	originalSenderJID := NormalizeSenderJID(v.Info.Sender.String())
// 	sendTextMessageViaBot(originalSenderJID, ackMsg)

// 	return nil
// }

// getDocumentRules returns predefined rules for different document types
func getDocumentRules() map[string]DocumentRule {
	return map[string]DocumentRule{
		"report_pemasangan_mti": {
			FilenamePrefixes: []string{"report pemasangan", "installation report"},
			AllowedUserTypes: []string{"ODOOMSHead", "WaBotSuperUser", "SupportStaff", "ODOOMSStaff", "CompanyPM", "CompanyPMO"},
			AllowedUserOf:    []string{"UserOfCSNA"},
			RequiredPatterns: []string{}, // Remove required patterns to allow files like "report pemasangan juli 2025.xlsx"
			MonthPatterns:    []string{"januari", "februari", "maret", "april", "mei", "juni", "juli", "agustus", "september", "oktober", "november", "desember", "january", "february", "march", "april", "may", "june", "july", "august", "september", "october", "november", "december"},
			YearRequired:     true,
			Description:      "Report Pemasangan MTI",
			ProcessFunc:      processExcelReportPemasanganMTI, // Add processing function
		},
		"report_penarikan_mti": {
			FilenamePrefixes: []string{"report penarikan", "update report penarikan", "withdrawal report"},
			AllowedUserTypes: []string{"ODOOMSHead", "WaBotSuperUser", "SupportStaff", "ODOOMSStaff", "CompanyPM", "CompanyPMO"},
			AllowedUserOf:    []string{"UserOfCSNA"},
			RequiredPatterns: []string{}, // Remove required patterns to allow files like "report penarikan juli 2025.xlsx"
			MonthPatterns:    []string{"januari", "februari", "maret", "april", "mei", "juni", "juli", "agustus", "september", "oktober", "november", "desember", "january", "february", "march", "april", "may", "june", "july", "august", "september", "october", "november", "december"},
			YearRequired:     true,
			Description:      "Report Penarikan MTI",
			ProcessFunc:      processExcelReportPenarikanMTI, // Add processing function
		},
		"report_vtr_mti": {
			FilenamePrefixes: []string{"report vtr", "update vtr", "vtr report"},
			AllowedUserTypes: []string{"ODOOMSHead", "WaBotSuperUser", "SupportStaff", "ODOOMSStaff", "CompanyPM", "CompanyPMO"},
			AllowedUserOf:    []string{"UserOfCSNA"},
			RequiredPatterns: []string{}, // Remove required patterns to allow files like "report vtr juli 2025.xlsx"
			MonthPatterns:    []string{"januari", "februari", "maret", "april", "mei", "juni", "juli", "agustus", "september", "oktober", "november", "desember", "january", "february", "march", "april", "may", "june", "july", "august", "september", "october", "november", "december"},
			YearRequired:     false,
			Description:      "Report VTR MTI",
			ProcessFunc:      processExcelReportVTRMTI, // Add processing function
		},
		// "general_document": {
		// 	FilenamePrefixes: []string{}, // matches any file
		// 	AllowedUserTypes: []string{"ODOOMSHead", "WaBotSuperUser"},
		// 	AllowedUserOf:    []string{"UserOfCSNA"},
		// 	RequiredPatterns: []string{},
		// 	MonthPatterns:    []string{},
		// 	YearRequired:     false,
		// 	Description:      "General Document (fallback)",
		// 	ProcessFunc:      processGeneralDocumentFile, // Add processing function
		// },
		// "invoice": {
		// 	FilenamePrefixes: []string{"invoice", "inv", "tagihan", "bill"},
		// 	AllowedUserTypes: []string{"ODOOMSHead", "WaBotSuperUser", "SupportStaff"},
		// 	AllowedUserOf:    []string{"UserOfCSNA", "UserOfHommyPay"},
		// 	RequiredPatterns: []string{},
		// 	MonthPatterns:    []string{},
		// 	YearRequired:     false,
		// 	Description:      "Invoice Document",
		// },
		// "technical_report": {
		// 	FilenamePrefixes: []string{"tech report", "technical report", "laporan teknis"},
		// 	AllowedUserTypes: []string{"WaBotSuperUser", "SupportStaff", "ODOOMSTechnician"},
		// 	AllowedUserOf:    []string{"UserOfCSNA"},
		// 	RequiredPatterns: []string{},
		// 	MonthPatterns:    []string{},
		// 	YearRequired:     false,
		// 	Description:      "Technical Report Document",
		// },
		// "customer_support": {
		// 	FilenamePrefixes: []string{"cs report", "customer support", "support ticket"},
		// 	AllowedUserTypes: []string{"SupportStaff", "WaBotSuperUser"},
		// 	AllowedUserOf:    []string{"UserOfCSNA", "UserOfHommyPay"},
		// 	RequiredPatterns: []string{},
		// 	MonthPatterns:    []string{},
		// 	YearRequired:     false,
		// 	Description:      "Customer Support Document",
		// },
	}
}

// sanitizeAndFilterDocument analyzes the document and determines if it should be allowed
func sanitizeAndFilterDocument(v *events.Message, user *model.WAPhoneUser, userLang string) DocumentFilterResult {
	if v == nil || v.Message.DocumentMessage == nil || v.Message.DocumentMessage.FileName == nil {
		return DocumentFilterResult{
			Allowed:      false,
			DocumentType: "unknown",
			Reason:       "No document or filename found",
			MessageEN:    "❌ Invalid document upload.",
			MessageID:    "❌ Upload dokumen tidak valid.",
		}
	}

	filename := strings.ToLower(*v.Message.DocumentMessage.FileName)
	rules := getDocumentRules()

	// Try to match document type based on filename
	var matchedRule DocumentRule
	var documentType string
	var matched bool

	// Check each rule to find a match
	for ruleType, rule := range rules {
		if ruleType == "general_document" {
			continue // Skip general rule for now
		}

		for _, prefix := range rule.FilenamePrefixes {
			if strings.HasPrefix(filename, prefix) {
				matchedRule = rule
				documentType = ruleType
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
		documentType = "general_document"
	}

	// Check user permissions
	userAllowed := checkUserPermissionForDocument(user, matchedRule)
	if !userAllowed.Allowed {
		return DocumentFilterResult{
			Allowed:      false,
			DocumentType: documentType,
			Reason:       userAllowed.Reason,
			MessageEN:    userAllowed.MessageEN,
			MessageID:    userAllowed.MessageID,
		}
	}

	// Check filename patterns for specific document types
	patternCheck := validateDocumentPatterns(filename, matchedRule)
	if !patternCheck.Valid {
		return DocumentFilterResult{
			Allowed:      false,
			DocumentType: documentType,
			Reason:       patternCheck.Reason,
			MessageEN:    patternCheck.MessageEN,
			MessageID:    patternCheck.MessageID,
		}
	}

	// All checks passed
	result := DocumentFilterResult{
		Allowed:      true,
		DocumentType: documentType,
		Reason:       fmt.Sprintf("Document type '%s' allowed for user", documentType),
		MessageEN:    "",
		MessageID:    "",
	}

	// Process the document if processing function is defined
	if matchedRule.ProcessFunc != nil {
		if err := matchedRule.ProcessFunc(v, user, userLang); err != nil {
			logrus.Errorf("Failed to process document: %v", err)
			// Don't fail the permission check if processing fails, just log it
		}
	}

	return result
}

// UserPermissionResult represents the result of user permission check
type UserPermissionResult struct {
	Allowed   bool
	Reason    string
	MessageEN string
	MessageID string
}

// checkUserPermissionForDocument checks if user has permission to upload specific document type
func checkUserPermissionForDocument(user *model.WAPhoneUser, rule DocumentRule) UserPermissionResult {
	// Check if user is banned
	if user.IsBanned {
		return UserPermissionResult{
			Allowed:   false,
			Reason:    "User is banned",
			MessageEN: "🚫 Your account is banned and cannot upload documents.",
			MessageID: "🚫 Akun Anda diblokir dan tidak dapat mengirim dokumen.",
		}
	}

	// Check user organization (UserOf)
	userOfAllowed := false
	if len(rule.AllowedUserOf) == 0 {
		userOfAllowed = true // If no restriction, allow all
	} else {
		userOfStr := getUserOfString(user.UserOf)
		for _, allowedUserOf := range rule.AllowedUserOf {
			if userOfStr == allowedUserOf {
				userOfAllowed = true
				break
			}
		}
	}

	if !userOfAllowed {
		return UserPermissionResult{
			Allowed:   false,
			Reason:    "User organization not allowed",
			MessageEN: "❌ Your organization is not allowed to upload this document type.",
			MessageID: "❌ Organisasi Anda tidak diizinkan mengirim jenis dokumen ini.",
		}
	}

	// Check user type
	userTypeAllowed := false
	if len(rule.AllowedUserTypes) == 0 {
		userTypeAllowed = true // If no restriction, allow all
	} else {
		userTypeStr := getUserTypeString(user.UserType)
		for _, allowedUserType := range rule.AllowedUserTypes {
			if userTypeStr == allowedUserType {
				userTypeAllowed = true
				break
			}
		}
	}

	if !userTypeAllowed {
		return UserPermissionResult{
			Allowed:   false,
			Reason:    "User type not allowed",
			MessageEN: "❌ Your user type is not allowed to upload this document type.",
			MessageID: "❌ Tipe pengguna Anda tidak diizinkan mengirim jenis dokumen ini.",
		}
	}

	return UserPermissionResult{
		Allowed: true,
		Reason:  "User permission granted",
	}
}

// PatternValidationResult represents the result of pattern validation
type PatternValidationResult struct {
	Valid     bool
	Reason    string
	MessageEN string
	MessageID string
}

// validateDocumentPatterns validates filename patterns for specific document rules
func validateDocumentPatterns(filename string, rule DocumentRule) PatternValidationResult {
	// Check required patterns
	for _, pattern := range rule.RequiredPatterns {
		if !strings.Contains(filename, pattern) {
			return PatternValidationResult{
				Valid:     false,
				Reason:    fmt.Sprintf("Missing required pattern: %s", pattern),
				MessageEN: fmt.Sprintf("❌ Document filename must contain '%s'.", pattern),
				MessageID: fmt.Sprintf("❌ Nama file dokumen harus mengandung '%s'.", pattern),
			}
		}
	}

	// // Debugging output
	// // This will help us understand what patterns are being checked
	// // and what the filename looks like
	// fmt.Printf("Validating filename: %s\n", filename)
	// fmt.Printf("Using rule: %s\n", rule.Description)
	// fmt.Printf("Allowed User Types: %v\n", rule.AllowedUserTypes)
	// fmt.Printf("Allowed User Of: %v\n", rule.AllowedUserOf)
	// fmt.Printf("Required Patterns: %v\n", rule.RequiredPatterns)
	// fmt.Printf("Month Patterns: %v\n", rule.MonthPatterns)
	// fmt.Printf("Year Required: %v\n", rule.YearRequired)

	// Check year requirement
	if rule.YearRequired {
		yearFound := false
		// Use regex to match any 4-digit year starting with "20"
		if matched := regexp.MustCompile(`20\d{2}`).FindString(filename); matched != "" {
			yearFound = true
			// fmt.Printf("Year found: %s\n", matched)
		}

		if !yearFound {
			return PatternValidationResult{
				Valid:     false,
				Reason:    "Year required but not found",
				MessageEN: "❌ Document filename must contain a year (e.g., 2025).",
				MessageID: "❌ Nama file dokumen harus mengandung tahun (contoh: 2025).",
			}
		}
	}

	// Check month patterns if specified (for report_pemasangan_mti and others)
	if len(rule.MonthPatterns) > 0 {
		monthFound := false

		// First check for "full month" pattern
		if strings.Contains(filename, "full month") {
			monthFound = true
			// fmt.Println("Month pattern found: full month")
		}

		// If not found, check for specific month names
		if !monthFound {
			for _, month := range rule.MonthPatterns {
				if strings.Contains(filename, month) {
					monthFound = true
					// fmt.Printf("Month pattern found: %s\n", month)
					break
				}
			}
		}

		if !monthFound {
			return PatternValidationResult{
				Valid:     false,
				Reason:    "Month pattern required but not found",
				MessageEN: "❌ Document filename must contain a month name or 'full month'.",
				MessageID: "❌ Nama file dokumen harus mengandung nama bulan atau 'full month'.",
			}
		}
	}

	// fmt.Println("All patterns validated successfully!")

	return PatternValidationResult{
		Valid:  true,
		Reason: "All patterns validated successfully",
	}
} // Helper functions to convert model enums to strings
func getUserOfString(userOf model.WAUserOf) string {
	switch userOf {
	case model.UserOfCSNA:
		return "UserOfCSNA"
	case model.UserOfHommyPay:
		return "UserOfHommyPay"
	default:
		return "Unknown"
	}
}

func getUserTypeString(userType model.WAUserType) string {
	switch userType {
	case model.WaBotSuperUser:
		return "WaBotSuperUser"
	case model.SupportStaff:
		return "SupportStaff"
	case model.ODOOMSHead:
		return "ODOOMSHead"
	case model.ODOOMSTechnician:
		return "ODOOMSTechnician"
	case model.CommonUser:
		return "CommonUser"
	case model.CompanyCEO:
		return "CompanyCEO"
	case model.CompanyCOO:
		return "CompanyCOO"
	case model.CompanyCBO:
		return "CompanyCBO"
	case model.CompanyPM:
		return "CompanyPM"
	case model.CompanyPMO:
		return "CompanyPMO"
	case model.CompanySecretary:
		return "CompanySecretary"
	case model.CompanyHR:
		return "CompanyHR"
	default:
		return "Unknown"
	}
}

// CheckFilePermission validates permissions for file/document uploads based on message type and user permissions
func CheckFilePermission(v *events.Message, msgType string, user *model.WAPhoneUser, userLang string) FilePermissionResult {
	// Define file permission rules for different message types
	fileRules := map[string]FilePermissionRule{
		"image": {
			AllowFunc: func(u *model.WAPhoneUser) (bool, string, string) {
				if u.IsBanned {
					return false, "🚫 Your account is banned and cannot upload images.", "🚫 Akun Anda diblokir dan tidak dapat mengirim gambar."
				}
				// Allow all registered users to send images
				return true, "", ""
			},
			MaxDailyQuota:     50,                                                                 // 50 images per day
			CooldownSeconds:   5,                                                                  // 5 seconds between image uploads
			MaxFileSizeBytes:  config.WebPanel.Get().Whatsmeow.MaxUploadedImageSize * 1024 * 1024, // max size in MB converted to bytes
			AllowedExtensions: config.WebPanel.Get().Whatsmeow.ImageAllowedExtensions,             // e.g. []string{".jpg", ".jpeg", ".png", ".gif", ".webp"}
			AllowedMimeTypes:  config.WebPanel.Get().Whatsmeow.ImageAllowedMimeTypes,              // e.g. []string{"image/jpeg", "image/png", "image/gif", "image/webp"}
			DenyMessageEN:     "❌ Image upload failed: quota exceeded, file too large, or unsupported format.",
			DenyMessageID:     "❌ Gagal mengirim gambar: quota habis, file terlalu besar, atau format tidak didukung.",
		},
		"video": {
			AllowFunc: func(u *model.WAPhoneUser) (bool, string, string) {
				if u.IsBanned {
					return false, "🚫 Your account is banned and cannot upload videos.", "🚫 Akun Anda diblokir dan tidak dapat mengirim video."
				}
				// Only allow certain user types to send videos due to bandwidth concerns
				if u.UserType == model.WaBotSuperUser ||
					u.UserType == model.SupportStaff ||
					u.PhoneNumber == config.WebPanel.Get().Whatsmeow.WaSuperUser {
					return true, "", ""
				}
				return false, "❌ You don't have permission to upload videos.", "❌ Anda tidak memiliki izin untuk mengirim video."
			},
			MaxDailyQuota:     10,                                                                 // 10 videos per day
			CooldownSeconds:   120,                                                                // 30 seconds between video uploads
			MaxFileSizeBytes:  config.WebPanel.Get().Whatsmeow.MaxUploadedVideoSize * 1024 * 1024, // max size in MB converted to bytes
			AllowedExtensions: config.WebPanel.Get().Whatsmeow.VideoAllowedExtensions,             // e.g. []string{".mp4", ".avi", ".mov", ".3gp"}
			AllowedMimeTypes:  config.WebPanel.Get().Whatsmeow.VideoAllowedMimeTypes,              // e.g. []string{"video/mp4", "video/x-msvideo", "video/quicktime", "video/3gpp"}
			DenyMessageEN:     "❌ Video upload failed: no permission, quota exceeded, file too large, or unsupported format.",
			DenyMessageID:     "❌ Gagal mengirim video: tidak ada izin, quota habis, file terlalu besar, atau format tidak didukung.",
		},
		"document": {
			AllowFunc: func(u *model.WAPhoneUser) (bool, string, string) {
				// Use the new document sanitization and filtering system
				result := sanitizeAndFilterDocument(v, u, userLang)
				if !result.Allowed {
					return false, result.MessageEN, result.MessageID
				}
				return true, "", ""
			},
			MaxDailyQuota:     50,                                                                    // 25 documents per day
			CooldownSeconds:   10,                                                                    // 10 seconds between document uploads
			MaxFileSizeBytes:  config.WebPanel.Get().Whatsmeow.MaxUploadedDocumentSize * 1024 * 1024, // max size in MB converted to bytes
			AllowedExtensions: config.WebPanel.Get().Whatsmeow.DocumentAllowedExtensions,             // e.g. []string{".pdf", ".doc", ".docx", ".txt", ".zip"}
			AllowedMimeTypes:  config.WebPanel.Get().Whatsmeow.DocumentAllowedMimeTypes,              // e.g. []string{"application/pdf", "application/msword", "application/vnd.openxmlformats-officedocument.wordprocessingml.document", "text/plain", "application/zip"}
			DenyMessageEN:     "❌ Document upload failed: no permission, quota exceeded, file too large, or unsupported format.",
			DenyMessageID:     "❌ Gagal mengirim dokumen: tidak ada izin, quota habis, file terlalu besar, atau format tidak didukung.",
		},
		"audio": {
			AllowFunc: func(u *model.WAPhoneUser) (bool, string, string) {
				if u.IsBanned {
					return false, "🚫 Your account is banned and cannot upload audio files.", "🚫 Akun Anda diblokir dan tidak dapat mengirim audio."
				}
				// Allow all registered users to send voice messages/audio
				return true, "", ""
			},
			MaxDailyQuota:     100,                                                                // 100 audio messages per day
			CooldownSeconds:   3,                                                                  // 3 seconds between audio uploads
			MaxFileSizeBytes:  config.WebPanel.Get().Whatsmeow.MaxUploadedAudioSize * 1024 * 1024, // max size in MB converted to bytes
			AllowedExtensions: config.WebPanel.Get().Whatsmeow.AudioAllowedExtensions,             // e.g. []string{".mp3", ".wav", ".ogg", ".m4a", ".aac"}
			AllowedMimeTypes:  config.WebPanel.Get().Whatsmeow.AudioAllowedMimeTypes,              // e.g. []string{"audio/mpeg", "audio/wav", "audio/ogg", "audio/mp4", "audio/aac"}
			DenyMessageEN:     "❌ Audio upload failed: quota exceeded, file too large, or unsupported format.",
			DenyMessageID:     "❌ Gagal mengirim audio: quota habis, file terlalu besar, atau format tidak didukung.",
		},
	}

	rule, ok := fileRules[msgType]
	if !ok {
		// Unsupported file type
		msg := "❌ Unsupported file type."
		if userLang == "id" {
			msg = "❌ Tipe file tidak didukung."
		}
		return FilePermissionResult{
			Allowed: false,
			Message: msg,
		}
	}

	userID := user.ID
	usesLeft := rule.MaxDailyQuota
	cooldownLeft := 0

	// FIRST: Check cooldown before doing any processing
	if rule.CooldownSeconds > 0 {
		cooldownKey := getCooldownKey("file_"+msgType, userID)
		ttl, _ := rdb.TTL(contx, cooldownKey).Result()
		if ttl > 0 {
			cooldownLeft = int(ttl.Seconds())
			msg := fmt.Sprintf("⏱ Please wait %d seconds before uploading another %s.", cooldownLeft, msgType)
			if userLang == "id" {
				msg = fmt.Sprintf("⏱ Tunggu %d detik sebelum mengirim %s lagi.", cooldownLeft, msgType)
			}
			return FilePermissionResult{
				Allowed:      false,
				Message:      msg,
				CooldownLeft: cooldownLeft,
			}
		}
	}

	// SECOND: Check quota before doing any processing
	if rule.MaxDailyQuota > 0 {
		usageKey := getUsageKey("file_"+msgType, userID)
		count, _ := rdb.Get(contx, usageKey).Int()
		usesLeft = rule.MaxDailyQuota - count
		if count >= rule.MaxDailyQuota {
			ttl, _ := rdb.TTL(contx, usageKey).Result()
			hours := int(ttl.Hours())
			minutes := int(ttl.Minutes()) % 60

			var msg string
			if userLang == "id" {
				msg = fmt.Sprintf("🚫 Anda telah mencapai batas harian untuk mengirim %s. Coba lagi dalam %dj %dm.", msgType, hours, minutes)
			} else {
				msg = fmt.Sprintf("🚫 You have reached your daily limit for %s uploads. Try again in %dh %dm.", msgType, hours, minutes)
			}

			return FilePermissionResult{
				Allowed:  false,
				Message:  msg,
				UsesLeft: 0,
			}
		}
	}

	// THIRD: Only now check user permission and process files (after quota/cooldown validation)
	allowed, customEN, customID := rule.AllowFunc(user)
	if !allowed {
		msg := customEN
		if userLang == "id" {
			msg = customID
		}
		if msg == "" { // fallback
			if userLang == "id" {
				msg = rule.DenyMessageID
			} else {
				msg = rule.DenyMessageEN
			}
		}
		return FilePermissionResult{
			Allowed: false,
			Message: msg,
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
		pipe.Incr(contx, usageKey)
		pipe.Expire(contx, usageKey, time.Duration(config.WebPanel.Get().Whatsmeow.RedisExpiry)*time.Hour)
	}
	if rule.CooldownSeconds > 0 {
		cooldownKey := getCooldownKey("file_"+msgType, userID)
		pipe.Set(contx, cooldownKey, "1", time.Duration(rule.CooldownSeconds)*time.Second)
	}
	_, _ = pipe.Exec(contx)

	return FilePermissionResult{
		Allowed:      true,
		UsesLeft:     usesLeft - 1, // after this use
		CooldownLeft: 0,
		MaxFileSize:  maxFileSize,
	}
}

// ValidateFileProperties validates file properties like size, extension, and MIME type
func ValidateFileProperties(filename string, fileSize int64, mimeType string, rule FilePermissionRule, userLang string) (bool, string) {
	// Check file size
	if fileSize > rule.MaxFileSizeBytes {
		maxSizeMB := rule.MaxFileSizeBytes / (1024 * 1024)
		if userLang == "id" {
			return false, fmt.Sprintf("📁 File terlalu besar! Maksimal %dMB diizinkan.", maxSizeMB)
		}
		return false, fmt.Sprintf("📁 File too large! Maximum %dMB allowed.", maxSizeMB)
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
			if userLang == "id" {
				return false, fmt.Sprintf("📁 Format file tidak didukung! Format yang diizinkan: %s", strings.Join(rule.AllowedExtensions, ", "))
			}
			return false, fmt.Sprintf("📁 Unsupported file format! Allowed formats: %s", strings.Join(rule.AllowedExtensions, ", "))
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
			if userLang == "id" {
				return false, fmt.Sprintf("📁 Tipe file _%s_ tidak didukung! Tipe yang diizinkan: %s", mimeType, strings.Join(rule.AllowedMimeTypes, ", "))
			}
			return false, fmt.Sprintf("📁 Unsupported file type: _%s_! Allowed types: %s", mimeType, strings.Join(rule.AllowedMimeTypes, ", "))
		}
	}

	return true, ""
}

// AddDocumentRule allows you to add custom document rules dynamically
func AddDocumentRule(ruleKey string, rule DocumentRule) {
	rules := getDocumentRules()
	rules[ruleKey] = rule
	// Note: This modifies the map temporarily. For persistent rules, modify getDocumentRules() directly
}

// GetDocumentTypeFromFilename analyzes filename and returns the document type
func GetDocumentTypeFromFilename(filename string) (string, DocumentRule, bool) {
	lowerFilename := strings.ToLower(filename)
	rules := getDocumentRules()

	for ruleType, rule := range rules {
		if ruleType == "general_document" {
			continue // Skip general rule for specific matching
		}

		for _, prefix := range rule.FilenamePrefixes {
			if strings.HasPrefix(lowerFilename, prefix) {
				return ruleType, rule, true
			}
		}
	}

	// Return general document rule as fallback
	return "general_document", rules["general_document"], false
}

// ValidateDocumentForUser is a convenience function to quickly check if a user can upload a specific document
func ValidateDocumentForUser(filename string, user *model.WAPhoneUser, userLang string) (bool, string) {
	// Analyze the filename directly using our helper functions
	lowerFilename := strings.ToLower(filename)
	rules := getDocumentRules()

	// Try to match document type based on filename
	var matchedRule DocumentRule
	var documentType string
	var matched bool

	// Check each rule to find a match
	for ruleType, rule := range rules {
		if ruleType == "general_document" {
			continue // Skip general rule for now
		}

		for _, prefix := range rule.FilenamePrefixes {
			if strings.HasPrefix(lowerFilename, prefix) {
				matchedRule = rule
				documentType = ruleType
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
		documentType = "general_document"
	}

	// Check user permissions
	userCheck := checkUserPermissionForDocument(user, matchedRule)
	if !userCheck.Allowed {
		if userLang == "id" {
			return false, userCheck.MessageID
		}
		return false, userCheck.MessageEN
	}

	// Check filename patterns
	patternCheck := validateDocumentPatterns(lowerFilename, matchedRule)
	if !patternCheck.Valid {
		if userLang == "id" {
			return false, patternCheck.MessageID
		}
		return false, patternCheck.MessageEN
	}

	return true, fmt.Sprintf("✅ Document '%s' allowed (type: %s)", filename, documentType)
}
