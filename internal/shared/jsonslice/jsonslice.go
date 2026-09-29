package jsonslice

import (
	"database/sql/driver"
	"encoding/json"
)

// Strings is a JSON-backed string slice suitable for SQL value/scan contracts.
type Strings []string

func (items Strings) Value() (driver.Value, error) {
	if items == nil {
		return nil, nil
	}
	return json.Marshal(items)
}

func (items *Strings) Scan(value interface{}) error {
	if value == nil {
		*items = Strings{}
		return nil
	}
	data, ok := value.([]byte)
	if !ok {
		return nil
	}
	return json.Unmarshal(data, items)
}

// Uints is a JSON-backed unsigned-integer slice suitable for SQL value/scan contracts.
type Uints []uint

func (items Uints) Value() (driver.Value, error) {
	if items == nil {
		return nil, nil
	}
	return json.Marshal(items)
}

func (items *Uints) Scan(value interface{}) error {
	if value == nil {
		*items = Uints{}
		return nil
	}
	data, ok := value.([]byte)
	if !ok {
		return nil
	}
	return json.Unmarshal(data, items)
}
