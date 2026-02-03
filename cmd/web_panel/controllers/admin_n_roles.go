package controllers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"service-platform/cmd/web_panel/fun"
	"service-platform/cmd/web_panel/model"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

func PostNewAdminUser(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		loginCookie, err := c.Request.Cookie("credentials")
		if err != nil || loginCookie == nil || loginCookie.Value == "" {
			// If cookie doesn't exist or is empty, redirect to login
			expiredCookie := http.Cookie{
				Name:    "credentials",
				Expires: time.Now().AddDate(0, 0, -1),
			}
			http.SetCookie(c.Writer, &expiredCookie)
			c.Redirect(http.StatusFound, "/login")
			return
		}

		var adminLogin model.Admin
		if err := db.Where("session = ?", loginCookie.Value).First(&adminLogin).Error; err != nil || adminLogin.ID == 0 {
			loginCookie.Expires = time.Now().AddDate(0, 0, -1)
			http.SetCookie(c.Writer, loginCookie)
			c.Redirect(302, "/login")
			return
		}

		var userData struct {
			FullName   string `json:"userFullname" binding:"required"`
			UserName   string `json:"username"`
			Email      string `json:"userEmail" binding:"required"`
			Password   string `json:"userPassword" binding:"required"`
			Phone      string `json:"userPhone" binding:"required"`
			Company    string `json:"companyName" binding:"required"`
			Role       string `json:"role" binding:"required"`
			UserStatus string `json:"userStatus" binding:"required"`
		}
		if err := c.ShouldBind(&userData); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		// Validate the password
		if err := fun.ValidatePassword(userData.Password); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		// fmt.Print(userData)

		userData.Password = fun.GenerateSaltedPassword(userData.Password)

		var adminStatus model.AdminStatus
		if err := db.First(&adminStatus, "title = ?", userData.UserStatus).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "Error Getting Status" + err.Error(),
			})
			return
		}

		var role model.Role
		roleName := strings.TrimSpace(userData.Role)
		if err := db.First(&role, "role_name = ?", roleName).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": fmt.Sprintf("Error getting role: %v, from role name: %s", err, roleName),
			})
			return
		}

		userData.UserName = userData.Email
		// Check if any existing record matches username, email, or phone
		var existingAdmin model.Admin
		db.Where("email = ? OR phone = ?", userData.Email, userData.Phone).First(&existingAdmin)

		if existingAdmin.ID != 0 {
			fmt.Println(existingAdmin)
			c.JSON(http.StatusConflict, gin.H{
				"error": "User with similar username, email, or phone already exists",
			})
			return
		}

		// If no similar record exists, create the new admin
		admin := model.Admin{
			Fullname:  userData.FullName,
			Username:  userData.UserName,
			Phone:     userData.Phone,
			Email:     userData.Email,
			Password:  userData.Password,
			LastLogin: time.Now(),
			Type:      0,
			Role:      int(role.ID),
			Status:    int(adminStatus.ID),
			CreateBy:  int(adminLogin.ID),
			UpdateBy:  int(adminLogin.ID),
		}

		// Use Gorm to create the record
		if err := db.Create(&admin).Error; err != nil {
			logrus.Errorf("Error creating user: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "Error creating user" + err.Error(),
			})
			return
		}
		// Save the newLog to the database
		db.Create(&model.LogActivity{
			AdminID:   adminLogin.ID,
			FullName:  adminLogin.Fullname,
			Action:    "Created New Admin",
			Status:    "Success",
			Log:       "New Admin " + userData.FullName + " Created successfully",
			IP:        c.ClientIP(),
			UserAgent: c.Request.UserAgent(),
			ReqMethod: c.Request.Method,
			ReqUri:    c.Request.RequestURI,
		})
		//Why not inserted and status OK?
		c.JSON(http.StatusOK, gin.H{"status": "OK", "message": "New Admin Created successfully"})
	}
}
func PatchAdminStatus(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		loginCookie, err := c.Request.Cookie("credentials")
		if err != nil || loginCookie == nil || loginCookie.Value == "" {
			// If cookie doesn't exist or is empty, redirect to login
			expiredCookie := http.Cookie{
				Name:    "credentials",
				Expires: time.Now().AddDate(0, 0, -1),
			}
			http.SetCookie(c.Writer, &expiredCookie)
			c.Redirect(http.StatusFound, "/login")
			return
		}

		var adminLogin model.Admin
		if err := db.Where("session = ?", loginCookie.Value).First(&adminLogin).Error; err != nil || adminLogin.ID == 0 {
			loginCookie.Expires = time.Now().AddDate(0, 0, -1)
			http.SetCookie(c.Writer, loginCookie)
			c.Redirect(302, "/login")
			return
		}

		var userData struct {
			ID     string `json:"id"`
			Status string `json:"status"`
		}
		if err := c.ShouldBind(&userData); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		var admin model.Admin
		if err := db.First(&admin, userData.ID).Error; err != nil {
			logrus.Errorf("Error finding user: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "Error finding user: " + err.Error(),
			})
			return
		}
		statusInt, err := strconv.Atoi(userData.Status)
		if err != nil {
			// Handle the error, e.g., if the string is not a valid integer
			fmt.Println("Error parsing integer:", err)
			return
		}

		// Update the status
		admin.Status = statusInt

		// Save the updated record
		if err := db.Save(&admin).Error; err != nil {
			logrus.Errorf("Error updating user status: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "Error updating user status: " + err.Error(),
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "User status updated successfully"})
		// Save the newLog to the database
		db.Create(&model.LogActivity{
			AdminID:   adminLogin.ID,
			FullName:  adminLogin.Fullname,
			Action:    "UPDATE",
			Status:    "Success",
			Log:       "Update Admin Status",
			IP:        c.ClientIP(),
			UserAgent: c.Request.UserAgent(),
			ReqMethod: c.Request.Method,
			ReqUri:    c.Request.RequestURI,
		})

	}
}
func PatchAdminData(db *gorm.DB) gin.HandlerFunc {
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
		var data struct {
			ID    string `json:"id"`
			Field string `json:"field"`
			Value string `json:"value"`
		}
		if err := c.ShouldBindJSON(&data); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		switch data.Field {
		case "fullname", "username":

		case "phone":
			pattern := `^\d{10,}$`
			// Compile the regular expression
			re := regexp.MustCompile(pattern)
			if !re.MatchString(data.Value) {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid Email name"})
				return
			}
		case "email":
			pattern := `^[^@]+@[^@.]+\..+$`
			// Compile the regular expression
			re := regexp.MustCompile(pattern)
			if !re.MatchString(data.Value) {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid Email name"})
				return
			}

		case "role":
			var role model.Role
			if err := db.Where("role_name = ?", data.Value).First(&role).Error; err != nil {
				if err == gorm.ErrRecordNotFound {
					c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid Role name"})
				} else {
					c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
				}
				return
			}

			if role.ID == 0 {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid Role name"})
				return
			}
			data.Value = strconv.Itoa(int(role.ID))

		case "status":
			var status model.AdminStatus
			if err := db.Where("title = ?", data.Value).First(&status).Error; err != nil {
				if err == gorm.ErrRecordNotFound {
					c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid Role name"})
				} else {
					c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
				}
				return
			}
			if status.ID == 0 {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid Status name"})
				return
			}
			data.Value = strconv.Itoa(int(status.ID))

		default:
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid field name"})
			return
		}

		var admin model.Admin
		// Find the record by ID or other unique identifier
		if err := db.Where("id = ?", data.ID).First(&admin).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Record not found"})
			return
		}

		// Update the field with the new value
		if err := db.Model(&admin).Update(data.Field, data.Value).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update record"})
			return
		}

		c.JSON(http.StatusOK, admin)

		db.Create(&model.LogActivity{
			AdminID:   uint(claims["id"].(float64)),
			FullName:  claims["fullname"].(string),
			Action:    "UPDATE",
			Status:    "Success",
			Log:       fmt.Sprintf("UPDATE Admin Data @ ID: %s; Field : %s; Value: %s; ", data.ID, data.Field, data.Value),
			IP:        c.ClientIP(),
			UserAgent: c.Request.UserAgent(),
			ReqMethod: c.Request.Method,
			ReqUri:    c.Request.RequestURI,
		})

	}
}

