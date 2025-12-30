package controllers

import (
	"fmt"
	"net/http"
	"reflect"
	"service-platform/internal/api/v1/dto"
	"service-platform/internal/config"
	"service-platform/internal/core/model"
	"service-platform/internal/pkg/fun"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// TableAppConfig godoc
// @Summary      Get App Config Table
// @Description  Retrieves the application configuration table data with pagination and sorting
// @Tags         AppConfig
// @Accept       x-www-form-urlencoded
// @Produce      json
// @Param        request formData dto.AppConfigTableRequest true "Table Request"
// @Success      200  {object}   map[string]interface{}
// @Failure      400  {object}   dto.APIErrorResponse
// @Router       /api/v1/{access}/tab-app-config/table [post]
func TableAppConfig(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var request dto.AppConfigTableRequest

		// Bind form data to request struct
		if err := c.Bind(&request); err != nil {
			fun.HandleAPIErrorSimple(c, http.StatusBadRequest, err.Error())
			return
		}

		t := reflect.TypeOf(model.AppConfig{})

		// Initialize the map
		columnMap := make(map[int]string)

		// Loop through the fields of the struct
		colNum := 0
		for i := 0; i < t.NumField(); i++ {
			field := t.Field(i)
			// Get the JSON key
			jsonKey := field.Tag.Get("json")
			if jsonKey == "" || jsonKey == "-" {
				continue
			}
			columnMap[colNum] = jsonKey
			colNum++
		}

		// Get the column name based on SortColumn value
		sortColumnName := columnMap[request.SortColumn]
		orderString := fmt.Sprintf("%s %s", sortColumnName, request.SortDir)

		// Initial query for filtering
		filteredQuery := db.Model(&model.AppConfig{})

		// Apply filters
		if request.Search != "" {
			for i := 0; i < t.NumField(); i++ {
				dataField := ""
				field := t.Field(i)
				dataType := field.Type.String()
				jsonKey := field.Tag.Get("json")
				gormTag := field.Tag.Get("gorm")

				// Initialize a variable to hold the column key
				columnKey := ""

				// Manually parse the gorm tag to find the column value
				tags := strings.Split(gormTag, ";")
				for _, tag := range tags {
					if strings.HasPrefix(tag, "column:") {
						columnKey = strings.TrimPrefix(tag, "column:")
						break
					}
				}
				if jsonKey == "" || jsonKey == "-" {
					if columnKey == "" || columnKey == "-" {
						continue
					} else {
						dataField = columnKey
					}
				} else {
					dataField = jsonKey
				}
				if jsonKey == "" {
					continue
				}
				if dataType != "string" {
					continue
				}

				filteredQuery = filteredQuery.Or("`"+dataField+"` LIKE ?", "%"+request.Search+"%")
			}

		} else {
			for i := 0; i < t.NumField(); i++ {
				field := t.Field(i)
				formKey := field.Tag.Get("json")

				if formKey == "" || formKey == "-" {
					continue
				}

				formValue := c.PostForm(formKey)

				if formValue != "" {
					if field.Type.Kind() == reflect.Bool ||
						(field.Type.Kind() == reflect.Ptr && field.Type.Elem().Kind() == reflect.Bool) {
						// Convert formValue "true"/"false" to bool or int
						boolVal := false
						if formValue == "true" {
							boolVal = true
						}
						filteredQuery = filteredQuery.Where("`"+formKey+"` = ?", boolVal)
					} else {
						// Other fields: use LIKE
						filteredQuery = filteredQuery.Where("`"+formKey+"` LIKE ?", "%"+formValue+"%")
					}
				}
			}
		}

		// Count the total number of records
		var totalRecords int64
		db.Model(&model.AppConfig{}).Count(&totalRecords)

		// Count the number of filtered records
		var filteredRecords int64
		filteredQuery.Count(&filteredRecords)

		// Apply sorting and pagination to the filtered query
		query := filteredQuery.Order(orderString)
		var Dbdata []model.AppConfig
		query = query.Preload("Role").Offset(request.Start).Limit(request.Length).Find(&Dbdata)

		if query.Error != nil {
			fun.HandleAPIErrorSimple(c, http.StatusInternalServerError, query.Error.Error())
			return
		}

		var data []gin.H
		for _, dataInDB := range Dbdata {
			newData := make(map[string]interface{})
			v := reflect.ValueOf(dataInDB)

			for i := 0; i < t.NumField(); i++ {
				field := t.Field(i)
				fieldValue := v.Field(i)

				// Get the JSON key
				theKey := field.Tag.Get("json")
				if theKey == "" {
					theKey = field.Tag.Get("form")
					if theKey == "" {
						continue
					}
				}

				// Handle data rendered in col - Enhanced Switch for AppConfig
				newData[theKey] = renderFieldAppConfig(theKey, fieldValue, dataInDB)
			}

			data = append(data, gin.H(newData))
		}

		// Respond with the formatted data for DataTables
		c.JSON(http.StatusOK, gin.H{
			"draw":            request.Draw,
			"recordsTotal":    totalRecords,
			"recordsFiltered": filteredRecords,
			"data":            data,
		})
	}
}

