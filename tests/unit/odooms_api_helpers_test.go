package unit

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"testing"

	odoomscontrollers "service-platform/internal/api/v1/controllers/odooms_controllers"
	"service-platform/internal/config"

	"github.com/stretchr/testify/assert"
)

// createTestConfigODOOMSAPIHelper creates a test configuration for ODOOMS testing
func createTestConfigODOOMSAPIHelper() *config.TypeManageService {
	return &config.TypeManageService{
		ODOOMS: struct {
			JsonRPCVersion string                         `yaml:"jsonrpc_version" validate:"required"`
			Login          string                         `yaml:"login" validate:"required"`
			Password       string                         `yaml:"password" validate:"required"`
			DB             string                         `yaml:"db" validate:"required"`
			URL            string                         `yaml:"url" validate:"required"`
			PathSession    string                         `yaml:"path_session" validate:"required"`
			PathGetData    string                         `yaml:"path_getdata" validate:"required"`
			PathUpdateData string                         `yaml:"path_updatedata" validate:"required"`
			PathCreateData string                         `yaml:"path_createdata" validate:"required"`
			MaxRetry       int                            `yaml:"max_retry" validate:"required"`
			RetryDelay     int                            `yaml:"retry_delay" validate:"required"`
			SessionTimeout int                            `yaml:"session_timeout" validate:"required"`
			DataTimeout    int                            `yaml:"data_timeout"`
			SkipSSLVerify  bool                           `yaml:"skip_ssl_verify"`
			SACData        map[string]config.ODOOMSACData `yaml:"sac" validate:"required"`
		}{
			JsonRPCVersion: "2.0",
			Login:          "desta@smartwebdindonesia.com",
			Password:       "Makan198",
			DB:             "gsa_db",
			URL:            "https://192.101.1.66:8069",
			PathSession:    "/web/session/authenticate",
			PathGetData:    "/api/getdata",
			PathUpdateData: "/api/updatedata",
			PathCreateData: "/api/createdata",
			MaxRetry:       3,
			RetryDelay:     1,
			SessionTimeout: 30,
			DataTimeout:    300,
			SkipSSLVerify:  true,
			SACData: map[string]config.ODOOMSACData{
				"test_sac": {
					Username: "sac_user",
					Fullname: "SAC User",
					Phone:    "+628123456789",
					Email:    "email@test.com",
					TTDPath:  "/path/to/ttd.png",
					Region:   1,
				},
			},
		},
	}
}

// TestCheckExistingTechnicianInODOOMS_Success tests successful technician lookup
func TestCheckExistingTechnicianInODOOMS_Success(t *testing.T) {
	// Create mock server
	server := createMockODOOServerForTechnician()
	defer server.Close()

	// Create test config
	testConfig := createTestConfigODOOMSAPIHelper()
	testConfig.ODOOMS.URL = server.URL

	// Test with phone number
	exists, _, err := odoomscontrollers.CheckExistingTechnicianInODOOMSWithConfig("", "", "+628123456789", *testConfig)
	assert.NoError(t, err)
	assert.True(t, exists)

	// Test with name
	exists, _, err = odoomscontrollers.CheckExistingTechnicianInODOOMSWithConfig("John Doe", "", "", *testConfig)
	assert.NoError(t, err)
	assert.True(t, exists)

	// Test with email
	exists, _, err = odoomscontrollers.CheckExistingTechnicianInODOOMSWithConfig("", "john.doe@example.com", "", *testConfig)
	assert.NoError(t, err)
	assert.True(t, exists)
}

// TestCheckExistingTechnicianInODOOMS_NotFound tests when technician is not found
func TestCheckExistingTechnicianInODOOMS_NotFound(t *testing.T) {
	// Create mock server that returns empty results
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/web/session/authenticate":
			// Mock authentication response
			w.Header().Set("Set-Cookie", "session_id=test_session_123; Path=/")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"jsonrpc": "2.0", "result": {"session_id": "test_session_123"}}`))
		case "/api/getdata":
			// Mock empty data response
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{
				"jsonrpc": "2.0",
				"result": {
					"success": true,
					"message": "No data found",
					"data": []
				}
			}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	// Create test config
	testConfig := createTestConfigODOOMSAPIHelper()
	testConfig.ODOOMS.URL = server.URL

	// Test with non-existent technician
	exists, _, err := odoomscontrollers.CheckExistingTechnicianInODOOMSWithConfig("Nonexistent", "nonexistent@example.com", "+628000000000", *testConfig)
	assert.Error(t, err)
	assert.False(t, exists)
	assert.Contains(t, err.Error(), "'result' array is empty")
}

// TestCheckExistingTechnicianInODOOMS_ServerError tests server error handling
func TestCheckExistingTechnicianInODOOMS_ServerError(t *testing.T) {
	// Create mock server that returns server error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`Internal Server Error`))
	}))
	defer server.Close()

	// Create test config
	testConfig := createTestConfigODOOMSAPIHelper()
	testConfig.ODOOMS.URL = server.URL

	// Test server error
	exists, _, err := odoomscontrollers.CheckExistingTechnicianInODOOMSWithConfig("Test", "test@example.com", "+628123456789", *testConfig)
	assert.Error(t, err)
	assert.False(t, exists)
}

// TestCheckExistingTechnicianInODOOMS_SessionExpired tests ODOO session expired error
func TestCheckExistingTechnicianInODOOMS_SessionExpired(t *testing.T) {
	// Create mock server that returns session expired error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/web/session/authenticate":
			w.Header().Set("Set-Cookie", "session_id=test_session_123; Path=/")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"jsonrpc": "2.0", "result": {"session_id": "test_session_123"}}`))
		case "/api/getdata":
			// Mock session expired error
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{
				"jsonrpc": "2.0",
				"error": {
					"message": "Odoo Session Expired"
				}
			}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	// Create test config
	testConfig := createTestConfigODOOMSAPIHelper()
	testConfig.ODOOMS.URL = server.URL

	// Test session expired error
	exists, _, err := odoomscontrollers.CheckExistingTechnicianInODOOMSWithConfig("Test", "test@example.com", "+628123456789", *testConfig)
	assert.Error(t, err)
	assert.False(t, exists)
	assert.Equal(t, "odoo Session Expired", err.Error())
}

