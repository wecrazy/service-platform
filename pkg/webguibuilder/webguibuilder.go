// Package webguibuilder provides builder helpers and constants for constructing
// server-side DataTable configurations used in the web GUI.
package webguibuilder

// variables and constants for DataTable configuration
const (
	ExportCopy      = "COPY"
	ExportPrint     = "PRINT"
	ExportCSV       = "CSV"
	ExportPdf       = "PDF"
	ExportAll       = "ALL"
	INSERTABLE      = true
	EDITABLE        = true
	DELETABLE       = true
	HideHeader      = true
	PASSWORDABLE    = true
	ScrollUpDown    = true
	ScrollLeftRight = true
)

// not is a helper function that returns the opposite of a boolean value. It is used in the configuration of DataTable features to enable or disable certain functionalities based on the provided constants.
func not(b bool) bool {
	return !b
}
