package database

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"service-platform/internal/config"
	"service-platform/internal/core/model"
	"service-platform/internal/pkg/fun"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type RolePermission struct {
	CanCreate int8
	CanRead   int8
	CanUpdate int8
	CanDelete int8
}

func seedRoles(db *gorm.DB) {
	var roleCount int64
	db.Model(&model.Role{}).Count(&roleCount)
	if roleCount == 0 {
		roles := []model.Role{
			{
				RoleName:  "Super User",
				CreatedBy: 0,
				Icon:      "fal fa-user-crown",
				ClassName: "bg-primary",
			},
			{
				RoleName:  "Client Company - User",
				CreatedBy: 0,
				Icon:      "fal fa-user",
				ClassName: "bg-label-success",
			},
			{
				RoleName:  "Client Company - Admin",
				CreatedBy: 0,
				Icon:      "fal fa-user-cog",
				ClassName: "bg-label-info",
			},
		}

		// Perform batch insert
		db.Create(&roles)

		for _, role := range roles {
			// Access IDs after insert
			logrus.Println("Insert New Roles ID : ", role.ID)
		}
	}
}

func seedFeature(db *gorm.DB) {
	var featureCount int64
	db.Model(&model.Feature{}).Count(&featureCount)
	if featureCount == 0 {
		var maxOrder uint
		db.Model(&model.Feature{}).Select("COALESCE(MAX(menu_order), 0)").Scan(&maxOrder)

		features := []model.Feature{
			{
				ParentID: 0,
				Title:    "Dashboard",
				Path:     "tab-dashboard",
				Status:   1,
				Level:    0,
				Icon:     "fad fa-tachometer-alt-fast",
			},
			/*
				Client Company
			*/
			// ADD: more client company pages & features
			// {
			// 	ParentID: 0,
			// 	Title:    "Hommy Pay",
			// 	Path:     "",
			// 	Status:   1,
			// 	Level:    0,
			// 	Icon:     "fad fa-money-check",
			// },
			// {
			// 	ParentID: 0,
			// 	Title:    "Data Ticket",
			// 	Path:     "tab-hommy-pay-cc-ticket",
			// 	Status:   1,
			// 	Level:    1,
			// 	// Level:    0,
			// 	Icon: "fad fa-ballot-check",
			// },
			// {
			// 	ParentID: 0,
			// 	Title:    "Merchant",
			// 	Path:     "tab-hommy-pay-cc-merchant",
			// 	Status:   1,
			// 	Level:    1,
			// 	// Level:    0,
			// 	Icon: "fad fa-store",
			// },
			/* * * * * * * * * * * * * * * * * * * * * * * * * * * */
			/*
				Whatsapp
			*/
			{
				ParentID: 0,
				Title:    "Whatsapp",
				Path:     "",
				Status:   1,
				Level:    0,
				Icon:     "fab fa-whatsapp",
			},
			{
				ParentID: 0,
				Title:    "Bot Whatsapp",
				Path:     "tab-whatsapp",
				Status:   1,
				Level:    1,
				// Level:    0,
				Icon: "fad fa-user-robot",
			},
			{
				ParentID: 0,
				Title:    "Whatsapp User Management",
				Path:     "tab-whatsapp-user-management",
				Status:   1,
				Level:    1,
				// Level:    0,
				Icon: "fad fa-user-cog",
			},
			// {
			// 	ParentID: 0,
			// 	Title:    "Chat & Messages",
			// 	Path:     "tab-whatsapp-conversation",
			// 	Status:   1,
			// 	Level:    1,
			// 	// Level:    0,
			// 	Icon: "fad fa-whatsapp",
			// },
			/* * * * * * * * * * * * * * * * * * * * * * * * * * * */
			{
				ParentID: 0,
				Title:    "Scheduler",
				Path:     "tab-scheduler",
				Status:   1,
				Level:    0,
				Icon:     "fad fa-alarm-clock",
			},
			/* * * * * * * * * * * * * * * * * * * * * * * * * * * */
			// {
			// 	ParentID: 0,
			// 	Title:    "Email",
			// 	Path:     "tab-email",
			// 	Status:   1,
			// 	Level:    0,
			// 	Icon:     "fad fa-envelope-open-text",
			// },
			{
				ParentID: 0,
				Title:    "App Configuration",
				Path:     "tab-app-config",
				Status:   1,
				Level:    0,
				Icon:     "fad fa-cogs",
			},
			{
				ParentID: 0,
				Title:    "System User & Roles",
				Path:     "tab-roles",
				Status:   1,
				Level:    0,
				Icon:     "fad fa-users",
			},
			{
				ParentID: 0,
				Title:    "System Log",
				Path:     "tab-system-log",
				Status:   1,
				Level:    0,
				Icon:     "fad fa-terminal",
			},
			{
				ParentID: 0,
				Title:    "Log Activity",
				Path:     "tab-activity-log",
				Status:   1,
				Level:    0,
				Icon:     "fad fa-money-check-edit",
			},
			{
				ParentID: 0,
				Title:    "User Profile",
				Path:     "tab-user-profile",
				Status:   1,
				Level:    0,
				Icon:     "fad fa-id-card-alt",
			},
		}

		for i := range features {
			maxOrder++
			features[i].MenuOrder = maxOrder
		}

		// Perform batch insert
		db.Create(&features)

		// Set parent-child relationships
		parents := []struct {
			Title         string
			ChildPrefixes []string
		}{
			{
				Title:         "Whatsapp",
				ChildPrefixes: []string{"tab-whatsapp"},
			},
			// {
			// 	Title:         "Client Company",
			// 	ChildPrefixes: []string{"tab-client_company"},
			// },
		}
		for _, p := range parents {
			var parent model.Feature
			if err := db.Where("title = ?", p.Title).First(&parent).Error; err != nil {
				logrus.Errorf("⚠️ Failed to find parent feature '%s': %v", p.Title, err)
				continue
			}

			for _, prefix := range p.ChildPrefixes {
				res := db.Model(&model.Feature{}).
					Where("path LIKE ?", prefix+"%").
					Update("parent_id", parent.ID)

				if res.Error != nil {
					logrus.Errorf("⚠️ Failed to update children for parent '%s' with prefix '%s': %v", p.Title, prefix, res.Error)
				} else {
					logrus.Infof("✅ Updated %d children for parent '%s' with prefix '%s'", res.RowsAffected, p.Title, prefix)
				}
			}
		}

		for _, feature := range features {
			logrus.Println("✅ Inserted DB Feature ID:", feature.ID, "| Menu Order:", feature.MenuOrder)
		}
	}
}

