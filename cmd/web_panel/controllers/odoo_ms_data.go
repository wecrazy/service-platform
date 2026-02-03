package controllers

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"service-platform/cmd/web_panel/config"
	"service-platform/cmd/web_panel/internal/gormdb"
	bnimodel "service-platform/cmd/web_panel/model/bni_model"
	mtimodel "service-platform/cmd/web_panel/model/mti_model"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// Project.task
type OdooTaskDataRequestItem struct {
	ID                  int               `json:"id"`
	WoNumber            string            `json:"x_no_task"`
	MerchantName        nullAbleString    `json:"x_merchant"`
	PicMerchant         nullAbleString    `json:"x_pic_merchant"`
	PicPhone            nullAbleString    `json:"x_pic_phone"`
	MerchantAddress     nullAbleString    `json:"partner_street"`
	MerchantCity        nullAbleString    `json:"x_studio_kota"`
	MerchantZip         nullAbleString    `json:"partner_zip"`
	Description         nullAbleString    `json:"x_title_cimb"` // "description"
	TaskType            nullAbleString    `json:"x_task_type"`
	Mid                 nullAbleString    `json:"x_cimb_master_mid"`
	Tid                 nullAbleString    `json:"x_cimb_master_tid"`
	Source              nullAbleString    `json:"x_source"`
	MessageCC           nullAbleString    `json:"x_message_call"`
	StatusMerchant      nullAbleString    `json:"x_status_merchant"`
	WoRemarkTiket       nullAbleString    `json:"x_wo_remark"`
	Longitude           nullAbleString    `json:"x_longitude"`
	Latitude            nullAbleString    `json:"x_latitude"`
	LinkPhoto           nullAbleString    `json:"x_link_photo"`
	TicketTypeId        nullAbleInterface `json:"x_ticket_type2"`
	WorksheetTemplateId nullAbleInterface `json:"worksheet_template_id"`
	CompanyId           nullAbleInterface `json:"company_id"`
	StageId             nullAbleInterface `json:"stage_id"`
	HelpdeskTicketId    nullAbleInterface `json:"helpdesk_ticket_id"`
	SnEdc               nullAbleInterface `json:"x_studio_edc"`
	EdcType             nullAbleInterface `json:"x_product"`
	TechnicianId        nullAbleInterface `json:"technician_id"`
	ReasonCodeId        nullAbleInterface `json:"x_reason_code_id"`
	WriteUid            nullAbleInterface `json:"write_uid"`
	SlaDeadline         nullAbleTime      `json:"x_sla_deadline"`
	CreateDate          nullAbleTime      `json:"create_date"`
	ReceivedDatetimeSpk nullAbleTime      `json:"x_received_datetime_spk"`
	PlanDate            nullAbleTime      `json:"planned_date_begin"`
	TimesheetLastStop   nullAbleTime      `json:"timesheet_timer_last_stop"`
	DateLastStageUpdate nullAbleTime      `json:"date_last_stage_update"`
	CommunicationLine   nullAbleString    `json:"x_communication_line"`
	WoRemark            nullAbleString    `json:"x_keterangan"`
	Message             nullAbleString    `json:"x_message"`

	// Tanda - Tanda EDC Rusak
	TxnDebitOnUs   nullAbleInteger `json:"x_txn_debit"`
	TxnDebitOffUs  nullAbleInteger `json:"x_txn_debit_off_us"`
	TxnCreditOnUs  nullAbleInteger `json:"x_txn_kredit"`
	TxnCreditOffUs nullAbleInteger `json:"x_txn_jcb"`
	TxnPrepaid     nullAbleInteger `json:"x_txn_prepaid"`
	TxnContactless nullAbleInteger `json:"x_txn_jcb_contactless"`
	TxnQR          nullAbleInteger `json:"x_txn_qr"`
	TxnPushToPay   nullAbleInteger `json:"x_txn_ptp"`
	KondisiEDC     nullAbleString  `json:"x_condition_edc"`

	// Problem EDC
	ProblemEDC nullAbleString `json:"x_problem_edc"`
}

func (t *OdooTaskDataRequestItem) UnmarshalJSON(data []byte) error {
	type Alias OdooTaskDataRequestItem
	aux := &struct {
		SlaDeadline         interface{} `json:"x_sla_deadline"`
		CreateDate          interface{} `json:"create_date"`
		ReceivedDatetimeSpk interface{} `json:"x_received_datetime_spk"`
		PlanDate            interface{} `json:"planned_date_begin"`
		TimesheetLastStop   interface{} `json:"timesheet_timer_last_stop"`
		DateLastStageUpdate interface{} `json:"date_last_stage_update"`

		*Alias
	}{
		Alias: (*Alias)(t),
	}

	if err := json.Unmarshal(data, &aux); err != nil {
		return fmt.Errorf("failed to unmarshal JSON: %v", err)
	}

	parseTimeField := func(value interface{}, add7Hours bool) (nullAbleTime, error) {
		switch v := value.(type) {
		case string:
			if v == "" || v == "null" {
				return nullAbleTime{Time: time.Time{}, Valid: false}, nil
			}
			parsedTime, err := time.Parse("2006-01-02 15:04:05", v)
			if err != nil {
				return nullAbleTime{}, fmt.Errorf("failed to parse time: %v", err)
			}
			if add7Hours {
				parsedTime = parsedTime.Add(7 * time.Hour) // Add 7 hours if flag is true
			}
			return nullAbleTime{Time: parsedTime, Valid: true}, nil
		case bool:
			if !v {
				return nullAbleTime{Time: time.Time{}, Valid: false}, nil
			}
			return nullAbleTime{}, errors.New("unexpected boolean value: true")
		case nil:
			return nullAbleTime{Time: time.Time{}, Valid: false}, nil
		default:
			return nullAbleTime{}, fmt.Errorf("unexpected type: %T", value)
		}
	}

	var err error

	if t.PlanDate, err = parseTimeField(aux.PlanDate, false); err != nil {
		return fmt.Errorf("PlanDate: %v", err)
	}

	if t.SlaDeadline, err = parseTimeField(aux.SlaDeadline, false); err != nil {
		return fmt.Errorf("SlaDeadline: %v", err)
	}

	if t.CreateDate, err = parseTimeField(aux.CreateDate, false); err != nil {
		return fmt.Errorf("CreateDate: %v", err)
	}

	if t.ReceivedDatetimeSpk, err = parseTimeField(aux.ReceivedDatetimeSpk, false); err != nil {
		return fmt.Errorf("ReceivedDatetimeSpk: %v", err)
	}

	if t.TimesheetLastStop, err = parseTimeField(aux.TimesheetLastStop, false); err != nil {
		return fmt.Errorf("TimesheetLastStop: %v", err)
	}

	// Only DateLastStageUpdate gets +7 hours if valid
	if t.DateLastStageUpdate, err = parseTimeField(aux.DateLastStageUpdate, true); err != nil {
		return fmt.Errorf("DateLastStageUpdate: %v", err)
	}

	return nil
}

