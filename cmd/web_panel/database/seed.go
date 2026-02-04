package database

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"service-platform/cmd/web_panel/fun"
	"service-platform/cmd/web_panel/model"
	contracttechnicianmodel "service-platform/cmd/web_panel/model/contract_technician_model"
	sptechnicianmodel "service-platform/cmd/web_panel/model/sp_technician_model"
	"service-platform/internal/config"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// RolePermission represents the permission level for a role
type RolePermission struct {
	Create int8
	Read   int8
	Update int8
	Delete int8
}

func seedAdminStatus(db *gorm.DB) {
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
			logrus.Println("Insert New data  with ID : ", adminStatus.ID)
		}
	}
}

func seedAdmin(db *gorm.DB) {
	// Find role IDs dynamically
	var superAdminRole model.Role
	if err := db.Where("role_name = ?", "Super Admin").First(&superAdminRole).Error; err != nil {
		logrus.Fatalf("Error while trying to fetch Super Admin role: %v", err)
	}

	var hommyPayClientRole model.Role
	if err := db.Where("role_name = ?", "Hommy Pay - Client").First(&hommyPayClientRole).Error; err != nil {
		logrus.Fatalf("Error while trying to fetch Hommy Pay - Client role: %v", err)
	}

	var hommyPayAdminRole model.Role
	if err := db.Where("role_name = ?", "Hommy Pay - Admin").First(&hommyPayAdminRole).Error; err != nil {
		logrus.Fatalf("Error while trying to fetch Hommy Pay - Admin role: %v", err)
	}

	var odooMSStaffRole model.Role
	if err := db.Where("role_name = ?", "ODOO MS - Staff").First(&odooMSStaffRole).Error; err != nil {
		logrus.Fatalf("Error while trying to fetch ODOO MS - Staff role: %v", err)
	}

	var csnaHRRole model.Role
	if err := db.Where("role_name = ?", "CSNA - Human Resource").First(&csnaHRRole).Error; err != nil {
		logrus.Fatalf("Error while trying to fetch CSNA - Human Resource role: %v", err)
	}

	var mtiClientRole model.Role
	if err := db.Where("role_name = ?", "MTI - Client").First(&mtiClientRole).Error; err != nil {
		logrus.Fatalf("Error while trying to fetch MTI - Client role: %v", err)
	}

	var dkiClientRole model.Role
	if err := db.Where("role_name = ?", "DKI - Client").First(&dkiClientRole).Error; err != nil {
		logrus.Fatalf("Error while trying to fetch DKI - Client role: %v", err)
	}

	var dspClientRole model.Role
	if err := db.Where("role_name = ?", "DSP - Client").First(&dspClientRole).Error; err != nil {
		logrus.Fatalf("Error while trying to fetch DSP - Client role: %v", err)
	}

	var bniClientRole model.Role
	if err := db.Where("role_name = ?", "BNI - Client").First(&bniClientRole).Error; err != nil {
		logrus.Fatalf("Error while trying to fetch BNI - Client role: %v", err)
	}

	var admins = []model.Admin{
		{
			Fullname:     "RM Developer",
			Username:     "rm_dev",
			Phone:        config.WebPanel.Get().Whatsmeow.WaSuperUser,
			Email:        "admin@webpanel.com",
			Password:     fun.GenerateSaltedPassword("Ro224171222#"),
			Type:         0,
			Role:         int(superAdminRole.ID),
			Status:       2,
			CreateBy:     0,
			UpdateBy:     0,
			LastLogin:    time.Now(),
			ProfileImage: "uploads/admin/1.jpg",
		},
		{
			Fullname:  "admin2",
			Username:  "admin2",
			Phone:     config.WebPanel.Get().Whatsmeow.WaSupport,
			Email:     "admin2@swi.com",
			Password:  fun.GenerateSaltedPassword("Ro224171222#"),
			Type:      0,
			Role:      int(superAdminRole.ID),
			Status:    2,
			CreateBy:  0,
			UpdateBy:  0,
			LastLogin: time.Now(),
		},
		{
			Fullname:     "Hommy Pay Client Test",
			Username:     "hommypay_client",
			Phone:        "6281111111111",
			Email:        "client@hommypay.com",
			Password:     fun.GenerateSaltedPassword("HommyPayClient123#"),
			Type:         0,
			Role:         int(hommyPayClientRole.ID),
			Status:       2,
			CreateBy:     0,
			UpdateBy:     0,
			LastLogin:    time.Now(),
			ProfileImage: "uploads/admin/client.jpg",
		},
		{
			Fullname:     "Hommy Pay Admin Test",
			Username:     "hommypay_admin",
			Phone:        "6281222222222",
			Email:        "admin@hommypay.com",
			Password:     fun.GenerateSaltedPassword("HommyPayAdmin123#"),
			Type:         0,
			Role:         int(hommyPayAdminRole.ID),
			Status:       2,
			CreateBy:     0,
			UpdateBy:     0,
			LastLogin:    time.Now(),
			ProfileImage: "uploads/admin/admin.png",
		},
		{
			Fullname:     "Admin Rawamangun Test",
			Username:     "ipal",
			Phone:        config.WebPanel.Get().Whatsmeow.WaTechnicalSupport,
			Email:        "desta@smartwebindonesia.com",
			Password:     fun.GenerateSaltedPassword("OdooMSStaff123#"),
			Type:         0,
			Role:         int(odooMSStaffRole.ID),
			Status:       2,
			CreateBy:     0,
			UpdateBy:     0,
			LastLogin:    time.Now(),
			ProfileImage: "uploads/admin/odoo.png",
		},
		{
			Fullname:     "CSNA Human Resource",
			Username:     "csna_hr",
			Phone:        "6281333333333",
			Email:        "hr@csna4u.com",
			Password:     fun.GenerateSaltedPassword("CsnaHR123#"),
			Type:         0,
			Role:         int(csnaHRRole.ID),
			Status:       2,
			CreateBy:     0,
			UpdateBy:     0,
			LastLogin:    time.Now(),
			ProfileImage: "uploads/admin/hr.png",
		},
		{
			Fullname:     "MTI Client Test",
			Username:     "mti_client",
			Phone:        "6281444444444",
			Email:        "client@mti.com",
			Password:     fun.GenerateSaltedPassword("MTIClient123#"),
			Type:         0,
			Role:         int(mtiClientRole.ID),
			Status:       2,
			CreateBy:     0,
			UpdateBy:     0,
			LastLogin:    time.Now(),
			ProfileImage: "uploads/admin/yokke.png",
		},
		{
			Fullname:     "DKI Client Test",
			Username:     "dki_client",
			Phone:        "6281555555555",
			Email:        "client@dki.com",
			Password:     fun.GenerateSaltedPassword("DKIClient123#"),
			Type:         0,
			Role:         int(dkiClientRole.ID),
			Status:       2,
			CreateBy:     0,
			UpdateBy:     0,
			LastLogin:    time.Now(),
			ProfileImage: "uploads/admin/dki.png",
		},
		{
			Fullname:     "DSP Client Test",
			Username:     "dsp_client",
			Phone:        "62822666666666",
			Email:        "client@dsp.com",
			Password:     fun.GenerateSaltedPassword("DSPClient123#"),
			Type:         0,
			Role:         int(dspClientRole.ID),
			Status:       2,
			CreateBy:     0,
			UpdateBy:     0,
			LastLogin:    time.Now(),
			ProfileImage: "uploads/admin/dsp.png",
		},
		{
			Fullname:     "BNI Client Test",
			Username:     "bni_client",
			Phone:        "6287777777777",
			Email:        "client@bni.com",
			Password:     fun.GenerateSaltedPassword("BNIClient123#"),
			Type:         0,
			Role:         int(bniClientRole.ID),
			Status:       2,
			CreateBy:     0,
			UpdateBy:     0,
			LastLogin:    time.Now(),
			ProfileImage: "uploads/admin/bni.png",
		},
	}

	for _, admin := range admins {
		var existingAdmin model.Admin
		if err := db.Where("email = ?", admin.Email).First(&existingAdmin).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				// Admin with this email does not exist, create it
				if err := db.Create(&admin).Error; err != nil {
					logrus.Printf("Error creating admin with email %s: %v", admin.Email, err)
				} else {
					logrus.Println("Inserted new admin with email:", admin.Email, "ID:", admin.ID)
				}
			} else {
				logrus.Printf("Error checking admin with email %s: %v", admin.Email, err)
			}
		} else {
			// logrus.Println("Admin with email", admin.Email, "already exists, skipping")
		}
	}
}

