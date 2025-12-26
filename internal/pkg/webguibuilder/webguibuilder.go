package webguibuilder

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
