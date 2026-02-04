package controllers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"service-platform/cmd/web_panel/fun"
	"service-platform/cmd/web_panel/internal/gormdb"
	sptechnicianmodel "service-platform/cmd/web_panel/model/sp_technician_model"
	stockopnamemodel "service-platform/cmd/web_panel/model/stock_opname_model"
	"service-platform/internal/config"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"github.com/xuri/excelize/v2"
	"go.mau.fi/whatsmeow/types/events"
	"gorm.io/gorm"
)

// StockOpnameReportItem represents a single item in the stock opname report,
// containing details about the technician, serial number, product, company, and location.
type StockOpnameReportItem struct {
	Technician string
	SN         string
	Product    string
	Company    string
	Location   string
}

var (
	getDataProductEDCCSNAMutex sync.Mutex
)

// GetDataProductEDCCSNA fetches product data from ODOO MS, processes it,
// and updates the local database with the latest product information.
// It handles data fetching in chunks and updates product details concurrently.
//
// Returns:
//   - error: An error if the data fetching or processing fails.
func GetDataProductEDCCSNA() error {
	taskDoing := "Get Data of Products EDC CSNA"
	startTime := time.Now()
	logrus.Infof("Starting %s", taskDoing)

	if !getDataProductEDCCSNAMutex.TryLock() {
		return fmt.Errorf("%s is still running, skipped this run", taskDoing)
	}
	defer getDataProductEDCCSNAMutex.Unlock()

	ODOOModel := "stock.production.lot"
	fieldID := []string{"id"}
	fields := []string{
		"id",
		"name",
		"product_id",
		"x_product_categ_id",
		"company_id",
	}

	excludedCompany := config.WebPanel.Get().ApiODOO.CompanyExcluded
	domain := []any{
		[]any{"company_id", "!=", excludedCompany},
	}
	order := "id asc"
	odooParams := map[string]any{
		"model":  ODOOModel,
		"domain": domain,
		"fields": fieldID,
		"order":  order,
	}

	payload := map[string]any{
		"jsonrpc": config.WebPanel.Get().ApiODOO.JSONRPC,
		"params":  odooParams,
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload for %s: %v", taskDoing, err)
	}

	ODOOResp, err := GetODOOMSData(string(payloadBytes))
	if err != nil {
		return fmt.Errorf("failed to get ODOO MS data for %s: %v", taskDoing, err)
	}

	ODOORespArray, ok := ODOOResp.([]any)
	if !ok {
		return fmt.Errorf("invalid ODOO MS response format for %s", taskDoing)
	}

	ids := extractUniqueIDs(ODOORespArray)

	if len(ids) == 0 {
		return fmt.Errorf("no data found in ODOO MS for %s", taskDoing)
	}

	const batchSize = 10000
	chunks := chunkIdsSlice(ids, batchSize)
	var allRecords []any

	// Use workers to process chunks concurrently
	type chunkRes struct {
		records []any
		err     error
		index   int
	}

	resChan := make(chan chunkRes, len(chunks))
	semaphore := make(chan struct{}, 5) // Limit to 5 concurrent workers

	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Minute)
	defer cancel()

	for i, chunk := range chunks {
		go func(chunkIndex int, chunkData []uint64) {
			defer func() {
				if r := recover(); r != nil {
					logrus.Errorf("Panic in chunk processing %d: %v", chunkIndex, r)
					resChan <- chunkRes{nil, fmt.Errorf("panic in chunk %d: %v", chunkIndex, r), chunkIndex}
				}
			}()

			select {
			case semaphore <- struct{}{}: // Acquire semaphore with timeout
			case <-ctx.Done():
				resChan <- chunkRes{nil, fmt.Errorf("chunk %d timeout", chunkIndex), chunkIndex}
				return
			}
			defer func() { <-semaphore }() // Release semaphore

			// logrus.Debugf("Processing (%s) chunk %d of %d (IDs %v to %v)", taskDoing, chunkIndex+1, len(chunks), chunkData[0], chunkData[len(chunkData)-1])

			chunkDomain := []interface{}{
				[]interface{}{"id", "=", chunkData},
			}

			odooParams := map[string]interface{}{
				"model":  ODOOModel,
				"domain": chunkDomain,
				"fields": fields,
				"order":  order,
			}

			payload := map[string]interface{}{
				"jsonrpc": config.WebPanel.Get().ApiODOO.JSONRPC,
				"params":  odooParams,
			}

			payloadBytes, err := json.Marshal(payload)
			if err != nil {
				resChan <- chunkRes{nil, fmt.Errorf("failed to marshal payload for chunk %d: %v", chunkIndex+1, err), chunkIndex}
				return
			}

			ODOOresponse, err := GetODOOMSData(string(payloadBytes))
			if err != nil {
				resChan <- chunkRes{nil, fmt.Errorf("failed fetching data from ODOO MS API for chunk %d: %v", chunkIndex+1, err), chunkIndex}
				return
			}

			ODOOResponseArray, ok := ODOOresponse.([]interface{})
			if !ok {
				resChan <- chunkRes{nil, fmt.Errorf("type assertion failed for chunk %d", chunkIndex+1), chunkIndex}
				return
			}

			resChan <- chunkRes{ODOOResponseArray, nil, chunkIndex}
		}(i, chunk)
	}

	// Collect results from all goroutines with timeout protection
	results := make([]chunkRes, len(chunks))
	for i := 0; i < len(chunks); i++ {
		select {
		case result := <-resChan:
			if result.index < len(results) {
				results[result.index] = result
			}
		case <-ctx.Done():
			logrus.Errorf("Timeout waiting for chunk results")
			return fmt.Errorf("timeout waiting for chunk results in %s", taskDoing)
		}
	}

	// Process results in order and handle errors gracefully
	for i, result := range results {
		if result.err != nil {
			logrus.Errorf("Error processing chunk %d: %v", i, result.err)
			continue // Continue with other chunks instead of failing completely
		}
		if result.records != nil {
			// logrus.Debugf("Appending %d records from chunk %d", len(result.records), i)
			allRecords = append(allRecords, result.records...)
		}
	}

	if len(allRecords) == 0 {
		return fmt.Errorf("no valid data retrieved from ODOO MS for %s", taskDoing)
	}

	ODOOResponseBytes, err := json.Marshal(allRecords)
	if err != nil {
		return fmt.Errorf("failed to marshal ODOO MS response for %s: %v", taskDoing, err)
	}

	// Pre-allocate slice with capacity based on number of records to reduce memory allocations
	var listOfData []ODOOSerialNumberItem
	estimatedCapacity := len(allRecords)
	if estimatedCapacity > 0 {
		listOfData = make([]ODOOSerialNumberItem, 0, estimatedCapacity)
	}

	if err := json.Unmarshal(ODOOResponseBytes, &listOfData); err != nil {
		return fmt.Errorf("failed to unmarshal ODOO MS data for %s: %v", taskDoing, err)
	}

	// Insert data into database
	db := gormdb.Databases.Web
	if db == nil {
		return fmt.Errorf("database connection is nil")
	}

	// Optional: Clear existing data
	result := db.Unscoped().Where("1=1").Delete(&stockopnamemodel.ProductEDCCSNA{})
	if result.Error != nil {
		if !errors.Is(result.Error, gorm.ErrRecordNotFound) {
			logrus.Errorf("Failed to clear existing ProductEDCCSNA data: %v", result.Error)
			return fmt.Errorf("failed to clear existing ProductEDCCSNA data: %v", result.Error)
		}
	} else {
		logrus.Infof("Cleared %d existing ProductEDCCSNA records", result.RowsAffected)
	}

	// Process listOfData in chunks to limit memory usage
	var totalInserted int
	chunkBatchSize := 10000
	dbBatchSize := 1000 // Limit DB batch size to avoid MySQL placeholder limit
	for i := 0; i < len(listOfData); i += chunkBatchSize {
		end := i + chunkBatchSize
		if end > len(listOfData) {
			end = len(listOfData)
		}
		dataChunk := listOfData[i:end]

		logrus.Infof("Processing data chunk %d-%d with %d items", i, end-1, len(dataChunk))

		var productsBatch []stockopnamemodel.ProductEDCCSNA
		for _, item := range dataChunk {
			productID, productName, err := parseJSONIDDataCombined(item.Product)
			if err != nil {
				logrus.Errorf("Failed to parse product data for SN %s: %v", item.SN.String, err)
				continue // Skip this item and continue with the next
			}

			productCategoryID, productCategoryName, err := parseJSONIDDataCombined(item.ProductCategory)
			if err != nil {
				logrus.Errorf("Failed to parse product category data for SN %s: %v", item.SN.String, err)
				continue // Skip this item and continue with the next
			}

			_, company, err := parseJSONIDDataCombined(item.Company)
			if err != nil {
				logrus.Errorf("Failed to parse company data for SN %s: %v", item.SN.String, err)
				continue // Skip this item and continue with the next
			}

			product := stockopnamemodel.ProductEDCCSNA{
				ID:                item.ID,
				Company:           strings.TrimSpace(company),
				SN_EDC:            strings.TrimSpace(item.SN.String),
				ProductID:         productID,
				Product:           strings.TrimSpace(productName),
				ProductCategoryID: productCategoryID,
				ProductCategory:   strings.TrimSpace(productCategoryName),
			}
			productsBatch = append(productsBatch, product)
		}

		logrus.Infof("Collected %d products to insert from this chunk", len(productsBatch))

		// Insert products in DB batches to avoid MySQL placeholder limit
		for j := 0; j < len(productsBatch); j += dbBatchSize {
			subEnd := j + dbBatchSize
			if subEnd > len(productsBatch) {
				subEnd = len(productsBatch)
			}
			subBatch := productsBatch[j:subEnd]

			result := db.Create(subBatch)
			if result.Error != nil {
				logrus.Errorf("Failed to insert ProductEDCCSNA sub-batch: %v", result.Error)
				return fmt.Errorf("failed to insert ProductEDCCSNA sub-batch: %v", result.Error)
			}
			logrus.Infof("Inserted %d rows in sub-batch (expected %d) from data chunk %d-%d offset %d-%d", result.RowsAffected, len(subBatch), i, end-1, j, subEnd-1)
			totalInserted += int(result.RowsAffected)
		}
	}

	logrus.Infof("Total inserted %d ProductEDCCSNA records in data chunks of %d and DB batches of %d", totalInserted, chunkBatchSize, dbBatchSize)

	// Log memory usage for monitoring
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)
	logrus.Infof("Memory Usage before processing Stock Opname: Alloc = %v MiB, TotalAlloc = %v MiB, Sys = %v MiB, NumGC = %v",
		memStats.Alloc/1024/1024,
		memStats.TotalAlloc/1024/1024,
		memStats.Sys/1024/1024,
		memStats.NumGC,
	)
	runtime.GC() // Force garbage collection to free up memory

	// Collect inserted IDs from the fetched data instead of querying DB
	var insertedIDs []uint
	for _, item := range listOfData {
		insertedIDs = append(insertedIDs, item.ID)
	}
	logrus.Infof("Collected %d inserted IDs for further processing", len(insertedIDs))

	// Update other columns in batches with concurrency
	idBatchSize := 10000
	type batchInfo struct {
		start int
		end   int
		index int
	}
	var batches []batchInfo
	for i := 0; i < len(insertedIDs); i += idBatchSize {
		end := i + idBatchSize
		if end > len(insertedIDs) {
			end = len(insertedIDs)
		}
		batches = append(batches, batchInfo{start: i, end: end, index: len(batches)})
	}

	var wg sync.WaitGroup
	sem := make(chan struct{}, 5) // Limit to 5 concurrent updates
	errChan := make(chan error, len(batches))

	for _, batch := range batches {
		wg.Add(1)
		go func(b batchInfo) {
			defer wg.Done()

			sem <- struct{}{}        // Acquire
			defer func() { <-sem }() // Release

			ids := insertedIDs[b.start:b.end]

			logrus.Infof("Processing IDs batch %d-%d with %d items", b.start, b.end-1, len(ids))

			// Get other columns data from ODOO API
			ODOOModel := "stock.move.line"
			fields := []string{
				"id",
				"state",
				"date",
				"reference",
				"origin",
				"product_id",
				"lot_id",
				"location_id",
				"location_dest_id",
			}
			domain := []any{
				[]any{"lot_id", "=", ids},
				[]any{"state", "=", "done"},
			}

			odooParams := map[string]any{
				"model":  ODOOModel,
				"domain": domain,
				"fields": fields,
			}

			payload := map[string]any{
				"jsonrpc": config.WebPanel.Get().ApiODOO.JSONRPC,
				"params":  odooParams,
			}

			payloadBytes, err := json.Marshal(payload)
			if err != nil {
				logrus.Errorf("Failed to marshal payload for stock.move.line at batch %d: %v", b.index, err)
				errChan <- err
				return
			}

			ODOOResp, err := GetODOOMSData(string(payloadBytes))
			if err != nil {
				logrus.Errorf("Failed to get ODOO data for stock.move.line at batch %d: %v", b.index, err)
				errChan <- err
				return
			}

			ODOORespArray, ok := ODOOResp.([]any)
			if !ok {
				err := fmt.Errorf("invalid ODOO response format for stock.move.line at batch %d", b.index)
				logrus.Error(err)
				errChan <- err
				return
			}

			if len(ODOORespArray) == 0 {
				logrus.Infof("No stock.move.line data for IDs at batch %d", b.index)
				return
			}

			ODOOResponseBytes, err := json.Marshal(ODOORespArray)
			if err != nil {
				logrus.Errorf("Failed to marshal ODOO response for stock.move.line at batch %d: %v", b.index, err)
				errChan <- err
				return
			}

			var moveLines []StockMoveLineItem
			if err := json.Unmarshal(ODOOResponseBytes, &moveLines); err != nil {
				logrus.Errorf("Failed to unmarshal stock.move.line data at batch %d: %v", b.index, err)
				errChan <- err
				return
			}

			logrus.Infof("Fetched %d stock.move.line records for IDs at batch %d", len(moveLines), b.index)

			// Collect updates for batch processing
			updatesMap := make(map[uint]map[string]interface{})
			for _, move := range moveLines {
				if move.SN.Valid {
					productid, _, err := parseJSONIDDataCombined(move.SN)
					if err != nil {
						logrus.Errorf("Failed to parse SN for move ID %d: %v", move.ID, err)
						continue
					}

					productID := uint(productid)

					if updatesMap[productID] == nil {
						updatesMap[productID] = make(map[string]interface{})
					}

					if move.Reference.Valid {
						updatesMap[productID]["reference"] = strings.TrimSpace(move.Reference.String)
					}

					if move.Source.Valid {
						updatesMap[productID]["source"] = strings.TrimSpace(move.Source.String)
					}

					if move.FromLocation.Valid {
						_, fromLoc, err := parseJSONIDDataCombined(move.FromLocation)
						if err == nil {
							updatesMap[productID]["from_location"] = strings.TrimSpace(fromLoc)
						}
					}
					if move.ToLocation.Valid {
						_, toLoc, err := parseJSONIDDataCombined(move.ToLocation)
						if err == nil {
							updatesMap[productID]["to_location"] = strings.TrimSpace(toLoc)
						}
					}
					var moveDate *time.Time
					if move.MoveDate.String != "" {
						parsedTime, err := time.Parse("2006-01-02 15:04:05", move.MoveDate.String)
						if err == nil {
							moveDate = &parsedTime
						}
					}

					updatesMap[productID]["date_product_move"] = moveDate
					// Add other fields as needed
				}
			}

			// Batch update for each field
			fieldNames := []string{"from_location", "to_location", "date_product_move", "reference", "source"}
			for _, field := range fieldNames {
				var ids []uint
				var values []interface{}
				for id, up := range updatesMap {
					if val, ok := up[field]; ok {
						ids = append(ids, id)
						values = append(values, val)
					}
				}
				if len(ids) == 0 {
					continue
				}

				// Build CASE statement
				caseStr := "CASE id "
				args := []interface{}{}
				for i, id := range ids {
					caseStr += "WHEN ? THEN ? "
					args = append(args, id, values[i])
				}
				caseStr += "END"

				result := db.Model(&stockopnamemodel.ProductEDCCSNA{}).Where("id IN ?", ids).Update(field, gorm.Expr(caseStr, args...))
				if result.Error != nil {
					logrus.Errorf("Failed to batch update %s for %d records in batch %d: %v", field, len(ids), b.index, result.Error)
					errChan <- result.Error
				} else {
					logrus.Infof("Batch updated %s for %d records in batch %d", field, len(ids), b.index)
				}
			}
		}(batch)
	}

	wg.Wait()
	close(errChan)

	// Check for errors
	var hasErrors bool
	for err := range errChan {
		if err != nil {
			hasErrors = true
			logrus.Errorf("Error in concurrent update: %v", err)
		}
	}
	if hasErrors {
		return fmt.Errorf("some updates failed")
	}

	totalDuration := time.Since(startTime)
	logrus.Infof("%s completed in %v", taskDoing, totalDuration)

	return nil
}

