// Package dto defines data transfer objects used by the API layer.
package dto

// APIErrorResponse represents the standardized error payload returned by API endpoints
// when a request cannot be successfully processed.
//
// It provides both human-readable and machine-readable error information and should be
// used consistently across all API responses that represent failure conditions.
type APIErrorResponse struct {

	// Error is a short, human-readable summary of the error.
	//
	// This value typically corresponds to the HTTP status text, such as:
	//   - "Bad Request"
	//   - "Unauthorized"
	//   - "Forbidden"
	//   - "Internal Server Error"
	//
	// This field is always expected to be present.
	Error string `json:"error" example:"Bad Request"`

	// Details provides an optional, more descriptive explanation of the error condition.
	//
	// This field may include validation errors, missing or invalid parameters,
	// or other contextual information intended to help the client understand
	// why the request failed.
	//
	// When omitted, clients should rely on the Error field alone.
	Details string `json:"details,omitempty" example:"Invalid input data"`

	// Code is an optional, stable, application-specific error identifier.
	//
	// This value is intended for programmatic handling by API clients and should
	// not change frequently. Examples include:
	//   - "VALIDATION_ERROR"
	//   - "AUTHENTICATION_FAILED"
	//   - "RESOURCE_NOT_FOUND"
	//
	// When provided, clients may use this field to implement conditional logic
	// without parsing human-readable error messages.
	Code string `json:"code,omitempty" example:"VALIDATION_ERROR"`
}
