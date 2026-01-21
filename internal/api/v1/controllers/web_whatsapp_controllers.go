package controllers

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"service-platform/internal/config"
	"service-platform/internal/core/model"
	whatsnyanmodel "service-platform/internal/core/model/whatsnyan_model"
	"service-platform/internal/pkg/fun"
	"service-platform/internal/whatsapp"
	pb "service-platform/proto"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// GetWhatsAppStatus godoc
// @Summary      Get WhatsApp Connection Status
// @Description  Returns the current WhatsApp connection status and account info
// @Tags         WhatsApp Web
// @Accept       json
// @Produce      json
// @Success      200  {object}   map[string]interface{}
// @Router       /api/v1/{access}/tab-whatsapp/status [get]
func GetWhatsAppStatus(c *gin.Context) {
	// Check if gRPC client is available
	if whatsapp.Client == nil {
		c.JSON(http.StatusOK, gin.H{
			"connected": false,
			"message":   "WhatsApp gRPC service not available",
		})
		return
	}

	// Check if WhatsApp client is actually connected
	ctx := c.Request.Context()
	isConnectedResp, err := whatsapp.Client.IsConnected(ctx, &pb.IsConnectedRequest{})

	if err != nil {
		// gRPC service is down or not responding
		c.JSON(http.StatusOK, gin.H{
			"connected": false,
			"message":   "WhatsApp service not responding",
		})
		return
	}

	if !isConnectedResp.Connected {
		// WhatsApp is not connected
		c.JSON(http.StatusOK, gin.H{
			"connected": false,
			"message":   isConnectedResp.Message,
		})
		return
	}

	// WhatsApp is connected - get account info
	accountResp, err := whatsapp.Client.GetMe(ctx, &pb.GetMeRequest{})
	if err == nil && accountResp.Success {
		c.JSON(http.StatusOK, gin.H{
			"connected": true,
			"message":   "WhatsApp is connected",
			"account": gin.H{
				"name":            accountResp.Name,
				"phone":           accountResp.PhoneNumber,
				"jid":             accountResp.Jid,
				"profile_pic_url": accountResp.ProfilePicUrl,
				"device":          accountResp.Device,
				"platform":        accountResp.Platform,
			},
		})
	} else {
		// Fallback if GetMe fails
		c.JSON(http.StatusOK, gin.H{
			"connected": true,
			"message":   "WhatsApp is connected",
			"account": gin.H{
				"name":            "WhatsApp Bot",
				"phone":           "",
				"jid":             "",
				"profile_pic_url": "",
				"device":          "",
				"platform":        "",
			},
		})
	}
}

// GetWhatsAppMessages godoc
// @Summary      Get Sent WhatsApp Messages
// @Description  Returns a paginated list of sent WhatsApp messages
// @Tags         WhatsApp Web
// @Accept       json
// @Produce      json
// @Param        page query int false "Page number" default(1)
// @Param        limit query int false "Items per page" default(20)
// @Success      200  {object}   map[string]interface{}
// @Router       /api/v1/{access}/tab-whatsapp/messages [get]
func GetWhatsAppMessages(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
		limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
		offset := (page - 1) * limit

		var messages []whatsnyanmodel.WhatsAppMsg
		var total int64

		// Count total
		db.Model(&whatsnyanmodel.WhatsAppMsg{}).Count(&total)

		// Get messages
		result := db.Order("created_at DESC").
			Limit(limit).
			Offset(offset).
			Find(&messages)

		if result.Error != nil {
			fun.HandleAPIErrorSimple(c, http.StatusInternalServerError, "Failed to fetch messages")
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"data": messages,
			"pagination": gin.H{
				"current_page": page,
				"total_pages":  (total + int64(limit) - 1) / int64(limit),
				"total_items":  total,
				"per_page":     limit,
			},
		})
	}
}

// GetWhatsAppIncomingMessages godoc
// @Summary      Get Incoming WhatsApp Messages
// @Description  Returns a paginated list of incoming WhatsApp messages
// @Tags         WhatsApp Web
// @Accept       json
// @Produce      json
// @Param        page query int false "Page number" default(1)
// @Param        limit query int false "Items per page" default(20)
// @Success      200  {object}   map[string]interface{}
// @Router       /api/v1/{access}/tab-whatsapp/incoming [get]
func GetWhatsAppIncomingMessages(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
		limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
		offset := (page - 1) * limit

		var messages []whatsnyanmodel.WhatsAppIncomingMsg
		var total int64

		// Count total
		db.Model(&whatsnyanmodel.WhatsAppIncomingMsg{}).Count(&total)

		// Get messages
		result := db.Order("created_at DESC").
			Limit(limit).
			Offset(offset).
			Find(&messages)

		if result.Error != nil {
			fun.HandleAPIErrorSimple(c, http.StatusInternalServerError, "Failed to fetch incoming messages")
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"data": messages,
			"pagination": gin.H{
				"current_page": page,
				"total_pages":  (total + int64(limit) - 1) / int64(limit),
				"total_items":  total,
				"per_page":     limit,
			},
		})
	}
}

// GetWhatsAppGroups godoc
// @Summary      Get WhatsApp Groups
// @Description  Returns a paginated list of WhatsApp groups (DataTables Server-Side)
// @Tags         WhatsApp Web
// @Accept       json
// @Produce      json
// @Success      200  {object}   map[string]interface{}
// @Router       /api/v1/{access}/tab-whatsapp/groups [post]
func GetWhatsAppGroups(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Parse DataTables request
		var dtRequest struct {
			Draw   int `json:"draw"`
			Start  int `json:"start"`
			Length int `json:"length"`
			Search struct {
				Value string `json:"value"`
			} `json:"search"`
			Order []struct {
				Column int    `json:"column"`
				Dir    string `json:"dir"`
			} `json:"order"`
		}

		if err := c.ShouldBindJSON(&dtRequest); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"draw":            0,
				"recordsTotal":    0,
				"recordsFiltered": 0,
				"data":            []interface{}{},
				"error":           "Invalid request format",
			})
			return
		}

		query := db.Model(&whatsnyanmodel.WhatsAppGroup{})

		// Apply global search
		if dtRequest.Search.Value != "" {
			searchPattern := "%" + strings.ToLower(dtRequest.Search.Value) + "%"
			query = query.Where(
				"LOWER(name) LIKE ? OR LOWER(jid) LIKE ?",
				searchPattern, searchPattern,
			)
		}

		// Count filtered records
		var recordsFiltered int64
		query.Count(&recordsFiltered)

		// Count total records (without filters)
		var recordsTotal int64
		db.Model(&whatsnyanmodel.WhatsAppGroup{}).Count(&recordsTotal)

		// Apply ordering
		if len(dtRequest.Order) > 0 {
			orderColumn := dtRequest.Order[0].Column
			orderDir := dtRequest.Order[0].Dir

			// Map column index to database field
			columnMap := []string{"id", "name", "jid", "owner", "participant_count", "is_announcements", "created_at"}
			if orderColumn >= 0 && orderColumn < len(columnMap) {
				orderField := columnMap[orderColumn]
				if orderDir == "desc" {
					query = query.Order(orderField + " DESC")
				} else {
					query = query.Order(orderField + " ASC")
				}
			}
		} else {
			query = query.Order("created_at DESC")
		}

		// Apply pagination
		query = query.Limit(dtRequest.Length).Offset(dtRequest.Start)

		// Get groups with participants
		var groups []whatsnyanmodel.WhatsAppGroup
		result := query.Preload("Participants").Find(&groups)

		if result.Error != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"draw":            dtRequest.Draw,
				"recordsTotal":    0,
				"recordsFiltered": 0,
				"data":            []interface{}{},
				"error":           "Failed to fetch groups",
			})
			return
		}

		// Return DataTables response
		c.JSON(http.StatusOK, gin.H{
			"draw":            dtRequest.Draw,
			"recordsTotal":    recordsTotal,
			"recordsFiltered": recordsFiltered,
			"data":            groups,
		})
	}
}

