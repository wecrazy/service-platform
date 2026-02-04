package odooms

import (
	"service-platform/internal/config"
	"time"

	"gorm.io/gorm"
)

type MSTechnicianPayroll struct {
	ID uint `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	gorm.Model

	No                            int     `gorm:"column:no" json:"no"`
	ContractNo                    string  `gorm:"column:contract_no" json:"contract_no"`
	TanggalJoin                   string  `gorm:"column:tanggal_join" json:"tanggal_join"`
	Name                          string  `gorm:"column:name" json:"name"`
	Email                         string  `gorm:"column:email" json:"email"`
	Group                         string  `gorm:"column:group" json:"group"`
	Basic                         float64 `gorm:"column:basic" json:"basic"`
	JOTarget                      int     `gorm:"column:jo_target" json:"jo_target"`
	PMMeet                        int     `gorm:"column:pm_meet" json:"pm_meet"`
	PMOver                        int     `gorm:"column:pm_over" json:"pm_over"`
	PMUnworked                    int     `gorm:"column:pm_unworked" json:"pm_unworked"`
	NonPMMeet                     int     `gorm:"column:non_pm_meet" json:"non_pm_meet"`
	NonPMOver                     int     `gorm:"column:non_pm_over" json:"non_pm_over"`
	NonPMUnworked                 int     `gorm:"column:non_pm_unworked" json:"non_pm_unworked"`
	IncentivePerTicket            float64 `gorm:"column:incentive_per_ticket" json:"incentive_per_ticket"`
	JOIncentives                  int     `gorm:"column:jo_incentives" json:"jo_incentives"`
	TotalIncentives               float64 `gorm:"column:total_incentives" json:"total_incentives"`
	TotalRegular                  float64 `gorm:"column:total_regular" json:"total_regular"`
	JOBP                          int     `gorm:"column:jo_bp" json:"jo_bp"`
	TotalBP                       float64 `gorm:"column:total_bp" json:"total_bp"`
	JOATM                         int     `gorm:"column:jo_atm" json:"jo_atm"`
	TotalATM                      float64 `gorm:"column:total_atm" json:"total_atm"`
	PotonganOverduePM             float64 `gorm:"column:potongan_overdue_pm" json:"potongan_overdue_pm"`
	PotonganOverdueNonPM          float64 `gorm:"column:potongan_overdue_non_pm" json:"potongan_overdue_non_pm"`
	PotonganOverdueUnworkedPM     float64 `gorm:"column:potongan_overdue_unworked_pm" json:"potongan_overdue_unworked_pm"`
	PotonganOverdueUnworkedNonPM  float64 `gorm:"column:potongan_overdue_unworked_non_pm" json:"potongan_overdue_unworked_non_pm"`
	TotalPotonganOverdue          float64 `gorm:"column:total_potongan_overdue" json:"total_potongan_overdue"`
	TotalPotonganUnworked         float64 `gorm:"column:total_potongan_unworked" json:"total_potongan_unworked"`
	TotalPotonganTotal            float64 `gorm:"column:total_potongan_total" json:"total_potongan_total"`
	TotalAkhir                    float64 `gorm:"column:total_akhir" json:"total_akhir"`
	DiBayarkan                    float64 `gorm:"column:di_bayarkan" json:"di_bayarkan"`
	BankPenerimaGaji              string  `gorm:"column:bank_penerima_gaji" json:"bank_penerima_gaji"`
	NomorRekeningBankPenerimaGaji string  `gorm:"column:nomor_rekening_bank_penerima_gaji" json:"nomor_rekening_bank_penerima_gaji"`
	NamaRekeningBankPenerimaGaji  string  `gorm:"column:nama_rekening_bank_penerima_gaji" json:"nama_rekening_bank_penerima_gaji"`
	NPWP                          string  `gorm:"column:npwp" json:"npwp"`
	JORegular                     int     `gorm:"column:jo_regular" json:"jo_regular"`
	JOBPAll                       int     `gorm:"column:jo_bp_all" json:"jo_bp_all"`
	Rapel                         float64 `gorm:"column:rapel" json:"rapel"`

	IncentiveSolvedPending      int     `gorm:"column:incentive_solved_pending" json:"incentive_solved_pending"`
	TotalIncentiveSolvedPending float64 `gorm:"column:total_incentive_solved_pending" json:"total_incentive_solved_pending"`
	IncentiveSolved             int     `gorm:"column:incentive_solved" json:"incentive_solved"`
	TotalIncentiveSolved        float64 `gorm:"column:total_incentive_solved" json:"total_incentive_solved"`
	JOInstallation              int     `gorm:"column:jo_installation" json:"jo_installation"`
	IncentiveInstallation       float64 `gorm:"column:incentive_installation" json:"incentive_installation"`

	UploadedBy      string     `gorm:"column:uploaded_by" json:"uploaded_by"`
	PayslipSent     bool       `gorm:"column:payslip_sent" json:"payslip_sent"`
	PayslipSentAt   *time.Time `gorm:"column:payslip_sent_at" json:"payslip_sent_at"`
	PayslipFilepath string     `gorm:"column:payslip_filepath" json:"payslip_filepath"`

	NoHP              string  `gorm:"column:no_hp" json:"no_hp"`
	Other             float64 `gorm:"column:other" json:"other"`
	RegeneratePayslip string  `gorm:"-" json:"regenerate_payslip"`
	SendPayslip       string  `gorm:"-" json:"send_payslip"`

	// For send in Whatsapp
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
	// Fields below are not part of the database table, used for UI purposes
	// SenderName           string `gorm:"-" json:"sender_name"`
	// DestinationName      string `gorm:"-" json:"destination_name"`
	WhatsappConversation string `gorm:"-" json:"whatsapp_conversation"`
}

func (MSTechnicianPayroll) TableName() string {
	return config.WebPanel.Get().Database.TbMSTechnicianPayroll
}

type MSTechnicianPayrollTicketsRegularEDC struct {
	ID uint `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	gorm.Model

	Ticket             string     `gorm:"column:ticket" json:"ticket"`
	TicketType         string     `gorm:"column:ticket_type" json:"ticket_type"`
	Company            string     `gorm:"column:company" json:"company"`
	Project            string     `gorm:"column:project" json:"project"`
	Technician         string     `gorm:"column:technician" json:"technician"`
	Stage              string     `gorm:"column:stage" json:"stage"`
	SLADeadline        *time.Time `gorm:"column:sla_deadline" json:"sla_deadline"`
	CompleteDatetimeWo *time.Time `gorm:"column:complete_datetime_wo" json:"complete_datetime_wo"`
	ReAssigned         bool       `gorm:"column:re_assigned" json:"re_assigned"`
	SLAStatus          string     `gorm:"column:sla_status" json:"sla_status"`
	SLAStatusAssume    string     `gorm:"column:sla_status_assume" json:"sla_status_assume"`
	Paid               bool       `gorm:"column:paid" json:"paid"`
	Why                string     `gorm:"column:why" json:"why"`
	MIDTID             string     `gorm:"column:midtid" json:"midtid"`
	WorkedState        bool       `gorm:"column:worked_state" json:"worked_state"`
	TaskCount          int        `gorm:"column:task_count" json:"task_count"`

	UploadedBy string `gorm:"column:uploaded_by" json:"uploaded_by"`
}