func seedRolePrivilege(db *gorm.DB) {
	var countData int64
	db.Model(&model.RolePrivilege{}).Count(&countData)
	if countData == 0 {
		var roleWebs []model.Role
		if result := db.Find(&roleWebs); result.Error != nil {
			logrus.Fatalf("Error while trying to fetch roles: %v", result.Error)
		}

		var features []model.Feature
		if result := db.Find(&features); result.Error != nil {
			logrus.Fatalf("Error while trying to fetch features: %v", result.Error)
		}

		var rolePrivileges []model.RolePrivilege

		for _, roleWeb := range roleWebs {
			for _, feature := range features {
				permission, hasAccess := getRolePermissions(roleWeb.RoleName, feature.Path)

				if hasAccess {
					rolePrivileges = append(rolePrivileges, model.RolePrivilege{
						RoleID:    roleWeb.ID,
						FeatureID: feature.ID,
						CanCreate: permission.CanCreate,
						CanRead:   permission.CanRead,
						CanUpdate: permission.CanUpdate,
						CanDelete: permission.CanDelete,
					})
				}
			}
		}

		// Perform batch insert
		db.Create(&rolePrivileges)
	}
}

func getRolePermissions(roleName, featurePath string) (RolePermission, bool) {
	roleNameLower := strings.ToLower(roleName)
	featurePathLower := strings.ToLower(featurePath)

	// Super User gets full access to everything
	if strings.Contains(roleNameLower, "super user") {
		return RolePermission{CanCreate: 1, CanRead: 1, CanUpdate: 1, CanDelete: 1}, true
	}

	// Client company roles only get access to their features and general features
	if strings.Contains(roleNameLower, "client_company_employee") {
		// Allow access to general features (dashboard, profile, etc.)
		if featurePath == "" ||
			// strings.HasPrefix(featurePathLower, "tab-dashboard") ||
			// strings.HasPrefix(featurePathLower, "tab-user-profile") ||
			strings.HasPrefix(featurePathLower, "tab-client_company_employee") {

			// Client Company Admin gets full CRUD access
			if strings.Contains(roleNameLower, "admin") {
				return RolePermission{CanCreate: 1, CanRead: 1, CanUpdate: 1, CanDelete: 1}, true
			}

			// Client Company Client gets read-only access
			if strings.Contains(roleNameLower, "user") {
				return RolePermission{CanCreate: 0, CanRead: 1, CanUpdate: 0, CanDelete: 0}, true
			}
		}
		return RolePermission{}, false // No access to other features
	}

	// Default role gets access to general features only
	if featurePath == "" ||
		strings.HasPrefix(featurePathLower, "tab-dashboard") {
		// strings.HasPrefix(featurePathLower, "tab-user-profile") {
		return RolePermission{CanCreate: 0, CanRead: 1, CanUpdate: 0, CanDelete: 0}, true
	}

	// No access to restricted features
	return RolePermission{}, false
}

