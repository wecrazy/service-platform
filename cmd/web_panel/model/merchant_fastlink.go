package model

import (
	"service-platform/cmd/web_panel/config"
	"time"

	"gorm.io/gorm"
)

type MerchantFastlink struct {
	ID        uint `gorm:"primarykey"`
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt gorm.DeletedAt `gorm:"index"`

	Status        int16  `json:"status" gorm:"column:status;default:1"`
	OdooPartnerId string `json:"odoo_partner_id" gorm:"column:odoo_partner_id"`
	ProfileImage  string `json:"profile_image" gorm:"column:profile_image"`

	Fullname          string `json:"fullname" gorm:"column:fullname"`
	Name              string `json:"name" gorm:"column:name"`
	NIK               string `json:"nik" gorm:"column:nik"`
	NPWP              string `json:"npwp" gorm:"column:npwp"`
	PemilikUsaha      string `json:"pemilik_usaha" gorm:"column:pemilik_usaha"`
	NPWPBadanUsaha    string `json:"npwp_badan_usaha" gorm:"column:npwp_badan_usaha"`
	NamaUsaha         string `json:"nama_usaha" gorm:"column:nama_usaha"`
	MCC               string `json:"mcc" gorm:"column:mcc;size:5"`
	JenisUsaha        string `json:"jenis_usaha" gorm:"column:jenis_usaha"`
	JenisUsahaEN      string `json:"jenis_usaha_en" gorm:"column:jenis_usaha_en"`
	StatusLokasiUsaha string `json:"status_lokasi_usaha" gorm:"column:status_lokasi_usaha"`
	BentukBadanUsaha  string `json:"bentuk_badan_usaha" gorm:"column:bentuk_badan_usaha"`
	MerchantName      string `json:"merchant_name" gorm:"column:merchant_name"`

	PhoneAreaCode        string    `json:"phone_area_code" gorm:"column:phone_area_code"`
	PhoneNumber          string    `json:"phone_number" binding:"required" gorm:"column:phone_number"`
	PhoneNumberReferensi string    `json:"phone_number_referensi" gorm:"column:phone_number_referensi"`
	Email                string    `json:"email" gorm:"column:email"`
	PasswordRaw          string    `json:"password_raw" gorm:"column:password_raw"`
	Password             string    `json:"password" gorm:"column:password"`
	PasswordLastChanged  time.Time `json:"password_last_changed" gorm:"column:password_last_changed"`

	Mdr          float64 `gorm:"column:mdr" json:"mdr"` // potongan fee (0%, 0.3%, 0.7%)
	DokumenLink  string  `gorm:"column:dokumen_link" json:"dokumen_link"`
	NMID         string  `json:"nmid" gorm:"column:nmid"`
	NobuMPAN     string  `json:"nobu_mpan" gorm:"column:nobu_mpan"`
	NobuMID      string  `json:"nobu_mid" gorm:"column:nobu_mid"`
	QRISPenerbit string  `json:"qris_penerbit" gorm:"column:qris_penerbit;size:20"`
	QRISStatic   bool    `json:"qris_static" gorm:"column:qris_static"`
	QRISDynamic  bool    `json:"qris_dynamic" gorm:"column:qris_dynamic"`
	QRISText     string  `json:"qris_text" gorm:"column:qris_text"`
	QRISImage    string  `json:"qris_image" gorm:"column:qris_image"`

	Address     string  `json:"address" gorm:"column:address"`
	PostalCode  string  `json:"postal_code" gorm:"column:postal_code;size:10"`
	Subdistrict string  `json:"subdistrict" gorm:"column:subdistrict"`
	District    string  `json:"district" gorm:"column:district"`
	City        string  `json:"city" gorm:"column:city"`
	Province    string  `json:"province" gorm:"column:province"`
	Country     string  `json:"country" gorm:"column:country"`
	Latitude    float64 `json:"latitude" gorm:"column:latitude;type:decimal(10,8)"`
	Longitude   float64 `json:"longitude" gorm:"column:longitude;type:decimal(11,8)"`

	Pin               int    `json:"pin" gorm:"column:pin"`
	StatusDescription string `json:"status_description" gorm:"column:status_description"`
	CreatedBy         uint   `json:"created_by" gorm:"column:created_by;default:0"`
	UpdatedBy         uint   `json:"updated_by" gorm:"column:updated_by;default:0"`

	Saldo          int    `json:"saldo" gorm:"column:saldo"`
	SaldoPending   int    `json:"saldo_pending" gorm:"column:saldo_pending"`
	IP             string `json:"ip" gorm:"column:ip"`
	Role           int    `json:"role" gorm:"column:role"`
	LastLogin      int64  `json:"last_login" gorm:"column:last_login"`
	LoginDelay     int64  `json:"login_delay" gorm:"column:login_delay"`
	Session        string `json:"session" gorm:"column:session"`
	SessionExpired int64  `json:"session_expired" gorm:"column:session_expired"`

	NoKK            string `json:"no_kk" gorm:"column:no_kk"`
	AlamatUsaha     string `json:"alamat_usaha" binding:"required" gorm:"column:alamat_usaha"`
	JumlahCabang    string `json:"jumlah_cabang" binding:"required" gorm:"column:jumlah_cabang"`
	RerataTransaksi string `json:"rerata_transaksi" binding:"required" gorm:"column:rerata_transaksi"`

	NomorRekening       string `json:"nomor_rekening" binding:"required" gorm:"column:nomor_rekening"`
	NamaPemilikRekening string `json:"nama_pemilik_nomor_rekening" binding:"required" gorm:"column:nama_pemilik_nomor_rekening"`
	NamaBank            string `json:"nama_bank" gorm:"column:nama_bank"`
	CodeBic             string `json:"codebic" gorm:"column:codebic"`
	CabangBank          string `json:"cabang_bank" gorm:"column:cabang_bank"`

	NamaPerusahaan       string `json:"nama_perusahaan" binding:"required" gorm:"column:nama_perusahaan"`
	NamaPenanggungJawab  string `json:"nama_penanggung_jawab" binding:"required" gorm:"column:nama_penanggung_jawab"`
	EmailPenanggungJawab string `json:"email_penanggung_jawab" binding:"required" gorm:"column:email_penanggung_jawab"`

	TypeEDC    string `json:"type_edc" binding:"required" gorm:"column:type_edc"`
	UsahaFisik bool   `json:"usaha_fisik" binding:"required" gorm:"column:usaha_fisik"`

	Foto string `json:"photos" gorm:"-"`
	// FotoLokasiUsahaDepan    string `json:"foto_lokasi_usaha_depan" binding:"required" gorm:"column:foto_lokasi_usaha_depan"`
	// FotoLokasiUsahaBelakang string `json:"foto_lokasi_usaha_belakang" binding:"required" gorm:"column:foto_lokasi_usaha_belakang"`
	// FotoLokasiUsahaProduksi string `json:"foto_lokasi_usaha_produksi" binding:"required" gorm:"column:foto_lokasi_usaha_produksi"`
	// FotoLokasiUsahaStok     string `json:"foto_lokasi_usaha_stok" binding:"required" gorm:"column:foto_lokasi_usaha_stok"`
	// FotoKTP                 string `json:"foto_ktp" binding:"required" gorm:"column:foto_ktp"`
	// FotoTTD                 string `json:"foto_ttd" binding:"required" gorm:"column:foto_ttd"`

	JenisPendaftaran string `json:"jenis_pendaftaran" binding:"required" gorm:"column:jenis_pendaftaran"`
	Koneksi          string `json:"koneksi" binding:"required" gorm:"column:koneksi"`
	SSIDWifi         string `json:"ssid_wifi" binding:"required" gorm:"column:ssid_wifi"`
	SSIDWifiPassword string `json:"ssid_wifi_password" binding:"required" gorm:"column:ssid_wifi_password"`

	KodeReferal string `json:"kode_referal" binding:"required" gorm:"column:kode_referal"`
	Category    string `json:"categ" binding:"required" gorm:"column:categ"`
	Values      string `json:"values" binding:"required" gorm:"-"`
}