func DeleteUserAdmin(db *gorm.DB) gin.HandlerFunc {
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

		id := c.Param("id")
		if id == "" {
			c.JSON(http.StatusNotFound, gin.H{"error": "ID not Found"})
			return
		}
		if strconv.Itoa(int(claims["id"].(float64))) == id {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Cannot Self Delete"})
			return
		}

		var user model.Admin
		if err := db.First(&user, id).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
			} else {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Error finding role:" + err.Error()})
			}
			return
		}
		// Check the user's status
		if user.Status == 1 || user.Status == 2 {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Must be INACTIVE User"})
			return
		}

		if err := db.Delete(&user).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error finding role:" + err.Error()})
			return
		}
		// Respond with a success message
		c.JSON(http.StatusOK, gin.H{"message": "User deleted successfully"})
		// Save the newLog to the database
		db.Create(&model.LogActivity{
			AdminID:   uint(claims["id"].(float64)),
			FullName:  claims["fullname"].(string),
			Action:    "DELETE",
			Status:    "Success",
			Log:       "Delete Admin User",
			IP:        c.ClientIP(),
			UserAgent: c.Request.UserAgent(),
			ReqMethod: c.Request.Method,
			ReqUri:    c.Request.RequestURI,
		})

	}
}

// Handle GetAdminTable
func GetAdminTable(db *gorm.DB) gin.HandlerFunc {
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
		privileges := claims["tab-roles"].(string)
		update := string(privileges[2])
		delete := string(privileges[3])
		// Variable to hold the results
		var adminStatuses []model.AdminStatus
		result := db.Find(&adminStatuses)
		if result.Error != nil {
			fmt.Println("Error occurred:", result.Error)
			return
		}
		var adminStatusMaps []gin.H
		for _, adminStatus := range adminStatuses {
			adminStatusMaps = append(adminStatusMaps, gin.H{
				"id":         adminStatus.ID,
				"title":      adminStatus.Title,
				"class_name": adminStatus.ClassName,
			})
		}
		// Variable to hold the results
		var adminRoles []model.Role
		resultRole := db.Find(&adminRoles)
		if resultRole.Error != nil {
			fmt.Println("Error occurred:", resultRole.Error)
			return
		}
		var adminRolesMaps []gin.H
		for _, adminRoles := range adminRoles {
			adminRolesMaps = append(adminRolesMaps, gin.H{
				"id":         adminRoles.ID,
				"title":      adminRoles.RoleName,
				"class_name": adminRoles.ClassName,
				"icon":       adminRoles.Icon,
			})
		}
		var admins []model.Admin
		resultAdmin := db.Find(&admins)
		if resultAdmin.Error != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "Error querying database",
			})
			return
		}
		// Format the result as needed
		var data []gin.H
		for _, admin := range admins {
			deletePrv := delete
			updatePrv := update
			if admin.ID == uint(claims["id"].(float64)) {
				deletePrv = "0"
				updatePrv = "0"
			}
			role_name := ""
			for _, adminRoles := range adminRoles {
				if admin.Role == int(adminRoles.ID) {
					role_name = adminRoles.RoleName
					break
				}
			}

			data = append(data, gin.H{
				"id":        admin.ID,
				"full_name": admin.Fullname,
				"role_id":   admin.Role,
				"role":      role_name,
				"username":  admin.Username,
				"phone":     admin.Phone,
				"type":      admin.Type,
				"email":     admin.Email,
				"status":    admin.Status,
				"avatar":    admin.ProfileImage,
				"statuses":  adminStatusMaps,
				"roles":     adminRolesMaps,
				"updating":  updatePrv,
				"deleting":  deletePrv,
			})
		}

		// Respond with the formatted data
		c.JSON(http.StatusOK, gin.H{
			"data": data,
		})
	}
}