// GetWhatsAppGroupByJID godoc
// @Summary      Get WhatsApp Group Details
// @Description  Returns detailed information about a specific WhatsApp group from database with live updates
// @Tags         WhatsApp Web
// @Accept       json
// @Produce      json
// @Param        jid path string true "Group JID"
// @Success      200  {object}   map[string]interface{}
// @Router       /api/v1/{access}/tab-whatsapp/groups/{jid} [get]
func GetWhatsAppGroupByJID(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		jid := c.Param("jid")

		// First, try to get basic info from database
		var dbGroup whatsnyanmodel.WhatsAppGroup
		dbResult := db.Preload("Participants").Where("jid = ?", jid).First(&dbGroup)

		// Initialize with database data
		groupData := map[string]interface{}{
			"jid":                  jid,
			"name":                 "-",
			"owner_jid":            "-",
			"topic":                "-",
			"topic_set_at":         nil,
			"topic_set_by":         "-",
			"linked_parent_jid":    "-",
			"is_default_sub_group": false,
			"is_parent":            false,
			"description":          "-",
			"photo_url":            "",
			"participants":         []map[string]interface{}{},
			"participant_count":    0,
			"settings": map[string]interface{}{
				"locked":                  false,
				"announcement_only":       false,
				"no_frequently_forwarded": false,
				"ephemeral":               false,
				"ephemeral_duration":      0,
			},
		}

		if dbResult.Error == nil {
			// Populate with database data
			groupData["name"] = dbGroup.Name
			groupData["owner_jid"] = dbGroup.OwnerJID
			groupData["topic"] = dbGroup.Topic
			if dbGroup.TopicSetAt.Unix() > 0 {
				groupData["topic_set_at"] = dbGroup.TopicSetAt.Unix()
			}
			groupData["topic_set_by"] = dbGroup.TopicSetBy
			groupData["linked_parent_jid"] = dbGroup.LinkedParentJID
			groupData["is_default_sub_group"] = dbGroup.IsDefaultSubGroup
			groupData["is_parent"] = dbGroup.IsParent

			// Participants from database
			participants := make([]map[string]interface{}, len(dbGroup.Participants))
			for i, p := range dbGroup.Participants {
				participants[i] = map[string]interface{}{
					"jid":                 p.UserJID,
					"is_admin":            p.IsAdmin,
					"is_super_admin":      p.IsSuperAdmin,
					"lid":                 p.LID,
					"display_name":        p.DisplayName,
					"profile_picture_url": p.ProfilePictureURL,
					"phone_number":        p.PhoneNumber,
				}
			}
			groupData["participants"] = participants
			groupData["participant_count"] = len(participants)
		}

		// Now try to get live data from WhatsApp
		if whatsapp.Client != nil {
			ctx := c.Request.Context()
			groupResp, err := whatsapp.Client.GetGroupInfo(ctx, &pb.GetGroupInfoRequest{
				GroupJid: jid,
			})

			if err == nil && groupResp.Success {
				// Update with live data
				if groupResp.Name != "" {
					groupData["name"] = groupResp.Name
				}
				if groupResp.Jid != "" {
					groupData["jid"] = groupResp.Jid
				}
				if groupResp.OwnerJid != "" {
					groupData["owner_jid"] = groupResp.OwnerJid
				}
				if groupResp.Topic != "" {
					groupData["topic"] = groupResp.Topic
					groupData["description"] = groupResp.Topic // Use topic as description
				}
				if groupResp.TopicSetAt > 0 {
					groupData["topic_set_at"] = groupResp.TopicSetAt
				}
				if groupResp.TopicSetBy != "" {
					groupData["topic_set_by"] = groupResp.TopicSetBy
				}
				if groupResp.LinkedParentJid != "" {
					groupData["linked_parent_jid"] = groupResp.LinkedParentJid
				}
				groupData["is_default_sub_group"] = groupResp.IsDefaultSubGroup
				groupData["is_parent"] = groupResp.IsParent
				if groupResp.PhotoUrl != "" {
					groupData["photo_url"] = groupResp.PhotoUrl
				}

				// Live participants
				if len(groupResp.Participants) > 0 {
					participants := make([]map[string]interface{}, len(groupResp.Participants))
					for i, p := range groupResp.Participants {
						participants[i] = map[string]interface{}{
							"jid":                 p.Jid,
							"is_admin":            p.IsAdmin,
							"is_super_admin":      p.IsSuperAdmin,
							"lid":                 p.Lid,
							"display_name":        p.DisplayName,
							"profile_picture_url": p.ProfilePictureUrl,
							"phone_number":        p.PhoneNumber,
						}
					}
					groupData["participants"] = participants
					groupData["participant_count"] = len(participants)
				}

				// Settings
				if groupResp.Settings != nil {
					groupData["settings"] = map[string]interface{}{
						"locked":                  groupResp.Settings.Locked,
						"announcement_only":       groupResp.Settings.AnnouncementOnly,
						"no_frequently_forwarded": groupResp.Settings.NoFrequentlyForwarded,
						"ephemeral":               groupResp.Settings.Ephemeral,
						"ephemeral_duration":      groupResp.Settings.EphemeralDuration,
					}
				}
			}
		}

		c.JSON(http.StatusOK, groupData)
	}
}

// GetWhatsAppGroupsDataTable godoc
// @Summary      Get WhatsApp Groups (DataTables Server-Side)
// @Description  Returns WhatsApp groups in DataTables server-side format with pagination and search
// @Tags         WhatsApp Web
// @Accept       json
// @Produce      json
// @Success      200  {object}   map[string]interface{}
// @Router       /api/v1/{access}/tab-whatsapp/groups/datatable [post]
func GetWhatsAppGroupsDataTable(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Parse DataTables request
		var dtRequest struct {
			Draw   int `json:"draw"`
			Start  int `json:"start"`
			Length int `json:"length"`
			Search struct {
				Value string `json:"value"`
			} `json:"search"`
			Order []struct {
				Column int    `json:"column"`
				Dir    string `json:"dir"`
			} `json:"order"`
			Columns []struct {
				Data       string `json:"data"`
				Searchable bool   `json:"searchable"`
				Orderable  bool   `json:"orderable"`
			} `json:"columns"`
		}

		if err := c.ShouldBindJSON(&dtRequest); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"draw":            0,
				"recordsTotal":    0,
				"recordsFiltered": 0,
				"data":            []interface{}{},
				"error":           "Invalid request format",
			})
			return
		}

		query := db.Model(&whatsnyanmodel.WhatsAppGroup{})

		// Apply global search
		if dtRequest.Search.Value != "" {
			searchPattern := "%" + strings.ToLower(dtRequest.Search.Value) + "%"
			query = query.Where(
				"LOWER(name) LIKE ? OR LOWER(jid) LIKE ? OR LOWER(owner_jid) LIKE ?",
				searchPattern, searchPattern, searchPattern,
			)
		}

		// Count filtered records
		var recordsFiltered int64
		query.Count(&recordsFiltered)

		// Count total records (without filters)
		var recordsTotal int64
		db.Model(&whatsnyanmodel.WhatsAppGroup{}).Count(&recordsTotal)

		// Apply ordering
		if len(dtRequest.Order) > 0 {
			orderColumn := dtRequest.Order[0].Column
			orderDir := dtRequest.Order[0].Dir

			// Map column index to database field
			// Columns: name, jid, participants (not sortable, fallback to created_at), created_at, actions (not sortable)
			columnMap := []string{"name", "jid", "created_at", "created_at", "created_at"}
			if orderColumn >= 0 && orderColumn < len(columnMap) {
				orderField := columnMap[orderColumn]
				if orderDir == "desc" {
					query = query.Order(orderField + " DESC")
				} else {
					query = query.Order(orderField + " ASC")
				}
			}
		} else {
			query = query.Order("created_at DESC")
		}

		// Apply pagination
		query = query.Limit(dtRequest.Length).Offset(dtRequest.Start)

		// Get groups with participants
		var groups []whatsnyanmodel.WhatsAppGroup
		result := query.Preload("Participants").Find(&groups)

		if result.Error != nil {
			logrus.Errorf("GetWhatsAppGroupsDataTable: %v", result.Error)
			c.JSON(http.StatusInternalServerError, gin.H{
				"draw":            dtRequest.Draw,
				"recordsTotal":    0,
				"recordsFiltered": 0,
				"data":            []interface{}{},
				"error":           "Failed to fetch groups: " + result.Error.Error(),
			})
			return
		}

		// Handle empty groups list (bot hasn't joined any groups yet)
		if groups == nil {
			groups = []whatsnyanmodel.WhatsAppGroup{}
		}

		// Return DataTables response
		c.JSON(http.StatusOK, gin.H{
			"draw":            dtRequest.Draw,
			"recordsTotal":    recordsTotal,
			"recordsFiltered": recordsFiltered,
			"data":            groups,
		})
	}
}

