package models

import "time"

// ImportProfile is a saved CSV column mapping, reusable on any vehicle of its owner.
type ImportProfile struct {
	ID               string            `json:"id"`
	Name             string            `json:"name"`
	ImportType       string            `json:"import_type"`
	Columns          map[string]string `json:"columns"`
	DateOrder        string            `json:"date_order"`
	DecimalSeparator string            `json:"decimal_separator"`
	CreatedAt        time.Time         `json:"created_at"`
}
