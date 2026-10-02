package services

import (
	"context"
	"testing"
	"time"

	"github.com/teslacost/teslacost/internal/models"
)

func TestNormalizeHeader(t *testing.T) {
	for raw, want := range map[string]string{
		"Distance (km)":    "distance_km",
		" Odomètre-MI ":    "odometer_mi",
		"\uFEFFDate":       "date",
		"Coût":             "cost",
		"Énergie (kWh)":    "kwh",
		"Prix par litre":   "price_per_liter",
		"Taux de change":   "fx_rate",
		"Kilométrage (km)": "odometer_km",
		"Total_Cost_EUR":   "total_cost_eur",
		"Duree_min":        "duration_min",
		"Étiquettes":       "tags",
		"Commentaire":      "notes",
		"Montant (€)":      "amount",
		"Total Cost (€)":   "total_cost",
		"unknown column":   "unknown_column",
		"odometer_km":      "odometer_km",
		"start_time":       "start_time",
		"Date/heure":       "datetime",
		"":                 "",
	} {
		if got := normalizeHeader(raw); got != want {
			t.Errorf("normalizeHeader(%q) = %q, want %q", raw, got, want)
		}
	}
}

func TestCanonicalHeadersAddressesNextToADate(t *testing.T) {
	got := canonicalHeaders([]string{"ID", "Date", "Start", "Destination"})
	if got[2] != "start_address" || got[3] != "end_address" {
		t.Errorf("start/end next to a date are addresses, got %v", got)
	}
	got = canonicalHeaders([]string{"start", "end", "distance"})
	if got[0] != "start" || got[1] != "end" {
		t.Errorf("alone, start/end stay the times, got %v", got)
	}
}

// The files the drives page exports must be importable again, in either language.
func TestImportReadsTheAppsOwnDriveExport(t *testing.T) {
	ev := testVehicle(models.PowertrainEV)
	for name, data := range map[string]string{
		"en": "ID,Date,Start,Destination,Distance_km,Duration_min,Consumption_kWh_100km,Energy_kWh,Total_Cost_EUR,Cost_km_EUR,Tags\n" +
			"d1,2026-09-15T08:00,\"Nantes\",\"Rennes\",45.5,52,15.8,7.2,1.80,0.040,\"Pro\"\n",
		"fr": "ID,Date,Depart,Arrivee,Distance_km,Duree_min,Conso_kWh_100km,Energie_kWh,Cout_Total_EUR,Cout_km_EUR,Tags\n" +
			"d1,2026-09-15T08:00,\"Nantes\",\"Rennes\",45.5,52,15.8,7.2,1.80,0.040,\"Pro\"\n",
	} {
		t.Run(name, func(t *testing.T) {
			res := previewOf(t, ev, data, CSVImportOptions{})
			if res.Type != ImportTypeDrives || res.ValidRows != 1 || res.InvalidRows != 0 {
				t.Fatalf("unexpected preview: %+v (%v)", res, rowCodes(res))
			}
		})
	}
}

func TestImportReadsFrenchHeaders(t *testing.T) {
	ev := testVehicle(models.PowertrainEV)
	res := previewOf(t, ev, "Date;Énergie (kWh);Coût;Devise;Lieu;Kilométrage (km)\n15/09/2026 14:30;42,5;18,50;EUR;Ionity;12345\n", CSVImportOptions{})
	if res.Type != ImportTypeCharges || res.ValidRows != 1 {
		t.Errorf("unexpected charges preview: %+v (%v)", res, rowCodes(res))
	}
	ice := testVehicle(models.PowertrainICE)
	res = previewOf(t, ice, "Date;Litres;Prix par litre;Carburant\n15/09/2026;40;1,80;SP95_E10\n", CSVImportOptions{})
	if res.Type != ImportTypeFuel || res.ValidRows != 1 {
		t.Errorf("unexpected fuel preview: %+v (%v)", res, rowCodes(res))
	}
}

