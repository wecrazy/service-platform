package webguibuilder

import (
	"html/template"
	"reflect"
	"service-platform/cmd/technical_assistance/fun"
	"service-platform/cmd/technical_assistance/model"
	"service-platform/cmd/technical_assistance/model/cc_model"
	"service-platform/cmd/technical_assistance/model/op_model"
	"service-platform/cmd/technical_assistance/webgui"
	"strings"

	"github.com/go-redis/redis/v8"
)

const (
	EXPORT_COPY       = "COPY"
	EXPORT_PRINT      = "PRINT"
	EXPORT_CSV        = "CSV"
	EXPORT_PDF        = "PDF"
	EXPORT_ALL        = "ALL"
	INSERTABLE        = true
	EDITABLE          = true
	DELETABLE         = true
	HIDE_HEADER       = true
	PASSWORDABLE      = true
	SCROLL_UP_DOWN    = true
	SCROLL_LEFT_RIGHT = true
)

func not(b bool) bool {
	return !b
}

func TABLE_KONFIRMASI_DATA_PENGERJAAN_PENDING(session string, redisDB *redis.Client) template.HTML {
	// Handling Manufactures
	var tableHeaders []webgui.Column
	uploaded_file := op_model.Pending{}
	// Use reflection to get the type of the struct
	t := reflect.TypeOf(uploaded_file)
	// Loop through the fields of the struct
	// tableHeaders = append(tableHeaders,
	// 	webgui.Column{
	// 		Data:       "id",
	// 		Header:     "",
	// 		Type:       "string",
	// 		Visible:    true,
	// 		Editable:   false,
	// 		Insertable: false,
	// 		Orderable:  false,
	// 		Filterable: false,
	// 	},
	// )
	for i := 0; i < t.NumField(); i++ {
		if i == 0 {
			tableHeaders = append(tableHeaders, webgui.Column{Data: "id", Header: "", Type: "int"})
			continue
		}

		field := t.Field(i)
		// Get the variable name
		varName := field.Name
		varName = fun.AddSpaceBeforeUppercase(varName)
		// Get the data type
		dataType := field.Type.String()
		// Get the JSON key
		jsonKey := field.Tag.Get("json")
		if jsonKey == "" || jsonKey == "-" {
			continue
		}

		tableHeaders = append(tableHeaders,
			webgui.Column{
				Data:       jsonKey,
				Header:     template.HTML(varName),
				Type:       dataType,
				Visible:    true,
				Editable:   false,
				Insertable: false,
				Orderable:  true,
			},
		)
	}

	templates := webgui.RenderDataTableServerSide(
		"Konfirmasi Pengerjaan Teknisi Pending",
		"dt_teknisi_pengerjaan_pending",
		fun.GLOBAL_URL+"web/"+fun.GetRedis("web:"+session, redisDB)+"/tab-konfirmasi-data-pending/table",
		5,
		[]int{5, 10, 25, 50, 100},
		[]any{[]any{1, "asc"}},
		tableHeaders,
		not(INSERTABLE), not(EDITABLE), not(DELETABLE), not(HIDE_HEADER), not(PASSWORDABLE),
		(SCROLL_UP_DOWN), not(SCROLL_LEFT_RIGHT),
		[]string{EXPORT_PRINT, EXPORT_CSV, EXPORT_ALL},
	)
	return templates
}

