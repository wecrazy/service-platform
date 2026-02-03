package controllers

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand/v2"
	"net/http"
	"regexp"
	"service-platform/cmd/web_panel/config"
	"service-platform/cmd/web_panel/fun"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

// Session cache for ODOO cookies to avoid repeated authentication
var (
	odooSessionCache      []*http.Cookie
	odooSessionCacheTime  time.Time
	odooSessionCacheMutex sync.RWMutex
	sessionCacheDuration  = 10 * time.Minute // Cache sessions for 10 minutes
)

// Optimized HTTP client with connection pooling and keep-alive
var odooHTTPClient = &http.Client{
	Transport: &http.Transport{
		TLSClientConfig:     &tls.Config{InsecureSkipVerify: true},
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 10,
		IdleConnTimeout:     90 * time.Second,
		DisableCompression:  true, // Disable compression for better performance
	},
}

// Stock.Production.Lot item from ODOO
type ODOOSerialNumberItem struct {
	ID              uint              `json:"id"`
	SN              nullAbleString    `json:"name"`
	Product         nullAbleInterface `json:"product_id"`
	ProductCategory nullAbleInterface `json:"x_product_categ_id"`
	Company         nullAbleInterface `json:"company_id"`
}

type WoRemarkEntry struct {
	Datetime time.Time
	Reason   string
	Message  string
}

type DataTechnicianODOOMSBasedOnName struct {
	Group string
	City  string
	Name  string
}

func GetODOOMSCookies(email string, password string) ([]*http.Cookie, error) {
	yamlCfg := config.GetConfig()

	odooConfig := yamlCfg.ApiODOO

	odooDB := odooConfig.Db
	odooJSONRPC := odooConfig.JSONRPC
	urlSession := odooConfig.UrlSession

	requestJSON := `{
		"jsonrpc": %v,
		"params": {
			"db": "%s",
			"login": "%s",
			"password": "%s"
		}
	}`
	rawJSON := fmt.Sprintf(requestJSON, odooJSONRPC, odooDB, email, password)

	maxRetries := odooConfig.MaxRetry
	if maxRetries <= 0 {
		maxRetries = 5
	}

	retryDelay := odooConfig.RetryDelay
	if retryDelay <= 0 {
		retryDelay = 10
	}

	var errMsg string

	reqTimeout, err := time.ParseDuration(odooConfig.SessionTimeout)
	if err != nil {
		errMsg = fmt.Sprintf("invalid ODOO_SESSION_TIMEOUT value: %v", err)
		return nil, errors.New(errMsg)
	}

	var response *http.Response

	for attempts := 1; attempts <= maxRetries; attempts++ {
		request, err := http.NewRequest("POST", urlSession, bytes.NewBufferString(rawJSON))
		if err != nil {
			errMsg = fmt.Sprintf("error creating request: %v", err)
			return nil, errors.New(errMsg)
		}

		request.Header.Set("Content-Type", "application/json")

		// Custom HTTP client with TLS verification disabled
		client := &http.Client{
			Timeout: reqTimeout,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, // Skips TLS verification
			},
		}

		// Send the request
		response, err = client.Do(request)
		if err != nil {
			logrus.Warningf("error making POST request (attempt %d/%d): %v", attempts, maxRetries, err)
			if attempts < maxRetries {
				time.Sleep(time.Duration(retryDelay) * time.Second) // Wait before retrying
				continue
			}
			return nil, err // Return error after final retry
		}

		// Check if the response is successful
		if response.StatusCode == http.StatusOK {
			break
		} else {
			logrus.Warningf("bad response, status code: %d (attempt %d/%d), response: %v", response.StatusCode, attempts, maxRetries, response)
			if attempts < maxRetries {
				response.Body.Close() // Close the body before retrying
				time.Sleep(time.Duration(retryDelay) * time.Second)
				continue
			}
			return nil, err // Return error if all attempts fail
		}
	}

	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		errMsg = fmt.Sprintf("post request failed with status code: %v", response.StatusCode)
		return nil, errors.New(errMsg)
	}

	_, err = io.ReadAll(response.Body)
	if err != nil {
		errMsg = fmt.Sprintf("error reading response body: %v", err)
		return nil, errors.New(errMsg)
	}

	cookieODOO := response.Cookies()
	// logrus.Infof("ODOO session for email: %v, pwd: %v obtained successfully.", email, password)

	return cookieODOO, nil
}

// getODOOSessionCookiesOptimized returns cached cookies if still valid, otherwise fetches new ones
func getODOOSessionCookiesOptimized(email, password string) ([]*http.Cookie, error) {
	odooSessionCacheMutex.RLock()
	// Check if we have valid cached session
	if odooSessionCache != nil && time.Since(odooSessionCacheTime) < sessionCacheDuration {
		cookies := make([]*http.Cookie, len(odooSessionCache))
		copy(cookies, odooSessionCache)
		odooSessionCacheMutex.RUnlock()
		return cookies, nil
	}
	odooSessionCacheMutex.RUnlock()

	// Need to fetch new session
	odooSessionCacheMutex.Lock()
	defer odooSessionCacheMutex.Unlock()

	// Double-check in case another goroutine updated the cache
	if odooSessionCache != nil && time.Since(odooSessionCacheTime) < sessionCacheDuration {
		cookies := make([]*http.Cookie, len(odooSessionCache))
		copy(cookies, odooSessionCache)
		return cookies, nil
	}

	// Fetch new session
	newCookies, err := GetODOOMSCookies(email, password)
	if err != nil {
		return nil, err
	}

	// Update cache
	odooSessionCache = make([]*http.Cookie, len(newCookies))
	copy(odooSessionCache, newCookies)
	odooSessionCacheTime = time.Now()

	return newCookies, nil
}

