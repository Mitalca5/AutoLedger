package services

import (
	"context"
	"math"
	"testing"
	"time"

	"github.com/teslacost/teslacost/internal/models"
	"github.com/teslacost/teslacost/internal/money"
)

func group(t *testing.T, s *ChargingPlaceStats, key string) PlaceGroup {
	t.Helper()
	for _, g := range s.Groups {
		if g.Key == key {
			return g
		}
	}
	t.Fatalf("no group %q in %+v", key, s.Groups)
	return PlaceGroup{}
}

func TestBuildPlaceStatsWeightsPricesAndReportsUnknownCosts(t *testing.T) {
	rows := []placeRow{
		// Home: 100 kWh for 20 EUR, of which 10 kWh free (solar) and 5 kWh without a recorded cost
		{Month: "2026-02", Key: "home", Name: "Home", Category: models.PlaceHome, Sessions: 3, KwhAdded: 105, Cost: eur(20), PricedKwh: 100, FreeSessions: 1, UnknownSessions: 1, UnknownKwh: 5},
		{Month: "2026-03", Key: "home", Name: "Home", Category: models.PlaceHome, Sessions: 1, KwhAdded: 50, Cost: eur(10), PricedKwh: 50},
		{Month: "2026-03", Key: "ionity", Name: "Ionity", Category: models.PlacePublic, Sessions: 2, KwhAdded: 60, Cost: eur(36), PricedKwh: 60},
		{Month: "2026-03", Key: "", Sessions: 1, KwhAdded: 20, Cost: eur(8), PricedKwh: 20, ZeroKwhSessions: 2},
	}
	s := buildPlaceStats(rows, PlacePeriod{Months: 12})

	home := group(t, s, "home")
	if home.KwhAdded != 155 || home.Cost != eur(30) || home.PricedKwh != 150 {
		t.Fatalf("home totals: %+v", home)
	}
	// 30 EUR over the 150 kWh whose cost is known; the 5 kWh without cost are not in the denominator
	if home.PricePerKwh == nil || math.Abs(*home.PricePerKwh-0.2) > 1e-9 {
		t.Fatalf("home weighted price: %v", home.PricePerKwh)
	}
	if home.UnknownSessions != 1 || home.UnknownKwh != 5 || home.FreeSessions != 1 {
		t.Fatalf("home free/unknown: %+v", home)
	}

	if s.Summary.ZeroKwhSessions != 2 {
		t.Errorf("zero-kWh sessions are reported apart: %d", s.Summary.ZeroKwhSessions)
	}
	var groupKwh float64
	var groupCost money.Cents
	for _, g := range s.Groups {
		groupKwh += g.KwhAdded
		groupCost += g.Cost
	}
	var monthKwh float64
	var monthCost money.Cents
	for _, m := range s.Series {
		monthKwh += m.KwhAdded
		monthCost += m.Cost
	}
	if groupKwh != s.Summary.KwhAdded || monthKwh != s.Summary.KwhAdded || groupCost != s.Summary.Cost || monthCost != s.Summary.Cost {
		t.Errorf("groups (%v, %d), months (%v, %d) and summary (%v, %d) must reconcile", groupKwh, groupCost, monthKwh, monthCost, s.Summary.KwhAdded, s.Summary.Cost)
	}
	if len(s.Series) != 2 || s.Series[0].Month != "2026-02" {
		t.Errorf("series sorted by month: %+v", s.Series)
	}
}

