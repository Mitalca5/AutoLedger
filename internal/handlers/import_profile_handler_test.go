package handlers

import (
	"strings"
	"testing"
)

func TestValidateImportProfile(t *testing.T) {
	ok := saveImportProfileRequest{Name: "  Bank  ", ImportType: "CHARGES", Columns: map[string]string{"A": "kwh", "B": ""}, DateOrder: "mdy", DecimalSeparator: ","}
	p, err := validateImportProfile(&ok)
	if err != nil || p.Name != "Bank" {
		t.Fatalf("valid profile rejected: %v", err)
	}
	if p, _ = validateImportProfile(&saveImportProfileRequest{Name: "x", ImportType: "ODOMETER"}); p == nil || p.Columns == nil {
		t.Error("a profile without columns keeps an empty map")
	}
	for name, req := range map[string]saveImportProfileRequest{
		"blank name":    {Name: " ", ImportType: "CHARGES"},
		"long name":     {Name: strings.Repeat("a", 61), ImportType: "CHARGES"},
		"unknown type":  {Name: "x", ImportType: "NOPE"},
		"unknown field": {Name: "x", ImportType: "CHARGES", Columns: map[string]string{"A": "liters"}},
		"bad date":      {Name: "x", ImportType: "CHARGES", DateOrder: "zzz"},
		"bad decimal":   {Name: "x", ImportType: "CHARGES", DecimalSeparator: ";"},
	} {
		if _, err := validateImportProfile(&req); err == nil {
			t.Errorf("%s: expected an error", name)
		}
	}
}