func (MerchantFastlink) TableName() string {
	return config.GetConfig().Database.TbMerchantFastlink
}

// var MerchantStatuses = map[string]int16{
// 	"REGISTRATION": -1,
// 	"REJECTED":     0,
// 	"PENDING":      1,
// 	"WAITING":      2,
// 	"ACCEPTED":     3,
// }
// var MerchantInfoStatuses = [...]string{"REJECTED", "PENDING", "WAITING", "ACCEPTED"}

// func (MerchantFastlink) StatusRegistration() int16 {
// 	return -1
// }
// func (MerchantFastlink) StatusRejected() int16 {
// 	return 0
// }
// func (MerchantFastlink) StatusPending() int16 {
// 	return 1
// }
// func (MerchantFastlink) StatusWaiting() int16 {
// 	return 2
// }
// func (MerchantFastlink) StatusAccepted() int16 {
// 	return 3
// }
// func (MerchantFastlink) StatusToStr(status int16) string {
// 	return MerchantInfoStatuses[status]
// }

// func (m *MerchantFastlink) BeforeCreate(tx *gorm.DB) (err error) {
// 	// Add logic before creating a record
// 	if m.PhoneAreaCode == "" {
// 		m.PhoneAreaCode = "62"
// 	}
// 	m.Role = 1
// 	m.CreatedAt = time.Now()
// 	m.UpdatedAt = time.Now()
// 	m.PasswordLastChanged = time.Now()
// 	if m.Status == 0 {
// 		m.Status = 1
// 	}
// 	return nil
// }

// func (m *MerchantFastlink) BeforeUpdate(tx *gorm.DB) (err error) {
// 	// Add logic before updating a record
// 	m.UpdatedAt = time.Now()
// 	return nil
// }

// func (m *MerchantFastlink) BeforeSave(tx *gorm.DB) (err error) {
// 	// Add logic before saving a record (applies to both create and update)
// 	// Example: Hashing the password before saving
// 	// if m.Password != "" {
// 	// 	hashedPassword, hashErr := hashPassword(m.Password)
// 	// 	if hashErr != nil {
// 	// 		return hashErr
// 	// 	}
// 	// 	m.Password = hashedPassword
// 	// }
// 	return nil
// }
