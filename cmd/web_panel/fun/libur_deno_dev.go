// Package fun provides utilities for interacting with Indonesian public holiday API.
// This package uses the libur.deno.dev API to fetch Indonesian public holiday information.

// IndonesianLibur represents the response structure for holiday check on a specific date.
// It contains the date, whether it's a holiday, and a list of holidays if applicable.

// IndonesiaHolidayError represents the error response structure from the API.
// It contains an error message and detailed error information.

// IndonesiaHoliday represents a single holiday entry with date and name.

// GetLibur retrieves holiday information for "today" or "tomorrow".
// It accepts "today" or "tomorrow" as parameters (case-insensitive).
// Returns IndonesianLibur data or an error if the request fails or parameter is invalid.

// GetYearHolidays retrieves all holidays for a specific year.
// If year is 0, it returns holidays for the current year.
// Returns a slice of IndonesiaHoliday or an error if the request fails.

// GetMonthBasedHolidaysOfCurrentYear retrieves holidays for a specific month in the current year.
// The month parameter is required (1-12).
// Returns a slice of IndonesiaHoliday or an error if the request fails.

// GetHolidaysBasedOnYearAndMonth retrieves holidays for a specific year and month.
// Both year and month parameters are required.
// Returns a slice of IndonesiaHoliday or an error if the request fails.

// GetHolidaysBasedOnYearMonthAndDay retrieves holiday information for a specific date.
// All parameters (year, month, day) are required.
// Returns IndonesianLibur data or an error if the request fails.
// ### Source: https://github.com/radyakaze/api-hari-libur

package fun

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"service-platform/cmd/web_panel/config"
)

type IndonesianLibur struct {
	Date        string   `json:"date"`
	IsHoliday   bool     `json:"is_holiday"`
	HolidayList []string `json:"holiday_list"`
}

type IndonesiaHolidayError struct {
	Message string              `json:"message"`
	Errors  map[string][]string `json:"errors"`
}

type IndonesiaHoliday struct {
	Date string `json:"date"`
	Name string `json:"name"`
}

func GetLibur(hari string) (*IndonesianLibur, error) {
	var url string
	switch strings.ToLower(hari) {
	case "today":
		url = config.GetConfig().API.IndonesianPublicHoliday + "/api/today"
	case "tomorrow":
		url = config.GetConfig().API.IndonesianPublicHoliday + "/api/tomorrow"
	default:
		return nil, fmt.Errorf("invalid parameter: %s", hari)
	}

	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch API: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}
	var data IndonesianLibur
	if err := json.Unmarshal(bodyBytes, &data); err == nil {
		return &data, nil
	}
	var apiErr IndonesiaHolidayError
	if err := json.Unmarshal(bodyBytes, &apiErr); err == nil && apiErr.Message != "" {
		return nil, fmt.Errorf("API error: %s, details: %v", apiErr.Message, apiErr.Errors)
	}
	return nil, fmt.Errorf("failed to decode JSON: %w", err)
}

func GetYearHolidays(year int) ([]IndonesiaHoliday, error) {
	var url string
	if year == 0 {
		url = config.GetConfig().API.IndonesianPublicHoliday + "/api"
	} else {
		url = fmt.Sprintf("%s/api?year=%d", config.GetConfig().API.IndonesianPublicHoliday, year)
	}

	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch API: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}
	var holidays []IndonesiaHoliday
	if err := json.Unmarshal(bodyBytes, &holidays); err == nil {
		return holidays, nil
	}
	var apiErr IndonesiaHolidayError
	if err := json.Unmarshal(bodyBytes, &apiErr); err == nil && apiErr.Message != "" {
		return nil, fmt.Errorf("API error: %s, details: %v", apiErr.Message, apiErr.Errors)
	}
	return nil, fmt.Errorf("failed to decode JSON: %w", err)
}

func GetMonthBasedHolidaysOfCurrentYear(month int) ([]IndonesiaHoliday, error) {
	if month == 0 {
		return nil, errors.New("month parameter is required")
	}

	url := fmt.Sprintf("%s/api?month=%d", config.GetConfig().API.IndonesianPublicHoliday, month)

	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch API: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}
	var holidays []IndonesiaHoliday
	if err := json.Unmarshal(bodyBytes, &holidays); err == nil {
		return holidays, nil
	}
	var apiErr IndonesiaHolidayError
	if err := json.Unmarshal(bodyBytes, &apiErr); err == nil && apiErr.Message != "" {
		return nil, fmt.Errorf("API error: %s, details: %v", apiErr.Message, apiErr.Errors)
	}
	return nil, fmt.Errorf("failed to decode JSON: %w", err)
}

func GetHolidaysBasedOnYearAndMonth(year, month int) ([]IndonesiaHoliday, error) {
	if year == 0 || month == 0 {
		return nil, errors.New("year and month parameters are required")
	}

	url := fmt.Sprintf("%s/api?year=%d&month=%d", config.GetConfig().API.IndonesianPublicHoliday, year, month)

	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch API: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}
	var holidays []IndonesiaHoliday
	if err := json.Unmarshal(bodyBytes, &holidays); err == nil {
		return holidays, nil
	}
	var apiErr IndonesiaHolidayError
	if err := json.Unmarshal(bodyBytes, &apiErr); err == nil && apiErr.Message != "" {
		return nil, fmt.Errorf("API error: %s, details: %v", apiErr.Message, apiErr.Errors)
	}
	return nil, fmt.Errorf("failed to decode JSON: %w", err)
}

func GetHolidaysBasedOnYearMonthAndDay(year, month, day int) (*IndonesianLibur, error) {
	if year == 0 || month == 0 || day == 0 {
		return nil, errors.New("year, month, and day parameters are required")
	}

	url := fmt.Sprintf("%s/api?year=%d&month=%d&day=%d", config.GetConfig().API.IndonesianPublicHoliday, year, month, day)

	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch API: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}
	var data IndonesianLibur
	if err := json.Unmarshal(bodyBytes, &data); err == nil {
		return &data, nil
	}
	var apiErr IndonesiaHolidayError
	if err := json.Unmarshal(bodyBytes, &apiErr); err == nil && apiErr.Message != "" {
		return nil, fmt.Errorf("API error: %s, details: %v", apiErr.Message, apiErr.Errors)
	}
	return nil, fmt.Errorf("failed to decode JSON: %w", err)
}