// Helpdesk.ticket
type OdooTicketDataRequestItem struct {
	ID                  uint              `json:"id"`
	TicketSubject       nullAbleString    `json:"name"`
	MerchantName        nullAbleString    `json:"x_merchant"`
	PicMerchant         nullAbleString    `json:"x_merchant_pic"`
	PicPhone            nullAbleString    `json:"x_merchant_pic_phone"`
	MerchantAddress     nullAbleString    `json:"x_studio_alamat"`
	MerchantCity        nullAbleString    `json:"x_studio_kota"`
	MerchantEmail       nullAbleString    `json:"partner_email"`
	MerchantZipCode     nullAbleString    `json:"x_merchant_zipcode"`
	ContactPerson       nullAbleString    `json:"contact_person"`
	ContactPhone        nullAbleString    `json:"x_merchant_phone"`
	Description         nullAbleString    `json:"description"`
	Priority            nullAbleString    `json:"priority"`
	Mid                 nullAbleString    `json:"x_master_mid"`
	Tid                 nullAbleString    `json:"x_master_tid"`
	Source              nullAbleString    `json:"x_source"`
	JobId               nullAbleString    `json:"x_job_id"`
	WOFirst             nullAbleString    `json:"x_wo_number"`
	WoNumberLast        nullAbleString    `json:"x_wo_number_last"`
	StatusMerchant      nullAbleString    `json:"x_status_merchant"` // Kondisi Merchant
	WoRemarkTiket       nullAbleString    `json:"x_wo_remark"`
	ReasonCode          nullAbleString    `json:"x_reasoncode"`
	Reason              nullAbleString    `json:"x_reason"`
	TaskType            nullAbleString    `json:"x_task_type"`
	LinkWO              nullAbleString    `json:"x_link"`
	KondisiEDC          nullAbleString    `json:"x_condition_edc"`
	StatusEDC           nullAbleString    `json:"x_status_edc"`
	WorksheetTemplateId nullAbleInterface `json:"x_worksheet_template_id"`
	TicketTypeId        nullAbleInterface `json:"ticket_type_id"`
	CompanyId           nullAbleInterface `json:"company_id"`
	StageId             nullAbleInterface `json:"stage_id"`
	SnEdc               nullAbleInterface `json:"x_merchant_sn_edc"`
	EdcType             nullAbleInterface `json:"x_merchant_tipe_edc"`
	TechnicianId        nullAbleInterface `json:"technician_id"`
	ProjectId           nullAbleInterface `json:"project_id"`
	TaskId              nullAbleInterface `json:"fsm_task_ids"`
	Latitude            nullAbleFloat     `json:"x_partner_latitude"`
	Longitude           nullAbleFloat     `json:"x_partner_longitude"`
	TaskCount           nullAbleInteger   `json:"fsm_task_count"`
	SlaDeadline         nullAbleTime      `json:"x_sla_deadline"`
	CreateDate          nullAbleTime      `json:"create_date"`
	ReceivedDatetimeSpk nullAbleTime      `json:"x_received_datetime_spk"`
	CompleteDatetimeWo  nullAbleTime      `json:"complete_datetime_wo"`
	Paid                nullAbleBoolean   `json:"x_paid"`
}

func (t *OdooTicketDataRequestItem) UnmarshalJSON(data []byte) error {
	type Alias OdooTicketDataRequestItem
	aux := &struct {
		SlaDeadline         interface{} `json:"x_sla_deadline"`
		CreateDate          interface{} `json:"create_date"`
		ReceivedDatetimeSpk interface{} `json:"x_received_datetime_spk"`
		CompleteDatetimeWo  interface{} `json:"complete_datetime_wo"`

		*Alias
	}{
		Alias: (*Alias)(t),
	}

	if err := json.Unmarshal(data, &aux); err != nil {
		return fmt.Errorf("failed to unmarshal JSON: %v", err)
	}

	parseTimeField := func(value interface{}, add7Hours bool) (nullAbleTime, error) {
		switch v := value.(type) {
		case string:
			if v == "" || v == "null" {
				return nullAbleTime{Time: time.Time{}, Valid: false}, nil
			}
			parsedTime, err := time.Parse("2006-01-02 15:04:05", v)
			if err != nil {
				return nullAbleTime{}, fmt.Errorf("failed to parse time: %v", err)
			}
			if add7Hours {
				parsedTime = parsedTime.Add(7 * time.Hour) // Add 7 hours if flag is true
			}
			return nullAbleTime{Time: parsedTime, Valid: true}, nil
		case bool:
			if !v {
				return nullAbleTime{Time: time.Time{}, Valid: false}, nil
			}
			return nullAbleTime{}, errors.New("unexpected boolean value: true")
		case nil:
			return nullAbleTime{Time: time.Time{}, Valid: false}, nil
		default:
			return nullAbleTime{}, fmt.Errorf("unexpected type: %T", value)
		}
	}

	var err error

	if t.SlaDeadline, err = parseTimeField(aux.SlaDeadline, false); err != nil {
		return fmt.Errorf("SlaDeadline: %v", err)
	}

	if t.CreateDate, err = parseTimeField(aux.CreateDate, false); err != nil {
		return fmt.Errorf("CreateDate: %v", err)
	}

	if t.ReceivedDatetimeSpk, err = parseTimeField(aux.ReceivedDatetimeSpk, false); err != nil {
		return fmt.Errorf("ReceivedDatetimeSpk: %v", err)
	}

	if t.CompleteDatetimeWo, err = parseTimeField(aux.CompleteDatetimeWo, false); err != nil {
		return fmt.Errorf("CompleteDatetimeWo: %v", err)
	}

	return nil
}

// OdooIRModelFields represents a field definition from the Odoo ir.model.fields model.
// It includes the field's ID, name, label, associated model, and type.
type OdooIRModelFields struct {
	ID         uint              `json:"id"`
	FieldName  string            `json:"name"`
	FieldLabel string            `json:"field_description"`
	ModelId    nullAbleInterface `json:"model_id"`
	FieldType  string            `json:"ttype"`
}

// Technician.Login
type OdooTechnicianLoginItem struct {
	ID           uint              `json:"id"`
	ImeiDevice   nullAbleString    `json:"imei_device"`
	DisplayName  nullAbleString    `json:"display_name"`
	TechnicianId nullAbleInterface `json:"technician_id"`
	CreateUid    nullAbleInterface `json:"create_uid"`
	WriteUid     nullAbleInterface `json:"write_uid"`
	LoginTime    nullAbleTime      `json:"login_time"`
	CreateDate   nullAbleTime      `json:"create_date"`
	WriteDate    nullAbleTime      `json:"write_date"`
	LastUpdate   nullAbleTime      `json:"__last_update"`
}

