package services

import (
	"context"
	"testing"

	"github.com/teslacost/teslacost/internal/models"
)

func TestCSVImportRejectsInvalidAmountsBeforeConversion(t *testing.T) {
	for _, value := range []string{"NaN", "+Inf", "-Inf", "1000000000000000000", "100000000", "-0.01"} {
		for _, kind := range []string{"charge", "fuel"} {
			t.Run(kind+"/"+value, func(t *testing.T) {
				csv, powertrain, code := "date,kwh,cost\n2026-09-15,10,"+value+"\n", models.PowertrainEV, "import.row.invalid_cost"
				if kind == "fuel" {
					csv, powertrain, code = "date,liters,amount\n2026-09-15,10,"+value+"\n", models.PowertrainICE, "import.row.invalid_amount"
				}
				res := previewOf(t, testVehicle(powertrain), csv, CSVImportOptions{})
				if res.ValidRows != 0 || res.InvalidRows != 1 || len(res.Errors) != 1 || res.Errors[0].Code != code {
					t.Fatalf("expected %s, got %+v", code, res)
				}
			})
		}
	}
}

func TestCSVImportAcceptsFreeAndMaximumChargeAmounts(t *testing.T) {
	for _, value := range []string{"0", "5.25", "99999999.99"} {
		res := previewOf(t, testVehicle(models.PowertrainEV), "date,kwh,cost\n2026-09-15,10,"+value+"\n", CSVImportOptions{})
		if res.ValidRows != 1 || res.InvalidRows != 0 {
			t.Fatalf("%s: %+v", value, res)
		}
	}
}

func TestCSVImportInvalidAmountFailsBeforeDatabaseWrites(t *testing.T) {
	// A nil repository makes a database write panic: validation must reject the batch first.
	svc := NewCSVImportService(nil, "Europe/Paris")
	for _, value := range []string{"NaN", "+Inf", "1000000000000000000"} {
		res, err := svc.Execute(context.Background(), testVehicle(models.PowertrainEV), []byte("date,kwh,cost\n2026-09-14,20,5\n2026-09-15,10,"+value+"\n"), CSVImportOptions{})
		if err != nil || res.Committed || res.ErrorCount != 1 || res.Errors[0].Code != "import.row.invalid_cost" {
			t.Fatalf("%s: %+v, %v", value, res, err)
		}
	}
}
