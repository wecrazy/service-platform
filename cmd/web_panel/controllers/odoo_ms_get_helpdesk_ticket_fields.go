package controllers

import (
	"encoding/json"
	"errors"
	"fmt"
	"service-platform/cmd/web_panel/config"
	odooms "service-platform/cmd/web_panel/model/odoo_ms"
	"sync"

	"gorm.io/gorm"
)

var (
	getFieldsHelpdeskTicketODOOMSMutex sync.Mutex
)

func GetHelpdeskTicketFields() error {
	taskDoing := "Get Helpdesk.Ticket Fields"
	if !getFieldsHelpdeskTicketODOOMSMutex.TryLock() {
		return fmt.Errorf("%s already running, please wait a moment", taskDoing)
	}
	defer getFieldsHelpdeskTicketODOOMSMutex.Unlock()

	ODOOModel := "ir.model.fields"
	domain := []interface{}{
		[]interface{}{"model", "=", "helpdesk.ticket"},
	}
	fields := []string{
		"id",
		"name",
		"field_description",
		"ttype",
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

	var fieldsData []OdooIRModelFields
	if err := json.Unmarshal(ODOOResponseBytes, &fieldsData); err != nil {
		return fmt.Errorf("failed to unmarshal ODOO response: %v", err)
	}

	for _, field := range fieldsData {
		var existingField odooms.ODOOMSTicketField
		result := dbWeb.Where("id = ?", field.ID).First(&existingField)
		if result.Error != nil {
			if errors.Is(result.Error, gorm.ErrRecordNotFound) {
				// Create new record
				newField := odooms.ODOOMSTicketField{
					ID:          field.ID,
					Name:        field.FieldName,
					Description: field.FieldLabel,
					Type:        field.FieldType,
				}
				if err := dbWeb.Create(&newField).Error; err != nil {
					return fmt.Errorf("failed to create ODOOMSTicketField: %v", err)
				}
			} else {
				return fmt.Errorf("failed to query ODOOMSTicketField: %v", result.Error)
			}
		} else {
			// Update existing record
			existingField.Name = field.FieldName
			existingField.Description = field.FieldLabel
			existingField.Type = field.FieldType
			if err := dbWeb.Save(&existingField).Error; err != nil {
				return fmt.Errorf("failed to update ODOOMSTicketField: %v", err)
			}
		}
	}

	return nil
}