func (MSTechnicianPayrollTicketsRegularEDC) TableName() string {
	return config.WebPanel.Get().Database.TbMSTechnicianPayrollTicketsRegularEDC
}

type MSTechnicianPayrollTicketsBP struct {
	ID uint `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	gorm.Model

	Ticket             string     `gorm:"column:ticket" json:"ticket"`
	TicketType         string     `gorm:"column:ticket_type" json:"ticket_type"`
	Company            string     `gorm:"column:company" json:"company"`
	Project            string     `gorm:"column:project" json:"project"`
	Technician         string     `gorm:"column:technician" json:"technician"`
	Stage              string     `gorm:"column:stage" json:"stage"`
	SLADeadline        *time.Time `gorm:"column:sla_deadline" json:"sla_deadline"`
	CompleteDatetimeWo *time.Time `gorm:"column:complete_datetime_wo" json:"complete_datetime_wo"`
	ReAssigned         bool       `gorm:"column:re_assigned" json:"re_assigned"`
	SLAStatus          string     `gorm:"column:sla_status" json:"sla_status"`
	SLAStatusAssume    string     `gorm:"column:sla_status_assume" json:"sla_status_assume"`
	Paid               bool       `gorm:"column:paid" json:"paid"`
	Why                string     `gorm:"column:why" json:"why"`
	MIDTID             string     `gorm:"column:midtid" json:"midtid"`
	MID                string     `gorm:"column:mid" json:"mid"`

	UploadedBy string `gorm:"column:uploaded_by" json:"uploaded_by"`
}

func (MSTechnicianPayrollTicketsBP) TableName() string {
	return config.WebPanel.Get().Database.TbMSTechnicianPayrollTicketsBP
}

type MSTechnicianPayrollTicketsUnworkedEDC struct {
	ID uint `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	gorm.Model

	Ticket     string `gorm:"column:ticket" json:"ticket"`
	TicketType string `gorm:"column:ticket_type" json:"ticket_type"`
	Company    string `gorm:"column:company" json:"company"`
	Project    string `gorm:"column:project" json:"project"`
	Technician string `gorm:"column:technician" json:"technician"`

	UploadedBy string `gorm:"column:uploaded_by" json:"uploaded_by"`
}

