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
	getODOOMSInventoryProductsMutex sync.Mutex
)

func GetODOOMSInventoryProducts() error {
	taskDoing := "GetODOOMSInventoryProducts"
	startTime := time.Now()

	if !getODOOMSInventoryProductsMutex.TryLock() {
		return fmt.Errorf("%s is already running, please wait", taskDoing)
	}
	defer getODOOMSInventoryProductsMutex.Unlock()

	logrus.Infof("Starting %s at %s", taskDoing, startTime)

	ODOOModel := "product.template"
	domain := []interface{}{
		[]interface{}{"active", "=", true},
	}
	fields := []string{
		"id",
		"name",
		"company_id",
		"list_price",
		"type",
		"categ_id",
		// ADD: other fields if needed
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

	var productTemplates []ODOOMSProductTemplate
	err = json.Unmarshal(ODOOResponseBytes, &productTemplates)
	if err != nil {
		return fmt.Errorf("failed to unmarshal response to struct: %v", err)
	}

	dbWeb := gormdb.Databases.Web

	for _, data := range productTemplates {
		var existingData odooms.InventoryProductTemplate

		_, company := parseJSONIDDataCombinedSafe(data.CompanyId)
		_, category := parseJSONIDDataCombinedSafe(data.CategId)

		result := dbWeb.Where("id = ?", data.ID).First(&existingData)
		if result.Error != nil {
			if errors.Is(result.Error, gorm.ErrRecordNotFound) {
				newData := odooms.InventoryProductTemplate{
					ID:              data.ID,
					Name:            data.Name.String,
					ProductType:     data.Type.String,
					ProductCategory: category,
					Company:         company,
					ListPrice:       data.ListPrice.Float,
				}

				if err := dbWeb.Create(&newData).Error; err != nil {
					logrus.Errorf("failed to insert new product.template ID %d: %v", data.ID, err)
				} else {
					logrus.Infof("inserted new product.template ID %d", data.ID)
				}
			} else {
				logrus.Errorf("failed to query product.template ID %d: %v", data.ID, result.Error)
			}
		} else {
			// Update existing record
			existingData.Name = data.Name.String
			existingData.ProductType = data.Type.String
			existingData.ProductCategory = category
			existingData.Company = company
			existingData.ListPrice = data.ListPrice.Float

			if err := dbWeb.Save(&existingData).Error; err != nil {
				logrus.Errorf("failed to update product.template ID %d: %v", data.ID, err)
			} else {
				// logrus.Infof("updated product.template ID %d", data.ID)
			}
		}
	}

	return nil
}