func GetODOOMSData(req string) (interface{}, error) {
	yamlCfg := config.GetConfig()

	urlGetData := yamlCfg.ApiODOO.UrlGetData

	maxRetries := yamlCfg.ApiODOO.MaxRetry
	if maxRetries <= 0 {
		maxRetries = 5
	}

	retryDelay := yamlCfg.ApiODOO.RetryDelay
	if retryDelay <= 0 {
		retryDelay = 10
	}

	reqTimeout, err := time.ParseDuration(yamlCfg.ApiODOO.GetDataTimeout)
	if err != nil {
		errMsg := fmt.Sprintf("invalid GET_DATA_TIMEOUT value: %v", err)
		return nil, errors.New(errMsg)
	}

	// Update client timeout dynamically
	odooHTTPClient.Timeout = reqTimeout

	var response *http.Response

	for attempts := 1; attempts <= maxRetries; attempts++ {
		request, err := http.NewRequest("POST", urlGetData, bytes.NewBufferString(req))
		if err != nil {
			return nil, err
		}

		request.Header.Set("Content-Type", "application/json")
		// Add Connection: keep-alive for better performance
		request.Header.Set("Connection", "keep-alive")

		// Use optimized session management with caching
		OdooSessionCookies, err := getODOOSessionCookiesOptimized(yamlCfg.ApiODOO.Login, yamlCfg.ApiODOO.Password)
		if err != nil {
			return nil, err
		}

		for _, cookie := range OdooSessionCookies {
			request.AddCookie(cookie)
		}

		// Send the request using optimized client
		response, err = odooHTTPClient.Do(request)
		if err != nil {
			logrus.Errorf("making POST request (attempt %d/%d): %v", attempts, maxRetries, err)
			if attempts < maxRetries {
				time.Sleep(time.Duration(retryDelay) * time.Second)
				continue
			}
			return nil, err
		}

		// Check if the response is successful
		if response.StatusCode == http.StatusOK {
			break
		} else {
			logrus.Errorf("Bad response, status code: %d (attempt %d/%d), response: %v", response.StatusCode, attempts, maxRetries, response)

			if attempts < maxRetries {
				response.Body.Close()
				time.Sleep(time.Duration(retryDelay) * time.Second)
				continue
			}
			return nil, fmt.Errorf("all retry attempts failed, last status code: %d", response.StatusCode)
		}
	}

	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("post request failed with status code: %v", response.StatusCode)
	}

	body, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response body: %v", err)
	}

	var jsonResponse map[string]interface{}
	err = json.Unmarshal(body, &jsonResponse)
	if err != nil {
		return nil, fmt.Errorf("parsing JSON response: %v", err)
	}

	// Check for error response from Odoo
	if errorResponse, ok := jsonResponse["error"].(map[string]interface{}); ok {
		if errorMessage, ok := errorResponse["message"].(string); ok && errorMessage == "Odoo Session Expired" {
			// Clear cached session on expiration
			odooSessionCacheMutex.Lock()
			odooSessionCache = nil
			odooSessionCacheMutex.Unlock()
			return nil, fmt.Errorf("error code: %v, message: %v", errorResponse["code"], errorMessage)
		}
	}

	// Check for the result in JSON response
	if result, ok := jsonResponse["result"].(map[string]interface{}); ok {
		// Log the message and success status if they exist
		if message, ok := result["message"].(string); ok {
			success, successOk := result["success"]
			logrus.Infof("ODOO Result, message: %v, status: %v", message, successOk && success == true)
		}
	}

	// Check for the existence and validity of the "result" field
	result, resultExists := jsonResponse["result"]
	if !resultExists {
		return nil, fmt.Errorf("result field missing in the response!, error with params: %v", bytes.NewBufferString(req))
	}

	// Check if the result is an array and ensure it's not empty
	resultArray, ok := result.([]interface{})
	if !ok || len(resultArray) == 0 {
		return nil, fmt.Errorf("cannot find the data you have been request. Unexpected result format or empty result!, error with params: %v", bytes.NewBufferString(req))
	}

	return result, nil
}

// InvalidateODOOSessionCache clears the cached session cookies
func InvalidateODOOSessionCache() {
	odooSessionCacheMutex.Lock()
	defer odooSessionCacheMutex.Unlock()
	odooSessionCache = nil
	logrus.Debug("ODOO session cache invalidated")
}

// GetODOOSessionCacheStats returns cache statistics for monitoring
func GetODOOSessionCacheStats() map[string]interface{} {
	odooSessionCacheMutex.RLock()
	defer odooSessionCacheMutex.RUnlock()

	stats := map[string]interface{}{
		"has_cache":   odooSessionCache != nil,
		"cache_age":   time.Since(odooSessionCacheTime).String(),
		"cache_valid": odooSessionCache != nil && time.Since(odooSessionCacheTime) < sessionCacheDuration,
	}

	return stats
}