func (l *OdooTechnicianLoginItem) UnmarshalJSON(data []byte) error {
	type Alias OdooTechnicianLoginItem
	aux := &struct {
		LoginTime  interface{} `json:"login_time"`
		CreateDate interface{} `json:"create_date"`
		WriteDate  interface{} `json:"write_date"`
		LastUpdate interface{} `json:"__last_update"`

		*Alias
	}{
		Alias: (*Alias)(l),
	}

	if err := json.Unmarshal(data, &aux); err != nil {
		return fmt.Errorf("failed to unmarshal JSON: %v", err)
	}

	parseTimeField := func(value interface{}, add7Hours bool) (nullAbleTime, error) {
		switch v := value.(type) {
		case string:
			if v == "" || v == "null" {
				return nullAbleTime{Time: time.Time{}, Valid: false}, nil
			}
			// Parse time in Jakarta timezone to avoid UTC conversion issues
			loc, _ := time.LoadLocation(config.GetConfig().Default.Timezone)
			parsedTime, err := time.ParseInLocation("02-01-2006 15:04:05", v, loc)
			if err != nil {
				return nullAbleTime{}, fmt.Errorf("failed to parse time: %v", err)
			}
			if add7Hours {
				parsedTime = parsedTime.Add(7 * time.Hour) // Add 7 hours if flag is true
			}
			return nullAbleTime{Time: parsedTime, Valid: true}, nil
		case bool:
			if !v {
				return nullAbleTime{Time: time.Time{}, Valid: false}, nil
			}
			return nullAbleTime{}, errors.New("unexpected boolean value: true")
		case nil:
			return nullAbleTime{Time: time.Time{}, Valid: false}, nil
		default:
			return nullAbleTime{}, fmt.Errorf("unexpected type: %T", value)
		}
	}

	var err error

	if l.LoginTime, err = parseTimeField(aux.LoginTime, false); err != nil {
		return fmt.Errorf("LoginTime: %v", err)
	}

	if l.CreateDate, err = parseTimeField(aux.CreateDate, false); err != nil {
		return fmt.Errorf("CreateDate: %v", err)
	}

	if l.WriteDate, err = parseTimeField(aux.WriteDate, false); err != nil {
		return fmt.Errorf("WriteDate: %v", err)
	}

	if l.LastUpdate, err = parseTimeField(aux.LastUpdate, false); err != nil {
		return fmt.Errorf("LastUpdate: %v", err)
	}
	return nil
}

// Technician.Download
type OdooTechnicianDownloadItem struct {
	ID             uint              `json:"id"`
	DisplayName    nullAbleString    `json:"display_name"`
	DownloadAmount nullAbleString    `json:"download_amount"`
	TechnicianId   nullAbleInterface `json:"technician_id"`
	CreateUid      nullAbleInterface `json:"create_uid"`
	WriteUid       nullAbleInterface `json:"write_uid"`
	DownloadTime   nullAbleTime      `json:"download_time"`
	CreateDate     nullAbleTime      `json:"create_date"`
	WriteDate      nullAbleTime      `json:"write_date"`
	LastUpdate     nullAbleTime      `json:"__last_update"`
}

func (d *OdooTechnicianDownloadItem) UnmarshalJSON(data []byte) error {
	type Alias OdooTechnicianDownloadItem
	aux := &struct {
		DownloadTime interface{} `json:"download_time"`
		CreateDate   interface{} `json:"create_date"`
		WriteDate    interface{} `json:"write_date"`
		LastUpdate   interface{} `json:"__last_update"`

		*Alias
	}{
		Alias: (*Alias)(d),
	}

	if err := json.Unmarshal(data, &aux); err != nil {
		return fmt.Errorf("failed to unmarshal JSON: %v", err)
	}

	parseTimeField := func(value interface{}, add7Hours bool) (nullAbleTime, error) {
		switch v := value.(type) {
		case string:
			if v == "" || v == "null" {
				return nullAbleTime{Time: time.Time{}, Valid: false}, nil
			}
			// Parse time in Jakarta timezone to avoid UTC conversion issues for download time
			loc, _ := time.LoadLocation(config.GetConfig().Default.Timezone)
			parsedTime, err := time.ParseInLocation("02-01-2006 15:04:05", v, loc)
			if err != nil {
				return nullAbleTime{}, fmt.Errorf("failed to parse time: %v", err)
			}
			if add7Hours {
				parsedTime = parsedTime.Add(7 * time.Hour) // Add 7 hours if flag is true
			}
			return nullAbleTime{Time: parsedTime, Valid: true}, nil
		case bool:
			if !v {
				return nullAbleTime{Time: time.Time{}, Valid: false}, nil
			}
			return nullAbleTime{}, errors.New("unexpected boolean value: true")
		case nil:
			return nullAbleTime{Time: time.Time{}, Valid: false}, nil
		default:
			return nullAbleTime{}, fmt.Errorf("unexpected type: %T", value)
		}
	}

	var err error

	if d.DownloadTime, err = parseTimeField(aux.DownloadTime, false); err != nil {
		return fmt.Errorf("DownloadTime: %v", err)
	}

	if d.CreateDate, err = parseTimeField(aux.CreateDate, false); err != nil {
		return fmt.Errorf("CreateDate: %v", err)
	}

	if d.WriteDate, err = parseTimeField(aux.WriteDate, false); err != nil {
		return fmt.Errorf("WriteDate: %v", err)
	}

	if d.LastUpdate, err = parseTimeField(aux.LastUpdate, false); err != nil {
		return fmt.Errorf("LastUpdate: %v", err)
	}
	return nil
}

// Job.Group
type JobGroupsItem struct {
	ID          uint            `json:"id"`
	Name        nullAbleString  `json:"name"`
	BasicSalary nullAbleInteger `json:"basic_salary"`
	TaskMax     nullAbleInteger `json:"task_max"`
	Insentive   nullAbleInteger `json:"insentive"`
}