// GetWhatsAppGroupsCount godoc
// @Summary      Get WhatsApp Groups Count
// @Description  Returns the total count of WhatsApp groups
// @Tags         WhatsApp Web
// @Accept       json
// @Produce      json
// @Success      200  {object}   map[string]int
// @Router       /api/v1/{access}/tab-whatsapp/groups/count [get]
func GetWhatsAppGroupsCount(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var count int64
		result := db.Model(&whatsnyanmodel.WhatsAppGroup{}).Count(&count)

		if result.Error != nil {
			fun.HandleAPIErrorSimple(c, http.StatusInternalServerError, "Failed to count groups")
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"count": count,
		})
	}
}

// SyncWhatsAppGroups godoc
// @Summary      Sync WhatsApp Groups
// @Description  Syncs WhatsApp groups from the connected account to the database
// @Tags         WhatsApp Web
// @Accept       json
// @Produce      json
// @Success      200  {object}   map[string]interface{}
// @Router       /api/v1/{access}/tab-whatsapp/groups/sync [post]
func SyncWhatsAppGroups(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Check if WhatsApp client is connected
		if whatsapp.Client == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"success": false,
				"message": "WhatsApp service is not available",
			})
			return
		}

		// Call the sync function
		go GetWhatsappGroup(db)

		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"message": "Groups sync started. Please refresh the groups table in a few seconds.",
		})
	}
}

// GetWhatsAppAutoReplyRules godoc
// @Summary      Get Auto Reply Rules
// @Description  Returns a paginated list of auto reply rules with language support
// @Tags         WhatsApp Web
// @Accept       json
// @Produce      json
// @Param        page query int false "Page number" default(1)
// @Param        limit query int false "Items per page" default(20)
// @Success      200  {object}   map[string]interface{}
// @Router       /api/v1/{access}/tab-whatsapp/auto-reply [get]
func GetWhatsAppAutoReplyRules(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
		limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
		offset := (page - 1) * limit

		var rules []map[string]interface{}
		var total int64

		// Count total
		db.Model(&model.WhatsappMessageAutoReply{}).Count(&total)

		// Get table names from config
		arTable := config.GetConfig().Database.TbWhatsappMessageAutoReply
		langTable := config.GetConfig().Database.TbLanguage

		// Get rules with language information
		result := db.Table(arTable + " as ar").
			Select("ar.id, ar.keywords, ar.reply_text, ar.for_user_type, ar.user_of, ar.language_id, ar.created_at, ar.updated_at, l.name as language, l.code as lang_code").
			Joins("LEFT JOIN " + langTable + " l ON ar.language_id = l.id").
			Order("ar.created_at DESC").
			Limit(limit).
			Offset(offset).
			Find(&rules)

		if result.Error != nil {
			fun.HandleAPIErrorSimple(c, http.StatusInternalServerError, "Failed to fetch auto reply rules")
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"data": rules,
			"pagination": gin.H{
				"current_page": page,
				"total_pages":  (total + int64(limit) - 1) / int64(limit),
				"total_items":  total,
				"per_page":     limit,
			},
		})
	}
}

// GetSupportedLanguagesCount godoc
// @Summary      Get Supported Languages Count
// @Description  Returns the count of supported languages
// @Tags         WhatsApp Web
// @Accept       json
// @Produce      json
// @Success      200  {object}   map[string]interface{}
// @Router       /api/v1/{access}/tab-whatsapp/languages/count [get]
func GetSupportedLanguagesCount(c *gin.Context) {
	languages := fun.GetSupportedLanguages()
	c.JSON(http.StatusOK, gin.H{
		"count": len(languages),
	})
}

// GetSupportedLanguagesList godoc
// @Summary      Get Supported Languages List
// @Description  Returns the list of all supported languages with their codes and names
// @Tags         WhatsApp Web
// @Accept       json
// @Produce      json
// @Success      200  {object}   map[string]interface{}
// @Router       /api/v1/{access}/tab-whatsapp/languages [get]
func GetSupportedLanguagesList(c *gin.Context) {
	languageCodes := fun.GetSupportedLanguages()

	// Build response with language codes and names
	languages := make([]map[string]string, 0, len(languageCodes))
	for _, code := range languageCodes {
		name := fun.LanguageNameMap[code]
		languages = append(languages, map[string]string{
			"code": code,
			"name": name,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"data":  languages,
		"count": len(languages),
	})
}

// GetWhatsAppAutoReplyRule godoc
// @Summary      Get Auto Reply Rule
// @Description  Returns a specific auto reply rule
// @Tags         WhatsApp Web
// @Accept       json
// @Produce      json
// @Param        id path int true "Rule ID"
// @Success      200  {object}   map[string]interface{}
// @Router       /api/v1/{access}/tab-whatsapp/auto-reply/{id} [get]
func GetWhatsAppAutoReplyRule(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.ParseUint(c.Param("id"), 10, 32)
		if err != nil {
			fun.HandleAPIErrorSimple(c, http.StatusBadRequest, "Invalid ID")
			return
		}

		// First, try to get the rule as a model to ensure it exists
		var ruleModel model.WhatsappMessageAutoReply
		if err := db.First(&ruleModel, id).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				fun.HandleAPIErrorSimple(c, http.StatusNotFound, fmt.Sprintf("Rule with ID %d not found, details: %v", id, err))
				return
			}
			fun.HandleAPIErrorSimple(c, http.StatusInternalServerError, "Failed to fetch rule")
			return
		}

		// Now get the rule with language information
		var rule map[string]interface{}
		arTable := config.GetConfig().Database.TbWhatsappMessageAutoReply
		langTable := config.GetConfig().Database.TbLanguage
		result := db.Table(arTable+" as ar").
			Select("ar.id, ar.keywords, ar.reply_text, ar.for_user_type, ar.user_of, ar.language_id, l.name as language, l.code as lang_code").
			Joins("LEFT JOIN "+langTable+" l ON ar.language_id = l.id").
			Where("ar.id = ?", id).
			Scan(&rule)

		if result.Error != nil {
			fun.HandleAPIErrorSimple(c, http.StatusInternalServerError, "Failed to fetch rule details")
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"data":    rule,
		})
	}
}

// CreateWhatsAppAutoReplyRule godoc
// @Summary      Create Auto Reply Rule
// @Description  Creates a new auto reply rule with language support
// @Tags         WhatsApp Web
// @Accept       json
// @Produce      json
// @Param        rule body map[string]interface{} true "Auto Reply Rule with language_id"
// @Success      201  {object}   map[string]interface{}
// @Router       /api/v1/{access}/tab-whatsapp/auto-reply [post]
func CreateWhatsAppAutoReplyRule(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var requestData map[string]interface{}
		if err := c.ShouldBindJSON(&requestData); err != nil {
			fun.HandleAPIErrorSimple(c, http.StatusBadRequest, err.Error())
			return
		}

		rule := model.WhatsappMessageAutoReply{
			Keywords:    requestData["keywords"].(string),
			ReplyText:   requestData["reply_text"].(string),
			ForUserType: requestData["for_user_type"].(string),
			UserOf:      requestData["user_of"].(string),
		}

		// Handle language_id - can be float64, int, string, or null
		if langID, ok := requestData["language_id"].(float64); ok && langID > 0 {
			rule.LanguageID = uint(langID)
		} else if langID, ok := requestData["language_id"].(int); ok && langID > 0 {
			rule.LanguageID = uint(langID)
		} else if langIDStr, ok := requestData["language_id"].(string); ok && langIDStr != "" {
			if id, err := strconv.ParseUint(langIDStr, 10, 32); err == nil && id > 0 {
				rule.LanguageID = uint(id)
			}
		}
		// If language_id is not provided, is 0, or is empty string, it stays as 0 (NULL)

		result := db.Create(&rule)
		if result.Error != nil {
			fun.HandleAPIErrorSimple(c, http.StatusInternalServerError, "Failed to create rule")
			return
		}

		c.JSON(http.StatusCreated, gin.H{
			"success": true,
			"message": "Auto reply rule created successfully",
			"data":    rule,
		})
	}
}