// processSPofStockOpnameTechnician processes the Stock Opname (SO) for technicians,
// identifying items that were not SO'd or are missing. It generates an Excel report
// and handles SP (Surat Peringatan) logic for technicians and SPLs based on the findings.
//
// Parameters:
//   - db: Database connection.
//   - forProject: The project identifier.
//   - hrdPersonaliaName: Name of the HRD personalia.
//   - hrdTTDPath: Path to the HRD signature image.
//   - hrdPhoneNumber: phone number that HRD used
//   - excludedTechnicians: List of technicians to exclude.
//   - needToSendTheSPTechnicianThroughWhatsapp: Map tracking SP sending status for technicians.
//   - needToSendTheSPSPLThroughWhatsapp: Map tracking SP sending status for SPLs.
//   - resignTechnicianReplacer: Name of the replacer for resigned technicians.
//   - audioDirForSPTechnician: Directory for technician SP audio files.
//   - pdfDirForSPTechnician: Directory for technician SP PDF files.
//   - audioDirForSPSPL: Directory for SPL SP audio files.
//   - pdfDirForSPSPL: Directory for SPL SP PDF files.
//   - excelReportDirUsed: Directory to save the generated Excel report.
//
// Returns:
//   - string: The file path of the generated Excel report.
//   - error: An error if the processing fails.
func processSPofStockOpnameTechnician(
	db *gorm.DB,
	forProject string,
	hrdPersonaliaName string,
	hrdTTDPath string,
	hrdPhoneNumber string,
	excludedTechnicians []string,
	needToSendTheSPTechnicianThroughWhatsapp map[string]int,
	needToSendTheSPSPLThroughWhatsapp map[string]int,
	resignTechnicianReplacer string,
	audioDirForSPTechnician string,
	pdfDirForSPTechnician string,
	audioDirForSPSPL string,
	pdfDirForSPSPL string,
	excelReportDirUsed string,
) (string, error) {
	var countOfProductEDCCSNA int64
	db.Model(&stockopnamemodel.ProductEDCCSNA{}).Count(&countOfProductEDCCSNA)
	if countOfProductEDCCSNA == 0 {
		return "", errors.New("no ProductEDCCSNA data available to process SP of Stock Opname")
	}

	if TechODOOMSData == nil {
		return "", errors.New("TechODOOMSData is nil, cannot process SP of Stock Opname")
	}

	var notSOItems []StockOpnameReportItem
	var missingEDCItems []StockOpnameReportItem

	// Try to process each technician existing in ODOO MS Stock Opname data
	for technician, data := range TechODOOMSData {
		if len(excludedTechnicians) > 0 {
			isExcluded := false
			for _, excludedTech := range excludedTechnicians {
				if strings.Contains(strings.ToLower(technician), excludedTech) {
					isExcluded = true
					break
				}
			}
			if isExcluded {
				logrus.Infof("Technician %s is in excluded list, skipping", technician)
				continue
			}
		}

		var namaTeknisi string
		if data.Name != "" {
			namaTeknisi = data.Name
		} else {
			namaTeknisi = technician
		}

		isSPL := false
		if strings.Contains(technician, "SPL") {
			isSPL = true
		}

		todayIsMonday := false
		if time.Now().Weekday() == time.Monday {
			todayIsMonday = true
		}

		if isSPL && !todayIsMonday {
			logrus.Infof("Today is not Monday, skipping SP processing for SPL %s", data.SPL)
			continue
		}

		// Reset SP status before do checking, coz might the SP back set to the SP-1
		if err := ResetTechnicianSP(technician, forProject); err != nil {
			if !errors.Is(err, gorm.ErrRecordNotFound) {
				logrus.Warnf("Failed to reset SP for technician %s: %v", technician, err)
			}
		}
		if err := ResetSPLSP(data.SPL, forProject); err != nil {
			if !errors.Is(err, gorm.ErrRecordNotFound) {
				logrus.Warnf("Failed to reset SP for SPL %s: %v", data.SPL, err)
			}
		}
		if err := ResetSACSP(data.SAC, forProject); err != nil {
			if !errors.Is(err, gorm.ErrRecordNotFound) {
				logrus.Warnf("Failed to reset SP for SAC %s: %v", data.SAC, err)
			}
		}

		// Get data of Stock Opname for the technician today
		stockOpnameTechnician, err := getStockOpnameOfTechnicianToday(technician)
		if err != nil {
			logrus.Errorf("Failed to get Stock Opname data for technician %s: %v", technician, err)

			productsEDCCSNAExists := make(map[string][]string) // Store existing EDC SN in ProductEDCCSNA for each company that not being SO by the technician
			for company, locStock := range data.TechnicianInventoryLocation {
				var edcOfCSNA []stockopnamemodel.ProductEDCCSNA
				if err := db.Where("product_category = ?", "EDC").
					Where("from_location = ?", locStock).
					Where("to_location = ?", locStock).
					Where("company = ?", company).
					Find(&edcOfCSNA).
					Error; err != nil {
					if errors.Is(err, gorm.ErrRecordNotFound) {
						logrus.Warnf("No ProductEDCCSNA found for processing SO of technician %s for company %s", technician, company)
						continue
					}
					logrus.Errorf("Failed to query ProductEDCCSNA for processing SO of technician %s for company %s: %v", technician, company, err)
					continue
				}

				totalEDCExists := len(edcOfCSNA)
				if totalEDCExists == 0 {
					logrus.Infof("No EDC found in ProductEDCCSNA for technician %s in company %s at location %s", technician, company, locStock)
					continue
				} else {
					if _, exists := productsEDCCSNAExists[company]; !exists {
						for _, edcItem := range edcOfCSNA {
							productsEDCCSNAExists[company] = append(productsEDCCSNAExists[company], fmt.Sprintf("%s (%s)", edcItem.SN_EDC, edcItem.Product))
							notSOItems = append(notSOItems, StockOpnameReportItem{
								Technician: technician,
								SN:         edcItem.SN_EDC,
								Product:    edcItem.Product,
								Company:    company,
								Location:   locStock,
							})
						}
					}
				} // .end of write the existing edc into map of each company
			}

			if len(productsEDCCSNAExists) > 0 {
				if !isSPL {
					splData, exists := TechODOOMSData[data.SPL]
					if !exists {
						logrus.Errorf("no SPL found for technician %s", technician)
						continue
					}
					var namaSPL, splCity string
					if splData.Name != "" {
						namaSPL = splData.Name
					} else {
						namaSPL = data.SPL
					}
					splCity = getSPLCity(data.SPL)
					if splCity == "" {
						splCity = "Unknown"
					}

					ODOOMSSAC := config.WebPanel.Get().ODOOMSSAC
					if len(ODOOMSSAC) == 0 {
						return "", errors.New("no data found for SAC ODOO Manage Service")
					}
					SACDataTechnician, ok := ODOOMSSAC[data.SAC]
					if !ok {
						return "", fmt.Errorf("no SAC data found for technician : %s", technician)
					}

					if err := processSPForTechnicianNotSOWithExistingEDC(
						db,
						forProject,
						hrdPersonaliaName,
						hrdTTDPath,
						hrdPhoneNumber,
						technician,
						namaTeknisi,
						namaSPL,
						splCity,
						SACDataTechnician,
						resignTechnicianReplacer,
						audioDirForSPTechnician,
						pdfDirForSPTechnician,
						needToSendTheSPTechnicianThroughWhatsapp,
						productsEDCCSNAExists,
					); err != nil {
						logrus.Errorf("got error while trying to process SP of technician %s coz not SO while still had existing EDC : %v", technician, err)
						continue
					}
					// .end of SP of technician
				} else {
					splData, exists := TechODOOMSData[data.SPL]
					if !exists {
						logrus.Errorf("no SPL found for technician %s", technician)
						continue
					}
					var namaSPL, splCity string
					if splData.Name != "" {
						namaSPL = splData.Name
					} else {
						namaSPL = data.SPL
					}
					splCity = getSPLCity(data.SPL)
					if splCity == "" {
						splCity = "Unknown"
					}

					ODOOMSSAC := config.WebPanel.Get().ODOOMSSAC
					if len(ODOOMSSAC) == 0 {
						return "", errors.New("no data found for SAC ODOO Manage Service")
					}
					SACData, ok := ODOOMSSAC[data.SAC]
					if !ok {
						return "", fmt.Errorf("no SAC data found for technician : %s", technician)
					}

					if err := processSPForSPLNotSOWithExistingEDC(
						db,
						forProject,
						hrdPersonaliaName,
						hrdTTDPath,
						hrdPhoneNumber,
						technician,
						namaSPL,
						splCity,
						SACData,
						resignTechnicianReplacer,
						audioDirForSPSPL,
						pdfDirForSPSPL,
						needToSendTheSPSPLThroughWhatsapp,
						productsEDCCSNAExists,
					); err != nil {
						logrus.Errorf("got error while trying to process SP of SPL %s coz not SO while still had existing EDC : %v", technician, err)
						continue
					}
				} // .end of SP of SPL
			} // .end of if there is existing edc for the technician in ProductEDCCSNA

			continue // continue to other technician coz got not SO today
		} // .end of got error while trying to get the technician doing SO today or not

		if stockOpnameTechnician == nil {
			logrus.Infof("No Stock Opname data for technician %s today", technician)
			continue
		}

		// Group Stock Opname data by company
		companySOData := make(map[string][]DataStockOpnameAggregate)
		for _, dataSO := range stockOpnameTechnician {
			companySO := dataSO.Company

			// Check for duplicates
			exists := false
			for _, existing := range companySOData[companySO] {
				if existing.ID == dataSO.ID {
					exists = true
					break
				}
			}

			if !exists {
				companySOData[companySO] = append(companySOData[companySO], dataSO)
			}
		} // .end of for looping the so technician data to make new map for each company its so data

		if len(companySOData) == 0 {
			logrus.Infof("No Stock Opname data after grouping by company for technician %s today", technician)
			continue
		}

		missingEDCNotSO := make(map[string][]string) // map of company to list of missing edc not so by the technician

		for companyFromSO, dataSOList := range companySOData {
			locSOTechnician, ok := data.TechnicianInventoryLocation[companyFromSO]
			if !ok {
				logrus.Errorf("No inventory location mapping for company %s of technician %s", companyFromSO, technician)
				continue
			}

			var productEDCCSNA []stockopnamemodel.ProductEDCCSNA
			if err := db.Where("product_category = ?", "EDC").
				Where("from_location = ?", locSOTechnician).
				Where("to_location = ?", locSOTechnician).
				Where("company = ?", companyFromSO).
				Find(&productEDCCSNA).
				Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					logrus.Warnf("No ProductEDCCSNA found for processing SO of technician %s for company %s", technician, companyFromSO)
					continue
				}
				logrus.Errorf("Failed to query ProductEDCCSNA for processing SO of technician %s for company %s: %v", technician, companyFromSO, err)
				continue
			}

			for _, dataSOInList := range dataSOList {
				// Check for missing EDC in Stock Opname details
				for _, edcItem := range productEDCCSNA {
					found := false

					if len(dataSOInList.DetailOperationsAPK) > 0 {
						for _, detailOPAPK := range dataSOInList.DetailOperationsAPK {
							if strings.TrimSpace(detailOPAPK.SN) == strings.TrimSpace(edcItem.SN_EDC) {
								found = true
								break
							}
						} // .end of for looping each detail operation apk in the so data
					} // .end of if there is detail operation apk in the so data

					if !found {
						if _, exists := missingEDCNotSO[companyFromSO]; !exists {
							missingEDCNotSO[companyFromSO] = []string{}
						}
						missingEDCNotSO[companyFromSO] = append(missingEDCNotSO[companyFromSO], fmt.Sprintf("%s (%s)", edcItem.SN_EDC, edcItem.Product))
						missingEDCItems = append(missingEDCItems, StockOpnameReportItem{
							Technician: technician,
							SN:         edcItem.SN_EDC,
							Product:    edcItem.Product,
							Company:    companyFromSO,
							Location:   locSOTechnician,
						})
					}
				} // .end of for looping each edc item in product edcc sna
			} // .end of for looping each SO data in the company's SO data list
		} // .end of for looping each company existing in so technician data

		if len(missingEDCNotSO) > 0 {
			if !isSPL {
				splData, exists := TechODOOMSData[data.SPL]
				if !exists {
					logrus.Errorf("no SPL found for technician %s", technician)
					continue
				}
				var namaSPL, splCity string
				if splData.Name != "" {
					namaSPL = splData.Name
				} else {
					namaSPL = data.SPL
				}
				splCity = getSPLCity(data.SPL)
				if splCity == "" {
					splCity = "Unknown"
				}

				ODOOMSSAC := config.WebPanel.Get().ODOOMSSAC
				if len(ODOOMSSAC) == 0 {
					return "", errors.New("no data found for SAC ODOO Manage Service")
				}
				SACDataTechnician, ok := ODOOMSSAC[data.SAC]
				if !ok {
					return "", fmt.Errorf("no SAC data found for technician : %s", technician)
				}

				if err := processSPForTechnicianWithMissingEDCNotSO(
					db,
					forProject,
					hrdPersonaliaName,
					hrdTTDPath,
					hrdPhoneNumber,
					technician,
					namaTeknisi,
					namaSPL,
					splCity,
					SACDataTechnician,
					resignTechnicianReplacer,
					audioDirForSPTechnician,
					pdfDirForSPTechnician,
					needToSendTheSPTechnicianThroughWhatsapp,
					missingEDCNotSO,
				); err != nil {
					logrus.Errorf("got error while trying to process SP of technician %s coz had missing EDC not SO : %v", technician, err)
					continue
				}
				// .end of SP of technician
			} else {
				splData, exists := TechODOOMSData[data.SPL]
				if !exists {
					logrus.Errorf("no SPL found for technician %s", technician)
					continue
				}
				var namaSPL, splCity string
				if splData.Name != "" {
					namaSPL = splData.Name
				} else {
					namaSPL = data.SPL
				}
				splCity = getSPLCity(data.SPL)
				if splCity == "" {
					splCity = "Unknown"
				}

				ODOOMSSAC := config.WebPanel.Get().ODOOMSSAC
				if len(ODOOMSSAC) == 0 {
					return "", errors.New("no data found for SAC ODOO Manage Service")
				}
				SACData, ok := ODOOMSSAC[data.SAC]
				if !ok {
					return "", fmt.Errorf("no SAC data found for technician : %s", technician)
				}

				if err := processSPForSPLWithMissingEDCNotSO(
					db,
					forProject,
					hrdPersonaliaName,
					hrdTTDPath,
					hrdPhoneNumber,
					technician,
					namaSPL,
					splCity,
					SACData,
					resignTechnicianReplacer,
					audioDirForSPSPL,
					pdfDirForSPSPL,
					needToSendTheSPSPLThroughWhatsapp,
					missingEDCNotSO,
				); err != nil {
					logrus.Errorf("got error while trying to process SP of SPL %s coz had missing EDC not SO : %v", technician, err)
					continue
				}
			} // .end of SP of SPL
		} // .end of if there is missing edc not so by the technician
	} // .end of for looping all technicians existing in ODOO

	return generateStockOpnameExcelReport(notSOItems, missingEDCItems, excelReportDirUsed)
}

