package webguibuilder

import (
	"fmt"
	"html/template"
	"reflect"
	"service-platform/cmd/web_panel/fun"
	"service-platform/cmd/web_panel/internal/gormdb"
	sptechnicianmodel "service-platform/cmd/web_panel/model/sp_technician_model"
	"service-platform/cmd/web_panel/webgui"

	"github.com/go-redis/redis/v8"
)

func TABLE_DATA_SP_TECHNICIAN(session string, redisDB *redis.Client) template.HTML {
	var tableHeaders []webgui.Column
	db := gormdb.Databases.Web
	dbData := sptechnicianmodel.TechnicianGotSP{}
	t := reflect.TypeOf(dbData)

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

	templates := webgui.RenderDataTableServerSideSPTechnician(
		"Surat Peringatan (SP) untuk Teknisi",
		"dt_sp_technician",
		fun.GLOBAL_URL+"web/"+fun.GetRedis("web:"+session, redisDB)+"/tab-hr-sp/table_technician",
		5,
		[]int{5, 10, 25, 50, 100, 250},
		[]any{[]any{1, "asc"}},
		tableHeaders,
		not(INSERTABLE),
		not(EDITABLE),
		DELETABLE,
		not(HIDE_HEADER),
		not(PASSWORDABLE),
		not(SCROLL_UP_DOWN), not(SCROLL_LEFT_RIGHT),
		[]string{EXPORT_PRINT, EXPORT_CSV},
		db,
	)
	return templates

}

func TABLE_DATA_SP_SPL(session string, redisDB *redis.Client) template.HTML {
	var tableHeaders []webgui.Column
	db := gormdb.Databases.Web
	dbData := sptechnicianmodel.SPLGotSP{}
	t := reflect.TypeOf(dbData)

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

	templates := webgui.RenderDataTableServerSideSPSPL(
		"Surat Peringatan (SP) untuk SPL",
		"dt_sp_spl",
		fun.GLOBAL_URL+"web/"+fun.GetRedis("web:"+session, redisDB)+"/tab-hr-sp/table_spl",
		5,
		[]int{5, 10, 25, 50, 100, 250},
		[]any{[]any{1, "asc"}},
		tableHeaders,
		not(INSERTABLE),
		not(EDITABLE),
		DELETABLE,
		not(HIDE_HEADER),
		not(PASSWORDABLE),
		not(SCROLL_UP_DOWN), not(SCROLL_LEFT_RIGHT),
		[]string{EXPORT_PRINT, EXPORT_CSV},
		db,
	)
	return templates

}

func TABLE_DATA_SP_SAC(session string, redisDB *redis.Client) template.HTML {
	var tableHeaders []webgui.Column
	db := gormdb.Databases.Web
	dbData := sptechnicianmodel.SACGotSP{}
	t := reflect.TypeOf(dbData)

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

	templates := webgui.RenderDataTableServerSideSPSAC(
		"Surat Peringatan (SP) untuk SAC",
		"dt_sp_sac",
		fun.GLOBAL_URL+"web/"+fun.GetRedis("web:"+session, redisDB)+"/tab-hr-sp/table_sac",
		5,
		[]int{5, 10, 25, 50, 100, 250},
		[]any{[]any{1, "asc"}},
		tableHeaders,
		not(INSERTABLE),
		not(EDITABLE),
		DELETABLE,
		not(HIDE_HEADER),
		not(PASSWORDABLE),
		not(SCROLL_UP_DOWN), not(SCROLL_LEFT_RIGHT),
		[]string{EXPORT_PRINT, EXPORT_CSV},
		db,
	)
	return templates

}

// GET_SP_TECHNICIAN_COUNT returns the count of SP Technician records
func GET_SP_TECHNICIAN_COUNT() string {
	db := gormdb.Databases.Web
	var count int64
	
	if err := db.Model(&sptechnicianmodel.TechnicianGotSP{}).Count(&count).Error; err != nil {
		return "0"
	}
	
	return fmt.Sprintf("%d", count)
}

// GET_SP_SPL_COUNT returns the count of SP SPL records
func GET_SP_SPL_COUNT() string {
	db := gormdb.Databases.Web
	var count int64
	
	if err := db.Model(&sptechnicianmodel.SPLGotSP{}).Count(&count).Error; err != nil {
		return "0"
	}
	
	return fmt.Sprintf("%d", count)
}

// GET_SP_SAC_COUNT returns the count of SP SAC records
func GET_SP_SAC_COUNT() string {
	db := gormdb.Databases.Web
	var count int64
	
	if err := db.Model(&sptechnicianmodel.SACGotSP{}).Count(&count).Error; err != nil {
		return "0"
	}
	
	return fmt.Sprintf("%d", count)
}
