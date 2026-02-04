package controllers

import (
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"math/rand"
	"net/http"
	"service-platform/cmd/technical_assistance/fun"
	"service-platform/cmd/technical_assistance/model"
	"service-platform/internal/config"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"gorm.io/gorm"
)

// Handle Main Page
func MainPage(db *gorm.DB, redisDB *redis.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		cookies := c.Request.Cookies()

		// Parse JWT token from cookie
		tokenString, err := c.Cookie("token")
		if err != nil {
			fun.ClearCookiesAndRedirect(c, cookies)
			return
		}
		tokenString = strings.ReplaceAll(tokenString, " ", "+")

		decrypted, err := fun.GetAESDecrypted(tokenString)
		if err != nil {
			fmt.Println("Error during decryption", err)
			fun.ClearCookiesAndRedirect(c, cookies)
			return
		}
		var claims map[string]interface{}
		err = json.Unmarshal(decrypted, &claims)
		if err != nil {
			fmt.Printf("Error converting JSON to map: %v", err)
			fun.ClearCookiesAndRedirect(c, cookies)
			return
		}
		emailToken := claims["email"].(string)
		if emailToken == "" {
			fun.ClearCookiesAndRedirect(c, cookies)
			return
		}
		// Validate additional cookies
		if !fun.ValidateCookie(c, "credentials", claims["session"]) ||
			!fun.ValidateCookie(c, "auth", claims["auth"]) ||
			!fun.ValidateCookie(c, "random", claims["random"]) {
			fun.RemoveEmailSession(db, emailToken)
			fun.ClearCookiesAndRedirect(c, cookies)
			return
		}
		session, ok := claims["session"].(string)
		if !ok {
			fun.RemoveEmailSession(db, emailToken)
			fun.ClearCookiesAndRedirect(c, cookies)
			return
		}
		var admin model.Admin
		resultAdmin := db.Where("id = ?", claims["id"]).First(&admin)
		if resultAdmin.Error != nil {
			fun.RemoveEmailSession(db, emailToken)
			fun.ClearCookiesAndRedirect(c, cookies)
			return
		}
		fmt.Println("id = ", admin.ID)
		fmt.Println("session = ", admin.Session)
		fmt.Println("session_send = ", session)
		if admin.Session != session {
			fun.RemoveEmailSession(db, emailToken)
			fun.ClearCookiesAndRedirect(c, cookies)
			return
		}
		var featuresPrivileges []struct {
			model.RolePrivilege
			ParentID  uint   `json:"parent_id" gorm:"column:parent_id"`
			Title     string `json:"title" gorm:"column:title"`
			Path      string `json:"path" gorm:"column:path"`
			MenuOrder uint   `json:"menu_order" gorm:"column:menu_order"`
			Status    uint   `json:"status" gorm:"column:status"`
			Level     uint   `json:"level" gorm:"column:level"`
			Icon      string `json:"icon" gorm:"column:icon"`
		}

		if err := db.
			Table("role_privileges a").
			Unscoped(). // Disable soft deletes for this query
			Select("a.*, b.parent_id , b.title , b.path , b.menu_order , b.status , b.level , b.icon").
			Joins("LEFT JOIN features b ON a.feature_id = b.id").
			Where("a.role_id = ?", claims["role"]).
			Order("b.menu_order").
			Find(&featuresPrivileges).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				fun.RemoveEmailSession(db, emailToken)
				fun.ClearCookiesAndRedirect(c, cookies)
				return
			}

			// Handle other errors
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "Error querying database" + err.Error(),
			})
			fun.RemoveEmailSession(db, emailToken)
			fun.ClearCookiesAndRedirect(c, cookies)
			return
		}

		fileContent := ""

		fileContentTab := ""
		for _, featurePrivilegeParent := range featuresPrivileges {
			fileContentChild := ""
			menuToggle := ""

			if len(strings.TrimSpace(featurePrivilegeParent.Path)) == 0 {
				for _, featurePrivilege := range featuresPrivileges {
					if featurePrivilege.ParentID == featurePrivilegeParent.MenuOrder {
						if featurePrivilege.Create == 0 &&
							featurePrivilege.Read == 0 &&
							featurePrivilege.Update == 0 &&
							featurePrivilege.Delete == 0 &&
							featurePrivilegeParent.Status == 0 {
							continue
						}
						fileContentChild += `        
							<li class="menu-item">
								<a href="#` + featurePrivilege.Path + `" class="menu-link">
									<div class="text-truncate" data-i18n="` + featurePrivilege.Title + `">` + featurePrivilege.Title + `</div>
								</a>
							</li>`
					}
				}

				if len(fileContentChild) > 0 {
					fileContentChild = `<ul class="menu-sub">` + fileContentChild + `</ul>`
					menuToggle = "menu-toggle"
				}
			}

			if featurePrivilegeParent.Level == 0 && featurePrivilegeParent.Status == 1 {
				hrefPath := ""
				if len(featurePrivilegeParent.Path) != 0 {
					if featurePrivilegeParent.Create == 0 &&
						featurePrivilegeParent.Read == 0 &&
						featurePrivilegeParent.Update == 0 &&
						featurePrivilegeParent.Delete == 0 {
						continue
					}
					hrefPath = `href="#` + featurePrivilegeParent.Path + `"`
				} else {
					if len(fileContentChild) == 0 {
						if featurePrivilegeParent.Create == 0 &&
							featurePrivilegeParent.Read == 0 &&
							featurePrivilegeParent.Update == 0 &&
							featurePrivilegeParent.Delete == 0 {
							continue
						}
					}
				}

				fileContent += `
					<li class="menu-item ">
						<a ` + hrefPath + ` class="menu-link ` + menuToggle + `">
							<i class="menu-icon tf-icons bx ` + featurePrivilegeParent.Icon + `"></i>
							<div class="text-truncate" data-i18n="` + featurePrivilegeParent.Title + `">` + featurePrivilegeParent.Title + `</div>
						</a>
						` + fileContentChild + `
					</li>
				`
			}

			if len(featurePrivilegeParent.Path) > 0 {
				fileContentTab += `<div id="` + featurePrivilegeParent.Path + `" class="tab-content flex-grow-1 container-p-y d-none h-100"></div>` //` + string(fileContent) + `
			}
		}

		randomAccessToken := fun.GenerateRandomString(20 + rand.Intn(30) + 1)
		err = redisDB.Set(context.Background(), "web:"+session, randomAccessToken, 0).Err()
		if err != nil {
			fun.RemoveEmailSession(db, emailToken)
			fun.ClearCookiesAndRedirect(c, cookies)
			return
		}
		imageMaps := map[string]interface{}{
			"t":  fun.GenerateRandomString(3),
			"id": admin.ID,
		}
		pathString, err := fun.GetAESEcryptedURLfromJSON(imageMaps)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not encripting image " + err.Error()})
			return
		}
		profile_image := fun.GLOBAL_URL + "profile/default.jpg?f=" + pathString

		c.HTML(http.StatusOK, "index.html", gin.H{
			"APP_NAME":         config.TechnicalAssistance.Get().APP_NAME,
			"APP_LOGO":         config.TechnicalAssistance.Get().APP_LOGO,
			"APP_VERSION":      config.TechnicalAssistance.Get().APP_VERSION_NO,
			"APP_VERSION_NO":   config.TechnicalAssistance.Get().APP_VERSION_NO,
			"APP_VERSION_CODE": config.TechnicalAssistance.Get().APP_VERSION_CODE,
			"APP_VERSION_NAME": config.TechnicalAssistance.Get().APP_VERSION_NAME,
			"ACCESS":           "web/" + randomAccessToken,
			"username":         claims["username"],
			"role":             claims["role_name"],
			"profile_image":    profile_image,
			"GLOBAL_URL":       fun.GLOBAL_URL,
			"sidebar":          template.HTML(string(fileContent)),
			"contents":         template.HTML(string(fileContentTab)),
			"IsNotDebug":       true,
		})

	}
}
