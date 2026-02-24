package webgui

import (
	"fmt"
	"html/template"

	"gorm.io/gorm"
)

// RenderDataTableServerSideAppConfig renders a server-side DataTable for AppConfig management with dynamic column configurations based on the provided parameters. It generates HTML for the DataTable, including filterable, editable, and insertable fields, as well as export options. The function takes into account specific fields relevant to AppConfig and applies appropriate input types and configurations for each column. The rendered HTML is returned as a template.HTML type for safe rendering in the web GUI.
func RenderDataTableServerSideAppConfig(title, tableName, endpoint string, pageLength int, lengthMenu []int, order []any, tableColumns []Column, insertable, editable, deletable, hideHeader, passwordable bool, scrollUpDown, scrollLeftRight bool, exportType []string, _ *gorm.DB) template.HTML {
	var columnArray []int
	for i, col := range tableColumns {
		if col.Visible {
			columnArray = append(columnArray, i)
		}

		tableColumns[i].Filterable = false
		tableColumns[i].Insertable = false

		switch col.Type {
		case "string":
			applyAppConfigStringColumn(tableColumns, i, tableName, endpoint, editable)
		case "image":
			applyAppConfigImageColumn(tableColumns, i, tableName, endpoint, editable)
		case "time.Time", "*time.Time":
			applyAppConfigTimeColumn(tableColumns, i, tableName)
		case "int", "int8", "int16", "int32", "uint", "int64":
			applyAppConfigIntColumn(tableColumns, i, tableName)
		case "bool":
			applyAppConfigBoolColumn(tableColumns, i, tableName)
		}
	}

	actionable := ""
	if editable || deletable {
		tableColumns = append(tableColumns, Column{Data: "", Header: template.HTML("<i class='bx bx-run'></i>"), Type: "", Editable: false})
		actionable = "orderable"
	}

	exportCopy, exportPrint, exportPdf, exportCsv, exportAllCsv := buildAppConfigExportFlags(exportType)

	passtrue := ""
	if passwordable {
		passtrue = `pass="true"`
	}

	renderedHTML, err := RenderTemplateToString("gui_server_table_app_config.html", map[string]any{
		"title":           template.HTML(title),
		"table_name":      tableName,
		"endpoint":        template.URL(endpoint),
		"table_columns":   tableColumns,
		"actionable":      actionable,
		"insertable":      insertable,
		"page_length":     pageLength,
		"length_menu":     lengthMenu,
		"order":           order,
		"hide_header":     hideHeader,
		"passwordable":    passwordable,
		"passtrue":        passtrue,
		"exportCopy":      exportCopy,
		"exportPrint":     exportPrint,
		"exportPdf":       exportPdf,
		"exportCsv":       exportCsv,
		"exportAllCsv":    exportAllCsv,
		"scrollUpDown":    scrollUpDown,
		"scrollLeftRight": scrollLeftRight,
		"columnArray":     columnArray,
	})
	if err != nil {
		return template.HTML("Error rendering template: " + err.Error())
	}

	return template.HTML(renderedHTML)
}

// buildAppConfigExportFlags returns boolean flags for each export type based on the provided export type slice.
func buildAppConfigExportFlags(exportType []string) (copyFlag, printFlag, pdfFlag, csvFlag, allCSVFlag bool) {
	for _, et := range exportType {
		switch et {
		case ExportCopy:
			copyFlag = true
		case ExportPrint:
			printFlag = true
		case ExportCSV:
			csvFlag = true
		case ExportPdf:
			pdfFlag = true
		case ExportAll:
			allCSVFlag = true
		}
	}
	return
}

