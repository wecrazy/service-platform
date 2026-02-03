package webguibuilder

import (
	"html/template"
	"reflect"
	"service-platform/cmd/web_panel/fun"
	"service-platform/cmd/web_panel/model"
	"service-platform/cmd/web_panel/webgui"

	"github.com/go-redis/redis/v8"
)

func TABLE_MERCHANT(session string, redisDB *redis.Client) template.HTML {
	// Handling Manufactures
	var tableHeaders []webgui.Column
	dbData := model.MerchantHommyPayCC{}
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

	templates := webgui.RenderDataTableServerSideDataMerchant(
		"Data Merchant",
		"dt_merchant_hommy_pay_cc",
		fun.GLOBAL_URL+"web/"+fun.GetRedis("web:"+session, redisDB)+"/tab-hommy-pay-cc-merchant/table",
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
	)
	return templates
}

func TABLE_MERCHANT_FASTLINK(session string, redisDB *redis.Client) template.HTML {
	// Handling Manufactures
	var tableHeaders []webgui.Column
	dbData := model.MerchantFastlink{}
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

	templates := webgui.RenderDataTableServerSideDataMerchant(
		"Data Merchant FastLink",
		"dt_merchant_fastlink",
		fun.GLOBAL_URL+"web/"+fun.GetRedis("web:"+session, redisDB)+"/tab-hommy-pay-cc-merchant/table_fastlink",
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
	)
	return templates
}

func TABLE_MERCHANT_KRESEKBAG(session string, redisDB *redis.Client) template.HTML {
	// Handling Manufactures
	var tableHeaders []webgui.Column
	dbData := model.MerchantKresekBag{}
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
				Editable:   false,
				Insertable: false,
				Orderable:  true,
			},
		)
	}

	templates := webgui.RenderDataTableServerSideDataMerchant(
		"Data Merchant",
		"dt_merchant_kresek_bag",
		fun.GLOBAL_URL+"web/"+fun.GetRedis("web:"+session, redisDB)+"/tab-hommy-pay-cc-merchant/table_kresekbag",
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
	)
	return templates
}