func seedUser(db *gorm.DB) {
	var superUserRole model.Role
	if err := db.Where("role_name = ?", "Super User").First(&superUserRole).Error; err != nil {
		logrus.Fatalf("Error while trying to find 'Super User' role: %v", err)
	}

	// Client Company Roles
	var clientCompanyAdminRole model.Role
	if err := db.Where("role_name = ?", "Client Company - Admin").First(&clientCompanyAdminRole).Error; err != nil {
		logrus.Fatalf("Error while trying to find 'Client Company - Admin' role: %v", err)
	}
	var clientCompanyUserRole model.Role
	if err := db.Where("role_name = ?", "Client Company - User").First(&clientCompanyUserRole).Error; err != nil {
		logrus.Fatalf("Error while trying to find 'Client Company - User' role: %v", err)
	}

	var lastLogin *time.Time
	now := time.Now()
	lastLogin = &now

	var users = []model.Users{
		{
			Fullname:     "RM Developer",
			Username:     "rm_dev",
			Phone:        config.GetConfig().Default.SuperUserPhone,
			Email:        config.GetConfig().Default.SuperUserEmail,
			Password:     fun.GenerateSaltedPassword(config.GetConfig().Default.SuperUserPassword),
			Type:         0,
			Role:         int(superUserRole.ID),
			Status:       2,
			CreateBy:     0,
			UpdateBy:     0,
			LastLogin:    lastLogin,
			ProfileImage: "uploads/admin/1.jpg",
		},
		{
			Fullname:     "Admin Client Company",
			Username:     "client_company_admin",
			Phone:        "628111111111",
			Email:        "admin@clientcompany.com",
			Password:     fun.GenerateSaltedPassword("ClientCompanyAdmin123#"),
			Type:         0,
			Role:         int(clientCompanyAdminRole.ID),
			Status:       2,
			CreateBy:     1,
			UpdateBy:     0,
			LastLogin:    lastLogin,
			ProfileImage: "uploads/admin/client_company_admin.jpg",
		},
		{
			Fullname:     "Client Company - User",
			Username:     "client_company_user",
			Phone:        "628222222222",
			Email:        "client@clientcompany.com",
			Password:     fun.GenerateSaltedPassword("ClientCompanyUser123#"),
			Type:         0,
			Role:         int(clientCompanyUserRole.ID),
			Status:       2,
			CreateBy:     1,
			UpdateBy:     0,
			LastLogin:    lastLogin,
			ProfileImage: "uploads/admin/client_company_user.jpg",
		},
	}

	if len(users) > 0 {
		for _, user := range users {
			var existingUser model.Users
			if err := db.Where("email = ?", user.Email).First(&existingUser).Error; err != nil {
				if err == gorm.ErrRecordNotFound {
					// User with this email does not exist, create it
					if err := db.Create(&user).Error; err != nil {
						logrus.Printf("Error creating user with email %s: %v", user.Email, err)
					} else {
						logrus.Println("Inserted new user with email:", user.Email, "ID:", user.ID)
					}
				} else {
					logrus.Printf("Error checking user with email %s: %v", user.Email, err)
				}
			} else {
				// Skip updating existing user
			}
		}
	}
}

