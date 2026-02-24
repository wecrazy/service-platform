package controllers

import (
	"os"
	"service-platform/internal/config"
	"service-platform/internal/core/model"
	"service-platform/pkg/fun"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// GetUserProfile godoc
// @Summary      Get User Profile Image
// @Description  Retrieves the profile image for a user based on encrypted ID
// @Tags         User
// @Accept       json
// @Produce      image/jpeg
// @Param        f   query      string  true  "Encrypted User ID"
// @Success      200  {file}     file
// @Failure      404  {string}   string "Not Found"
// @Router       /user/profile [get]
func GetUserProfile(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Content-Type", "image/jpeg")

		avatarDir, err := fun.FindValidDirectory([]string{
			"web/assets/img/avatars",
			"../web/assets/img/avatars",
			"../../web/assets/img/avatars",
			"../../../web/assets/img/avatars",
		})

		if err != nil {
			logrus.Errorf("avatar directory not found: %v", err)
			c.Status(404)
			return
		}

		filePath := avatarDir + "/default.jpg"

		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			logrus.Errorf("default profile image not found: %v", err)
			c.Status(404)
			return
		}

		pathParam := c.Query("f")
		claims, err := fun.GetAESDecryptedURLtoJSON(pathParam)
		if err != nil {
			logrus.Errorf("got error during decryption: %v", err)
			c.File(filePath)
			return
		}
		var user model.Users
		if err := db.Where("id = ?", claims["id"]).First(&user).Error; err != nil {
			logrus.Errorf("got error: %v", err)
			c.File(filePath)
			return
		}

		if user.Session == "" || user.SessionExpired == 0 {
			// logrus.Errorf("no session found for %s", user.Email)
			c.File(filePath)
			return
		}
		if user.ProfileImage == "" {
			c.File(filePath)
			return
		}

		if _, err := os.Stat(user.ProfileImage); os.IsNotExist(err) {
			// Try find it in another dir
			dirToSearch := []string{
				user.ProfileImage,
				"../" + user.ProfileImage,
				"../../" + user.ProfileImage,
				"../../../" + user.ProfileImage,
				config.ServicePlatform.Get().App.StaticDir + "/" + user.ProfileImage,
			}
			foundPath, err := fun.FindValidFile(dirToSearch)
			if err != nil {
				logrus.Errorf("cannot find profile image in any known directory: %v", err)
				c.File(filePath)
				return
			}
			filePath = foundPath
		}

		// Open the file
		file, err := os.Open(filePath)
		if err != nil {
			logrus.Errorf("cannot opening file: %v", err)
			c.File(filePath)
			return
		}
		defer file.Close()

		// Serve the file
		c.File(filePath)
	}
}