// generateStockOpnameExcelReport generates an Excel report for Stock Opname discrepancies.
// It creates two sheets: "Not SO" for items not in Stock Opname and "Missing EDC Not SO" for missing EDC items.
//
// Parameters:
//   - notSOItems: A list of items that were not found in the Stock Opname.
//   - missingEDCItems: A list of EDC items that are missing and not in the Stock Opname.
//   - excelReportDirUsed: The directory path where the Excel report will be saved.
//
// Returns:
//   - string: The full file path of the generated Excel report.
//   - error: An error if the file creation or saving fails.
func generateStockOpnameExcelReport(notSOItems, missingEDCItems []StockOpnameReportItem, excelReportDirUsed string) (string, error) {
	f := excelize.NewFile()
	// Sheet 1: Not SO
	sheet1 := "Not SO"
	f.SetSheetName("Sheet1", sheet1)

	titlesNotSO := []struct {
		Title string
		Size  float64
	}{
		{"Technician", 35},
		{"EDC SN", 35},
		{"Product", 25},
		{"Company", 20},
		{"Stock Location", 50},
	}
	var columnsNotSO []ExcelColumn
	for i, t := range titlesNotSO {
		columnsNotSO = append(columnsNotSO, ExcelColumn{
			ColIndex: fun.GetColName(i),
			ColTitle: t.Title,
			ColSize:  t.Size,
		})
	}
	for _, col := range columnsNotSO {
		f.SetCellValue(sheet1, fmt.Sprintf("%s1", col.ColIndex), col.ColTitle)
		f.SetColWidth(sheet1, col.ColIndex, col.ColIndex, col.ColSize)
	}
	lastColNotSO := fun.GetColName(len(columnsNotSO) - 1)
	filterRangeNotSO := fmt.Sprintf("A1:%s1", lastColNotSO)
	f.AutoFilter(sheet1, filterRangeNotSO, []excelize.AutoFilterOptions{})

	notSORowIndex := 2
	for _, item := range notSOItems {
		for _, column := range columnsNotSO {
			cell := fmt.Sprintf("%s%d", column.ColIndex, notSORowIndex)
			var value interface{} = "N/A"
			needToSetValue := true

			switch column.ColTitle {
			case "Technician":
				value = item.Technician
			case "EDC SN":
				value = item.SN
			case "Product":
				value = item.Product
			case "Company":
				value = item.Company
			case "Stock Location":
				value = item.Location
			}

			if needToSetValue {
				f.SetCellValue(sheet1, cell, value)
			}
		}
		notSORowIndex++
	}

	// Sheet 2: Missing EDC
	sheet2 := "Missing EDC Not SO"
	f.NewSheet(sheet2)

	titlesMissingEDC := []struct {
		Title string
		Size  float64
	}{
		{"Technician", 35},
		{"EDC SN", 35},
		{"Product", 25},
		{"Company", 20},
		{"Stock Location", 50},
	}
	var columnsMissingEDC []ExcelColumn
	for i, t := range titlesMissingEDC {
		columnsMissingEDC = append(columnsMissingEDC, ExcelColumn{
			ColIndex: fun.GetColName(i),
			ColTitle: t.Title,
			ColSize:  t.Size,
		})
	}
	for _, col := range columnsMissingEDC {
		f.SetCellValue(sheet2, fmt.Sprintf("%s1", col.ColIndex), col.ColTitle)
		f.SetColWidth(sheet2, col.ColIndex, col.ColIndex, col.ColSize)
	}
	lastColMissingEDC := fun.GetColName(len(columnsMissingEDC) - 1)
	filterRangeMissingEDC := fmt.Sprintf("A1:%s1", lastColMissingEDC)
	f.AutoFilter(sheet2, filterRangeMissingEDC, []excelize.AutoFilterOptions{})

	missingEDCRowIndex := 2
	for _, item := range missingEDCItems {
		for _, column := range columnsMissingEDC {
			cell := fmt.Sprintf("%s%d", column.ColIndex, missingEDCRowIndex)
			var value interface{} = "N/A"
			needToSetValue := true

			switch column.ColTitle {
			case "Technician":
				value = item.Technician
			case "EDC SN":
				value = item.SN
			case "Product":
				value = item.Product
			case "Company":
				value = item.Company
			case "Stock Location":
				value = item.Location
			}

			if needToSetValue {
				f.SetCellValue(sheet2, cell, value)
			}
		}
		missingEDCRowIndex++
	}

	// Create pivot
	if len(notSOItems) > 0 {
		sheetPivotNotSO := "PIVOT - Not SO"
		f.NewSheet(sheetPivotNotSO)
		pivotNotSODataRange := fmt.Sprintf("%s!$A$1:$%s$%d", sheet1, lastColNotSO, notSORowIndex-1)
		pivotRange := fmt.Sprintf("%s!A8:E20000", sheetPivotNotSO)
		err := f.AddPivotTable(&excelize.PivotTableOptions{
			Name:            sheetPivotNotSO,
			DataRange:       pivotNotSODataRange,
			PivotTableRange: pivotRange,
			Rows: []excelize.PivotTableField{
				{Data: "Technician"},
			},
			Data: []excelize.PivotTableField{
				{Data: "EDC SN", Subtotal: "count"},
			},
			Filter: []excelize.PivotTableField{
				{Data: "Product"},
				{Data: "Company"},
				{Data: "Stock Location"},
			},
			RowGrandTotals:      true,
			ColGrandTotals:      true,
			ShowDrill:           true,
			ShowRowHeaders:      true,
			ShowColHeaders:      true,
			ShowLastColumn:      true,
			PivotTableStyleName: "PivotStyleLight10", // Set your desired style here
		})
		if err != nil {
			logrus.Errorf("failed to create pivot table for Not SO sheet: %v", err)
		}
	}

	if len(missingEDCItems) > 0 {
		sheetPivotMissingEDC := "PIVOT - Missing EDC Not SO"
		f.NewSheet(sheetPivotMissingEDC)
		pivotMissingEDCDataRange := fmt.Sprintf("%s!$A$1:$%s$%d", sheet2, lastColMissingEDC, missingEDCRowIndex-1)
		pivotRange := fmt.Sprintf("%s!A8:E20000", sheetPivotMissingEDC)
		err := f.AddPivotTable(&excelize.PivotTableOptions{
			Name:            sheetPivotMissingEDC,
			DataRange:       pivotMissingEDCDataRange,
			PivotTableRange: pivotRange,
			Rows: []excelize.PivotTableField{
				{Data: "Technician"},
			},
			Data: []excelize.PivotTableField{
				{Data: "EDC SN", Subtotal: "count"},
			},
			Filter: []excelize.PivotTableField{
				{Data: "Product"},
				{Data: "Company"},
				{Data: "Stock Location"},
			},
			RowGrandTotals:      true,
			ColGrandTotals:      true,
			ShowDrill:           true,
			ShowRowHeaders:      true,
			ShowColHeaders:      true,
			ShowLastColumn:      true,
			PivotTableStyleName: "PivotStyleLight10", // Set your desired style here
		})
		if err != nil {
			logrus.Errorf("failed to create pivot table for Missing EDC Not SO sheet: %v", err)
		}
	}

	fileName := fmt.Sprintf("Stock_Opname_%s.xlsx", time.Now().Format("02Jan2006"))
	fullFilePath := filepath.Join(excelReportDirUsed, fileName)
	if err := f.SaveAs(fullFilePath); err != nil {
		return "", fmt.Errorf("failed to save excel file: %v", err)
	}

	return fullFilePath, nil
}