// Res.Partner
type OdooResPartnerItem struct {
	ID                     uint              `json:"id"`
	MIDTID                 nullAbleString    `json:"name"`
	MerchantName           nullAbleString    `json:"x_merchant"`
	MerchantCode           nullAbleString    `json:"x_merchant_code"`
	MerchantGroupCode      nullAbleString    `json:"x_merchant_group_code"`
	MerchantGroupName      nullAbleString    `json:"x_merchant_group_name"`
	MerchantPIC            nullAbleString    `json:"x_merchant_pic"`
	MerchantPICPhoneNumber nullAbleString    `json:"x_merchant_pic_phone"`
	AlamatPengirimanEDC    nullAbleString    `json:"x_alamat_pengiriman_edc"`
	ContactPerson          nullAbleString    `json:"x_contact_person"`
	AlamatPerusahaan       nullAbleString    `json:"street"`
	PhoneNumber            nullAbleString    `json:"phone"`
	MobilePhone            nullAbleString    `json:"mobile"`
	Email                  nullAbleString    `json:"email"`
	Keterangan             nullAbleString    `json:"x_description"`
	MSISDNSimCard          nullAbleString    `json:"x_msisdn_sim_card"`
	ICCID                  nullAbleString    `json:"iccid_simcard"`
	Fitur                  nullAbleString    `json:"x_fitur"`
	ServicePoint           nullAbleString    `json:"x_service_point"`
	MerchantLastStatus     nullAbleString    `json:"merchant_last_status"`
	Source                 nullAbleString    `json:"x_source"`
	Notes                  nullAbleString    `json:"comment"`
	Technician             nullAbleInterface `json:"technician_id"`
	ZoneGroup              nullAbleInterface `json:"zone_group_id"`
	SNEDC                  nullAbleInterface `json:"x_studio_sn_edc"`
	TipeEDC                nullAbleInterface `json:"x_product"`
	SimCard                nullAbleInterface `json:"x_simcard"`
	SimCardProvider        nullAbleInterface `json:"x_simcard_provider"`
	EmployeeID             nullAbleInterface `json:"x_employee_id"`
	SamCard                nullAbleInterface `json:"x_samcard"`
	GeoLatitude            nullAbleFloat     `json:"partner_latitude"`
	GeoLongitude           nullAbleFloat     `json:"partner_longitude"`
	Active                 nullAbleBoolean   `json:"active"`
	AutoPM                 nullAbleBoolean   `json:"x_auto_pm"`

	TIDMaster    nullAbleString `json:"x_master_tid"`
	TIDMasterOld nullAbleString `json:"x_cimb_master_tid"`
	TIDRegular   nullAbleString `json:"x_cimb_tid"`
	TIDBank      nullAbleString `json:"x_cimb_tid2"`
	TID3         nullAbleString `json:"x_cimb_tid3"`
	TID4         nullAbleString `json:"x_cimb_tid4"`
	TID5         nullAbleString `json:"x_cimb_tid5"`
	TID6         nullAbleString `json:"x_cimb_tid6"`
	TID7         nullAbleString `json:"x_cimb_tid7"`
	TID8         nullAbleString `json:"x_cimb_tid8"`
	TIDQR        nullAbleString `json:"x_cimb_tidqr"`

	MIDMaster    nullAbleString `json:"x_master_mid"`
	MIDMasterOld nullAbleString `json:"x_cimb_master_mid"`
	MIDRegular   nullAbleString `json:"x_cimb_mid"`
	MIDBank      nullAbleString `json:"x_cimb_mid2"`
	MID3         nullAbleString `json:"x_cimb_mid3"`
	MID4         nullAbleString `json:"x_cimb_mid4"`
	MID5         nullAbleString `json:"x_cimb_mid5"`
	MID6         nullAbleString `json:"x_cimb_mid6"`
	MID7         nullAbleString `json:"x_cimb_mid7"`
	MID8         nullAbleString `json:"x_cimb_mid8"`
	MIDQR        nullAbleString `json:"x_cimb_midqr"`
}

// Res.Company
type ODOOMSCompanyItem struct {
	ID         uint              `json:"id"`
	Name       nullAbleString    `json:"name"`
	WriteDate  nullAbleTime      `json:"write_date"`
	CreateDate nullAbleTime      `json:"create_date"`
	PartnerId  nullAbleInterface `json:"partner_id"`
	WriteUid   nullAbleInterface `json:"write_uid"`
	CreateUid  nullAbleInterface `json:"create_uid"`
}

// Helpdesk.Ticket.Type
type OdooHelpdeskTicketTypeItem struct {
	ID                       uint                 `json:"id"`
	Sequence                 nullAbleInteger      `json:"sequence"`
	Type                     nullAbleString       `json:"name"`
	TaskType                 nullAbleString       `json:"x_task_type"`
	WorksheetTemplateId      nullAbleInterface    `json:"x_worksheet_template_id"`
	DirectlyPlanIntervantion nullAbleBoolean      `json:"x_plan_intervention"`
	MultiCompany             nullAbleArrayInteger `json:"x_studio_multi_company"`
}

// FS.Params
type ODOOMSFSParams struct {
	ID             uint              `json:"id"`
	Active         nullAbleBoolean   `json:"active"`
	CreatedBy      nullAbleInterface `json:"create_uid"`
	LastUpdatedBy  nullAbleInterface `json:"write_uid"`
	CreatedOn      nullAbleTime      `json:"create_date"`
	LastModifiedOn nullAbleTime      `json:"_last_update"`
	LastUpdatedOn  nullAbleTime      `json:"write_date"`
	DisplayName    nullAbleString    `json:"display_name"`
	Logs           nullAbleString    `json:"logs"`
	Name           nullAbleString    `json:"name"`
	Value          nullAbleString    `json:"value"`
}

func (d *ODOOMSFSParams) UnmarshalJSON(data []byte) error {
	type Alias ODOOMSFSParams
	aux := &struct {
		CreatedOn      interface{} `json:"create_date"`
		LastModifiedOn interface{} `json:"_last_update"`
		LastUpdatedOn  interface{} `json:"write_date"`

		*Alias
	}{
		Alias: (*Alias)(d),
	}

	if err := json.Unmarshal(data, &aux); err != nil {
		return fmt.Errorf("failed to unmarshal JSON: %v", err)
	}

	parseTimeField := func(value interface{}, add7Hours bool) (nullAbleTime, error) {
		switch v := value.(type) {
		case string:
			if v == "" || v == "null" {
				return nullAbleTime{Time: time.Time{}, Valid: false}, nil
			}
			// Parse time in Jakarta timezone to avoid UTC conversion issues for download time
			loc, _ := time.LoadLocation(config.GetConfig().Default.Timezone)
			parsedTime, err := time.ParseInLocation("2006-01-02 15:04:05", v, loc)
			if err != nil {
				return nullAbleTime{}, fmt.Errorf("failed to parse time: %v", err)
			}
			if add7Hours {
				parsedTime = parsedTime.Add(7 * time.Hour) // Add 7 hours if flag is true
			}
			return nullAbleTime{Time: parsedTime, Valid: true}, nil
		case bool:
			if !v {
				return nullAbleTime{Time: time.Time{}, Valid: false}, nil
			}
			return nullAbleTime{}, errors.New("unexpected boolean value: true")
		case nil:
			return nullAbleTime{Time: time.Time{}, Valid: false}, nil
		default:
			return nullAbleTime{}, fmt.Errorf("unexpected type: %T", value)
		}
	}

	var err error

	if d.CreatedOn, err = parseTimeField(aux.CreatedOn, false); err != nil {
		return fmt.Errorf("CreatedOn: %v", err)
	}

	if d.LastModifiedOn, err = parseTimeField(aux.LastModifiedOn, false); err != nil {
		return fmt.Errorf("LastModifiedOn: %v", err)
	}

	if d.LastUpdatedOn, err = parseTimeField(aux.LastUpdatedOn, false); err != nil {
		return fmt.Errorf("LastUpdatedOn: %v", err)
	}
	return nil
}