func seedAdminChangePwdLog(db *gorm.DB) {
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
			logrus.Println("Insert New admin_password_changelog  with ID : ", admin_password_changelog.ID)
		}
	}
}

func seedRoles(db *gorm.DB) {
	var roleCount int64
	db.Model(&model.Role{}).Count(&roleCount)
	if roleCount == 0 {
		roles := []model.Role{
			{
				RoleName:  "Super Admin",
				CreatedBy: 0,
				Icon:      "fal fa-user-crown",
				ClassName: "bg-label-primary",
			},
			{
				RoleName:  "Hommy Pay - Client",
				CreatedBy: 0,
				Icon:      "fal fa-university",
				ClassName: "bg-label-success",
			},
			{
				RoleName:  "Hommy Pay - Admin",
				CreatedBy: 0,
				Icon:      "fal fa-user-cog",
				ClassName: "bg-label-info",
			},
			{
				RoleName:  "ODOO MS - Staff",
				CreatedBy: 0,
				Icon:      "fal fa-server",
				ClassName: "bg-label-warning",
			},
			{
				RoleName:  "ODOO MS - Head",
				CreatedBy: 0,
				Icon:      "fal fa-user-tie",
				ClassName: "bg-label-danger",
			},
			{
				RoleName:  "CSNA - Human Resource",
				CreatedBy: 0,
				Icon:      "fal fa-user-hard-hat",
				ClassName: "bg-label-secondary",
			},
			{
				RoleName:  "MTI - Client",
				CreatedBy: 0,
				Icon:      "fal fa-handshake",
				ClassName: "bg-label-success",
			},
			{
				RoleName:  "DKI - Client",
				CreatedBy: 0,
				Icon:      "fal fa-handshake",
				ClassName: "bg-label-success",
			},
			{
				RoleName:  "DSP - Client",
				CreatedBy: 0,
				Icon:      "fal fa-handshake",
				ClassName: "bg-label-success",
			},
			{
				RoleName:  "BNI - Client",
				CreatedBy: 0,
				Icon:      "fal fa-handshake",
				ClassName: "bg-label-success",
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

// getRolePermissions determines the permissions for a role based on its name and feature path
func getRolePermissions(roleName, featurePath string) (RolePermission, bool) {
	roleNameLower := strings.ToLower(roleName)
	featurePathLower := strings.ToLower(featurePath)

	// Super Admin gets full access to everything
	if strings.Contains(roleNameLower, "super admin") {
		return RolePermission{Create: 1, Read: 1, Update: 1, Delete: 1}, true
	}

	// CSNA - Human Resource role explicit access to tab-hr
	if strings.Contains(roleNameLower, "csna") && strings.Contains(roleNameLower, "human resource") {
		// Allow access to general features (dashboard, profile, etc.)
		if featurePath == "" ||
			// strings.HasPrefix(featurePathLower, "tab-wp-dashboard") ||
			// strings.HasPrefix(featurePathLower, "tab-user-profile") ||
			strings.HasPrefix(featurePathLower, "tab-hr") {
			return RolePermission{Create: 1, Read: 1, Update: 1, Delete: 1}, true
		}
		return RolePermission{}, false // No access to other features
	}

	// Hommy Pay roles only get access to their features and general features
	if strings.Contains(roleNameLower, "hommy pay") {
		// Allow access to general features (dashboard, profile, etc.)
		if featurePath == "" ||
			// strings.HasPrefix(featurePathLower, "tab-wp-dashboard") ||
			// strings.HasPrefix(featurePathLower, "tab-user-profile") ||
			strings.HasPrefix(featurePathLower, "tab-hommy-pay") {

			// Hommy Pay Admin gets full CRUD access
			if strings.Contains(roleNameLower, "admin") {
				return RolePermission{Create: 1, Read: 1, Update: 1, Delete: 1}, true
			}

			// Hommy Pay Client gets read-only access
			if strings.Contains(roleNameLower, "client") {
				return RolePermission{Create: 0, Read: 1, Update: 0, Delete: 0}, true
			}
		}
		return RolePermission{}, false // No access to other features
	}

	// ODOO MS roles get access to their features and general features
	if strings.Contains(roleNameLower, "odoo ms") {
		// Allow access to general features (dashboard, profile, etc.) and ODOO MS features
		if featurePath == "" ||
			// strings.HasPrefix(featurePathLower, "tab-wp-dashboard") ||
			// strings.HasPrefix(featurePathLower, "tab-user-profile") ||
			strings.HasPrefix(featurePathLower, "tab-odoo-ms") {

			// ODOO MS Head gets full CRUD access
			if strings.Contains(roleNameLower, "head") {
				return RolePermission{Create: 1, Read: 1, Update: 1, Delete: 1}, true
			}

			// ODOO MS Staff gets read and update access
			if strings.Contains(roleNameLower, "staff") {
				return RolePermission{Create: 0, Read: 1, Update: 1, Delete: 0}, true
			}
		}
		return RolePermission{}, false // No access to other features
	}

	// MTI roles only get access to their features and general features
	if strings.Contains(roleNameLower, "mti") {
		// Allow access to general features (dashboard, profile, etc.)
		if featurePath == "" ||
			// strings.HasPrefix(featurePathLower, "tab-wp-dashboard") ||
			// strings.HasPrefix(featurePathLower, "tab-user-profile") ||
			strings.HasPrefix(featurePathLower, "tab-mti") {

			// MTI Admin gets full CRUD access
			if strings.Contains(roleNameLower, "admin") {
				return RolePermission{Create: 1, Read: 1, Update: 1, Delete: 1}, true
			}

			// MTI Client gets read-only access
			if strings.Contains(roleNameLower, "client") {
				return RolePermission{Create: 0, Read: 1, Update: 0, Delete: 0}, true
			}
		}
		return RolePermission{}, false // No access to other features
	}

	// DKI roles only get access to their features and general features
	if strings.Contains(roleNameLower, "dki") {
		// Allow access to general features (dashboard, profile, etc.)
		if featurePath == "" ||
			// strings.HasPrefix(featurePathLower, "tab-wp-dashboard") ||
			// strings.HasPrefix(featurePathLower, "tab-user-profile") ||
			strings.HasPrefix(featurePathLower, "tab-dki") {

			// DKI Admin gets full CRUD access
			if strings.Contains(roleNameLower, "admin") {
				return RolePermission{Create: 1, Read: 1, Update: 1, Delete: 1}, true
			}

			// DKI Client gets read-only access
			if strings.Contains(roleNameLower, "client") {
				return RolePermission{Create: 0, Read: 1, Update: 0, Delete: 0}, true
			}
		}
		return RolePermission{}, false // No access to other features
	}

	// DSP roles only get access to their features and general features
	if strings.Contains(roleNameLower, "dsp") {
		// Allow access to general features (dashboard, profile, etc.)
		if featurePath == "" ||
			// strings.HasPrefix(featurePathLower, "tab-wp-dashboard") ||
			// strings.HasPrefix(featurePathLower, "tab-user-profile") ||
			strings.HasPrefix(featurePathLower, "tab-dsp") {
			// DSP Admin gets full CRUD access
			if strings.Contains(roleNameLower, "admin") {
				return RolePermission{Create: 1, Read: 1, Update: 1, Delete: 1}, true
			}
			// DSP Client gets read-only access
			if strings.Contains(roleNameLower, "client") {
				return RolePermission{Create: 0, Read: 1, Update: 0, Delete: 0}, true
			}
		}
		return RolePermission{}, false // No access to other features
	}

	// BNI roles only get access to their features and general features
	if strings.Contains(roleNameLower, "bni") {
		// Allow access to general features (dashboard, profile, etc.)
		if featurePath == "" ||
			// strings.HasPrefix(featurePathLower, "tab-wp-dashboard") ||
			// strings.HasPrefix(featurePathLower, "tab-user-profile") ||
			strings.HasPrefix(featurePathLower, "tab-bni") {
			// BNI Admin gets full CRUD access
			if strings.Contains(roleNameLower, "admin") {
				return RolePermission{Create: 1, Read: 1, Update: 1, Delete: 1}, true
			}
			// BNI Client gets read-only access
			if strings.Contains(roleNameLower, "client") {
				return RolePermission{Create: 0, Read: 1, Update: 0, Delete: 0}, true
			}
		}
		return RolePermission{}, false // No access to other features
	}

	// Default role gets access to general features only
	if featurePath == "" ||
		strings.HasPrefix(featurePathLower, "tab-wp-dashboard") {
		// strings.HasPrefix(featurePathLower, "tab-user-profile") {
		return RolePermission{Create: 0, Read: 1, Update: 0, Delete: 0}, true
	}

	// No access to restricted features
	return RolePermission{}, false
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
						Create:    permission.Create,
						Read:      permission.Read,
						Update:    permission.Update,
						Delete:    permission.Delete,
					})
				}
			}
		}

		// Perform batch insert
		db.Create(&rolePrivileges)
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
				Path:     "tab-wp-dashboard",
				Status:   1,
				Level:    0,
				Icon:     "fad fa-tachometer-alt-fast",
			},
			/*
				Hommy Pay
			*/
			{
				ParentID: 0,
				Title:    "Hommy Pay",
				Path:     "",
				Status:   1,
				Level:    0,
				Icon:     "fad fa-money-check",
			},
			{
				ParentID: 0,
				Title:    "Data Ticket",
				Path:     "tab-hommy-pay-cc-ticket",
				Status:   1,
				Level:    1,
				// Level:    0,
				Icon: "fad fa-ballot-check",
			},
			{
				ParentID: 0,
				Title:    "Merchant",
				Path:     "tab-hommy-pay-cc-merchant",
				Status:   1,
				Level:    1,
				// Level:    0,
				Icon: "fad fa-store",
			},
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
				Icon:     "fab fa-whatsapp-square",
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
			{
				ParentID: 0,
				Title:    "Chat & Messages",
				Path:     "tab-whatsapp-conversation",
				Status:   1,
				Level:    1,
				// Level:    0,
				Icon: "fad fa-whatsapp",
			},
			/* * * * * * * * * * * * * * * * * * * * * * * * * * * */
			/*
				ODOO Manage Service
			*/
			{
				ParentID: 0,
				Title:    "ODOO MS",
				Path:     "",
				Status:   1,
				Level:    0,
				Icon:     "fad fa-server",
			},
			{
				ParentID: 0,
				Title:    "Upload Data (Import via Excel)",
				Path:     "tab-odoo-ms-upload-excel",
				Status:   1,
				Level:    1,
				// Level:    0,
				Icon: "fad fa-file-import",
			},
			/* * * * * * * * * * * * * * * * * * * * * * * * * * * */
			/*
				CSNA Human Resource
			*/
			{
				ParentID: 0,
				Title:    "Surat Peringatan (SP)",
				Path:     "tab-hr-sp",
				Status:   1,
				Level:    0,
				Icon:     "fad fa-file-exclamation",
			},
			{
				ParentID: 0,
				Title:    "Kontrak Teknisi",
				Path:     "tab-hr-kontrak-teknisi",
				Status:   1,
				Level:    0,
				Icon:     "fad fa-file-contract",
			},
			{
				ParentID: 0,
				Title:    "Slip Gaji Teknisi",
				Path:     "tab-hr-slip-gaji-teknisi",
				Status:   1,
				Level:    0,
				Icon:     "fad fa-file-invoice-dollar",
			},
			{
				ParentID: 0,
				Title:    "Surat Peringatan (SP) - Stock Opname",
				Path:     "tab-hr-sp-so",
				Status:   1,
				Level:    0,
				Icon:     "fad fa-bell-exclamation",
			},
			/* * * * * * * * * * * * * * * * * * * * * * * * * * * */
			/*
				MTI
			*/
			{
				ParentID: 0,
				Title:    "MTI",
				Path:     "",
				Status:   1,
				Level:    0,
				Icon:     "fad fa-handshake",
			},
			{
				ParentID: 0,
				Title:    "Monitoring Data PM (Preventive Maintenance)",
				Path:     "tab-mti-monitoring-pm",
				Status:   1,
				Level:    1,
				// Level:    0,
				Icon: "fad fa-chart-pie",
			},
			{
				ParentID: 0,
				Title:    "Monitoring Data Non-PM",
				Path:     "tab-mti-monitoring-non-pm",
				Status:   1,
				Level:    1,
				// Level:    0,
				Icon: "fad fa-chart-pie",
			},
			/* * * * * * * * * * * * * * * * * * * * * * * * * * * */
			/*
				DKI
			*/
			{
				ParentID: 0,
				Title:    "DKI",
				Path:     "",
				Status:   1,
				Level:    0,
				Icon:     "fad fa-table",
			},
			{
				ParentID: 0,
				Title:    "Data Ticket",
				Path:     "tab-dki-ticket",
				Status:   1,
				Level:    1,
				// Level:    0,
				Icon: "fad fa-ballot-check",
			},
			/* * * * * * * * * * * * * * * * * * * * * * * * * * * */
			/*
				DSP
			*/
			{
				ParentID: 0,
				Title:    "DSP",
				Path:     "",
				Status:   1,
				Level:    0,
				Icon:     "fad fa-vote-yea",
			},
			{
				ParentID: 0,
				Title:    "Data Ticket",
				Path:     "tab-dsp-ticket",
				Status:   1,
				Level:    1,
				// Level:    0,
				Icon: "fad fa-ballot-check",
			},
			/* * * * * * * * * * * * * * * * * * * * * * * * * * * */
			/*
				BNI
			*/
			{
				ParentID: 0,
				Title:    "BNI",
				Path:     "",
				Status:   1,
				Level:    0,
				Icon:     "fad fa-handshake",
			},
			{
				ParentID: 0,
				Title:    "Monitoring Data PM (Preventive Maintenance)",
				Path:     "tab-bni-monitoring-pm",
				Status:   1,
				Level:    1,
				// Level:    0,
				Icon: "fad fa-chart-pie",
			},
			{
				ParentID: 0,
				Title:    "Monitoring Data Non-PM",
				Path:     "tab-bni-monitoring-non-pm",
				Status:   1,
				Level:    1,
				// Level:    0,
				Icon: "fad fa-chart-pie",
			},
			/* * * * * * * * * * * * * * * * * * * * * * * * * * * */
			{
				ParentID: 0,
				Title:    "Email",
				Path:     "tab-email",
				Status:   1,
				Level:    0,
				Icon:     "fad fa-envelope-open-text",
			},
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
			{
				Title:         "Hommy Pay",
				ChildPrefixes: []string{"tab-hommy-pay-cc"},
			},
			{
				Title:         "ODOO MS",
				ChildPrefixes: []string{"tab-odoo-ms"},
			},
			{
				Title:         "MTI",
				ChildPrefixes: []string{"tab-mti"},
			},
			{
				Title:         "DKI",
				ChildPrefixes: []string{"tab-dki"},
			},
			{
				Title:         "DSP",
				ChildPrefixes: []string{"tab-dsp"},
			},
			{
				Title:         "BNI",
				ChildPrefixes: []string{"tab-bni"},
			},
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