// getStockOpnameOfTechnicianToday retrieves the Stock Opname data for a specific technician
// for the current day from ODOO MS. It fetches stock picking records and aggregates
// relevant details including detailed operations.
//
// Parameters:
//   - technician: The name or ID of the technician.
//
// Returns:
//   - []DataStockOpnameAggregate: A list of aggregated Stock Opname data.
//   - error: An error if the data retrieval fails.
func getStockOpnameOfTechnicianToday(technician string) ([]DataStockOpnameAggregate, error) {
	loc, _ := time.LoadLocation(config.WebPanel.Get().Default.Timezone)
	timeNow := time.Now().In(loc)

	startTime := timeNow

	startOfDay := time.Date(timeNow.Year(), timeNow.Month(), timeNow.Day(), 0, 0, 0, 0, loc)
	endOfDay := time.Date(timeNow.Year(), timeNow.Month(), timeNow.Day(), 23, 59, 59, 0, loc)

	startOfDay = startOfDay.Add(-7 * time.Hour) // Adjust to UTC-7
	endOfDay = endOfDay.Add(-7 * time.Hour)     // Adjust to UTC-7

	startDateParam := startOfDay.Format("2006-01-02 15:04:05")
	endDateParam := endOfDay.Format("2006-01-02 15:04:05")

	if technician == "" {
		return nil, errors.New("technician parameter is empty")
	}

	var originToSearch string
	isSourceDocumentTechnician := true

	if strings.Contains(strings.ToLower(technician), "spl") {
		isSourceDocumentTechnician = false
	}

	if isSourceDocumentTechnician {
		originToSearch = "stock opname[teknisi"
	} else {
		originToSearch = "stock opname[spl"
	}

	ODOOModel := "stock.picking"
	domain := []any{
		[]any{"technician_id_fs", "=", technician},
		[]any{"origin", "ilike", originToSearch},
		[]any{"create_date", ">=", startDateParam},
		[]any{"create_date", "<=", endDateParam},
		// []any{"scheduled_date", ">=", startDateParam},
		// []any{"scheduled_date", "<=", endDateParam},
	}

	fieldID := []string{"id"}
	fields := []string{
		"id",
		"name",
		"partner_id",
		"picking_type_id",
		"location_id",
		"location_dest_id",
		"company_id",
		"user_id",
		"technician_id",
		"technician_id_fs",
		"scheduled_date",
		"origin",
		"location_dest_categ",
		"carrier",
		"tracking_number",
		"x_link",
		"state",
		"move_line_ids_without_package",
	}
	order := "id asc"
	odooParams := map[string]any{
		"model":  ODOOModel,
		"domain": domain,
		"fields": fieldID,
		"order":  order,
	}

	payload := map[string]any{
		"jsonrpc": config.WebPanel.Get().ApiODOO.JSONRPC,
		"params":  odooParams,
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal payload for getStockOpnameOfTechnicianToday of technician %s: %v", technician, err)
	}

	var ids []uint64

	ODOOResp, err := GetODOOMSData(string(payloadBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to get ODOO MS data for getStockOpnameOfTechnicianToday of technician %s: %v", technician, err)
	}

	ODOORespArray, ok := ODOOResp.([]any)
	if !ok {
		logrus.Errorf("invalid ODOO MS response format for getStockOpnameOfTechnicianToday of technician %s", technician)
	} else {
		ids = extractUniqueIDs(ODOORespArray)
	}

	if len(ids) == 0 {
		logrus.Infof("No data found with scheduled_date, trying with create_date for technician %s", technician)

		// Delete scheduled_date params (last 2 items) and add create_date params
		domain = append(domain[:2],
			[]any{"create_date", ">=", startDateParam},
			[]any{"create_date", "<=", endDateParam},
		)

		odooParams["domain"] = domain
		payload["params"] = odooParams

		payloadBytes, err = json.Marshal(payload)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal payload for retry with create_date: %v", err)
		}

		ODOOResp, err = GetODOOMSData(string(payloadBytes))
		if err != nil {
			return nil, fmt.Errorf("failed to get ODOO MS data for retry with create_date: %v", err)
		}

		ODOORespArray, ok = ODOOResp.([]any)
		if !ok {
			return nil, fmt.Errorf("invalid ODOO MS response format for retry with create_date")
		}

		ids = extractUniqueIDs(ODOORespArray)

		if len(ids) == 0 {
			return nil, fmt.Errorf("no Stock Opname data found in ODOO MS for technician %s today", technician)
		}
	}

	const batchSize = 100
	chunks := chunkIdsSlice(ids, batchSize)
	var allRecords []any

	// Use workers to process chunks concurrently
	type chunkRes struct {
		records []any
		err     error
		index   int
	}

	resChan := make(chan chunkRes, len(chunks))
	semaphore := make(chan struct{}, 5) // Limit to 5 concurrent workers

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()

	for i, chunk := range chunks {
		go func(chunkIndex int, chunkData []uint64) {
			defer func() {
				if r := recover(); r != nil {
					logrus.Errorf("Panic in chunk processing %d: %v", chunkIndex, r)
					resChan <- chunkRes{nil, fmt.Errorf("panic in chunk %d: %v", chunkIndex, r), chunkIndex}
				}
			}()

			select {
			case semaphore <- struct{}{}: // Acquire semaphore with timeout
			case <-ctx.Done():
				resChan <- chunkRes{nil, fmt.Errorf("chunk %d timeout", chunkIndex), chunkIndex}
				return
			}
			defer func() { <-semaphore }() // Release semaphore

			// logrus.Debugf("Processing (%s) chunk %d of %d (IDs %v to %v)", taskDoing, chunkIndex+1, len(chunks), chunkData[0], chunkData[len(chunkData)-1])

			chunkDomain := []interface{}{
				[]interface{}{"id", "=", chunkData},
			}

			odooParams := map[string]interface{}{
				"model":  ODOOModel,
				"domain": chunkDomain,
				"fields": fields,
				"order":  order,
			}

			payload := map[string]interface{}{
				"jsonrpc": config.WebPanel.Get().ApiODOO.JSONRPC,
				"params":  odooParams,
			}

			payloadBytes, err := json.Marshal(payload)
			if err != nil {
				resChan <- chunkRes{nil, fmt.Errorf("failed to marshal payload for chunk %d: %v", chunkIndex+1, err), chunkIndex}
				return
			}

			ODOOresponse, err := GetODOOMSData(string(payloadBytes))
			if err != nil {
				resChan <- chunkRes{nil, fmt.Errorf("failed fetching data from ODOO MS API for chunk %d: %v", chunkIndex+1, err), chunkIndex}
				return
			}

			ODOOResponseArray, ok := ODOOresponse.([]interface{})
			if !ok {
				resChan <- chunkRes{nil, fmt.Errorf("type assertion failed for chunk %d", chunkIndex+1), chunkIndex}
				return
			}

			resChan <- chunkRes{ODOOResponseArray, nil, chunkIndex}
		}(i, chunk)
	}

	// Collect results from all goroutines with timeout protection
	results := make([]chunkRes, len(chunks))
	for i := 0; i < len(chunks); i++ {
		select {
		case result := <-resChan:
			if result.index < len(results) {
				results[result.index] = result
			}
		case <-ctx.Done():
			logrus.Errorf("Timeout waiting for chunk results")
			return nil, fmt.Errorf("timeout waiting for chunk results in getStockOpnameOfTechnicianToday of technician %s", technician)
		}
	}

	// Process results in order and handle errors gracefully
	for i, result := range results {
		if result.err != nil {
			logrus.Errorf("Error processing chunk %d: %v", i, result.err)
			continue // Continue with other chunks instead of failing completely
		}
		if result.records != nil {
			// logrus.Debugf("Appending %d records from chunk %d", len(result.records), i)
			allRecords = append(allRecords, result.records...)
		}
	}

	if len(allRecords) == 0 {
		return nil, fmt.Errorf("no valid Stock Opname data retrieved from ODOO MS for technician %s today", technician)
	}

	ODOOResponseBytes, err := json.Marshal(allRecords)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal ODOO MS response for getStockOpnameOfTechnicianToday of technician %s: %v", technician, err)
	}

	var listOfData []OdooStockPickingItem
	if err := json.Unmarshal(ODOOResponseBytes, &listOfData); err != nil {
		return nil, fmt.Errorf("failed to unmarshal ODOO MS data for getStockOpnameOfTechnicianToday of technician %s: %v", technician, err)
	}

	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)
	logrus.Infof("Memory Usage during getStockOpnameOfTechnicianToday for %s: Alloc = %v MiB, TotalAlloc = %v MiB, Sys = %v MiB, NumGC = %v",
		technician,
		memStats.Alloc/1024/1024,
		memStats.TotalAlloc/1024/1024,
		memStats.Sys/1024/1024,
		memStats.NumGC,
	)
	runtime.GC() // Force garbage collection to free up memory

	var dataOfStockOpname []DataStockOpnameAggregate

	for _, odooData := range listOfData {
		var scheduledDate *time.Time
		if odooData.ScheduledDate.String != "" {
			parsedTime, err := time.Parse("2006-01-02 15:04:05", odooData.ScheduledDate.String)
			if err != nil {
				logrus.Errorf("Failed to parse scheduled_date for Stock Opname ID %d: %v", odooData.ID, err)
			} else {
				parsedTime = parsedTime.Add(7 * time.Hour)
				scheduledDate = &parsedTime
			}
		}

		_, responsible := parseJSONIDDataCombinedSafe(odooData.Responsible)
		_, sourceLoc := parseJSONIDDataCombinedSafe(odooData.SourceLocation)
		_, destLoc := parseJSONIDDataCombinedSafe(odooData.DestionationLocation)
		_, company := parseJSONIDDataCombinedSafe(odooData.Company)

		dataSO := DataStockOpnameAggregate{
			ID:                odooData.ID,
			TicketSO:          odooData.Name.String,
			ScheduledDate:     scheduledDate,
			Responsible:       responsible,
			SourceDocument:    odooData.SourceDocument.String,
			SourceLocation:    sourceLoc,
			LocDestinationCat: odooData.LocationDestinationCategory.String,
			DestLocation:      destLoc,
			Company:           company,
			LinkBAST:          odooData.LinkBast.String,
			Status:            odooData.Status.String,
		}

		var detailedOperationAPKIDs []int
		if odooData.DetailedOperationAPKIDs.Valid {
			detailedOperationAPKIDs = odooData.DetailedOperationAPKIDs.Ints
		}

		if len(detailedOperationAPKIDs) > 0 {
			dataDetailedOperationsAPK, err := getStockMoveLineDetailedOperationsAPK(detailedOperationAPKIDs)
			if err != nil {
				logrus.Errorf("Failed to get detailed operations APK for Stock Opname ID %d: %v", odooData.ID, err)
			} else {
				dataSO.DetailOperationsAPK = dataDetailedOperationsAPK
			}
		} // .end of existing data of operations APK in stock picking

		if !containsSO(dataOfStockOpname, odooData.ID) {
			dataOfStockOpname = append(dataOfStockOpname, dataSO)
		}
	}

	if len(dataOfStockOpname) > 0 {
		return dataOfStockOpname, nil
	}

	totalDuration := time.Since(startTime)
	logrus.Infof("getStockOpnameOfTechnicianToday for %s completed in %v", technician, totalDuration)

	return nil, fmt.Errorf("no Stock Opname data found for technician %s today after processing", technician)
}

