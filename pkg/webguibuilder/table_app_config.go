package webguibuilder

import (
	"html/template"
	"reflect"
	"service-platform/internal/config"
	"service-platform/internal/core/model"
	"service-platform/internal/database"
	"service-platform/pkg/fun"
	"service-platform/pkg/webgui"

	"github.com/go-redis/redis/v8"
)

// TableAppConfiguration generates an HTML template for a DataTable that displays the application configuration settings. It uses reflection to dynamically create table headers based on the fields of the AppConfig struct and configures the DataTable with server-side processing, pagination, sorting, and export options. The function takes a session identifier and a Redis client to fetch the appropriate data for the table.
func TableAppConfiguration(session string, redisDB *redis.Client) template.HTML {
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
		config.GlobalURL+"api/v1/"+fun.GetRedis("web:"+session, redisDB)+"/tab-app-config/table",
		5,
		[]int{5, 10, 25, 50, 100, 250},
		[]any{[]any{1, "asc"}},
		tableHeaders,
		INSERTABLE,
		EDITABLE,
		DELETABLE,
		not(HideHeader),
		not(PASSWORDABLE),
		not(ScrollUpDown), not(ScrollLeftRight),
		[]string{ExportPrint, ExportCSV, ExportAll},
		database.DBList.Main,
	)
	return templates
}