func seedWhatsappLanguage(db *gorm.DB) {
	// WhatsApp Bot Language
	var languageCount int64
	db.Model(&model.Language{}).Count(&languageCount)
	if languageCount == 0 {
		languages := []model.Language{
			{
				Name: "Bahasa Indonesia",
				Code: "id",
			},
			{
				Name: "English",
				Code: "us",
			},
			// Add more languages as needed
		}

		db.Create(&languages)

		for _, language := range languages {
			logrus.Println("🏳 Insert New Language with ID:", language.ID)
		}
	}
}

func seedWhatsappPhoneUser(db *gorm.DB) {
	// Seed Whatsapp Bot User
	var waPhoneUserCount int64
	db.Model(&model.WAPhoneUser{}).Count(&waPhoneUserCount)

	allowedTypes := model.AllWAMessageTypes
	jsonBytes, err := json.Marshal(allowedTypes)
	if err != nil {
		panic(fmt.Sprintf("failed to marshal allowed types: %v", err))
	}

	if waPhoneUserCount == 0 {
		waUsers := []model.WAPhoneUser{
			{
				FullName:      "Wegirandol Histara Littu",
				Email:         "wegirandol@smartwebindonesia.com",
				PhoneNumber:   config.WebPanel.Get().Whatsmeow.WaSuperUser,
				IsRegistered:  true,
				AllowedChats:  model.BothChat,
				AllowedTypes:  datatypes.JSON(jsonBytes),
				AllowedToCall: true,
				Description:   "Phone number of Whatsapp SUPER USER",
				IsBanned:      false,
				UserType:      model.WaBotSuperUser,
				MaxDailyQuota: 250,
				UserOf:        model.UserOfCSNA,
			},
			{
				FullName:      "Pak Made Widiajaya",
				Email:         "made@smartwebdindonesia.com",
				PhoneNumber:   "62816836777",
				IsRegistered:  true,
				AllowedChats:  model.DirectChat,
				AllowedTypes:  datatypes.JSON(jsonBytes),
				AllowedToCall: false,
				Description:   "Project Manager - Made Widiajaya",
				IsBanned:      false,
				UserType:      model.CompanyPM,
				MaxDailyQuota: 250,
				UserOf:        model.UserOfCSNA,
			},
			{
				FullName:      "RM Test - Ipal",
				Email:         "desta@smartwebindonesia.com",
				PhoneNumber:   "6287883507445",
				IsRegistered:  true,
				AllowedChats:  model.DirectChat,
				AllowedTypes:  datatypes.JSON(jsonBytes),
				AllowedToCall: false,
				Description:   "Rawamangun Tester & Technical Support",
				IsBanned:      false,
				UserType:      model.ODOOMSStaff,
				MaxDailyQuota: 250,
				UserOf:        model.UserOfCSNA,
			},
			{
				FullName:      "Pak Johannes Filandow",
				Email:         "filandow@smartwebindonesia.com",
				PhoneNumber:   "62816765987",
				IsRegistered:  true,
				AllowedChats:  model.DirectChat,
				AllowedTypes:  datatypes.JSON(jsonBytes),
				AllowedToCall: false,
				Description:   "CEO (Chief Executive Officer) - CSNA",
				IsBanned:      false,
				UserType:      model.CompanyCEO,
				MaxDailyQuota: 250,
				UserOf:        model.UserOfCSNA,
			},
			{
				FullName:      "Mr. Oliver Hou",
				Email:         "oliver@csna4u.com",
				PhoneNumber:   "886936122313", // Taiwan number
				IsRegistered:  true,
				AllowedChats:  model.DirectChat,
				AllowedTypes:  datatypes.JSON(jsonBytes),
				AllowedToCall: false,
				Description:   "COO (Chief Operating Officer) - CSNA",
				IsBanned:      false,
				UserType:      model.CompanyCOO,
				MaxDailyQuota: 250,
				UserOf:        model.UserOfCSNA,
			},
			{
				FullName:      "Cindy Chang (Jiajia)",
				Email:         "cindychang@cybersoft.com.tw",
				PhoneNumber:   "886922541216", // Taiwan number
				IsRegistered:  true,
				AllowedChats:  model.DirectChat,
				AllowedTypes:  datatypes.JSON(jsonBytes),
				AllowedToCall: false,
				Description:   "Secretary to Mr. Oliver - CSNA",
				IsBanned:      false,
				UserType:      model.CompanySecretary,
				MaxDailyQuota: 250,
				UserOf:        model.UserOfCSNA,
			},
			{
				FullName:      "Nicken Bagenda",
				Email:         "nickenb@csna4u.com",
				PhoneNumber:   "6287780546451",
				IsRegistered:  true,
				AllowedChats:  model.DirectChat,
				AllowedTypes:  datatypes.JSON(jsonBytes),
				AllowedToCall: false,
				Description:   "CBO (Chief Business Officer) - CSNA",
				IsBanned:      false,
				UserType:      model.CompanyCBO,
				MaxDailyQuota: 250,
				UserOf:        model.UserOfCSNA,
			},
			{
				FullName:      "Fauzi Abdillah",
				Email:         "fauziab@csna4u.com",
				PhoneNumber:   "6281210035600",
				IsRegistered:  true,
				AllowedChats:  model.DirectChat,
				AllowedTypes:  datatypes.JSON(jsonBytes),
				AllowedToCall: false,
				Description:   "Project Management Officer - MANDIRI",
				IsBanned:      false,
				UserType:      model.CompanyPMO,
				MaxDailyQuota: 250,
				UserOf:        model.UserOfCSNA,
			},
			{
				FullName:      config.WebPanel.Get().Default.PTHRD[0].Name,
				Email:         config.WebPanel.Get().Default.PTHRD[0].Email,
				PhoneNumber:   config.WebPanel.Get().Default.PTHRD[0].PhoneNumber,
				IsRegistered:  true,
				AllowedChats:  model.DirectChat,
				AllowedTypes:  datatypes.JSON(jsonBytes),
				AllowedToCall: false,
				Description:   "HRD (Human Resource Development) - CSNA",
				IsBanned:      false,
				UserType:      model.CompanyHR,
				MaxDailyQuota: 250,
				UserOf:        model.UserOfCSNA,
			},
			// {
			// 	FullName:      config.WebPanel.Get().Default.PTHRD[1].Name,
			// 	Email:         config.WebPanel.Get().Default.PTHRD[1].Email,
			// 	PhoneNumber:   config.WebPanel.Get().Default.PTHRD[1].PhoneNumber,
			// 	IsRegistered:  true,
			// 	AllowedChats:  model.DirectChat,
			// 	AllowedTypes:  datatypes.JSON(jsonBytes),
			// 	AllowedToCall: false,
			// 	Description:   "HRD (Human Resource Development) - CSNA",
			// 	IsBanned:      false,
			// 	UserType:      model.CompanyHR,
			// 	MaxDailyQuota: 250,
			// 	UserOf:        model.UserOfCSNA,
			// },
			{
				FullName:      "IT Support CSNA",
				Email:         "rifaldi@smartwebindonesia.com",
				PhoneNumber:   "6281364426111",
				IsRegistered:  true,
				AllowedChats:  model.DirectChat,
				AllowedTypes:  datatypes.JSON(jsonBytes),
				AllowedToCall: false,
				Description:   "IT Support CSNA - Ipal (AIPOS)",
				IsBanned:      false,
				UserType:      model.ODOOMSStaff,
				MaxDailyQuota: 500,
				UserOf:        model.UserOfCSNA,
			},
		}
		botUsedPhoneNumber := config.WebPanel.Get().Whatsmeow.WaBotUsed
		for i, phone := range botUsedPhoneNumber {
			fullName := ""
			switch i {
			case 0:
				fullName = "Bot Whatsapp (Development)"
			case 1:
				fullName = "Bot Whatsapp (Production)"
			default:
				fullName = fmt.Sprintf("Bot Whatsapp %d", i+1)
			}

			waUsers = append(waUsers, model.WAPhoneUser{
				FullName:      fullName,
				Email:         fmt.Sprintf("bot_wa_wp_%d@gmail.com", i+1),
				PhoneNumber:   phone,
				IsRegistered:  true,
				AllowedChats:  model.BothChat,
				AllowedTypes:  datatypes.JSON(jsonBytes),
				AllowedToCall: true,
				Description:   "Phone number of Whatsapp Bot User",
				IsBanned:      false,
				UserType:      model.WaBotSuperUser,
				MaxDailyQuota: 250,
				UserOf:        model.UserOfCSNA,
			})
		}

		for _, waUser := range waUsers {
			// Check if user with this phone number already exists
			var existingUser model.WAPhoneUser
			if err := db.Where("phone_number = ?", waUser.PhoneNumber).First(&existingUser).Error; err != nil {
				if err == gorm.ErrRecordNotFound {
					// User doesn't exist, create new one
					if err := db.Create(&waUser).Error; err != nil {
						panic(fmt.Sprintf("Failed to seed WhatsApp user with phone %s: %v", waUser.PhoneNumber, err))
					}
					logrus.Printf("✅ Created new WhatsApp user: %s (%s)", waUser.FullName, waUser.PhoneNumber)
				} else {
					// Some other database error occurred
					panic(fmt.Sprintf("Error checking existing WhatsApp user with phone %s: %v", waUser.PhoneNumber, err))
				}
			} else {
				// User already exists, skip creation
				logrus.Printf("⚠️ WhatsApp user with phone %s already exists, skipping creation for %s as %s (%s)", waUser.PhoneNumber, waUser.FullName, waUser.Description, waUser.UserType)
			}
		}
	}
}

