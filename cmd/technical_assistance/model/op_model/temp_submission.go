package op_model

import (
	"time"
)

// TempSubmission represents the temp_submission table, a temporary storage for submissions (means while pending/error submitted but soon it will be fixed)
type TempSubmission struct {
	ID int `gorm:"column:id;primaryKey;autoIncrement" json:"id"`

	IDTask              string     `gorm:"column:id_task;type:varchar(250)" json:"id_task"`
	WONumber            string     `gorm:"column:wo;type:varchar(250)" json:"wo"`
	SPKNumber           string     `gorm:"column:spk;type:varchar(250)" json:"spk"`
	Problem             string     `gorm:"column:problem;type:text" json:"problem"`
	ReceivedDatetimeSPK *time.Time `gorm:"column:received_datetime_spk" json:"received_datetime_spk"`
	TypeCase            string     `gorm:"column:type_case;type:varchar(250)" json:"type_case"`
	Type                string     `gorm:"column:type;type:varchar(250)" json:"type"`
	Type2               string     `gorm:"column:type2;type:varchar(250)" json:"type2"`
	SLA                 *time.Time `gorm:"column:sla" json:"sla"`
	TimeStart           *time.Time `gorm:"column:time_start" json:"time_start"`
	TimeStop            *time.Time `gorm:"column:time_stop" json:"time_stop"`
	Keterangan          string     `gorm:"column:keterangan;type:text" json:"keterangan"`
	Desc                string     `gorm:"column:desc;type:text" json:"desc"`
	Company             string     `gorm:"column:company;type:varchar(250)" json:"company"`
	Reason              string     `gorm:"column:reason;type:varchar(250)" json:"reason"`
	TID                 string     `gorm:"column:tid;type:varchar(100)" json:"tid"`
	Merchant            string     `gorm:"column:merchant;type:text" json:"merchant"`
	Teknisi             string     `gorm:"column:teknisi;type:varchar(250)" json:"teknisi"`
	MID                 string     `gorm:"column:mid;type:text" json:"mid"`
	Alamat              string     `gorm:"column:alamat;type:text" json:"alamat"`
	TipeEdc             string     `gorm:"column:edc_type;type:text" json:"edc_type"`
	SnEdc               string     `gorm:"column:sn;type:text" json:"sn"`
	TidBank             string     `gorm:"column:tid_bank;type:text" json:"tid_bank"`
	Date                *time.Time `gorm:"column:date" json:"date"` // Date in Dashboard
	DateOnCheck         *time.Time `gorm:"column:date_on_check" json:"date_on_check"`
	TaFeedback          string     `gorm:"column:ta_feedback;type:text" json:"ta_feedback"`

	Email   string `gorm:"column:email;type:varchar(250)" json:"email"` // Email of the submitter / editor
	Method  string `gorm:"column:method;type:varchar(250)" json:"method"`
	LogEdit string `gorm:"column:log_edit;type:text" json:"log_edit"` // Log of edits made if its being edited

	Foto  string `gorm:"-" json:"foto"`
	Cek   string `gorm:"-" json:"-"`
	Edit  string `gorm:"-" json:"edit"`
	Hapus string `gorm:"-" json:"hapus"`
}

func (TempSubmission) TableName() string {
	return "temp_submission"
}

// Query to create table:
// CREATE TABLE IF NOT EXISTS `temp_submission` (
//   `id` INT AUTO_INCREMENT PRIMARY KEY,

//   `id_task` VARCHAR(250) NULL,
//   `wo` VARCHAR(250) NULL,
//   `spk` VARCHAR(250) NULL,
//   `problem` TEXT NULL,
//   `received_datetime_spk` DATETIME NULL,
//   `type_case` VARCHAR(250) NULL,
//   `type` VARCHAR(250) NULL,
//   `type2` VARCHAR(250) NULL,
//   `sla` DATETIME NULL,
//   `time_start` DATETIME NULL,
//   `time_stop` DATETIME NULL,
//   `keterangan` TEXT NULL,
//   `desc` TEXT NULL,
//   `company` VARCHAR(250) NULL,
//   `reason` VARCHAR(250) NULL,
//   `tid` VARCHAR(100) NULL,
//   `merchant` TEXT NULL,
//   `teknisi` VARCHAR(250) NULL,
//   `mid` TEXT NULL,
//   `alamat` TEXT NULL,
//   `edc_type` TEXT NULL,
//   `sn` TEXT NULL,
//   `tid_bank` TEXT NULL,
//   `date` DATETIME NULL,
//   `date_on_check` DATETIME NULL,
//   `ta_feedback` TEXT NULL,
//   `email` VARCHAR(250) NULL,
//   `method` VARCHAR(250) NULL,
//   `log_edit` TEXT NULL
// ) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci;
