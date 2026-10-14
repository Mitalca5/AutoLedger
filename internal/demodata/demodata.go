// Package demodata generates the deterministic demo datasets (one electric, one combustion) used for the
// read-only demo and the documentation screenshots. Dates are relative to a reference day, so a dataset is
// always "recent", and the same reference day always yields the same data.
package demodata

import (
	"fmt"
	"math"
	"math/rand"
	"time"

	"github.com/teslacost/teslacost/internal/models"
)

const Days = 240

type Vehicle struct {
	Name          string
	Make, Model   string
	Powertrain    string
	StartOdometer float64
	Currency      string
	KWhPer100Km   float64
	PricePerKWh   float64
}

// Event is one call of the ingestion API (/api/integrations/homeassistant/event).
type Event struct {
	ID   string
	Type string
	At   time.Time
	Data map[string]any
}

type Expense struct {
	Kind        string // "maintenance" or "toll"
	Category    string
	Description string
	AmountCents int64
	Date        time.Time
}

// Ownership is the acquisition record of the vehicle: a cash purchase, or a LOA when Lease is set (PriceCents is
// then the value of the vehicle at the start of the contract).
type Ownership struct {
	Date       time.Time
	Odometer   float64
	PriceCents int64
	FeesCents  int64
	// ResaleCents and HoldingMonths are the planned resale of a purchase; with a Lease the planned value is
	// Lease.EndValueCents instead.
	ResaleCents   int64
	HoldingMonths int
	Lease         *Lease
}

// CarpoolLeg is one stage of a carpooled trip, with the costs the driver recorded for it.
type CarpoolLeg struct {
	From, To         string
	DistanceKm       float64
	ElectricityCents int64
	TollsCents       int64
}

// CarpoolPassenger rides from the start of leg Board to the end of leg Alight (0-based leg indexes).
type CarpoolPassenger struct {
	Name          string
	Board, Alight int
	Seats         int
	PaidCents     int64
}

// Carpool is a multi-leg shared trip where passengers get on and off along the way.
type Carpool struct {
	Title      string
	DaysAgo    int
	Legs       []CarpoolLeg
	Passengers []CarpoolPassenger
}

// Lease is a LOA contract.
type Lease struct {
	DurationMonths     int
	MonthlyRentCents   int64
	DownPaymentCents   int64
	KmAllowancePerYear float64
	ExcessKmPrice      float64
	OptionPriceCents   int64
	// EndValueCents is the estimated market value of the vehicle at the end of the contract.
	EndValueCents int64
}

type Reminder struct {
	Title string
	Due   time.Time
}

type Tire struct {
	Brand, Model, Dimension, Season string
	PurchaseDate                    time.Time
	PriceCents                      int64
	Position                        string
	MountedOdometer                 float64
	LifespanKm                      int
}

// Comparison is a saved EV vs combustion scenario of the vehicle (retrospective mode).
type Comparison struct {
	Name                   string
	AnnualKm               float64
	Years                  int
	FuelType               string
	LPer100Km, FuelPrice   float64
	PurchaseCents          int64
	ResaleCents            int64
	MaintenanceYearlyCents int64
	InsuranceYearlyCents   int64
	TaxYearlyCents         int64
}

type Dataset struct {
	Key       string
	Vehicle   Vehicle
	Events    []Event
	Expenses  []Expense
	Reminders []Reminder
	Tires     []Tire
	// Ownership is the acquisition of the vehicle, which completes the TCO.
	Ownership Ownership
	// Carpool fills the carpooling page; nil leaves it empty.
	Carpool *Carpool
	// MonthlyBudgetCents is the household budget shown on the fleet page (account level); 0 sets none.
	MonthlyBudgetCents int64
	// Comparison is nil for a dataset that carries no scenario.
	Comparison *Comparison
	// FinalOdometer is the odometer after the last event.
	FinalOdometer float64
}

func Keys() []string { return []string{"ev", "ice"} }

// Build returns the dataset for key, ending on the day before today (UTC midnight of today).
func Build(key string, today time.Time) (Dataset, error) {
	today = time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, time.UTC)
	switch key {
	case "ev":
		return buildEV(today), nil
	case "ice":
		return buildICE(today), nil
	case "layout":
		return buildLayout(today), nil
	}
	return Dataset{}, fmt.Errorf("unknown dataset %q (expected ev, ice or layout)", key)
}

func round(v float64, decimals int) float64 {
	p := math.Pow(10, float64(decimals))
	return math.Round(v*p) / p
}