func seedBadWords(db *gorm.DB) {
	// Check if there are already bad words
	var count int64
	db.Model(&model.BadWord{}).Count(&count)

	if count == 0 {
		// Seed data
		badWords := []model.BadWord{
			// Indonesian (id)
			{Word: "anjing", Language: "id", Category: model.CategoryUmpatan, IsEnabled: true},
			{Word: "bajingan", Language: "id", Category: model.CategoryUmpatan, IsEnabled: true},
			{Word: "jancok", Language: "id", Category: model.CategoryUmpatan, IsEnabled: true},
			{Word: "jancuk", Language: "id", Category: model.CategoryUmpatan, IsEnabled: true},
			{Word: "bangsat", Language: "id", Category: model.CategoryUmpatan, IsEnabled: true},
			{Word: "bodoh", Language: "id", Category: model.CategoryGeneral, IsEnabled: true},
			{Word: "kontol", Language: "id", Category: model.CategorySexual, IsEnabled: true},
			{Word: "memek", Language: "id", Category: model.CategorySexual, IsEnabled: true},
			{Word: "ngentot", Language: "id", Category: model.CategorySexual, IsEnabled: true},
			{Word: "goblok", Language: "id", Category: model.CategoryGeneral, IsEnabled: true},
			{Word: "tolol", Language: "id", Category: model.CategoryGeneral, IsEnabled: true},
			{Word: "tai", Language: "id", Category: model.CategoryGeneral, IsEnabled: true},
			{Word: "setan", Language: "id", Category: model.CategoryUmpatan, IsEnabled: true},
			{Word: "babi", Language: "id", Category: model.CategoryUmpatan, IsEnabled: true},
			{Word: "kampret", Language: "id", Category: model.CategoryUmpatan, IsEnabled: true},
			{Word: "puki", Language: "id", Category: model.CategoryUmpatan, IsEnabled: true},
			{Word: "cukimai", Language: "id", Category: model.CategoryUmpatan, IsEnabled: true},
			{Word: "telaso", Language: "id", Category: model.CategoryUmpatan, IsEnabled: true},
			{Word: "tailaso", Language: "id", Category: model.CategoryUmpatan, IsEnabled: true},
			{Word: "coklat", Language: "id", Category: model.CategoryRasis, IsEnabled: false}, // example of disabled word

			// English (en)
			{Word: "bitch", Language: "en", Category: model.CategoryUmpatan, IsEnabled: true},
			{Word: "fuck", Language: "en", Category: model.CategoryUmpatan, IsEnabled: true},
			{Word: "idiot", Language: "en", Category: model.CategoryGeneral, IsEnabled: true},
			{Word: "nigger", Language: "en", Category: model.CategoryGeneral, IsEnabled: true},
			{Word: "nigga", Language: "en", Category: model.CategoryGeneral, IsEnabled: true},
			{Word: "dick", Language: "en", Category: model.CategoryGeneral, IsEnabled: true},
		}

		if err := db.Create(&badWords).Error; err != nil {
			logrus.Error("Failed to seed bad words: ", err)
		} else {
			logrus.Info("✅ Seeded bad words successfully")
		}
	}
}

