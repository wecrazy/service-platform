// Package seed provides database seeding functionality for initial application data.
//
// This package contains functions to populate the database with essential initial data
// including roles, features, users, permissions, and reference data. The seeding
// functions are designed to be idempotent - they check for existing data before
// inserting to avoid duplicates.
//
// The package is used by the migration system to ensure consistent initial data
// across different environments and deployments.
//
// Key seeding functions:
//   - SeedRoles: Creates default user roles (Super User, Client Company roles)
//   - SeedFeature: Creates application features and menu structure
//   - SeedRolePrivilege: Assigns permissions to roles for features
//   - SeedUser: Creates default system users
//   - SeedUserStatus: Creates user status definitions
//   - SeedUserPasswordChangeLog: Initializes password change tracking
//   - SeedWhatsappUser: Creates default WhatsApp bot users
//   - SeedWhatsappLanguage: Populates supported languages for WhatsApp
//   - SeedBadWords: Creates bad word filters by language
//   - SeedWhatsAppMsgAutoReply: Sets up automated message responses
//   - SeedAppConfig: Creates default application configuration
//   - SeedIndonesiaRegion: Imports Indonesian regional data
//
// All seeding functions use configuration values for table names and data,
// ensuring consistency with the application's database schema configuration.
package seed

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"service-platform/internal/config"
	"service-platform/internal/core/model"
	telegrammodel "service-platform/internal/core/model/telegram_model"
	"service-platform/internal/pkg/fun"
	"strings"
	"time"

	"github.com/nyaruka/phonenumbers"

	"github.com/sirupsen/logrus"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// RolePermission defines CRUD permissions for a role on a specific feature.
// This struct is used internally by the permission seeding logic to determine
// what operations a role can perform on application features.
//
// Fields represent permission levels:
//   - CanCreate: 1 if role can create new records, 0 otherwise
//   - CanRead: 1 if role can read/view records, 0 otherwise
//   - CanUpdate: 1 if role can modify existing records, 0 otherwise
//   - CanDelete: 1 if role can delete records, 0 otherwise
type RolePermission struct {
	CanCreate int8
	CanRead   int8
	CanUpdate int8
	CanDelete int8
}