// applyAppConfigStringColumn configures a string-type column for AppConfig DataTable rendering.
func applyAppConfigStringColumn(tableColumns []Column, i int, tableName, endpoint string, editable bool) {
	col := tableColumns[i]
	allowedCols := map[string]bool{
		"app_name": true, "app_logo": true, "app_version": true,
		"version_no": true, "version_code": true, "version_name": true,
	}
	allowedInsertCols := map[string]bool{
		"app_name": true, "app_logo": true, "app_version": true,
		"version_no": true, "version_code": true, "version_name": true,
	}

	if allowedCols[col.Data] {
		tableColumns[i].Filterable = true
	}
	if allowedInsertCols[col.Data] {
		tableColumns[i].Insertable = true
	}

	filterID := "ft_" + tableName + "_" + col.Data
	editID := "ed_" + tableName + "_" + col.Data
	insertID := "in_" + tableName + "_" + col.Data
	tableColumns[i].EditID = editID

	if tableColumns[i].SelectableSrc != "" {
		tableColumns[i].EditForm = stringSelectableSrcEditForm(col.Header, tableColumns[i].SelectableSrc, editID, col.Data, i)
		tableColumns[i].Filter = stringSelectableSrcFilter(col.Header, tableColumns[i].SelectableSrc, filterID, i)
		tableColumns[i].InsertField = stringSelectableSrcInsertField(col.Header, tableColumns[i].SelectableSrc, insertID, col.Data, i)
	} else {
		applyAppConfigStringFieldByData(tableColumns, i, col, insertID, editID, filterID)
	}

	className := "control"
	returnValue := ""
	if i > 0 {
		className = ""
		returnValue = buildStringReturnValue(tableColumns[i], col.Data, endpoint, editable)
	}
	tableColumns[i].ColumnConfig = stringColumnConfig(className, returnValue, tableColumns[i].Visible, tableColumns[i].Orderable, i)
}

// applyAppConfigStringFieldByData sets insert/edit/filter fields for a string column based on the column data name.
func applyAppConfigStringFieldByData(tableColumns []Column, i int, col Column, insertID, editID, filterID string) {
	switch col.Data {
	case "app_logo":
		tableColumns[i].InsertField = template.HTML(buildLogoInsertHTML(insertID, col.Data, i))
		tableColumns[i].EditForm = template.HTML(buildLogoEditHTML(editID, col.Data, i))
		tableColumns[i].Filter = stringFilter("Logo URL", filterID, i)
	case "app_name":
		tableColumns[i].InsertField = stringInsertField("Application Name", insertID, col.Data, i)
		tableColumns[i].EditForm = stringEditForm("Application Name", editID, col.Data, i)
		tableColumns[i].Filter = stringFilter("Application Name", filterID, i)
	case "app_version":
		tableColumns[i].InsertField = stringInsertField("App Version", insertID, col.Data, i)
		tableColumns[i].EditForm = stringEditForm("App Version", editID, col.Data, i)
		tableColumns[i].Filter = stringFilter("App Version", filterID, i)
	case "version_no":
		tableColumns[i].InsertField = stringInsertField("Version Number", insertID, col.Data, i)
		tableColumns[i].EditForm = stringEditForm("Version Number", editID, col.Data, i)
		tableColumns[i].Filter = stringFilter("Version Number", filterID, i)
	case "version_code":
		tableColumns[i].InsertField = stringInsertField("Version Code", insertID, col.Data, i)
		tableColumns[i].EditForm = stringEditForm("Version Code", editID, col.Data, i)
		tableColumns[i].Filter = stringFilter("Version Code", filterID, i)
	case "version_name":
		tableColumns[i].InsertField = stringInsertField("Version Name", insertID, col.Data, i)
		tableColumns[i].EditForm = stringEditForm("Version Name", editID, col.Data, i)
		tableColumns[i].Filter = stringFilter("Version Name", filterID, i)
	default:
		tableColumns[i].InsertField = stringInsertField(col.Header, insertID, col.Data, i)
		tableColumns[i].EditForm = stringEditForm(col.Header, editID, col.Data, i)
		tableColumns[i].Filter = stringFilter(col.Header, filterID, i)
	}
}

// buildStringReturnValue builds the JS/HTML return value for a string column cell renderer.
func buildStringReturnValue(col Column, data, endpoint string, editable bool) string {
	if !editable {
		return `<p>'+data+'</p>`
	}
	if col.Editable {
		pass := ""
		if col.Passwordable {
			pass = `pass="true"`
		}
		if col.SelectableSrc != "" {
			return `<p class="selectable-suggestion" data-origin="'+extract_data+'" patch="` + endpoint + `" field="` + data + `" select-option="` + string(col.SelectableSrc) + `" point="'+full['id']+'" ` + pass + `>'+data+'</p>`
		}
		return `<p class="editable" data-origin="'+extract_data+'" patch="` + endpoint + `" field="` + data + `" point="'+full['id']+'" ` + pass + `>'+data+'</p>`
	}
	return `<p>'+data+'</p>`
}

