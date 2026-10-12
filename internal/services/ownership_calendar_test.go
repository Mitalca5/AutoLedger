package services

import (
	"testing"

	"github.com/teslacost/teslacost/internal/models"
	"github.com/teslacost/teslacost/internal/money"
)

func TestLeaseCalendarMonthEnd(t *testing.T) {
	for _, year := range []int{2024, 2026} {
		duration := 1
		rent := money.FromFloat(300)
		o := &models.VehicleOwnership{AcquisitionType: models.AcquisitionLLD, StartDate: d(year, 1, 31), LeaseDurationMonths: &duration, LeaseMonthlyRent: &rent, LeaseIncludesInsurance: true}
		now := d(year, 3, 1)
		got := ComputeOwnershipCosts(o, now, 0)
		if got.ContractEndDate == nil || got.ContractEndDate.Month() != 2 || got.IncludesInsurance || o.InLeasePhase(now) {
			t.Fatalf("lease overflow: %+v", got)
		}
	}
}