// renderFieldAppConfig handles the rendering of different field types for the AppConfig table
func renderFieldAppConfig(fieldKey string, fieldValue reflect.Value, dataInDB model.AppConfig) interface{} {
	switch fieldKey {
	case "id":
		return renderIDFieldAppConfig(fieldValue)

	case "created_at", "updated_at", "deleted_at":
		return renderTimeFieldAppConfig(fieldValue)

	case "role_id":
		return renderRoleFieldAppConfig(fieldValue, dataInDB)

	case "app_name":
		return renderAppNameFieldAppConfig(fieldValue)

	case "app_logo":
		return renderAppLogoFieldAppConfig(fieldValue)

	case "app_version", "version_no", "version_name":
		return renderVersionFieldAppConfig(fieldKey, fieldValue)

	case "version_code":
		return renderVersionCodeAppConfig(fieldValue)

	case "is_active":
		return renderActiveFieldAppConfig(fieldValue)

	case "description":
		return renderDescriptionFieldAppConfig(fieldValue)

	default:
		return renderDefaultFieldAppConfig(fieldValue)
	}
}

// renderIDFieldAppConfig renders the ID field with a badge
func renderIDFieldAppConfig(fieldValue reflect.Value) string {
	id := fieldValue.Interface().(uint)
	return fmt.Sprintf(`<span class="badge bg-info text-white fw-bold">#%d</span>`, id)
}

// renderTimeFieldAppConfig renders timestamp fields
func renderTimeFieldAppConfig(fieldValue reflect.Value) interface{} {
	if fieldValue.Type() == reflect.TypeOf(time.Time{}) {
		t := fieldValue.Interface().(time.Time)
		if !t.IsZero() {
			return fmt.Sprintf(`<span class="text-muted small">%s</span>`, t.Format("2006-01-02 15:04:05"))
		} else {
			return `<span class="text-muted">-</span>`
		}
	}
	return fieldValue.Interface()
}