// Handle getRoles
func GetRolesGui(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {

		fileContent := ""

		var rolesAndTotals []struct {
			model.Role
			Total string `json:"total_admin" gorm:"column:total_admin"`
		}
		query := "SELECT r.*, COUNT(a.`role`) AS total_admin FROM roles r LEFT JOIN admins a ON r.id = a.`role` WHERE r.deleted_at IS NULL GROUP BY r.id;"
		db.Raw(query).Scan(&rolesAndTotals)

		for _, rolesAndTotal := range rolesAndTotals {
			var results []struct {
				FullName string `json:"fullname" gorm:"column:fullname"`
			}
			query := `SELECT fullname FROM admins WHERE role = ? LIMIT 5`
			db.Raw(query, rolesAndTotal.ID).Scan(&results)

			roleID := strconv.FormatInt(int64(rolesAndTotal.ID), 10)
			deleteBtn := ``
			if rolesAndTotal.ID != 1 {
				deleteBtn = `
				<a href="javascript:void(0);" 
				data-bs-toggle="tooltip"
				data-popup="tooltip-custom"
				data-bs-placement="top"
				title="DELETE" 
				class="text-danger"
				onclick="deleteRole('` + roleID + `' , '` + rolesAndTotal.RoleName + `')"
				>
				<i class='bx bx-message-square-x bx-rotate-90'></i>
			</a>`
			}
			// Use the Icon if available, otherwise default to a generic icon
			roleIcon := rolesAndTotal.Icon
			if roleIcon == "" {
				roleIcon = "fal fa-user-circle" // Default icon
			}

			// Use the ClassName if available for additional styling
			cardClass := "card"
			if rolesAndTotal.ClassName != "" {
				cardClass += " " + rolesAndTotal.ClassName
			}

			// Handle proper grammar for user count
			var userCountText string
			switch rolesAndTotal.Total {
			case "0":
				userCountText = "No users found yet"
			case "1":
				userCountText = "Total 1 user"
			default:
				userCountText = "Total " + rolesAndTotal.Total + " users"
			}

			fileContent += `
			<div class="col-xl-4 col-lg-6 col-md-6">
				<div class="` + cardClass + `">
					<div class="card-body position-relative">
						<div class="d-flex justify-content-between mb-1">
							<h6 class="fw-normal">` + userCountText + `</h6>
							<div class="role-icon position-absolute mt-3" style="top: 15px; right: 20px;">
								<i class="` + roleIcon + ` text-primary" style="font-size: 3.5rem; opacity: 0.25;"></i>
							</div>
						</div>
						<div class="d-flex justify-content-between align-items-end">
							<div class="role-heading" style="max-width: 70%;">
								<h4 class="mb-4 role-name d-flex align-items-center" onclick="filterRole('` + rolesAndTotal.RoleName + `')">
									` + rolesAndTotal.RoleName + `
								</h4>
								<a
									href="javascript:;"
									data-bs-toggle="modal"
									data-bs-target="#addRoleModal"
									class="role-edit-modal text-warning d-flex"
									onclick="editRole('` + roleID + `' , '` + rolesAndTotal.RoleName + `')"
									><i class='bx bxs-edit' ></i> Edit Role</a
								>
							</div>
							<div class="role-actions">
								` + deleteBtn + `
								<a href="javascript:void(0);"	
									data-popup="tooltip-custom"
									data-bs-placement="top"
									title="DUPLICATE" 
									data-bs-toggle="modal"
									data-bs-target="#addRoleModal"
									class="text-info role-edit-modal"
									onclick="editRole('` + roleID + `' , '` + rolesAndTotal.RoleName + `', true)"
									>
									<i class='bx bx-copy'></i>
								</a>
							</div>
						</div>
					</div>
				</div>
			</div>
			`
		}
		fileContent +=
			`<div class="col-xl-4 col-lg-6 col-md-6">
			<div class="card h-100">
			<div class="row h-100">
				<div class="col-sm-5">
				<div class="d-flex align-items-end h-100 justify-content-center mt-sm-0 mt-3">
					<img
					src="../assets/img/illustrations/sitting-girl-with-laptop-light.png"
					class="img-fluid"
					alt="Image"
					width="120"
					data-app-light-img="illustrations/sitting-girl-with-laptop-light.png"
					data-app-dark-img="illustrations/sitting-girl-with-laptop-dark.png" />
				</div>
				</div>
				<div class="col-sm-7">
				<div class="card-body text-sm-end text-center ps-sm-0">
					<button
					data-bs-target="#addRoleModal"
					data-bs-toggle="modal"
					class="btn btn-outline-primary mb-3 text-nowrap add-new-role"
					onclick="editRole(0, '')">
					<i class="far fa-plus me-2 fs-3"></i> New Role
					</button>
					<p class="mb-0">Create and configure a new role with custom permissions if the desired role doesn't exist in the system yet</p>
				</div>
				</div>
			</div>
			</div>
		</div>`

		c.JSON(http.StatusOK, gin.H{
			"data": fileContent,
		})
	}
}

