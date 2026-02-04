package model

import (
	"service-platform/internal/config"
	"time"

	"gorm.io/gorm"
)

type TicketHommyPayCC struct {
	// ID uint `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	ID uint `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	gorm.Model
	// CreatedAt       time.Time      `json:"created_at"`
	// UpdatedAt       time.Time      `json:"updated_at"`
	// DeletedAt       gorm.DeletedAt `gorm:"index" json:"deleted_at"`
	Priority        string     `gorm:"type:text;column:priority;default:NULL" json:"priority"`
	SlaDeadline     *time.Time `gorm:"column:sla_deadline" json:"sla_deadline"` // sla deadline default is tomorrow
	TicketId        int        `gorm:"type:int;column:ticket_id" json:"ticket_id"`
	TicketNumber    string     `gorm:"type:text;column:ticket_number;default:NULL" json:"ticket_number"`
	TicketTypeId    int        `gorm:"type:int;column:ticket_type_id" json:"-"`
	TicketType      string     `gorm:"type:text;column:ticket_type;default:NULL" json:"ticket_type"`
	StageId         int        `gorm:"type:int;column:stage_id" json:"-"`
	Stage           string     `gorm:"type:text;column:stage;default:NULL" json:"stage"`
	TechnicianId    int        `gorm:"type:int;column:technician_id" json:"-"`
	TechnicianName  string     `gorm:"type:text;column:technician_name;default:NULL" json:"technician_name"`
	CustomerId      int        `gorm:"type:int;column:customer_id" json:"-"`
	Customer        string     `gorm:"type:text;column:customer;default:NULL" json:"customer"`
	CustomerPhone   string     `gorm:"type:text;column:customer_phone;default:NULL" json:"customer_phone"`
	CustomerEmail   string     `gorm:"type:text;column:customer_email;default:NULL" json:"customer_email"`
	CustomerAddress string     `gorm:"type:text;column:customer_address;default:NULL" json:"customer_address"`
	SnId            int        `gorm:"type:int;column:sn_id" json:"-"`
	Sn              string     `gorm:"type:text;column:sn;default:NULL" json:"sn"`
	ProductId       int        `gorm:"type:int;column:product_id" json:"-"`
	Product         string     `gorm:"type:text;column:product;default:NULL" json:"product"`
	Description     string     `gorm:"type:text;column:description;default:NULL" json:"description"` // from Customer
	Keterangan      string     `gorm:"type:text;column:keterangan;default:NULL" json:"keterangan"`
	AdminId         int        `gorm:"type:int;column:admin_id" json:"-"`
	StatusInOdoo    string     `gorm:"type:text;column:status_in_odoo;default:NULL" json:"status_in_odoo"`
	StanzaId        string     `gorm:"type:text;column:stanza_id;default:NULL" json:"stanza_id"` // to store message id from customer
}

func (TicketHommyPayCC) TableName() string {
	return config.WebPanel.Get().Database.TbTicketHommyPayCC
}