func seedAppConfig(db *gorm.DB) {
	var count int64
	db.Model(&model.AppConfig{}).Count(&count)

	if count == 0 {
		// Find role IDs dynamically
		var superAdminRole model.Role
		if err := db.Where("role_name = ?", "Super Admin").First(&superAdminRole).Error; err != nil {
			logrus.Errorf("Error while trying to fetch Super Admin role: %v", err)
			return
		}

		var hommyPayClientRole model.Role
		if err := db.Where("role_name = ?", "Hommy Pay - Client").First(&hommyPayClientRole).Error; err != nil {
			logrus.Errorf("Error while trying to fetch Hommy Pay - Client role: %v", err)
			return
		}

		var hommyPayAdminRole model.Role
		if err := db.Where("role_name = ?", "Hommy Pay - Admin").First(&hommyPayAdminRole).Error; err != nil {
			logrus.Errorf("Error while trying to fetch Hommy Pay - Admin role: %v", err)
			return
		}

		var odooMSStaffRole model.Role
		if err := db.Where("role_name = ?", "ODOO MS - Staff").First(&odooMSStaffRole).Error; err != nil {
			logrus.Errorf("Error while trying to fetch ODOO MS - Staff role: %v", err)
			return
		}

		var odooMSHeadRole model.Role
		if err := db.Where("role_name = ?", "ODOO MS - Head").First(&odooMSHeadRole).Error; err != nil {
			logrus.Errorf("Error while trying to fetch ODOO MS - Head role: %v", err)
			return
		}

		var csnaHRRole model.Role
		if err := db.Where("role_name = ?", "CSNA - Human Resource").First(&csnaHRRole).Error; err != nil {
			logrus.Errorf("Error while trying to fetch CSNA - Human Resource role: %v", err)
			return
		}

		var mtiClientRole model.Role
		if err := db.Where("role_name = ?", "MTI - Client").First(&mtiClientRole).Error; err != nil {
			logrus.Errorf("Error while trying to fetch MTI - Client role: %v", err)
			return
		}

		var dkiClientRole model.Role
		if err := db.Where("role_name = ?", "DKI - Client").First(&dkiClientRole).Error; err != nil {
			logrus.Errorf("Error while trying to fetch DKI - Client role: %v", err)
			return
		}

		var dspClientRole model.Role
		if err := db.Where("role_name = ?", "DSP - Client").First(&dspClientRole).Error; err != nil {
			logrus.Errorf("Error while trying to fetch DSP - Client role: %v", err)
			return
		}

		var bniClientRole model.Role
		if err := db.Where("role_name = ?", "BNI - Client").First(&bniClientRole).Error; err != nil {
			logrus.Errorf("Error while trying to fetch BNI - Client role: %v", err)
			return
		}

		appConfigs := []model.AppConfig{
			{
				RoleID:      superAdminRole.ID,
				AppName:     "Web Panel",
				AppLogo:     "/assets/self/img/logo_web.png",
				AppVersion:  "Beta",
				VersionNo:   "1",
				VersionCode: "0.0.0.1.2025.07.31",
				VersionName: "rm_dev",
				IsActive:    true,
				Description: "Dashboard for managing the entire system, including user roles, permissions, and configurations. Also for managing web & apps like Hommy Pay & Whatsapp Bot.",
			},
			{
				RoleID:      hommyPayClientRole.ID,
				AppName:     "Hommy Pay",
				AppLogo:     "/assets/self/img/hommy_pay_logo.png",
				AppVersion:  "Release",
				VersionNo:   "1",
				VersionCode: "1.0.0.0.2025.07.31",
				VersionName: "gl_prod",
				IsActive:    true,
				Description: "Hommy Pay Client App for managing transactions, tickets, and merchant data. Designed for Hommy Pay clients to access their financial data and manage their accounts.",
			},
			{
				RoleID:      hommyPayAdminRole.ID,
				AppName:     "Hommy Pay",
				AppLogo:     "/assets/self/img/hommy_pay_logo.png",
				AppVersion:  "Release",
				VersionNo:   "1",
				VersionCode: "1.0.0.0.2025.07.31",
				VersionName: "gl_prod",
				IsActive:    true,
				Description: "Hommy Pay Admin App for managing transactions, tickets, and merchant data. Designed for Hommy Pay administrators to manage the system and oversee client accounts.",
			},
			{
				RoleID:      odooMSStaffRole.ID,
				AppName:     "ODOO MS",
				AppLogo:     "/assets/self/img/odoo_logo.png",
				AppVersion:  "Release",
				VersionNo:   "1",
				VersionCode: "1.0.0.0.2025.08.04",
				VersionName: "rm_dev",
				IsActive:    true,
				Description: "ODOO MS Staff App for managing ODOO Manage Service operations. Designed for ODOO MS staff to manage tickets, updates, and service requests.",
			},
			{
				RoleID:      odooMSHeadRole.ID,
				AppName:     "ODOO MS",
				AppLogo:     "/assets/self/img/odoo_logo.png",
				AppVersion:  "Release",
				VersionNo:   "1",
				VersionCode: "1.0.0.0.2025.08.04",
				VersionName: "rm_dev",
				IsActive:    true,
				Description: "ODOO MS Staff App for managing ODOO Manage Service operations. Designed for ODOO MS staff to manage tickets, updates, and service requests.",
			},
			{
				RoleID:      csnaHRRole.ID,
				AppName:     "Human Resource",
				AppLogo:     "/assets/self/img/csna.png",
				AppVersion:  "Release",
				VersionNo:   "1",
				VersionCode: "1.0.0.0.2025.09.03",
				VersionName: "csna_hr",
				IsActive:    true,
				Description: "Human Resource App for managing employee data, attendance, and payroll. Designed for CSNA HR to oversee and manage all HR-related activities.",
			},
			{
				RoleID:      mtiClientRole.ID,
				AppName:     "MTI",
				AppLogo:     "/assets/self/img/yokke.png",
				AppVersion:  "Release",
				VersionNo:   "1",
				VersionCode: "1.0.0.0.2025.09.15",
				VersionName: "mti_monitoring_app",
				IsActive:    true,
				Description: "MTI Client App for monitoring manage service data, including preventive maintenance (PM) and non-PM activities. Designed for MTI clients to access and manage their service data.",
			},
			{
				RoleID:      dkiClientRole.ID,
				AppName:     "DKI",
				AppLogo:     "/assets/self/img/dki.png",
				AppVersion:  "Release",
				VersionNo:   "1",
				VersionCode: "1.0.0.0.2025.09.16",
				VersionName: "dki_ticketing_app",
				IsActive:    true,
				Description: "DKI Client App for managing tickets and service requests. Designed for DKI clients to access their ticketing data and manage their accounts.",
			},
			{
				RoleID:      dspClientRole.ID,
				AppName:     "DSP",
				AppLogo:     "/assets/self/img/dsp.png",
				AppVersion:  "Release",
				VersionNo:   "1",
				VersionCode: "1.0.0.0.2025.11.12",
				VersionName: "dsp_ticketing_app",
				IsActive:    true,
				Description: "DSP Client App for managing tickets and service requests. Designed for DSP clients to access their ticketing data and manage their accounts.",
			},
			{
				RoleID:      bniClientRole.ID,
				AppName:     "BNI",
				AppLogo:     "/assets/self/img/bni.png",
				AppVersion:  "Beta",
				VersionNo:   "1",
				VersionCode: "1.0.0.0.2026.01.08",
				VersionName: "bni_ticketing_app",
				IsActive:    true,
				Description: "BNI Client App for managing tickets and service requests. Designed for BNI clients to access their ticketing data and manage their accounts.",
			},
		}

		if err := db.Create(&appConfigs).Error; err != nil {
			logrus.Error("Failed to seed app configs: ", err)
		} else {
			logrus.Info("✅ Seeded app configs successfully")
		}
	}
}