func TestHomeSavings(t *testing.T) {
	rows := []placeRow{
		{Month: "2026-03", Key: "home", Category: models.PlaceHome, Sessions: 2, KwhAdded: 100, Cost: eur(20), PricedKwh: 100},
		{Month: "2026-03", Key: "ionity", Category: models.PlacePublic, Sessions: 2, KwhAdded: 50, Cost: eur(30), PricedKwh: 50},
		{Month: "2026-03", Key: "", Sessions: 1, KwhAdded: 40, Cost: eur(40), PricedKwh: 40},
	}
	h := buildPlaceStats(rows, PlacePeriod{}).HomeSavings
	if h.Reason != "" || h.HomePricePerKwh == nil || *h.HomePricePerKwh != 0.2 {
		t.Fatalf("home price: %+v", h)
	}
	// 50 kWh paid 30 EUR away would have cost 10 EUR at 0.20: 20 EUR saved. The unclassified 40 kWh are not counted.
	if h.AwayCost != eur(30) || h.HypotheticalCost != eur(10) || h.Savings != eur(20) || h.UnclassifiedKwh != 40 {
		t.Errorf("savings: %+v", h)
	}

	free := buildPlaceStats([]placeRow{
		{Month: "2026-03", Key: "home", Category: models.PlaceHome, Sessions: 1, KwhAdded: 100, Cost: eur(20), PricedKwh: 100},
		{Month: "2026-03", Key: "work", Category: models.PlaceWork, Sessions: 5, KwhAdded: 50, Cost: 0, PricedKwh: 50, FreeSessions: 5},
	}, PlacePeriod{}).HomeSavings
	if free.Savings != -eur(10) {
		t.Errorf("free charging away is cheaper than home, savings must be negative: %+v", free)
	}

	if r := buildPlaceStats([]placeRow{{Month: "2026-03", Key: "ionity", Category: models.PlacePublic, Sessions: 1, KwhAdded: 10, Cost: eur(5), PricedKwh: 10}}, PlacePeriod{}).HomeSavings.Reason; r != HomeSavingsNoHome {
		t.Errorf("no home charging: reason %q", r)
	}
	if r := buildPlaceStats([]placeRow{{Month: "2026-03", Key: "home", Category: models.PlaceHome, Sessions: 1, KwhAdded: 10, Cost: eur(2), PricedKwh: 10}}, PlacePeriod{}).HomeSavings.Reason; r != HomeSavingsNoAway {
		t.Errorf("no away charging: reason %q", r)
	}
}

