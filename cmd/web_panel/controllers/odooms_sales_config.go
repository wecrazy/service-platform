package controllers

import (
	"encoding/json"
	"errors"
	"fmt"
	"service-platform/cmd/web_panel/internal/gormdb"
	odooms "service-platform/cmd/web_panel/model/odoo_ms"
	"service-platform/internal/config"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

var (
	getODOOMSFSParamPaymentMutex sync.Mutex
)

func GetODOOMSFSParamPayment() error {
	taskDoing := "GetODOOMSFSParamPayment"
	startTime := time.Now()

	if !getODOOMSFSParamPaymentMutex.TryLock() {
		return fmt.Errorf("%s is already running, please wait", taskDoing)
	}
	defer getODOOMSFSParamPaymentMutex.Unlock()

	logrus.Infof("Starting %s at %s", taskDoing, startTime)

	ODOOModel := "fs.param.payment"
	domain := []interface{}{
		[]interface{}{"active", "=", true},
	}
	fields := []string{
		"id",
		"name",
		"param_type",
		"price",
		"company_id",
	}
	order := "id desc"

	odooParams := map[string]interface{}{
		"model":  ODOOModel,
		"domain": domain,
		"fields": fields,
		"order":  order,
	}

	payload := map[string]interface{}{
		"jsonrpc": config.WebPanel.Get().ApiODOO.JSONRPC,
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

	var fsParamPayments []ODOOMSFSParamPayment
	err = json.Unmarshal(ODOOResponseBytes, &fsParamPayments)
	if err != nil {
		return fmt.Errorf("failed to unmarshal response to struct: %v", err)
	}

	dbWeb := gormdb.Databases.Web

	for _, data := range fsParamPayments {
		_, company := parseJSONIDDataCombinedSafe(data.CompanyId)

		var existingData odooms.ODOOMSFSParamPayment
		result := dbWeb.Where("id = ?", data.ID).First(&existingData)
		if result.Error != nil {
			if errors.Is(result.Error, gorm.ErrRecordNotFound) {

				newData := odooms.ODOOMSFSParamPayment{
					ID:           data.ID,
					ParamType:    data.ParamType.String,
					ParamKey:     data.Name.String,
					ParamPrice:   data.Price.Int,
					ParamCompany: company,
				}

				if err := dbWeb.Create(&newData).Error; err != nil {
					logrus.Errorf("failed to insert new fs.param.payment ID %d: %v", data.ID, err)
				} else {
					logrus.Infof("inserted new fs.param.payment ID %d", data.ID)
				}
			} else {
				logrus.Errorf("error checking existing fs.param.payment ID %d: %v", data.ID, result.Error)
			}
		} else {
			existingData.ParamType = data.ParamType.String
			existingData.ParamKey = data.Name.String
			existingData.ParamPrice = data.Price.Int
			existingData.ParamCompany = company
			if err := dbWeb.Save(&existingData).Error; err != nil {
				logrus.Errorf("failed to update fs.param.payment ID %d: %v", data.ID, err)
			} else {
				// logrus.Infof("updated existing fs.param.payment ID %d", data.ID)
			}
		}
	}

	return nil
}