func TestPreviewReportsTheDetectedMapping(t *testing.T) {
	ev := testVehicle(models.PowertrainEV)
	res := previewOf(t, ev, "Date,Energie,Bonus\n2026-09-15,42.5,x\n", CSVImportOptions{Type: ImportTypeCharges})
	if len(res.Mapping) != 3 || res.Mapping[1].Field != "kwh" || !res.Mapping[1].Detected {
		t.Fatalf("unexpected mapping: %+v", res.Mapping)
	}
	if len(res.Fields) == 0 {
		t.Error("the preview lists the fields a column can feed")
	}
}

func TestMappingOverridesTheHeaders(t *testing.T) {
	ev := testVehicle(models.PowertrainEV)
	data := "Jour,Truc,Machin\n2026-09-15,42.5,18.5\n"
	if res := previewOf(t, ev, data, CSVImportOptions{Type: ImportTypeCharges}); res.ValidRows != 0 {
		t.Fatalf("columns with unknown names are not readable: %+v", res)
	}
	res := previewOf(t, ev, data, CSVImportOptions{Type: ImportTypeCharges, Mapping: map[int]string{1: "kwh", 2: "cost"}})
	if res.ValidRows != 1 || res.Mapping[1].Detected || res.Mapping[2].Field != "cost" {
		t.Errorf("mapped columns are imported: %+v (%v)", res, rowCodes(res))
	}
	res = previewOf(t, ev, "Date,kWh,Cost\n2026-09-15,42.5,18.5\n", CSVImportOptions{Type: ImportTypeCharges, Mapping: map[int]string{2: ""}})
	if res.ValidRows != 0 {
		t.Errorf("an ignored column is not read: %+v", res)
	}
}

func TestMappingUnitPerColumn(t *testing.T) {
	ev := testVehicle(models.PowertrainEV)
	res := previewOf(t, ev, "Date,Dist\n2026-09-15 08:00,10\n", CSVImportOptions{Type: ImportTypeDrives, Mapping: map[int]string{1: "distance_mi"}})
	if res.ValidRows != 1 {
		t.Fatalf("a mapped distance_mi column is valid: %+v (%v)", res, rowCodes(res))
	}
}

func TestDecimalSeparatorAndDateOrder(t *testing.T) {
	if v, err := parseFloatWith("1,234.5", "."); err != nil || v != 1234.5 {
		t.Errorf("dot decimal: %v %v", v, err)
	}
	if v, err := parseFloatWith("1.234,5", ","); err != nil || v != 1234.5 {
		t.Errorf("comma decimal: %v %v", v, err)
	}
	if v, err := parseFloatWith("1,5", ""); err != nil || v != 1.5 {
		t.Errorf("detected: %v %v", v, err)
	}
	mdy, err := parseTimeWith("03/04/2026", time.UTC, DateOrderMDY)
	if err != nil || mdy.Month() != time.March {
		t.Errorf("mdy: %v %v", mdy, err)
	}
	dmy, _ := parseTimeWith("03/04/2026", time.UTC, "")
	if dmy.Month() != time.April {
		t.Errorf("day first by default: %v", dmy)
	}
	ymd, err := parseTimeWith("2026.04.03", time.UTC, DateOrderYMD)
	if err != nil || ymd.Month() != time.April {
		t.Errorf("ymd with dots: %v %v", ymd, err)
	}
}

func TestMappingIsValidated(t *testing.T) {
	ev := testVehicle(models.PowertrainEV)
	for name, opts := range map[string]CSVImportOptions{
		"unknown field":  {Type: ImportTypeCharges, Mapping: map[int]string{0: "nope"}},
		"unknown column": {Type: ImportTypeCharges, Mapping: map[int]string{9: "kwh"}},
		"bad decimal":    {Type: ImportTypeCharges, DecimalSeparator: ";"},
		"bad date order": {Type: ImportTypeCharges, DateOrder: "xyz"},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := NewCSVImportService(nil, "UTC").Preview(context.Background(), ev, []byte("Date,kWh\n2026-09-15,1\n"), opts); err == nil {
				t.Error("expected an error")
			}
		})
	}
}
