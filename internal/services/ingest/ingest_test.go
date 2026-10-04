package ingest

import (
	"math"
	"testing"
	"time"
)

func fptr(v float64) *float64 { return &v }
func iptr(v int) *int         { return &v }

var t0 = time.Date(2026, 9, 1, 8, 0, 0, 0, time.UTC)

func TestNormalizeDriveEndTime(t *testing.T) {
	later := t0.Add(45 * time.Minute)
	before := t0.Add(-time.Hour)
	cases := []struct {
		name    string
		in      DriveInput
		wantEnd time.Time
		wantMin int
	}{
		{"given end wins", DriveInput{Start: t0, End: &later, DurationMin: iptr(10), DistanceKm: 25}, later, 45},
		{"end not after start falls back to duration", DriveInput{Start: t0, End: &before, DurationMin: iptr(20), DistanceKm: 25}, t0.Add(20 * time.Minute), 20},
		{"duration used without end", DriveInput{Start: t0, DurationMin: iptr(30), DistanceKm: 25}, t0.Add(30 * time.Minute), 30},
		{"zero duration ignored, assumed speed used", DriveInput{Start: t0, DurationMin: iptr(0), DistanceKm: 25}, t0.Add(30 * time.Minute), 30},
		{"assumed speed", DriveInput{Start: t0, DistanceKm: 100}, t0.Add(120 * time.Minute), 120},
		{"very short drive lasts at least a minute", DriveInput{Start: t0, DistanceKm: 0.1}, t0.Add(time.Minute), 1},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := NormalizeDrive(c.in)
			if !got.End.Equal(c.wantEnd) || got.DurationMin != c.wantMin {
				t.Fatalf("end %v duration %d, want %v %d", got.End, got.DurationMin, c.wantEnd, c.wantMin)
			}
		})
	}
}

func TestNormalizeDriveEnergy(t *testing.T) {
	cases := []struct {
		name          string
		energy        *float64
		vehicle       *float64
		wantKwh       float64
		wantCons      float64
		wantEstimated bool
	}{
		{"typed energy", fptr(15), fptr(20), 15, 15, false},
		{"missing energy uses the vehicle consumption", nil, fptr(20), 20, 20, true},
		{"zero energy counts as missing", fptr(0), fptr(18), 18, 18, true},
		{"negative energy counts as missing", fptr(-3), nil, 16, 16, true},
		{"no vehicle estimate uses the default", nil, nil, 16, 16, true},
		{"zero vehicle estimate uses the default", nil, fptr(0), 16, 16, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := NormalizeDrive(DriveInput{Start: t0, DistanceKm: 100, EnergyKwh: c.energy, VehicleKwh100km: c.vehicle})
			if math.Abs(got.EnergyKwh-c.wantKwh) > 1e-9 || math.Abs(got.Kwh100km-c.wantCons) > 1e-9 || got.EnergyEstimated != c.wantEstimated {
				t.Fatalf("got %+v", got)
			}
		})
	}
}

func TestEstimateDriveEnergyScalesWithDistance(t *testing.T) {
	energy, cons := EstimateDriveEnergy(fptr(20), 50)
	if energy != 10 || cons != 20 {
		t.Fatalf("got %v kWh at %v/100km", energy, cons)
	}
}

func TestValidDriveDistance(t *testing.T) {
	for km, want := range map[float64]bool{0: false, -1: false, 0.1: true, 3000: true, 3000.1: false} {
		if got := ValidDriveDistance(km); got != want {
			t.Errorf("%v km: got %v, want %v", km, got, want)
		}
	}
}

func TestValidChargeEnergy(t *testing.T) {
	for kwh, want := range map[float64]bool{0: false, -5: false, 0.1: true, 1000: true, 1001: false, math.NaN(): false, math.Inf(1): false} {
		if got := ValidChargeEnergy(kwh); got != want {
			t.Errorf("%v kWh: got %v, want %v", kwh, got, want)
		}
	}
}

func TestCompleteOdometers(t *testing.T) {
	start, end := CompleteOdometers(fptr(1000), nil, 25)
	if *start != 1000 || *end != 1025 {
		t.Fatalf("from start: %v %v", *start, *end)
	}
	start, end = CompleteOdometers(nil, fptr(1025), 25)
	if *start != 1000 || *end != 1025 {
		t.Fatalf("from end: %v %v", *start, *end)
	}
	start, end = CompleteOdometers(fptr(1), fptr(2), 25)
	if *start != 1 || *end != 2 {
		t.Fatalf("both given must be kept: %v %v", *start, *end)
	}
	start, end = CompleteOdometers(nil, nil, 25)
	if start != nil || end != nil {
		t.Fatal("nothing given, nothing derived")
	}
}

func TestDeriveDistance(t *testing.T) {
	cases := []struct {
		name               string
		reported, from, to *float64
		wantKm             float64
		wantOK             bool
	}{
		{"reported", fptr(12), fptr(1000), fptr(2000), 12, true},
		{"reported zero is refused", fptr(0), nil, nil, 0, false},
		{"reported too long is refused", fptr(4000), nil, nil, 4000, false},
		{"odometer difference", nil, fptr(1000), fptr(1025.5), 25.5, true},
		{"odometer going backwards is unknown", nil, fptr(1000), fptr(990), 0, true},
		{"odometer standing still is unknown", nil, fptr(1000), fptr(1000), 0, true},
		{"one odometer is unknown", nil, fptr(1000), nil, 0, true},
		{"nothing is unknown", nil, nil, nil, 0, true},
	}
	for _, c := range cases {
		km, ok := DeriveDistance(c.reported, c.from, c.to)
		if km != c.wantKm || ok != c.wantOK {
			t.Errorf("%s: got %v, %v want %v, %v", c.name, km, ok, c.wantKm, c.wantOK)
		}
	}
}

func TestNormalizeDriveWithoutDistance(t *testing.T) {
	start := time.Date(2026, 5, 1, 8, 0, 0, 0, time.UTC)
	end := start.Add(20 * time.Minute)

	got := NormalizeDrive(DriveInput{Start: start, End: &end, VehicleKwh100km: fptr(18)})
	if got.EnergyKnown || got.EnergyEstimated || got.EnergyKwh != 0 || got.Kwh100km != 0 {
		t.Errorf("no distance and no energy: nothing to estimate, got %+v", got)
	}
	if got.DurationMin != 20 {
		t.Errorf("duration: got %d, want 20", got.DurationMin)
	}

	got = NormalizeDrive(DriveInput{Start: start, End: &end, EnergyKwh: fptr(4)})
	if !got.EnergyKnown || got.EnergyKwh != 4 || got.Kwh100km != 0 || got.EnergyEstimated {
		t.Errorf("measured energy without distance: got %+v", got)
	}
	if math.IsInf(got.Kwh100km, 0) || math.IsNaN(got.Kwh100km) {
		t.Errorf("consumption must not divide by zero: %v", got.Kwh100km)
	}
}