// TestCheckExistingTechnicianInODOOMS_InvalidResponse tests invalid response handling
func TestCheckExistingTechnicianInODOOMS_InvalidResponse(t *testing.T) {
	// Create mock server that returns invalid JSON
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/web/session/authenticate":
			w.Header().Set("Set-Cookie", "session_id=test_session_123; Path=/")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"jsonrpc": "2.0", "result": {"session_id": "test_session_123"}}`))
		case "/api/getdata":
			// Mock invalid JSON response
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`invalid json`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	// Create test config
	testConfig := createTestConfigODOOMSAPIHelper()
	testConfig.ODOOMS.URL = server.URL

	// Test invalid response
	exists, _, err := odoomscontrollers.CheckExistingTechnicianInODOOMSWithConfig("Test", "test@example.com", "+628123456789", *testConfig)
	assert.Error(t, err)
	assert.False(t, exists)
}

// TestCheckExistingTechnicianInODOOMS_NoResultField tests missing result field
func TestCheckExistingTechnicianInODOOMS_NoResultField(t *testing.T) {
	// Create mock server that returns response without result field
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/web/session/authenticate":
			w.Header().Set("Set-Cookie", "session_id=test_session_123; Path=/")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"jsonrpc": "2.0", "result": {"session_id": "test_session_123"}}`))
		case "/api/getdata":
			// Mock response without result field
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{
				"jsonrpc": "2.0",
				"some_other_field": "value"
			}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	// Create test config
	testConfig := createTestConfigODOOMSAPIHelper()
	testConfig.ODOOMS.URL = server.URL

	// Test missing result field
	exists, _, err := odoomscontrollers.CheckExistingTechnicianInODOOMSWithConfig("Test", "test@example.com", "+628123456789", *testConfig)
	assert.Error(t, err)
	assert.False(t, exists)
	assert.Contains(t, err.Error(), "'result' field not found in the response")
}

// createMockODOOServerForTechnician creates a mock server specifically for technician tests
func createMockODOOServerForTechnician() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/web/session/authenticate":
			// Mock authentication response
			w.Header().Set("Set-Cookie", "session_id=test_session_123; Path=/")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"jsonrpc": "2.0", "result": {"session_id": "test_session_123"}}`))
		case "/api/getdata":
			// Mock technician data response
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{
				"jsonrpc": "2.0",
				"result": {
					"success": true,
					"message": "Data retrieved successfully",
					"data": [
						{
							"id": 1,
							"email": "john.doe@example.com",
							"x_no_telp": "+628123456789",
							"x_technician_name": "John Doe"
						}
					]
				}
			}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
}

// TestCheckExistingTechnicianInODOOMS_EmptyParameters tests with empty parameters
func TestCheckExistingTechnicianInODOOMS_EmptyParameters(t *testing.T) {
	// Create mock server
	server := createMockODOOServerForTechnician()
	defer server.Close()

	// Create test config
	testConfig := createTestConfigODOOMSAPIHelper()
	testConfig.ODOOMS.URL = server.URL

	// Test with all empty parameters (should still work but return error due to empty domain)
	exists, _, err := odoomscontrollers.CheckExistingTechnicianInODOOMSWithConfig("", "", "", *testConfig)
	assert.Error(t, err)
	assert.False(t, exists)
	assert.Contains(t, err.Error(), "at least one search parameter")
}

// TestCheckExistingTechnicianInODOOMS_SingleParameter tests with single parameter
// run it with: go test -v ./tests/unit -run ^TestCheckExistingTechnicianInODOOMS_SingleParameter$
func TestCheckExistingTechnicianInODOOMS_SingleParameter(t *testing.T) {
	// // Create mock server
	// server := createMockODOOServerForTechnician()
	// defer server.Close()

	var err error
	config.ManageService.MustInit("manage-service") // Load config with name "manage-service.%s.yaml"
	if !config.ManageService.IsLoaded() {
		err = errors.New("failed to load configuration")
		log.Fatal(err)
	}

	cfg := config.ManageService.Get()

	// Create test config
	testConfig := createTestConfigODOOMSAPIHelper()
	// testConfig.ODOOMS.URL = server.URL
	testConfig.ODOOMS.URL = cfg.ODOOMS.URL

	// Test with only email
	exists, _, err := odoomscontrollers.CheckExistingTechnicianInODOOMSWithConfig("", "testmfjr@gmail.com", "", cfg)

	fmt.Println("TestCheckExistingTechnicianInODOOMS_SingleParameter:", exists)
	fmt.Printf("Error: %v\n", err)
	t.Logf("Status: %v", exists)

	assert.NoError(t, err)
	assert.True(t, exists)
}