// getStockMoveLineDetailedOperationsAPK fetches detailed operation lines for stock moves
// from ODOO MS based on a list of IDs. It returns a list of DetailedOperationsAPKStockOpname
// containing product, serial number, status, and location information.
//
// Parameters:
//   - listID: A slice of integer IDs for the stock move lines.
//
// Returns:
//   - []DetailedOperationsAPKStockOpname: A list of detailed operation data.
//   - error: An error if the data retrieval fails.
func getStockMoveLineDetailedOperationsAPK(listID []int) ([]DetailedOperationsAPKStockOpname, error) {
	ODOOModel := "stock.move.line"
	fieldID := []string{"id"}
	fields := []string{
		"id",
		"product_id",
		"lot_id",
		"x_status",
		"location_id",
		"location_dest_id",
	}

	domain := []any{
		[]any{"id", "=", listID},
	}
	order := "id desc"
	odooParams := map[string]any{
		"model":  ODOOModel,
		"domain": domain,
		"fields": fieldID,
		"order":  order,
	}

	payload := map[string]any{
		"jsonrpc": config.WebPanel.Get().ApiODOO.JSONRPC,
		"params":  odooParams,
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal payload for getStockMoveLineDetailedOperationsAPK: %v", err)
	}

	var ids []uint64
	ODOOResp, err := GetODOOMSData(string(payloadBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to get ODOO MS data for getStockMoveLineDetailedOperationsAPK: %v", err)
	}

	ODOORespArray, ok := ODOOResp.([]any)
	if !ok {
		return nil, fmt.Errorf("invalid ODOO MS response format for getStockMoveLineDetailedOperationsAPK")
	} else {
		ids = extractUniqueIDs(ODOORespArray)
	}

	if len(ids) == 0 {
		return nil, fmt.Errorf("no data found in ODOO MS for getStockMoveLineDetailedOperationsAPK")
	}

	const batchSize = 100
	chunks := chunkIdsSlice(ids, batchSize)
	var allRecords []any

	// Use workers to process chunks concurrently
	type chunkRes struct {
		records []any
		err     error
		index   int
	}

	resChan := make(chan chunkRes, len(chunks))
	semaphore := make(chan struct{}, 5) // Limit to 5 concurrent workers

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()

	for i, chunk := range chunks {
		go func(chunkIndex int, chunkData []uint64) {
			defer func() {
				if r := recover(); r != nil {
					logrus.Errorf("Panic in chunk processing %d: %v", chunkIndex, r)
					resChan <- chunkRes{nil, fmt.Errorf("panic in chunk %d: %v", chunkIndex, r), chunkIndex}
				}
			}()

			select {
			case semaphore <- struct{}{}: // Acquire semaphore with timeout
			case <-ctx.Done():
				resChan <- chunkRes{nil, fmt.Errorf("chunk %d timeout", chunkIndex), chunkIndex}
				return
			}
			defer func() { <-semaphore }() // Release semaphore

			// logrus.Debugf("Processing (%s) chunk %d of %d (IDs %v to %v)", taskDoing, chunkIndex+1, len(chunks), chunkData[0], chunkData[len(chunkData)-1])

			chunkDomain := []interface{}{
				[]interface{}{"id", "=", chunkData},
			}

			odooParams := map[string]interface{}{
				"model":  ODOOModel,
				"domain": chunkDomain,
				"fields": fields,
				"order":  order,
			}

			payload := map[string]interface{}{
				"jsonrpc": config.WebPanel.Get().ApiODOO.JSONRPC,
				"params":  odooParams,
			}

			payloadBytes, err := json.Marshal(payload)
			if err != nil {
				resChan <- chunkRes{nil, fmt.Errorf("failed to marshal payload for chunk %d: %v", chunkIndex+1, err), chunkIndex}
				return
			}

			ODOOresponse, err := GetODOOMSData(string(payloadBytes))
			if err != nil {
				resChan <- chunkRes{nil, fmt.Errorf("failed fetching data from ODOO MS API for chunk %d: %v", chunkIndex+1, err), chunkIndex}
				return
			}

			ODOOResponseArray, ok := ODOOresponse.([]interface{})
			if !ok {
				resChan <- chunkRes{nil, fmt.Errorf("type assertion failed for chunk %d", chunkIndex+1), chunkIndex}
				return
			}

			resChan <- chunkRes{ODOOResponseArray, nil, chunkIndex}
		}(i, chunk)
	}

	// Collect results from all goroutines with timeout protection
	results := make([]chunkRes, len(chunks))
	for i := 0; i < len(chunks); i++ {
		select {
		case result := <-resChan:
			if result.index < len(results) {
				results[result.index] = result
			}
		case <-ctx.Done():
			logrus.Errorf("Timeout waiting for chunk results")
			return nil, fmt.Errorf("timeout waiting for chunk results in getStockMoveLineDetailedOperationsAPK")
		}
	}

	// Process results in order and handle errors gracefully
	for i, result := range results {
		if result.err != nil {
			logrus.Errorf("Error processing chunk %d: %v", i, result.err)
			continue // Continue with other chunks instead of failing completely
		}
		if result.records != nil {
			// logrus.Debugf("Appending %d records from chunk %d", len(result.records), i)
			allRecords = append(allRecords, result.records...)
		}
	}

	if len(allRecords) == 0 {
		return nil, fmt.Errorf("no valid Stock Move Line Detailed Operations APK data retrieved from ODOO MS")
	}

	ODOOResponseBytes, err := json.Marshal(allRecords)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal ODOO MS response for getStockMoveLineDetailedOperationsAPK: %v", err)
	}

	var listOfData []StockMoveLineItem
	if err := json.Unmarshal(ODOOResponseBytes, &listOfData); err != nil {
		return nil, fmt.Errorf("failed to unmarshal ODOO MS data for getStockMoveLineDetailedOperationsAPK: %v", err)
	}

	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)
	logrus.Infof("Memory Usage during getStockMoveLineDetailedOperationsAPK: Alloc = %v MiB, TotalAlloc = %v MiB, Sys = %v MiB, NumGC = %v",
		memStats.Alloc/1024/1024,
		memStats.TotalAlloc/1024/1024,
		memStats.Sys/1024/1024,
		memStats.NumGC,
	)
	runtime.GC() // Force garbage collection to free up memory

	returnDataOfDetailedOperationsAPKSO := make([]DetailedOperationsAPKStockOpname, 0)
	seenSNs := make(map[string]bool)

	for _, odooData := range listOfData {
		var detailedOpAPK DetailedOperationsAPKStockOpname

		_, productName := parseJSONIDDataCombinedSafe(odooData.Product)
		_, lotName := parseJSONIDDataCombinedSafe(odooData.SN)
		detailedOpAPK.Product = productName

		if lotName != "" {
			if seenSNs[lotName] {
				logrus.Infof("Duplicate SN %s found in Detailed Operations APK, skipping", lotName)
				continue
			}
			seenSNs[lotName] = true
		}

		detailedOpAPK.SN = lotName
		detailedOpAPK.Status = odooData.Status.String

		_, locationName := parseJSONIDDataCombinedSafe(odooData.FromLocation)
		detailedOpAPK.From = locationName

		_, destLocationName := parseJSONIDDataCombinedSafe(odooData.ToLocation)
		detailedOpAPK.To = destLocationName

		returnDataOfDetailedOperationsAPKSO = append(returnDataOfDetailedOperationsAPKSO, detailedOpAPK)
	}

	return returnDataOfDetailedOperationsAPKSO, nil
}