func GetODOOHommyPayCookies(email string, password string) ([]*http.Cookie, error) {
	yamlCfg := config.GetConfig()

	odooConfig := yamlCfg.ApiODOO

	odooDB := odooConfig.DbHommyPay
	odooJSONRPC := odooConfig.JSONRPC
	urlSession := odooConfig.UrlSessionHommyPay

	requestJSON := `{
		"jsonrpc": %v,
		"params": {
			"db": "%s",
			"login": "%s",
			"password": "%s"
		}
	}`
	rawJSON := fmt.Sprintf(requestJSON, odooJSONRPC, odooDB, email, password)

	maxRetries := odooConfig.MaxRetry
	if maxRetries <= 0 {
		maxRetries = 5
	}

	retryDelay := odooConfig.RetryDelay
	if retryDelay <= 0 {
		retryDelay = 10
	}

	var errMsg string

	reqTimeout, err := time.ParseDuration(odooConfig.SessionTimeout)
	if err != nil {
		errMsg = fmt.Sprintf("invalid ODOO_SESSION_TIMEOUT value: %v", err)
		return nil, errors.New(errMsg)
	}

	var response *http.Response

	for attempts := 1; attempts <= maxRetries; attempts++ {
		request, err := http.NewRequest("POST", urlSession, bytes.NewBufferString(rawJSON))
		if err != nil {
			errMsg = fmt.Sprintf("error creating request: %v", err)
			return nil, errors.New(errMsg)
		}

		request.Header.Set("Content-Type", "application/json")

		// Custom HTTP client with TLS verification disabled
		client := &http.Client{
			Timeout: reqTimeout,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, // Skips TLS verification
			},
		}

		// Send the request
		response, err = client.Do(request)
		if err != nil {
			logrus.Warnf("error making POST request (attempt %d/%d): %v", attempts, maxRetries, err)
			if attempts < maxRetries {
				time.Sleep(time.Duration(retryDelay) * time.Second) // Wait before retrying
				continue
			}
			return nil, err // Return error after final retry
		}

		// Check if the response is successful
		if response.StatusCode == http.StatusOK {
			break
		} else {
			logrus.Warnf("bad response, status code: %d (attempt %d/%d), response: %v", response.StatusCode, attempts, maxRetries, response)
			if attempts < maxRetries {
				response.Body.Close() // Close the body before retrying
				time.Sleep(time.Duration(retryDelay) * time.Second)
				continue
			}
			return nil, err // Return error if all attempts fail
		}
	}

	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		errMsg = fmt.Sprintf("post request failed with status code: %v", response.StatusCode)
		return nil, errors.New(errMsg)
	}

	_, err = io.ReadAll(response.Body)
	if err != nil {
		errMsg = fmt.Sprintf("error reading response body: %v", err)
		return nil, errors.New(errMsg)
	}

	cookieODOO := response.Cookies()
	// logrus.Infof("ODOO session for email: %v, pwd: %v obtained successfully.", email, password)

	return cookieODOO, nil
}

func FetchODOOMS(url, method, req string) ([]byte, error) {
	yamlCfg := config.GetConfig()

	maxRetries := yamlCfg.ApiODOO.MaxRetry
	if maxRetries <= 0 {
		maxRetries = 5
	}
	retryDelay := yamlCfg.ApiODOO.RetryDelay
	if retryDelay <= 0 {
		retryDelay = 10
	}
	reqTimeout, err := time.ParseDuration(yamlCfg.ApiODOO.GetDataTimeout)
	if err != nil {
		errMsg := fmt.Sprintf("invalid GET_DATA_TIMEOUT value: %v", err)
		return nil, errors.New(errMsg)
	}
	var response *http.Response
	for attempts := 1; attempts <= maxRetries; attempts++ {
		request, err := http.NewRequest(method, url, bytes.NewBufferString(req))
		if err != nil {
			logrus.Errorf("creating request: %v", err)
			return nil, err
		}
		request.Header.Set("Content-Type", "application/json")
		OdooSessionCookies, err := GetODOOMSCookies(yamlCfg.ApiODOO.Login, yamlCfg.ApiODOO.Password)
		if err != nil {
			return nil, err
		}
		for _, cookie := range OdooSessionCookies {
			request.AddCookie(cookie)
		}
		// Custom HTTP client with TLS verification disabled
		client := &http.Client{
			Timeout: reqTimeout,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, // Skips TLS verification
			},
		}
		// Send the request
		response, err = client.Do(request)
		if err != nil {
			logrus.Errorf("making %v request (attempt %d/%d): %v", method, attempts, maxRetries, err)
			if attempts < maxRetries {
				time.Sleep(time.Duration(retryDelay) * time.Second) // Wait before retrying
				continue
			}
			return nil, err // Return error after final retry
		}
		// Check if the response is successful
		if response.StatusCode == http.StatusOK {
			break
		} else {
			logrus.Errorf(
				"Bad response, status code: %d (attempt %d/%d), path: %s, response: %v",
				response.StatusCode, attempts, maxRetries, request.URL.Path, response,
			)
			if attempts < maxRetries {
				response.Body.Close() // Close the body before retrying
				time.Sleep(time.Duration(retryDelay) * time.Second)
				continue
			}
			return nil, err // Return error if all attempts fail
		}
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%v request failed with status code: %v", method, response.StatusCode)
	}

	body, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response body: %v", err)
	}

	// Default values
	return body, nil
}

func FetchODOOHommyPay(url, method, req string) ([]byte, error) {
	yamlCfg := config.GetConfig()

	maxRetries := yamlCfg.ApiODOO.MaxRetry
	if maxRetries <= 0 {
		maxRetries = 5
	}
	retryDelay := yamlCfg.ApiODOO.RetryDelay
	if retryDelay <= 0 {
		retryDelay = 10
	}
	reqTimeout, err := time.ParseDuration(yamlCfg.ApiODOO.GetDataTimeout)
	if err != nil {
		errMsg := fmt.Sprintf("invalid GET_DATA_TIMEOUT value: %v", err)
		return nil, errors.New(errMsg)
	}
	var response *http.Response
	for attempts := 1; attempts <= maxRetries; attempts++ {
		request, err := http.NewRequest(method, url, bytes.NewBufferString(req))
		if err != nil {
			logrus.Errorf("creating request: %v", err)
			return nil, err
		}
		request.Header.Set("Content-Type", "application/json")
		OdooSessionCookies, err := GetODOOHommyPayCookies(yamlCfg.ApiODOO.LoginHommyPay, yamlCfg.ApiODOO.PasswordHommyPay)
		if err != nil {
			return nil, err
		}
		for _, cookie := range OdooSessionCookies {
			request.AddCookie(cookie)
		}
		// Custom HTTP client with TLS verification disabled
		client := &http.Client{
			Timeout: reqTimeout,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, // Skips TLS verification
			},
		}
		// Send the request
		response, err = client.Do(request)
		if err != nil {
			logrus.Errorf("making %v request (attempt %d/%d): %v", method, attempts, maxRetries, err)
			if attempts < maxRetries {
				time.Sleep(time.Duration(retryDelay) * time.Second) // Wait before retrying
				continue
			}
			return nil, err // Return error after final retry
		}
		// Check if the response is successful
		if response.StatusCode == http.StatusOK {
			break
		} else {
			logrus.Errorf(
				"Bad response, status code: %d (attempt %d/%d), path: %s, response: %v",
				response.StatusCode, attempts, maxRetries, request.URL.Path, response,
			)
			if attempts < maxRetries {
				response.Body.Close() // Close the body before retrying
				time.Sleep(time.Duration(retryDelay) * time.Second)
				continue
			}
			return nil, err // Return error if all attempts fail
		}
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%v request failed with status code: %v", method, response.StatusCode)
	}

	body, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response body: %v", err)
	}

	// Default values
	return body, nil
}

