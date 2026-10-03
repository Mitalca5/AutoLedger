package models

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestPowertrainCapabilities(t *testing.T) {
	cases := []struct {
		powertrain                    string
		charge, refuel, linkTeslaMate bool
	}{
		{PowertrainEV, true, false, true},
		{PowertrainICE, false, true, false},
		{PowertrainPHEV, true, true, true},
		{PowertrainREEV, true, true, true},
		{"", true, false, true},
	}
	for _, c := range cases {
		v := &Vehicle{Powertrain: c.powertrain}
		if v.CanCharge() != c.charge || v.CanRefuel() != c.refuel || v.CanLinkTeslaMate() != c.linkTeslaMate {
			t.Errorf("powertrain %q: got charge=%v refuel=%v teslamate=%v", c.powertrain, v.CanCharge(), v.CanRefuel(), v.CanLinkTeslaMate())
		}
	}
}

// Code outside this package asks for a capability; only validation and defaults may name a powertrain.
func TestNoPowertrainComparisonOutsideModels(t *testing.T) {
	re := regexp.MustCompile(`(==|!=)\s*(models\.)?Powertrain(EV|ICE)\b`)
	root := filepath.Join("..", "..", "internal")
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return err
		}
		if strings.Contains(filepath.ToSlash(path), "internal/models/") {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if loc := re.FindIndex(data); loc != nil {
			t.Errorf("%s compares a powertrain directly; use a Can* capability", path)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