func generateMJMLTemplateForReportSOWithSPOrWarningLetter(useFor string, spTo string, spNumber int, recipient string, message string) (string, string) {
	if message == "" || recipient == "" || useFor == "" {
		return "", ""
	}

	var emailTitle, emailSubject string
	if useFor == "report_so" {
		emailTitle = "Report SO (With technicians got SP)"
		emailSubject = fmt.Sprintf("Report SO - %s", time.Now().Format("02 Jan 2006"))
	} else {
		emailTitle = fmt.Sprintf("Surat Peringatan %d untuk Saudara(i) %s", spNumber, spTo)
		emailSubject = fmt.Sprintf("SP %d - %s (%s)", spNumber, spTo, time.Now().Format("02 Jan 2006"))
	}

	var sb strings.Builder
	sb.WriteString("<mjml>")
	sb.WriteString(fmt.Sprintf(`
			<mj-head>
				<mj-preview>%s</mj-preview>
				<mj-style inline="inline">
				.body-section {
					background-color: #ffffff;
					padding: 30px;
					border-radius: 12px;
					box-shadow: 0 2px 8px rgba(0, 0, 0, 0.08);
				}
				.footer-text {
					color: #6b7280;
					font-size: 12px;
					text-align: center;
					padding-top: 10px;
					border-top: 1px solid #e5e7eb;
				}
				.header-title {
					font-size: 66px;
					font-weight: bold;
					color: #1E293B;
					text-align: left;
				}
				.cta-button {
					background-color: #6D28D9;
					color: #ffffff;
					padding: 12px 24px;
					border-radius: 8px;
					font-size: 16px;
					font-weight: bold;
					text-align: center;
					display: inline-block;
				}
				.email-info {
					color: #374151;
					font-size: 16px;
				}
				</mj-style>
			</mj-head>`,
		emailTitle,
	))

	sb.WriteString(fmt.Sprintf(`
			<mj-body background-color="#f8fafc">
				<!-- Main Content -->
				<mj-section css-class="body-section" padding="20px">
				<mj-column>
					<mj-text font-size="20px" color="#1E293B" font-weight="bold">Yth. Sdr(i) %v</mj-text>
					<mj-text font-size="16px" color="#4B5563" line-height="1.6">
						%s
					</mj-text>

					<mj-divider border-color="#e5e7eb"></mj-divider>

					<mj-text font-size="16px" color="#374151">
					Best Regards,<br>
					<b><i>%v</i></b>
					</mj-text>
				</mj-column>
				</mj-section>

				<!-- Footer -->
				<mj-section>
				<mj-column>
					<mj-text css-class="footer-text">
					⚠ This is an automated email. Please do not reply directly.
					</mj-text>
					<mj-text font-size="12px" color="#6b7280">
					<b>Service Report.</b><br>
					<!--
					<br>
					<a href="wa.me/%s">
					📞 Support
					</a>
					-->
					</mj-text>
				</mj-column>
				</mj-section>

			</mj-body>
			`,
		strings.ToUpper(recipient),
		message,
		config.WebPanel.Get().Default.PT,
		config.WebPanel.Get().Whatsmeow.WaTechnicalSupport,
	))
	sb.WriteString("</mjml>")

	mjmlTemplate := sb.String()
	return mjmlTemplate, emailSubject
}