func seedNomorSuratSP(db *gorm.DB) {
	var SuratSPCounter = map[string]int{
		"LAST_NOMOR_SURAT_SP1_GENERATED": config.WebPanel.Get().SPTechnician.LastNomorSuratSP1Generated,
		"LAST_NOMOR_SURAT_SP2_GENERATED": config.WebPanel.Get().SPTechnician.LastNomorSuratSP2Generated,
		"LAST_NOMOR_SURAT_SP3_GENERATED": config.WebPanel.Get().SPTechnician.LastNomorSuratSP3Generated,
	}

	for key, value := range SuratSPCounter {
		var existing sptechnicianmodel.NomorSuratSP
		if err := db.Where("id = ?", key).First(&existing).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				newEntry := sptechnicianmodel.NomorSuratSP{
					ID:               key,
					LastNomorSuratSP: value,
				}
				if err := db.Create(&newEntry).Error; err != nil {
					logrus.Errorf("Failed to create entry for %s: %v", key, err)
				} else {
					logrus.Infof("Created entry for %s with value %d", key, value)
				}
			} else {
				logrus.Errorf("Error checking existence of %s: %v", key, err)
			}
		}
	}

}

func seedNomorSuratContract(db *gorm.DB) {
	var SuratContractCounter = map[string]int{
		"LAST_NOMOR_SURAT_CONTRACT_GENERATED": config.WebPanel.Get().ContractTechnicianODOO.LastNomorSuratGenerated,
	}

	for key, value := range SuratContractCounter {
		var existing contracttechnicianmodel.NomorSuratContract
		if err := db.Where("id = ?", key).First(&existing).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				newEntry := contracttechnicianmodel.NomorSuratContract{
					ID:             key,
					LastNomorSurat: value,
				}
				if err := db.Create(&newEntry).Error; err != nil {
					logrus.Errorf("Failed to create entry for %s: %v", key, err)
				} else {
					logrus.Infof("Created entry for %s with value %d", key, value)
				}
			} else {
				logrus.Errorf("Error checking existence of %s: %v", key, err)
			}
		}
	}
}

