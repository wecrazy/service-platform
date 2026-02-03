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
	"gorm.io/gorm"
)

var (
	getODOOMSFSParamsMutex sync.Mutex
)

func GetODOOMSFSParams() error {
	taskDoing := "GetODOOMSFSParams"
	startTime := time.Now()

	if !getODOOMSFSParamsMutex.TryLock() {
		return fmt.Errorf("%s is already running, please wait", taskDoing)
	}
	defer getODOOMSFSParamsMutex.Unlock()

	logrus.Infof("Starting %s at %s", taskDoing, startTime)

	ODOOModel := "fs.params"
	domain := []interface{}{
		[]interface{}{"active", "=", true},
	}
	fields := []string{
		"id",
		"create_uid",
		"write_uid",
		"display_name",
		"name",
		"logs",
		"value",
	}
	order := "id desc"

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

	var fsParamsData []ODOOMSFSParams
	err = json.Unmarshal(ODOOResponseBytes, &fsParamsData)
	if err != nil {
		return fmt.Errorf("failed to unmarshal response to struct: %v", err)
	}

	dbWeb := gormdb.Databases.Web

	for _, data := range fsParamsData {
		var existingData odooms.ODOOMSFSParams
		result := dbWeb.Where("id = ?", data.ID).First(&existingData)
		if result.Error != nil {
			if errors.Is(result.Error, gorm.ErrRecordNotFound) {
				newData := odooms.ODOOMSFSParams{
					ID:         data.ID,
					ParamKey:   data.Name.String,
					ParamValue: data.Value.String,
					Logs:       data.Logs.String,
				}

				if err := dbWeb.Create(&newData).Error; err != nil {
					logrus.Errorf("failed to insert fs.params ID %d: %v", data.ID, err)
				} else {
					logrus.Infof("inserted new fs.params ID %d", data.ID)
				}
			} else {
				logrus.Errorf("error checking existing fs.params ID %d: %v", data.ID, result.Error)
			}
		} else {
			existingData.ParamKey = data.Name.String
			existingData.ParamValue = data.Value.String
			existingData.Logs = data.Logs.String
			if err := dbWeb.Save(&existingData).Error; err != nil {
				logrus.Errorf("failed to update fs.params ID %d: %v", data.ID, err)
			} else {
				// logrus.Infof("updated existing fs.params ID %d", data.ID)
			}
		}
	}

	return nil
}
