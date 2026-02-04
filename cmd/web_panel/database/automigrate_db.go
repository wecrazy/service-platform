package database

import (
	"fmt"
	"service-platform/cmd/web_panel/model"
	bnimodel "service-platform/cmd/web_panel/model/bni_model"
	contracttechnicianmodel "service-platform/cmd/web_panel/model/contract_technician_model"
	dkimodel "service-platform/cmd/web_panel/model/dki_model"
	dspmodel "service-platform/cmd/web_panel/model/dsp_model"
	mtimodel "service-platform/cmd/web_panel/model/mti_model"
	odooms "service-platform/cmd/web_panel/model/odoo_ms"
	reportmodel "service-platform/cmd/web_panel/model/report_model"
	sptechnicianmodel "service-platform/cmd/web_panel/model/sp_technician_model"
	stockopnamemodel "service-platform/cmd/web_panel/model/stock_opname_model"
	whatsappmodel "service-platform/cmd/web_panel/model/whatsapp_model"
	"service-platform/internal/config"

	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

func AutoMigrateWeb(db *gorm.DB) {
	// Run migrations in stages to avoid foreign key issues

	// Stage 1: Create base models without foreign key relationships
	if err := db.AutoMigrate(
		&model.Admin{},
		&model.AdminStatus{},
		&model.AdminPasswordChangeLog{},
		&model.Role{},
		&model.RolePrivilege{},
		&model.Feature{},
		&model.LogActivity{},
		&model.Language{},
		&model.BadWord{},
		&model.AppConfig{},

		&model.TicketType{},
		&model.TicketStage{},
		&model.MerchantHommyPayCC{},
		&model.TicketHommyPayCC{},

		&model.WAMessage{},
		&model.WAMessageReply{},
		&model.WAPhoneUser{},

		&model.IncomingEmail{},
		&model.TrustedClient{},

		// KresekBag
		&model.MerchantKresekBag{},

		// Report
		&reportmodel.EngineersProductivityData{},
		&reportmodel.TaskComparedData{},
		&reportmodel.MTIReportPemasangan{},
		&reportmodel.MTIReportPenarikan{},
		&reportmodel.MTIReportVTR{},
		&reportmodel.ReportScheduled{},
		&reportmodel.MonitoringTicketODOOMS{},
		&reportmodel.ODOOMSSLAReport{},

		// ODOO Manage Service
		&odooms.ODOOMSTicketField{},
		&odooms.ODOOMSTaskField{},
		&odooms.UploadedExcelToODOOMS{},
		&odooms.ODOOMSJobGroups{},
		&odooms.ODOOMSTechnicianData{},
		&odooms.ODOOMSFSParams{},
		&odooms.ODOOMSFSParamPayment{},
		&odooms.InventoryProductTemplate{},
		&odooms.ODOOMSCompany{},
		&odooms.ODOOMSTicketType{},
		&odooms.CSNABALost{},
		&odooms.MSTechnicianPayroll{},
		&odooms.MSTechnicianPayrollTicketsRegularEDC{},
		&odooms.MSTechnicianPayrollTicketsBP{},
		&odooms.MSTechnicianPayrollTicketsUnworkedEDC{},
		&odooms.MSTechnicianPayrollTicketsRegularATM{},
		&odooms.MSTechnicianPayrollTicketsUnworkedATM{},
		&odooms.MSTechnicianPayrollDedicatedATM{},
		// Stock Opname
		&stockopnamemodel.ProductEDCCSNA{},

		// SP Technician
		&sptechnicianmodel.TechnicianGotSP{},
		&sptechnicianmodel.SPLGotSP{},
		&sptechnicianmodel.SACGotSP{},
		&sptechnicianmodel.SPWhatsAppMessage{},
		&sptechnicianmodel.SPTelegramMessage{},
		&sptechnicianmodel.JOPlannedForTechnicianODOOMS{},
		&sptechnicianmodel.JOPlannedForTechnicianODOOATM{},
		&sptechnicianmodel.NomorSuratSP{},
		// SP Stock Opname
		&sptechnicianmodel.SPofStockOpname{},
		&sptechnicianmodel.SPStockOpnameWhatsappMessage{},

		// Contract Technician
		&contracttechnicianmodel.ContractTechnicianODOO{},
		&contracttechnicianmodel.NomorSuratContract{},

		// MTI
		&mtimodel.MTIOdooMSData{},

		// DKI
		&dkimodel.TicketDKI{},

		// DSP
		&dspmodel.TicketDSP{},

		// BNI
		&bnimodel.BNIOdooMSData{},
	); err != nil {
		logrus.Fatalf("Error while trying to automigrate db stage 1: %v", err)
	}

	// Stage 2: Create WhatsApp models without complex relationships
	if err := db.AutoMigrate(
		&whatsappmodel.WAContactInfo{},  // No dependencies
		&whatsappmodel.WAConversation{}, // Depends on Admin (User)
	); err != nil {
		logrus.Fatalf("Error while trying to automigrate db stage 2: %v", err)
	}

	// Stage 3: Create models that depend on conversations
	if err := db.AutoMigrate(
		&whatsappmodel.WAChatMessage{},      // Depends on WAConversation
		&whatsappmodel.WAGroupParticipant{}, // Depends on WAConversation
	); err != nil {
		logrus.Fatalf("Error while trying to automigrate db stage 3: %v", err)
	}

	// Stage 4: Create models that depend on messages
	if err := db.AutoMigrate(
		&whatsappmodel.WAMediaFile{},             // Depends on WAChatMessage
		&whatsappmodel.WAMessageDeliveryStatus{}, // Depends on WAChatMessage
	); err != nil {
		logrus.Fatalf("Error while trying to automigrate db: %v", err)
	}

	// Seeder
	seedRoles(db)
	seedFeature(db)
	seedRolePrivilege(db)
	seedAdmin(db)
	seedAdminStatus(db)
	seedAdminChangePwdLog(db)
	seedWhatsappLanguage(db)
	seedWhatsappPhoneUser(db)
	seedBadWords(db)
	seedAppConfig(db)
	seedNomorSuratSP(db)
	seedNomorSuratContract(db)
	seedIndonesiaRegion(db)

	// Seed Whatsapp Examples
	// controllers.SeedWhatsappSampleData(db)

	// Set auto-increment for TicketHommyPayCC only if table is empty
	var ticketHommyPayCCCount int64
	db.Model(&model.TicketHommyPayCC{}).Count(&ticketHommyPayCCCount)
	if ticketHommyPayCCCount == 0 {
		tableName := config.WebPanel.Get().Database.TbTicketHommyPayCC
		startID := config.WebPanel.Get().HommyPayCCData.StartTicketId

		sql := fmt.Sprintf("ALTER TABLE %s AUTO_INCREMENT = %d", tableName, startID)
		if err := db.Exec(sql).Error; err != nil {
			logrus.Fatalf("Error setting auto-increment for TicketHommyPayCC: %v", err)
		} else {
			logrus.Info("✅ Successfully set auto-increment for TicketHommyPayCC")
		}
	}

}