// FS.Param.Payment
type ODOOMSFSParamPayment struct {
	ID          uint              `json:"id"`
	Active      nullAbleBoolean   `json:"active"`
	Name        nullAbleString    `json:"name"`
	DisplayName nullAbleString    `json:"display_name"`
	ParamType   nullAbleString    `json:"param_type"`
	Price       nullAbleInteger   `json:"price"`
	CreateDate  nullAbleTime      `json:"create_date"`
	WriteDate   nullAbleTime      `json:"write_date"`
	LastUpdate  nullAbleTime      `json:"__last_update"`
	CompanyId   nullAbleInterface `json:"company_id"`
	CreateUid   nullAbleInterface `json:"create_uid"`
	WriteUid    nullAbleInterface `json:"write_uid"`
}

func (d *ODOOMSFSParamPayment) UnmarshalJSON(data []byte) error {
	type Alias ODOOMSFSParamPayment
	aux := &struct {
		CreateDate interface{} `json:"create_date"`
		WriteDate  interface{} `json:"write_date"`
		LastUpdate interface{} `json:"__last_update"`

		*Alias
	}{
		Alias: (*Alias)(d),
	}

	if err := json.Unmarshal(data, &aux); err != nil {
		return fmt.Errorf("failed to unmarshal JSON: %v", err)
	}

	parseTimeField := func(value interface{}, add7Hours bool) (nullAbleTime, error) {
		switch v := value.(type) {
		case string:
			if v == "" || v == "null" {
				return nullAbleTime{Time: time.Time{}, Valid: false}, nil
			}
			// Parse time in Jakarta timezone to avoid UTC conversion issues for download time
			loc, _ := time.LoadLocation(config.GetConfig().Default.Timezone)
			parsedTime, err := time.ParseInLocation("2006-01-02 15:04:05", v, loc)
			if err != nil {
				return nullAbleTime{}, fmt.Errorf("failed to parse time: %v", err)
			}
			if add7Hours {
				parsedTime = parsedTime.Add(7 * time.Hour) // Add 7 hours if flag is true
			}
			return nullAbleTime{Time: parsedTime, Valid: true}, nil
		case bool:
			if !v {
				return nullAbleTime{Time: time.Time{}, Valid: false}, nil
			}
			return nullAbleTime{}, errors.New("unexpected boolean value: true")
		case nil:
			return nullAbleTime{Time: time.Time{}, Valid: false}, nil
		default:
			return nullAbleTime{}, fmt.Errorf("unexpected type: %T", value)
		}
	}

	var err error

	if d.CreateDate, err = parseTimeField(aux.CreateDate, false); err != nil {
		return fmt.Errorf("CreateDate: %v", err)
	}

	if d.WriteDate, err = parseTimeField(aux.WriteDate, false); err != nil {
		return fmt.Errorf("WriteDate: %v", err)
	}

	if d.LastUpdate, err = parseTimeField(aux.LastUpdate, false); err != nil {
		return fmt.Errorf("LastUpdate: %v", err)
	}
	return nil
}