func TABLE_VIEW_DATA_PENDING(session string, redisDB *redis.Client) template.HTML {
	// Handling Manufactures
	var tableHeaders []webgui.Column
	uploaded_file := op_model.Pending{}
	// Use reflection to get the type of the struct
	t := reflect.TypeOf(uploaded_file)
	// Loop through the fields of the struct
	// tableHeaders = append(tableHeaders,
	// 	webgui.Column{
	// 		Data:       "id",
	// 		Header:     "",
	// 		Type:       "string",
	// 		Visible:    true,
	// 		Editable:   false,
	// 		Insertable: false,
	// 		Orderable:  false,
	// 		Filterable: false,
	// 	},
	// )
	for i := 0; i < t.NumField(); i++ {
		if i == 0 {
			tableHeaders = append(tableHeaders, webgui.Column{Data: "id", Header: "", Type: "int"})
			continue
		}

		field := t.Field(i)
		// Get the variable name
		varName := field.Name
		varName = fun.AddSpaceBeforeUppercase(varName)
		// Get the data type
		dataType := field.Type.String()
		// Get the JSON key
		jsonKey := field.Tag.Get("json")
		if jsonKey == "" || jsonKey == "-" {
			continue
		}

		tableHeaders = append(tableHeaders,
			webgui.Column{
				Data:       jsonKey,
				Header:     template.HTML(varName),
				Type:       dataType,
				Visible:    true,
				Editable:   false,
				Insertable: false,
				Orderable:  true,
			},
		)
	}

	templates := webgui.RenderDataTableServerSide(
		"Data Pending",
		"dt_data_pending",
		fun.GLOBAL_URL+"web/"+fun.GetRedis("web:"+session, redisDB)+"/tab-log-act/table3",
		5,
		[]int{5, 10, 25, 50, 100},
		[]any{[]any{1, "asc"}},
		tableHeaders,
		not(INSERTABLE), not(EDITABLE), not(DELETABLE), not(HIDE_HEADER), not(PASSWORDABLE),
		not(SCROLL_UP_DOWN), not(SCROLL_LEFT_RIGHT),
		[]string{EXPORT_PRINT, EXPORT_CSV, EXPORT_ALL},
	)
	return templates
}

