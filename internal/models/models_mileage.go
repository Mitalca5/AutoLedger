package models

import "github.com/teslacost/teslacost/internal/money"

// MileageRate is one slice of a mileage allowance scale typed by the user. The slice covers the cumulative
// distance of the year from FromKm up to ToKm (open-ended when nil); rates sharing a label form one scale.
type MileageRate struct {
	ID        string  `json:"id"`
	UserID    string  `json:"-"`
	Label     string  `json:"label"`
	Year      int     `json:"year"`
	FromKm    int     `json:"from_km"`
	ToKm      *int    `json:"to_km"`
	RatePerKm float64 `json:"rate_per_km"`
}

// MileageTagTotal sums the trips of one tag over the period; an empty tag is the untagged trips.
type MileageTagTotal struct {
	Tag        string      `json:"tag"`
	Trips      int         `json:"trips"`
	DistanceKm float64     `json:"distance_km"`
	Tolls      money.Cents `json:"tolls"`
	// Allowance is the scale applied to the distance, nil when no scale was chosen.
	Allowance *money.Cents `json:"allowance"`
}

// MileageReport totals the trips per tag over a period.
type MileageReport struct {
	From      string            `json:"from,omitempty"`
	To        string            `json:"to,omitempty"`
	RateLabel string            `json:"rate_label,omitempty"`
	Currency  string            `json:"currency"`
	Tags      []MileageTagTotal `json:"tags"`
}