func seedUserStatus(db *gorm.DB) {
	var count int64
	db.Model(&model.UserStatus{}).Count(&count)
	if count == 0 {
		var userStatuses = []model.UserStatus{
			{
				ID:        1,
				Title:     "PENDING",
				ClassName: "badge bg-label-warning",
			},
			{
				ID:        2,
				Title:     "ACTIVE",
				ClassName: "badge bg-label-success",
			},
			{
				ID:        3,
				Title:     "INACTIVE",
				ClassName: "badge bg-label-secondary",
			},
		}

		// Perform batch insert
		db.Create(&userStatuses)

		for _, userStatus := range userStatuses {
			// Access IDs after insert
			logrus.Println("Insert New data with ID : ", userStatus.ID)
		}
	}
}

func seedUserPasswordChangeLog(db *gorm.DB) {
	var count int64
	db.Model(&model.UserPasswordChangeLog{}).Count(&count)
	if count == 0 {
		var userPwdChangeLogs []model.UserPasswordChangeLog

		var users []model.Users
		db.Find(&users)
		for _, user := range users {
			userPwdChangeLogs = append(userPwdChangeLogs, model.UserPasswordChangeLog{Email: user.Email, Password: user.Password})
		}

		// Perform batch insert
		db.Create(&userPwdChangeLogs)

		for _, userPwdChangeLog := range userPwdChangeLogs {
			// Access IDs after insert
			logrus.Println("Insert New user_password_changelog  with ID : ", userPwdChangeLog.ID)
		}
	}
}

func seedIndonesiaRegion(db *gorm.DB) {
	tableName := config.GetConfig().Database.TbIndonesiaRegion

	// Check if table exists
	if !tableExists(db, tableName) {
		logrus.Infof("Table '%s' does not exist. Creating table structure first...", tableName)

		// Step 1: Create the table structure using GORM AutoMigrate
		if err := createIndonesiaRegionTable(db); err != nil {
			logrus.Errorf("Failed to create table structure for indonesia region: %v", err)
			return
		}

		logrus.Infof("✅ Table structure for '%s' created successfully", tableName)
	}

	var count int64
	if err := db.Table(tableName).Count(&count).Error; err != nil {
		logrus.Errorf("Failed to count records in table '%s': %v", tableName, err)
		return
	}

	if count == 0 {
		logrus.Infof("Importing data from SQL file into table '%s'...", tableName)

		sqlDumpedFile := config.GetConfig().Database.DumpedIndonesiaRegionSQL
		if _, err := os.Stat(sqlDumpedFile); os.IsNotExist(err) {
			sqlDumpedFile += "internal/" + sqlDumpedFile
		}

		err := importIndonesiaRegionData(db, sqlDumpedFile)
		if err != nil {
			logrus.Errorf("Failed to import data for indonesia region: %v", err)
		} else {
			logrus.Infof("✅ Successfully imported data into table '%s'", tableName)
		}
	} else {
		// logrus.Infof("Table '%s' already contains %d records, skipping data import", tableName, count)
	}
}

func createIndonesiaRegionTable(db *gorm.DB) error {
	// Create the table with custom table name from config
	tableName := config.GetConfig().Database.TbIndonesiaRegion

	// Set custom table name for migration
	err := db.Table(tableName).AutoMigrate(&model.IndonesiaRegion{})
	if err != nil {
		return fmt.Errorf("failed to create table structure: %v", err)
	}

	return nil
}

func seedWhatsappUser(db *gorm.DB) {
	var count int64
	db.Model(&model.WAUsers{}).Count(&count)

	if count == 0 {
		allowedMsgTypes := model.AllWAMessageTypes
		data, err := json.Marshal(allowedMsgTypes)
		if err != nil {
			logrus.Fatalf("Error marshaling allowed message types: %v", err)
		}
		allowedTypes := datatypes.JSON(data)

		users := []model.WAUsers{
			{
				FullName:      "RM Developer",
				Email:         config.GetConfig().Default.SuperUserEmail,
				PhoneNumber:   config.GetConfig().Default.SuperUserPhone,
				IsRegistered:  true,
				AllowedChats:  model.BothChat,
				AllowedTypes:  allowedTypes,
				AllowedToCall: true,
				MaxDailyQuota: 250,
				Description:   "Super user WA account",
				IsBanned:      false,
				UserType:      model.SuperUser,
				UserOf:        model.CompanyEmployee,
			},
		}

		for _, user := range users {
			err := db.Create(&user).Error
			if err != nil {
				logrus.Fatalf("Error creating WA user %s: %v", user.Email, err)
			} else {
				logrus.Printf("📱 Successfully inserted WA User %s - %s", user.FullName, user.PhoneNumber)
			}
		}
	}
}

