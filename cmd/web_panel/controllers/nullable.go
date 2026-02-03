package controllers

import (
	"encoding/json"
	"errors"
	"time"
)

type nullAbleTime struct {
	Time  time.Time
	Valid bool
}

type nullAbleString struct {
	String string
	Valid  bool
}

func (ns *nullAbleString) UnmarshalJSON(data []byte) error {
	if string(data) == "null" || string(data) == "false" {
		ns.String = ""
		ns.Valid = false
		return nil
	}

	if err := json.Unmarshal(data, &ns.String); err != nil {
		return err
	}
	ns.Valid = true
	return nil
}

// Nullable Float Type (For ID fields)
type nullAbleFloat struct {
	Float float64
	Valid bool
}

func (nf *nullAbleFloat) UnmarshalJSON(data []byte) error {
	if string(data) == "null" || string(data) == "false" {
		nf.Float = 0
		nf.Valid = false
		return nil
	}

	if err := json.Unmarshal(data, &nf.Float); err != nil {
		return err
	}
	nf.Valid = true
	return nil
}

// Nullable Interface (For arrays or mixed types)
type nullAbleInterface struct {
	Data  interface{}
	Valid bool
}

func (ni *nullAbleInterface) UnmarshalJSON(data []byte) error {
	if string(data) == "null" || string(data) == "false" {
		ni.Data = nil
		ni.Valid = false
		return nil
	}

	var temp interface{}
	if err := json.Unmarshal(data, &temp); err != nil {
		return err
	}

	ni.Data = temp
	ni.Valid = true
	return nil
}

func (ni nullAbleInterface) IsEmpty() bool {
	return !ni.Valid || ni.Data == nil
}

func (ni nullAbleInterface) ToIntSlice() []int {
	if ni.Data == nil || !ni.Valid {
		return []int{}
	}

	// Try to assert the data as a slice of interfaces
	if dataSlice, ok := ni.Data.([]interface{}); ok {
		intSlice := make([]int, len(dataSlice))
		for i, v := range dataSlice {
			// Convert each value to int
			if num, ok := v.(float64); ok {
				intSlice[i] = int(num) // Convert float64 to int
			}
		}
		return intSlice
	}

	// Return empty slice if conversion fails
	return []int{}
}

type nullAbleInteger struct {
	Int   int
	Valid bool
}

func (ni *nullAbleInteger) UnmarshalJSON(data []byte) error {
	// Handle null and false as invalid
	if string(data) == "null" || string(data) == "false" {
		ni.Int = 0
		ni.Valid = false
		return nil
	}

	// Try to unmarshal into an integer
	var temp int
	if err := json.Unmarshal(data, &temp); err != nil {
		return errors.New("invalid type for nullAbleInteger")
	}

	ni.Int = temp
	ni.Valid = true
	return nil
}

func (ni nullAbleInteger) IsEmpty() bool {
	return !ni.Valid
}

type nullAbleArrayInteger struct {
	Ints  []int
	Valid bool
}

func (nai *nullAbleArrayInteger) UnmarshalJSON(data []byte) error {
	if string(data) == "null" || string(data) == "false" {
		nai.Ints = nil
		nai.Valid = false
		return nil
	}

	if err := json.Unmarshal(data, &nai.Ints); err != nil {
		return err
	}
	nai.Valid = true
	return nil
}

func (nai nullAbleArrayInteger) IsEmpty() bool {
	return !nai.Valid || len(nai.Ints) == 0
}

type nullAbleBoolean struct {
	Bool  bool
	Valid bool
}

func (nb *nullAbleBoolean) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		nb.Bool = false
		nb.Valid = false
		return nil
	}

	if err := json.Unmarshal(data, &nb.Bool); err != nil {
		return err
	}
	nb.Valid = true
	return nil
}

func (nb nullAbleBoolean) IsEmpty() bool {
	return !nb.Valid
}

func parseJSONIDDataCombined(nullableData nullAbleInterface) (int, string, error) {
	if nullableData.IsEmpty() {
		return 0, "", nil // Return default values for empty data
	}

	arrayData, ok := nullableData.Data.([]interface{})
	if !ok || len(arrayData) < 2 {
		return 0, "", errors.New("invalid array data")
	}

	dataIDFloat, ok := arrayData[0].(float64)
	if !ok {
		return 0, "", errors.New("invalid type for data ID; expected float64")
	}
	dataID := int(dataIDFloat)

	dataString, ok := arrayData[1].(string)
	if !ok {
		return 0, "", errors.New("invalid type for data string; expected string")
	}

	return dataID, dataString, nil
}

// parseJSONIDDataCombinedSafe is an optimized version that doesn't log errors for better performance
func parseJSONIDDataCombinedSafe(nullableData nullAbleInterface) (int, string) {
	if nullableData.IsEmpty() {
		return 0, ""
	}

	arrayData, ok := nullableData.Data.([]interface{})
	if !ok || len(arrayData) < 2 {
		return 0, ""
	}

	dataIDFloat, ok := arrayData[0].(float64)
	if !ok {
		return 0, ""
	}
	dataID := int(dataIDFloat)

	dataString, ok := arrayData[1].(string)
	if !ok {
		return 0, ""
	}

	return dataID, dataString
}
