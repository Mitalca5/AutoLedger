package demodata

import (
	"reflect"
	"testing"
	"time"
)

var ref = time.Date(2026, 10, 4, 15, 0, 0, 0, time.UTC)

func TestBuildIsDeterministic(t *testing.T) {
	for _, key := range Keys() {
		a, err := Build(key, ref)
		if err != nil {
			t.Fatal(err)
		}
		b, _ := Build(key, ref.Add(3*time.Hour))
		if !reflect.DeepEqual(a, b) {
			t.Errorf("%s: two builds on the same day differ", key)
		}
	}
	if _, err := Build("boat", ref); err == nil {
		t.Error("an unknown dataset must be rejected")
	}
}

func TestEventsAreUniqueAndInThePast(t *testing.T) {
	for _, key := range Keys() {
		ds, _ := Build(key, ref)
		seen := map[string]bool{}
		for _, e := range ds.Events {
			if seen[e.ID] {
				t.Fatalf("%s: duplicate event id %s", key, e.ID)
			}
			seen[e.ID] = true
			if !e.At.Before(ref) {
				t.Fatalf("%s: event %s is not in the past", key, e.ID)
			}
		}
		if len(ds.Events) < 60 {
			t.Errorf("%s: only %d events", key, len(ds.Events))
		}
	}
}

func TestEVChargesStayWithinTheBattery(t *testing.T) {
	ds, _ := Build("ev", ref)
	charges, driven := 0, 0.0
	for _, e := range ds.Events {
		switch e.Type {
		case "charging_session_end":
			charges++
			start, end := e.Data["soc_start"].(int), e.Data["soc_end"].(int)
			if start < 0 || end > 100 || start >= end {
				t.Fatalf("%s: SoC %d -> %d", e.ID, start, end)
			}
			if kwh := e.Data["energy_kwh"].(float64); kwh <= 0 || kwh > 75 {
				t.Fatalf("%s: %.1f kWh", e.ID, kwh)
			}
		case "drive":
			driven += e.Data["distance"].(float64)
		}
	}
	if charges < 20 {
		t.Errorf("only %d charges", charges)
	}
	if got := ds.FinalOdometer - ds.Vehicle.StartOdometer; got < driven-1 || got > driven+1 {
		t.Errorf("odometer advanced %.1f km for %.1f km driven", got, driven)
	}
}

func TestICEFillUpsMatchTheConsumption(t *testing.T) {
	ds, _ := Build("ice", ref)
	fills, liters := 0, 0.0
	for _, e := range ds.Events {
		if e.Type == "fuel" {
			fills++
			liters += e.Data["liters"].(float64)
		}
	}
	if fills < 6 {
		t.Fatalf("only %d fill-ups", fills)
	}
	per100 := liters / (ds.FinalOdometer - ds.Vehicle.StartOdometer) * 100
	if per100 < 5 || per100 > 6.5 {
		t.Errorf("%.2f L/100 km", per100)
	}
}