func seedWhatsappLanguage(db *gorm.DB) {
	// WhatsApp Bot Language
	var languageCount int64
	db.Model(&model.Language{}).Count(&languageCount)
	if languageCount == 0 {
		var languages []model.Language
		// Use LanguageNameMap from fun package
		for code, name := range fun.LanguageNameMap {
			languages = append(languages, model.Language{
				Name: name,
				Code: code,
			})
		}

		db.Create(&languages)

		for _, language := range languages {
			logrus.Println("🏳 Insert New Language with ID:", language.ID)
		}
	}
}

func seedBadWords(db *gorm.DB) {
	var count int64
	db.Model(&model.BadWord{}).Count(&count)

	if count == 0 {
		// Define bad words grouped by language code
		badWordsMap := map[string][]struct {
			Word     string
			Category model.BadWordCategory
		}{
			fun.LangID: {
				{"anjing", model.CategoryUmpatan},
				{"bajingan", model.CategoryUmpatan},
				{"bangsat", model.CategoryUmpatan},
				{"jancok", model.CategoryUmpatan},
				{"jancuk", model.CategoryUmpatan},
				{"kontol", model.CategorySexual},
				{"memek", model.CategorySexual},
				{"ngentot", model.CategorySexual},
				{"goblok", model.CategoryGeneral},
				{"tolol", model.CategoryGeneral},
				{"bodoh", model.CategoryGeneral},
				{"tai", model.CategoryGeneral},
				{"setan", model.CategoryUmpatan},
				{"babi", model.CategoryUmpatan},
				{"kampret", model.CategoryUmpatan},
				{"puki", model.CategoryUmpatan},
				{"cukimai", model.CategoryUmpatan},
				{"telaso", model.CategoryUmpatan},
				{"tailaso", model.CategoryUmpatan},
				{"lonte", model.CategorySexual},
				{"pelacur", model.CategorySexual},
			},
			fun.LangEN: {
				{"fuck", model.CategoryUmpatan},
				{"fucking", model.CategoryUmpatan},
				{"bitch", model.CategoryUmpatan},
				{"asshole", model.CategoryUmpatan},
				{"bastard", model.CategoryUmpatan},
				{"idiot", model.CategoryGeneral},
				{"stupid", model.CategoryGeneral},
				{"dumb", model.CategoryGeneral},
				{"nigger", model.CategoryRasis},
				{"nigga", model.CategoryRasis},
				{"retard", model.CategoryGeneral},
				{"dick", model.CategorySexual},
				{"pussy", model.CategorySexual},
				{"cunt", model.CategorySexual},
				{"whore", model.CategorySexual},
				{"slut", model.CategorySexual},
			},
			fun.LangES: {
				{"puta", model.CategoryUmpatan},
				{"puto", model.CategoryUmpatan},
				{"mierda", model.CategoryUmpatan},
				{"cabron", model.CategoryUmpatan},
				{"joder", model.CategoryUmpatan},
				{"gilipollas", model.CategoryGeneral},
				{"idiota", model.CategoryGeneral},
				{"pendejo", model.CategoryUmpatan},
				{"coño", model.CategorySexual},
			},
			fun.LangFR: {
				{"merde", model.CategoryUmpatan},
				{"putain", model.CategoryUmpatan},
				{"connard", model.CategoryUmpatan},
				{"salope", model.CategoryUmpatan},
				{"encule", model.CategorySexual},
				{"idiot", model.CategoryGeneral},
			},
			fun.LangDE: {
				{"scheisse", model.CategoryUmpatan},
				{"arschloch", model.CategoryUmpatan},
				{"schlampe", model.CategoryUmpatan},
				{"hurensohn", model.CategoryUmpatan},
				{"idiot", model.CategoryGeneral},
			},
			fun.LangPT: {
				{"merda", model.CategoryUmpatan},
				{"porra", model.CategoryUmpatan},
				{"caralho", model.CategoryUmpatan},
				{"puta", model.CategoryUmpatan},
				{"idiota", model.CategoryGeneral},
			},
			fun.LangRU: {
				{"сука", model.CategoryUmpatan},
				{"блять", model.CategoryUmpatan},
				{"мудак", model.CategoryUmpatan},
				{"идиот", model.CategoryGeneral},
				{"дебил", model.CategoryGeneral},
			},

			fun.LangJP: {
				{"馬鹿", model.CategoryGeneral}, // baka
				{"阿呆", model.CategoryGeneral}, // aho
				{"死ね", model.CategoryUmpatan}, // shine
				{"くそ", model.CategoryUmpatan}, // kuso
			},
			fun.LangCN: {
				{"傻逼", model.CategoryUmpatan},
				{"笨蛋", model.CategoryGeneral},
				{"操", model.CategorySexual},
				{"肏", model.CategorySexual},
				{"狗屎", model.CategoryUmpatan},
			},
			fun.LangAR: {
				{"كلب", model.CategoryUmpatan},
				{"حيوان", model.CategoryUmpatan},
				{"غبي", model.CategoryGeneral},
				{"حقير", model.CategoryUmpatan},
			},
		}

		var badWords []model.BadWord

		for code, words := range badWordsMap {
			// Search language in DB
			var lang model.Language
			if err := db.Where("code = ?", code).First(&lang).Error; err != nil {
				logrus.Warnf("Language code '%s' not found in DB, skipping bad words for it.", code)
				continue
			}

			for _, w := range words {
				badWords = append(badWords, model.BadWord{
					Word:      w.Word,
					Language:  lang.Code, // Use code from DB
					Category:  w.Category,
					IsEnabled: true,
				})
			}
		}

		if len(badWords) > 0 {
			if err := db.Create(&badWords).Error; err != nil {
				logrus.Error("Failed to seed bad words: ", err)
			} else {
				logrus.Info("✅ Seeded bad words successfully")
			}
		}
	}
}