func seedIndonesiaRegion(db *gorm.DB) {
	// Get the table name from config
	tableName := config.WebPanel.Get().Database.TbIndonesiaRegion

	// Check if table exists
	if !tableExists(db, tableName) {
		logrus.Infof("Table '%s' does not exist. Creating table structure first...", tableName)

		// Step 1: Create the table structure using GORM AutoMigrate
		if err := createIndonesiaRegionTable(db); err != nil {
			logrus.Errorf("Failed to create table structure for indonesia region: %v", err)
			return
		}

		logrus.Infof("✅ Table structure for '%s' created successfully", tableName)

		// Step 2: Check if table has data
		var count int64
		if err := db.Table(tableName).Count(&count).Error; err != nil {
			logrus.Errorf("Failed to count records in table '%s': %v", tableName, err)
			return
		}

		if count == 0 {
			logrus.Infof("Importing data from SQL file into table '%s'...", tableName)
			// Step 3: Import data from SQL file (only INSERT statements)
			if err := importIndonesiaRegionData(db, config.WebPanel.Get().Database.DumpedIndonesiaRegionSQL); err != nil {
				logrus.Errorf("Failed to import data for indonesia region: %v", err)
				return
			}
			logrus.Infof("✅ Successfully imported data into table '%s'", tableName)
		} else {
			logrus.Infof("Table '%s' already contains %d records, skipping data import", tableName, count)
		}
	}
}