func FetchODOOKresekBag(url, method, req string) ([]byte, error) {
	yamlCfg := config.GetConfig()

	maxRetries := yamlCfg.ApiODOO.MaxRetry
	if maxRetries <= 0 {
		maxRetries = 5
	}
	retryDelay := yamlCfg.ApiODOO.RetryDelay
	if retryDelay <= 0 {
		retryDelay = 10
	}
	reqTimeout, err := time.ParseDuration(yamlCfg.ApiODOO.GetDataTimeout)
	if err != nil {
		errMsg := fmt.Sprintf("invalid GET_DATA_TIMEOUT value: %v", err)
		return nil, errors.New(errMsg)
	}
	var response *http.Response
	for attempts := 1; attempts <= maxRetries; attempts++ {
		request, err := http.NewRequest(method, url, bytes.NewBufferString(req))
		if err != nil {
			logrus.Errorf("creating request: %v", err)
			return nil, err
		}
		request.Header.Set("Content-Type", "application/json")
		OdooSessionCookies, err := GetODOOKresekBagCookies(yamlCfg.ApiODOO.LoginKresekBag, yamlCfg.ApiODOO.PasswordKresekBag)
		if err != nil {
			return nil, err
		}
		for _, cookie := range OdooSessionCookies {
			request.AddCookie(cookie)
		}
		// Custom HTTP client with TLS verification disabled
		client := &http.Client{
			Timeout: reqTimeout,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, // Skips TLS verification
			},
		}
		// Send the request
		response, err = client.Do(request)
		if err != nil {
			logrus.Errorf("making %v request (attempt %d/%d): %v", method, attempts, maxRetries, err)
			if attempts < maxRetries {
				time.Sleep(time.Duration(retryDelay) * time.Second) // Wait before retrying
				continue
			}
			return nil, err // Return error after final retry
		}
		// Check if the response is successful
		if response.StatusCode == http.StatusOK {
			break
		} else {
			logrus.Errorf(
				"Bad response, status code: %d (attempt %d/%d), path: %s, response: %v",
				response.StatusCode, attempts, maxRetries, request.URL.Path, response,
			)
			if attempts < maxRetries {
				response.Body.Close() // Close the body before retrying
				time.Sleep(time.Duration(retryDelay) * time.Second)
				continue
			}
			return nil, err // Return error if all attempts fail
		}
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%v request failed with status code: %v", method, response.StatusCode)
	}

	body, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response body: %v", err)
	}

	// Default values
	return body, nil
}

func GetODOOKresekBagCookies(email string, password string) ([]*http.Cookie, error) {
	yamlCfg := config.GetConfig()

	odooConfig := yamlCfg.ApiODOO

	odooDB := odooConfig.DbKresekBag
	odooJSONRPC := odooConfig.JSONRPC
	urlSession := odooConfig.UrlSessionKresekBag

	requestJSON := `{
		"jsonrpc": %v,
		"params": {
			"db": "%s",
			"login": "%s",
			"password": "%s"
		}
	}`
	rawJSON := fmt.Sprintf(requestJSON, odooJSONRPC, odooDB, email, password)

	maxRetries := odooConfig.MaxRetry
	if maxRetries <= 0 {
		maxRetries = 5
	}

	retryDelay := odooConfig.RetryDelay
	if retryDelay <= 0 {
		retryDelay = 10
	}

	var errMsg string

	reqTimeout, err := time.ParseDuration(odooConfig.SessionTimeout)
	if err != nil {
		errMsg = fmt.Sprintf("invalid ODOO_SESSION_TIMEOUT value: %v", err)
		return nil, errors.New(errMsg)
	}

	var response *http.Response

	for attempts := 1; attempts <= maxRetries; attempts++ {
		request, err := http.NewRequest("POST", urlSession, bytes.NewBufferString(rawJSON))
		if err != nil {
			errMsg = fmt.Sprintf("error creating request: %v", err)
			return nil, errors.New(errMsg)
		}

		request.Header.Set("Content-Type", "application/json")

		// Custom HTTP client with TLS verification disabled
		client := &http.Client{
			Timeout: reqTimeout,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, // Skips TLS verification
			},
		}

		// Send the request
		response, err = client.Do(request)
		if err != nil {
			logrus.Warnf("error making POST request (attempt %d/%d): %v", attempts, maxRetries, err)
			if attempts < maxRetries {
				time.Sleep(time.Duration(retryDelay) * time.Second) // Wait before retrying
				continue
			}
			return nil, err // Return error after final retry
		}

		// Check if the response is successful
		if response.StatusCode == http.StatusOK {
			break
		} else {
			logrus.Warnf("bad response, status code: %d (attempt %d/%d), response: %v", response.StatusCode, attempts, maxRetries, response)
			if attempts < maxRetries {
				response.Body.Close() // Close the body before retrying
				time.Sleep(time.Duration(retryDelay) * time.Second)
				continue
			}
			return nil, err // Return error if all attempts fail
		}
	}

	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		errMsg = fmt.Sprintf("post request failed with status code: %v", response.StatusCode)
		return nil, errors.New(errMsg)
	}

	_, err = io.ReadAll(response.Body)
	if err != nil {
		errMsg = fmt.Sprintf("error reading response body: %v", err)
		return nil, errors.New(errMsg)
	}

	cookieODOO := response.Cookies()
	// logrus.Infof("ODOO session for email: %v, pwd: %v obtained successfully.", email, password)

	return cookieODOO, nil
}