// lastEventAt returns the time of the latest event of the given type (zero when there is none).
func lastEventAt(events []Event, typ string) time.Time {
	var last time.Time
	for _, e := range events {
		if e.Type == typ && e.At.After(last) {
			last = e.At
		}
	}
	return last
}

func rfc(t time.Time) string { return t.UTC().Format(time.RFC3339) }

var places = []string{"Home", "Office", "Gym", "Supermarket", "Client site", "Station"}

func buildEV(today time.Time) Dataset {
	rng := rand.New(rand.NewSource(7))
	const (
		battery     = 75.0
		consumption = 16.4 // kWh/100 km
		homePrice   = 0.2276
		dcPrice     = 0.46
	)
	ds := Dataset{Key: "ev", Vehicle: Vehicle{
		Name: "Model 3", Make: "Tesla", Model: "Model 3 Long Range", Powertrain: "EV",
		StartOdometer: 18400, Currency: "EUR", KWhPer100Km: 16.5, PricePerKWh: homePrice,
	}}
	odo := ds.Vehicle.StartOdometer
	soc := 80.0
	for i := 0; i < Days; i++ {
		day := today.AddDate(0, 0, -(Days - i))
		wd := day.Weekday()
		var trips []float64
		switch {
		case wd >= time.Monday && wd <= time.Friday:
			trips = []float64{round(14+rng.Float64()*10, 1), round(14+rng.Float64()*10, 1)}
		case wd == time.Saturday && rng.Intn(3) > 0:
			trips = []float64{round(6+rng.Float64()*18, 1)}
		case wd == time.Sunday && rng.Intn(5) == 0:
			trips = []float64{round(150+rng.Float64()*80, 1)}
		}
		if len(trips) == 1 && trips[0] > 100 && soc < 90 {
			added := (90 - soc) / 100 * battery
			start := day.Add(5 * time.Hour)
			ds.Events = append(ds.Events, Event{
				ID: fmt.Sprintf("demo-ev-precharge-%03d", i), Type: "charging_session_end", At: start,
				Data: map[string]any{
					"start_time": rfc(start), "end_time": rfc(start.Add(90 * time.Minute)),
					"energy_kwh": round(added, 2), "energy_added_kwh": round(added, 2), "cost": round(added*homePrice, 2),
					"location": "Home", "charger_name": "Home", "odometer_km": round(odo, 1),
					"soc_start": int(math.Round(soc)), "soc_end": 90,
				},
			})
			soc = 90
		}
		for n, km := range trips {
			start := day.Add(time.Duration(8+n*9) * time.Hour).Add(time.Duration(rng.Intn(40)) * time.Minute)
			from, to := places[0], places[1]
			if n == 1 {
				from, to = to, from
			}
			if km > 100 {
				to = "Airport"
			}
			ds.Events = append(ds.Events, Event{
				ID: fmt.Sprintf("demo-ev-drive-%03d-%d", i, n), Type: "drive", At: start,
				Data: map[string]any{
					"start_time": rfc(start), "end_time": rfc(start.Add(time.Duration(km*1.6) * time.Minute)),
					"duration_min": int(km * 1.6), "distance": km,
					"start_odometer": round(odo, 1), "end_odometer": round(odo+km, 1),
					"start_address": from, "end_address": to,
					"energy_kwh": round(km*consumption/100, 2),
				},
			})
			odo += km
			soc -= km * consumption / 100 / battery * 100
		}
		if soc < 28 {
			target := 80.0
			added := (target - soc) / 100 * battery
			fast := len(trips) == 1 && trips[0] > 100
			price, place, start, hours := homePrice, "Home", day.Add(22*time.Hour), 4.0
			if fast {
				price, place, start, hours = dcPrice, "Highway Supercharger", day.Add(13*time.Hour), 0.6
			}
			cost := round(added*price, 2)
			ds.Events = append(ds.Events, Event{
				ID: fmt.Sprintf("demo-ev-charge-%03d", i), Type: "charging_session_end", At: start,
				Data: map[string]any{
					"start_time": rfc(start), "end_time": rfc(start.Add(time.Duration(hours * float64(time.Hour)))),
					"energy_kwh": round(added, 2), "energy_added_kwh": round(added, 2), "cost": cost,
					"location": place, "charger_name": place, "odometer_km": round(odo, 1),
					"soc_start": int(math.Round(soc)), "soc_end": int(target),
				},
			})
			soc = target
		}
	}
	// The most recent session always falls in the last days, so the current month is never empty.
	if lastCharge := lastEventAt(ds.Events, "charging_session_end"); today.Sub(lastCharge) > 5*24*time.Hour {
		start := today.AddDate(0, 0, -1).Add(22 * time.Hour)
		added := (80 - soc) / 100 * battery
		ds.Events = append(ds.Events, Event{
			ID: "demo-ev-charge-latest", Type: "charging_session_end", At: start,
			Data: map[string]any{
				"start_time": rfc(start), "end_time": rfc(start.Add(4 * time.Hour)),
				"energy_kwh": round(added, 2), "energy_added_kwh": round(added, 2), "cost": round(added*homePrice, 2),
				"location": "Home", "charger_name": "Home", "odometer_km": round(odo, 1),
				"soc_start": int(math.Round(soc)), "soc_end": 80,
			},
		})
	}
	ds.FinalOdometer = odo
	at := func(d int) time.Time { return today.AddDate(0, 0, -d) }
	ds.Ownership = Ownership{Date: at(Days + 30), Odometer: ds.Vehicle.StartOdometer - 400, PriceCents: 4290000, FeesCents: 38000,
		// An option priced above the expected market value: the dashboard says returning the car is the better deal.
		Lease: &Lease{
			DurationMonths: 48, MonthlyRentCents: 38900, DownPaymentCents: 400000, KmAllowancePerYear: 12000,
			ExcessKmPrice: 0.12, OptionPriceCents: 2350000, EndValueCents: 2100000,
		}}
	// The second passenger gets on at the stop of the first leg and rides to the end, so the legs are shared unevenly.
	ds.Carpool = &Carpool{
		Title: "Lyon to Annecy, Friday evening", DaysAgo: 12,
		Legs: []CarpoolLeg{
			{From: "Lyon", To: "Chambery", DistanceKm: 112, ElectricityCents: 1150, TollsCents: 1120},
			{From: "Chambery", To: "Annecy", DistanceKm: 52, ElectricityCents: 540, TollsCents: 0},
		},
		Passengers: []CarpoolPassenger{
			{Name: "Alice", Board: 0, Alight: 1, Seats: 1, PaidCents: 1500},
			{Name: "Bruno", Board: 1, Alight: 1, Seats: 1, PaidCents: 300},
		},
	}
	ds.MonthlyBudgetCents = 45000
	ds.Expenses = []Expense{
		{"maintenance", "INSURANCE", "Insurance, 12 months", 58000, at(215)},
		{"maintenance", "MAINTENANCE", "Annual inspection", 18900, at(150)},
		{"maintenance", "ACCESSORY", "Type 2 charging cable", 4500, at(190)},
		{"maintenance", "REPAIR", "Windshield chip repair", 6900, at(70)},
		{"toll", "", "A7 Lyon to Orange", 1840, at(21)},
		{"toll", "", "A8 Aix to Nice", 1260, at(95)},
	}
	ds.Reminders = []Reminder{
		{"Brake fluid", today.AddDate(0, 0, 9)},
		{"Cabin air filter", today.AddDate(0, 0, 45)},
		{"Technical inspection", models.AddCalendarMonths(today, 6)},
	}
	ds.Tires = []Tire{
		{"Michelin", "Pilot Sport 4", "235/45 R18", "SUMMER", at(200), 79200, "FL", ds.Vehicle.StartOdometer, 40000},
		{"Michelin", "Pilot Sport 4", "235/45 R18", "SUMMER", at(200), 79200, "FR", ds.Vehicle.StartOdometer, 40000},
		{"Michelin", "Pilot Sport 4", "235/45 R18", "SUMMER", at(200), 79200, "RL", ds.Vehicle.StartOdometer, 40000},
		{"Michelin", "Pilot Sport 4", "235/45 R18", "SUMMER", at(200), 79200, "RR", ds.Vehicle.StartOdometer, 40000},
	}
	ds.Comparison = &Comparison{
		Name: "Model 3 vs Clio", AnnualKm: 13000, Years: 5, FuelType: "SP95_E10", LPer100Km: 5.6, FuelPrice: 1.82,
		PurchaseCents: 1850000, ResaleCents: 700000, MaintenanceYearlyCents: 45000, InsuranceYearlyCents: 61200, TaxYearlyCents: 0,
	}
	return ds
}

