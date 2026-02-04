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
	getODOOMSCompaniesMutex sync.Mutex
)

func GetCompanyODOOMS() error {
	taskDoing := "GetODOOMSCompany"
	startTime := time.Now()

	if !getODOOMSCompaniesMutex.TryLock() {
		return fmt.Errorf("%s is already running, please wait until it's finished", taskDoing)
	}
	defer getODOOMSCompaniesMutex.Unlock()

	logrus.Infof("Starting %s at %s", taskDoing, startTime)

	ODOOModel := "res.company"
	domain := []interface{}{
		// []interface{}{"active", "=", true},
	}
	fields := []string{
		"id",
		"name",
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

	var odooMSCompanies []ODOOMSCompanyItem
	if err := json.Unmarshal(ODOOResponseBytes, &odooMSCompanies); err != nil {
		return fmt.Errorf("failed to unmarshal ODOO response to ODOOMSCompanyItem slice: %v", err)
	}

	dbWeb := gormdb.Databases.Web

	for _, data := range odooMSCompanies {
		var existingCompany odooms.ODOOMSCompany
		result := dbWeb.Where("id = ?", data.ID).First(&existingCompany)
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			newCompany := odooms.ODOOMSCompany{
				ID:   data.ID,
				Name: data.Name.String,
			}
			if err := dbWeb.Create(&newCompany).Error; err != nil {
				logrus.Errorf("failed to insert new company ID %d: %v", data.ID, err)
			} else {
				logrus.Infof("inserted new company ID %d", data.ID)
			}
		} else if result.Error != nil {
			logrus.Errorf("error checking existing company ID %d: %v", data.ID, result.Error)
		} else {
			existingCompany.Name = data.Name.String
			if err := dbWeb.Save(&existingCompany).Error; err != nil {
				logrus.Errorf("failed to update company ID %d: %v", data.ID, err)
			} else {
				// logrus.Infof("updated company ID %d", data.ID)
			}
		}
	}

	logrus.Infof("Completed %s in %s", taskDoing, time.Since(startTime))

	return nil
}
