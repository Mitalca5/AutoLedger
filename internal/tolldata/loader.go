package tolldata

import (
	"encoding/json"
	"fmt"
	"log"
	"strconv"

	"github.com/teslacost/teslacost/internal/money"
)

// Station is a single toll gate/barrier from the OpenTollData reference dataset.
type Station struct {
	Name     string
	Operator string
	Type     string // "open" or "close"
	Lat      float64
	Lon      float64
	FreeFlow bool // closed-network station crossed at full speed, so never filtered as a drive-by
}

// Dataset is the in-memory, ready-to-query form of the vendored OpenTollData snapshot.
type Dataset struct {
	Stations         []Station
	NetworkByStation map[string]string                 // station name -> network_name (closed-network connected component)
	OpenPrice        map[string]money.Cents            // station name -> class 1 price
	ClosedPrice      map[string]map[string]money.Cents // entry station -> exit station -> class 1 price
}

type rawPrice struct {
	Price map[string]string `json:"price"`
}

type rawTollDescription struct {
	OperatorRef string `json:"operator_ref"`
	Lat         string `json:"lat"`
	Lon         string `json:"lon"`
	Operator    string `json:"operator"`
	Type        string `json:"type"`
}

type rawNetwork struct {
	NetworkName string                         `json:"network_name"`
	Tolls       []string                       `json:"tolls"`
	Connection  map[string]map[string]rawPrice `json:"connection"`
}

type rawFile struct {
	TollDescription map[string]rawTollDescription `json:"toll_description"`
	Networks        []rawNetwork                  `json:"networks"`
	OpenTollPrice   map[string]rawPrice           `json:"open_toll_price"`
}

type rawExtraGates struct {
	Gates []struct {
		Station  string  `json:"station"`
		Lat      float64 `json:"lat"`
		Lon      float64 `json:"lon"`
		FreeFlow bool    `json:"free_flow"`
	} `json:"gates"`
}

// loadExtraGates returns the gates of extra_gates.json, each taking the operator and type of the
// dataset station it belongs to (same name, hence same network and prices).
func loadExtraGates(stations []Station) ([]Station, error) {
	raw, err := FS.ReadFile("data/extra_gates.json")
	if err != nil {
		return nil, fmt.Errorf("failed to read embedded extra toll gates: %w", err)
	}
	var f rawExtraGates
	if err := json.Unmarshal(raw, &f); err != nil {
		return nil, fmt.Errorf("failed to parse embedded extra toll gates: %w", err)
	}

	byName := make(map[string]Station, len(stations))
	for _, st := range stations {
		byName[st.Name] = st
	}
	gates := make([]Station, 0, len(f.Gates))
	for _, g := range f.Gates {
		base, ok := byName[g.Station]
		if !ok {
			return nil, fmt.Errorf("extra toll gate refers to unknown station %q", g.Station)
		}
		gates = append(gates, Station{Name: base.Name, Operator: base.Operator, Type: base.Type, Lat: g.Lat, Lon: g.Lon, FreeFlow: g.FreeFlow})
	}
	return gates, nil
}

// Load parses the embedded OpenTollData snapshot into an in-memory Dataset.
func Load() (*Dataset, error) {
	raw, err := FS.ReadFile("data/opentolldata_network.json")
	if err != nil {
		return nil, fmt.Errorf("failed to read embedded toll dataset: %w", err)
	}

	var f rawFile
	if err := json.Unmarshal(raw, &f); err != nil {
		return nil, fmt.Errorf("failed to parse embedded toll dataset: %w", err)
	}

	ds := &Dataset{
		Stations:         make([]Station, 0, len(f.TollDescription)),
		NetworkByStation: make(map[string]string, len(f.TollDescription)),
		OpenPrice:        make(map[string]money.Cents, len(f.OpenTollPrice)),
		ClosedPrice:      make(map[string]map[string]money.Cents),
	}

	for name, td := range f.TollDescription {
		lat, errLat := strconv.ParseFloat(td.Lat, 64)
		lon, errLon := strconv.ParseFloat(td.Lon, 64)
		if errLat != nil || errLon != nil {
			log.Printf("[tolldata] skipping station %q: invalid coordinates (lat=%q lon=%q)", name, td.Lat, td.Lon)
			continue
		}
		ds.Stations = append(ds.Stations, Station{Name: name, Operator: td.Operator, Type: td.Type, Lat: lat, Lon: lon})
	}

	gates, err := loadExtraGates(ds.Stations)
	if err != nil {
		return nil, err
	}
	ds.Stations = append(ds.Stations, gates...)

	for _, net := range f.Networks {
		for _, toll := range net.Tolls {
			ds.NetworkByStation[toll] = net.NetworkName
		}
		for from, tos := range net.Connection {
			for to, rp := range tos {
				price, ok := parseClass1(rp)
				if !ok {
					continue
				}
				if ds.ClosedPrice[from] == nil {
					ds.ClosedPrice[from] = make(map[string]money.Cents)
				}
				ds.ClosedPrice[from][to] = price
			}
		}
	}

	for name, rp := range f.OpenTollPrice {
		price, ok := parseClass1(rp)
		if !ok {
			log.Printf("[tolldata] skipping open toll price for %q: missing or invalid class_1 price", name)
			continue
		}
		ds.OpenPrice[name] = price
	}

	return ds, nil
}

// parseClass1 extracts and converts the class_1 (light vehicle) price from a raw price entry.
func parseClass1(rp rawPrice) (money.Cents, bool) {
	raw, ok := rp.Price["class_1"]
	if !ok {
		return 0, false
	}
	v, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return 0, false
	}
	return money.FromFloat(v), true
}