func CheckSNExistinginODOOMS(serialNumber string) (bool, error) {
	odooModel := "stock.production.lot"
	odooFields := []string{
		"id",
		"name",
	}
	odooOrder := "id desc"

	domain := []interface{}{
		[]interface{}{"name", "=", serialNumber},
	}

	odooParams := map[string]interface{}{
		"domain": domain,
		"model":  odooModel,
		"fields": odooFields,
		"order":  odooOrder,
	}

	payload := map[string]interface{}{
		"jsonrpc": config.GetConfig().ApiODOO.JSONRPC,
		"params":  odooParams,
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return false, err
	}

	result, err := GetODOOMSData(string(payloadBytes))
	if err != nil {
		return false, err
	}

	resultArray, ok := result.([]interface{})
	if !ok {
		return false, fmt.Errorf("%v", "failed to assert results as []interface{}")
	}

	if len(resultArray) == 0 {
		return false, fmt.Errorf("no data found for SN: %s", serialNumber)
	}

	recordMap, ok := resultArray[0].(map[string]interface{})
	if !ok {
		return false, errors.New("failed to assert record as map[string]interface{}")
	}

	var odooData ODOOSerialNumberItem
	jsonData, err := json.Marshal(recordMap)
	if err != nil {
		return false, err
	}

	err = json.Unmarshal(jsonData, &odooData)
	if err != nil {
		return false, err
	}

	if odooData.SN.String != serialNumber {
		return false, fmt.Errorf("sn: %v is not found in ODOO MS", serialNumber)
	}

	// Default
	return true, nil
}

func InsertDataTicketInODOOHommyPay() {
	odooModel := "helpdesk.ticket"
	odooParams := map[string]interface{}{
		"company_id": 1, // PT SWI => debug
	}
	odooParams["active"] = true
	odooParams["name"] = time.Now().Format("2006-01-02 15:04:05") + "Test Ticket insert " + fun.GenerateRandomString(10)
	odooParams["model"] = odooModel
	random0until3 := rand.Int64N(4)
	odooParams["priority"] = strconv.Itoa(int(random0until3))

	payload := map[string]interface{}{
		"jsonrpc": config.GetConfig().ApiODOO.JSONRPC,
		"params":  odooParams,
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		logrus.Errorf("error marshalling payload: %v", err)
		return
	}

	url := config.GetConfig().ApiODOO.UrlCreateDataHommyPay
	method := "POST"

	body, err := FetchODOOHommyPay(url, method, string(payloadBytes))
	if err != nil {
		logrus.Error(err)
		return
	}

	var jsonResponse map[string]interface{}
	err = json.Unmarshal(body, &jsonResponse)
	if err != nil {
		logrus.Errorf("error unmarshalling JSON response: %v", err)
		return
	}
	if errorResponse, ok := jsonResponse["error"].(map[string]interface{}); ok {
		if errorMessage, ok := errorResponse["message"].(string); ok && errorMessage == "Odoo Session Expired" {
			logrus.Errorf("error code: %v, message: %v", errorResponse["code"], errorMessage)
			return
		}
	}
	if result, ok := jsonResponse["result"].(map[string]interface{}); ok {
		if message, ok := result["message"].(string); ok {
			success, successOk := result["success"]
			logrus.Infof("ODOO Result, message: %v, status: %v", message, successOk && success == true)
		}
	} else {
		logrus.Warnf("unexpected response format: %v", jsonResponse)
		return
	}
	if result, ok := jsonResponse["result"].(map[string]interface{}); ok {
		if id, ok := result["id"].(float64); ok {
			logrus.Infof("Ticket created successfully with ID: %v", id)
		} else {
			logrus.Errorf("ID not found in the response: %v", jsonResponse)
		}
	} else {
		logrus.Errorf("unexpected response format: %v", jsonResponse)
		return
	}
}

func extractUniqueIDs(arrays ...[]interface{}) []uint64 {
	seen := make(map[uint64]struct{})
	var result []uint64

	for _, array := range arrays {
		for _, item := range array {
			recordMap, ok := item.(map[string]interface{})
			if !ok {
				continue
			}
			rawID, exists := recordMap["id"]
			if !exists {
				continue
			}
			floatID, ok := rawID.(float64)
			if !ok {
				continue
			}
			uintID := uint64(floatID)
			if _, exists := seen[uintID]; !exists {
				seen[uintID] = struct{}{}
				result = append(result, uintID)
			}
		}
	}
	return result
}

func chunkIdsSlice(ids []uint64, size int) [][]uint64 {
	if size <= 0 {
		return nil
	}

	var chunks [][]uint64
	for i := 0; i < len(ids); i += size {
		end := i + size
		if end > len(ids) {
			end = len(ids)
		}
		chunks = append(chunks, ids[i:end])
	}
	return chunks
}

func techGroup(technician string) (string, error) {
	if strings.TrimSpace(technician) == "" {
		return "", errors.New("technician name is empty")
	}

	words := strings.Fields(technician)
	if len(words) > 0 {
		return words[0], nil
	}

	return "", errors.New("unable to extract group from technician name")
}

