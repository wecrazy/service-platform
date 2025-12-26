package model

import (
	"service-platform/internal/config"

	"gorm.io/gorm"
)

type Role struct {
	gorm.Model
	RoleName  string `json:"role_name" gorm:"column:role_name"`
	ClassName string `json:"class_name" gorm:"column:class_name"`
	Icon      string `json:"icon" gorm:"column:icon"`
	CreatedBy uint   `json:"created_by" gorm:"column:created_by"`
}

func (Role) TableName() string {
	return config.GetConfig().Database.TbRole
}

type RolePrivilege struct {
	gorm.Model
	RoleID    uint `json:"role_id" gorm:"column:role_id"`
	FeatureID uint `json:"feature_id" gorm:"column:feature_id"`
	CanCreate int8 `json:"can_create" gorm:"column:can_create"`
	CanRead   int8 `json:"can_read" gorm:"column:can_read"`
	CanUpdate int8 `json:"can_update" gorm:"column:can_update"`
	CanDelete int8 `json:"can_delete" gorm:"column:can_delete"`
}

func (RolePrivilege) TableName() string {
	return config.GetConfig().Database.TbRolePrivilege
}