func seedWhatsAppMsgAutoReply(db *gorm.DB) {
	var count int64
	db.Model(&model.WhatsappMessageAutoReply{}).Count(&count)

	dataSeparator := config.GetConfig().Default.DataSeparator
	if dataSeparator == "" {
		dataSeparator = "|"
	}

	if count == 0 {
		// Define seeds for each language
		seeds := map[string][]struct {
			Keywords  []string
			ReplyText string
		}{
			fun.LangID: {
				{Keywords: []string{"halo", "hai", "pagi", "siang", "sore", "malam"}, ReplyText: "Halo! Ada yang bisa kami bantu?"},
				{Keywords: []string{"bantuan", "help", "tolong"}, ReplyText: "Silakan jelaskan masalah Anda, kami akan segera membantu."},
				{Keywords: []string{"harga", "biaya"}, ReplyText: "Untuk informasi harga, silakan kunjungi website kami atau hubungi sales kami."},
			},
			fun.LangEN: {
				{Keywords: []string{"hello", "hi", "hey", "morning", "afternoon", "evening"}, ReplyText: "Hello! How can we help you?"},
				{Keywords: []string{"help", "support", "assist"}, ReplyText: "Please describe your issue, we will help you shortly."},
				{Keywords: []string{"price", "cost", "pricing"}, ReplyText: "For pricing information, please visit our website or contact our sales team."},
			},
			fun.LangES: {
				{Keywords: []string{"hola", "buenos dias", "buenas tardes", "buenas noches"}, ReplyText: "¡Hola! ¿En qué podemos ayudarte?"},
				{Keywords: []string{"ayuda", "soporte", "asistencia"}, ReplyText: "Por favor describe tu problema, te ayudaremos en breve."},
			},
			fun.LangFR: {
				{Keywords: []string{"bonjour", "salut", "bonsoir"}, ReplyText: "Bonjour! Comment pouvons-nous vous aider?"},
				{Keywords: []string{"aide", "support", "assistance"}, ReplyText: "Veuillez décrire votre problème, nous vous aiderons sous peu."},
			},
			fun.LangDE: {
				{Keywords: []string{"hallo", "guten morgen", "guten tag", "guten abend"}, ReplyText: "Hallo! Wie können wir Ihnen helfen?"},
				{Keywords: []string{"hilfe", "support", "unterstützung"}, ReplyText: "Bitte beschreiben Sie Ihr Problem, wir werden Ihnen in Kürze helfen."},
			},
			fun.LangPT: {
				{Keywords: []string{"olá", "oi", "bom dia", "boa tarde", "boa noite"}, ReplyText: "Olá! Como podemos ajudar?"},
				{Keywords: []string{"ajuda", "suporte", "assistência"}, ReplyText: "Por favor, descreva seu problema, ajudaremos em breve."},
			},
			fun.LangRU: {
				{Keywords: []string{"привет", "здравствуйте", "доброе утро", "добрый день", "добрый вечер"}, ReplyText: "Здравствуйте! Чем мы можем вам помочь?"},
				{Keywords: []string{"помощь", "поддержка"}, ReplyText: "Пожалуйста, опишите вашу проблему, мы скоро вам поможем."},
			},
			fun.LangJP: {
				{Keywords: []string{"こんにちは", "おはよう", "こんばんは"}, ReplyText: "こんにちは！どのようにお手伝いできますか？"},
				{Keywords: []string{"助けて", "サポート", "ヘルプ"}, ReplyText: "問題を説明してください。すぐにサポートいたします。"},
			},
			fun.LangCN: {
				{Keywords: []string{"你好", "早安", "午安", "晚安"}, ReplyText: "你好！有什么我们可以帮你的吗？"},
				{Keywords: []string{"帮助", "支持", "协助"}, ReplyText: "请描述您的问题，我们将尽快为您提供帮助。"},
			},
			fun.LangAR: {
				{Keywords: []string{"مرحبا", "صباح الخير", "مساء الخير"}, ReplyText: "مرحبا! كيف يمكننا مساعدتك؟"},
				{Keywords: []string{"مساعدة", "دعم"}, ReplyText: "يرجى وصف مشكلتك، وسنساعدك قريبا."},
			},
		}

		for langCode, replies := range seeds {
			var langID uint
			if err := db.Model(&model.Language{}).Where("code = ?", langCode).Select("id").Scan(&langID).Error; err != nil {
				logrus.Errorf("Error while trying to fetch language ID for code '%s': %v", langCode, err)
				continue
			}

			if langID == 0 {
				logrus.Warnf("Language '%s' not found in DB, skipping auto-replies.", langCode)
				continue
			}

			var autoReplies []model.WhatsappMessageAutoReply
			for _, r := range replies {
				// Join keywords with the configured separator
				keywords := strings.Join(r.Keywords, dataSeparator)

				autoReplies = append(autoReplies, model.WhatsappMessageAutoReply{
					LanguageID:  langID,
					Keywords:    keywords,
					ReplyText:   r.ReplyText,
					ForUserType: string(model.CommonUser),
					UserOf:      string(model.CompanyEmployee),
				})
			}

			if len(autoReplies) > 0 {
				if err := db.Create(&autoReplies).Error; err != nil {
					logrus.Errorf("Failed to seed WhatsApp message auto-reply for %s: %v", langCode, err)
				} else {
					logrus.Infof("✅ Seeded WhatsApp message auto-reply successfully for language '%s'", langCode)
				}
			}
		}
	}
}

