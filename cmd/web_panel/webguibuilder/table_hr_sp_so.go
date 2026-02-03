package webguibuilder

import (
	"html/template"
	"reflect"
	"service-platform/cmd/web_panel/fun"
	"service-platform/cmd/web_panel/internal/gormdb"
	sptechnicianmodel "service-platform/cmd/web_panel/model/sp_technician_model"
	"service-platform/cmd/web_panel/webgui"

	"github.com/go-redis/redis/v8"
)

func TABLE_DATA_SP_SO(session string, redisDB *redis.Client) template.HTML {
	var tableHeaders []webgui.Column
	db := gormdb.Databases.Web
	dbData := sptechnicianmodel.SPofStockOpname{}
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

	templates := webgui.RenderDataTableServerSideSPSO(
		"Surat Peringatan (SP) - Stock Opname",
		"dt_sp_stock_opname",
		fun.GLOBAL_URL+"web/"+fun.GetRedis("web:"+session, redisDB)+"/tab-hr-sp-so/table",
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
