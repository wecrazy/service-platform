package webguibuilder

import (
	"html/template"
	"reflect"
	"service-platform/internal/config"
	"service-platform/internal/core/model"
	"service-platform/internal/database"
	"service-platform/internal/pkg/fun"
	"service-platform/internal/pkg/webgui"

	"github.com/go-redis/redis/v8"
)

func TABLE_APP_CONFIGURATION(session string, redisDB *redis.Client) template.HTML {
	// Handling Manufactures
	var tableHeaders []webgui.Column
	dbData := model.AppConfig{}
	// Use reflection to get the type of the struct
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
				Editable:   true,
				Insertable: true,
				Orderable:  true,
			},
		)
	}

	templates := webgui.RenderDataTableServerSideAppConfig(
		"Configuration",
		"dt_app_config",
		config.GLOBAL_URL+"web/"+fun.GetRedis("web:"+session, redisDB)+"/tab-app-config/table",
		5,
		[]int{5, 10, 25, 50, 100, 250},
		[]any{[]any{1, "asc"}},
		tableHeaders,
		INSERTABLE,
		EDITABLE,
		DELETABLE,
		not(HIDE_HEADER),
		not(PASSWORDABLE),
		not(SCROLL_UP_DOWN), not(SCROLL_LEFT_RIGHT),
		[]string{EXPORT_PRINT, EXPORT_CSV, EXPORT_ALL},
		database.DBList.Main,
	)
	return templates
}
