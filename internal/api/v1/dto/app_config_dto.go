package dto

// AppConfigTableRequest represents the request for the App Config table (DataTables)
type AppConfigTableRequest struct {
	// Draw counter. This is used by DataTables to ensure that the Ajax returns from server-side processing requests are drawn in sequence by DataTables.
	Draw int `form:"draw" example:"1"`
	// Paging first record indicator. This is the start point in the current data set (0 index based - i.e. 0 is the first record).
	Start int `form:"start" example:"0"`
	// Number of records that the table can display in the current draw.
	Length int `form:"length" example:"10"`
	// Global search value. To be applied to all columns which have searchable as true.
	Search string `form:"search[value]"`
	// Column to which ordering should be applied.
	SortColumn int `form:"order[0][column]" example:"0"`
	// Ordering direction for this column. It will be asc or desc.
	SortDir string `form:"order[0][dir]" example:"asc"`

	// Filter by No
	No string `form:"no" json:"no"`
	// Filter by Full Name
	FullName string `form:"full_name" json:"full_name" gorm:"column:full_name"`
}
