package model

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
)

// JSONSlice is a []string that serializes to/from JSON TEXT in the database.
type JSONSlice []string

// Value implements driver.Valuer for database storage.
func (j JSONSlice) Value() (driver.Value, error) {
	if j == nil {
		return "[]", nil
	}
	b, err := json.Marshal(j)
	if err != nil {
		return nil, fmt.Errorf("marshaling JSONSlice: %w", err)
	}
	return string(b), nil
}

// Scan implements sql.Scanner for database retrieval.
func (j *JSONSlice) Scan(value interface{}) error {
	if value == nil {
		*j = nil
		return nil
	}
	var bytes []byte
	switch v := value.(type) {
	case string:
		bytes = []byte(v)
	case []byte:
		bytes = v
	default:
		return fmt.Errorf("JSONSlice.Scan: unsupported type %T", value)
	}
	return json.Unmarshal(bytes, j)
}

// JSONMap is a map[string]string that serializes to/from JSON TEXT in the database.
type JSONMap map[string]string

// Value implements driver.Valuer for database storage.
func (j JSONMap) Value() (driver.Value, error) {
	if j == nil {
		return "{}", nil
	}
	b, err := json.Marshal(j)
	if err != nil {
		return nil, fmt.Errorf("marshaling JSONMap: %w", err)
	}
	return string(b), nil
}

// Scan implements sql.Scanner for database retrieval.
func (j *JSONMap) Scan(value interface{}) error {
	if value == nil {
		*j = nil
		return nil
	}
	var bytes []byte
	switch v := value.(type) {
	case string:
		bytes = []byte(v)
	case []byte:
		bytes = v
	default:
		return fmt.Errorf("JSONMap.Scan: unsupported type %T", value)
	}
	return json.Unmarshal(bytes, j)
}
