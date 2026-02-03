package database

import (
	"fmt"
	"log"
	"service-platform/cmd/technical_assistance/fun"
	"service-platform/cmd/technical_assistance/model"
	"service-platform/cmd/technical_assistance/model/op_model"
	"time"

	"gorm.io/gorm"
)

func AutoMigrateWeb(db *gorm.DB) {

	// Run migrations
	if err := db.AutoMigrate(
		&model.Admin{},
		&model.AdminStatus{},
		&model.AdminPasswordChangeLog{},
		&model.Role{},
		&model.RolePrivilege{},
		&model.Feature{},
		&model.LogActivity{},
		&model.UploadedFiles{},

		// TA Model
		&op_model.WAMessage{},
		&op_model.TAHandledData{},
	); err != nil {
		log.Fatal(err)
	}

	var adminStatusCount int64
	db.Model(&model.AdminStatus{}).Count(&adminStatusCount)
	if adminStatusCount == 0 {
		var adminStatuses = []model.AdminStatus{
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
		db.Create(&adminStatuses)

		for _, adminStatus := range adminStatuses {
			// Access IDs after insert
			fmt.Println("Insert New smtp  with ID : ", adminStatus.ID)
		}
	}

	var adminCount int64
	db.Model(&model.Admin{}).Count(&adminCount)
	if adminCount == 0 {
		var admins = []model.Admin{
			{
				Fullname:  "admin",
				Username:  "admin",
				Phone:     "081234567890",
				Email:     "admin@swi.com",
				Password:  fun.GenerateSaltedPassword("P@ssw0rd123"),
				Type:      0,
				Role:      1,
				Status:    2,
				CreateBy:  0,
				UpdateBy:  0,
				LastLogin: time.Now(),
			},
			{
				Fullname:  "admin2",
				Username:  "admin2",
				Phone:     "081234567890",
				Email:     "admin2@swi.com",
				Password:  fun.GenerateSaltedPassword("P@ssw0rd123"),
				Type:      0,
				Role:      1,
				Status:    2,
				CreateBy:  0,
				UpdateBy:  0,
				LastLogin: time.Now(),
			},
			{
				Fullname:  "admin3",
				Username:  "admin3",
				Phone:     "081234567890",
				Email:     "admin3@swi.com",
				Password:  fun.GenerateSaltedPassword("P@ssw0rd123"),
				Type:      0,
				Role:      1,
				Status:    2,
				CreateBy:  0,
				UpdateBy:  0,
				LastLogin: time.Now(),
			},
		}

		// Perform batch insert
		db.Create(&admins)

		for _, admin := range admins {
			// Access IDs after insert
			fmt.Println("Insert New Admin  with ID : ", admin.ID)
		}
	}
	var adminPasswordChangelogCount int64
	db.Model(&model.AdminPasswordChangeLog{}).Count(&adminPasswordChangelogCount)
	if adminPasswordChangelogCount == 0 {
		var admin_password_changelogs []model.AdminPasswordChangeLog

		var admins []model.Admin
		db.Find(&admins)
		for _, admin := range admins {
			admin_password_changelogs = append(admin_password_changelogs, model.AdminPasswordChangeLog{Email: admin.Email, Password: admin.Password})
		}

		// Perform batch insert
		db.Create(&admin_password_changelogs)

		for _, admin_password_changelog := range admin_password_changelogs {
			// Access IDs after insert
			fmt.Println("Insert New admin_password_changelog  with ID : ", admin_password_changelog.ID)
		}
	}
	var roleCount int64
	db.Model(&model.Role{}).Count(&roleCount)
	if roleCount == 0 {
		roles := []model.Role{
			{
				RoleName:  "Super Admin",
				CreatedBy: 0,
			},
			{
				RoleName:  "Merchant",
				CreatedBy: 0,
			},
			{
				RoleName:  "Admin Issuer",
				CreatedBy: 0,
			},
			{
				RoleName:  "Admin Transaction",
				CreatedBy: 0,
			},
			{
				RoleName:  "Admin Merchant",
				CreatedBy: 0,
			},
			{
				RoleName:  "Admin Settlement",
				CreatedBy: 0,
			},
			{
				RoleName:  "Operator",
				CreatedBy: 0,
			},
		}

		// Perform batch insert
		db.Create(&roles)

		for _, role := range roles {
			// Access IDs after insert
			fmt.Println("Insert New Roles ID : ", role.ID)
		}
	}

	var featureCount int64
	db.Model(&model.Feature{}).Count(&featureCount)

	features := []model.Feature{
		{
			Title: "Teknisi Pending", Path: "tab-konfirmasi-data-pending", Icon: "bx bx-loader",
		},
		{
			Title: "Teknisi Error", Path: "tab-konfirmasi-data-error", Icon: "bx bx-bug-alt",
		},
		{
			Title: "Awaiting Submission", Path: "tab-konfirmasi-data-submission", Icon: "bx bxs-hourglass-top",
		},
		{
			Title: "Log Activity Team Technical Assistance", Path: "tab-log-act", Icon: "bx bx-list-check",
		},
		{
			Title: "System User & Roles", Path: "tab-roles", Icon: "bx bx-group",
		},
		{
			Title: "System Log", Path: "tab-system-log", Icon: "bx bx-terminal",
		},
		{
			Title: "Log Activity", Path: "tab-activity-log", Icon: "bx bx-universal-access",
		},
		{
			Title: "User Profile", Path: "tab-user-profile", Icon: "bx bx-user-circle",
		},
		{
			Title: "TA Report", Path: "tab-report", Icon: "bx bx-file",
		},
	}

	for i, f := range features {
		var existing model.Feature
		result := db.Where("title = ? AND path = ?", f.Title, f.Path).First(&existing)

		if result.Error != nil && result.Error == gorm.ErrRecordNotFound {
			// Not found → Insert new
			f.ParentID = 0
			f.MenuOrder = uint(i + 1)
			f.Status = 1
			f.Level = 0
			db.Create(&f)
			fmt.Println("Inserted Feature:", f.Title, "→ ID:", f.ID)
		} else {
			// Found → Update if needed
			existing.MenuOrder = uint(i + 1)
			existing.Icon = f.Icon
			db.Save(&existing)
			fmt.Println("Updated Feature:", existing.Title, "→ ID:", existing.ID)
		}
	}

	var roles []model.Role
	if err := db.Find(&roles).Error; err != nil {
		log.Fatal("Failed to fetch roles:", err)
	}

	var allFeatures []model.Feature
	if err := db.Find(&allFeatures).Error; err != nil {
		log.Fatal("Failed to fetch features:", err)
	}

	for _, role := range roles {
		// Get all feature_ids that already exist in privileges for this role
		var existingFeatureIDs []uint
		if err := db.Model(&model.RolePrivilege{}).
			Where("role_id = ?", role.ID).
			Pluck("feature_id", &existingFeatureIDs).Error; err != nil {
			log.Fatal("Failed to fetch existing privileges:", err)
		}

		// Convert to map for fast lookup
		existingMap := make(map[uint]bool)
		for _, fid := range existingFeatureIDs {
			existingMap[fid] = true
		}

		// Loop through all features and insert missing privileges
		for _, feature := range allFeatures {
			if !existingMap[feature.ID] {
				newPriv := model.RolePrivilege{
					RoleID:    role.ID,
					FeatureID: feature.ID,
					Create:    1,
					Read:      1,
					Update:    1,
					Delete:    1,
				}
				if err := db.Create(&newPriv).Error; err != nil {
					log.Printf("Failed to insert privilege for Role %d, Feature %d: %v", role.ID, feature.ID, err)
				} else {
					fmt.Printf("Inserted RolePrivilege → RoleID: %d, FeatureID: %d\n", role.ID, feature.ID)
				}
			}
		}
	}

}
