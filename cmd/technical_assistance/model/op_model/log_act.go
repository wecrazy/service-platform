package op_model

import "time"

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
	return "log_act"
}