// UpdateWhatsAppAutoReplyRule godoc
// @Summary      Update Auto Reply Rule
// @Description  Updates an existing auto reply rule with language support
// @Tags         WhatsApp Web
// @Accept       json
// @Produce      json
// @Param        id path int true "Rule ID"
// @Param        rule body map[string]interface{} true "Auto Reply Rule with language_id"
// @Success      200  {object}   map[string]interface{}
// @Router       /api/v1/{access}/tab-whatsapp/auto-reply/{id} [put]
func UpdateWhatsAppAutoReplyRule(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.ParseUint(c.Param("id"), 10, 32)
		if err != nil {
			fun.HandleAPIErrorSimple(c, http.StatusBadRequest, "Invalid ID")
			return
		}

		var rule model.WhatsappMessageAutoReply
		if err := db.First(&rule, id).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				fun.HandleAPIErrorSimple(c, http.StatusNotFound, "Rule not found")
				return
			}
			fun.HandleAPIErrorSimple(c, http.StatusInternalServerError, "Failed to fetch rule")
			return
		}

		var requestData map[string]interface{}
		if err := c.ShouldBindJSON(&requestData); err != nil {
			fun.HandleAPIErrorSimple(c, http.StatusBadRequest, err.Error())
			return
		}

		// Update fields
		if keywords, ok := requestData["keywords"].(string); ok {
			rule.Keywords = keywords
		}
		if replyText, ok := requestData["reply_text"].(string); ok {
			rule.ReplyText = replyText
		}
		if userType, ok := requestData["for_user_type"].(string); ok {
			rule.ForUserType = userType
		}
		if userOf, ok := requestData["user_of"].(string); ok {
			rule.UserOf = userOf
		}

		// Handle language_id - can be 0 (NULL), float64, or int
		if langID, ok := requestData["language_id"].(float64); ok {
			rule.LanguageID = uint(langID)
		} else if langID, ok := requestData["language_id"].(int); ok {
			rule.LanguageID = uint(langID)
		} else if langIDStr, ok := requestData["language_id"].(string); ok && langIDStr != "" {
			if id, err := strconv.ParseUint(langIDStr, 10, 32); err == nil {
				rule.LanguageID = uint(id)
			}
		} else {
			// If language_id is not provided or is null/empty string, set to 0 (NULL in DB)
			rule.LanguageID = 0
		}

		if err := db.Save(&rule).Error; err != nil {
			fun.HandleAPIErrorSimple(c, http.StatusInternalServerError, "Failed to update rule")
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"message": "Auto reply rule updated successfully",
			"data":    rule,
		})
	}
}

// DeleteWhatsAppAutoReplyRule godoc
// @Summary      Delete Auto Reply Rule
// @Description  Deletes an auto reply rule
// @Tags         WhatsApp Web
// @Accept       json
// @Produce      json
// @Param        id path int true "Rule ID"
// @Success      200  {object}   map[string]interface{}
// @Router       /api/v1/{access}/tab-whatsapp/auto-reply/{id} [delete]
func DeleteWhatsAppAutoReplyRule(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.ParseUint(c.Param("id"), 10, 32)
		if err != nil {
			fun.HandleAPIErrorSimple(c, http.StatusBadRequest, "Invalid ID")
			return
		}

		result := db.Delete(&model.WhatsappMessageAutoReply{}, id)
		if result.Error != nil {
			fun.HandleAPIErrorSimple(c, http.StatusInternalServerError, "Failed to delete rule")
			return
		}

		if result.RowsAffected == 0 {
			fun.HandleAPIErrorSimple(c, http.StatusNotFound, "Rule not found")
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"message": "Auto reply rule deleted successfully",
		})
	}
}

// GetWhatsAppContactsCount godoc
// @Summary      Get WhatsApp Contacts Count
// @Description  Returns the count of synced WhatsApp contacts
// @Tags         WhatsApp Web
// @Accept       json
// @Produce      json
// @Success      200  {object}   map[string]interface{}
// @Router       /api/v1/{access}/tab-whatsapp/contacts-count [get]
func GetWhatsAppContactsCount(c *gin.Context) {
	if whatsapp.Client == nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "WhatsApp gRPC service not available",
		})
		return
	}

	ctx := c.Request.Context()
	hasContactsResp, err := whatsapp.Client.HasContacts(ctx, &pb.HasContactsRequest{})

	if err != nil || !hasContactsResp.Success {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "Failed to fetch contacts count",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"count":   0, // TODO: Implement actual contacts count from gRPC service
		"message": "Contacts count retrieved",
	})
}

// GetWhatsAppPhoneStatus godoc
// @Summary      Get WhatsApp Phone Status
// @Description  Returns battery, connection status, and device info
// @Tags         WhatsApp Web
// @Accept       json
// @Produce      json
// @Success      200  {object}   map[string]interface{}
// @Router       /api/v1/{access}/tab-whatsapp/phone-status [get]
func GetWhatsAppPhoneStatus(c *gin.Context) {
	if whatsapp.Client == nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "WhatsApp gRPC service not available",
		})
		return
	}

	ctx := c.Request.Context()
	meResp, err := whatsapp.Client.GetMe(ctx, &pb.GetMeRequest{})

	if err != nil || !meResp.Success {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "Failed to fetch phone status",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"battery": 0, // TODO: Implement battery level from whatsmeow Store
		"status":  "Connected",
		"device":  meResp.Device,
		"message": "Phone status retrieved",
	})
}

// GetWhatsAppLanguages godoc
// @Summary      Get All Supported Languages
// @Description  Returns list of all supported languages for auto-reply
// @Tags         WhatsApp Web
// @Accept       json
// @Produce      json
// @Success      200  {object}   map[string]interface{}
// @Router       /api/v1/{access}/tab-whatsapp/languages [get]
func GetWhatsAppLanguages(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var languages []model.Language

		result := db.Table(config.GetConfig().Database.TbLanguage).
			Select("id, name, code").
			Order("name ASC").
			Find(&languages)

		if result.Error != nil {
			fun.HandleAPIErrorSimple(c, http.StatusInternalServerError, "Failed to fetch languages: "+result.Error.Error())
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"data":    languages,
		})
	}
}

// GetWhatsAppLanguagesCount godoc
// @Summary      Get Supported Languages Count
// @Description  Returns the count of supported languages
// @Tags         WhatsApp Web
// @Accept       json
// @Produce      json
// @Success      200  {object}   map[string]interface{}
// @Router       /api/v1/{access}/tab-whatsapp/languages/count [get]
func GetWhatsAppLanguagesCount(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var count int64

		db.Table(config.GetConfig().Database.TbLanguage).Count(&count)

		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"count":   count,
		})
	}
}

//