// renderRoleFieldAppConfig renders the role field with enhanced card display
func renderRoleFieldAppConfig(fieldValue reflect.Value, dataInDB model.AppConfig) string {
	if dataInDB.Role.ID > 0 && dataInDB.Role.RoleName != "" {
		icon := dataInDB.Role.Icon
		if icon == "" {
			icon = "fal fa-user"
		}
		className := dataInDB.Role.ClassName
		if className == "" {
			className = "bg-label-primary"
		}

		// Extract the color from className (e.g., "bg-label-primary" -> "primary")
		colorClass := "primary"
		if strings.Contains(className, "bg-label-") {
			colorClass = strings.Replace(className, "bg-label-", "", 1)
		} else if strings.Contains(className, "bg-") {
			colorClass = strings.Replace(className, "bg-", "", 1)
		}

		// Create detailed role card
		return fmt.Sprintf(
			`<div class="role-card card border-0 shadow-sm" style="max-width: 300px;">
				<div class="card-body p-3">
					<div class="d-flex align-items-center mb-2">
						<div class="avatar avatar-sm me-3">
							<span class="avatar-initial rounded bg-%s">
								<i class="%s text-white" style="font-size: 14px;"></i>
							</span>
						</div>
						<div class="flex-grow-1">
							<h6 class="card-title mb-0 fw-bold text-dark">%s</h6>
							<small class="text-muted">Role ID: #%d</small>
						</div>
					</div>
					<div class="role-details">
						<div class="d-flex align-items-center mb-1">
							<i class="bx bx-palette text-info me-2"></i>
							<span class="small text-muted">Class:</span>
							<span class="badge %s ms-2 small">%s</span>
						</div>
						<div class="d-flex align-items-center mb-1">
							<i class="bx bx-time text-warning me-2"></i>
							<span class="small text-muted">Created:</span>
							<span class="small ms-2">%s</span>
						</div>
						<div class="d-flex align-items-center">
							<i class="bx bx-edit text-success me-2"></i>
							<span class="small text-muted">Updated:</span>
							<span class="small ms-2">%s</span>
						</div>
					</div>
				</div>
				<div class="card-footer bg-light border-0 p-2">
					<div class="d-flex justify-content-between align-items-center">
						<span class="badge bg-%s text-white small">
							<i class="%s me-1" style="font-size: 12px;"></i>
							Active Role
						</span>
						<small class="text-muted">ID: %d</small>
					</div>
				</div>
			</div>`,
			htmlEscape(colorClass),                         // Avatar background color
			htmlEscape(icon),                               // Icon (FontAwesome)
			htmlEscape(dataInDB.Role.RoleName),             // Role name
			dataInDB.Role.ID,                               // Role ID in subtitle
			htmlEscape(className),                          // Full class name for badge
			htmlEscape(colorClass),                         // Color name for display
			dataInDB.Role.CreatedAt.Format("Jan 02, 2006"), // Created date
			dataInDB.Role.UpdatedAt.Format("Jan 02, 2006"), // Updated date
			htmlEscape(colorClass),                         // Footer badge color
			htmlEscape(icon),                               // Footer icon
			dataInDB.Role.ID,                               // Footer ID
		)
	} else {
		roleID := fieldValue.Interface().(uint)
		// Fallback card for missing role data
		return fmt.Sprintf(
			`<div class="role-card card border-0 shadow-sm border-danger" style="max-width: 300px;">
				<div class="card-body p-3">
					<div class="d-flex align-items-center mb-2">
						<div class="avatar avatar-sm me-3">
							<span class="avatar-initial rounded bg-secondary">
								<i class="bx bx-question-mark text-white"></i>
							</span>
						</div>
						<div class="flex-grow-1">
							<h6 class="card-title mb-0 fw-bold text-danger">Unknown Role</h6>
							<small class="text-muted">Role ID: #%d</small>
						</div>
					</div>
					<div class="role-details">
						<div class="d-flex align-items-center mb-1">
							<i class="bx bx-error text-danger me-2"></i>
							<span class="small text-danger">Role data not found</span>
						</div>
						<div class="d-flex align-items-center">
							<i class="bx bx-info-circle text-info me-2"></i>
							<span class="small text-muted">Please check role configuration</span>
						</div>
					</div>
				</div>
				<div class="card-footer bg-light border-0 p-2">
					<div class="d-flex justify-content-between align-items-center">
						<span class="badge bg-danger text-white small">
							<i class="bx bx-x me-1"></i>
							Missing Data
						</span>
						<small class="text-muted">ID: %d</small>
					</div>
				</div>
			</div>`,
			roleID, // Role ID in subtitle
			roleID, // Footer ID
		)
	}
}

// renderAppNameFieldAppConfig renders the app name with enhanced styling
func renderAppNameFieldAppConfig(fieldValue reflect.Value) string {
	appName, ok := fieldValue.Interface().(string)
	if ok && appName != "" {
		return fmt.Sprintf(
			`<div class="app-name-container d-flex align-items-center gap-3 p-3 rounded-3" style="background: linear-gradient(135deg, #667eea 0%%, #764ba2 100%%);">
				<div class="app-icon-wrapper">
					<div class="d-flex align-items-center justify-content-center rounded-circle bg-white shadow-lg" style="width: 45px; height: 45px;">
						<i class="bx bx-rocket text-primary fs-3"></i>
					</div>
				</div>
				<div class="app-info">
					<div class="app-label text-white fw-bold text-uppercase opacity-75 small">Application</div>
					<div class="app-name text-white fw-bold fs-3">%s</div>
				</div>
			</div>`,
			htmlEscape(appName),
		)
	}
	return `<div class="app-name-container d-flex align-items-center gap-3 p-3 rounded-3 border bg-light">
		<div class="d-flex align-items-center justify-content-center rounded-circle bg-secondary" style="width: 45px; height: 45px;">
			<i class="bx bx-mobile-off text-white fs-3"></i>
		</div>
		<div class="text-muted">
			<div class="small">Application</div>
			<div class="fw-bold">No Name Set</div>
			<span class="badge bg-danger bg-opacity-10 text-danger">
				<i class="bx bx-x me-1"></i>
				Missing
			</span>
		</div>
	</div>`
}

