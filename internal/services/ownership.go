package services

import (
	"fmt"
	"math"
	"time"

	"github.com/teslacost/teslacost/internal/models"
	"github.com/teslacost/teslacost/internal/money"
)

// OwnershipCosts holds the economic (non-cash) adjustments derived from the ownership contract.
// Cash flows (purchase, rents, fees, interest) are entries of the cost ledger; these values turn them into
// a cost of ownership spread over time.
type OwnershipCosts struct {
	// Depreciation of an owned vehicle (purchase, or LOA after the purchase option is exercised).
	Depreciation money.Cents
	// DepreciationUnknown is set when a bought vehicle has no depreciation because the price, the resale value or the
	// holding period is missing: its full cost leaves out the largest item, which is not the same as a zero.
	DepreciationUnknown bool
	// LeasePrepaidAdjustment spreads the down payment and application fees over the contract duration:
	// negative while the contract runs (part of the cash paid upfront is not consumed yet).
	LeasePrepaidAdjustment money.Cents
	// LeaseEndFeesAdjustment accrues the estimated return fees over the contract duration.
	LeaseEndFeesAdjustment money.Cents
	// LeaseExcessKm is the excess-mileage penalty accrued so far against the pro-rata allowance.
	LeaseExcessKm money.Cents
	// LeaseExcessKmProjected is the penalty expected at the end of the contract at the current pace.
	LeaseExcessKmProjected money.Cents
	LeaseKmDriven          float64
	LeaseKmAllowanceToDate float64
	ContractEndDate        *time.Time

	IncludesMaintenance bool
	IncludesInsurance   bool
	IncludesTires       bool

	// Missing lists the settings needed for a complete cost of ownership.
	Missing []string
	// MissingFields names the depreciation inputs not entered: "purchase_price", "resale_value", "holding_period".
	MissingFields []string
}

// monthsBetween returns the number of months (fractional) between two dates.
func monthsBetween(from, to time.Time) float64 {
	return to.Sub(from).Hours() / 24 / (365.25 / 12)
}

func clamp01(v float64) float64 {
	return math.Max(0, math.Min(1, v))
}

func centsOrZero(c *money.Cents) money.Cents {
	if c == nil {
		return 0
	}
	return *c
}

// ComputeOwnershipCosts derives depreciation and lease adjustments at a given time.
// kmSinceStart is the distance driven since the start of the contract.
func ComputeOwnershipCosts(o *models.VehicleOwnership, now time.Time, kmSinceStart float64) OwnershipCosts {
	var c OwnershipCosts
	if o == nil {
		c.Missing = append(c.Missing, "Acquisition mode not entered (cash, loan, LOA or LLD): depreciation or lease payments missing from the full cost")
		return c
	}

	switch o.AcquisitionType {
	case models.AcquisitionCash, models.AcquisitionLoan:
		if o.PurchasePrice == nil || *o.PurchasePrice <= 0 {
			c.Missing = append(c.Missing, "Purchase price not entered")
			c.MissingFields = append(c.MissingFields, "purchase_price")
			c.DepreciationUnknown = true
		} else {
			base := *o.PurchasePrice + centsOrZero(o.PurchaseFees) - centsOrZero(o.Incentives)
			dep, ok := depreciation(base, o.StartDate, o, now)
			c.Depreciation = dep
			if !ok {
				c.DepreciationUnknown = true
				if o.ExpectedResaleValue == nil {
					c.MissingFields = append(c.MissingFields, "resale_value")
				}
				if o.ExpectedHoldingMonths == nil || *o.ExpectedHoldingMonths <= 0 {
					c.MissingFields = append(c.MissingFields, "holding_period")
				}
				c.Missing = append(c.Missing, "Estimated resale value or holding period not entered: depreciation excluded from the full cost")
			}
		}
		if o.AcquisitionType == models.AcquisitionLoan && (o.LoanAmount == nil || *o.LoanAmount <= 0 ||
			o.LoanDurationMonths == nil || *o.LoanDurationMonths <= 0 || o.LoanRatePct == nil) {
			c.Missing = append(c.Missing, "Incomplete loan (amount borrowed, rate or duration): interest not calculated")
		}

	case models.AcquisitionLOA, models.AcquisitionLLD:
		computeLease(&c, o, now, kmSinceStart)
	}
	return c
}

// depreciation of an owned vehicle: realized at sale, otherwise spread linearly over the expected holding period.
func depreciation(base money.Cents, from time.Time, o *models.VehicleOwnership, now time.Time) (money.Cents, bool) {
	if o.EndDate != nil && !now.Before(*o.EndDate) && o.SalePrice != nil {
		return base - *o.SalePrice, true
	}
	if o.ExpectedResaleValue == nil || o.ExpectedHoldingMonths == nil || *o.ExpectedHoldingMonths <= 0 {
		return 0, false
	}
	until := now
	if o.EndDate != nil && o.EndDate.Before(until) {
		until = *o.EndDate
	}
	depreciable := base - *o.ExpectedResaleValue
	if depreciable <= 0 {
		return 0, true
	}
	ratio := clamp01(monthsBetween(from, until) / float64(*o.ExpectedHoldingMonths))
	return money.FromFloat(depreciable.Float() * ratio), true
}

