package webgui

import (
	"fmt"
	"html/template"
	"service-platform/cmd/web_panel/fun"
	"service-platform/cmd/web_panel/model"
	"service-platform/internal/config"
	"strings"

	"gorm.io/gorm"
)

func RenderDataTableServerSideWhatsappUserManagement(title, table_name, endpoint string, page_length int, length_menu []int, order []any, table_columns []Column, insertable, editable, deletable, hideHeader, passwordable bool, scrollUpDown, scrollLeftRight bool, exportType []string, db *gorm.DB) template.HTML {
	var column_array []int
	for i, col := range table_columns {
		if col.Visible {
			column_array = append(column_array, i)
		}

		table_columns[i].Filterable = false
		table_columns[i].Insertable = false

		switch col.Type {
		case "string":
			allowedCols := map[string]bool{
				"full_name":    true,
				"email":        true,
				"phone_number": true,
				"description":  true,
			}

			allowedInsertCols := map[string]bool{
				"full_name":    true,
				"email":        true,
				"phone_number": true,
				"description":  true,
			}

			if allowedCols[col.Data] {
				table_columns[i].Filterable = true
			}

			if allowedInsertCols[col.Data] {
				table_columns[i].Insertable = true
			}

			filter_id := "ft_" + table_name + "_" + col.Data
			edit_id := "ed_" + table_name + "_" + col.Data
			insert_id := "in_" + table_name + "_" + col.Data
			table_columns[i].EditId = edit_id
			if table_columns[i].SelectableSrc != "" {
				table_columns[i].EditForm = stringSelectableSrcEditForm(col.Header, table_columns[i].SelectableSrc, edit_id, col.Data, i)
				table_columns[i].Filter = stringSelectableSrcFilter(col.Header, table_columns[i].SelectableSrc, filter_id, i)
				table_columns[i].InsertField = stringSelectableSrcInsertField(col.Header, table_columns[i].SelectableSrc, insert_id, col.Data, i)
			} else {
				switch table_columns[i].Data {
				default:
					table_columns[i].InsertField = stringInsertField(col.Header, insert_id, col.Data, i)
					table_columns[i].EditForm = stringEditForm(col.Header, edit_id, col.Data, i)
					table_columns[i].Filter = stringFilter(col.Header, filter_id, i)
				}
			}

			className := "control"
			returnValue := ""
			if i > 0 {
				className = ""
				if editable {
					if table_columns[i].Editable {
						pass := ""
						if table_columns[i].Passwordable {
							pass = `pass="true"`
						}
						if table_columns[i].SelectableSrc != "" {
							returnValue = `<p class="selectable-suggestion" data-origin="'+extract_data+'" patch="` + endpoint + `" field="` + col.Data + `" select-option="` + string(table_columns[i].SelectableSrc) + `" point="'+full['id']+'" ` + pass + `>'+data+'</p>`
						} else {
							returnValue = `<p class="editable" data-origin="'+extract_data+'" patch="` + endpoint + `" field="` + col.Data + `" point="'+full['id']+'" ` + pass + `>'+data+'</p>`
						}
					} else {
						returnValue = `<p>'+data+'</p>`

					}
				} else {
					returnValue = `<p>'+data+'</p>`
				}
			}

			table_columns[i].ColumnConfig = stringColumnConfig(className, returnValue, table_columns[i].Visible, table_columns[i].Orderable, i)
		case "image":
			// filter_id := "ft_" + table_name + "_" + col.Data
			edit_id := "ed_" + table_name + "_" + col.Data
			insert_id := "in_" + table_name + "_" + col.Data
			table_columns[i].EditId = edit_id
			table_columns[i].Orderable = false
			table_columns[i].Filterable = false
			table_columns[i].Filter = ""

			table_columns[i].InsertField = imageInsertField(col.Header, insert_id, col.Data, i)
			table_columns[i].EditForm = imageEditForm(col.Header, edit_id, col.Data, i)

			className := "control"
			returnValue := ""
			if i > 0 {
				className = ""
				if editable {
					if table_columns[i].Editable {
						returnValue = `<img src="'+data+'" alt="Image" style="width: 100%%;height:auto;" class="editable-image" data-origin="'+data+'" patch="` + endpoint + `" field="` + col.Data + `" point="'+full['id']+'" /> `
					} else {
						returnValue = `<img src="'+data+'" alt="Image" style="width: 100%% ; height: auto;"/>`

					}
				} else {
					returnValue = `<img src="'+data+'" alt="Image" style="width: 100%% ; height: auto;"/>`
				}
			}

			table_columns[i].ColumnConfig = imageTableColumnConfig(className, returnValue, table_columns[i].Visible, table_columns[i].Orderable, i)

		case "time.Time", "*time.Time":
			filter_id := "ft_" + table_name + "_" + col.Data
			edit_id := "ed_" + table_name + "_" + col.Data
			insert_id := "in_" + table_name + "_" + col.Data
			table_columns[i].EditId = edit_id

			table_columns[i].InsertField = timeTimeInsertField(col.Header, insert_id, col.Data, i)
			table_columns[i].EditForm = timeTimeEditForm(col.Header, edit_id, col.Data, i)
			table_columns[i].Filter = timeTimeFilter(col.Header, filter_id, table_name, i)

		case "int", "int8", "int16", "int32", "uint", "int64":
			allowedCols := map[string]bool{
				"max_daily_quota": true,
			}

			allowedInsertCols := map[string]bool{
				"max_daily_quota": true,
			}

			if allowedCols[col.Data] {
				table_columns[i].Filterable = true
			}

			if allowedInsertCols[col.Data] {
				table_columns[i].Insertable = true
			}

			filter_id := "ft_" + table_name + "_" + col.Data
			edit_id := "ed_" + table_name + "_" + col.Data
			insert_id := "in_" + table_name + "_" + col.Data
			table_columns[i].EditId = edit_id
			if table_columns[i].SelectableSrc != "" {
				table_columns[i].EditForm = intSelectableSrcEditForm(col.Header, table_columns[i].SelectableSrc, edit_id, col.Data, i)
				table_columns[i].Filter = intSelectableSrcFilter(col.Header, table_columns[i].SelectableSrc, filter_id, i)
				table_columns[i].InsertField = intSelectableSrcInsertField(col.Header, table_columns[i].SelectableSrc, insert_id, col.Data, i)
			} else {
				table_columns[i].InsertField = intInsertField(col.Header, insert_id, col.Data, i)
				table_columns[i].Filter = intFilter(col.Header, filter_id, i)
				table_columns[i].EditForm = intEditForm(col.Header, edit_id, col.Data, i)
			}

			className := "control"
			returnValue := ""
			if i > 0 {
				className = ""
				if editable {
					if table_columns[i].Editable {
						if table_columns[i].SelectableSrc != "" {
							returnValue = `<p class="selectable-suggestion" data-origin="'+data+'" patch="` + endpoint + `" field="` + col.Data + `" select-option="` + string(table_columns[i].SelectableSrc) + `" point="'+full['id']+'" >'+data+'</p>`
						} else {
							returnValue = `<p class="editable" data-origin="'+data+'" patch="` + endpoint + `" field="` + col.Data + `" point="'+full['id']+'" >'+data+'</p>`
						}
					} else {
						returnValue = `<p>'+data+'</p>`

					}
				} else {
					returnValue = `<p>'+data+'</p>`
				}
			}

			table_columns[i].ColumnConfig = intColumnConfig(className, returnValue, table_columns[i].Visible, table_columns[i].Orderable, i)

		case "model.WAAllowedChatMode":
			allowedCols := map[string]bool{
				"allowed_chats": true,
			}
			if allowedCols[col.Data] {
				table_columns[i].Filterable = true
				table_columns[i].Insertable = true
			}
			filter_id := "ft_" + table_name + "_" + col.Data
			edit_id := "ed_" + table_name + "_" + col.Data
			insert_id := "in_" + table_name + "_" + col.Data

			// Build options
			optionsHTML := ""
			for _, mode := range model.AllWAAllowedChatModes {
				optionsHTML += fmt.Sprintf(`<option value="%s">%s</option>`, mode, fun.CapitalizeWord(string(mode)))
			}

			table_columns[i].Filter = allowedChatFilterWhatsappUserManagement(col.Header, filter_id, optionsHTML, i)
			table_columns[i].EditForm = allowedChatEditFormWhatsappUserManagement(col.Header, edit_id, col.Data, optionsHTML, i)
			table_columns[i].InsertField = allowedChatInsertFieldWhatsappUserManagement(col.Header, insert_id, col.Data, optionsHTML, i)

		case "model.WAUserType":
			allowedCols := map[string]bool{
				"user_type": true,
			}
			if allowedCols[col.Data] {
				table_columns[i].Filterable = true
				table_columns[i].Insertable = true
				table_columns[i].Editable = false
			}
			filter_id := "ft_" + table_name + "_" + col.Data
			edit_id := "ed_" + table_name + "_" + col.Data
			insert_id := "in_" + table_name + "_" + col.Data

			// Build options
			optionsHTML := ""
			for _, t := range model.AllWAUserTypes {
				optionsHTML += fmt.Sprintf(`<option value="%s">%s</option>`, t, fun.SnakeToCapitalized(string(t)))
			}

			// Form build
			table_columns[i].Filter = waUserTypeFilterWhatsappUserManagement(col.Header, filter_id, optionsHTML, i)
			table_columns[i].EditForm = waUserTypeEditFormWhatsappUserManagement(col.Header, edit_id, col.Data, optionsHTML, i)
			table_columns[i].InsertField = waUserTypeInsertFieldWhatsappUserManagement(col.Header, insert_id, col.Data, optionsHTML, i)

		case "datatypes.JSON":
			allowedCols := map[string]bool{
				"allowed_types": true,
			}
			if allowedCols[col.Data] {
				table_columns[i].Filterable = true
				table_columns[i].Insertable = true
				table_columns[i].Editable = false // FIX: this SOON !!
			}

			filter_id := "ft_" + table_name + "_" + col.Data
			edit_id := "ed_" + table_name + "_" + col.Data
			insert_id := "in_" + table_name + "_" + col.Data

			// Build HTML options dynamically from your constant slice
			optionsHTML := ""
			for _, t := range model.AllWAMessageTypes {
				optionsHTML += fmt.Sprintf(`<option value="%s">%s</option>`, t, strings.Title(string(t)))
			}

			table_columns[i].Filter = allowedTypesFilterWhatsappUserManagement(col.Header, filter_id, optionsHTML, i)
			table_columns[i].EditForm = allowedTypesEditFormWhatsappUserManagement(col.Header, edit_id, col.Data, optionsHTML, i)
			table_columns[i].InsertField = allowedTypesInsertFieldWhatsappUserManagement(col.Header, insert_id, col.Data, optionsHTML, i)

		case "bool":
			allowedCols := map[string]bool{
				"is_registered":   true,
				"allowed_to_call": true,
			}

			allowedInsertCols := map[string]bool{
				"is_registered":   true,
				"allowed_to_call": true,
			}

			allowedEditedCols := map[string]bool{
				"allowed_to_call": true,
			}

			if allowedCols[col.Data] {
				table_columns[i].Filterable = true
			}
			if allowedInsertCols[col.Data] {
				table_columns[i].Insertable = true
			}
			if !allowedEditedCols[col.Data] {
				table_columns[i].Editable = false
			}

			filter_id := "ft_" + table_name + "_" + col.Data
			edit_id := "ed_" + table_name + "_" + col.Data
			insert_id := "in_" + table_name + "_" + col.Data
			table_columns[i].EditId = edit_id

			// Common yes/no options, no "All"
			yesNoOptionsHTML := `
	<option value="true" class="fw-bold text-success">Yes</option>
	<option value="false" class="fw-bold text-danger">No</option>
`

			// Filter options: prepend "All" option
			filterOptionsHTML := `
	<option value="" class="fw-bold">All</option>
	<option value="true" class="fw-bold text-success">Yes</option>
	<option value="false" class="fw-bold text-danger">No</option>
`
			table_columns[i].EditForm = booleanEditFormWhatsappUserManagement(col.Header, edit_id, col.Data, yesNoOptionsHTML, i)
			table_columns[i].Filter = booleanFilterWhatsappUserManagement(col.Header, filter_id, filterOptionsHTML, i)
			table_columns[i].InsertField = booleanInsertFieldWhatsappUserManagement(col.Header, insert_id, col.Data, yesNoOptionsHTML, i)

		// // Render
		// className := "control"
		// returnValue := ""
		// if i > 0 {
		// 	className = ""
		// 	if editable {
		// 		if table_columns[i].Editable {
		// 			returnValue = `<p class="editable" data-origin="'+data+'" patch="` + endpoint + `" field="` + col.Data + `" point="'+full['id']+'" >'+(data ? "✔️" : "✖️")+'</p>`
		// 		} else {
		// 			returnValue = `<p>'+(data ? "✔️" : "✖️")+'</p>`
		// 		}
		// 	} else {
		// 		returnValue = `<p>'+(data ? "✔️" : "✖️")+'</p>`
		// 	}
		// }
		// table_columns[i].ColumnConfig = boolColumnConfig(className, returnValue, table_columns[i].Visible, table_columns[i].Orderable, i)

		case "model.WAUserOf":
			allowedCols := map[string]bool{
				"user_of": true,
			}
			if allowedCols[col.Data] {
				table_columns[i].Filterable = true
				table_columns[i].Insertable = true
			}
			filter_id := "ft_" + table_name + "_" + col.Data
			edit_id := "ed_" + table_name + "_" + col.Data
			insert_id := "in_" + table_name + "_" + col.Data

			// Build options
			optionsHTML := ""
			for _, mode := range model.AllUserOf {
				var optLabel string
				if mode == model.UserOfCSNA {
					optLabel = config.WebPanel.Get().Default.PT
				} else {
					optLabel = fun.SnakeToCapitalized(string(mode))
				}
				optionsHTML += fmt.Sprintf(`<option value="%s">%s</option>`, mode, optLabel)
			}

			table_columns[i].Filter = userOfFilterWhatsappUserManagement(col.Header, filter_id, optionsHTML, i)
			table_columns[i].EditForm = userOfEditFormWhatsappUserManagement(col.Header, edit_id, col.Data, optionsHTML, i)
			table_columns[i].InsertField = userOfInsertFieldWhatsappUserManagement(col.Header, insert_id, col.Data, optionsHTML, i)

			// default:
			// 	table_columns[i].Filterable = false
		}

	}
	actionable := ""
	if editable || deletable {
		table_columns = append(table_columns, Column{Data: "", Header: template.HTML("<i class='bx bx-run'></i>"), Type: "", Editable: false})
		actionable = "orderable"
	}

	export_copy := false
	export_print := false
	export_pdf := false
	export_csv := false
	export_all_csv := false
	for _, export_type := range exportType {
		switch export_type {
		case EXPORT_COPY:
			export_copy = true
		case EXPORT_PRINT:
			export_print = true
		case EXPORT_CSV:
			export_csv = true
		case EXPORT_PDF:
			export_csv = true
		case EXPORT_ALL:
			export_all_csv = true
		}
	}
	passtrue := ""
	if passwordable {
		passtrue = `pass="true"`

	}
	renderedHTML, err := RenderTemplateToString("gui_server_table_whatsapp_user_management.html", map[string]any{
		"title":           template.HTML(title),
		"table_name":      table_name,
		"endpoint":        template.URL(endpoint),
		"table_columns":   table_columns,
		"actionable":      actionable,
		"insertable":      insertable,
		"page_length":     page_length,
		"length_menu":     length_menu,
		"order":           order,
		"hide_header":     hideHeader,
		"passwordable":    passwordable,
		"passtrue":        passtrue,
		"export_copy":     export_copy,
		"export_print":    export_print,
		"export_pdf":      export_pdf,
		"export_csv":      export_csv,
		"export_all_csv":  export_all_csv,
		"scrollUpDown":    scrollUpDown,
		"scrollLeftRight": scrollLeftRight,
		"column_array":    column_array,
	})
	if err != nil {
		return template.HTML("Error rendering template")
	}

	return template.HTML(renderedHTML)
}