// renderAppLogoFieldAppConfig renders the app logo with clickable image preview modal
func renderAppLogoFieldAppConfig(fieldValue reflect.Value) string {
	logoURL, ok := fieldValue.Interface().(string)
	if ok && logoURL != "" {
		var filenameLogo string = "app-logo"

		parts := strings.Split(logoURL, "/")
		if len(parts) > 0 {
			filename := parts[len(parts)-1]
			if filename != "" {
				filenameLogo = filename
			}
		}

		return fmt.Sprintf(
			`<div class="d-flex align-items-center gap-3 app-logo-container">
				<div class="logo-preview-wrapper position-relative" style="cursor: pointer;" onclick="showLogoModal('%s', '%s')" onmouseenter="showOverlay(this)" onmouseleave="hideOverlay(this)">
					<img src="%s" alt="App Logo" class="rounded-3 border shadow-sm logo-preview" 
						 style="height:50px; width:50px; object-fit:cover; transition: all 0.3s ease;">
					<div class="logo-overlay position-absolute top-0 start-0 w-100 h-100 d-flex align-items-center justify-content-center rounded-3"
						 style="background: rgba(0,0,0,0.7); opacity: 0; transition: opacity 0.3s ease; pointer-events: none;">
						<i class="bx bx-zoom-in text-white fs-4"></i>
					</div>
				</div>
				<div class="logo-info">
					<div class="logo-filename fw-bold text-dark small">%s</div>
					<div class="logo-url text-muted small text-truncate" style="max-width: 200px;" title="%s">%s</div>
				</div>
			</div>
			<script>
			if (!window.showLogoModal) {
				window.showLogoModal = function(logoUrl, filename) {
					Swal.fire({
						title: 'App Logo Preview',
						html: '<div class="text-center">' +
							  '<img src="' + logoUrl + '" alt="App Logo" class="img-fluid rounded-3 mb-3" style="max-height: 400px; max-width: 100%%; box-shadow: 0 10px 30px rgba(0,0,0,0.3);">' +
							  '<div class="mt-3">' +
							  '<h5 class="fw-bold text-primary">' + filename + '</h5>' +
							  '<p class="text-muted small">' + logoUrl + '</p>' +
							  '<div class="d-flex justify-content-center gap-2 mt-3">' +
							  '<a href="' + logoUrl + '" target="_blank" class="btn btn-primary btn-sm">' +
							  '<i class="bx bx-link-external me-1"></i>Open Original</a>' +
							  '<button class="btn btn-outline-secondary btn-sm" onclick="navigator.clipboard.writeText(\'' + logoUrl + '\'); Swal.showValidationMessage(\'URL copied to clipboard!\');">' +
							  '<i class="bx bx-copy me-1"></i>Copy URL</button>' +
							  '</div></div></div>',
						showConfirmButton: false,
						showCloseButton: true,
						width: '600px',
						padding: '2rem',
						background: '#fff',
						backdrop: 'rgba(0,0,0,0.8)',
						customClass: {
							popup: 'rounded-3 shadow-lg'
						}
					});
				}
			}
			
			if (!window.showOverlay) {
				window.showOverlay = function(element) {
					const img = element.querySelector('.logo-preview');
					const overlay = element.querySelector('.logo-overlay');
					
					if (img) {
						img.style.transform = 'scale(1.1)';
						img.style.boxShadow = '0 8px 25px rgba(0,0,0,0.3)';
					}
					
					if (overlay) {
						overlay.style.opacity = '1';
					}
				}
				
				window.hideOverlay = function(element) {
					const img = element.querySelector('.logo-preview');
					const overlay = element.querySelector('.logo-overlay');
					
					if (img) {
						img.style.transform = 'scale(1)';
						img.style.boxShadow = '0 4px 6px rgba(0,0,0,0.1)';
					}
					
					if (overlay) {
						overlay.style.opacity = '0';
					}
				}
			}
			</script>`,
			htmlEscape(logoURL),
			htmlEscape(filenameLogo),
			htmlEscape(logoURL),
			htmlEscape(filenameLogo),
			htmlEscape(logoURL),
			htmlEscape(logoURL),
		)
	}
	return `<div class="d-flex align-items-center gap-3 p-3 rounded-3 border bg-light">
		<div class="d-flex align-items-center justify-content-center rounded-3 bg-white border shadow-sm" style="height:50px; width:50px;">
			<i class="bx bx-image text-muted fs-3"></i>
		</div>
		<div class="logo-info">
			<div class="text-muted fw-bold small">No Logo</div>
			<div class="text-muted small">Upload an image</div>
			<span class="badge bg-warning bg-opacity-10 text-warning">
				<i class="bx bx-image-add me-1"></i>
				Missing
			</span>
		</div>
	</div>`
}