// Product.Template
type ODOOMSProductTemplate struct {
	ID                                   uint              `json:"id"`
	Active                               nullAbleBoolean   `json:"active"`
	ActivityDateDeadline                 nullAbleTime      `json:"activity_date_deadline"`
	ActivityExceptionDecoration          nullAbleString    `json:"activity_exception_decoration"`
	ActivityExceptionIcon                nullAbleString    `json:"activity_exception_icon"`
	ActivityIds                          nullAbleInterface `json:"activity_ids"`
	ActivityState                        nullAbleString    `json:"activity_state"`
	ActivitySummary                      nullAbleString    `json:"activity_summary"`
	ActivityTypeId                       nullAbleInterface `json:"activity_type_id"`
	ActivityUserId                       nullAbleInterface `json:"activity_user_id"`
	AttributeLineIds                     nullAbleInterface `json:"attribute_line_ids"`
	Barcode                              nullAbleString    `json:"barcode"`
	CanImage1024BeZoomed                 nullAbleBoolean   `json:"can_image_1024_be_zoomed"`
	CategId                              nullAbleInterface `json:"categ_id"`
	Color                                nullAbleInteger   `json:"color"`
	CompanyId                            nullAbleInterface `json:"company_id"`
	CostCurrencyId                       nullAbleInterface `json:"cost_currency_id"`
	CostMethod                           nullAbleString    `json:"cost_method"`
	CreateDate                           nullAbleTime      `json:"create_date"`
	CreateUid                            nullAbleInterface `json:"create_uid"`
	CurrencyId                           nullAbleInterface `json:"currency_id"`
	DefaultCode                          nullAbleString    `json:"default_code"`
	Description                          nullAbleString    `json:"description"`
	DescriptionPicking                   nullAbleString    `json:"description_picking"`
	DescriptionPickingin                 nullAbleString    `json:"description_pickingin"`
	DescriptionPickingout                nullAbleString    `json:"description_pickingout"`
	DescriptionPurchase                  nullAbleString    `json:"description_purchase"`
	DescriptionSale                      nullAbleString    `json:"description_sale"`
	DisplayName                          nullAbleString    `json:"display_name"`
	ExpensePolicy                        nullAbleString    `json:"expense_policy"`
	HasConfigurableAttributes            nullAbleBoolean   `json:"has_configurable_attributes"`
	Image1024                            nullAbleString    `json:"image_1024"`
	Image128                             nullAbleString    `json:"image_128"`
	Image1920                            nullAbleString    `json:"image_1920"`
	Image256                             nullAbleString    `json:"image_256"`
	Image512                             nullAbleString    `json:"image_512"`
	IncomingQty                          nullAbleFloat     `json:"incoming_qty"`
	InvoicePolicy                        nullAbleString    `json:"invoice_policy"`
	IsProductVariant                     nullAbleBoolean   `json:"is_product_variant"`
	LastUpdate                           nullAbleTime      `json:"__last_update"`
	ListPrice                            nullAbleFloat     `json:"list_price"`
	LocationId                           nullAbleInterface `json:"location_id"`
	LstPrice                             nullAbleFloat     `json:"lst_price"`
	MessageAttachmentCount               nullAbleInteger   `json:"message_attachment_count"`
	MessageChannelIds                    nullAbleInterface `json:"message_channel_ids"`
	MessageFollowerIds                   nullAbleInterface `json:"message_follower_ids"`
	MessageHasError                      nullAbleBoolean   `json:"message_has_error"`
	MessageHasErrorCounter               nullAbleInteger   `json:"message_has_error_counter"`
	MessageHasSmsError                   nullAbleBoolean   `json:"message_has_sms_error"`
	MessageIds                           nullAbleInterface `json:"message_ids"`
	MessageIsFollower                    nullAbleBoolean   `json:"message_is_follower"`
	MessageMainAttachmentId              nullAbleInterface `json:"message_main_attachment_id"`
	MessageNeedaction                    nullAbleBoolean   `json:"message_needaction"`
	MessageNeedactionCounter             nullAbleInteger   `json:"message_needaction_counter"`
	MessagePartnerIds                    nullAbleInterface `json:"message_partner_ids"`
	MessageUnread                        nullAbleBoolean   `json:"message_unread"`
	MessageUnreadCounter                 nullAbleInteger   `json:"message_unread_counter"`
	Name                                 nullAbleString    `json:"name"`
	NbrReorderingRules                   nullAbleInteger   `json:"nbr_reordering_rules"`
	OutgoingQty                          nullAbleFloat     `json:"outgoing_qty"`
	PackagingIds                         nullAbleInterface `json:"packaging_ids"`
	Price                                nullAbleFloat     `json:"price"`
	PricelistId                          nullAbleInterface `json:"pricelist_id"`
	PricelistItemCount                   nullAbleInteger   `json:"pricelist_item_count"`
	ProductVariantCount                  nullAbleInteger   `json:"product_variant_count"`
	ProductVariantId                     nullAbleInterface `json:"product_variant_id"`
	ProductVariantIds                    nullAbleInterface `json:"product_variant_ids"`
	ProjectId                            nullAbleInterface `json:"project_id"`
	ProjectTemplateId                    nullAbleInterface `json:"project_template_id"`
	PropertyAccountExpenseId             nullAbleInterface `json:"property_account_expense_id"`
	PropertyAccountIncomeId              nullAbleInterface `json:"property_account_income_id"`
	PropertyStockInventory               nullAbleInterface `json:"property_stock_inventory"`
	PropertyStockProduction              nullAbleInterface `json:"property_stock_production"`
	PurchaseOk                           nullAbleBoolean   `json:"purchase_ok"`
	QtyAvailable                         nullAbleFloat     `json:"qty_available"`
	Rental                               nullAbleBoolean   `json:"rental"`
	ReorderingMaxQty                     nullAbleFloat     `json:"reordering_max_qty"`
	ReorderingMinQty                     nullAbleFloat     `json:"reordering_min_qty"`
	ResponsibleId                        nullAbleInterface `json:"responsible_id"`
	RouteFromCategIds                    nullAbleInterface `json:"route_from_categ_ids"`
	RouteIds                             nullAbleInterface `json:"route_ids"`
	SaleDelay                            nullAbleFloat     `json:"sale_delay"`
	SaleLineWarn                         nullAbleString    `json:"sale_line_warn"`
	SaleLineWarnMsg                      nullAbleString    `json:"sale_line_warn_msg"`
	SaleOk                               nullAbleBoolean   `json:"sale_ok"`
	SalesCount                           nullAbleFloat     `json:"sales_count"`
	SellerIds                            nullAbleInterface `json:"seller_ids"`
	Sequence                             nullAbleInteger   `json:"sequence"`
	ServicePolicy                        nullAbleString    `json:"service_policy"`
	ServiceTracking                      nullAbleString    `json:"service_tracking"`
	ServiceType                          nullAbleString    `json:"service_type"`
	StandardPrice                        nullAbleFloat     `json:"standard_price"`
	SupplierTaxesId                      nullAbleInterface `json:"supplier_taxes_id"`
	TaxesId                              nullAbleInterface `json:"taxes_id"`
	Tracking                             nullAbleString    `json:"tracking"`
	Type                                 nullAbleString    `json:"type"`
	UomId                                nullAbleInterface `json:"uom_id"`
	UomName                              nullAbleString    `json:"uom_name"`
	UomPoId                              nullAbleInterface `json:"uom_po_id"`
	ValidProductTemplateAttributeLineIds nullAbleInterface `json:"valid_product_template_attribute_line_ids"`
	Valuation                            nullAbleString    `json:"valuation"`
	VariantSellerIds                     nullAbleInterface `json:"variant_seller_ids"`
	VirtualAvailable                     nullAbleFloat     `json:"virtual_available"`
	VisibleExpensePolicy                 nullAbleBoolean   `json:"visible_expense_policy"`
	Volume                               nullAbleFloat     `json:"volume"`
	VolumeUomName                        nullAbleString    `json:"volume_uom_name"`
	WarehouseId                          nullAbleInterface `json:"warehouse_id"`
	WebsiteMessageIds                    nullAbleInterface `json:"website_message_ids"`
	Weight                               nullAbleFloat     `json:"weight"`
	WeightUomName                        nullAbleString    `json:"weight_uom_name"`
	WorksheetTemplateId                  nullAbleInterface `json:"worksheet_template_id"`
	WriteDate                            nullAbleTime      `json:"write_date"`
	WriteUid                             nullAbleInterface `json:"write_uid"`
	XEdcLocation                         nullAbleInterface `json:"x_edc_location"`
	XMerek                               nullAbleString    `json:"x_merek"`
	XStokPindah                          nullAbleInterface `json:"x_stok_pindah"`
}