// GetWhatsAppUserStatistics godoc
// @Summary      Get WhatsApp User Statistics
// @Description  Returns statistics about WhatsApp users with optional filters
// @Tags         WhatsApp User Management
// @Accept       json
// @Produce      json
// @Param        user_type query string false "Filter by user type"
// @Param        user_of query string false "Filter by user of"
// @Param        status query string false "Filter by status (active/banned)"
// @Success      200  {object}   map[string]interface{}
// @Router       /api/v1/{access}/tab-whatsapp-user-management/statistics [get]
func GetWhatsAppUserStatistics(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get optional filters
		userType := c.Query("user_type")
		userOf := c.Query("user_of")
		status := c.Query("status")

		// Base query for counting
		baseQuery := db.Model(&model.WAUsers{})

		// Apply filters to base query
		if userType != "" {
			baseQuery = baseQuery.Where("user_type = ?", userType)
		}
		if userOf != "" {
			baseQuery = baseQuery.Where("user_of = ?", userOf)
		}

		var total, active, registered, banned int64

		// Total count with filters
		baseQuery.Count(&total)

		// Active count (not banned) with filters
		activeQuery := db.Model(&model.WAUsers{}).Where("is_banned = ?", false)
		if userType != "" {
			activeQuery = activeQuery.Where("user_type = ?", userType)
		}
		if userOf != "" {
			activeQuery = activeQuery.Where("user_of = ?", userOf)
		}
		activeQuery.Count(&active)

		// Registered count with filters
		registeredQuery := db.Model(&model.WAUsers{}).Where("is_registered = ?", true)
		if userType != "" {
			registeredQuery = registeredQuery.Where("user_type = ?", userType)
		}
		if userOf != "" {
			registeredQuery = registeredQuery.Where("user_of = ?", userOf)
		}
		registeredQuery.Count(&registered)

		// Banned count with filters
		bannedQuery := db.Model(&model.WAUsers{}).Where("is_banned = ?", true)
		if userType != "" {
			bannedQuery = bannedQuery.Where("user_type = ?", userType)
		}
		if userOf != "" {
			bannedQuery = bannedQuery.Where("user_of = ?", userOf)
		}
		bannedQuery.Count(&banned)

		// If status filter is applied, adjust the counts
		switch status {
		case "active":
			banned = 0
			total = active
		case "banned":
			active = 0
			total = banned
		default:
			// No adjustment
		}

		c.JSON(http.StatusOK, gin.H{
			"total":      total,
			"active":     active,
			"registered": registered,
			"banned":     banned,
		})
	}
}

// GetWhatsAppUsers godoc
// @Summary      Get WhatsApp Users
// @Description  Returns a paginated list of WhatsApp users with filters
// @Tags         WhatsApp User Management
// @Accept       json
// @Produce      json
// @Param        page query int false "Page number" default(1)
// @Param        limit query int false "Items per page" default(20)
// @Param        search query string false "Search by name, phone, or email"
// @Param        user_type query string false "Filter by user type"
// @Param        user_of query string false "Filter by user of"
// @Param        status query string false "Filter by status (active/banned)"
// @Success      200  {object}   map[string]interface{}
// @Router       /api/v1/{access}/tab-whatsapp-user-management/users [get]
func GetWhatsAppUsers(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
		limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
		offset := (page - 1) * limit

		search := c.Query("search")
		userType := c.Query("user_type")
		userOf := c.Query("user_of")
		status := c.Query("status")

		query := db.Model(&model.WAUsers{})

		// Apply filters
		if search != "" {
			searchPattern := "%" + strings.ToLower(search) + "%"
			query = query.Where(
				"LOWER(full_name) LIKE ? OR LOWER(email) LIKE ? OR phone_number LIKE ?",
				searchPattern, searchPattern, searchPattern,
			)
		}

		if userType != "" {
			query = query.Where("user_type = ?", userType)
		}

		if userOf != "" {
			query = query.Where("user_of = ?", userOf)
		}

		switch status {
		case "active":
			query = query.Where("is_banned = ?", false)
		case "banned":
			query = query.Where("is_banned = ?", true)
		default:
			// No status filter
		}

		var total int64
		query.Count(&total)

		var users []model.WAUsers
		result := query.Order("created_at DESC").
			Limit(limit).
			Offset(offset).
			Find(&users)

		if result.Error != nil {
			fun.HandleAPIErrorSimple(c, http.StatusInternalServerError, "Failed to fetch users")
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"data": users,
			"pagination": gin.H{
				"current_page": page,
				"total_pages":  (total + int64(limit) - 1) / int64(limit),
				"total_items":  total,
				"per_page":     limit,
			},
		})
	}
}

// GetWhatsAppUsersDataTable godoc
// @Summary      Get WhatsApp Users (DataTables Server-Side)
// @Description  Returns WhatsApp users in DataTables server-side format
// @Tags         WhatsApp User Management
// @Accept       json
// @Produce      json
// @Success      200  {object}   map[string]interface{}
// @Router       /api/v1/{access}/tab-whatsapp-user-management/users/datatable [post]
func GetWhatsAppUsersDataTable(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Parse DataTables request
		var dtRequest struct {
			Draw   int `json:"draw"`
			Start  int `json:"start"`
			Length int `json:"length"`
			Search struct {
				Value string `json:"value"`
			} `json:"search"`
			Order []struct {
				Column int    `json:"column"`
				Dir    string `json:"dir"`
			} `json:"order"`
			Columns []struct {
				Data       string `json:"data"`
				Searchable bool   `json:"searchable"`
				Orderable  bool   `json:"orderable"`
			} `json:"columns"`
			// Custom filter fields
			UserType string `json:"user_type"`
			UserOf   string `json:"user_of"`
			Status   string `json:"status"`
		}

		if err := c.ShouldBindJSON(&dtRequest); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format"})
			return
		}

		query := db.Model(&model.WAUsers{})

		// Apply global search
		if dtRequest.Search.Value != "" {
			searchPattern := "%" + strings.ToLower(dtRequest.Search.Value) + "%"
			query = query.Where(
				"LOWER(full_name) LIKE ? OR LOWER(email) LIKE ? OR phone_number LIKE ?",
				searchPattern, searchPattern, searchPattern,
			)
		}

		// Apply additional filters from JSON body
		if dtRequest.UserType != "" {
			query = query.Where("user_type = ?", dtRequest.UserType)
		}

		if dtRequest.UserOf != "" {
			query = query.Where("user_of = ?", dtRequest.UserOf)
		}

		switch dtRequest.Status {
		case "active":
			query = query.Where("is_banned = ?", false)
		case "banned":
			query = query.Where("is_banned = ?", true)
		default:
			// No status filter
		}

		// Count filtered records
		var recordsFiltered int64
		query.Count(&recordsFiltered)

		// Count total records (without filters)
		var recordsTotal int64
		db.Model(&model.WAUsers{}).Count(&recordsTotal)

		// Apply ordering
		if len(dtRequest.Order) > 0 {
			orderColumn := dtRequest.Order[0].Column
			orderDir := dtRequest.Order[0].Dir

			// Map column index to database field
			columnMap := []string{"id", "full_name", "email", "phone_number", "user_type", "user_of", "is_banned", "created_at"}
			if orderColumn >= 0 && orderColumn < len(columnMap) {
				orderField := columnMap[orderColumn]
				if orderDir == "desc" {
					query = query.Order(orderField + " DESC")
				} else {
					query = query.Order(orderField + " ASC")
				}
			}
		} else {
			query = query.Order("created_at DESC")
		}

		// Apply pagination
		query = query.Limit(dtRequest.Length).Offset(dtRequest.Start)

		// Get users
		var users []model.WAUsers
		result := query.Find(&users)

		if result.Error != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"draw":            dtRequest.Draw,
				"recordsTotal":    0,
				"recordsFiltered": 0,
				"data":            []interface{}{},
				"error":           "Failed to fetch users",
			})
			return
		}

		// Return DataTables response
		c.JSON(http.StatusOK, gin.H{
			"draw":            dtRequest.Draw,
			"recordsTotal":    recordsTotal,
			"recordsFiltered": recordsFiltered,
			"data":            users,
		})
	}
}

// GetWhatsAppUser godoc
// @Summary      Get WhatsApp User
// @Description  Returns a specific WhatsApp user
// @Tags         WhatsApp User Management
// @Accept       json
// @Produce      json
// @Param        id path int true "User ID"
// @Success      200  {object}   model.WAUsers
// @Router       /api/v1/{access}/tab-whatsapp-user-management/users/{id} [get]
func GetWhatsAppUser(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.ParseUint(c.Param("id"), 10, 32)
		if err != nil {
			fun.HandleAPIErrorSimple(c, http.StatusBadRequest, "Invalid ID")
			return
		}

		var user model.WAUsers
		result := db.First(&user, id)

		if result.Error != nil {
			if result.Error == gorm.ErrRecordNotFound {
				fun.HandleAPIErrorSimple(c, http.StatusNotFound, "User not found")
				return
			}
			fun.HandleAPIErrorSimple(c, http.StatusInternalServerError, "Failed to fetch user")
			return
		}

		c.JSON(http.StatusOK, user)
	}
}

