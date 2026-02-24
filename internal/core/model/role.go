package model

import (
	"service-platform/internal/config"

	"gorm.io/gorm"
)

// Role represents an application role with associated privileges.
type Role struct {
	gorm.Model
	RoleName  string `json:"role_name" gorm:"column:role_name"`
	ClassName string `json:"class_name" gorm:"column:class_name"`
	Icon      string `json:"icon" gorm:"column:icon"`
	CreatedBy uint   `json:"created_by" gorm:"column:created_by"`
}

// TableName returns the database table name for Role.
func (Role) TableName() string {
	return config.ServicePlatform.Get().Database.TbRole
}

// RolePrivilege represents a privilege granted to a role.
type RolePrivilege struct {
	gorm.Model
	RoleID    uint `json:"role_id" gorm:"column:role_id"`
	FeatureID uint `json:"feature_id" gorm:"column:feature_id"`
	CanCreate int8 `json:"can_create" gorm:"column:can_create"`
	CanRead   int8 `json:"can_read" gorm:"column:can_read"`
	CanUpdate int8 `json:"can_update" gorm:"column:can_update"`
	CanDelete int8 `json:"can_delete" gorm:"column:can_delete"`
}

// TableName returns the database table name for RolePrivilege.
func (RolePrivilege) TableName() string {
	return config.ServicePlatform.Get().Database.TbRolePrivilege
}
