// Package odoomsmodel defines the data structures for ODOO Manage Service (ODOOMS) like:
// - ODOOMSTechnicianItem: Represents a technician item in the ODOO Manage Service system, that get from fs.technician model
// These models are used to unmarshal JSON responses from ODOO's API and to work with ODOO data within the service-platform application.
// The fields in these models use fun.NullAble types to handle nullable values from ODOO, which can be null or false in the JSON response.
// For more details on how to use these models, see the documentation for the ODOOMS integration in the service-platform application.
package odoomsmodel

import "service-platform/pkg/fun"

// ODOOMSTechnicianItem represents a technician item in the ODOO Manage Service system, that get from fs.technician model
type ODOOMSTechnicianItem struct {
	ID                           uint                  `json:"id"`
	Email                        fun.NullAbleString    `json:"email"`
	Password                     fun.NullAbleString    `json:"password"`
	NoTelp                       fun.NullAbleString    `json:"x_no_telp"`
	TechnicianName               fun.NullAbleString    `json:"x_technician_name"`
	NameFS                       fun.NullAbleString    `json:"name"`
	Head                         fun.NullAbleString    `json:"x_spl_leader"`
	SPL                          fun.NullAbleString    `json:"technician_code"`
	LoginIDs                     []fun.NullAbleFloat   `json:"login_ids"`
	DownloadIDs                  []fun.NullAbleFloat   `json:"download_ids"`
	EmployeeIDs                  []fun.NullAbleFloat   `json:"employee_ids"`
	CreatedOn                    fun.NullAbleString    `json:"create_date"`
	CreatedUID                   fun.NullAbleInterface `json:"create_uid"`
	WriteDate                    fun.NullAbleString    `json:"write_date"` // last updated time
	WriteUID                     fun.NullAbleInterface `json:"write_uid"`
	JobGroupID                   fun.NullAbleInterface `json:"job_group_id"`
	NIK                          fun.NullAbleString    `json:"nik"`
	Alamat                       fun.NullAbleString    `json:"address"`
	Area                         fun.NullAbleString    `json:"area"`
	TempatTanggalLahir           fun.NullAbleString    `json:"birth_status"`
	StatusPerkawinan             fun.NullAbleString    `json:"marriage_status"`
	BankPenerimaGaji             fun.NullAbleString    `json:"payment_bank"`
	NoRekeningBankPenerimaGaji   fun.NullAbleString    `json:"payment_bank_id"`
	NamaRekeningBankPenerimaGaji fun.NullAbleString    `json:"payment_bank_name"`
	Active                       fun.NullAbleBoolean   `json:"active"`
	EmployeeCode                 fun.NullAbleString    `json:"x_employee_code"`
	TechnicianLocations          []fun.NullAbleFloat   `json:"technician_locations"`
}
