package webguibuilder

import (
	"html/template"
	"reflect"
	"service-platform/cmd/web_panel/fun"
	"service-platform/cmd/web_panel/internal/gormdb"
	dspmodel "service-platform/cmd/web_panel/model/dsp_model"
	"service-platform/cmd/web_panel/webgui"

	"github.com/go-redis/redis/v8"
)

func TABLE_TICKET_DSP(session string, redisDB *redis.Client) template.HTML {
	// Handling Manufactures
	dbWeb := gormdb.Databases.Web
	var tableHeaders []webgui.Column
	dbData := dspmodel.TicketDSP{}
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

	templates := webgui.RenderDataTableServerSideTicketDSP(
		"Ticket WO (Work Order)",
		"dt_data_ticket_dsp",
		fun.GLOBAL_URL+"web/"+fun.GetRedis("web:"+session, redisDB)+"/tab-dsp-ticket/table",
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
		dbWeb,
	)
	return templates
}