func seedAppConfig(db *gorm.DB) {
	var count int64
	db.Model(&model.AppConfig{}).Count(&count)

	if count == 0 {
		var superUserRole model.Role
		if err := db.Where("role_name = ?", "Super User").First(&superUserRole).Error; err != nil {
			logrus.Errorf("Error while trying to fetch Super User role: %v", err)
			return
		}

		var clientCompanyUserRole model.Role
		if err := db.Where("role_name = ?", "Client Company - User").First(&clientCompanyUserRole).Error; err != nil {
			logrus.Errorf("Error while trying to fetch Client Company - User role: %v", err)
			return
		}

		var clientCompanyAdminRole model.Role
		if err := db.Where("role_name = ?", "Client Company - Admin").First(&clientCompanyAdminRole).Error; err != nil {
			logrus.Errorf("Error while trying to fetch Client Company - Admin role: %v", err)
			return
		}

		appConfigs := []model.AppConfig{
			{
				RoleID:      superUserRole.ID,
				AppName:     "Developer App",
				AppLogo:     "/assets/self/img/logo.png",
				AppVersion:  "Debug",
				VersionNo:   "1",
				VersionCode: fmt.Sprintf("0.0.0.1.%v", time.Now().Format("2006.01.02")),
				VersionName: "rm_dev",
				IsActive:    true,
				Description: "Developer App for testing and development purposes. Not intended for production use.",
			},
			{
				RoleID:      clientCompanyUserRole.ID,
				AppName:     "Client Company",
				AppLogo:     "/assets/self/img/client_company.png",
				AppVersion:  "Release",
				VersionNo:   "Release",
				VersionCode: fmt.Sprintf("1.0.0.0.%v", time.Now().Format("2006.01.02")),
				VersionName: "prod",
				IsActive:    true,
				Description: "Client Company App for end-users to access Client Company services and features. Designed for clients to interact with the Client Company platform.",
			},
			{
				RoleID:      clientCompanyAdminRole.ID,
				AppName:     "Client Company",
				AppLogo:     "/assets/self/img/client_company.png",
				AppVersion:  "Release",
				VersionNo:   "Release",
				VersionCode: fmt.Sprintf("1.0.0.0.%v", time.Now().Format("2006.01.02")),
				VersionName: "prod",
				IsActive:    true,
				Description: "Client Company Admin App for administrators to manage and oversee Client Company platform operations. Provides tools and interfaces for effective administration.",
			},
		}

		if err := db.Create(&appConfigs).Error; err != nil {
			logrus.Error("Failed to seed app configs: ", err)
		} else {
			logrus.Info("✅ Seeded app configs successfully")
		}
	}
}