func TestIntegrationPlaceStatsReconcileWithTheLedger(t *testing.T) {
	db, repo := setupIntegrationDB(t, false)
	ctx := context.Background()
	v := mustVehicle(t, repo, "place-stats@example.com")
	other := mustVehicle(t, repo, "place-stats-other@example.com")

	home, err := repo.CreateChargingPlace(ctx, v.UserID, "Home", models.PlaceHome)
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.SetChargingPlaceAlias(ctx, v.UserID, "Maison", &home.ID); err != nil {
		t.Fatal(err)
	}
	pub, err := repo.CreateChargingPlace(ctx, v.UserID, "Public", models.PlacePublic)
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.SetChargingPlaceAlias(ctx, v.UserID, "Valence Supercharger, Valence", &pub.ID); err != nil {
		t.Fatal(err)
	}

	addr := func(s string) *string { return &s }
	day := time.Date(time.Now().Year(), time.Now().Month(), 2, 8, 0, 0, 0, time.UTC)
	paid, free := money.Cents(2000), money.Cents(0)
	add := func(vehicleID string, i int, address *string, kwh float64, cost *money.Cents) {
		c := &models.ChargeLog{VehicleID: vehicleID, Date: day.Add(time.Duration(i) * time.Hour), Address: address, KwhAdded: kwh, Cost: cost, Currency: "EUR"}
		if cost == nil {
			// a manual charge needs a cost: insert the unknown-cost one the way a synchronisation does
			tmID := 1000 + i
			c.TeslaMateChargeID = &tmID
			if _, err := repo.UpsertTeslaMateCharge(ctx, c); err != nil {
				t.Fatal(err)
			}
			return
		}
		if err := repo.CreateManualCharge(ctx, c); err != nil {
			t.Fatal(err)
		}
	}
	add(v.ID, 1, addr("Maison"), 40, &paid)
	add(v.ID, 2, addr(" maison "), 20, &free)
	add(v.ID, 3, addr("Valence Supercharger, Valence"), 30, &paid)
	add(v.ID, 4, addr("Valence Supercharger, Valence"), 10, nil)
	add(v.ID, 5, addr("Somewhere else"), 15, &paid)
	add(v.ID, 6, addr("Maison"), 0, &free)
	add(other.ID, 7, addr("Maison"), 99, &paid) // another vehicle, another account

	svc := NewEnergyStatsService(db.Pool, "UTC")
	s, err := svc.PlaceStats(ctx, v.ID, v.UserID, PlacePeriod{Months: 12}, time.Now())
	if err != nil {
		t.Fatal(err)
	}

	h := group(t, s, home.ID)
	if h.Sessions != 2 || h.KwhAdded != 60 || h.Cost != paid || h.FreeSessions != 1 {
		t.Errorf("home: %+v", h)
	}
	p := group(t, s, pub.ID)
	if p.Sessions != 2 || p.KwhAdded != 40 || p.PricedKwh != 30 || p.UnknownSessions != 1 || p.UnknownKwh != 10 {
		t.Errorf("public: %+v", p)
	}
	if u := group(t, s, ""); u.KwhAdded != 15 {
		t.Errorf("unclassified: %+v", u)
	}
	if s.Summary.ZeroKwhSessions != 1 || s.Summary.Sessions != 5 {
		t.Errorf("summary: %+v", s.Summary)
	}

	// The recorded cost of the groups is the energy cost of the cost ledger for these charges
	var ledger money.Cents
	if err := db.Pool.QueryRow(ctx, `SELECT COALESCE(SUM(amount_eur), 0) FROM cost_ledger WHERE vehicle_id = $1 AND source_table = 'charge_logs'`, v.ID).Scan(&ledger); err != nil {
		t.Fatal(err)
	}
	if ledger != s.Summary.Cost {
		t.Errorf("ledger energy cost %d, statistics cost %d", ledger, s.Summary.Cost)
	}

	// A custom period keeps the days from..to inclusive
	dayStr := day.Format("2006-01-02")
	same, err := svc.PlaceStats(ctx, v.ID, v.UserID, PlacePeriod{From: dayStr, To: dayStr}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if same.Summary.Sessions != s.Summary.Sessions || same.From != dayStr {
		t.Errorf("the whole day is included: %+v", same.Summary)
	}
	before := day.AddDate(0, 0, -1).Format("2006-01-02")
	none, err := svc.PlaceStats(ctx, v.ID, v.UserID, PlacePeriod{From: before, To: before}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if none.Summary.Sessions != 0 || len(none.Groups) != 0 {
		t.Errorf("the day before has no charge: %+v", none.Summary)
	}

	// Another account's places never classify this vehicle's charges
	foreign, err := svc.PlaceStats(ctx, v.ID, other.UserID, PlacePeriod{Months: 12}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if len(foreign.Groups) != 1 || foreign.Groups[0].Key != "" {
		t.Errorf("without its owner's places everything is unclassified: %+v", foreign.Groups)
	}
}

func TestPlacePeriodBounds(t *testing.T) {
	loc, err := time.LoadLocation("Europe/Paris")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 10, 12, 9, 0, 0, 0, loc)

	since, until, err := PlacePeriod{From: "2026-09-01", To: "2026-09-30"}.bounds(loc, now)
	if err != nil || since == nil || until == nil {
		t.Fatalf("custom period: %v %v %v", since, until, err)
	}
	// The last day is included: the interval ends at the start of the next day
	if !since.Equal(time.Date(2026, 9, 1, 0, 0, 0, 0, loc)) || !until.Equal(time.Date(2026, 10, 1, 0, 0, 0, 0, loc)) {
		t.Errorf("bounds %v .. %v", since, until)
	}

	since, until, err = PlacePeriod{Months: 1}.bounds(loc, now)
	if err != nil || until != nil || !since.Equal(time.Date(2026, 10, 1, 0, 0, 0, 0, loc)) {
		t.Errorf("one month is the current month: %v %v %v", since, until, err)
	}
	since, _, _ = PlacePeriod{Months: 3}.bounds(loc, now)
	if !since.Equal(time.Date(2026, 8, 1, 0, 0, 0, 0, loc)) {
		t.Errorf("three months start in August: %v", since)
	}
	if since, until, _ := (PlacePeriod{}).bounds(loc, now); since != nil || until != nil {
		t.Errorf("no months and no dates is the whole history")
	}

	for _, p := range []PlacePeriod{{From: "2026-09-30", To: "2026-09-01"}, {From: "2026-09-01"}, {From: "x", To: "2026-09-01"}} {
		if _, _, err := p.bounds(loc, now); err == nil {
			t.Errorf("%+v must be refused", p)
		}
	}
}