func (p *ODOOMSProductTemplate) UnmarshalJSON(data []byte) error {
	type Alias ODOOMSProductTemplate
	aux := &struct {
		ActivityDateDeadline interface{} `json:"activity_date_deadline"`
		CreateDate           interface{} `json:"create_date"`
		LastUpdate           interface{} `json:"__last_update"`
		WriteDate            interface{} `json:"write_date"`

		*Alias
	}{
		Alias: (*Alias)(p),
	}

	if err := json.Unmarshal(data, &aux); err != nil {
		return fmt.Errorf("failed to unmarshal JSON: %v", err)
	}

	parseTimeField := func(value interface{}, add7Hours bool) (nullAbleTime, error) {
		switch v := value.(type) {
		case string:
			if v == "" || v == "null" {
				return nullAbleTime{Time: time.Time{}, Valid: false}, nil
			}
			// Parse time in Jakarta timezone to avoid UTC conversion issues for
			loc, _ := time.LoadLocation(config.GetConfig().Default.Timezone)
			parsedTime, err := time.ParseInLocation("2006-01-02 15:04:05", v, loc)
			if err != nil {
				return nullAbleTime{}, fmt.Errorf("failed to parse time: %v", err)
			}
			if add7Hours {
				parsedTime = parsedTime.Add(7 * time.Hour) // Add 7 hours if flag is true
			}
			return nullAbleTime{Time: parsedTime, Valid: true}, nil
		case bool:
			if !v {
				return nullAbleTime{Time: time.Time{}, Valid: false}, nil
			}
			return nullAbleTime{}, errors.New("unexpected boolean value: true")
		case nil:
			return nullAbleTime{Time: time.Time{}, Valid: false}, nil
		default:
			return nullAbleTime{}, fmt.Errorf("unexpected type: %T", value)
		}
	}

	var err error
	if p.ActivityDateDeadline, err = parseTimeField(aux.ActivityDateDeadline, false); err != nil {
		return fmt.Errorf("ActivityDateDeadline: %v", err)
	}

	if p.CreateDate, err = parseTimeField(aux.CreateDate, false); err != nil {
		return fmt.Errorf("CreateDate: %v", err)
	}

	if p.LastUpdate, err = parseTimeField(aux.LastUpdate, false); err != nil {
		return fmt.Errorf("LastUpdate: %v", err)
	}

	if p.WriteDate, err = parseTimeField(aux.WriteDate, false); err != nil {
		return fmt.Errorf("WriteDate: %v", err)
	}
	return nil
}

// Stock.Picking
type OdooStockPickingItem struct {
	ID                            uint                 `json:"id"`
	Name                          nullAbleString       `json:"name"`
	Contact                       nullAbleInterface    `json:"partner_id"`
	OperationType                 nullAbleInterface    `json:"picking_type_id"`
	SourceLocation                nullAbleInterface    `json:"location_id"`
	DestionationLocation          nullAbleInterface    `json:"location_dest_id"`
	Company                       nullAbleInterface    `json:"company_id"`
	Responsible                   nullAbleInterface    `json:"user_id"`
	Responsible2                  nullAbleInterface    `json:"technician_id"`
	TechnicianDestinationLocation nullAbleInterface    `json:"technician_id_fs"`
	ScheduledDate                 nullAbleString       `json:"scheduled_date"`
	SourceDocument                nullAbleString       `json:"origin"`
	LocationDestinationCategory   nullAbleString       `json:"location_dest_categ"`
	Carrier                       nullAbleString       `json:"carrier"`
	NomorResi                     nullAbleString       `json:"tracking_number"`
	LinkBast                      nullAbleString       `json:"x_link"`
	Status                        nullAbleString       `json:"state"`
	DetailedOperationAPKIDs       nullAbleArrayInteger `json:"move_line_ids_without_package"`
}

// Helpdesk.Team
type OdooHelpdeskTeamItem struct {
	ID           uint              `json:"id"`
	HelpdeskTeam nullAbleString    `json:"name"`
	Alias        nullAbleInterface `json:"alias_id"`
	Company      nullAbleInterface `json:"company_id"`
}

// Res.Users
type OdooResUsersItem struct {
	ID                   uint              `json:"id"`
	Name                 nullAbleString    `json:"name"`
	Technician           nullAbleInterface `json:"technician_id"`
	Login                nullAbleString    `json:"login"`
	JobGroup             nullAbleInterface `json:"job_group_id"`
	SmartCard            nullAbleBoolean   `json:"x_smartcard"`
	EmployeeCode         nullAbleString    `json:"x_employee_code"`
	LatestAuthentication nullAbleTime      `json:"login_date"`
	EmployeeCount        nullAbleInteger   `json:"employee_count"`
	Phone                nullAbleString    `json:"phone"`
}

// Project.Project
type OdooProjectProjectItem struct {
	ID             uint              `json:"id"`
	Name           nullAbleString    `json:"name"`
	LabelTasks     nullAbleString    `json:"label_tasks"`
	ProjectManager nullAbleInterface `json:"user_id"`
	Visibility     nullAbleString    `json:"privacy_visibility"`
	Company        nullAbleInterface `json:"company_id"`
	WorkingTime    nullAbleInterface `json:"resource_calendar_id"`
}

// Stock.Move.Line
type StockMoveLineItem struct {
	ID        uint           `json:"id"`
	Stage     nullAbleString `json:"state"`
	Reference nullAbleString `json:"reference"`
	Source    nullAbleString `json:"origin"`
	Status    nullAbleString `json:"x_status"`
	// MoveDate     nullAbleTime      `json:"date"`
	MoveDate     nullAbleString    `json:"date"`
	Product      nullAbleInterface `json:"product_id"`
	SN           nullAbleInterface `json:"lot_id"`
	FromLocation nullAbleInterface `json:"location_id"`
	ToLocation   nullAbleInterface `json:"location_dest_id"`
}

// fs.technician.location
type TechnicianLocationItem struct {
	ID           int               `json:"id"`
	CompanyID    nullAbleInterface `json:"company_id"`
	LocationID   nullAbleInterface `json:"location_id"`
	TechnicianID nullAbleInterface `json:"technician_id"`
	DisplayName  nullAbleString    `json:"display_name"`
}

type TechnicianLocationInfo struct {
	CompanyName  string
	LocationName string
}