// SeedRoles creates the default user roles in the system.
//
// This function populates the roles table with three default roles:
//   - Super User: Full system access with administrative privileges
//   - Client Company - User: Standard client company user with limited access
//   - Client Company - Admin: Client company administrator with elevated permissions
//
// The function is idempotent - it only creates roles if none exist in the database.
// Each role includes display properties like icons and CSS classes for the UI.
//
// Parameters:
//   - db: GORM database instance
//
// The function performs a batch insert of all roles and logs their creation.
func SeedRoles(db *gorm.DB) {
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

// SeedFeature creates the application feature definitions and menu structure.
//
// This function populates the features table with all available application features,
// including their hierarchical relationships, menu ordering, and access paths.
// Features are organized into a menu structure with parent-child relationships.
//
// The function creates features for:
//   - Dashboard: Main application dashboard
//   - WhatsApp: WhatsApp bot management and configuration
//   - Scheduler: Scheduled job management
//   - App Configuration: System configuration settings
//   - User Management: Roles, users, and permissions
//   - System Logs: Activity and system logging
//   - User Profile: User profile management
//
// Features are assigned menu order numbers automatically, and parent-child
// relationships are established for hierarchical menu structures.
//
// Parameters:
//   - db: GORM database instance
//
// The function performs batch inserts and establishes parent-child relationships
// based on feature path prefixes.
func SeedFeature(db *gorm.DB) {
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
			/*
				Telegram
			*/
			{
				ParentID: 0,
				Title:    "Telegram",
				Path:     "",
				Status:   1,
				Level:    0,
				Icon:     "fab fa-telegram-plane",
			},
			{
				ParentID: 0,
				Title:    "Bot Telegram",
				Path:     "tab-telegram",
				Status:   1,
				Level:    1,
				Icon:     "fab fa-telegram",
			},
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
				ChildPrefixes: []string{"tab-whatsapp", "tab-whatsapp-user-management"},
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

// SeedRolePrivilege assigns permissions to roles for each application feature.
//
// This function creates role-privilege mappings that determine what operations
// each role can perform on different features. It uses the getRolePermissions
// helper function to determine appropriate permissions based on role names
// and feature paths.
//
// Permission logic:
//   - Super User: Full CRUD access to all features
//   - Client Company roles: Access to their specific features and general features
//   - Default roles: Read-only access to basic features
//
// Parameters:
//   - db: GORM database instance
//
// The function queries existing roles and features, then creates privilege
// records for all valid role-feature combinations.
func SeedRolePrivilege(db *gorm.DB) {
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

// getRolePermissions determines the appropriate CRUD permissions for a role on a feature.
//
// This helper function implements the business logic for role-based access control,
// determining what operations a specific role can perform on a given feature.
//
// Parameters:
//   - roleName: Name of the user role
//   - featurePath: Path/identifier of the application feature
//
// Returns:
//   - RolePermission: Struct containing CRUD permission flags
//   - bool: true if the role has access to the feature, false otherwise
//
// Permission rules:
//   - Super User: Full access to everything
//   - Client Company roles: Access to company-specific and general features
//   - Other roles: Limited access to basic features only
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

// SeedUser creates default system users with appropriate roles and credentials.
//
// This function creates initial users required for system operation:
//   - RM Developer (Super User): Full system access for development/administration
//   - Admin Client Company: Administrative access for client companies
//   - Client Company User: Standard user access for client companies
//
// User credentials are sourced from configuration:
//   - Super user details from config.Default.SuperUser*
//   - Client company users have predefined credentials
//
// All users are created with:
//   - Encrypted passwords using the application's password hashing
//   - Appropriate role assignments
//   - Default profile images
//   - Active status
//
// Parameters:
//   - db: GORM database instance
//
// The function ensures users are only created if they don't already exist.
func SeedUser(db *gorm.DB) {
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
			Phone:        config.ServicePlatform.Get().Default.SuperUserPhone,
			Email:        config.ServicePlatform.Get().Default.SuperUserEmail,
			Password:     fun.GenerateSaltedPassword(config.ServicePlatform.Get().Default.SuperUserPassword),
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

// SeedUserStatus creates the standard user status definitions.
//
// This function populates the user status table with three standard statuses:
//   - PENDING (ID: 1): User account awaiting activation
//   - ACTIVE (ID: 2): User account is active and can use the system
//   - INACTIVE (ID: 3): User account is deactivated
//
// Each status includes display properties (title and CSS class) for UI rendering.
//
// Parameters:
//   - db: GORM database instance
//
// The function performs a batch insert of status definitions.
func SeedUserStatus(db *gorm.DB) {
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

// SeedUserPasswordChangeLog initializes the user password change log table.
//
// This function populates the user_password_changelog table with existing
// users' current passwords to establish a baseline for password change tracking.
//
// Parameters:
//   - db: GORM database instance
//
// The function retrieves all existing users and creates corresponding
// password change log entries with their current passwords.
func SeedUserPasswordChangeLog(db *gorm.DB) {
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

// SeedIndonesiaRegion imports Indonesian regional data from SQL dump files.
//
// This function populates the indonesia_region table with comprehensive
// regional data including provinces, cities, districts, and villages.
// The data is imported from a SQL dump file specified in configuration.
//
// The function performs the following steps:
// 1. Checks if the target table exists, creates it if necessary using GORM AutoMigrate
// 2. Verifies the SQL dump file exists at the configured path
// 3. Parses and executes INSERT statements from the SQL file
// 4. Handles MySQL-to-PostgreSQL syntax differences
// 5. Skips non-INSERT statements and comments
//
// Parameters:
//   - db: GORM database instance
//
// The function uses configuration values for:
//   - Table name (config.Database.TbIndonesiaRegion)
//   - SQL dump file path (config.Database.DumpedIndonesiaRegionSQL)
//
// Errors in SQL file import are logged but don't prevent the seeding process
// from continuing, allowing the application to start even with missing regional data.
func SeedIndonesiaRegion(db *gorm.DB) {
	tableName := config.ServicePlatform.Get().Database.TbIndonesiaRegion

	// Check if table exists
	if !fun.TableExists(db, tableName) {
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

		sqlDumpedFile := config.ServicePlatform.Get().Database.DumpedIndonesiaRegionSQL

		internalDir, err := fun.FindValidDirectory([]string{
			"internal",
			"../internal",
			"../../internal",
			"../../../internal",
		})
		if err != nil {
			logrus.Errorf("Failed to locate 'internal' directory: %v", err)
			return
		}

		if _, err := os.Stat(sqlDumpedFile); os.IsNotExist(err) {
			sqlDumpedFile = filepath.Join(internalDir, sqlDumpedFile)
		}

		err = importIndonesiaRegionData(db, sqlDumpedFile)
		if err != nil {
			logrus.Errorf("Failed to import data for indonesia region: %v", err)
		} else {
			logrus.Infof("✅ Successfully imported data into table '%s'", tableName)
		}
	} else {
		// logrus.Infof("Table '%s' already contains %d records, skipping data import", tableName, count)
	}
}

// createIndonesiaRegionTable creates the Indonesia region table structure using GORM AutoMigrate.
//
// This helper function ensures the indonesia_region table exists with the correct schema
// before attempting to import data. It uses GORM's AutoMigrate functionality to create
// or update the table structure based on the IndonesiaRegion model.
//
// The function uses the configured table name from config.Database.TbIndonesiaRegion
// to maintain consistency with the application's database naming conventions.
//
// Parameters:
//   - db: GORM database instance
//
// Returns:
//   - error: Any error encountered during table creation
//
// This function is called automatically by SeedIndonesiaRegion if the table doesn't exist.
func createIndonesiaRegionTable(db *gorm.DB) error {
	// Create the table with custom table name from config
	tableName := config.ServicePlatform.Get().Database.TbIndonesiaRegion

	// Set custom table name for migration
	err := db.Table(tableName).AutoMigrate(&model.IndonesiaRegion{})
	if err != nil {
		return fmt.Errorf("failed to create table structure: %v", err)
	}

	return nil
}

// SeedWhatsappUser creates default WhatsApp bot users for system operation.
//
// This function creates WhatsApp user accounts that the system uses for
// automated messaging and bot operations. Currently creates a super user
// WhatsApp account with full permissions and high daily quota.
//
// The function configures:
//   - Full message type permissions (text, image, document, etc.)
//   - Both personal and group chat capabilities
//   - Calling permissions
//   - Daily message quota limits
//   - User type and organizational assignments
//
// Parameters:
//   - db: GORM database instance
//
// WhatsApp user credentials are sourced from configuration settings.
func SeedWhatsappUser(db *gorm.DB) {
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
				Email:         config.ServicePlatform.Get().Default.SuperUserEmail,
				PhoneNumber:   config.ServicePlatform.Get().Default.SuperUserPhone,
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

// SeedWhatsappLanguage populates the supported languages for WhatsApp bot.
//
// This function inserts language definitions into the languages table
// using a predefined mapping of language codes to names from the fun package.
//
// Parameters:
//   - db: GORM database instance
//
// The function checks if any languages already exist to avoid duplicates,
// ensuring idempotent behavior.
func SeedWhatsappLanguage(db *gorm.DB) {
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

// SeedBadWords creates bad word filters for multiple languages.
//
// This function populates the bad_words table with offensive words
// categorized by type (e.g., sexual, racist, general insults) for
// various languages supported by the application.
//
// Parameters:
//   - db: GORM database instance
//
// The function checks if any bad words already exist to avoid duplicates,
// ensuring idempotent behavior.
func SeedBadWords(db *gorm.DB) {
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

// SeedWhatsAppMsgAutoReply creates default auto-reply messages for WhatsApp bot.
//
// This function populates the whatsapp_message_auto_replies table with
// predefined auto-reply messages for various languages. Each auto-reply
// is associated with specific keywords that trigger the response.
//
// Parameters:
//   - db: GORM database instance
//
// The function checks if any auto-replies already exist to avoid duplicates,
// ensuring idempotent behavior.
func SeedWhatsAppMsgAutoReply(db *gorm.DB) {
	var count int64
	db.Model(&model.WhatsappMessageAutoReply{}).Count(&count)

	dataSeparator := config.ServicePlatform.Get().Default.DataSeparator
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

// SeedAppConfig creates default application configurations for different user roles.
//
// This function populates the app_configs table with predefined configurations
// for various user roles, including Super User, Client Company - User, and
// Client Company - Admin. Each configuration includes app name, logo, version,
// and description tailored to the role.
//
// Parameters:
//   - db: GORM database instance
//
// The function checks if any app configurations already exist to avoid duplicates,
// ensuring idempotent behavior.
func SeedAppConfig(db *gorm.DB) {
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

// importIndonesiaRegionData imports regional data from a SQL dump file into the database.
//
// This function reads a SQL file containing INSERT statements for Indonesian regional data
// (provinces, cities, districts, villages) and executes them against the PostgreSQL database.
// The function handles MySQL-to-PostgreSQL syntax differences and filters out non-INSERT statements.
//
// Parameters:
//   - db: GORM database instance
//   - filePath: Path to the SQL dump file containing regional data
//
// Returns:
//   - error: Any error encountered during the import process
//
// The function performs the following operations:
// 1. Validates the SQL file exists and is readable
// 2. Parses the SQL content into individual statements
// 3. Filters and executes only INSERT statements
// 4. Handles syntax differences between MySQL and PostgreSQL
// 5. Skips comments, CREATE TABLE statements, and other non-INSERT commands
//
// This allows importing large datasets of regional information without manual SQL execution.
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
	tableName := config.ServicePlatform.Get().Database.TbIndonesiaRegion

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

// SeedTelegramUser creates default Telegram users for system operation.
// Each user is assigned properties such as full name, username, phone number, email, user type, user organization, ban status, verification status, and daily quota.
//
// Parameters:
//   - db: GORM database instance
//
// The function checks if any Telegram users already exist to avoid duplicates, ensuring idempotent behavior.
func SeedTelegramUser(db *gorm.DB) {
	var count int64
	db.Model(&telegrammodel.TelegramUsers{}).Count(&count)

	telegramUsers := []telegrammodel.TelegramUsers{
		{
			ChatID:        nil,
			FullName:      "Super Admin - Wegil",
			Username:      "rm_developer",
			PhoneNumber:   "+6287718545247",
			Email:         "wegirandol@smartwebindonesia.com",
			UserType:      telegrammodel.SuperUser,
			UserOf:        telegrammodel.CompanyEmployee,
			IsBanned:      false,
			VerifiedUser:  true,
			MaxDailyQuota: 250,
			Description:   "Super User of Telegram Bot",
		},
	}

	for _, user := range telegramUsers {
		var existingUser telegrammodel.TelegramUsers

		validPhone, err := phonenumbers.Parse(user.PhoneNumber, "ID")
		if err != nil {
			logrus.Printf("Error parsing [ID] phone number %s: %v", user.PhoneNumber, err)
			continue
		}
		user.PhoneNumber = phonenumbers.Format(validPhone, phonenumbers.E164)

		if err := db.Where("phone_number = ?", user.PhoneNumber).First(&existingUser).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				// User with this phone number does not exist, create it
				if err := db.Create(&user).Error; err != nil {
					logrus.Printf("Error creating Telegram user with phone number %s: %v", user.PhoneNumber, err)
				} else {
					logrus.Println("Inserted new Telegram user with phone number:", user.PhoneNumber, "ID:", user.ID)
				}
			} else {
				logrus.Printf("Error checking Telegram user with phone number %s: %v", user.PhoneNumber, err)
			}
		} else {
			// Skip updating existing user
		}
	}

}

// SeedTelegramUserOfSACMS creates Telegram users for SACMS from configuration.
//
// This function reads SACMS user data from the application configuration
// and inserts Telegram user records into the database if they do not already exist.
//
// Parameters:
//   - db: GORM database instance
//
// The function checks for existing users by phone number to avoid duplicates,
// ensuring idempotent behavior.
func SeedTelegramUserOfSACMS(db *gorm.DB) {
	sacData := config.ManageService.Get().ODOOMS.SACData
	if len(sacData) == 0 {
		logrus.Info("No SAC data found in configuration, skipping Telegram UserOf SACMS seeding.")
		return
	}

	for _, sac := range sacData {
		var existingUser telegrammodel.TelegramUsers

		validPhone, err := phonenumbers.Parse(sac.Phone, "ID")
		if err != nil {
			logrus.Printf("Error parsing [ID] phone number %s: %v", sac.Phone, err)
			continue
		}
		sac.Phone = phonenumbers.Format(validPhone, phonenumbers.E164)

		if err := db.Where("phone_number = ?", sac.Phone).First(&existingUser).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				insertSAC := telegrammodel.TelegramUsers{
					ChatID:        nil,
					FullName:      sac.Fullname,
					Username:      sac.Username,
					PhoneNumber:   sac.Phone,
					Email:         sac.Email,
					UserType:      telegrammodel.SACMS,
					UserOf:        telegrammodel.CompanyEmployee,
					IsBanned:      false,
					VerifiedUser:  true,
					MaxDailyQuota: 150,
					Description:   fmt.Sprintf("SAC Region %d", sac.Region),
				}

				// User with this phone number does not exist, create it
				if err := db.Create(&insertSAC).Error; err != nil {
					logrus.Printf("Error creating Telegram user with phone number %s: %v", sac.Phone, err)
				} else {
					logrus.Println("Inserted new Telegram user with phone number:", sac.Phone, "Region:", sac.Region, "ID:", insertSAC.ID)
				}
			} else {
				logrus.Printf("Error checking Telegram user with phone number %s: %v", sac.Phone, err)
			}
		} else {
			// Skip updating existing user
		}
	}

}
