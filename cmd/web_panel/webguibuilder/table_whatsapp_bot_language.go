package webguibuilder

import (
	"html/template"
	"reflect"
	"service-platform/cmd/web_panel/fun"
	"service-platform/cmd/web_panel/model"
	"service-platform/cmd/web_panel/webgui"

	"github.com/go-redis/redis/v8"
	"gorm.io/gorm"
)

func TABLE_WHATSAPP_BOT_LANGUAGE(session string, redisDB *redis.Client, db *gorm.DB) template.HTML {
	var tableHeaders []webgui.Column
	dbData := model.Language{}
	// Use reflection to get the type of the struct
	t := reflect.TypeOf(dbData)
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

	templates := webgui.RenderDataTableServerSideWhatsappBotLanguage(
		"Whatsapp Bot Language",
		"dt_whatsapp_language",
		fun.GLOBAL_URL+"web/"+fun.GetRedis("web:"+session, redisDB)+"/tab-whatsapp/table_language",
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
		[]string{EXPORT_PRINT, EXPORT_CSV, EXPORT_ALL},
		db,
	)
	return templates

}
