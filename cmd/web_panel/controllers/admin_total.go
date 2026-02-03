package controllers

import (
	"net/http"
	"service-platform/cmd/web_panel/model"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func GetTotalAdmin(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var adminCount int64
		db.Model(&model.Admin{}).Count(&adminCount)

		c.JSON(http.StatusOK, gin.H{"data": adminCount})

	}
}
