package odoomsmodel

import "service-platform/internal/pkg/fun"

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
	CreatedUid                   fun.NullAbleInterface `json:"create_uid"`
	WriteDate                    fun.NullAbleString    `json:"write_date"` // last updated time
	WriteUid                     fun.NullAbleInterface `json:"write_uid"`
	JobGroupId                   fun.NullAbleInterface `json:"job_group_id"`
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
