package tamodel

import (
	"service-platform/internal/config"
	"time"
)

// LogAct represents a log activity record in the database
type LogAct struct {
	ID              int        `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	Wo              *string    `gorm:"column:wo;type:varchar(300);default:null" json:"wo"`
	SpkNumber       string     `gorm:"column:spk;type:varchar(500);not null;default:0" json:"spk"`
	Teknisi         string     `gorm:"column:teknisi;type:varchar(500);not null;default:0" json:"teknisi"`
	TypeCase        string     `gorm:"column:type_case;type:varchar(500);not null;default:0" json:"type_case"`
	Problem         string     `gorm:"column:problem;type:varchar(500);not null;default:0" json:"problem"`
	Type            string     `gorm:"column:type;type:varchar(500);not null;default:0" json:"type"`
	Type2           string     `gorm:"column:type2;type:varchar(500);not null;default:0" json:"type2"`
	Sla             string     `gorm:"column:sla;type:varchar(500);not null;default:0" json:"sla"`
	Rc              string     `gorm:"column:rc;type:varchar(500);not null;default:0" json:"rc"`
	Tid             string     `gorm:"column:tid;type:varchar(500);not null;default:0" json:"tid"`
	Keterangan      string     `gorm:"column:keterangan;type:varchar(500);not null;default:0" json:"keterangan"`
	Email           string     `gorm:"column:email;type:varchar(255);not null" json:"email"`
	Method          string     `gorm:"column:method;type:varchar(100);not null" json:"method"`
	Reason          *string    `gorm:"column:reason;type:varchar(300);default:null" json:"reason"`
	Date            *time.Time `gorm:"column:date" json:"date"`
	DateOnCheck     *time.Time `gorm:"column:date_on_check" json:"date_on_check"`
	DateInDashboard string     `gorm:"column:date_in_dashboard" json:"date_in_dashboard"`
	TaFeedback      string     `gorm:"column:ta_feedback" json:"ta_feedback"`
	LogEdit         string     `gorm:"column:log_edit;type:text" json:"log_edit"`
}

func (LogAct) TableName() string {
	return config.GetConfig().TechnicalAssistance.Tables.TBLogActivity
}

// Pending represents a pending task record in the database
type Pending struct {
	ID                  int     `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	IDTask              string  `gorm:"column:id_task;type:varchar(50);not null;default:0" json:"id_task"`
	WoNumber            string  `gorm:"column:wo;type:varchar(300);not null;default:0" json:"wo"`
	SpkNumber           string  `gorm:"column:spk;type:varchar(300);not null;default:0" json:"spk"`
	ReceivedDatetimeSpk string  `gorm:"column:receiveDate;type:varchar(300);not null;default:0" json:"receiveDate"`
	Type                *string `gorm:"column:type;type:varchar(300);default:null" json:"type"`
	Type2               *string `gorm:"column:type2;type:varchar(300);default:null" json:"type2"`
	Sla                 *string `gorm:"column:sla;type:varchar(300);default:null" json:"sla"`
	TimeStart           string  `gorm:"column:time_start;type:varchar(50);not null;default:0" json:"time_start"`
	TimeStop            string  `gorm:"column:time_stop;type:varchar(50);not null;default:0" json:"time_stop"`
	Keterangan          *string `gorm:"column:keterangan;type:varchar(2000);default:null" json:"keterangan"`
	Desc                *string `gorm:"column:desc;type:varchar(2000);default:null" json:"desc"`
	Company             string  `gorm:"column:company;type:varchar(300);default:0" json:"company"`
	Reason              string  `gorm:"column:reason;type:varchar(300);default:0" json:"reason"`
	TID                 string  `gorm:"column:tid;type:varchar(50);default:0" json:"tid"`
	Merchant            *string `gorm:"column:merchant;type:text;default:null" json:"merchant"`
	Teknisi             string  `gorm:"column:teknisi;type:varchar(300);default:null" json:"teknisi"`

	// Problem    *string   `gorm:"column:problem;type:text;default:null" json:"problem"`

	MID         string     `gorm:"column:mid;type:text;default:null" json:"mid"`
	Alamat      string     `gorm:"column:alamat;type:text;default:null" json:"alamat"`
	TipeEdc     string     `gorm:"column:edc_type;type:text;default:null" json:"edc_type"`
	SnEdc       string     `gorm:"column:sn;type:text;default:null" json:"sn"`
	TidBank     string     `gorm:"column:tid_bank;type:text;default:null" json:"tid_bank"`
	Date        time.Time  `gorm:"column:date" json:"date"`
	DateOnCheck *time.Time `gorm:"column:date_on_check" json:"-"`
	Foto        string     `gorm:"-" json:"foto"`
	TaFeedback  string     `gorm:"column:ta_feedback;type:text;default:null" json:"ta_feedback"`
	Cek         string     `gorm:"-" json:"cek"`
	Edit        string     `gorm:"-" json:"edit"`
	Konfirmasi  string     `gorm:"-" json:"konfirmasi"`
	Hapus       string     `gorm:"-" json:"hapus"`
}

