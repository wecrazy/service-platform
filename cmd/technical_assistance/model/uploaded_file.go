package model

import (
	"time"

	"gorm.io/gorm"
)

// UploadedFiles represents the database table structure for uploaded files
type UploadedFiles struct {
	ID        uint           `json:"id" gorm:"primaryKey;autoIncrement;column:id"`
	FileName  string         `json:"file_name" gorm:"column:file_name"`
	FilePath  string         `json:"file_path" gorm:"column:file_path"`
	FileSize  int64          `json:"file_size" gorm:"column:file_size"` // Size in bytes
	UserID    uint           `json:"user_id" gorm:"column:user_id"`     // Assuming files are associated with a user
	CreatedAt time.Time      `json:"created_at" gorm:"column:created_at"`
	UpdatedAt time.Time      `gorm:"column:updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index"`
}

// TableName overrides the default table name for GORM
func (UploadedFiles) TableName() string {
	return "uploaded_files"
}