// SLA Status with conditions :
// - PM <= 15th set as Meet SLA
// - Overdue : New & Visited
func setSLAStatus(
	taskCount int,
	SLADeadline nullAbleTime,
	CompleteDatetimeWO nullAbleTime,
	WoRemark, taskType nullAbleString) (string, time.Time, string, string) {
	// Special handling for Preventive Maintenance
	if taskType.Valid && strings.ToLower(taskType.String) == "preventive maintenance" {
		loc, _ := time.LoadLocation(config.GetConfig().Default.Timezone)
		now := time.Now().In(loc)

		// If SLA deadline is valid and in the current month and <= 15th
		if SLADeadline.Valid && !SLADeadline.Time.IsZero() {
			slaDeadline := SLADeadline.Time
			if slaDeadline.Year() == now.Year() &&
				slaDeadline.Month() == now.Month() &&
				slaDeadline.Day() <= 15 {
				// PM and SLA deadline <= 15th: always On Target Solved
				if taskCount >= 2 && WoRemark.Valid && WoRemark.String != "" {
					entries, err := parseWoRemark(WoRemark.String)
					if err != nil {
						logrus.Print(err)
						return "On Target Solved", time.Time{}, "", ""
					}
					if len(entries) > 0 {
						firstTask := entries[0]
						// PM with valid WoRemark: On Target Solved
						return "On Target Solved", firstTask.Datetime, firstTask.Reason, firstTask.Message
					}
				}
				// PM without valid WoRemark: On Target Solved
				return "On Target Solved", CompleteDatetimeWO.Time, "", ""
			}
		}
		// If SLA deadline is not in current month or > 15th, proceed with normal logic below
	}

	// Check if SLA deadline is past and no completion: Overdue (New)
	now := time.Now()
	if SLADeadline.Valid && SLADeadline.Time.Before(now) && (!CompleteDatetimeWO.Valid || CompleteDatetimeWO.Time.IsZero()) {
		return "Overdue (New)", time.Time{}, "", ""
	}

	// Main logic for Overdue (New) and Overdue (Visited)
	if taskCount >= 2 {
		if WoRemark.Valid && WoRemark.String != "" {
			entries, err := parseWoRemark(WoRemark.String)
			if err != nil {
				logrus.Print(err)
				// No visit info: Not Visit
				return "Not Visit", time.Time{}, "", ""
			}

			if len(entries) > 0 {
				firstTask := entries[0]

				if SLADeadline.Time.IsZero() || firstTask.Datetime.IsZero() {
					// Missing SLA or first task datetime: Not Visit
					return "Not Visit", time.Time{}, firstTask.Reason, firstTask.Message
				}

				if firstTask.Datetime.Before(SLADeadline.Time) {
					// First task before SLA: On Target Solved
					return "On Target Solved", firstTask.Datetime, firstTask.Reason, firstTask.Message
				} else {
					// Overdue: check if visited or new
					if CompleteDatetimeWO.Valid && !CompleteDatetimeWO.Time.IsZero() {
						// Overdue and has CompleteDatetimeWO: Overdue (Visited)
						return "Overdue (Visited)", firstTask.Datetime, firstTask.Reason, firstTask.Message
					} else {
						// Overdue and no CompleteDatetimeWO: Overdue (New)
						return "Overdue (New)", firstTask.Datetime, firstTask.Reason, firstTask.Message
					}
				}
			} else {
				// No WoRemark entries
				if SLADeadline.Time.IsZero() || CompleteDatetimeWO.Time.IsZero() {
					// Missing SLA or CompleteDatetimeWO: Not Visit
					return "Not Visit", time.Time{}, "", ""
				}

				if CompleteDatetimeWO.Time.Before(SLADeadline.Time) {
					// CompleteDatetimeWO before SLA: On Target Solved
					return "On Target Solved", time.Time{}, "", ""
				} else {
					// Overdue: check if visited or new
					if CompleteDatetimeWO.Valid && !CompleteDatetimeWO.Time.IsZero() {
						// Overdue and has CompleteDatetimeWO: Overdue (Visited)
						return "Overdue (Visited)", time.Time{}, "", ""
					} else {
						// Overdue and no CompleteDatetimeWO: Overdue (New)
						return "Overdue (New)", time.Time{}, "", ""
					}
				}
			}
		} else {
			// No WoRemark
			if SLADeadline.Time.IsZero() || CompleteDatetimeWO.Time.IsZero() {
				// Missing SLA or CompleteDatetimeWO: Not Visit
				return "Not Visit", time.Time{}, "", ""
			}

			if CompleteDatetimeWO.Time.Before(SLADeadline.Time) {
				// CompleteDatetimeWO before SLA: On Target Solved
				return "On Target Solved", time.Time{}, "", ""
			} else {
				// Overdue: check if visited or new
				if CompleteDatetimeWO.Valid && !CompleteDatetimeWO.Time.IsZero() {
					// Overdue and has CompleteDatetimeWO: Overdue (Visited)
					return "Overdue (Visited)", time.Time{}, "", ""
				} else {
					// Overdue and no CompleteDatetimeWO: Overdue (New)
					return "Overdue (New)", time.Time{}, "", ""
				}
			}
		}
	} else {
		// taskCount < 2
		if SLADeadline.Time.IsZero() || CompleteDatetimeWO.Time.IsZero() {
			// Missing SLA or CompleteDatetimeWO: Not Visit
			return "Not Visit", time.Time{}, "", ""
		}

		if CompleteDatetimeWO.Time.Before(SLADeadline.Time) {
			// CompleteDatetimeWO before SLA: On Target Solved
			return "On Target Solved", time.Time{}, "", ""
		} else {
			// Overdue: check if visited or new
			if CompleteDatetimeWO.Valid && !CompleteDatetimeWO.Time.IsZero() {
				// Overdue and has CompleteDatetimeWO: Overdue (Visited)
				return "Overdue (Visited)", time.Time{}, "", ""
			} else {
				// Overdue and no CompleteDatetimeWO: Overdue (New)
				return "Overdue (New)", time.Time{}, "", ""
			}
		}
	}
}

func excludeDataForSLAReport(data OdooTicketDataRequestItem, slaStatus string) bool {
	excludedDataPingMerchantAndAOB := excludeOverdueTicketPingMerchantAndAOB(data, slaStatus)
	if excludedDataPingMerchantAndAOB {
		return true
	}

	// Default
	return false
}

