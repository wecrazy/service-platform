package fun

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// NullAbleTime represents a nullable time value.
type NullAbleTime struct {
	Time  time.Time
	Valid bool
}

// NullAbleString represents a nullable string value.
type NullAbleString struct {
	String string
	Valid  bool
}

// UnmarshalJSON implements json.Unmarshaler for NullAbleString.
func (ns *NullAbleString) UnmarshalJSON(data []byte) error {
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

// NullAbleFloat represents a nullable float value, typically used for ID fields.
type NullAbleFloat struct {
	Float float64
	Valid bool
}

// UnmarshalJSON implements json.Unmarshaler for NullAbleFloat.
func (nf *NullAbleFloat) UnmarshalJSON(data []byte) error {
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

// NullAbleInterface represents a nullable interface value, useful for arrays or mixed types.
type NullAbleInterface struct {
	Data  interface{}
	Valid bool
}

// UnmarshalJSON implements json.Unmarshaler for NullAbleInterface.
func (ni *NullAbleInterface) UnmarshalJSON(data []byte) error {
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

// IsEmpty returns true if the NullAbleInterface is invalid or nil.
func (ni NullAbleInterface) IsEmpty() bool {
	return !ni.Valid || ni.Data == nil
}

// ToIntSlice converts the data to a slice of ints if possible.
func (ni NullAbleInterface) ToIntSlice() []int {
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

// NullAbleInteger represents a nullable integer value.
type NullAbleInteger struct {
	Int   int
	Valid bool
}

// UnmarshalJSON implements json.Unmarshaler for NullAbleInteger.
func (ni *NullAbleInteger) UnmarshalJSON(data []byte) error {
	// Handle null and false as invalid
	if string(data) == "null" || string(data) == "false" {
		ni.Int = 0
		ni.Valid = false
		return nil
	}

	// Try to unmarshal into an integer
	var temp int
	if err := json.Unmarshal(data, &temp); err != nil {
		return fmt.Errorf("failed to unmarshal NullAbleInteger: %w", err)
	}

	ni.Int = temp
	ni.Valid = true
	return nil
}

// IsEmpty returns true if the NullAbleInteger is invalid.
func (ni NullAbleInteger) IsEmpty() bool {
	return !ni.Valid
}

// NullAbleArrayInteger represents a nullable array of integers.
type NullAbleArrayInteger struct {
	Ints  []int
	Valid bool
}

// UnmarshalJSON implements json.Unmarshaler for NullAbleArrayInteger.
func (nai *NullAbleArrayInteger) UnmarshalJSON(data []byte) error {
	if string(data) == "null" || string(data) == "false" {
		nai.Ints = nil
		nai.Valid = false
		return nil
	}

	if err := json.Unmarshal(data, &nai.Ints); err != nil {
		return fmt.Errorf("failed to unmarshal NullAbleArrayInteger: %w", err)
	}
	nai.Valid = true
	return nil
}

// IsEmpty returns true if the NullAbleArrayInteger is invalid or empty.
func (nai NullAbleArrayInteger) IsEmpty() bool {
	return !nai.Valid || len(nai.Ints) == 0
}

// NullAbleBoolean represents a nullable boolean value.
type NullAbleBoolean struct {
	Bool  bool
	Valid bool
}

// UnmarshalJSON implements json.Unmarshaler for NullAbleBoolean.
func (nb *NullAbleBoolean) UnmarshalJSON(data []byte) error {
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

// IsEmpty returns true if the NullAbleBoolean is invalid.
func (nb NullAbleBoolean) IsEmpty() bool {
	return !nb.Valid
}

// ParseJSONIDDataCombined parses combined ID and string data from a nullable interface.
func ParseJSONIDDataCombined(nullableData NullAbleInterface) (int, string, error) {
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

// ParseJSONIDDataCombinedSafe is an optimized version of ParseJSONIDDataCombined that doesn't return errors for better performance.
func ParseJSONIDDataCombinedSafe(nullableData NullAbleInterface) (int, string) {
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
