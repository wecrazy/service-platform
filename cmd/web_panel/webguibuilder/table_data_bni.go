package webguibuilder

import (
	"html/template"
	"reflect"
	"service-platform/cmd/web_panel/fun"
	"service-platform/cmd/web_panel/internal/gormdb"
	bnimodel "service-platform/cmd/web_panel/model/bni_model"
	"service-platform/cmd/web_panel/webgui"

	"github.com/go-redis/redis/v8"
)

func TABLE_DATA_PM_BNI(session string, redisDB *redis.Client) template.HTML {
	var tableHeaders []webgui.Column
	db := gormdb.Databases.Web
	dbData := bnimodel.BNIOdooMSData{}
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

		excludedKeys := map[string]bool{
			"":          true,
			"-":         true,
			"task_type": true,
		}

		if excludedKeys[jsonKey] {
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

	templates := webgui.RenderDataTableServerSidePMBNI(
		"Detail Data",
		"dt_data_pm_bni",
		fun.GLOBAL_URL+"web/"+fun.GetRedis("web:"+session, redisDB)+"/tab-bni-monitoring-pm/table_pm",
		5,
		[]int{5, 10, 25, 50, 100, 250},
		[]any{[]any{1, "asc"}},
		tableHeaders,
		not(INSERTABLE),
		not(EDITABLE),
		not(DELETABLE),
		not(HIDE_HEADER),
		not(PASSWORDABLE),
		not(SCROLL_UP_DOWN), not(SCROLL_LEFT_RIGHT),
		[]string{EXPORT_PRINT, EXPORT_CSV},
		db,
	)
	return templates

}

func TABLE_DATA_NON_PM_BNI(session string, redisDB *redis.Client) template.HTML {
	var tableHeaders []webgui.Column
	db := gormdb.Databases.Web
	dbData := bnimodel.BNIOdooMSData{}
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

		excludedKeys := map[string]bool{
			"":  true,
			"-": true,
			// "task_type": true,
		}

		if excludedKeys[jsonKey] {
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

	templates := webgui.RenderDataTableServerSideNonPMBNI(
		"Detail Data",
		"dt_data_non_pm_bni",
		fun.GLOBAL_URL+"web/"+fun.GetRedis("web:"+session, redisDB)+"/tab-bni-monitoring-non-pm/table_non_pm",
		5,
		[]int{5, 10, 25, 50, 100, 250},
		[]any{[]any{1, "asc"}},
		tableHeaders,
		not(INSERTABLE),
		not(EDITABLE),
		not(DELETABLE),
		not(HIDE_HEADER),
		not(PASSWORDABLE),
		not(SCROLL_UP_DOWN), not(SCROLL_LEFT_RIGHT),
		[]string{EXPORT_PRINT, EXPORT_CSV},
		db,
	)
	return templates

}
