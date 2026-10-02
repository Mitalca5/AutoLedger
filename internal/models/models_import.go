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

// ImportBatch is one CSV import: the rows it created carry its id until the batch is undone.
type ImportBatch struct {
	ID         string    `json:"id"`
	VehicleID  string    `json:"vehicle_id"`
	ImportType string    `json:"import_type"`
	RowCount   int       `json:"row_count"`
	CreatedAt  time.Time `json:"created_at"`
	// Remaining counts the rows still in place: edits and deletions after the import lower it.
	Remaining int `json:"remaining"`
}