func GenerateSPStockOpnameRepliedExcel(db *gorm.DB, dateFilter string) (string, error) {
	// Find SPWhatsAppMessage with reply text not empty, for_project = "ODOO MS", and no sac_got_sp_id
	query := db.Where("whatsapp_reply_text != '' AND for_project = ? AND sac_got_sp_id IS NULL", "ODOO MS")
	if dateFilter != "" {
		query = query.Where("DATE(whatsapp_sent_at) = ?", dateFilter)
	}
	var messages []sptechnicianmodel.SPWhatsAppMessage
	if err := query.Find(&messages).Error; err != nil {
		return "", fmt.Errorf("failed to query SPWhatsAppMessage: %v", err)
	}

	if len(messages) == 0 {
		if dateFilter != "" {
			return "", fmt.Errorf("no SP WhatsApp messages with replies found for ODOO MS project on date %s", dateFilter)
		}
		return "", fmt.Errorf("no SP WhatsApp messages with replies found for ODOO MS project")
	}

	// Prepare data for Excel
	type ReportItem struct {
		Type        string
		Name        string
		SPNumber    int
		Pelanggaran string
		GotSPAt     *time.Time
		PhoneNumber string
		ReplyText   string
		SPURL       string
	}

	var reportItems []ReportItem

	// Process each message
	for _, msg := range messages {
		var name string
		var filePath string
		var gotSPAt *time.Time
		var pelanggaran string
		var spType string

		if msg.TechnicianGotSPID != nil {
			var tech sptechnicianmodel.TechnicianGotSP
			if err := db.First(&tech, *msg.TechnicianGotSPID).Error; err != nil {
				continue
			}
			name = tech.Technician
			spType = msg.WhatSP
			switch msg.NumberOfSP {
			case 1:
				if strings.Contains(strings.ToLower(tech.PelanggaranSP1), "stock opname") {
					pelanggaran = tech.PelanggaranSP1
					gotSPAt = tech.GotSP1At
					filePath = tech.SP1FilePath
				}
			case 2:
				if strings.Contains(strings.ToLower(tech.PelanggaranSP2), "stock opname") {
					pelanggaran = tech.PelanggaranSP2
					gotSPAt = tech.GotSP2At
					filePath = tech.SP2FilePath
				}
			case 3:
				if strings.Contains(strings.ToLower(tech.PelanggaranSP3), "stock opname") {
					pelanggaran = tech.PelanggaranSP3
					gotSPAt = tech.GotSP3At
					filePath = tech.SP3FilePath
				}
			}
		} else if msg.SPLGotSPID != nil {
			var spl sptechnicianmodel.SPLGotSP
			if err := db.First(&spl, *msg.SPLGotSPID).Error; err != nil {
				continue
			}
			name = spl.SPL
			spType = msg.WhatSP
			switch msg.NumberOfSP {
			case 1:
				if strings.Contains(strings.ToLower(spl.PelanggaranSP1), "stock opname") {
					pelanggaran = spl.PelanggaranSP1
					gotSPAt = spl.GotSP1At
					filePath = spl.SP1FilePath
				}
			case 2:
				if strings.Contains(strings.ToLower(spl.PelanggaranSP2), "stock opname") {
					pelanggaran = spl.PelanggaranSP2
					gotSPAt = spl.GotSP2At
					filePath = spl.SP2FilePath
				}
			case 3:
				if strings.Contains(strings.ToLower(spl.PelanggaranSP3), "stock opname") {
					pelanggaran = spl.PelanggaranSP3
					gotSPAt = spl.GotSP3At
					filePath = spl.SP3FilePath
				}
			}
		}

		if pelanggaran != "" {
			spURL := ""
			if filePath != "" {
				// Clean the file path by removing the base directory
				filePath = strings.ReplaceAll(filePath, "web/file/sp_technician/", "")
				filePath = strings.ReplaceAll(filePath, "web/file/sp_spl/", "")
				// Determine the proxy path based on SP type
				proxyPath := "/proxy-pdf-sp-technician/"
				if msg.SPLGotSPID != nil {
					proxyPath = "/proxy-pdf-sp-spl/"
				}
				spURL = config.WebPanel.Get().App.WebPublicURL + proxyPath + filePath
			}
			reportItems = append(reportItems, ReportItem{
				Type:        spType,
				Name:        name,
				SPNumber:    int(msg.NumberOfSP),
				Pelanggaran: pelanggaran,
				GotSPAt:     gotSPAt,
				PhoneNumber: msg.WhatsappMessageSentTo,
				ReplyText:   msg.WhatsappReplyText,
				SPURL:       spURL,
			})
		}
	}

	// Generate Excel
	f := excelize.NewFile()
	sheet1 := "Stock Opname SP Replied"
	f.SetSheetName("Sheet1", sheet1)

	titlesHeader := []struct {
		Title string
		Width float64
	}{
		{"Type", 15},
		{"Name", 30},
		{"SP Number", 12},
		{"Pelanggaran", 50},
		{"Got SP At", 20},
		{"Phone Number", 20},
		{"Reply Text", 50},
		{"SP URL", 50},
	}
	var columnsHeader []ExcelColumn
	for i, t := range titlesHeader {
		columnsHeader = append(columnsHeader, ExcelColumn{
			ColIndex: fun.GetColName(i),
			ColTitle: t.Title,
			ColSize:  t.Width,
		})
	}
	for _, col := range columnsHeader {
		f.SetCellValue(sheet1, fmt.Sprintf("%s1", col.ColIndex), col.ColTitle)
		f.SetColWidth(sheet1, col.ColIndex, col.ColIndex, col.ColSize)
	}
	lastColHeader := fun.GetColName(len(columnsHeader) - 1)
	filterRangeHeader := fmt.Sprintf("A1:%s1", lastColHeader)
	f.AutoFilter(sheet1, filterRangeHeader, []excelize.AutoFilterOptions{})

	rowIndex := 2
	for _, item := range reportItems {
		for _, column := range columnsHeader {
			cell := fmt.Sprintf("%s%d", column.ColIndex, rowIndex)
			var value any = "N/A"
			needToSetValue := true

			switch strings.ToLower(column.ColTitle) {
			case "type":
				value = item.Type
			case "name":
				value = item.Name
			case "sp number":
				value = item.SPNumber
			case "pelanggaran":
				value = item.Pelanggaran
			case "got sp at":
				if item.GotSPAt != nil {
					value = item.GotSPAt.Format("2006-01-02 15:04:05")
				} else {
					needToSetValue = false
				}
			case "phone number":
				value = item.PhoneNumber
			case "reply text":
				value = item.ReplyText
			case "sp url":
				if item.SPURL != "" {
					needToSetValue = false
					// Create hyperlink style (blue and underlined)
					linkStyle, _ := f.NewStyle(&excelize.Style{
						Font: &excelize.Font{
							Color:     "#0000FF",
							Underline: "single",
						},
					})
					f.SetCellHyperLink(sheet1, cell, item.SPURL, "External")
					value = "View PDF"
					f.SetCellValue(sheet1, cell, value)
					f.SetCellStyle(sheet1, cell, cell, linkStyle)
				}
			}

			if needToSetValue {
				f.SetCellValue(sheet1, cell, value)
			}
		}
		rowIndex++
	}

	// Find directory
	excelReportDir, err := fun.FindValidDirectory([]string{
		"web/file/excel_report",
		"../web/file/excel_report",
		"../../web/file/excel_report",
		"../../../web/file/excel_report",
	})
	if err != nil {
		return "", fmt.Errorf("failed to find valid directory for excel report: %v", err)
	}
	now := time.Now()
	excelReportDirUsed := filepath.Join(excelReportDir, now.Format("2006-01-02"))
	if err := os.MkdirAll(excelReportDirUsed, 0755); err != nil {
		return "", fmt.Errorf("failed to create directory: %v", err)
	}

	fileName := fmt.Sprintf("SP_Stock_Opname_Replied_%s_%d.xlsx", now.Format("02Jan2006"), now.Unix())
	fullFilePath := filepath.Join(excelReportDirUsed, fileName)
	if err := f.SaveAs(fullFilePath); err != nil {
		return "", fmt.Errorf("failed to save excel file: %v", err)
	}

	return fullFilePath, nil
}