func excludeOverdueTicketPingMerchantAndAOB(data OdooTicketDataRequestItem, slaStatus string) bool {
	_, ticketType := parseJSONIDDataCombinedSafe(data.TicketTypeId)
	_, technician := parseJSONIDDataCombinedSafe(data.TechnicianId)

	// Exclude if SLA status contains "overdue" AND ticket type contains "ping merchant" or "aob"
	if strings.Contains(strings.ToLower(slaStatus), "overdue") {
		containsWord := []string{
			"ping merchant",
			"aob",
		}

		for _, word := range containsWord {
			if strings.Contains(strings.ToLower(ticketType), word) {
				return true // Exclude overdue tickets with specific ticket types
			}
		}

		// Don't exclude all overdue tickets - only those with specific conditions above
		// #### return true #### // <-- This was the bug!!
	}

	// Exclude if technician contains "aob"
	if strings.Contains(strings.ToLower(strings.TrimSpace(technician)), "aob") {
		return true
	}

	// Exclude if description contains "ping merchant" variants
	excludeDesc := []string{
		"ping merchant",
		"pingmerchant",
		"ping-merchant",
		"#ping merchant",
		"#pingmerchant",
		"#ping-merchant",
	}
	descLower := strings.ToLower(strings.TrimSpace(data.Description.String))
	for _, ex := range excludeDesc {
		if strings.Contains(descLower, ex) {
			return true
		}
	}

	return false
}

func parseReasonCode(reasonCode string) []string {
	if strings.TrimSpace(reasonCode) == "" {
		return nil
	}

	reasonCodes := strings.Split(reasonCode, "; ")

	var cleanedReasonCodes []string
	for _, code := range reasonCodes {
		if strings.TrimSpace(code) != "" {
			cleanedReasonCodes = append(cleanedReasonCodes, code)
		}
	}

	if len(cleanedReasonCodes) == 0 {
		return nil
	}

	return cleanedReasonCodes
}

func SLAExpired(slaDeadline nullAbleTime) string {
	// Check if the value is null or invalid
	if !slaDeadline.Valid {
		return "SLA Not Found!"
	}

	loc, _ := time.LoadLocation(config.GetConfig().Default.Timezone)
	now := time.Now().In(loc)
	slaTime := slaDeadline.Time.In(loc) // Convert SLA time to local timezone

	// Truncate times to midnight for date-only comparison
	nowDate := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)
	slaDate := time.Date(slaTime.Year(), slaTime.Month(), slaTime.Day(), 0, 0, 0, 0, loc)

	// Calculate the difference in days
	daysDiff := int(slaDate.Sub(nowDate).Hours() / 24)

	// Generate the response based on the difference
	switch {
	case daysDiff == 0:
		return fmt.Sprintf("SLA expires today at %s", slaTime.Format("15:04"))
	case daysDiff == 1:
		return "SLA expires tomorrow"
	case daysDiff > 1:
		return fmt.Sprintf("SLA expires in %d days", daysDiff)
	case daysDiff == -1:
		return "SLA expired yesterday"
	default: // daysDiff < -1
		return fmt.Sprintf("SLA expired %d days ago", -daysDiff)
	}
}

func parseWoRemark(remark string) ([]WoRemarkEntry, error) {
	// Pattern now handles multiline messages with (?s) flag for . to match newlines
	pattern := `(?i)(?:technical user|false)\s+on\s+(\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2})\s+:\s+([^,]+),\s+((?s:.*?));`

	re := regexp.MustCompile(pattern)
	matches := re.FindAllStringSubmatch(remark, -1)

	var results []WoRemarkEntry
	loc, _ := time.LoadLocation(config.GetConfig().Default.Timezone) // Change to your local timezone

	for _, match := range matches {
		if len(match) == 4 {
			parsedTime, err := time.ParseInLocation("2006-01-02 15:04:05", match[1], loc)
			if err != nil {
				return nil, fmt.Errorf("failed to parse datetime: %v", err)
			}

			results = append(results, WoRemarkEntry{
				Datetime: parsedTime,
				Reason:   match[2],
				Message:  strings.TrimSpace(match[3]),
			})
		}
	}

	// Sort entries by datetime in ascending order (earliest first)
	sort.Slice(results, func(i, j int) bool {
		return results[i].Datetime.Before(results[j].Datetime)
	})

	return results, nil
}

func CleanSPKNumber(spk string) string {
	re := regexp.MustCompile(`\s*\(.*?\)`)
	return re.ReplaceAllString(spk, "")
}

func derefString(ptr *string) string {
	if ptr == nil {
		return ""
	}
	return *ptr
}

func derefTime(ptr *time.Time) string {
	if ptr == nil || ptr.IsZero() {
		return ""
	}
	return ptr.Format("2006-01-02 15:04:05")
}

func derefDate(ptr *time.Time) string {
	if ptr == nil || ptr.IsZero() {
		return ""
	}
	// return ptr.Format("2006-01-02")
	return ptr.Format("02 January 2006")
}

func derefDay(ptr *time.Time) string {
	if ptr == nil || ptr.IsZero() {
		return ""
	}
	return ptr.Format("02")
}

func ParsedDataTechnicianODOOMS(technician string) *DataTechnicianODOOMSBasedOnName {
	if strings.TrimSpace(technician) == "" {
		return nil
	}

	// Split the technician string into parts
	parts := strings.Fields(strings.TrimSpace(technician))
	if len(parts) < 3 {
		return nil // Not enough parts to parse
	}

	// Extract group (first part like "3.6", "1.1", etc.)
	group := parts[0]

	// Extract city (second part like "Mataram", "Jakpus", etc.)
	city := parts[1]

	// Extract name (remaining parts joined)
	name := strings.Join(parts[2:], " ")

	return &DataTechnicianODOOMSBasedOnName{
		Group: group,
		City:  city,
		Name:  name,
	}
}

// SLA Status without PM <= 15th set as Meet SLA
// func setSLAStatus(taskCount int, SLADeadline nullAbleTime, CompleteDatetimeWO nullAbleTime, WoRemark nullAbleString) (string, time.Time, string, string) {
// 	if taskCount >= 2 {
// 		if WoRemark.Valid && WoRemark.String != "" {
// 			entries, err := parseWoRemark(WoRemark.String)
// 			if err != nil {
// 				logrus.Print(err)
// 				return "Not Visit", time.Time{}, "", ""
// 			}

