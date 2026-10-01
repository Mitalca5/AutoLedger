package models

import "testing"

func TestDerivedTelemetryMode(t *testing.T) {
	url := func(s string) *string { return &s }
	cases := []struct {
		name       string
		powertrain string
		apiURL     *string
		want       string
	}{
		{"electric with a teslamateapi URL", PowertrainEV, url("http://teslamate:8080"), TelemetryConnected},
		{"default powertrain with a URL", "", url("http://teslamate:8080"), TelemetryConnected},
		{"electric without URL", PowertrainEV, nil, TelemetryManual},
		{"blank URL", PowertrainEV, url("   "), TelemetryManual},
		{"combustion, even with a URL", PowertrainICE, url("http://teslamate:8080"), TelemetryManual},
	}
	for _, c := range cases {
		v := &Vehicle{Powertrain: c.powertrain, TeslaMateAPIURL: c.apiURL}
		if got := v.DerivedTelemetryMode(); got != c.want {
			t.Errorf("%s: got %s, want %s", c.name, got, c.want)
		}
	}
}
