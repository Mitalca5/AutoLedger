package services

import (
	"testing"

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