func (MSTechnicianPayrollTicketsUnworkedEDC) TableName() string {
	return config.WebPanel.Get().Database.TbMSTechnicianPayrollTicketsUnworkedEDC
}

type MSTechnicianPayrollTicketsRegularATM struct {
	ID uint `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	gorm.Model

	Ticket             string     `gorm:"column:ticket" json:"ticket"`
	TicketType         string     `gorm:"column:ticket_type" json:"ticket_type"`
	Company            string     `gorm:"column:company" json:"company"`
	Project            string     `gorm:"column:project" json:"project"`
	Technician         string     `gorm:"column:technician" json:"technician"`
	Stage              string     `gorm:"column:stage" json:"stage"`
	SLADeadline        *time.Time `gorm:"column:sla_deadline" json:"sla_deadline"`
	CompleteDatetimeWo *time.Time `gorm:"column:complete_datetime_wo" json:"complete_datetime_wo"`
	ReAssigned         bool       `gorm:"column:re_assigned" json:"re_assigned"`
	SLAStatus          string     `gorm:"column:sla_status" json:"sla_status"`
	SLAStatusAssume    string     `gorm:"column:sla_status_assume" json:"sla_status_assume"`
	Paid               bool       `gorm:"column:paid" json:"paid"`
	Why                string     `gorm:"column:why" json:"why"`
	SNATM              string     `gorm:"column:sn_atm" json:"sn_atm"`
	WorkedState        bool       `gorm:"column:worked_state" json:"worked_state"`
	TaskCount          int        `gorm:"column:task_count" json:"task_count"`

	UploadedBy string `gorm:"column:uploaded_by" json:"uploaded_by"`
}

func (MSTechnicianPayrollTicketsRegularATM) TableName() string {
	return config.WebPanel.Get().Database.TbMSTechnicianPayrollTicketsRegularATM
}

type MSTechnicianPayrollTicketsUnworkedATM struct {
	ID uint `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	gorm.Model

	Ticket     string `gorm:"column:ticket" json:"ticket"`
	TicketType string `gorm:"column:ticket_type" json:"ticket_type"`
	Company    string `gorm:"column:company" json:"company"`
	Project    string `gorm:"column:project" json:"project"`
	Technician string `gorm:"column:technician" json:"technician"`
	Stage      string `gorm:"column:stage" json:"stage"`

	UploadedBy string `gorm:"column:uploaded_by" json:"uploaded_by"`
}

func (MSTechnicianPayrollTicketsUnworkedATM) TableName() string {
	return config.WebPanel.Get().Database.TbMSTechnicianPayrollTicketsUnworkedATM
}