func GetPhotosOfTaskODOOMS() gin.HandlerFunc {
	return func(c *gin.Context) {
		idTask := c.Param("id")
		if idTask == "" || idTask == "0" {
			c.JSON(http.StatusNotFound, gin.H{"message": "photo not found"})
			return
		}

		table := c.Query("table")
		if table == "" {
			c.JSON(http.StatusNotFound, gin.H{"message": "table not found"})
			return
		}

		var tableName string
		dbWeb := gormdb.Databases.Web

		switch strings.ToLower(table) {
		case "task-mti":
			tableName = config.GetConfig().MTI.TBDataODOOMS
		case "task-bni":
			tableName = config.GetConfig().BNI.TBDataODOOMS
		default:
			c.JSON(http.StatusNotFound, gin.H{"message": "table not found"})
			return
		}

		var joData interface{}
		switch strings.ToLower(table) {
		case "task-mti":
			var dataInDB mtimodel.MTIOdooMSData
			result := dbWeb.Table(tableName).Where("id = ?", idTask).First(&dataInDB)
			if result.Error != nil {
				c.JSON(http.StatusNotFound, gin.H{"message": "photo not found"})
				return
			}
			joData = dataInDB
		case "task-bni":
			var dataInDB bnimodel.BNIOdooMSData
			result := dbWeb.Table(tableName).Where("id = ?", idTask).First(&dataInDB)
			if result.Error != nil {
				c.JSON(http.StatusNotFound, gin.H{"message": "photo not found"})
				return
			}
			joData = dataInDB

		default:
			c.JSON(http.StatusNotFound, gin.H{"message": "photo not found"})
			return
		}

		id_foto := []string{
			"x_foto_bast", "x_foto_ceklis", "x_foto_edc", "x_foto_pic", "x_foto_setting",
			"x_foto_thermal", "x_foto_toko", "x_foto_training", "x_foto_transaksi",
			"x_tanda_tangan_pic", "x_tanda_tangan_teknisi",
			"x_foto_sticker_edc", "x_foto_screen_guard", "x_foto_all_transaction",
			"x_foto_transaksi_bmri", "x_foto_transaksi_bni", "x_foto_transaksi_bri",
			"x_foto_transaksi_btn", "x_foto_transaksi_patch", "x_foto_screen_p2g",
			"x_foto_kontak_stiker_pic",
			"x_foto_selfie_video_call", "x_foto_selfie_teknisi_merchant",
		}

		judul_foto := []string{
			"Foto BAST", "Foto Media Promo", "Foto SN EDC", "Foto PIC Merchant", "Foto Pengaturan",
			"Foto Thermal", "Foto Merchant", "Foto Surat Training", "Foto Transaksi",
			"Tanda Tangan PIC", "Tanda Tangan Teknisi",
			"Foto Stiker EDC", "Foto Screen Gard", "Foto Sales Draft All Memberbank",
			"Foto Sales Draft BMRI", "Foto Sales Draft BNI", "Foto Sales Draft BRI",
			"Foto Sales Draft BTN", "Foto Sales Draft Patch L", "Foto Screen P2G",
			"Foto Kontak Stiker PIC",
			"Foto Selfie Video Call", "Foto Selfie Teknisi dan Merchant",
		}

		var id_task string
		var ticketSubject string
		var woNumber string
		var teknisi string
		var merchant string
		var mid string
		var tid string

		switch strings.ToLower(table) {
		case "task-mti":
			data := joData.(mtimodel.MTIOdooMSData)
			id_task = strconv.Itoa(int(data.ID))
			ticketSubject = data.TicketSubject
			woNumber = data.WONumber
			teknisi = data.Technician
			merchant = data.MerchantName
			mid = data.Mid
			tid = data.Tid
		case "task-bni":
			data := joData.(bnimodel.BNIOdooMSData)
			id_task = strconv.Itoa(int(data.ID))
			ticketSubject = data.TicketSubject
			woNumber = data.WONumber
			teknisi = data.Technician
			merchant = data.MerchantName
			mid = data.Mid
			tid = data.Tid
		default:
			c.JSON(http.StatusNotFound, gin.H{"message": "photo not found"})
			return
		}

		var html strings.Builder
		html.WriteString(`
<!DOCTYPE html>
<html lang="en">
<head>
	<meta charset="UTF-8">
	<meta name="viewport" content="width=device-width, initial-scale=1.0">
	<title>Photo Gallery</title>
	<link href="https://cdn.jsdelivr.net/npm/bootstrap@5.3.0/dist/css/bootstrap.min.css" rel="stylesheet">
	<link href="https://cdn.jsdelivr.net/npm/bootstrap-icons@1.10.5/font/bootstrap-icons.css" rel="stylesheet">
	<style>
		body {
			background-color: #f8f9fa;
		}
		.photo-card {
			transition: transform 0.2s ease-in-out;
		}
		.photo-card:hover {
			transform: scale(1.05);
			box-shadow: 0 4px 20px rgba(0,0,0,0.2);
		}
		.data-label {
			font-weight: 600;
			color: #343a40;
		}
	</style>
</head>
<body>
	<script>
		function openImagePopup(url) {
			const width = 400;
			const height = 700;
			const left = 0;
			const top = 20;

			window.open(
				url,
				'imagePopup',
				'width=' + width + ',height=' + height + ',left=' + left + ',top=' + top + ',resizable=yes,scrollbars=yes'
			);
		}
	</script>

	<div class="container mt-4">
		<!-- <h2 class="text-center mb-4">Photo Gallery for ID Task: ` + id_task + `</h2> -->

		<div class="card mb-4 shadow-sm border-0">
	<div class="card-body">
		<div class="row g-3">
			<div class="col-md-6 d-flex">
				<div class="me-3">
					<i class="bi bi-person-lines-fill fs-3 text-secondary"></i>
				</div>
				<div>
				<div><span class="fw-semibold text-muted">Ticket Subject:</span> ` + ticketSubject + `</div>
				<div><span class="fw-semibold text-muted">WO Number:</span> ` + woNumber + `</div>
				<div><span class="fw-semibold text-muted">Teknisi:</span> ` + teknisi + `</div>
				</div>
			</div>
			<div class="col-md-6 d-flex">
				<div class="me-3">
					<i class="bi bi-shop-window fs-3 text-secondary"></i>
				</div>
				<div>
					<div><span class="fw-semibold text-muted">Merchant:</span> ` + merchant + `</div>
					<div><span class="fw-semibold text-muted">MID:</span> ` + mid + `</div>
					<div><span class="fw-semibold text-muted">TID:</span> ` + tid + `</div>
				</div>
			</div>
		</div>
	</div>
</div>

		<div class="row row-cols-1 row-cols-sm-2 row-cols-md-3 g-4">
`)

		for i, id := range id_foto {
			html.WriteString(fmt.Sprintf(`
			<div class="col">
				<div class="card photo-card h-100 text-center">
					<img src="/here/file/%s@%s" 
						class="card-img-top" 
						alt="%s"
						style="height:250px; object-fit:contain; cursor:pointer;"
						onclick="openImagePopup(this.src);"
						onerror="this.onerror=null; this.src='/assets/self/img/no-img.jpg';">
					<div class="card-body">
						<h5 class="card-title">%s</h5>
					</div>
				</div>
			</div>
	`, id_task, id, judul_foto[i], judul_foto[i]))
		}

		html.WriteString(`
		</div>
		<div class="text-center mt-4">
		<!-- 
			<a href="/" class="btn btn-secondary">
				<i class="bi bi-arrow-left-circle me-1"></i> Back to Home
			</a>
			-->
		</div>
	</div>
	<script src="https://cdn.jsdelivr.net/npm/bootstrap@5.3.0/dist/js/bootstrap.bundle.min.js"></>
</body>
</html>
`)

		c.Header("Content-Type", "text/html")
		c.String(http.StatusOK, html.String())
	}
}