// DownloadSPStockOpnameRepliedExcel is a Gin handler that generates and serves the SP Stock Opname Replied Excel report.
// It deletes the file after serving to save storage.
func DownloadSPStockOpnameRepliedExcel(c *gin.Context) {
	db := gormdb.Databases.Web
	if db == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database connection not available"})
		return
	}

	dateStr := c.Query("date")
	dateFilter := ""
	if dateStr != "" {
		parsedDate, err := fun.ParseFlexibleDate(dateStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid date format: " + err.Error()})
			return
		}
		dateFilter = parsedDate.Format("2006-01-02")
	}

	filePath, err := GenerateSPStockOpnameRepliedExcel(db, dateFilter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Serve the file
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filepath.Base(filePath)))
	c.File(filePath)

	// Delete the file after a delay to ensure download completes
	go func() {
		time.Sleep(5 * time.Minute) // Wait 5 minutes for download to complete
		if err := os.Remove(filePath); err != nil {
			logrus.Errorf("Failed to delete temporary Excel file %s: %v", filePath, err)
		}
	}()
}

// Get and send the SO report through the WhatsApp message
func GetReportOfStockOpname(v *events.Message, userLang string) {
	// eventToDO := "get generated SO report"
	stanzaID := v.Info.ID
	originalSenderJID := NormalizeSenderJID(v.Info.Sender.String())

	tz := config.WebPanel.Get().Default.Timezone
	timezoneLocation, err := time.LoadLocation(tz)
	if err != nil {
		logrus.Errorf("Failed to load timezone location %s: %v", tz, err)
		tz = "Asia/Jakarta"
		timezoneLocation, _ = time.LoadLocation(tz)
	}
	today := time.Now().In(timezoneLocation)
	todayFormatted := today.Format("02Jan2006")

	excelDir, err := fun.FindValidDirectory([]string{
		"web/file/excel_report",
		"../web/file/excel_report",
		"../../web/file/excel_report",
		"../../../web/file/excel_report",
	})
	if err != nil {
		id := "⚠️ Mohon maaf, terjadi kesalahan saat mencari direktori laporan Stock Opname."
		en := "⚠️ Sorry, an error occurred while locating the Stock Opname report directory."
		sendLangMessageWithStanza(v, stanzaID, originalSenderJID, id, en, userLang)
		return
	}
	excelDir = filepath.Join(excelDir, today.Format("2006-01-02"))

	excelFilename := fmt.Sprintf("Stock_Opname_%s.xlsx", todayFormatted)
	excelFilePath := filepath.Join(excelDir, excelFilename)
	if _, err := os.Stat(excelFilePath); os.IsNotExist(err) {
		id := fmt.Sprintf("⚠️ Mohon maaf, laporan Stock Opname (%v) untuk hari ini belum tersedia.", excelFilename)
		en := fmt.Sprintf("⚠️ Sorry, the Stock Opname report (%v) for today is not yet available.", excelFilename)
		sendLangMessageWithStanza(v, stanzaID, originalSenderJID, id, en, userLang)
		return
	}

	excelFileCaption := "Report Stock Opname " + todayFormatted
	SendExcelFileWithStanza(v, stanzaID, originalSenderJID, excelFilePath, excelFileCaption, nil, userLang)
}

// GetReportOfListSO is a Gin handler that serves the Stock Opname report Excel file for download.
func GetReportOfListSO() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Add timeout context
		ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
		defer cancel()
		c.Request = c.Request.WithContext(ctx)

		tz := config.WebPanel.Get().Default.Timezone
		timezoneLocation, err := time.LoadLocation(tz)
		if err != nil {
			logrus.Errorf("Failed to load timezone location %s: %v", tz, err)
			tz = "Asia/Jakarta"
			timezoneLocation, _ = time.LoadLocation(tz)
		}
		today := time.Now().In(timezoneLocation)
		todayFormatted := today.Format("02Jan2006")

		excelDir, err := fun.FindValidDirectory([]string{
			"web/file/excel_report",
			"../web/file/excel_report",
			"../../web/file/excel_report",
			"../../../web/file/excel_report",
		})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to locate the Stock Opname report directory"})
			return
		}
		excelDir = filepath.Join(excelDir, today.Format("2006-01-02"))

		excelFilename := fmt.Sprintf("Stock_Opname_%s.xlsx", todayFormatted)
		excelFilePath := filepath.Join(excelDir, excelFilename)

		// Check if file exists and is readable
		file, err := os.Open(excelFilePath)
		if err != nil {
			if os.IsNotExist(err) {
				c.JSON(http.StatusNotFound, gin.H{"error": fmt.Sprintf("The Stock Opname report (%s) for today is not yet available", excelFilename)})
			} else {
				c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to access the Stock Opname report (%s): %v", excelFilename, err)})
			}
			return
		}
		file.Close() // Close immediately after checking

		// Serve the file
		c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", excelFilename))
		c.File(excelFilePath)
	}
}