// renderVersionFieldAppConfig renders version fields with enhanced styling
func renderVersionFieldAppConfig(fieldKey string, fieldValue reflect.Value) string {
	version, ok := fieldValue.Interface().(string)
	if ok && version != "" {
		versionConfig := getVersionConfig(fieldKey)
		return fmt.Sprintf(
			`<div class="version-container d-flex align-items-center gap-2 p-2 rounded border" style="background: linear-gradient(135deg, %s 0%%, %s 100%%);">
				<div class="version-icon d-flex align-items-center justify-content-center rounded-circle bg-white shadow-sm" style="width: 35px; height: 35px;">
					<i class="fal %s text-%s fs-5"></i>
				</div>
				<div class="version-info">
					<!-- <div class="version-label text-white fw-bold small text-uppercase opacity-75">%s</div> -->
					<div class="version-value text-white fw-bold">%s</div>
				</div>
			</div>`,
			versionConfig.gradientFrom,
			versionConfig.gradientTo,
			versionConfig.icon,
			versionConfig.iconColor,
			versionConfig.label,
			htmlEscape(version),
		)
	}
	return `<div class="d-flex align-items-center gap-2 p-2 rounded border bg-light opacity-75">
		<div class="d-flex align-items-center justify-content-center rounded-circle bg-secondary" style="width: 35px; height: 35px;">
			<i class="bx bx-question-mark text-white"></i>
		</div>
		<div class="text-muted">
			<div class="small">Version</div>
			<div class="fw-bold">Not Set</div>
		</div>
	</div>`
}

// renderVersionCodeAppConfig renders the version code with enhanced styling
func renderVersionCodeAppConfig(fieldValue reflect.Value) string {
	versionCode, ok := fieldValue.Interface().(string)
	if ok && versionCode != "" {
		return fmt.Sprintf(
			`<div class="version-code-container d-flex align-items-center gap-2 p-3 rounded-3 border" style="background: linear-gradient(135deg, #ff9a9e 0%%, #fecfef 100%%);">
				<div class="code-icon d-flex align-items-center justify-content-center rounded-circle bg-white shadow-lg" style="width: 40px; height: 40px;">
					<i class="fal fa-laptop-code text-warning fs-4"></i>
				</div>
				<div class="code-info">
					<div class="code-value text-white fw-bold fs-6 font-monospace">%s</div>
				</div>
				<div class="code-badge ms-auto">
					<span class="badge bg-white text-warning fw-bold">
						<i class="fal fa-code-commit me-1"></i>
						%s
					</span>
				</div>
			</div>`,
			htmlEscape(versionCode),
			strings.ToUpper(config.GetConfig().App.Version),
		)
	}
	return `<div class="d-flex align-items-center gap-2 p-3 rounded-3 border bg-light opacity-75">
		<div class="d-flex align-items-center justify-content-center rounded-circle bg-secondary" style="width: 40px; height: 40px;">
			<i class="bx bx-code-off text-white"></i>
		</div>
		<div class="text-muted">
			<div class="fw-bold">Not Available</div>
		</div>
	</div>`
}

