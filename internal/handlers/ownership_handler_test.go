package handlers

import (
	"encoding/json"
	"testing"

	"github.com/teslacost/teslacost/internal/models"
)

func decodeOwnership(t *testing.T, body string) *SaveOwnershipRequest {
	t.Helper()
	var req SaveOwnershipRequest
	if err := json.Unmarshal([]byte(body), &req); err != nil {
		t.Fatal(err)
	}
	return &req
}

func TestBuildOwnershipValidation(t *testing.T) {
	cases := []struct {
		name, body, wantErr string
	}{
		{"unknown type", `{"acquisition_type":"GIFT","start_date":"2025-01-01"}`, "ownership.mode_invalid"},
		{"missing start", `{"acquisition_type":"CASH","purchase_price":40000}`, "ownership.start_required"},
		{"cash without price", `{"acquisition_type":"CASH","start_date":"2025-01-01"}`, "ownership.price_required"},
		{"resale above cost", `{"acquisition_type":"CASH","start_date":"2025-01-01","purchase_price":30000,"incentives":5000,"expected_resale_value":26000}`, "ownership.resale_above_cost"},
		{"loan above price", `{"acquisition_type":"LOAN","start_date":"2025-01-01","purchase_price":30000,"loan_amount":35000,"loan_rate_pct":4,"loan_duration_months":60}`, "ownership.loan_above_cost"},
		{"loan without rate", `{"acquisition_type":"LOAN","start_date":"2025-01-01","purchase_price":30000,"loan_amount":20000,"loan_duration_months":60}`, "ownership.loan_rate"},
		{"lease without rent", `{"acquisition_type":"LLD","start_date":"2025-01-01","lease_duration_months":36}`, "ownership.rent_required"},
		{"option without price", `{"acquisition_type":"LOA","start_date":"2025-01-01","lease_monthly_rent":400,"lease_duration_months":36,"option_exercised_date":"2028-01-01"}`, "ownership.option_needs_price"},
		{"loa end value above start value", `{"acquisition_type":"LOA","start_date":"2025-01-01","lease_monthly_rent":400,"lease_duration_months":36,"purchase_price":30000,"expected_resale_value":31000}`, "ownership.resale_above_price"},
		{"lease resale without option", `{"acquisition_type":"LLD","start_date":"2025-01-01","lease_monthly_rent":400,"lease_duration_months":36,"end_date":"2028-01-01","sale_price":10000}`, "ownership.leased_no_resale"},
		{"sale without end date", `{"acquisition_type":"CASH","start_date":"2025-01-01","purchase_price":30000,"sale_price":15000}`, "ownership.resale_needs_end"},
		{"end before start", `{"acquisition_type":"CASH","start_date":"2025-01-01","purchase_price":30000,"end_date":"2024-01-01"}`, "ownership.end_before_start"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := buildOwnership("v", decodeOwnership(t, tc.body))
			if err == nil || errorCode(err) != tc.wantErr {
				t.Fatalf("expected error code %q, got %v", tc.wantErr, err)
			}
		})
	}
}

func TestBuildOwnershipKeepsOnlyRelevantFields(t *testing.T) {
	o, err := buildOwnership("v", decodeOwnership(t, `{
		"acquisition_type":"lld","start_date":"2025-01-01","lease_monthly_rent":"419.90","lease_duration_months":48,
		"lease_down_payment":2500,"lease_km_allowance_per_year":15000,"lease_excess_km_price":0.12,
		"lease_includes_maintenance":true,"purchase_price":45000,"loan_amount":20000,"lease_purchase_option_price":18000
	}`))
	if err != nil {
		t.Fatal(err)
	}
	if o.AcquisitionType != models.AcquisitionLLD || *o.LeaseMonthlyRent != 41990 || !o.LeaseIncludesMaintenance {
		t.Fatalf("unexpected lease: %+v", o)
	}
	if o.PurchasePrice != nil || o.LoanAmount != nil || o.LeasePurchaseOptionPrice != nil {
		t.Fatal("purchase, loan and purchase option fields do not apply to an LLD")
	}
}

func TestBuildOwnershipKeepsTheLOAValuesWhileTheVehicleIsLeased(t *testing.T) {
	o, err := buildOwnership("v", decodeOwnership(t, `{
		"acquisition_type":"LOA","start_date":"2025-01-01","lease_monthly_rent":330,"lease_duration_months":60,
		"lease_purchase_option_price":20000,"purchase_price":45000,"expected_resale_value":24000,"expected_holding_months":12
	}`))
	if err != nil {
		t.Fatal(err)
	}
	if o.PurchasePrice == nil || *o.PurchasePrice != 4500000 || o.ExpectedResaleValue == nil || *o.ExpectedResaleValue != 2400000 {
		t.Fatalf("the value at the start and at the end of the contract must be kept: %+v", o)
	}
	if o.ExpectedHoldingMonths != nil {
		t.Fatal("the holding period only applies once the purchase option is exercised")
	}
}