func (Pending) TableName() string {
	return config.GetConfig().TechnicalAssistance.Tables.TBPending
}

// Error represents an photo error record in the database
type Error struct {
	ID                  int        `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	IDTask              string     `gorm:"column:id_task;type:varchar(50);not null;default:0" json:"id_task"`
	WoNumber            string     `gorm:"column:wo;type:varchar(300);not null;default:0" json:"wo"`
	SpkNumber           string     `gorm:"column:spk;type:varchar(300);not null;default:0" json:"spk"`
	ReceivedDatetimeSpk string     `gorm:"column:receiveDate;type:varchar(300);not null;default:0" json:"receiveDate"`
	Type                *string    `gorm:"column:type;type:varchar(300);default:null" json:"type"`
	Type2               *string    `gorm:"column:type2;type:varchar(300);default:null" json:"type2"`
	Sla                 *string    `gorm:"column:sla;type:varchar(300);default:null" json:"sla"`
	TimeStart           string     `gorm:"column:time_start;type:varchar(50);not null;default:0" json:"time_start"`
	TimeStop            string     `gorm:"column:time_stop;type:varchar(50);not null;default:0" json:"time_stop"`
	Keterangan          *string    `gorm:"column:keterangan;type:varchar(2000);default:null" json:"keterangan"`
	Desc                *string    `gorm:"column:desc;type:varchar(2000);default:null" json:"desc"`
	Company             string     `gorm:"column:company;type:varchar(300);default:0" json:"company"`
	Reason              string     `gorm:"column:reason;type:varchar(300);default:0" json:"reason"`
	TID                 string     `gorm:"column:tid;type:varchar(50);default:0" json:"tid"`
	Merchant            *string    `gorm:"column:merchant;type:text;default:null" json:"merchant"`
	Teknisi             string     `gorm:"column:teknisi;type:varchar(300);default:null" json:"teknisi"`
	Problem             *string    `gorm:"column:problem;type:text;default:null" json:"problem"`
	MID                 string     `gorm:"column:mid;type:text;default:null" json:"mid"`
	Alamat              string     `gorm:"column:alamat;type:text;default:null" json:"alamat"`
	TipeEdc             string     `gorm:"column:edc_type;type:text;default:null" json:"edc_type"`
	SnEdc               string     `gorm:"column:sn;type:text;default:null" json:"sn"`
	TidBank             string     `gorm:"column:tid_bank;type:text;default:null" json:"tid_bank"`
	Date                time.Time  `gorm:"column:date" json:"date"`
	DateOnCheck         *time.Time `gorm:"column:date_on_check" json:"-"`
	Foto                string     `gorm:"-" json:"foto"`
	TaFeedback          string     `gorm:"column:ta_feedback;type:text;default:null" json:"ta_feedback"`
	Cek                 string     `gorm:"-" json:"cek"`
	Edit                string     `gorm:"-" json:"edit"`
	Konfirmasi          string     `gorm:"-" json:"konfirmasi"`
	Hapus               string     `gorm:"-" json:"hapus"`
}

func (Error) TableName() string {
	return config.GetConfig().TechnicalAssistance.Tables.TBError
}