// buildLogoInsertHTML builds the HTML for the app logo insert field with URL/file toggle.
func buildLogoInsertHTML(insertID, dataField string, i int) string {
	idx := fmt.Sprintf("%d", i)
	idxPrev := fmt.Sprintf("%d", i-1)
	return `<label class="form-label d-flex justify-content-between align-items-center">
App Logo
<div class="form-check form-switch ms-2">
<input class="form-check-input" type="checkbox" id="logoUploadToggle_` + insertID + `">
<label class="form-check-label ms-2" for="logoUploadToggle_` + insertID + `">Upload file instead of URL</label>
</div>
</label>
<input id="` + insertID + `" name="` + dataField + `" type="text" class="form-control logo-url-input" data-column="` + idx + `" placeholder="Enter logo URL" data-column-index="` + idxPrev + `" />
<input id="` + insertID + `_file" name="` + dataField + `_file" type="file" class="form-control logo-file-input" data-column="` + idx + `" placeholder="Upload logo image" accept=".jpg,.jpeg,.png,.gif,.webp" data-column-index="` + idxPrev + `" style="display:none;" />
<script>
document.getElementById('logoUploadToggle_` + insertID + `').addEventListener('change', function() {
const urlInput = document.querySelector('#` + insertID + `');
const fileInput = document.querySelector('#` + insertID + `_file');
if (this.checked) {
urlInput.style.display = 'none';
fileInput.style.display = 'block';
urlInput.required = false;
fileInput.required = true;
} else {
urlInput.style.display = 'block';
fileInput.style.display = 'none';
fileInput.required = false;
urlInput.required = true;
}
});
</script>`
}

// buildLogoEditHTML builds the HTML for the app logo edit form with URL/file toggle.
func buildLogoEditHTML(editID, dataField string, i int) string {
	idx := fmt.Sprintf("%d", i)
	idxPrev := fmt.Sprintf("%d", i-1)
	return `<label class="form-label">App Logo</label>
<div class="mb-2">
<div class="form-check form-switch">
<input class="form-check-input" type="checkbox" id="logoUploadToggleEdit_` + editID + `">
<label class="form-check-label" for="logoUploadToggleEdit_` + editID + `">Upload new file instead of URL</label>
</div>
</div>
<input id="` + editID + `" name="` + dataField + `" type="text" class="form-control logo-url-input" data-column="` + idx + `" placeholder="Enter logo URL" data-column-index="` + idxPrev + `" />
<input id="` + editID + `_file" name="` + dataField + `_file" type="file" class="form-control logo-file-input" data-column="` + idx + `" placeholder="Upload logo image" accept=".jpg,.jpeg,.png,.gif,.webp" data-column-index="` + idxPrev + `" style="display:none;" />
<script>
document.getElementById('logoUploadToggleEdit_` + editID + `').addEventListener('change', function() {
const urlInput = document.querySelector('#` + editID + `');
const fileInput = document.querySelector('#` + editID + `_file');
if (this.checked) {
urlInput.style.display = 'none';
fileInput.style.display = 'block';
} else {
urlInput.style.display = 'block';
fileInput.style.display = 'none';
}
});
</script>`
}

// applyAppConfigImageColumn configures an image-type column for AppConfig DataTable rendering.
func applyAppConfigImageColumn(tableColumns []Column, i int, tableName, endpoint string, editable bool) {
	col := tableColumns[i]
	editID := "ed_" + tableName + "_" + col.Data
	insertID := "in_" + tableName + "_" + col.Data
	tableColumns[i].EditID = editID
	tableColumns[i].Orderable = false
	tableColumns[i].Filterable = false
	tableColumns[i].Filter = ""
	tableColumns[i].InsertField = imageInsertField(col.Header, insertID, col.Data, i)
	tableColumns[i].EditForm = imageEditForm(col.Header, editID, col.Data, i)

	className := "control"
	returnValue := ""
	if i > 0 {
		className = ""
		if editable && tableColumns[i].Editable {
			returnValue = `<img src="'+data+'" alt="Image" style="width: 100%%;height:auto;" class="editable-image" data-origin="'+data+'" patch="` + endpoint + `" field="` + col.Data + `" point="'+full['id']+'" /> `
		} else {
			returnValue = `<img src="'+data+'" alt="Image" style="width: 100%% ; height: auto;"/>`
		}
	}
	tableColumns[i].ColumnConfig = imageTableColumnConfig(className, returnValue, tableColumns[i].Visible, tableColumns[i].Orderable, i)
}