// CreateWhatsAppUser godoc
// @Summary      Create WhatsApp User
// @Description  Creates a new WhatsApp user
// @Tags         WhatsApp User Management
// @Accept       json
// @Produce      json
// @Param        user body model.WAUsers true "WhatsApp User"
// @Success      201  {object}   map[string]interface{}
// @Router       /api/v1/{access}/tab-whatsapp-user-management/users [post]
func CreateWhatsAppUser(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var user model.WAUsers

		if err := c.ShouldBindJSON(&user); err != nil {
			fun.HandleAPIErrorSimple(c, http.StatusBadRequest, fmt.Sprintf("Invalid request: %v", err))
			return
		}

		if whatsapp.Client == nil {
			fun.HandleAPIErrorSimple(c, http.StatusServiceUnavailable, "WhatsApp gRPC service not available")
			return
		}

		// Trim whitespace and check again
		phoneNumber := strings.TrimSpace(user.PhoneNumber)
		if phoneNumber == "" {
			fun.HandleAPIErrorSimple(c, http.StatusBadRequest, "Phone number is required")
			return
		}

		// Format phone number for WhatsApp validation
		// Remove any non-digit characters
		phoneFormatted := regexp.MustCompile(`\D`).ReplaceAllString(phoneNumber, "")

		if phoneFormatted == "" {
			fun.HandleAPIErrorSimple(c, http.StatusBadRequest, "Phone number must contain digits")
			return
		}

		// Validate phone number with WhatsApp
		ctx := c.Request.Context()
		isOnWAResp, err := whatsapp.Client.IsOnWhatsApp(ctx, &pb.IsOnWhatsAppRequest{
			PhoneNumbers: []string{phoneFormatted},
		})

		if err != nil {
			fun.HandleAPIErrorSimple(c, http.StatusInternalServerError, fmt.Sprintf("Failed to validate phone number: %v", err))
			return
		}

		if !isOnWAResp.Success || len(isOnWAResp.Results) == 0 {
			fun.HandleAPIErrorSimple(c, http.StatusBadRequest, fmt.Sprintf("Phone number validation failed. Please check the number format. Submitted: %s", phoneFormatted))
			return
		}

		waResult := isOnWAResp.Results[0]
		if !waResult.IsRegistered {
			fun.HandleAPIErrorSimple(c, http.StatusBadRequest, fmt.Sprintf("Phone number %s is not registered on WhatsApp", phoneFormatted))
			return
		}

		// Update phone number to the standardized format from WhatsApp (e.g., 628xxx)
		// Extract the phone number from JID (format: 628xxx@s.whatsapp.net)
		if waResult.Jid != "" {
			jidParts := strings.Split(waResult.Jid, "@")
			if len(jidParts) > 0 {
				user.PhoneNumber = jidParts[0]
			}
		} else {
			// If no JID returned, use the validated format
			user.PhoneNumber = phoneFormatted
		}

		// Check if phone number already exists in database
		var existing model.WAUsers
		if err := db.Where("phone_number = ?", user.PhoneNumber).First(&existing).Error; err == nil {
			fun.HandleAPIErrorSimple(c, http.StatusConflict, "Phone number already exists")
			return
		}

		result := db.Create(&user)
		if result.Error != nil {
			fun.HandleAPIErrorSimple(c, http.StatusInternalServerError, fmt.Sprintf("Failed to create user: %v", result.Error))
			return
		}

		c.JSON(http.StatusCreated, gin.H{
			"success": true,
			"message": "WhatsApp user created successfully",
			"data":    user,
		})
	}
}

// UpdateWhatsAppUser godoc
// @Summary      Update WhatsApp User
// @Description  Updates an existing WhatsApp user
// @Tags         WhatsApp User Management
// @Accept       json
// @Produce      json
// @Param        id path int true "User ID"
// @Param        user body model.WAUsers true "WhatsApp User"
// @Success      200  {object}   map[string]interface{}
// @Router       /api/v1/{access}/tab-whatsapp-user-management/users/{id} [put]
func UpdateWhatsAppUser(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.ParseUint(c.Param("id"), 10, 32)
		if err != nil {
			fun.HandleAPIErrorSimple(c, http.StatusBadRequest, "Invalid ID")
			return
		}

		var user model.WAUsers
		if err := db.First(&user, id).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				fun.HandleAPIErrorSimple(c, http.StatusNotFound, "User not found")
				return
			}
			fun.HandleAPIErrorSimple(c, http.StatusInternalServerError, "Failed to fetch user")
			return
		}

		var updates model.WAUsers
		if err := c.ShouldBindJSON(&updates); err != nil {
			fun.HandleAPIErrorSimple(c, http.StatusBadRequest, err.Error())
			return
		}

		// Check if phone number is being changed and validate with WhatsApp
		if updates.PhoneNumber != user.PhoneNumber {
			if whatsapp.Client == nil {
				fun.HandleAPIErrorSimple(c, http.StatusServiceUnavailable, "WhatsApp gRPC service not available")
				return
			}

			// Format phone number for WhatsApp validation
			// Remove any non-digit characters
			phoneFormatted := regexp.MustCompile(`\D`).ReplaceAllString(updates.PhoneNumber, "")

			// Validate phone number with WhatsApp
			ctx := c.Request.Context()
			isOnWAResp, err := whatsapp.Client.IsOnWhatsApp(ctx, &pb.IsOnWhatsAppRequest{
				PhoneNumbers: []string{phoneFormatted},
			})

			if err != nil {
				fun.HandleAPIErrorSimple(c, http.StatusInternalServerError, fmt.Sprintf("Failed to validate phone number: %v", err))
				return
			}

			if !isOnWAResp.Success || len(isOnWAResp.Results) == 0 {
				fun.HandleAPIErrorSimple(c, http.StatusBadRequest, fmt.Sprintf("Phone number validation failed. Please check the number format. Submitted: %s", phoneFormatted))
				return
			}

			waResult := isOnWAResp.Results[0]
			if !waResult.IsRegistered {
				fun.HandleAPIErrorSimple(c, http.StatusBadRequest, fmt.Sprintf("Phone number %s is not registered on WhatsApp", phoneFormatted))
				return
			}

			// Update phone number to the standardized format from WhatsApp (e.g., 628xxx)
			// Extract the phone number from JID (format: 628xxx@s.whatsapp.net)
			if waResult.Jid != "" {
				jidParts := strings.Split(waResult.Jid, "@")
				if len(jidParts) > 0 {
					updates.PhoneNumber = jidParts[0]
				}
			} else {
				// If no JID returned, use the validated format
				updates.PhoneNumber = phoneFormatted
			}

			// Check if the standardized phone number already exists in database
			var existing model.WAUsers
			if err := db.Where("phone_number = ? AND id != ?", updates.PhoneNumber, id).First(&existing).Error; err == nil {
				fun.HandleAPIErrorSimple(c, http.StatusConflict, "Phone number already exists")
				return
			}
		}

		// Update fields
		user.FullName = updates.FullName
		user.Email = updates.Email
		user.PhoneNumber = updates.PhoneNumber
		user.AllowedChats = updates.AllowedChats
		user.AllowedTypes = updates.AllowedTypes
		user.AllowedToCall = updates.AllowedToCall
		user.MaxDailyQuota = updates.MaxDailyQuota
		user.Description = updates.Description
		user.UseBot = updates.UseBot
		user.UserType = updates.UserType
		user.UserOf = updates.UserOf

		if err := db.Save(&user).Error; err != nil {
			fun.HandleAPIErrorSimple(c, http.StatusInternalServerError, "Failed to update user")
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"message": "WhatsApp user updated successfully",
			"data":    user,
		})
	}
}

// ToggleBanWhatsAppUser godoc
// @Summary      Toggle Ban WhatsApp User
// @Description  Bans or unbans a WhatsApp user
// @Tags         WhatsApp User Management
// @Accept       json
// @Produce      json
// @Param        id path int true "User ID"
// @Param        ban body map[string]bool true "Ban status"
// @Success      200  {object}   map[string]interface{}
// @Router       /api/v1/{access}/tab-whatsapp-user-management/users/{id}/ban [patch]
func ToggleBanWhatsAppUser(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.ParseUint(c.Param("id"), 10, 32)
		if err != nil {
			fun.HandleAPIErrorSimple(c, http.StatusBadRequest, "Invalid ID")
			return
		}

		var banRequest struct {
			IsBanned bool `json:"is_banned"`
		}

		if err := c.ShouldBindJSON(&banRequest); err != nil {
			fun.HandleAPIErrorSimple(c, http.StatusBadRequest, err.Error())
			return
		}

		result := db.Model(&model.WAUsers{}).Where("id = ?", id).Update("is_banned", banRequest.IsBanned)
		if result.Error != nil {
			fun.HandleAPIErrorSimple(c, http.StatusInternalServerError, "Failed to update user")
			return
		}

		if result.RowsAffected == 0 {
			fun.HandleAPIErrorSimple(c, http.StatusNotFound, "User not found")
			return
		}

		action := "unbanned"
		if banRequest.IsBanned {
			action = "banned"
		}

		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"message": fmt.Sprintf("User %s successfully", action),
		})
	}
}