// createIndonesiaRegionTable creates the indonesia_region table structure using GORM AutoMigrate
func createIndonesiaRegionTable(db *gorm.DB) error {
	// Create the table with custom table name from config
	tableName := config.WebPanel.Get().Database.TbIndonesiaRegion

	// Set custom table name for migration
	err := db.Table(tableName).AutoMigrate(&model.IndonesiaRegion{})
	if err != nil {
		return fmt.Errorf("failed to create table structure: %v", err)
	}

	return nil
}

// importIndonesiaRegionData imports only the INSERT data from SQL file
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
	tableName := config.WebPanel.Get().Database.TbIndonesiaRegion

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
			statement = strings.ReplaceAll(statement, "`indonesia_region`", "`"+tableName+"`")
			statement = strings.ReplaceAll(statement, "indonesia_region", tableName)

			if err := db.Exec(statement).Error; err != nil {
				logrus.Warnf("Warning executing INSERT statement: %v", err)
				// Continue with other inserts even if one fails
			}
		}
	}

	return nil
}

// tableExists checks if a table exists in the database
func tableExists(db *gorm.DB, tableName string) bool {
	var exists bool
	query := "SELECT EXISTS(SELECT 1 FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = ?)"
	err := db.Raw(query, tableName).Scan(&exists).Error
	if err != nil {
		logrus.Errorf("Error checking if table '%s' exists: %v", tableName, err)
		return false
	}
	return exists
}