// Handle getRoles
func GetRolesList(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var roles []model.Role
		db.Find(&roles)
		var roleData []string
		for _, role := range roles {
			roleData = append(roleData, role.RoleName)
		}

		c.JSON(http.StatusOK, gin.H{
			"data": roleData,
		})
	}
}
func PatchAdminRoles(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		loginCookie, err := c.Request.Cookie("credentials")
		if err != nil || loginCookie == nil || loginCookie.Value == "" {
			// If cookie doesn't exist or is empty, redirect to login
			expiredCookie := http.Cookie{
				Name:    "credentials",
				Expires: time.Now().AddDate(0, 0, -1),
			}
			http.SetCookie(c.Writer, &expiredCookie)
			c.Redirect(http.StatusFound, "/login")
			return
		}

		var adminLogin model.Admin
		if err := db.Where("session = ?", loginCookie.Value).First(&adminLogin).Error; err != nil || adminLogin.ID == 0 {
			loginCookie.Expires = time.Now().AddDate(0, 0, -1)
			http.SetCookie(c.Writer, loginCookie)
			c.Redirect(302, "/login")
			return
		}
		var req struct {
			ID       string `json:"id"`
			Username string `json:"username"`
			Rolename string `json:"rolename"`
		}

		req.ID = c.Request.FormValue("id")
		req.Username = c.Request.FormValue("username")
		req.Rolename = c.Request.FormValue("rolename")
		if req.ID == "" && req.Username == "" && req.Rolename == "" {
			c.JSON(http.StatusBadRequest, gin.H{"status": "failed"})
			return
		}
		var admin model.Admin
		if err := db.Where("id = ?", req.ID).First(&admin).Error; err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		var role model.Role
		if err := db.Where("role_name = ?", req.Rolename).First(&role).Error; err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		admin.Role = int(role.ID)

		// var role model.Role
		if err := db.Save(&admin).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error updating database record"})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"message": "success",
		})
		// Save the newLog to the database
		db.Create(&model.LogActivity{
			AdminID:   adminLogin.ID,
			FullName:  adminLogin.Fullname,
			Action:    "PUT",
			Status:    "Success",
			Log:       "Change Admin Roles",
			IP:        c.ClientIP(),
			UserAgent: c.Request.UserAgent(),
			ReqMethod: c.Request.Method,
			ReqUri:    c.Request.RequestURI,
		})

	}
}
func DeleteRoles(db *gorm.DB) gin.HandlerFunc {
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

		roleID := c.Query("data")
		roleName := c.Query("rolename")

		if roleID == "" || roleID == "1" || roleName == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Request Not Valid"})
		}
		var admins []model.Admin
		resultAdmin := db.Where("role = ?", roleID).Find(&admins)
		if resultAdmin.Error != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "There Is Active User"})
			return
		}
		if len(admins) > 0 {
			for _, admin := range admins {
				if admin.Status == 1 || admin.Status == 2 {
					c.JSON(http.StatusBadRequest, gin.H{"error": "There Is Active User" + admin.Fullname})
					return
				}
			}
		}

		var role model.Role
		// Find the record to delete
		result := db.Where("id = ? AND role_name = ?", roleID, roleName).First(&role)
		if result.Error != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "No valid Role"})
			return
		}

		// Delete the record
		result = db.Delete(&role)
		if result.Error != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Something Wrong in Delete Role"})
			return
		}

		result2 := db.Model(&model.Admin{}).Where("role = ?", roleID).Updates(map[string]interface{}{"role": 0, "status": 0})
		if result2.Error != nil {
			fmt.Println("Error updating the role:", result2.Error)
			return
		}

		lastLoginStr := fmt.Sprintf("user Afected :  %d", result2.RowsAffected)

		c.JSON(http.StatusOK, gin.H{
			"msg": lastLoginStr,
		})
		// Save the newLog to the database
		db.Create(&model.LogActivity{
			AdminID:   uint(claims["id"].(float64)),
			FullName:  claims["fullname"].(string),
			Action:    "DELETE",
			Status:    "Success",
			Log:       "Delete Role " + roleName,
			IP:        c.ClientIP(),
			UserAgent: c.Request.UserAgent(),
			ReqMethod: c.Request.Method,
			ReqUri:    c.Request.RequestURI,
		})

	}
}
func PostRole(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		// check cookies
		cookies := c.Request.Cookies()

		var credentialsCookie *http.Cookie
		for _, cookie := range cookies {
			if cookie.Name == "credentials" {
				credentialsCookie = cookie
				break
			}
		}
		if credentialsCookie == nil || len(credentialsCookie.Value) == 0 {
			c.Redirect(302, "/login")
			fun.ClearCookiesAndRedirect(c, cookies)
			return
		}

		var adminsWithRoles struct {
			model.Admin
			RoleName string `json:"role_name" gorm:"column:role_name"`
		}

		if err := db.
			Table("admins a").
			Unscoped(). // Disable soft deletes for this query
			Select("a.*, b.role_name").
			Joins("LEFT JOIN roles b ON a.role = b.id").
			Where("a.session = ?", credentialsCookie.Value).
			Limit(1).
			Find(&adminsWithRoles).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				credentialsCookie.Expires = time.Now().AddDate(0, 0, -1)
				http.SetCookie(c.Writer, credentialsCookie)
				c.Redirect(302, "/login")
				return
			}

			// Handle other errors
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "Error querying database" + err.Error(),
			})
			return
		}

		roleID := c.Request.FormValue("role_id")
		if roleID == "0" {
			modalRoleName := c.Request.FormValue("modalRoleName")
			regexpObject := regexp.MustCompile(`["'<>\\/]`)
			if regexpObject.MatchString(modalRoleName) {
				c.JSON(http.StatusBadRequest, gin.H{"error": `Error must not contain "<>\\/`})
				return
			}

			var role model.Role
			db.Where("role_name = ?", modalRoleName).First(&role)
			if role.ID != 0 {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Error Similar Role Name"})
				return
			}

			role1 := model.Role{
				RoleName:  modalRoleName,
				CreatedBy: adminsWithRoles.ID,
			}

			_ = db.Transaction(func(tx *gorm.DB) error {
				tx.Create(&role1)
				return nil
			})

			var features []model.Feature
			if result := db.Find(&features); result.Error != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Error : " + result.Error.Error()})
				return
			}

			// Print the selected features
			for _, feature := range features {
				var roleCreate int8
				var roleRead int8
				var roleUpdate int8
				var roleDelete int8
				featureIDStr := strconv.FormatUint(uint64(feature.ID), 10)
				if c.Request.FormValue("roleCreate-"+featureIDStr) == "on" {
					roleCreate = 1
				} else {
					roleCreate = 0
				}
				if c.Request.FormValue("roleRead-"+featureIDStr) == "on" {
					roleRead = 1
				} else {
					roleRead = 0
				}
				if c.Request.FormValue("roleUpdate-"+featureIDStr) == "on" {
					roleUpdate = 1
				} else {
					roleUpdate = 0
				}
				if c.Request.FormValue("roleDelete-"+featureIDStr) == "on" {
					roleDelete = 1
				} else {
					roleDelete = 0
				}
				db.Create(&model.RolePrivilege{
					RoleID:    role1.ID,
					FeatureID: feature.ID,
					Create:    roleCreate,
					Read:      roleRead,
					Update:    roleUpdate,
					Delete:    roleDelete,
				})
			}
			// Save the newLog to the database
			db.Create(&model.LogActivity{
				AdminID:   adminsWithRoles.ID,
				FullName:  adminsWithRoles.Fullname,
				Action:    "CREATE",
				Status:    "Success",
				Log:       "Create New Roles Named " + modalRoleName,
				IP:        c.ClientIP(),
				UserAgent: c.Request.UserAgent(),
				ReqMethod: c.Request.Method,
				ReqUri:    c.Request.RequestURI,
			})
		} else {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Error Creating new Role"})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"status":  "success",
			"message": "Data successfully updated",
		})
	}
}
func PatchRole(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		// check cookies
		cookies := c.Request.Cookies()

		var credentialsCookie *http.Cookie
		for _, cookie := range cookies {
			if cookie.Name == "credentials" {
				credentialsCookie = cookie
				break
			}
		}
		// if credentialsCookie == nil || len(credentialsCookie.Value) == 0 {
		// 	c.Redirect(302, "/login")
		// 	return
		// } else if credentialsCookie != nil && credentialsCookie.Value == "" {
		// 	credentialsCookie.Expires = time.Now().AddDate(0, 0, -1)
		// 	http.SetCookie(c.Writer, credentialsCookie)
		// 	c.Redirect(302, "/login")
		// 	return
		// }
		// If cookie is nil or has an empty value, redirect to login
		if credentialsCookie == nil || credentialsCookie.Value == "" {
			if credentialsCookie != nil { // Only attempt to expire if the cookie exists
				credentialsCookie.Expires = time.Now().AddDate(0, 0, -1)
				http.SetCookie(c.Writer, credentialsCookie)
			}
			c.Redirect(302, "/login")
			return
		}

		var adminsWithRoles struct {
			model.Admin
			RoleName string `json:"role_name" gorm:"column:role_name"`
		}

		if err := db.
			Table("admins a").
			Unscoped(). // Disable soft deletes for this query
			Select("a.*, b.role_name").
			Joins("LEFT JOIN roles b ON a.role = b.id").
			Where("a.session = ?", credentialsCookie.Value).
			Limit(1).
			Find(&adminsWithRoles).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				credentialsCookie.Expires = time.Now().AddDate(0, 0, -1)
				http.SetCookie(c.Writer, credentialsCookie)
				c.Redirect(302, "/login")
				return
			}

			// Handle other errors
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "Error querying database" + err.Error(),
			})
			return
		}

		roleID := c.Request.FormValue("role_id")
		if roleID == "0" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Error updating Role"})
			return
		}
		var role model.Role
		if err := db.Where("id = ?", roleID).Find(&role).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error querying database"})
			return
		}
		regexpObject := regexp.MustCompile(`["'<>\\/]`)
		if regexpObject.MatchString(c.Request.FormValue("modalRoleName")) {
			c.JSON(http.StatusBadRequest, gin.H{"error": `Error must not contain "<>\\/`})
			return
		}
		role.RoleName = c.Request.FormValue("modalRoleName")

		if err := db.Save(&role).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error updating database record"})
			return
		}

		var rolePrivileges []model.RolePrivilege
		if err := db.Where("role_id = ?", roleID).Find(&rolePrivileges).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error querying database"})
			return
		}

		for _, rolePrivilege := range rolePrivileges {
			featureIDStr := strconv.FormatUint(uint64(rolePrivilege.FeatureID), 10)
			if c.Request.FormValue("roleCreate-"+featureIDStr) == "on" {
				rolePrivilege.Create = 1
			} else {
				rolePrivilege.Create = 0
			}
			if c.Request.FormValue("roleRead-"+featureIDStr) == "on" {
				rolePrivilege.Read = 1
			} else {
				rolePrivilege.Read = 0
			}
			if c.Request.FormValue("roleUpdate-"+featureIDStr) == "on" {
				rolePrivilege.Update = 1
			} else {
				rolePrivilege.Update = 0
			}
			if c.Request.FormValue("roleDelete-"+featureIDStr) == "on" {
				rolePrivilege.Delete = 1
			} else {
				rolePrivilege.Delete = 0
			}

			// Update the record in the database
			if err := db.Save(&rolePrivilege).Error; err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Error updating database record"})
				return
			}
		}
		db.Model(&model.Admin{}).Where("role = ?", roleID).Updates(map[string]interface{}{
			"session":         "",
			"session_expired": 0,
		})
		var emails []string
		db.Model(&model.Admin{}).Where("role = ?", roleID).Pluck("email", &emails)
		// for _, email := range emails {
		// 	ws.CloseWebsocketConnection(email)
		// }
		// Save the newLog to the database
		db.Create(&model.LogActivity{
			AdminID:   adminsWithRoles.ID,
			FullName:  adminsWithRoles.Fullname,
			Action:    "UPDATE",
			Status:    "Success",
			Log:       "Update Roles ID " + roleID,
			IP:        c.ClientIP(),
			UserAgent: c.Request.UserAgent(),
			ReqMethod: c.Request.Method,
			ReqUri:    c.Request.RequestURI,
		})

		c.JSON(http.StatusOK, gin.H{
			"status":  "success",
			"message": "Data successfully updated",
		})
	}
}
func ModalTabRoles(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {

		uuid := c.Query("data")
		// check cookies
		cookies := c.Request.Cookies()

		var credentialsCookie *http.Cookie
		for _, cookie := range cookies {
			if cookie.Name == "credentials" {
				credentialsCookie = cookie
				break
			}
		}
		// if credentialsCookie == nil || len(credentialsCookie.Value) == 0 {
		// 	c.Redirect(302, "/login")
		// 	return
		// } else if credentialsCookie != nil && credentialsCookie.Value == "" {
		// 	credentialsCookie.Expires = time.Now().AddDate(0, 0, -1)
		// 	http.SetCookie(c.Writer, credentialsCookie)
		// 	c.Redirect(302, "/login")
		// 	return
		// }
		// If cookie is nil or has an empty value, redirect to login
		if credentialsCookie == nil || credentialsCookie.Value == "" {
			if credentialsCookie != nil { // Only attempt to expire if the cookie exists
				credentialsCookie.Expires = time.Now().AddDate(0, 0, -1)
				http.SetCookie(c.Writer, credentialsCookie)
			}
			c.Redirect(302, "/login")
			return
		}

		var adminsWithRoles struct {
			model.Admin
			RoleName string `json:"role_name" gorm:"column:role_name"`
		}

		if err := db.
			Table("admins a").
			Unscoped(). // Disable soft deletes for this query
			Select("a.*, b.role_name").
			Joins("LEFT JOIN roles b ON a.role = b.id").
			Where("a.session = ?", credentialsCookie.Value).
			Limit(1).
			Find(&adminsWithRoles).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				credentialsCookie.Expires = time.Now().AddDate(0, 0, -1)
				http.SetCookie(c.Writer, credentialsCookie)
				c.Redirect(302, "/login")
				return
			}

			// Handle other errors
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "Error querying database" + err.Error(),
			})
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
		role_id := uuid
		empty := false
		if uuid == "0" {
			role_id = "1"
			empty = true
		}
		if err := db.
			Table("role_privileges a").
			Unscoped(). // Disable soft deletes for this query
			Select("a.*, b.parent_id , b.title , b.path , b.menu_order , b.status , b.level , b.icon").
			Joins("LEFT JOIN features b ON a.feature_id = b.id").
			Where("a.role_id = ?", role_id).
			Order("b.menu_order").
			Find(&featuresPrivileges).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				// Handle case where no records are found
				c.JSON(http.StatusNotFound, gin.H{
					"error": "No matching record found",
				})
				return
			}

			// Handle other errors
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "Error querying database" + err.Error(),
			})
			return
		}

		fileContent :=
			`<tr>
				<td class="text-nowrap fw-medium">
				Administrator Access
				<i
					class="bx bx-info-circle bx-xs"
					data-bs-toggle="tooltip"
					data-bs-placement="top"
					title="Allows a full access to the system"></i>
				</td>
				<td>
				<div class="form-check form-check-dark">
					<input  type="hidden" name="role_id" value="` + uuid + `"/>
					<input class="form-check-input " type="checkbox" id="selectAllRole" />
					<label class="form-check-label" for="selectAll"> Select All </label>
				</div>
				</td>
			</tr>`

		for _, featurePrivilegeParent := range featuresPrivileges {
			fileContentChild := ""
			// menuToggle := ""

			if len(strings.TrimSpace(featurePrivilegeParent.Path)) == 0 {
				for _, featurePrivilege := range featuresPrivileges {
					if featurePrivilege.ParentID == featurePrivilegeParent.MenuOrder {
						featureID := strconv.FormatUint(uint64(featurePrivilege.FeatureID), 10)
						readChecked := ``
						if featurePrivilege.Read == 1 && !empty {
							readChecked = `checked`
						}
						updateChecked := ``
						if featurePrivilege.Update == 1 && !empty {
							updateChecked = `checked`
						}
						createChecked := ``
						if featurePrivilege.Create == 1 && !empty {
							createChecked = `checked`
						}
						deleteChecked := ``
						if featurePrivilege.Delete == 1 && !empty {
							deleteChecked = `checked`
						}
						allChecked := ``
						if featurePrivilege.Read == 1 &&
							featurePrivilege.Update == 1 &&
							featurePrivilege.Create == 1 &&
							featurePrivilege.Delete == 1 &&
							!empty {
							allChecked = `checked`
						}
						fileContentChild += `
						<tr>
							<td class="text-nowrap fw-medium">` + featurePrivilege.Title + `</td>
							<td>
								<div class="d-flex all-checkbox">
									<div class="form-check form-check-dark me-3 me-lg-5">
										<input class="form-check-input all-check-role check-all-line" type="checkbox" id="roleAll-` + featureID + `" ` + allChecked + `/>
										<label class="form-check-label" for="roleAll-` + featureID + `"> All </label>
									</div>
									<div class="form-check me-3 me-lg-5">
									</div>
									<div class="form-check form-check-primary me-3 me-lg-5">
										<input class="form-check-input all-check-role" type="checkbox" id="roleRead-` + featureID + `" name="roleRead-` + featureID + `" ` + readChecked + `/>
										<label class="form-check-label" for="roleRead-` + featureID + `"> Read </label>
									</div>
									<div class="form-check form-check-info me-3 me-lg-5">
										<input class="form-check-input all-check-role" type="checkbox" id="roleUpdate-` + featureID + `" name="roleUpdate-` + featureID + `" ` + updateChecked + `/>
										<label class="form-check-label" for="roleUpdate-` + featureID + `"> Update </label>
									</div>
									<div class="form-check form-check-success me-3 me-lg-5">
										<input class="form-check-input all-check-role" type="checkbox" id="roleCreate-` + featureID + `" name="roleCreate-` + featureID + `" ` + createChecked + `/>
										<label class="form-check-label" for="roleCreate-` + featureID + `"> Create </label>
									</div>
									<div class="form-check form-check-danger ">
										<input class="form-check-input all-check-role" type="checkbox" id="roleDelete-` + featureID + `" name="roleDelete-` + featureID + `" ` + deleteChecked + `/>
										<label class="form-check-label" for="roleDelete-` + featureID + `"> Delete </label>
									</div>
								</div>
							</td>
						</tr>
						`
					}
				}
			}

			if featurePrivilegeParent.Level == 0 && featurePrivilegeParent.Status == 1 {
				featureID := strconv.FormatUint(uint64(featurePrivilegeParent.FeatureID), 10)
				readChecked := ``
				if featurePrivilegeParent.Read == 1 && !empty {
					readChecked = `checked`
				}
				updateChecked := ``
				if featurePrivilegeParent.Update == 1 && !empty {
					updateChecked = `checked`
				}
				createChecked := ``
				if featurePrivilegeParent.Create == 1 && !empty {
					createChecked = `checked`
				}
				deleteChecked := ``
				if featurePrivilegeParent.Delete == 1 && !empty {
					deleteChecked = `checked`
				}
				allChecked := ``
				if featurePrivilegeParent.Read == 1 &&
					featurePrivilegeParent.Update == 1 &&
					featurePrivilegeParent.Create == 1 &&
					featurePrivilegeParent.Delete == 1 &&
					!empty {
					allChecked = `checked`
				}
				crudInfo := ``
				if len(featurePrivilegeParent.Path) > 0 {
					crudInfo = `
					<td>
						<div class="d-flex all-checkbox">
							<div class="form-check form-check-dark me-3 me-lg-5">
								<input class="form-check-input all-check-role check-all-line" type="checkbox" id="roleAll-` + featureID + `" ` + allChecked + `/>
								<label class="form-check-label" for="roleAll-` + featureID + `"> All </label>
							</div>
							<div class="form-check me-3 me-lg-5">
							</div>
							<div class="form-check form-check-primary me-3 me-lg-5">
								<input class="form-check-input all-check-role" type="checkbox" id="roleRead-` + featureID + `" name="roleRead-` + featureID + `" ` + readChecked + `/>
								<label class="form-check-label" for="roleRead-` + featureID + `"> Read </label>
							</div>
							<div class="form-check form-check-info me-3 me-lg-5">
								<input class="form-check-input all-check-role" type="checkbox" id="roleUpdate-` + featureID + `" name="roleUpdate-` + featureID + `" ` + updateChecked + `/>
								<label class="form-check-label" for="roleUpdate-` + featureID + `"> Update </label>
							</div>
							<div class="form-check form-check-success me-3 me-lg-5">
								<input class="form-check-input all-check-role" type="checkbox" id="roleCreate-` + featureID + `" name="roleCreate-` + featureID + `" ` + createChecked + `/>
								<label class="form-check-label" for="roleCreate-` + featureID + `"> Create </label>
							</div>
							<div class="form-check form-check-danger">
								<input class="form-check-input all-check-role" type="checkbox" id="roleDelete-` + featureID + `" name="roleDelete-` + featureID + `" ` + deleteChecked + `/>
								<label class="form-check-label" for="roleDelete-` + featureID + `"> Delete </label>
							</div>
						</div>
					</td>`
				}
				fileContent += `
				<tr>
					<td class="text-nowrap fw-semibold text-dark fs-5">` + featurePrivilegeParent.Title + `</td>
					` + crudInfo + `
				</tr>
				` + fileContentChild
			}
		}

		fileContent +=
			`<script>
				$(".check-all-line").change(function() {
					var isChecked = $(this).prop("checked");
					$(this).closest(".all-checkbox").find(".all-check-role").prop("checked", isChecked);
				});
				$("#selectAllRole").change(function() {
					var isChecked = $(this).prop("checked");
					$(".all-check-role").prop("checked", isChecked);
				});
				$(".all-check-role").change(function () {
					var isChecked = $(this).prop("checked");
					checkRoleAllCheck()
				});
				checkRoleAllCheck()
				function checkRoleAllCheck(){
					var childCheckboxes = $(".all-check-role");
					
					// Check the overall state of child checkboxes
					var allChecked = childCheckboxes.length === childCheckboxes.filter(":checked").length;
					var noneChecked = childCheckboxes.filter(":checked").length === 0;
				
					const checkbox = document.getElementById('selectAllRole');
					if (allChecked) {
						checkbox.indeterminate = false;
						checkbox.checked = true;
					} else if (noneChecked) {
						checkbox.indeterminate = false;
						checkbox.checked = false;
					} else {
						checkbox.indeterminate = true;
					}
				}
			</script>`
		c.JSON(http.StatusOK, gin.H{
			"data": fileContent,
		})
	}
}
