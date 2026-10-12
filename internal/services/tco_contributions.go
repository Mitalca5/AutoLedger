package services

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"time"

	"github.com/teslacost/teslacost/internal/money"
)

// Contribution categories that can be drilled down from a monthly total.
const (
	ContributionEnergy    = "ENERGY"
	ContributionInsurance = "INSURANCE"
)

// ErrUnsupportedContributionCategory is returned for a category without a drill-down.
var ErrUnsupportedContributionCategory = errors.New("unsupported contribution category")

// ErrInvalidContributionMonth is returned when the month is not a YYYY-MM value.
var ErrInvalidContributionMonth = errors.New("invalid contribution month")

var contributionMonthPattern = regexp.MustCompile(`^\d{4}-(0[1-9]|1[0-2])$`)

// MonthlyContributionItem is one recorded entry that makes up a monthly category total.
type MonthlyContributionItem struct {
	Date           time.Time   `json:"date"`
	SourceTable    string      `json:"source_table"`
	SourceID       string      `json:"source_id"`
	Label          string      `json:"label,omitempty"`
	Amount         money.Cents `json:"amount"`
	Recurring      bool        `json:"recurring,omitempty"`
	IntervalMonths int         `json:"interval_months,omitempty"`
}

// MonthlyContributionEstimate is the part of a monthly total that is estimated rather than recorded.
type MonthlyContributionEstimate struct {
	Amount money.Cents `json:"amount"`
	Km     float64     `json:"km"`
	Kwh    float64     `json:"kwh"`
}

// MonthlyContributions explains one category total of a month: the recorded entries and, for
// energy, the estimate added for distance driven before tracking started.
type MonthlyContributions struct {
	Month    string                       `json:"month"`
	Category string                       `json:"category"`
	Total    money.Cents                  `json:"total"`
	Items    []MonthlyContributionItem    `json:"items"`
	Estimate *MonthlyContributionEstimate `json:"estimate,omitempty"`
}

// ValidateContributionQuery checks the month and category of a drill-down request.
func ValidateContributionQuery(month, category string) error {
	if !contributionMonthPattern.MatchString(month) {
		return ErrInvalidContributionMonth
	}
	switch category {
	case ContributionEnergy, ContributionInsurance:
		return nil
	}
	return ErrUnsupportedContributionCategory
}

// assembleContributions builds the response from the recorded items and the month's computed
// figures, so the listed amounts always add up to the total shown on the monthly chart.
func assembleContributions(month, category string, items []MonthlyContributionItem, mc MonthlyCost) MonthlyContributions {
	out := MonthlyContributions{Month: month, Category: category, Items: items}
	if out.Items == nil {
		out.Items = []MonthlyContributionItem{}
	}
	switch category {
	case ContributionEnergy:
		out.Total = mc.Energy
		if mc.SmoothedEnergy != 0 {
			out.Estimate = &MonthlyContributionEstimate{Amount: mc.SmoothedEnergy, Km: mc.EstimatedEnergyKm, Kwh: mc.SmoothedKwh}
		}
	case ContributionInsurance:
		out.Total = mc.Insurance
	}
	return out
}

// MonthlyCostContributions lists the ledger entries behind one category of a month, using the
// same reporting timezone and filters as the monthly timeline.
func (s *TCOService) MonthlyCostContributions(ctx context.Context, vehicleID, month, category string) (*MonthlyContributions, error) {
	if err := ValidateContributionQuery(month, category); err != nil {
		return nil, err
	}

	summary, err := s.ComputeVehicleTCO(ctx, vehicleID)
	if err != nil {
		return nil, err
	}
	var mc MonthlyCost
	for _, m := range summary.MonthlyCosts {
		if m.Month == month {
			mc = m
			break
		}
	}

	rows, err := s.pool.Query(ctx, `
		SELECT l.entry_date, l.source_table, l.source_id::text, l.amount_eur,
		       COALESCE(m.description, c.address, ''),
		       COALESCE(m.is_recurring, FALSE),
		       COALESCE(m.recurrence_interval_months, 0)
		FROM cost_ledger l
		LEFT JOIN maintenance_expenses m ON l.source_table = 'maintenance_expenses' AND m.id = l.source_id
		LEFT JOIN charge_logs c ON l.source_table = 'charge_logs' AND c.id = l.source_id
		WHERE l.vehicle_id = $1
		  AND l.category = $2
		  AND TO_CHAR(l.entry_date AT TIME ZONE $3, 'YYYY-MM') = $4
		ORDER BY l.entry_date, l.source_id;
	`, vehicleID, category, s.timezone, month)
	if err != nil {
		return nil, fmt.Errorf("list contributions: %w", err)
	}
	defer rows.Close()

	var items []MonthlyContributionItem
	for rows.Next() {
		var it MonthlyContributionItem
		if err := rows.Scan(&it.Date, &it.SourceTable, &it.SourceID, &it.Amount, &it.Label, &it.Recurring, &it.IntervalMonths); err != nil {
			return nil, err
		}
		items = append(items, it)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	out := assembleContributions(month, category, items, mc)
	return &out, nil
}
