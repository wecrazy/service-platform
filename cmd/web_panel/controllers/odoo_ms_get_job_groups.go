package controllers

import (
	"encoding/json"
	"errors"
	"fmt"
	"service-platform/cmd/web_panel/config"
	"service-platform/cmd/web_panel/internal/gormdb"
	odooms "service-platform/cmd/web_panel/model/odoo_ms"
	"sync"

	"gorm.io/gorm"
)

var (
	getJobGroupsODOOMSMutex sync.Mutex
)

func GetJobGroupsODOOMS() error {
	taskDoing := "Get Job Groups ODOO MS"
	if !getJobGroupsODOOMSMutex.TryLock() {
		return fmt.Errorf("%s already running, please wait a moment", taskDoing)
	}
	defer getJobGroupsODOOMSMutex.Unlock()

	dbWeb := gormdb.Databases.Web

	ODOOModel := "job.group"
	domain := []interface{}{}
	fields := []string{
		"id",
		"name",
		"basic_salary",
		"task_max",
		"insentive",
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

	var odooData []JobGroupsItem
	err = json.Unmarshal(ODOOResponseBytes, &odooData)
	if err != nil {
		return fmt.Errorf("failed to unmarshal response to struct: %v", err)
	}

	for _, item := range odooData {
		var existingData odooms.ODOOMSJobGroups
		result := dbWeb.Where("id = ?", item.ID).First(&existingData)
		if result.Error != nil {
			if errors.Is(result.Error, gorm.ErrRecordNotFound) {
				// Create new record
				newRecord := odooms.ODOOMSJobGroups{
					ID:               item.ID,
					Name:             item.Name.String,
					BasicSalary:      item.BasicSalary.Int,
					TaskMax:          item.TaskMax.Int,
					InsentivePerTask: item.Insentive.Int,
				}
				if err := dbWeb.Create(&newRecord).Error; err != nil {
					return fmt.Errorf("failed to create new Job Group record (ID: %d): %v", item.ID, err)
				}
			} else {
				return fmt.Errorf("error querying Job Group record (ID: %d): %v", item.ID, result.Error)
			}
		} else {
			// Update existing record
			existingData.Name = item.Name.String
			existingData.BasicSalary = item.BasicSalary.Int
			existingData.TaskMax = item.TaskMax.Int
			existingData.InsentivePerTask = item.Insentive.Int

			if err := dbWeb.Save(&existingData).Error; err != nil {
				return fmt.Errorf("failed to update Job Group record (ID: %d): %v", item.ID, err)
			}
		}
	}

	return nil
}
