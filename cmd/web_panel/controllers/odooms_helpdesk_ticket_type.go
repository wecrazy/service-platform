package controllers

import (
	"encoding/json"
	"errors"
	"fmt"
	"service-platform/cmd/web_panel/config"
	"service-platform/cmd/web_panel/internal/gormdb"
	odooms "service-platform/cmd/web_panel/model/odoo_ms"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

var (
	getODOOMSHelpdeskTicketTypeMutex sync.Mutex
)

func GetTicketTypeODOOMS() error {
	taskDoing := "GetTicketTypeODOOMS"
	startTime := time.Now()

	if !getODOOMSHelpdeskTicketTypeMutex.TryLock() {
		return fmt.Errorf("%s is still running, please wait", taskDoing)
	}
	defer getODOOMSHelpdeskTicketTypeMutex.Unlock()

	logrus.Infof("Starting %s at %s", taskDoing, startTime)

	ODOOModel := "helpdesk.ticket.type"
	domain := []interface{}{
		// []interface{}{"active", "=", true},
	}
	fields := []string{
		"id",
		"sequence",
		"name",
		"x_task_type",
		"x_worksheet_template_id",
		"x_plan_intervention",
		"x_studio_multi_company",
	}
	order := "id asc"

	odooParams := map[string]interface{}{
		"model":  ODOOModel,
		"domain": domain,
		"fields": fields,
		"order":  order,
	}

	payload := map[string]interface{}{
		"jsonrpc": config.GetConfig().ApiODOO.JSONRPC,
		"params":  odooParams,
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	ODOOresponse, err := GetODOOMSData(string(payloadBytes))
	if err != nil {
		errMsg := fmt.Sprintf("failed fetching data from ODOO MS API: %v", err)
		return errors.New(errMsg)
	}

	ODOOResponseArray, ok := ODOOresponse.([]interface{})
	if !ok {
		errMsg := "failed to asset results as []interface{}"
		return errors.New(errMsg)
	}

	ODOOResponseBytes, err := json.Marshal(ODOOResponseArray)
	if err != nil {
		return fmt.Errorf("failed to marshal combined response: %v", err)
	}

	var odooMSHelpdeskTicketTypes []OdooHelpdeskTicketTypeItem
	if err := json.Unmarshal(ODOOResponseBytes, &odooMSHelpdeskTicketTypes); err != nil {
		return fmt.Errorf("failed to unmarshal ODOO response: %v", err)
	}

	dbWeb := gormdb.Databases.Web

	for _, data := range odooMSHelpdeskTicketTypes {
		worksheetTemplateID, worksheetTemplate := parseJSONIDDataCombinedSafe(data.WorksheetTemplateId)
		var multiCompanyJSON datatypes.JSON
		if data.MultiCompany.Valid {
			multiCompanyJSONBytes, err := json.Marshal(data.MultiCompany.Ints)
			if err != nil {
				logrus.Errorf("failed to marshal multi company for ticket type ID %d: %v", data.ID, err)
			} else {
				multiCompanyJSON = datatypes.JSON(multiCompanyJSONBytes)
			}
		}

		var existingTicketType odooms.ODOOMSTicketType
		result := dbWeb.Where("id = ?", data.ID).First(&existingTicketType)
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			newTicketType := odooms.ODOOMSTicketType{
				ID:                       data.ID,
				Sequence:                 data.Sequence.Int,
				Type:                     data.Type.String,
				TaskType:                 data.TaskType.String,
				WorksheetTemplateId:      worksheetTemplateID,
				WorksheetTemplate:        worksheetTemplate,
				DirectlyPlanIntervantion: data.DirectlyPlanIntervantion.Bool,
				MultiCompany:             multiCompanyJSON,
			}
			if err := dbWeb.Create(&newTicketType).Error; err != nil {
				logrus.Errorf("failed to insert new ticket type ID %d: %v", data.ID, err)
			}
		} else if result.Error != nil {
			logrus.Errorf("error checking existing ticket type ID %d: %v", data.ID, result.Error)
		} else {
			existingTicketType.Sequence = data.Sequence.Int
			existingTicketType.Type = data.Type.String
			existingTicketType.TaskType = data.TaskType.String
			existingTicketType.WorksheetTemplateId = worksheetTemplateID
			existingTicketType.WorksheetTemplate = worksheetTemplate
			existingTicketType.DirectlyPlanIntervantion = data.DirectlyPlanIntervantion.Bool
			existingTicketType.MultiCompany = multiCompanyJSON

			if err := dbWeb.Save(&existingTicketType).Error; err != nil {
				logrus.Errorf("failed to update ticket type ID %d: %v", data.ID, err)
			}
		}
	}

	logrus.Infof("Completed %s in %s", taskDoing, time.Since(startTime))

	return nil
}