// 			if len(entries) > 0 {
// 				firstTask := entries[len(entries)-1]

// 				if SLADeadline.Time.IsZero() || firstTask.Datetime.IsZero() {
// 					return "Not Visit", time.Time{}, firstTask.Reason, firstTask.Message
// 				}

// 				if firstTask.Datetime.Before(SLADeadline.Time) {
// 					return "On Target Solved", firstTask.Datetime, firstTask.Reason, firstTask.Message
// 				} else {
// 					return "Overdue", firstTask.Datetime, firstTask.Reason, firstTask.Message
// 				}
// 			} else {
// 				if SLADeadline.Time.IsZero() || CompleteDatetimeWO.Time.IsZero() {
// 					return "Not Visit", time.Time{}, "", ""
// 				}

// 				if CompleteDatetimeWO.Time.Before(SLADeadline.Time) {
// 					return "On Target Solved", time.Time{}, "", ""
// 				} else {
// 					return "Overdue", time.Time{}, "", ""
// 				}
// 			}
// 		} else {
// 			if SLADeadline.Time.IsZero() || CompleteDatetimeWO.Time.IsZero() {
// 				return "Not Visit", time.Time{}, "", ""
// 			}

// 			if CompleteDatetimeWO.Time.Before(SLADeadline.Time) {
// 				return "On Target Solved", time.Time{}, "", ""
// 			} else {
// 				return "Overdue", time.Time{}, "", ""
// 			}
// 		}
// 	} else {
// 		if SLADeadline.Time.IsZero() || CompleteDatetimeWO.Time.IsZero() {
// 			return "Not Visit", time.Time{}, "", ""
// 		}

// 		if CompleteDatetimeWO.Time.Before(SLADeadline.Time) {
// 			return "On Target Solved", time.Time{}, "", ""
// 		} else {
// 			return "Overdue", time.Time{}, "", ""
// 		}
// 	}
// }

// // SLA Status with PM <= 15th set as Meet SLA
// func setSLAStatus(
// 	taskCount int,
// 	SLADeadline nullAbleTime,
// 	CompleteDatetimeWO nullAbleTime,
// 	WoRemark, taskType nullAbleString) (string, time.Time, string, string) {
// 	// Special handling for Preventive Maintenance
// 	if taskType.Valid && strings.ToLower(taskType.String) == "preventive maintenance" {
// 		loc, _ := time.LoadLocation(config.GetConfig().Default.Timezone)
// 		now := time.Now().In(loc)

// 		// Check if SLA deadline is valid and in the current month and <= 15th
// 		if SLADeadline.Valid && !SLADeadline.Time.IsZero() {
// 			// Use the SLA deadline as-is without timezone conversion to avoid double conversion
// 			slaDeadline := SLADeadline.Time

// 			// Check if SLA deadline is in current month and day <= 15th
// 			if slaDeadline.Year() == now.Year() &&
// 				slaDeadline.Month() == now.Month() &&
// 				slaDeadline.Day() <= 15 {
// 				// If it's PM and SLA deadline is <= 15th of current month, always return "On Target Solved"
// 				if taskCount >= 2 && WoRemark.Valid && WoRemark.String != "" {
// 					entries, err := parseWoRemark(WoRemark.String)
// 					if err != nil {
// 						logrus.Print(err)
// 						return "On Target Solved", time.Time{}, "", ""
// 					}
// 					if len(entries) > 0 {
// 						firstTask := entries[len(entries)-1]
// 						return "On Target Solved", firstTask.Datetime, firstTask.Reason, firstTask.Message
// 					}
// 				}
// 				return "On Target Solved", CompleteDatetimeWO.Time, "", ""
// 			}
// 		}
// 		// If SLA deadline is not in current month or > 15th, proceed with normal logic below
// 	}

// 	if taskCount >= 2 {
// 		if WoRemark.Valid && WoRemark.String != "" {
// 			entries, err := parseWoRemark(WoRemark.String)
// 			if err != nil {
// 				logrus.Print(err)
// 				return "Not Visit", time.Time{}, "", ""
// 			}

// 			if len(entries) > 0 {
// 				firstTask := entries[len(entries)-1]

// 				if SLADeadline.Time.IsZero() || firstTask.Datetime.IsZero() {
// 					return "Not Visit", time.Time{}, firstTask.Reason, firstTask.Message
// 				}

// 				if firstTask.Datetime.Before(SLADeadline.Time) {
// 					return "On Target Solved", firstTask.Datetime, firstTask.Reason, firstTask.Message
// 				} else {
// 					return "Overdue", firstTask.Datetime, firstTask.Reason, firstTask.Message
// 				}
// 			} else {
// 				if SLADeadline.Time.IsZero() || CompleteDatetimeWO.Time.IsZero() {
// 					return "Not Visit", time.Time{}, "", ""
// 				}

// 				if CompleteDatetimeWO.Time.Before(SLADeadline.Time) {
// 					return "On Target Solved", time.Time{}, "", ""
// 				} else {
// 					return "Overdue", time.Time{}, "", ""
// 				}
// 			}
// 		} else {
// 			if SLADeadline.Time.IsZero() || CompleteDatetimeWO.Time.IsZero() {
// 				return "Not Visit", time.Time{}, "", ""
// 			}

// 			if CompleteDatetimeWO.Time.Before(SLADeadline.Time) {
// 				return "On Target Solved", time.Time{}, "", ""
// 			} else {
// 				return "Overdue", time.Time{}, "", ""
// 			}
// 		}
// 	} else {
// 		if SLADeadline.Time.IsZero() || CompleteDatetimeWO.Time.IsZero() {
// 			return "Not Visit", time.Time{}, "", ""
// 		}

// 		if CompleteDatetimeWO.Time.Before(SLADeadline.Time) {
// 			return "On Target Solved", time.Time{}, "", ""
// 		} else {
// 			return "Overdue", time.Time{}, "", ""
// 		}
// 	}
// }