func importIndonesiaRegionData(db *gorm.DB, filePath string) error {
	// Get the absolute path
	absPath, err := filepath.Abs(filePath)
	if err != nil {
		return fmt.Errorf("failed to get absolute path: %v", err)
	}

	// Check if file exists
	if _, err := os.Stat(absPath); os.IsNotExist(err) {
		return fmt.Errorf("SQL file does not exist: %s", absPath)
	}

	// Read the SQL file
	file, err := os.Open(absPath)
	if err != nil {
		return fmt.Errorf("failed to open SQL file: %v", err)
	}
	defer file.Close()

	// Read all content
	content, err := io.ReadAll(file)
	if err != nil {
		return fmt.Errorf("failed to read SQL file: %v", err)
	}

	// Get table name from config
	tableName := config.GetConfig().Database.TbIndonesiaRegion

	// Split the content by semicolons to get individual SQL statements
	sqlStatements := strings.Split(string(content), ";")

	// Execute only INSERT statements
	for _, statement := range sqlStatements {
		statement = strings.TrimSpace(statement)
		if statement == "" {
			continue
		}

		// Skip comments and MySQL-specific commands
		if strings.HasPrefix(statement, "/*") ||
			strings.HasPrefix(statement, "--") ||
			strings.HasPrefix(statement, "/*!") {
			continue
		}

		// Skip CREATE TABLE statements (we already created the table)
		if strings.Contains(strings.ToUpper(statement), "CREATE TABLE") {
			continue
		}

		// Only process INSERT statements
		if strings.HasPrefix(strings.ToUpper(strings.TrimSpace(statement)), "INSERT") {
			// Replace the table name in INSERT statement to use config table name
			// For PostgreSQL, remove backticks (MySQL syntax) and use table name directly
			statement = strings.ReplaceAll(statement, "`indonesia_region`", tableName)
			statement = strings.ReplaceAll(statement, "indonesia_region", tableName)
			// Remove all remaining backticks (for column names)
			statement = strings.ReplaceAll(statement, "`", "")
			// Convert MySQL escaped quotes to PostgreSQL format
			statement = strings.ReplaceAll(statement, "\\'", "''")

			if err := db.Exec(statement).Error; err != nil {
				logrus.Warnf("Warning executing INSERT statement: %v", err)
				// Continue with other inserts even if one fails
			}
		}
	}

	return nil
}