func TABLE_VIEW_DATA_ERROR(session string, redisDB *redis.Client) template.HTML {
	// Handling Manufactures
	var tableHeaders []webgui.Column
	uploaded_file := op_model.Error{}
	// Use reflection to get the type of the struct
	t := reflect.TypeOf(uploaded_file)
	// Loop through the fields of the struct
	// tableHeaders = append(tableHeaders,
	// 	webgui.Column{
	// 		Data:       "id",
	// 		Header:     "",
	// 		Type:       "string",
	// 		Visible:    true,
	// 		Editable:   false,
	// 		Insertable: false,
	// 		Orderable:  false,
	// 		Filterable: false,
	// 	},
	// )
	for i := 0; i < t.NumField(); i++ {
		if i == 0 {
			tableHeaders = append(tableHeaders, webgui.Column{Data: "id", Header: "", Type: "int"})
			continue
		}

		field := t.Field(i)
		// Get the variable name
		varName := field.Name
		varName = fun.AddSpaceBeforeUppercase(varName)
		// Get the data type
		dataType := field.Type.String()
		// Get the JSON key
		jsonKey := field.Tag.Get("json")
		if jsonKey == "" || jsonKey == "-" {
			continue
		}

		tableHeaders = append(tableHeaders,
			webgui.Column{
				Data:       jsonKey,
				Header:     template.HTML(varName),
				Type:       dataType,
				Visible:    true,
				Editable:   false,
				Insertable: false,
				Orderable:  true,
			},
		)
	}

	templates := webgui.RenderDataTableServerSide(
		"Data Foto Error",
		"dt_data_foto_error",
		fun.GLOBAL_URL+"web/"+fun.GetRedis("web:"+session, redisDB)+"/tab-log-act/table2",
		5,
		[]int{5, 10, 25, 50, 100},
		[]any{[]any{1, "asc"}},
		tableHeaders,
		not(INSERTABLE), not(EDITABLE), not(DELETABLE), not(HIDE_HEADER), not(PASSWORDABLE),
		not(SCROLL_UP_DOWN), not(SCROLL_LEFT_RIGHT),
		[]string{EXPORT_PRINT, EXPORT_CSV, EXPORT_ALL},
	)
	return templates
}
func TABLE_KONFIRMASI_DATA_PENGERJAAN_ERROR(session string, redisDB *redis.Client) template.HTML {
	// Handling Manufactures
	var tableHeaders []webgui.Column
	uploaded_file := op_model.Error{}
	// Use reflection to get the type of the struct
	t := reflect.TypeOf(uploaded_file)
	// Loop through the fields of the struct
	// tableHeaders = append(tableHeaders,
	// 	webgui.Column{
	// 		Data:       "id",
	// 		Header:     "",
	// 		Type:       "string",
	// 		Visible:    true,
	// 		Editable:   false,
	// 		Insertable: false,
	// 		Orderable:  false,
	// 		Filterable: false,
	// 	},
	// )
	for i := 0; i < t.NumField(); i++ {
		if i == 0 {
			tableHeaders = append(tableHeaders, webgui.Column{Data: "id", Header: "", Type: "int"})
			continue
		}

		field := t.Field(i)
		// Get the variable name
		varName := field.Name
		varName = fun.AddSpaceBeforeUppercase(varName)
		// Get the data type
		dataType := field.Type.String()
		// Get the JSON key
		jsonKey := field.Tag.Get("json")
		if jsonKey == "" || jsonKey == "-" {
			continue
		}

		tableHeaders = append(tableHeaders,
			webgui.Column{
				Data:       jsonKey,
				Header:     template.HTML(varName),
				Type:       dataType,
				Visible:    true,
				Editable:   false,
				Insertable: false,
				Orderable:  true,
			},
		)
	}

	templates := webgui.RenderDataTableServerSide(
		"Konfirmasi Pengerjaan Teknisi Error",
		"dt_teknisi_pengerjaan_error",
		fun.GLOBAL_URL+"web/"+fun.GetRedis("web:"+session, redisDB)+"/tab-konfirmasi-data-error/table",
		5,
		[]int{5, 10, 25, 50, 100},
		[]any{[]any{1, "asc"}},
		tableHeaders,
		not(INSERTABLE), not(EDITABLE), not(DELETABLE), not(HIDE_HEADER), not(PASSWORDABLE),
		not(SCROLL_UP_DOWN), not(SCROLL_LEFT_RIGHT),
		[]string{EXPORT_PRINT, EXPORT_CSV, EXPORT_ALL},
	)
	return templates
}
func TABLE_KONFIRMASI_DATA_PENGERJAAN_SUBMISSION(session string, redisDB *redis.Client) template.HTML {
	// Handling Manufactures
	var tableHeaders []webgui.Column
	uploaded_file := op_model.TempSubmission{}
	// Use reflection to get the type of the struct
	t := reflect.TypeOf(uploaded_file)
	// Loop through the fields of the struct
	// tableHeaders = append(tableHeaders,
	// 	webgui.Column{
	// 		Data:       "id",
	// 		Header:     "",
	// 		Type:       "string",
	// 		Visible:    true,
	// 		Editable:   false,
	// 		Insertable: false,
	// 		Orderable:  false,
	// 		Filterable: false,
	// 	},
	// )
	for i := 0; i < t.NumField(); i++ {
		if i == 0 {
			tableHeaders = append(tableHeaders, webgui.Column{Data: "id", Header: "", Type: "int"})
			continue
		}

		field := t.Field(i)
		// Get the variable name
		varName := field.Name
		varName = fun.AddSpaceBeforeUppercase(varName)
		// Get the data type
		dataType := field.Type.String()
		// Get the JSON key
		jsonKey := field.Tag.Get("json")
		if jsonKey == "" || jsonKey == "-" {
			continue
		}

		tableHeaders = append(tableHeaders,
			webgui.Column{
				Data:       jsonKey,
				Header:     template.HTML(varName),
				Type:       dataType,
				Visible:    true,
				Editable:   false,
				Insertable: false,
				Orderable:  true,
			},
		)
	}

	templates := webgui.RenderDataTableServerSide(
		"Data Yang Perlu Follow-Up Lagi Kedepannya",
		"dt_teknisi_pengerjaan_submission",
		fun.GLOBAL_URL+"web/"+fun.GetRedis("web:"+session, redisDB)+"/tab-konfirmasi-data-submission/table",
		5,
		[]int{5, 10, 25, 50, 100},
		[]any{[]any{1, "asc"}},
		tableHeaders,
		not(INSERTABLE), not(EDITABLE), not(DELETABLE), not(HIDE_HEADER), not(PASSWORDABLE),
		not(SCROLL_UP_DOWN), not(SCROLL_LEFT_RIGHT),
		[]string{EXPORT_PRINT, EXPORT_CSV, EXPORT_ALL},
	)
	return templates
}
func TABLE_LOG_ACT(session string, redisDB *redis.Client) template.HTML {
	// Handling Manufactures
	var tableHeaders []webgui.Column
	uploaded_file := op_model.LogAct{}
	// Use reflection to get the type of the struct
	t := reflect.TypeOf(uploaded_file)
	// Loop through the fields of the struct
	// tableHeaders = append(tableHeaders,
	// 	webgui.Column{
	// 		Data:       "id",
	// 		Header:     "",
	// 		Type:       "string",
	// 		Visible:    true,
	// 		Editable:   false,
	// 		Insertable: false,
	// 		Orderable:  false,
	// 		Filterable: false,
	// 	},
	// )
	for i := 0; i < t.NumField(); i++ {
		if i == 0 {
			tableHeaders = append(tableHeaders, webgui.Column{Data: "id", Header: "", Type: "int"})
			continue
		}

		field := t.Field(i)
		// Get the variable name
		varName := field.Name
		varName = fun.AddSpaceBeforeUppercase(varName)
		// Get the data type
		dataType := field.Type.String()
		// Get the JSON key
		jsonKey := field.Tag.Get("json")
		if jsonKey == "" || jsonKey == "-" {
			continue
		}

		tableHeaders = append(tableHeaders,
			webgui.Column{
				Data:       jsonKey,
				Header:     template.HTML(varName),
				Type:       dataType,
				Visible:    true,
				Editable:   false,
				Insertable: false,
				Orderable:  true,
			},
		)
	}

	templates := webgui.RenderDataTableServerSide(
		"Log Activty Team Technical Assistance",
		"dt_ta_log_activity",
		fun.GLOBAL_URL+"web/"+fun.GetRedis("web:"+session, redisDB)+"/tab-log-act/table",
		5,
		[]int{5, 10, 25, 50, 100},
		[]any{[]any{1, "asc"}},
		tableHeaders,
		not(INSERTABLE), not(EDITABLE), not(DELETABLE), not(HIDE_HEADER), not(PASSWORDABLE),
		not(SCROLL_UP_DOWN), not(SCROLL_LEFT_RIGHT),
		[]string{EXPORT_PRINT, EXPORT_CSV, EXPORT_ALL},
	)
	return templates
}
func TABLE_MERCHANT_JO_HMIN1_CALL_LOG(session string, redisDB *redis.Client) template.HTML {
	// Handling Manufactures
	var tableHeaders []webgui.Column
	uploaded_file := cc_model.JOMerchantHmin1CallLog{}
	// Use reflection to get the type of the struct
	t := reflect.TypeOf(uploaded_file)
	// Loop through the fields of the struct
	// tableHeaders = append(tableHeaders,
	// 	webgui.Column{
	// 		Data:       "id",
	// 		Header:     "",
	// 		Type:       "string",
	// 		Visible:    true,
	// 		Editable:   false,
	// 		Insertable: false,
	// 		Orderable:  false,
	// 		Filterable: false,
	// 	},
	// )
	for i := 0; i < t.NumField(); i++ {
		if i == 0 {
			tableHeaders = append(tableHeaders, webgui.Column{Data: "id", Header: "ID", Type: "int"})
			continue
		}

		field := t.Field(i)
		// Get the variable name
		varName := field.Name
		varName = fun.AddSpaceBeforeUppercase(varName)
		// Get the data type
		dataType := field.Type.String()
		// Get the JSON key
		jsonKey := field.Tag.Get("json")
		if jsonKey == "" || jsonKey == "-" {
			continue
		}

		// var visibleCol bool
		switch strings.ToLower(jsonKey) {
		case "id_cs":
			varName = "CS NAME"
		}

		tableHeaders = append(tableHeaders,
			webgui.Column{
				Data:       jsonKey,
				Header:     template.HTML(varName),
				Type:       dataType,
				Visible:    true,
				Editable:   false,
				Insertable: false,
				Orderable:  true,
				Filterable: true,
			},
		)
	}

	templates := webgui.RenderDataTableServerSide(
		"MERCHANT H-1 CALL LOG",
		"dt_merchant_call_log",
		fun.GLOBAL_URL+"web/"+fun.GetRedis("web:"+session, redisDB)+"/tab-cc-merchant-call-log/table",
		10,
		[]int{10, 25, 50, 100},
		[]any{[]any{15, "asc"}},
		tableHeaders,
		not(INSERTABLE), not(EDITABLE), not(DELETABLE), not(HIDE_HEADER), not(PASSWORDABLE),
		not(SCROLL_UP_DOWN), not(SCROLL_LEFT_RIGHT),
		[]string{EXPORT_PRINT, EXPORT_CSV, EXPORT_ALL},
	)
	return templates
}
func TABLE_UPLOADED_FILE(session string, redisDB *redis.Client) template.HTML {
	// Handling Manufactures
	var tableHeaders []webgui.Column
	uploaded_file := model.UploadedFiles{}
	// Use reflection to get the type of the struct
	t := reflect.TypeOf(uploaded_file)
	// Loop through the fields of the struct
	tableHeaders = append(tableHeaders,
		webgui.Column{
			Data:       "id",
			Header:     "",
			Type:       "string",
			Visible:    true,
			Editable:   false,
			Insertable: false,
			Orderable:  false,
			Filterable: false,
		},
	)
	for i := 0; i < t.NumField(); i++ {
		// if i == 0 {
		// 	tableHeaders = append(tableHeaders, webgui.Column{Data: "id", Header: "ID", Type: "int"})
		// 	continue
		// }

		field := t.Field(i)
		// Get the variable name
		varName := field.Name
		varName = fun.AddSpaceBeforeUppercase(varName)
		// Get the data type
		dataType := field.Type.String()
		// Get the JSON key
		jsonKey := field.Tag.Get("json")
		if jsonKey == "" || jsonKey == "-" {
			continue
		}

		tableHeaders = append(tableHeaders,
			webgui.Column{
				Data:       jsonKey,
				Header:     template.HTML(varName),
				Type:       dataType,
				Visible:    true,
				Editable:   false,
				Insertable: false,
				Orderable:  true,
			},
		)
	}

	templates := webgui.RenderDataTableServerSide(
		"File di unggah",
		"dt_uploaded_files",
		fun.GLOBAL_URL+"web/"+fun.GetRedis("web:"+session, redisDB)+"/tab-uploaded-file/table",
		10,
		[]int{10, 25, 50, 100, 200, 500, 1000},
		[]any{[]any{1, "asc"}},
		tableHeaders,
		INSERTABLE, EDITABLE, DELETABLE, not(HIDE_HEADER), not(PASSWORDABLE),
		not(SCROLL_UP_DOWN), not(SCROLL_LEFT_RIGHT),
		[]string{EXPORT_PRINT, EXPORT_CSV, EXPORT_ALL},
	)
	return templates
}
func TABLE_KUNJUNGAN_TEKNISI(session string, redisDB *redis.Client) template.HTML {
	// Handling Manufactures
	var tableHeaders []webgui.Column
	teknisi := model.TeknisiKunjungan{}
	// Use reflection to get the type of the struct
	t := reflect.TypeOf(teknisi)
	// Loop through the fields of the struct
	for i := 0; i < t.NumField(); i++ {
		if i == 0 {
			tableHeaders = append(tableHeaders, webgui.Column{Data: "", Header: "", Type: "", Visible: true})
			continue
		}

		field := t.Field(i)
		// Get the variable name
		varName := field.Name
		varName = fun.AddSpaceBeforeUppercase(varName)
		// Get the data type
		dataType := field.Type.String()
		// Get the JSON key
		jsonKey := field.Tag.Get("json")
		if jsonKey == "" || jsonKey == "-" {
			continue
		}
		switch varName {
		case "Serial Number":
			varName = "SN Terminal"
		case "T I D":
			varName = "EDC TID"
		case "M I D":
			varName = "EDC MID"
		}

		tableHeaders = append(tableHeaders,
			webgui.Column{
				Data:       jsonKey,
				Header:     template.HTML(varName),
				Type:       dataType,
				Visible:    true,
				Editable:   true,
				Insertable: true,
				Orderable:  false,
			},
		)
	}

	templates := webgui.RenderDataTableServerSide(
		`Data <b class="text-primary">Kunjugan</b> Teknisi`,
		"dt_kunjugan_teknisi",
		fun.GLOBAL_URL+"web/"+fun.GetRedis("web:"+session, redisDB)+"/tab-teknisi/kunjungan/table",
		5,
		[]int{5, 10, 25, 50, 100, 200, 500, 1000},
		[]any{[]any{1, "desc"}},
		tableHeaders,
		not(INSERTABLE), not(EDITABLE), not(DELETABLE), not(HIDE_HEADER), not(PASSWORDABLE),
		not(SCROLL_UP_DOWN), not(SCROLL_LEFT_RIGHT),
		[]string{EXPORT_PRINT, EXPORT_CSV, EXPORT_ALL},
	)
	return templates
}
func TABLE_TRACK_TEKNISI(session string, redisDB *redis.Client) template.HTML {
	// Handling Manufactures
	var tableHeaders []webgui.Column
	teknisi := model.Teknisi{}
	// Use reflection to get the type of the struct
	t := reflect.TypeOf(teknisi)
	// Loop through the fields of the struct
	for i := 0; i < t.NumField(); i++ {
		if i == 0 {
			tableHeaders = append(tableHeaders, webgui.Column{Data: "", Header: "", Type: ""})
			continue
		}

		field := t.Field(i)
		// Get the variable name
		varName := field.Name
		varName = fun.AddSpaceBeforeUppercase(varName)
		// Get the data type
		dataType := field.Type.String()
		// Get the JSON key
		jsonKey := field.Tag.Get("json")
		if jsonKey == "" || jsonKey == "-" {
			continue
		}
		if jsonKey == "odoo_id" {
			dataType = "string"
		}

		switch varName {
		case "Serial Number":
			varName = "SN Terminal"
		case "T I D":
			varName = "EDC TID"
		case "M I D":
			varName = "EDC MID"
		}
		tableHeaders = append(tableHeaders,
			webgui.Column{
				Data:       jsonKey,
				Header:     template.HTML(varName),
				Type:       dataType,
				Visible:    true,
				Editable:   true,
				Insertable: true,
				Orderable:  false,
			},
		)
	}

	templates := webgui.RenderDataTableServerSide(
		"Teknisi",
		"dt_track_teknisi",
		fun.GLOBAL_URL+"web/"+fun.GetRedis("web:"+session, redisDB)+"/tab-track-teknisi/teknisi/table",
		10,
		[]int{10, 25, 50, 100, 200, 500, 1000},
		[]any{[]any{1, "asc"}},
		tableHeaders,
		INSERTABLE, EDITABLE, DELETABLE, HIDE_HEADER, not(PASSWORDABLE), //insertable, editable, deletable, hideHeader, passwordable bool, scrollUpDown, scrollLeftRight bool,
		not(SCROLL_UP_DOWN), not(SCROLL_LEFT_RIGHT),
		[]string{EXPORT_PRINT, EXPORT_CSV, EXPORT_ALL},
	)
	return templates
}
