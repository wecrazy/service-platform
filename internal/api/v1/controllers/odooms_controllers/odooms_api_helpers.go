package odoomscontrollers

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"service-platform/internal/config"
	odoomsmodel "service-platform/internal/core/model/odooms_model"
	"time"

	"github.com/sirupsen/logrus"
)

// CheckExistingTechnicianInODOOMS checks if a technician exists in ODOOMS based on name, email, or phone number
func CheckExistingTechnicianInODOOMS(name, email, phoneNumber string) (bool, error) {
	return CheckExistingTechnicianInODOOMSWithConfig(name, email, phoneNumber, config.GetConfig())
}

// checkExistingTechnicianInODOOMSWithConfig is the testable version that accepts config as parameter
func CheckExistingTechnicianInODOOMSWithConfig(name, email, phoneNumber string, cfg config.YamlConfig) (bool, error) {
	if name == "" && email == "" && phoneNumber == "" {
		return false, errors.New("at least one search parameter (name, email, or phoneNumber) must be provided")
	}

	odooModel := "fs.technician"

	conditions := []any{}

	if phoneNumber != "" {
		conditions = append(conditions,
			[]any{"x_no_telp", "ilike", phoneNumber},
		)
	}

	if name != "" {
		conditions = append(conditions,
			[]any{"x_technician_name", "ilike", name},
		)
	}

	if email != "" {
		conditions = append(conditions,
			[]any{"email", "ilike", email},
		)
	}

	odooDomain := []any{}

	if len(conditions) > 0 {
		// Add N-1 "|" operators
		for i := 0; i < len(conditions)-1; i++ {
			odooDomain = append(odooDomain, "|")
		}

		// Append all conditions
		odooDomain = append(odooDomain, conditions...)
	} else {
		// No search conditions provided
		return false, errors.New("at least one search parameter (name, email, or phoneNumber) must be provided")
	}

	odooFields := []string{
		"id",
		"email",
		"x_no_telp",
		"x_technician_name",
	}
	odooOrder := "id desc"

	odooParams := map[string]any{
		"model":  odooModel,
		"domain": odooDomain,
		"fields": odooFields,
		"order":  odooOrder,
	}

	payload := map[string]any{
		"jsonrpc": cfg.ODOOManageService.JsonRPCVersion,
		"params":  odooParams,
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return false, err
	}

	methodUsed := "POST"
	url := cfg.ODOOManageService.URL + cfg.ODOOManageService.PathGetData

	// Create a temporary helper for this request
	tempHelper := NewODOOMSAPIHelper(&cfg)

	// Get session cookies
	sessionCookies, err := tempHelper.GetODOOMSCookies(cfg.ODOOManageService.Login, cfg.ODOOManageService.Password)
	if err != nil {
		return false, fmt.Errorf("failed to get session cookies: %w", err)
	}

	// Make the request directly instead of using FetchODOOMS
	maxRetries := cfg.ODOOManageService.MaxRetry
	if maxRetries <= 0 {
		maxRetries = 3
	}

	retryDelay := cfg.ODOOManageService.RetryDelay
	if retryDelay <= 0 {
		retryDelay = 3
	}

	reqTimeout := time.Duration(cfg.ODOOManageService.DataTimeout) * time.Second
	if reqTimeout <= 0 {
		reqTimeout = 5 * time.Minute // 300 seconds default
	}

	tempHelper.client.Timeout = reqTimeout

	var lastErr error
	for attempts := 1; attempts <= maxRetries; attempts++ {
		request, err := http.NewRequest(methodUsed, url, bytes.NewBuffer(payloadBytes))
		if err != nil {
			return false, fmt.Errorf("failed to create request: %w", err)
		}
		request.Header.Set("Content-Type", "application/json")

		// Add session cookies to request
		for _, cookie := range sessionCookies {
			request.AddCookie(cookie)
		}

		response, err := tempHelper.client.Do(request)
		if err != nil {
			logrus.WithError(err).Warningf("Request failed (attempt %d/%d)", attempts, maxRetries)
			lastErr = err
			if attempts < maxRetries {
				time.Sleep(time.Duration(retryDelay) * time.Second)
			}
			continue
		}

		defer response.Body.Close()

		body, err := io.ReadAll(response.Body)
		if err != nil {
			return false, fmt.Errorf("failed to read response body: %w", err)
		}

		if response.StatusCode != http.StatusOK {
			logrus.WithField("status", response.StatusCode).Warningf("Bad response status: %d (attempt %d/%d)", response.StatusCode, attempts, maxRetries)
			if attempts < maxRetries {
				time.Sleep(time.Duration(retryDelay) * time.Second)
				continue
			}
			return false, fmt.Errorf("request failed with status: %d", response.StatusCode)
		}

		// Parse response
		var jsonResponse map[string]any
		err = json.Unmarshal(body, &jsonResponse)
		if err != nil {
			return false, err
		}

		if errorResponse, ok := jsonResponse["error"].(map[string]any); ok {
			if errorMessage, ok := errorResponse["message"].(string); ok && errorMessage == "Odoo Session Expired" {
				return false, errors.New("odoo Session Expired")
			} else {
				return false, fmt.Errorf("odoo Error: %v", errorResponse)
			}
		}

		if result, ok := jsonResponse["result"].(map[string]any); ok {
			if message, ok := result["message"].(string); ok {
				success, successOk := result["success"]
				logrus.Infof("ODOO MS Result, message: %v, status: %v", message, successOk && success == true)
			}
		}

		// Check for the existence and validity of the "result" field
		result, resultExists := jsonResponse["result"]
		if !resultExists {
			return false, fmt.Errorf("'result' field not found in the response: %v", jsonResponse)
		}

		// Check if the result is a map (with data field) or an array directly
		var resultArray []any
		if resultMap, ok := result.(map[string]any); ok {
			// Result is a map, check for data field
			if data, dataExists := resultMap["data"]; dataExists {
				if arr, arrOk := data.([]any); arrOk {
					resultArray = arr
				} else {
					return false, fmt.Errorf("'data' field is not an array: %v", data)
				}
			} else {
				return false, fmt.Errorf("'data' field not found in result map: %v", resultMap)
			}
		} else if arr, ok := result.([]any); ok {
			// Result is directly an array
			resultArray = arr
		} else {
			return false, fmt.Errorf("'result' is neither a map nor an array: %v", result)
		}

		// Ensure the array is not empty
		if len(resultArray) == 0 {
			return false, fmt.Errorf("'result' array is empty: %v", resultArray)
		}

		// Take only the first item
		firstItem := resultArray[0]

		// Check that the first item is a map
		itemMap, ok := firstItem.(map[string]any)
		if !ok {
			return false, fmt.Errorf("first item is not a map: %v", firstItem)
		}

		// Parse into struct
		var odooData odoomsmodel.ODOOMSTechnicianItem
		jsonData, err := json.Marshal(itemMap)
		if err != nil {
			return false, fmt.Errorf("error marshalling first item: %v", err)
		}

		err = json.Unmarshal(jsonData, &odooData)
		if err != nil {
			return false, fmt.Errorf("error unmarshalling first item: %v", err)
		}

		return true, nil
	}

	return false, fmt.Errorf("all retry attempts failed, last error: %w", lastErr)
}