func buildICE(today time.Time) Dataset {
	rng := rand.New(rand.NewSource(11))
	const (
		tank        = 42.0
		consumption = 5.6 // L/100 km
	)
	ds := Dataset{Key: "ice", Vehicle: Vehicle{
		Name: "Clio V", Make: "Renault", Model: "Clio TCe 100", Powertrain: "ICE",
		StartOdometer: 54200, Currency: "EUR",
	}}
	odo := ds.Vehicle.StartOdometer
	fuel := 30.0
	for i := 0; i < Days; i++ {
		day := today.AddDate(0, 0, -(Days - i))
		wd := day.Weekday()
		var km float64
		switch {
		case wd >= time.Monday && wd <= time.Friday:
			km = round(26+rng.Float64()*14, 1)
		case wd == time.Saturday:
			km = round(8+rng.Float64()*25, 1)
		case wd == time.Sunday && rng.Intn(4) == 0:
			km = round(150+rng.Float64()*200, 1)
		}
		odo += km
		fuel -= km * consumption / 100
		if wd == time.Wednesday || wd == time.Thursday {
			ds.Events = append(ds.Events, Event{
				ID: fmt.Sprintf("demo-ice-odo-%03d", i), Type: "odometer", At: day.Add(20 * time.Hour),
				Data: map[string]any{"odometer": round(odo, 1)},
			})
		}
		if fuel < 9 {
			liters := round(tank-fuel, 2)
			ppl := round(1.72+0.012*float64(i%17)+rng.Float64()*0.05, 3)
			at := day.Add(18 * time.Hour)
			ds.Events = append(ds.Events, Event{
				ID: fmt.Sprintf("demo-ice-fuel-%03d", i), Type: "fuel", At: at,
				Data: map[string]any{
					"liters": liters, "price_per_liter": ppl, "amount": round(liters*ppl, 2),
					"fuel_type": "SP95_E10", "is_full_tank": true, "odometer_km": round(odo, 1),
				},
			})
			fuel = tank
		}
	}
	if lastFill := lastEventAt(ds.Events, "fuel"); today.Sub(lastFill) > 5*24*time.Hour {
		liters := round(tank-fuel, 2)
		at := today.AddDate(0, 0, -1).Add(18 * time.Hour)
		ds.Events = append(ds.Events, Event{
			ID: "demo-ice-fuel-latest", Type: "fuel", At: at,
			Data: map[string]any{
				"liters": liters, "price_per_liter": 1.789, "amount": round(liters*1.789, 2),
				"fuel_type": "SP95_E10", "is_full_tank": true, "odometer_km": round(odo, 1),
			},
		})
	}
	ds.FinalOdometer = odo
	at := func(d int) time.Time { return today.AddDate(0, 0, -d) }
	ds.Ownership = Ownership{Date: at(Days + 90), Odometer: ds.Vehicle.StartOdometer - 1500, PriceCents: 1450000, FeesCents: 12000, ResaleCents: 850000, HoldingMonths: 60}
	ds.Expenses = []Expense{
		{"maintenance", "INSURANCE", "Insurance, 12 months", 61200, at(230)},
		{"maintenance", "MAINTENANCE", "Service: oil, oil filter, air filter", 21900, at(120)},
		{"maintenance", "REPAIR", "Front brake pads and discs", 38500, at(60)},
		{"maintenance", "ACCESSORY", "Roof bars", 8900, at(180)},
		{"toll", "", "A6 Paris to Lyon", 3240, at(40)},
		{"toll", "", "A9 Orange to Perpignan", 2150, at(110)},
	}
	ds.Reminders = []Reminder{
		{"Oil change", today.AddDate(0, 0, 14)},
		{"Timing belt check", today.AddDate(0, 0, 60)},
		{"Technical inspection", models.AddCalendarMonths(today, 3)},
	}
	for _, pos := range []string{"FL", "FR", "RL", "RR"} {
		ds.Tires = append(ds.Tires, Tire{"Continental", "EcoContact 6", "195/55 R16", "SUMMER", at(235), 38800, pos, ds.Vehicle.StartOdometer, 45000})
	}
	return ds
}

