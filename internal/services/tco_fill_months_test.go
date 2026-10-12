package services

import "testing"

func TestFillMonthsMakesTheTimelineContinuous(t *testing.T) {
	in := []MonthlyCost{{Month: "2026-01", Energy: 100}, {Month: "2026-04", Energy: 50}}
	got := fillMonths(in, "2026-06")
	want := []string{"2026-01", "2026-02", "2026-03", "2026-04", "2026-05", "2026-06"}
	if len(got) != len(want) {
		t.Fatalf("got %d months, want %d: %+v", len(got), len(want), got)
	}
	for i, m := range want {
		if got[i].Month != m {
			t.Fatalf("month %d: got %s, want %s", i, got[i].Month, m)
		}
	}
	if got[0].Energy != 100 || got[3].Energy != 50 || got[1].Energy != 0 || got[4].Energy != 0 {
		t.Fatalf("recorded amounts must stay on their month and gaps be empty: %+v", got)
	}
	if got[1].CostPerKm != nil {
		t.Fatal("an empty month has no known cost per km")
	}
}

func TestFillMonthsKeepsALaterRecordedMonthAndHandlesEmptyHistory(t *testing.T) {
	got := fillMonths([]MonthlyCost{{Month: "2026-05"}, {Month: "2026-08"}}, "2026-06")
	if len(got) != 4 || got[3].Month != "2026-08" {
		t.Fatalf("a month recorded after the current one (a due instalment) must stay: %+v", got)
	}
	if got := fillMonths(nil, "2026-10"); len(got) != 1 || got[0].Month != "2026-10" {
		t.Fatalf("no record: the current month alone, got %+v", got)
	}
}