// getVersionConfig returns the configuration for version field styling
func getVersionConfig(fieldKey string) struct {
	class, icon, gradientFrom, gradientTo, iconColor, label string
} {
	type VersionConfig struct {
		class, icon, gradientFrom, gradientTo, iconColor, label string
	}

	configs := map[string]VersionConfig{
		"app_version":  {"primary", "fa-tablet-alt", "#0fcbff", "#33065f", "primary", "App Version"},
		"version_no":   {"secondary", "fa-hashtag", "#f093fb", "#f5576c", "danger", "Version No"},
		"version_code": {"warning", "fa-laptop-code", "#4facfe", "#00f2fe", "info", "Version Code"},
		"version_name": {"info", "fa-code-merge", "#43e97b", "#38f9d7", "success", "Version Name"},
	}
	if config, exists := configs[fieldKey]; exists {
		return struct{ class, icon, gradientFrom, gradientTo, iconColor, label string }{
			config.class, config.icon, config.gradientFrom, config.gradientTo, config.iconColor, config.label,
		}
	}
	return struct{ class, icon, gradientFrom, gradientTo, iconColor, label string }{
		"primary", "fa-mobile", "#667eea", "#764ba2", "primary", "Version",
	}
}

// renderActiveFieldAppConfig renders the is_active field with enhanced status display
func renderActiveFieldAppConfig(fieldValue reflect.Value) string {
	isActive := fieldValue.Interface().(bool)
	if isActive {
		return `<div class="status-container d-flex align-items-center gap-3 p-3 rounded-3" style="background: linear-gradient(135deg, #667eea 0%, #764ba2 100%); min-width: 160px;">
			<div class="status-icon-wrapper position-relative">
				<div class="d-flex align-items-center justify-content-center rounded-circle bg-white shadow-lg" style="width: 45px; height: 45px;">
					<i class="fal fa-skating text-success fs-3"></i>
				</div>
				<div class="position-absolute top-0 start-100 translate-middle">
					<span class="badge bg-success rounded-pill">
						<i class="bx bx-check fs-6"></i>
					</span>
				</div>
			</div>
			<div class="status-content">
				<div class="status-label text-white fw-bold text-uppercase opacity-75 small">Status</div>
				<div class="status-text text-white fw-bold fs-6">ACTIVE</div>
				<!--
				<div class="status-pulse">
					<span class="badge bg-success bg-opacity-75 pulse-animation">
						<i class="bx bx-wifi me-1"></i>
						Online
					</span>
				</div>
				-->
			</div>
		</div>
		<style>
		.pulse-animation {
			animation: pulse 2s infinite;
		}
		@keyframes pulse {
			0% { opacity: 1; }
			50% { opacity: 0.5; }
			100% { opacity: 1; }
		}
		</style>`
	}
	return `<div class="status-container d-flex align-items-center gap-3 p-3 rounded-3" style="background: linear-gradient(135deg, #ff6b6b 0%, #ffd93d 100%); min-width: 160px;">
		<div class="status-icon-wrapper position-relative">
			<div class="d-flex align-items-center justify-content-center rounded-circle bg-white shadow-lg" style="width: 45px; height: 45px;">
				<i class="bx bx-x-circle text-danger fs-3"></i>
			</div>
			<div class="position-absolute top-0 start-100 translate-middle">
				<span class="badge bg-danger rounded-pill">
					<i class="bx bx-x fs-6"></i>
				</span>
			</div>
		</div>
		<div class="status-content">
			<div class="status-label text-white fw-bold text-uppercase opacity-75 small">Status</div>
			<div class="status-text text-white fw-bold fs-6">INACTIVE</div>
			<div class="status-pulse">
				<span class="badge bg-danger bg-opacity-75">
					<i class="bx bx-wifi-off me-1"></i>
					Offline
				</span>
			</div>
		</div>
	</div>`
}

// renderDescriptionFieldAppConfig renders description with truncation
func renderDescriptionFieldAppConfig(fieldValue reflect.Value) string {
	desc, ok := fieldValue.Interface().(string)
	if ok && desc != "" {
		const maxLen = 50
		short := desc
		if len(desc) > maxLen {
			short = desc[:maxLen] + "..."
		}
		return fmt.Sprintf(
			`<div class="description-cell" title="%s">
				<i class="bx bx-info-circle text-primary me-1"></i>
				<span>%s</span>
			</div>`,
			htmlEscape(desc),
			htmlEscape(short),
		)
	}
	return `<span class="text-muted text-danger">
		<i class="bx bx-message-x me-1"></i>
		No description
	</span>`
}

// renderDefaultFieldAppConfig handles fallback rendering for unspecified fields
func renderDefaultFieldAppConfig(fieldValue reflect.Value) interface{} {
	value := fieldValue.Interface()
	if str, ok := value.(string); ok && str == "" {
		return `<span class="text-muted">-</span>`
	}
	return value
}
