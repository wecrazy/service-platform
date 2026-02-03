package gormdb

import "gorm.io/gorm"

type DBUsed struct {
	Web      *gorm.DB
	FastLink *gorm.DB
	TA       *gorm.DB
	WebTA    *gorm.DB
}

// Global databases
var Databases *DBUsed