// applyAppConfigTimeColumn configures a time.Time-type column for AppConfig DataTable rendering.
func applyAppConfigTimeColumn(tableColumns []Column, i int, tableName string) {
	col := tableColumns[i]
	filterID := "ft_" + tableName + "_" + col.Data
	editID := "ed_" + tableName + "_" + col.Data
	insertID := "in_" + tableName + "_" + col.Data
	tableColumns[i].EditID = editID
	tableColumns[i].InsertField = timeTimeInsertField(col.Header, insertID, col.Data, i)
	tableColumns[i].EditForm = timeTimeEditForm(col.Header, editID, col.Data, i)
	tableColumns[i].Filter = timeTimeFilter(col.Header, filterID, tableName, i)
}

// applyAppConfigIntColumn configures an int-type column for AppConfig DataTable rendering.
func applyAppConfigIntColumn(tableColumns []Column, i int, tableName string) {
	col := tableColumns[i]
	allowedCols := map[string]bool{"role_id": true}
	allowedInsertCols := map[string]bool{"role_id": true}
	allowedEditedCols := map[string]bool{"role_id": true}

	if allowedCols[col.Data] {
		tableColumns[i].Filterable = true
	}
	if allowedInsertCols[col.Data] {
		tableColumns[i].Insertable = true
	}
	if !allowedEditedCols[col.Data] {
		tableColumns[i].Editable = false
	}

	filterID := "ft_" + tableName + "_" + col.Data
	editID := "ed_" + tableName + "_" + col.Data
	insertID := "in_" + tableName + "_" + col.Data
	tableColumns[i].EditID = editID

	if tableColumns[i].SelectableSrc != "" {
		tableColumns[i].EditForm = intSelectableSrcEditForm(col.Header, tableColumns[i].SelectableSrc, editID, col.Data, i)
		tableColumns[i].Filter = intSelectableSrcFilter(col.Header, tableColumns[i].SelectableSrc, filterID, i)
		tableColumns[i].InsertField = intSelectableSrcInsertField(col.Header, tableColumns[i].SelectableSrc, insertID, col.Data, i)
	} else {
		switch col.Data {
		case "role_id":
			tableColumns[i].InsertField = intInsertField("Role ID", insertID, col.Data, i)
			tableColumns[i].Filter = intFilter("Role ID", filterID, i)
			tableColumns[i].EditForm = intEditForm("Role ID", editID, col.Data, i)
		default:
			tableColumns[i].InsertField = intInsertField(col.Header, insertID, col.Data, i)
			tableColumns[i].Filter = intFilter(col.Header, filterID, i)
			tableColumns[i].EditForm = intEditForm(col.Header, editID, col.Data, i)
		}
	}
}

// applyAppConfigBoolColumn configures a bool-type column for AppConfig DataTable rendering.
func applyAppConfigBoolColumn(tableColumns []Column, i int, tableName string) {
	col := tableColumns[i]
	allowedCols := map[string]bool{"is_active": true}
	allowedInsertCols := map[string]bool{"is_active": true}
	allowedEditedCols := map[string]bool{"is_active": true}

	if allowedCols[col.Data] {
		tableColumns[i].Filterable = true
	}
	if allowedInsertCols[col.Data] {
		tableColumns[i].Insertable = true
	}
	if !allowedEditedCols[col.Data] {
		tableColumns[i].Editable = false
	}

	filterID := "ft_" + tableName + "_" + col.Data
	editID := "ed_" + tableName + "_" + col.Data
	insertID := "in_" + tableName + "_" + col.Data
	tableColumns[i].EditID = editID

	yesNoOptionsHTML := `
<option value="true" class="fw-bold text-success">Yes</option>
<option value="false" class="fw-bold text-danger">No</option>
`
	filterOptionsHTML := `
<option value="" class="fw-bold">All</option>
<option value="true" class="fw-bold text-success">Yes</option>
<option value="false" class="fw-bold text-danger">No</option>
`
	tableColumns[i].EditForm = booleanEditFormWhatsappUserManagement(col.Header, editID, col.Data, yesNoOptionsHTML, i)
	tableColumns[i].Filter = booleanFilterWhatsappUserManagement(col.Header, filterID, filterOptionsHTML, i)
	tableColumns[i].InsertField = booleanInsertFieldWhatsappUserManagement(col.Header, insertID, col.Data, yesNoOptionsHTML, i)
}