// DeleteWhatsAppUser godoc
// @Summary      Delete WhatsApp User
// @Description  Deletes a WhatsApp user
// @Tags         WhatsApp User Management
// @Accept       json
// @Produce      json
// @Param        id path int true "User ID"
// @Success      200  {object}   map[string]interface{}
// @Router       /api/v1/{access}/tab-whatsapp-user-management/users/{id} [delete]
func DeleteWhatsAppUser(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.ParseUint(c.Param("id"), 10, 32)
		if err != nil {
			fun.HandleAPIErrorSimple(c, http.StatusBadRequest, "Invalid ID")
			return
		}

		result := db.Delete(&model.WAUsers{}, id)
		if result.Error != nil {
			fun.HandleAPIErrorSimple(c, http.StatusInternalServerError, "Failed to delete user")
			return
		}

		if result.RowsAffected == 0 {
			fun.HandleAPIErrorSimple(c, http.StatusNotFound, "User not found")
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"message": "WhatsApp user deleted successfully",
		})
	}
}

// ExportWhatsAppUsers godoc
// @Summary      Export WhatsApp Users
// @Description  Exports WhatsApp users to CSV
// @Tags         WhatsApp User Management
// @Accept       json
// @Produce      text/csv
// @Success      200  {file}   file
// @Router       /api/v1/{access}/tab-whatsapp-user-management/users/export [get]
func ExportWhatsAppUsers(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var users []model.WAUsers

		// Apply same filters as GetWhatsAppUsers
		query := db.Model(&model.WAUsers{})

		search := c.Query("search")
		userType := c.Query("user_type")
		userOf := c.Query("user_of")
		status := c.Query("status")

		if search != "" {
			searchPattern := "%" + strings.ToLower(search) + "%"
			query = query.Where(
				"LOWER(full_name) LIKE ? OR LOWER(email) LIKE ? OR phone_number LIKE ?",
				searchPattern, searchPattern, searchPattern,
			)
		}

		if userType != "" {
			query = query.Where("user_type = ?", userType)
		}

		if userOf != "" {
			query = query.Where("user_of = ?", userOf)
		}

		switch status {
		case "active":
			query = query.Where("is_banned = ?", false)
		case "banned":
			query = query.Where("is_banned = ?", true)
		default:
			// No status filter
		}

		result := query.Order("created_at DESC").Find(&users)
		if result.Error != nil {
			fun.HandleAPIErrorSimple(c, http.StatusInternalServerError, "Failed to fetch users")
			return
		}

		// Generate CSV
		c.Header("Content-Description", "File Transfer")
		c.Header("Content-Disposition", "attachment; filename=whatsapp_users.csv")
		c.Header("Content-Type", "text/csv")

		// Write CSV header
		csvData := "ID,Full Name,Email,Phone Number,User Type,User Of,Allowed Chats,Allowed Types,Max Daily Quota,Allowed To Call,Use Bot,Is Registered,Is Banned,Created At\n"

		// Write data rows
		for _, user := range users {
			allowedTypes := "[]"
			if user.AllowedTypes != nil {
				typesBytes, _ := json.Marshal(user.AllowedTypes)
				allowedTypes = string(typesBytes)
			}

			csvData += fmt.Sprintf("%d,%s,%s,%s,%s,%s,%s,\"%s\",%d,%t,%t,%t,%t,%s\n",
				user.ID,
				user.FullName,
				user.Email,
				user.PhoneNumber,
				user.UserType,
				user.UserOf,
				user.AllowedChats,
				allowedTypes,
				user.MaxDailyQuota,
				user.AllowedToCall,
				user.UseBot,
				user.IsRegistered,
				user.IsBanned,
				user.CreatedAt.Format("2006-01-02 15:04:05"),
			)
		}

		c.String(http.StatusOK, csvData)
	}
}

// GetDataSeparator godoc
// @Summary      Get Data Separator Configuration
// @Description  Returns the configured data separator character from config
// @Tags         WhatsApp Web
// @Accept       json
// @Produce      json
// @Success      200  {object}   map[string]interface{}
// @Router       /api/v1/{access}/tab-whatsapp/data-separator [get]
func GetDataSeparator(c *gin.Context) {
	separator := config.GetConfig().Default.DataSeparator

	c.JSON(http.StatusOK, gin.H{
		"success":   true,
		"separator": separator,
	})
}

// ImportWhatsAppUsers godoc
// GetImportTemplate godoc
// @Summary      Get CSV Import Template with valid options
// @Description  Returns CSV template data with valid values for user_type, user_of, allowed_chats, and allowed_types
// @Tags         WhatsApp User Management
// @Accept       json
// @Produce      json
// @Success      200  {object}   map[string]interface{}
// @Router       /api/v1/{access}/tab-whatsapp-user-management/users/import-template [get]
func GetImportTemplate(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Build valid options from model constants
		userTypes := []string{
			string(model.CommonUser),
			string(model.SuperUser),
			string(model.ClientUser),
			string(model.AdministratorUser),
		}

		userOfOptions := []string{
			string(model.CompanyEmployee),
			string(model.ClientCompanyEmployee),
		}

		allowedChatsOptions := []string{
			string(model.DirectChat),
			string(model.GroupChat),
			string(model.BothChat),
		}

		messageTypes := []string{
			string(model.TextMessage),
			string(model.ImageMessage),
			string(model.VideoMessage),
			string(model.DocumentMessage),
			string(model.AudioMessage),
			string(model.StickerMessage),
			string(model.LocationMessage),
			string(model.ContactMessage),
		}

		// Return template info
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"headers": []string{
				"full_name",
				"email",
				"phone_number",
				"user_type",
				"user_of",
				"allowed_chats",
				"max_daily_quota",
				"allowed_to_call",
				"use_bot",
				"allowed_types",
				"description",
			},
			"options": gin.H{
				"user_type":      userTypes,
				"user_of":        userOfOptions,
				"allowed_chats":  allowedChatsOptions,
				"message_types":  messageTypes,
				"boolean_values": []string{"true", "false"},
			},
			"defaults": gin.H{
				"user_type":       string(model.CommonUser),
				"user_of":         string(model.CompanyEmployee),
				"allowed_chats":   string(model.BothChat),
				"max_daily_quota": 10,
				"allowed_to_call": false,
				"use_bot":         true,
				"allowed_types":   "text",
			},
			"examples": []map[string]string{
				{
					"full_name":       "John Doe",
					"email":           "john@example.com",
					"phone_number":    "628123456789",
					"user_type":       string(model.CommonUser),
					"user_of":         string(model.CompanyEmployee),
					"allowed_chats":   string(model.BothChat),
					"max_daily_quota": "10",
					"allowed_to_call": "false",
					"use_bot":         "true",
					"allowed_types":   "text|image",
					"description":     "Example user",
				},
				{
					"full_name":       "Jane Smith",
					"email":           "jane@example.com",
					"phone_number":    "628987654321",
					"user_type":       string(model.SuperUser),
					"user_of":         string(model.CompanyEmployee),
					"allowed_chats":   string(model.DirectChat),
					"max_daily_quota": "50",
					"allowed_to_call": "true",
					"use_bot":         "true",
					"allowed_types":   "text|image|video|document",
					"description":     "Super user with more permissions",
				},
			},
			"notes": []string{
				"phone_number: Indonesian format (628xxx or 08xxx)",
				"user_type: " + strings.Join(userTypes, ", "),
				"user_of: " + strings.Join(userOfOptions, ", "),
				"allowed_chats: " + strings.Join(allowedChatsOptions, ", "),
				"allowed_types: Pipe-separated (|) list from: " + strings.Join(messageTypes, ", "),
				"allowed_to_call, use_bot: true or false",
				"If phone exists, user will be updated; otherwise created",
			},
		})
	}
}

