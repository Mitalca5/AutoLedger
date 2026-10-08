package money

import (
	"encoding/json"
	"math/big"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
)

func TestRateParseAndString(t *testing.T) {
	cases := map[string]string{
		"0.2516":    "0.2516",
		"0.20":      "0.20",
		"0.2":       "0.20",
		"1":         "1.00",
		"0.34925":   "0.34925",
		"0.0000005": "0.000001",
		"1e-1":      "0.10",
		"-0.2516":   "-0.2516",
	}
	for in, want := range cases {
		r, err := ParseRate(in)
		if err != nil {
			t.Fatalf("%s: %v", in, err)
		}
		if r.String() != want {
			t.Errorf("%s: got %s want %s", in, r, want)
		}
	}
	for _, bad := range []string{"abc", "1000000", "-1000000"} {
		if _, err := ParseRate(bad); err == nil {
			t.Errorf("%q should be rejected", bad)
		}
	}
}

func TestRateCostKeepsPrecision(t *testing.T) {
	r, _ := ParseRate("0.2516")
	if got := r.Cost(10); got != 252 {
		t.Errorf("10 kWh at 0.2516: got %s want 2.52", got)
	}
	if got := r.Cost(100); got != 2516 {
		t.Errorf("100 kWh at 0.2516: got %s want 25.16", got)
	}
	r, _ = ParseRate("0.349")
	if got := r.Cost(40); got != 1396 {
		t.Errorf("40 kWh at 0.349: got %s want 13.96", got)
	}
}

func TestRateJSONRoundTrip(t *testing.T) {
	var v struct {
		A Rate  `json:"a"`
		B *Rate `json:"b"`
	}
	if err := json.Unmarshal([]byte(`{"a": 0.1696, "b": "0.25"}`), &v); err != nil {
		t.Fatal(err)
	}
	if v.A != 169600 || *v.B != 250000 {
		t.Fatalf("got %d %d", v.A, *v.B)
	}
	out, _ := json.Marshal(v)
	if string(out) != `{"a":0.1696,"b":0.25}` {
		t.Errorf("got %s", out)
	}
}

func TestRateNumeric(t *testing.T) {
	var r Rate
	if err := r.ScanNumeric(pgtype.Numeric{Int: big.NewInt(251600), Exp: -6, Valid: true}); err != nil || r != 251600 {
		t.Fatalf("got %d %v", r, err)
	}
	if err := r.ScanNumeric(pgtype.Numeric{Int: big.NewInt(3), Exp: -1, Valid: true}); err != nil || r != 300000 {
		t.Fatalf("got %d %v", r, err)
	}
	if err := r.ScanNumeric(pgtype.Numeric{}); err == nil {
		t.Error("NULL should fail")
	}
	if err := r.ScanNumeric(pgtype.Numeric{NaN: true, Valid: true}); err == nil {
		t.Error("NaN should fail")
	}
	n, _ := Rate(251600).NumericValue()
	if n.Int.Int64() != 251600 || n.Exp != -6 {
		t.Errorf("got %+v", n)
	}
}
