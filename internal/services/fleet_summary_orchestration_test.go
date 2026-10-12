package services

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/teslacost/teslacost/internal/models"
	"github.com/teslacost/teslacost/internal/money"
)

type fakeFleetStore struct {
	summary *models.FleetSummaryResponse
	err     error
	budget  *money.Cents
	saved   bool
}

func (f *fakeFleetStore) GetFleetSummary(context.Context, string, string) (*models.FleetSummaryResponse, error) {
	return f.summary, f.err
}

func (f *fakeFleetStore) SetFleetMonthlyBudget(_ context.Context, _ string, budget *money.Cents) error {
	f.budget, f.saved = budget, true
	return f.err
}

type fakeVehicleTCO map[string]*TCOSummary

func (f fakeVehicleTCO) ComputeVehicleTCO(_ context.Context, vehicleID string) (*TCOSummary, error) {
	if sum, ok := f[vehicleID]; ok {
		return sum, nil
	}
	return nil, errors.New("no TCO")
}

// fakeEnergyStats answers the cost per 100 km of the energy statistics per vehicle; a missing vehicle has none.
type fakeEnergyStats map[string]*float64

func (f fakeEnergyStats) Compute(_ context.Context, vehicleID string) (*EnergyStats, error) {
	if _, ok := f[vehicleID]; !ok {
		return nil, errors.New("no energy statistics")
	}
	return &EnergyStats{Summary: EnergySummary{CostPer100km: f[vehicleID]}}, nil
}

func fleetOfTwo(month string) *models.FleetSummaryResponse {
	return &models.FleetSummaryResponse{
		MonthlyCosts: []models.FleetMonthlyCost{{Month: month, TotalCost: 999}},
		Vehicles:     []models.VehicleFleetMetric{{VehicleID: "a", MonthCost: 111}, {VehicleID: "b", MonthCost: 222}},
	}
}

func TestFleetSummaryRebuildsMonthsFromVehicleTCOs(t *testing.T) {
	month := time.Now().Format("2006-01")
	tco := func(total, energy money.Cents) *TCOSummary {
		return &TCOSummary{Powertrain: "EV", MonthlyCosts: []MonthlyCost{{Month: month, Total: total, Energy: energy}}}
	}
	svc := &FleetService{repo: &fakeFleetStore{summary: fleetOfTwo(month)}, tco: fakeVehicleTCO{"a": tco(1000, 400), "b": tco(500, 100)}, energy: fakeEnergyStats{}}

	got, err := svc.GetSummary(context.Background(), "u")
	if err != nil {
		t.Fatal(err)
	}
	m := got.MonthlyCosts[0]
	if m.TotalCost != 1500 || m.EnergyCost != 500 || m.OtherCost != 1000 || m.ByVehicle["a"] != 1000 || m.ByVehicle["b"] != 500 {
		t.Errorf("month rebuilt from the TCOs: %+v, want total 1500, energy 500, other 1000, a 1000, b 500", m)
	}
	if got.CurrentMonthCost != 1500 || got.Vehicles[0].MonthCost != 1000 || got.Vehicles[1].MonthCost != 500 {
		t.Errorf("current month: household %d, vehicles %d and %d, want 1500, 1000 and 500", got.CurrentMonthCost, got.Vehicles[0].MonthCost, got.Vehicles[1].MonthCost)
	}
	if got.Vehicles[0].Powertrain != "EV" {
		t.Errorf("powertrain from the TCO: %q", got.Vehicles[0].Powertrain)
	}
}

func TestFleetSummaryKeepsStoredMonthsWhenAVehicleHasNoTCO(t *testing.T) {
	month := time.Now().Format("2006-01")
	only := &TCOSummary{Powertrain: "EV", MonthlyCosts: []MonthlyCost{{Month: month, Total: 1000, Energy: 400}}}
	svc := &FleetService{repo: &fakeFleetStore{summary: fleetOfTwo(month)}, tco: fakeVehicleTCO{"a": only}, energy: fakeEnergyStats{}}

	got, err := svc.GetSummary(context.Background(), "u")
	if err != nil {
		t.Fatal(err)
	}
	// A partial sum would understate the household, so the stored figures stand; the vehicle that has a TCO still gets its comparison figures
	if got.MonthlyCosts[0].TotalCost != 999 || got.Vehicles[0].MonthCost != 111 || got.Vehicles[1].MonthCost != 222 {
		t.Errorf("stored months kept: month %d, vehicles %d and %d, want 999, 111 and 222", got.MonthlyCosts[0].TotalCost, got.Vehicles[0].MonthCost, got.Vehicles[1].MonthCost)
	}
	if got.Vehicles[0].Powertrain != "EV" || got.Vehicles[1].Powertrain != "" {
		t.Errorf("powertrains: %q and %q, want EV and none", got.Vehicles[0].Powertrain, got.Vehicles[1].Powertrain)
	}
}

func TestFleetSummaryAndBudgetReportStoreErrors(t *testing.T) {
	boom := errors.New("down")
	svc := &FleetService{repo: &fakeFleetStore{err: boom}, tco: fakeVehicleTCO{}, energy: fakeEnergyStats{}}
	if _, err := svc.GetSummary(context.Background(), "u"); !errors.Is(err, boom) {
		t.Errorf("summary error: got %v, want the store's", err)
	}
	if err := svc.SetMonthlyBudget(context.Background(), "u", nil); !errors.Is(err, boom) {
		t.Errorf("budget error: got %v, want the store's", err)
	}

	store := &fakeFleetStore{}
	budget := money.Cents(25000)
	if err := (&FleetService{repo: store}).SetMonthlyBudget(context.Background(), "u", &budget); err != nil || !store.saved || *store.budget != 25000 {
		t.Errorf("budget stored: err %v, saved %v", err, store.saved)
	}
}

func TestFleetEnergyCostPer100KmComesFromTheEnergyStatistics(t *testing.T) {
	stats := 3.73
	svc := &FleetService{
		repo:   &fakeFleetStore{summary: fleetOfTwo(time.Now().Format("2006-01"))},
		tco:    fakeVehicleTCO{},
		energy: fakeEnergyStats{"a": &stats, "b": nil},
	}
	got, err := svc.GetSummary(context.Background(), "u")
	if err != nil {
		t.Fatal(err)
	}
	if got.Vehicles[0].EnergyCostPer100Km != 3.73 {
		t.Fatalf("vehicle a: got %v, want the energy-statistics 3.73", got.Vehicles[0].EnergyCostPer100Km)
	}
	if got.Vehicles[1].EnergyCostPer100Km != 0 {
		t.Fatalf("vehicle b has no tracked figure: got %v, want 0 (unknown)", got.Vehicles[1].EnergyCostPer100Km)
	}
}

func TestFleetEnergyCostPer100KmIsUnknownWhenTheStatisticsFail(t *testing.T) {
	svc := &FleetService{
		repo:   &fakeFleetStore{summary: fleetOfTwo(time.Now().Format("2006-01"))},
		tco:    fakeVehicleTCO{},
		energy: fakeEnergyStats{},
	}
	got, err := svc.GetSummary(context.Background(), "u")
	if err != nil {
		t.Fatal(err)
	}
	if got.Vehicles[0].EnergyCostPer100Km != 0 {
		t.Fatalf("got %v, want 0", got.Vehicles[0].EnergyCostPer100Km)
	}
}