// @Summary      Import WhatsApp Users from CSV
// @Description  Imports users from CSV file, creates new users and updates existing ones based on phone numbers
// @Tags         WhatsApp User Management
// @Accept       multipart/form-data
// @Produce      json
// @Param        file formData file true "CSV file"
// @Success      200  {object}   map[string]interface{}
// @Router       /api/v1/{access}/tab-whatsapp-user-management/users/import [post]
func ImportWhatsAppUsers(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		file, err := c.FormFile("file")
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"message": "No file uploaded",
			})
			return
		}

		// Open the uploaded file
		src, err := file.Open()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"message": "Failed to open file",
			})
			return
		}
		defer src.Close()

		// Parse CSV
		reader := csv.NewReader(src)
		records, err := reader.ReadAll()
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"message": "Failed to parse CSV file",
			})
			return
		}

		if len(records) < 2 {
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"message": "CSV file is empty or has no data rows",
			})
			return
		}

		// Check if gRPC client is available for phone validation
		if whatsapp.Client == nil {
			logrus.Warn("WhatsApp gRPC client not available for phone validation")
		}

		ctx := c.Request.Context()
		created := 0
		updated := 0
		failed := 0

		// Skip header row, process data rows
		for i, record := range records[1:] {
			if len(record) < 3 {
				failed++
				logrus.Warnf("Row %d: insufficient columns", i+2)
				continue
			}

			fullName := strings.TrimSpace(record[0])
			email := strings.TrimSpace(record[1])
			phoneNumber := strings.TrimSpace(record[2])

			// Basic validation
			if fullName == "" || email == "" || phoneNumber == "" {
				failed++
				logrus.Warnf("Row %d: missing required fields", i+2)
				continue
			}

			// Format phone number - remove non-digits
			re := regexp.MustCompile(`\D`)
			phoneNumber = re.ReplaceAllString(phoneNumber, "")

			// Ensure it starts with 62 (Indonesia)
			if !strings.HasPrefix(phoneNumber, "62") {
				if strings.HasPrefix(phoneNumber, "0") {
					phoneNumber = "62" + phoneNumber[1:]
				} else if strings.HasPrefix(phoneNumber, "8") {
					phoneNumber = "62" + phoneNumber
				} else {
					failed++
					logrus.Warnf("Row %d: invalid phone number format: %s", i+2, phoneNumber)
					continue
				}
			}

			// Validate phone with WhatsApp if available
			var validatedPhone string
			if whatsapp.Client != nil {
				waResp, err := whatsapp.Client.IsOnWhatsApp(ctx, &pb.IsOnWhatsAppRequest{
					PhoneNumbers: []string{phoneNumber},
				})

				if err != nil || !waResp.Success || len(waResp.Results) == 0 || !waResp.Results[0].IsRegistered {
					failed++
					logrus.Warnf("Row %d: phone number not registered on WhatsApp: %s", i+2, phoneNumber)
					continue
				}

				// Extract phone from JID (format: 628xxx@s.whatsapp.net)
				jidParts := strings.Split(waResp.Results[0].Jid, "@")
				if len(jidParts) > 0 {
					validatedPhone = jidParts[0]
				} else {
					validatedPhone = phoneNumber
				}
			} else {
				validatedPhone = phoneNumber
			}

			// Check if user exists by phone number
			var existingUser model.WAUsers
			err := db.Where("phone_number = ?", validatedPhone).First(&existingUser).Error

			user := model.WAUsers{
				FullName:    fullName,
				Email:       email,
				PhoneNumber: validatedPhone,
			}

			// Parse optional fields
			if len(record) > 3 && strings.TrimSpace(record[3]) != "" {
				user.UserType = model.WAUserType(strings.TrimSpace(record[3]))
			} else {
				user.UserType = model.CommonUser
			}

			if len(record) > 4 && strings.TrimSpace(record[4]) != "" {
				user.UserOf = model.WAUserOf(strings.TrimSpace(record[4]))
			} else {
				user.UserOf = model.CompanyEmployee
			}

			if len(record) > 5 && strings.TrimSpace(record[5]) != "" {
				user.AllowedChats = model.WAAllowedChatMode(strings.TrimSpace(record[5]))
			} else {
				user.AllowedChats = model.BothChat
			}

			if len(record) > 6 && strings.TrimSpace(record[6]) != "" {
				if quota, err := strconv.Atoi(strings.TrimSpace(record[6])); err == nil {
					user.MaxDailyQuota = quota
				}
			} else {
				user.MaxDailyQuota = 10
			}

			if len(record) > 7 && strings.TrimSpace(record[7]) != "" {
				user.AllowedToCall = strings.ToLower(strings.TrimSpace(record[7])) == "true"
			}

			if len(record) > 8 && strings.TrimSpace(record[8]) != "" {
				user.UseBot = strings.ToLower(strings.TrimSpace(record[8])) == "true"
			} else {
				user.UseBot = true
			}

			// Handle AllowedTypes as JSON
			if len(record) > 9 && strings.TrimSpace(record[9]) != "" {
				allowedTypesStr := strings.TrimSpace(record[9])
				// Convert pipe-separated or comma-separated to JSON array
				types := strings.Split(strings.ReplaceAll(allowedTypesStr, "|", ","), ",")
				for i := range types {
					types[i] = strings.TrimSpace(types[i])
				}
				if typesJSON, err := json.Marshal(types); err == nil {
					user.AllowedTypes = typesJSON
				} else {
					defaultTypes, _ := json.Marshal([]string{"text"})
					user.AllowedTypes = defaultTypes
				}
			} else {
				defaultTypes, _ := json.Marshal([]string{"text"})
				user.AllowedTypes = defaultTypes
			}

			if len(record) > 10 {
				user.Description = strings.TrimSpace(record[10])
			}

			if err == gorm.ErrRecordNotFound {
				// Create new user
				if err := db.Create(&user).Error; err != nil {
					failed++
					logrus.Warnf("Row %d: failed to create user: %v", i+2, err)
				} else {
					created++
				}
			} else {
				// Update existing user
				user.ID = existingUser.ID
				if err := db.Model(&existingUser).Updates(&user).Error; err != nil {
					failed++
					logrus.Warnf("Row %d: failed to update user: %v", i+2, err)
				} else {
					updated++
				}
			}
		}

		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"message": fmt.Sprintf("Import completed: %d created, %d updated, %d failed", created, updated, failed),
			"created": created,
			"updated": updated,
			"failed":  failed,
		})
	}
}

// GetWhatsAppProfilePicture godoc
// @Summary      Get WhatsApp Profile Picture
// @Description  Downloads and serves a WhatsApp profile picture as a proper image file
// @Tags         WhatsApp Web
// @Accept       json
// @Produce      image/jpeg,image/png,image/webp
// @Param        jid path string true "User JID"
// @Success      200  {file}   binary
// @Router       /api/v1/{access}/tab-whatsapp/profile-picture/{jid} [get]
func GetWhatsAppProfilePicture(c *gin.Context) {
	jid := c.Param("jid")

	// Check if gRPC client is available
	if whatsapp.Client == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"success": false,
			"message": "WhatsApp gRPC service not available",
		})
		return
	}

	// Call gRPC service to get profile picture
	ctx := c.Request.Context()
	resp, err := whatsapp.Client.GetProfilePicture(ctx, &pb.GetProfilePictureRequest{
		Jid: jid,
	})

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": fmt.Sprintf("Failed to get profile picture: %v", err),
		})
		return
	}

	if !resp.Success {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": resp.Message,
		})
		return
	}

	// Set content type header
	contentType := resp.ContentType
	if contentType == "" {
		contentType = "image/jpeg" // Default fallback
	}

	// Set headers for file download
	filename := fmt.Sprintf("profile-%s.jpg", strings.ReplaceAll(jid, "@", "-"))
	c.Header("Content-Type", contentType)
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))

	// Return the image data
	c.Data(http.StatusOK, contentType, resp.ImageData)
}
