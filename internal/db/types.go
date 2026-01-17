package db

import (
	"database/sql/driver"
	"encoding/json"
)

type Strings []string

func (c Strings) Value() (driver.Value, error) {
	b, err := json.Marshal(c)
	return string(b), err
}

func (c *Strings) Scan(input any) error {
	return json.Unmarshal(input.([]byte), c)
}

type Int64s []int64

func (c Int64s) Value() (driver.Value, error) {
	b, err := json.Marshal(c)
	return string(b), err
}

func (c *Int64s) Scan(input any) error {
	return json.Unmarshal(input.([]byte), c)
}

type BitBool bool

// Value implements the driver.Valuer interface,
// and turns the BitBool into a bitfield (BIT(1)) for MySQL storage.
func (b BitBool) Value() (driver.Value, error) {
	if b {
		return []byte{1}, nil
	} else {
		return []byte{0}, nil
	}
}

// Scan implements the sql.Scanner interface,
// and turns the bitfield incoming from MySQL into a BitBool
func (b *BitBool) Scan(src any) error {
	v, _ := src.([]byte)
	*b = v[0] == 1
	return nil
}