// buildLayout is a plug-in hybrid made to stress the rendering rather than to look like a real car: very long
// names, another currency, a far-off outlier drive and a tiny one, a large expense, an overdue reminder and a
// purchase with no planned resale (the full cost then reports what is missing). It is not part of the default
// set seeded for the public demo.
func buildLayout(today time.Time) Dataset {
	rng := rand.New(rand.NewSource(23))
	const (
		long        = "Workplace car park, level -2, bay 14, next to the visitors entrance and the cycle store"
		consumption = 17.0 // kWh/100 km, electric part
		tank        = 43.0
	)
	ds := Dataset{Key: "layout", Vehicle: Vehicle{
		Name:       "Family estate plug-in hybrid, the long name that has to be truncated on a phone",
		Make:       "A manufacturer with a particularly long name",
		Model:      "Touring 1.5 plug-in hybrid four-wheel drive Executive Premium",
		Powertrain: "PHEV", StartOdometer: 12800, Currency: "GBP", KWhPer100Km: 17, PricePerKWh: 0.28,
	}}
	odo := ds.Vehicle.StartOdometer
	fuel := 20.0
	for i := 0; i < Days; i++ {
		day := today.AddDate(0, 0, -(Days - i))
		wd := day.Weekday()
		if wd == time.Sunday || (wd == time.Saturday && rng.Intn(2) == 0) {
			continue
		}
		km := round(12+rng.Float64()*28, 1)
		switch i {
		case Days / 3:
			km = 420.4 // a long motorway day
		case Days / 2:
			km = 0.3 // a bump of the garage
		}
		start := day.Add(8 * time.Hour)
		to := long
		if i%3 == 0 {
			to = "Home"
		}
		ds.Events = append(ds.Events, Event{
			ID: fmt.Sprintf("demo-layout-drive-%03d", i), Type: "drive", At: start,
			Data: map[string]any{
				"start_time": rfc(start), "end_time": rfc(start.Add(time.Duration(km*1.5+1) * time.Minute)),
				"duration_min": int(km*1.5) + 1, "distance": km,
				"start_odometer": round(odo, 1), "end_odometer": round(odo+km, 1),
				"start_address": "Home", "end_address": to, "energy_kwh": round(km*consumption/100, 2),
			},
		})
		odo += km
		fuel -= km * 0.055
		if i%4 == 0 {
			added := round(6+rng.Float64()*8, 2)
			at := day.Add(19 * time.Hour)
			ds.Events = append(ds.Events, Event{
				ID: fmt.Sprintf("demo-layout-charge-%03d", i), Type: "charging_session_end", At: at,
				Data: map[string]any{
					"start_time": rfc(at), "end_time": rfc(at.Add(3 * time.Hour)),
					"energy_kwh": added, "energy_added_kwh": added, "cost": round(added*0.28, 2),
					"location": "Home", "charger_name": "Home", "odometer_km": round(odo, 1),
				},
			})
		}
		if fuel < 8 {
			liters := round(tank-fuel, 2)
			at := day.Add(18 * time.Hour)
			ds.Events = append(ds.Events, Event{
				ID: fmt.Sprintf("demo-layout-fuel-%03d", i), Type: "fuel", At: at,
				Data: map[string]any{
					"liters": liters, "price_per_liter": 1.52, "amount": round(liters*1.52, 2),
					"fuel_type": "SP95_E10", "is_full_tank": true, "odometer_km": round(odo, 1),
				},
			})
			fuel = tank
		}
	}
	ds.FinalOdometer = odo
	at := func(d int) time.Time { return today.AddDate(0, 0, -d) }
	ds.Ownership = Ownership{Date: at(Days + 20), Odometer: ds.Vehicle.StartOdometer - 20, PriceCents: 3800000, FeesCents: 25000}
	ds.Expenses = []Expense{
		{"maintenance", "REPAIR", "Hybrid battery cooling system replaced under the extended warranty claim, labour and parts, second visit", 428000, at(45)},
		{"maintenance", "INSURANCE", "Comprehensive insurance with breakdown cover and courtesy car, 12 months", 112000, at(200)},
		{"maintenance", "MAINTENANCE", "Annual service", 24900, at(90)},
		{"toll", "", "Dartford Crossing", 250, at(30)},
	}
	ds.Reminders = []Reminder{
		{"MOT test, overdue since last week and needs to be booked before the insurance renewal", today.AddDate(0, 0, -10)},
		{"Software update at the dealer", today.AddDate(0, 0, 5)},
	}
	for _, pos := range []string{"FL", "FR", "RL", "RR"} {
		ds.Tires = append(ds.Tires, Tire{"Winter brand with a long name", "All-season model with a long name", "225/45 R19 96W XL", "WINTER", at(Days + 10), 31200, pos, ds.Vehicle.StartOdometer, 12000})
	}
	return ds
}
