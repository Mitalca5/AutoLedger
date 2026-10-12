package handlers

import (
	"testing"
)

func TestComparisonProjectionRequiresCurrency(t *testing.T) {
	for _, c := range []struct {
		in, want string
		valid    bool
	}{{"usd", "USD", true}, {" ARS ", "ARS", true}, {"", "", false}, {"EU1", "", false}} {
		r := projectionPreviewRequest()
		r.Name = "Currency"
		r.Currency = c.in
		err := validateComparisonRequest(&r)
		if (err == nil) != c.valid {
			t.Fatalf("currency %q: %v", c.in, err)
		}
		if c.valid && r.toScenario("user").Currency != c.want {
			t.Fatalf("currency not retained: %+v", r)
		}
	}
}