type MSTechnicianPayrollDedicatedATM struct {
	ID uint `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	gorm.Model

	No                           int     `gorm:"column:no" json:"no"`
	ContractNo                   string  `gorm:"column:contract_no" json:"contract_no"`
	Name                         string  `gorm:"column:name" json:"name"`
	Email                        string  `gorm:"column:email" json:"email"`
	Group                        string  `gorm:"column:group" json:"group"`
	Basic                        float64 `gorm:"column:basic" json:"basic"`
	JOTarget                     int     `gorm:"column:jo_target" json:"jo_target"`
	PMMeet                       int     `gorm:"column:pm_meet" json:"pm_meet"`
	PMOver                       int     `gorm:"column:pm_over" json:"pm_over"`
	PMUnworked                   int     `gorm:"column:pm_unworked" json:"pm_unworked"`
	NonPMMeet                    int     `gorm:"column:non_pm_meet" json:"non_pm_meet"`
	NonPMOver                    int     `gorm:"column:non_pm_over" json:"non_pm_over"`
	NonPMUnworked                int     `gorm:"column:non_pm_unworked" json:"non_pm_unworked"`
	IncentivePerTicket           float64 `gorm:"column:incentive_per_ticket" json:"incentive_per_ticket"`
	JOIncentives                 int     `gorm:"column:jo_incentives" json:"jo_incentives"`
	TotalIncentives              float64 `gorm:"column:total_incentives" json:"total_incentives"`
	TotalRegular                 float64 `gorm:"column:total_regular" json:"total_regular"`
	JOBP                         int     `gorm:"column:jo_bp" json:"jo_bp"`
	TotalBP                      float64 `gorm:"column:total_bp" json:"total_bp"`
	JOATM                        int     `gorm:"column:jo_atm" json:"jo_atm"`
	TotalATM                     float64 `gorm:"column:total_atm" json:"total_atm"`
	PotonganOverduePM            float64 `gorm:"column:potongan_overdue_pm" json:"potongan_overdue_pm"`
	PotonganOverdueNonPM         float64 `gorm:"column:potongan_overdue_non_pm" json:"potongan_overdue_non_pm"`
	PotonganOverdueUnworkedPM    float64 `gorm:"column:potongan_overdue_unworked_pm" json:"potongan_overdue_unworked_pm"`
	PotonganOverdueUnworkedNonPM float64 `gorm:"column:potongan_overdue_unworked_non_pm" json:"potongan_overdue_unworked_non_pm"`
	TotalPotonganOverdue         float64 `gorm:"column:total_potongan_overdue" json:"total_potongan_overdue"`
	TotalPotonganUnworked        float64 `gorm:"column:total_potongan_unworked" json:"total_potongan_unworked"`
	TotalPotonganTotal           float64 `gorm:"column:total_potongan_total" json:"total_potongan_total"`
	TotalAkhir                   float64 `gorm:"column:total_akhir" json:"total_akhir"`
	DiBayarkan                   float64 `gorm:"column:di_bayarkan" json:"di_bayarkan"`
	Bank                         string  `gorm:"column:bank" json:"bank"`
	AccNo                        string  `gorm:"column:acc_no" json:"acc_no"`
	AccName                      string  `gorm:"column:acc_name" json:"acc_name"`
	NPWP                         string  `gorm:"column:npwp" json:"npwp"`
	JORegular                    int     `gorm:"column:jo_regular" json:"jo_regular"`
	JOBPAll                      int     `gorm:"column:jo_bp_all" json:"jo_bp_all"`

	UploadedBy      string     `gorm:"column:uploaded_by" json:"uploaded_by"`
	PayslipSent     bool       `gorm:"column:payslip_sent" json:"payslip_sent"`
	PayslipSentAt   *time.Time `gorm:"column:payslip_sent_at" json:"payslip_sent_at"`
	PayslipFilepath string     `gorm:"column:payslip_filepath" json:"payslip_filepath"`

	NoHP              string  `gorm:"column:no_hp" json:"no_hp"`
	Other             float64 `gorm:"column:other" json:"other"`
	RegeneratePayslip string  `gorm:"-" json:"regenerate_payslip"`
	SendPayslip       string  `gorm:"-" json:"send_payslip"`

	// For send in Whatsapp
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
	// Fields below are not part of the database table, used for UI purposes
	// SenderName           string `gorm:"-" json:"sender_name"`
	// DestinationName      string `gorm:"-" json:"destination_name"`
	WhatsappConversation string `gorm:"-" json:"whatsapp_conversation"`
}

func (MSTechnicianPayrollDedicatedATM) TableName() string {
	return config.WebPanel.Get().Database.TbMSTechnicianPayrollDedicatedATM
}
