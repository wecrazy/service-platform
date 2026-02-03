package reportmodel

import (
	"service-platform/cmd/web_panel/config"
	"time"

	"gorm.io/gorm"
)

type MTIReportPemasangan struct {
	ID uint `gorm:"primaryKey" json:"id"`
	gorm.Model

	NoPerintahKerja     string     `gorm:"column:no_perintah_kerja" json:"no_perintah_kerja"`
	AktivitasKerja      string     `gorm:"column:aktivitas_kerja" json:"aktivitas_kerja"`
	MID                 string     `gorm:"column:mid" json:"mid"`
	TID                 string     `gorm:"column:tid" json:"tid"`
	TIDSebelum          string     `gorm:"column:tid_sebelum" json:"tid_sebelum"`
	NamaResmiMerchant   string     `gorm:"column:nama_resmi_merchant" json:"nama_resmi_merchant"`
	NamaMerchant        string     `gorm:"column:nama_merchant" json:"nama_merchant"`
	Alamat123           string     `gorm:"column:alamat_1_3" json:"alamat_1_3"`
	ContactPerson       string     `gorm:"column:contact_person" json:"contact_person"`
	NoTelepon           string     `gorm:"column:no_telepon" json:"no_telepon"`
	Region              string     `gorm:"column:region" json:"region"`
	Kota                string     `gorm:"column:kota" json:"kota"`
	KodePos             string     `gorm:"column:kode_pos" json:"kode_pos"`
	NoHP                string     `gorm:"column:no_hp" json:"no_hp"`
	MerchantSegment     string     `gorm:"column:merchant_segment" json:"merchant_segment"`
	TipeEDC             string     `gorm:"column:tipe_edc" json:"tipe_edc"`
	SNEDC               string     `gorm:"column:sn_edc" json:"sn_edc"`
	SNSimcard           string     `gorm:"column:sn_simcard" json:"sn_simcard"`
	PenyediaSimcard     string     `gorm:"column:penyedia_simcard" json:"penyedia_simcard"`
	SNSamcard           string     `gorm:"column:sn_samcard" json:"sn_samcard"`
	Vendor              string     `gorm:"column:vendor" json:"vendor"`
	FiturEDC            string     `gorm:"column:fitur_edc" json:"fitur_edc"`
	TipeKoneksiEDC      string     `gorm:"column:tipe_koneksi_edc" json:"tipe_koneksi_edc"`
	TglMulaiKerja       *time.Time `gorm:"column:tgl_mulai_kerja" json:"tgl_mulai_kerja"`
	TglSLATarget        *time.Time `gorm:"column:tgl_sla_target" json:"tgl_sla_target"`
	TglSelesaiKerja     *time.Time `gorm:"column:tgl_selesai_kerja" json:"tgl_selesai_kerja"`
	StatusEDC           string     `gorm:"column:status_edc" json:"status_edc"`
	Versi               string     `gorm:"column:versi" json:"versi"`
	StatusPerintahKerja string     `gorm:"column:status_perintah_kerja" json:"status_perintah_kerja"`
	Remarks             string     `gorm:"column:remarks" json:"remarks"`
	PemilikTertunda     string     `gorm:"column:pemilik_tertunda" json:"pemilik_tertunda"`
	AlasanTertunda      string     `gorm:"column:alasan_tertunda" json:"alasan_tertunda"`
	Teknisi             string     `gorm:"column:teknisi" json:"teknisi"`
	ServicePoint        string     `gorm:"column:service_point" json:"service_point"`
	NoPermintaanKerja   string     `gorm:"column:no_permintaan_kerja" json:"no_permintaan_kerja"`
	Gudang              string     `gorm:"column:gudang" json:"gudang"`
	// Red Mark in Excel => maybe soon compared with ODOO data
	Status    string     `gorm:"column:status" json:"status"`
	Remark    string     `gorm:"column:remark" json:"remark"`
	RootCause string     `gorm:"column:root_cause" json:"root_cause"`
	TglPasang *time.Time `gorm:"column:tgl_pemasangan" json:"tgl_pemasangan"`
	LinkWod   string     `gorm:"column:link_wod" json:"link_wod"`
}

func (MTIReportPemasangan) TableName() string {
	return config.GetConfig().ReportMTI.DBTablePemasangan
}
