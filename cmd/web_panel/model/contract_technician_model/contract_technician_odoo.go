package contracttechnicianmodel

import (
	"service-platform/cmd/web_panel/config"
	"time"

	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type ContractTechnicianODOO struct {
	ID uint `gorm:"primaryKey;autoIncrement" json:"id"`
	gorm.Model

	TechnicianID int    `gorm:"column:technician_id;type:int" json:"technician_id"`
	Technician   string `gorm:"column:technician;type:varchar(255)" json:"technician"`
	Name         string `gorm:"column:name;type:varchar(255)" json:"name"`
	ForProject   string `gorm:"column:for_project;type:varchar(255)" json:"for_project"`
	Email        string `gorm:"column:email;type:varchar(255)" json:"email"`
	Phone        string `gorm:"column:phone;type:varchar(50)" json:"phone"`
	SPL          string `gorm:"column:spl;type:varchar(255)" json:"spl"`
	SAC          string `gorm:"column:sac;type:varchar(255)" json:"sac"`

	LastLogin      *time.Time `gorm:"column:last_login" json:"last_login"`
	LastDownloadJO *time.Time `gorm:"column:last_download_jo" json:"last_download_jo"`
	UserCreatedOn  *time.Time `gorm:"column:user_created_on" json:"user_created_on"`
	FirstUploadJO  *time.Time `gorm:"column:first_upload_jo" json:"first_upload_jo"`
	LastVisit      *time.Time `gorm:"column:last_visit" json:"last_visit"`
	JoinDate       *time.Time `gorm:"column:join_date" json:"join_date"` // Used for OLD Technician (before 2025)
	EmployeeCode   string     `gorm:"column:employee_code;type:varchar(255)" json:"employee_code"`

	WONumber                  datatypes.JSON `gorm:"column:wo_number;type:json" json:"wo_number"`
	TicketSubject             datatypes.JSON `gorm:"column:ticket_subject;type:json" json:"ticket_subject"`
	WONumberAlreadyVisit      datatypes.JSON `gorm:"column:wo_number_already_visit;type:json" json:"wo_number_already_visit"`
	TicketSubjectAlreadyVisit datatypes.JSON `gorm:"column:ticket_subject_already_visit;type:json" json:"ticket_subject_already_visit"`

	JobGroupID                   int    `gorm:"column:job_group_id;type:int" json:"job_group_id"`
	NIK                          string `gorm:"column:nik;type:varchar(50)" json:"nik"`
	Alamat                       string `gorm:"column:alamat;type:text" json:"alamat"`
	Area                         string `gorm:"column:area;type:varchar(100)" json:"area"`
	TempatTanggalLahir           string `gorm:"column:tempat_tanggal_lahir;type:varchar(100)" json:"tempat_tanggal_lahir"`
	StatusPerkawinan             string `gorm:"column:status_perkawinan;type:varchar(100)" json:"status_perkawinan"`
	BankPenerimaGaji             string `gorm:"column:bank_penerima_gaji;type:varchar(100)" json:"bank_penerima_gaji"`
	NomorRekeningBank            string `gorm:"column:nomor_rekening_bank;type:varchar(100)" json:"nomor_rekening_bank"`
	NamaRekeningBankPenerimaGaji string `gorm:"column:nama_rekening_bank_penerima_gaji;type:varchar(255)" json:"nama_rekening_bank_penerima_gaji"`

	WhatsappChatID        string     `gorm:"column:whatsapp_chat_id;type:varchar(255)" json:"-"`
	WhatsappSentAt        *time.Time `gorm:"column:whatsapp_sent_at" json:"-"`
	WhatsappChatJID       string     `gorm:"column:whatsapp_chat_jid;type:varchar(255)" json:"-"`
	WhatsappSenderJID     string     `gorm:"column:whatsapp_sender_jid;type:varchar(255)" json:"-"`
	WhatsappMessageBody   string     `gorm:"column:whatsapp_message_body;type:text" json:"-"`
	WhatsappMessageType   string     `gorm:"column:whatsapp_message_type;type:varchar(50)" json:"-"`
	WhatsappQuotedMsgID   string     `gorm:"column:whatsapp_quoted_msg_id;type:varchar(255)" json:"-"`
	WhatsappReplyText     string     `gorm:"column:whatsapp_reply_text;type:text" json:"-"`
	WhatsappReactionEmoji string     `gorm:"column:whatsapp_reaction_emoji;type:varchar(16)" json:"-"`
	WhatsappMentions      string     `gorm:"column:whatsapp_mentions;type:text" json:"-"`
	WhatsappIsGroup       bool       `gorm:"column:whatsapp_is_group;default:false" json:"-"`
	WhatsappMsgStatus     string     `gorm:"column:whatsapp_msg_status;type:varchar(50)" json:"-"`
	WhatsappRepliedBy     string     `gorm:"column:whatsapp_replied_by;type:varchar(255)" json:"-"`
	WhatsappRepliedAt     *time.Time `gorm:"column:whatsapp_replied_at" json:"-"`
	WhatsappReactedBy     string     `gorm:"column:whatsapp_reacted_by;type:varchar(255)" json:"-"`
	WhatsappReactedAt     *time.Time `gorm:"column:whatsapp_reacted_at" json:"-"`

	ContractStartDate *time.Time `gorm:"column:contract_start_date" json:"contract_start_date"`
	ContractEndDate   *time.Time `gorm:"column:contract_end_date" json:"contract_end_date"`
	IsContractSent    bool       `gorm:"column:is_contract_sent;default:false" json:"is_contract_sent"`
	ContractFilePath  string     `gorm:"column:contract_file_path;type:text" json:"contract_file_path"`
	ContractSendAt    *time.Time `gorm:"column:contract_send_at" json:"contract_send_at"`

	RegenerateContract   string `gorm:"-" json:"regenerate_contract"`
	SendContract         string `gorm:"-" json:"send_contract"`
	WhatsappConversation string `gorm:"-" json:"whatsapp_conversation"`

	IsNotified bool `gorm:"column:is_notified;default:false" json:"is_notified"`
}

func (ContractTechnicianODOO) TableName() string {
	return config.GetConfig().ContractTechnicianODOO.TBContractTechnician
}
