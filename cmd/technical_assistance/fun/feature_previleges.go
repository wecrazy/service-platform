package fun

import (
	"service-platform/cmd/technical_assistance/model"

	"gorm.io/gorm"
)

func CheckUserPreviledges(db *gorm.DB, roleID interface{}, pathName string) bool {
	var user_roles []struct {
		model.RolePrivilege
		Path string `json:"path" gorm:"column:path"`
	}

	if err := db.
		Table("role_privileges rp").
		Unscoped(). // Disable soft deletes for this query
		Select("rp.*,f.path").
		Joins("LEFT JOIN features f ON f.id = rp.feature_id").
		// Where("rp.role_id = ?", roleID).
		Where("rp.role_id = ? AND f.Path = ?", roleID, pathName).
		Find(&user_roles).Error; err != nil {
		return false
	}
	return true
}