func computeLease(c *OwnershipCosts, o *models.VehicleOwnership, now time.Time, kmSinceStart float64) {
	if o.LeaseDurationMonths != nil && *o.LeaseDurationMonths > 0 {
		contractEnd := models.AddCalendarMonths(o.StartDate, *o.LeaseDurationMonths)
		c.ContractEndDate = &contractEnd
	}
	if o.LeaseMonthlyRent == nil || *o.LeaseMonthlyRent <= 0 || o.LeaseDurationMonths == nil || *o.LeaseDurationMonths <= 0 {
		c.Missing = append(c.Missing, "Incomplete lease (monthly rent or duration): rent not counted")
		return
	}
	duration := *o.LeaseDurationMonths
	contractEnd := *c.ContractEndDate

	// The lease phase ends at the contract term, at an early return, or when the purchase option is exercised.
	phaseEnd := contractEnd
	if o.EndDate != nil && o.EndDate.Before(phaseEnd) {
		phaseEnd = *o.EndDate
	}
	optionExercised := o.AcquisitionType == models.AcquisitionLOA && o.OptionExercisedDate != nil && !now.Before(*o.OptionExercisedDate)
	if o.OptionExercisedDate != nil && o.OptionExercisedDate.Before(phaseEnd) {
		phaseEnd = *o.OptionExercisedDate
	}
	started := !now.Before(o.StartDate)
	ended := !now.Before(phaseEnd)

	ratio := 0.0
	switch {
	case ended:
		ratio = 1
	case started:
		ratio = clamp01(monthsBetween(o.StartDate, now) / float64(duration))
	}

	inLease := started && !ended
	c.IncludesMaintenance = inLease && o.LeaseIncludesMaintenance
	c.IncludesInsurance = inLease && o.LeaseIncludesInsurance
	c.IncludesTires = inLease && o.LeaseIncludesTires

	if started {
		prepaid := centsOrZero(o.LeaseDownPayment) + centsOrZero(o.LeaseFees)
		c.LeasePrepaidAdjustment = money.FromFloat(prepaid.Float()*ratio) - prepaid
	}

	if o.LeaseEndFeesEstimate != nil && !optionExercised {
		accrued := money.FromFloat(o.LeaseEndFeesEstimate.Float() * ratio)
		var recognized money.Cents
		if ended {
			recognized = *o.LeaseEndFeesEstimate
		}
		c.LeaseEndFeesAdjustment = accrued - recognized
	}

	if o.LeaseKmAllowancePerYear == nil || o.LeaseExcessKmPrice == nil {
		c.Missing = append(c.Missing, "Mileage allowance or price per extra kilometre not entered: overage penalty not estimated")
	} else if started && !optionExercised {
		elapsed := math.Min(monthsBetween(o.StartDate, now), monthsBetween(o.StartDate, phaseEnd))
		c.LeaseKmDriven = kmSinceStart
		c.LeaseKmAllowanceToDate = *o.LeaseKmAllowancePerYear * elapsed / 12
		c.LeaseExcessKm = money.FromFloat(math.Max(0, kmSinceStart-c.LeaseKmAllowanceToDate) * *o.LeaseExcessKmPrice)
		if elapsed >= 1 {
			projectedKm := kmSinceStart / elapsed * float64(duration)
			totalAllowance := *o.LeaseKmAllowancePerYear * float64(duration) / 12
			c.LeaseExcessKmProjected = money.FromFloat(math.Max(0, projectedKm-totalAllowance) * *o.LeaseExcessKmPrice)
		}
	}

	if o.AcquisitionType == models.AcquisitionLOA {
		if o.LeasePurchaseOptionPrice == nil {
			c.Missing = append(c.Missing, "LOA purchase option price not entered")
		}
		if optionExercised && o.LeasePurchaseOptionPrice != nil {
			dep, ok := depreciation(*o.LeasePurchaseOptionPrice, *o.OptionExercisedDate, o, now)
			c.Depreciation = dep
			if !ok {
				c.DepreciationUnknown = true
				c.Missing = append(c.Missing, "Purchase option exercised: estimated resale value or holding period not entered, depreciation excluded")
			}
		}
	}
}

// LeaseWarnings returns informational warnings about the lease contract (mileage pace, contract end).
func (c OwnershipCosts) LeaseWarnings(now time.Time) []string {
	var warnings []string
	if c.LeaseExcessKmProjected > 0 {
		warnings = append(warnings, fmt.Sprintf(
			"At the current pace the mileage allowance will be exceeded: estimated end-of-contract penalty of %s", c.LeaseExcessKmProjected))
	}
	if c.ContractEndDate != nil && now.Before(*c.ContractEndDate) && monthsBetween(now, *c.ContractEndDate) <= 3 {
		warnings = append(warnings, fmt.Sprintf(
			"The lease ends on %s: consider the purchase option or the return", c.ContractEndDate.Format("02/01/2006")))
	}
	return warnings
}
