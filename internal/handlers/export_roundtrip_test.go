package handlers

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/teslacost/teslacost/internal/models"
	"github.com/teslacost/teslacost/internal/services"
)

func TestExportRoundTripReproducesTheData(t *testing.T) {
	repo := authTestRepo(t)
	ctx := context.Background()
	u, err := repo.CreateUser(ctx, "export-roundtrip@example.com", "hash")
	if err != nil {
		t.Fatal(err)
	}
	newVehicle := func(name string) *models.Vehicle {
		v := &models.Vehicle{UserID: u.ID, Name: name, Powertrain: models.PowertrainPHEV, TeslaMateAuthType: models.AuthModeNone}
		if err := repo.CreateVehicle(ctx, v); err != nil {
			t.Fatal(err)
		}
		return v
	}
	src, dst := newVehicle("Source"), newVehicle("Empty")
	imp := services.NewCSVImportService(repo, "Europe/Paris")
	exp := services.NewExportService(repo)

	files := map[services.ImportType]string{
		services.ImportTypeCharges:  "date,kwh,cost,location\n2026-09-15 10:00,10.5,3.15,\"Home, garage\"\n2026-09-16 10:00,12,4,\n",
		services.ImportTypeDrives:   "start_time,end_time,distance_km,kwh,tag\n2026-09-15 08:00,2026-09-15 09:00,40,6.2,pro\n2026-09-17 08:00,2026-09-17 08:30,12,,\n",
		services.ImportTypeFuel:     "date,liters,amount,fuel_type,odometer_km\n2026-09-15,30,50.4,DIESEL,12000\n2026-09-20,20,33,SP98,12400\n",
		services.ImportTypeOdometer: "date,odometer_km,notes\n2026-09-01,11000,start\n",
	}
	for typ, csv := range files {
		res, err := imp.Execute(ctx, src, []byte(csv), services.CSVImportOptions{Type: typ, SkipDuplicates: true, DistanceUnit: "km", UserID: u.ID})
		if err != nil || !res.Committed {
			t.Fatalf("seed %s: %v %+v", typ, err, res)
		}
	}

	for _, unit := range []string{"km", "mi"} {
		dst := newVehicle("Empty-" + unit)
		for typ := range files {
			opts := services.ExportOptions{Type: services.ExportType(strings.ToLower(string(typ))), Format: services.ExportCSV, Unit: unit}
			out, err := exp.Export(ctx, src, opts)
			if err != nil {
				t.Fatalf("export %s: %v", typ, err)
			}
			res, err := imp.Execute(ctx, dst, out.Body, services.CSVImportOptions{Type: typ, SkipDuplicates: true, DistanceUnit: unit, UserID: u.ID})
			if err != nil || !res.Committed || res.ErrorCount != 0 {
				t.Fatalf("re-import %s (%s): %v %+v", typ, unit, err, res)
			}
			again, err := exp.Export(ctx, dst, opts)
			if err != nil {
				t.Fatal(err)
			}
			if string(again.Body) != string(out.Body) {
				t.Fatalf("%s (%s) differs after the round trip:\n%s\n---\n%s", typ, unit, out.Body, again.Body)
			}
		}
	}
	_ = dst

	charges, _, _ := repo.ListCharges(ctx, src.ID, false, 100, 0)
	if len(charges) != 2 {
		t.Fatalf("expected 2 charges, got %d", len(charges))
	}
}

func TestExportFiltersFormatAndSafety(t *testing.T) {
	repo := authTestRepo(t)
	ctx := context.Background()
	u, err := repo.CreateUser(ctx, "export-filters@example.com", "hash")
	if err != nil {
		t.Fatal(err)
	}
	v := &models.Vehicle{UserID: u.ID, Name: "Car", TeslaMateAuthType: models.AuthModeNone}
	if err := repo.CreateVehicle(ctx, v); err != nil {
		t.Fatal(err)
	}
	imp := services.NewCSVImportService(repo, "UTC")
	seed := "start_time,end_time,distance_km,tag,start_address\n" +
		"2026-09-01 08:00,2026-09-01 09:00,10,pro,=HYPERLINK(\"x\")\n" +
		"2026-10-01 08:00,2026-10-01 09:00,20,perso,Home\n"
	if res, err := imp.Execute(ctx, v, []byte(seed), services.CSVImportOptions{Type: services.ImportTypeDrives, DistanceUnit: "km", UserID: u.ID}); err != nil || !res.Committed {
		t.Fatalf("seed: %v %+v", err, res)
	}
	exp := services.NewExportService(repo)

	from := time.Date(2026, 9, 15, 0, 0, 0, 0, time.UTC)
	out, err := exp.Export(ctx, v, services.ExportOptions{Type: services.ExportDrives, Format: services.ExportCSV, From: &from})
	if err != nil || strings.Count(string(out.Body), "\n") != 2 || strings.Contains(string(out.Body), "2026-09-01") {
		t.Fatalf("the period must exclude September 1st: %v\n%s", err, out.Body)
	}
	out, _ = exp.Export(ctx, v, services.ExportOptions{Type: services.ExportDrives, Format: services.ExportCSV, Tag: "pro"})
	if !strings.Contains(string(out.Body), "'=HYPERLINK") || strings.Contains(string(out.Body), "perso") {
		t.Fatalf("tag filter or formula guard failed:\n%s", out.Body)
	}

	out, err = exp.Export(ctx, v, services.ExportOptions{Type: services.ExportDrives, Format: services.ExportJSON, Unit: "mi"})
	if err != nil {
		t.Fatal(err)
	}
	var rows []map[string]any
	if err := json.Unmarshal(out.Body, &rows); err != nil || len(rows) != 2 {
		t.Fatalf("invalid JSON: %v %s", err, out.Body)
	}
	if _, ok := rows[0]["distance_mi"]; !ok || rows[0]["end_address"] != nil {
		t.Fatalf("expected distance_mi and a null end_address: %v", rows[0])
	}

	if _, err := exp.Export(ctx, v, services.ExportOptions{Type: "secrets", Format: services.ExportCSV}); err == nil {
		t.Fatal("an unknown type must be refused")
	}
	if _, err := exp.Export(ctx, v, services.ExportOptions{Type: services.ExportCharges, Format: "xml"}); err == nil {
		t.Fatal("an unknown format must be refused")
	}
	for _, typ := range []services.ExportType{services.ExportCharges, services.ExportFuel, services.ExportOdometer, services.ExportExpenses, services.ExportMaintenance} {
		out, err := exp.Export(ctx, v, services.ExportOptions{Type: typ, Format: services.ExportJSON})
		if err != nil || string(out.Body) != "[]" {
			t.Fatalf("empty %s must export an empty list: %v %s", typ, err, out.Body)
		}
	}
}
