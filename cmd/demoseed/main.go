// Command demoseed fills a running AutoLedger instance with a deterministic demo dataset through its public
// HTTP API: a user, an electric vehicle and a combustion vehicle with about eight months of history.
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/cookiejar"
	"os"
	"strings"
	"time"

	"github.com/teslacost/teslacost/internal/demodata"
)

type client struct {
	base  string
	http  *http.Client
	token string
}

func (c *client) do(method, path string, body, out any, bearer string) error {
	var rd io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return err
		}
		rd = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, c.base+path, rd)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if bearer == "" {
		bearer = c.token
	}
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	res, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	data, _ := io.ReadAll(res.Body)
	if res.StatusCode >= 300 {
		return fmt.Errorf("%s %s: %d %s", method, path, res.StatusCode, strings.TrimSpace(string(data)))
	}
	if out != nil && len(data) > 0 {
		return json.Unmarshal(data, out)
	}
	return nil
}

func (c *client) login(email, password string) error {
	creds := map[string]string{"email": email, "password": password}
	// The account may already exist (a re-run): registration then fails and the login below decides.
	_ = c.do("POST", "/api/auth/register", creds, nil, "")
	var res struct {
		Token       string `json:"token"`
		AccessToken string `json:"access_token"`
	}
	if err := c.do("POST", "/api/auth/login", creds, &res, ""); err != nil {
		return err
	}
	c.token = res.AccessToken
	if c.token == "" {
		c.token = res.Token
	}
	if c.token == "" {
		return fmt.Errorf("login returned no token")
	}
	return nil
}

func day(t time.Time) string { return t.Format("2006-01-02") }

func (c *client) seed(ds demodata.Dataset, apiToken string) error {
	var v struct {
		ID string `json:"id"`
	}
	err := c.do("POST", "/api/vehicles", map[string]any{
		"name": ds.Vehicle.Name, "make": ds.Vehicle.Make, "model": ds.Vehicle.Model,
		"powertrain": ds.Vehicle.Powertrain, "current_odometer": ds.Vehicle.StartOdometer,
		"currency": ds.Vehicle.Currency, "estimated_kwh_100km": ds.Vehicle.KWhPer100Km,
		"estimated_price_per_kwh": ds.Vehicle.PricePerKWh,
	}, &v, "")
	if err != nil {
		return err
	}
	for _, e := range ds.Events {
		err := c.do("POST", "/api/integrations/homeassistant/event", map[string]any{
			"event_type": e.Type, "vehicle_id": v.ID, "event_id": e.ID,
			"timestamp": e.At.Format(time.RFC3339), "data": e.Data,
		}, nil, apiToken)
		if err != nil {
			return err
		}
	}
	for _, t := range ds.Tires {
		err := c.do("POST", "/api/vehicles/"+v.ID+"/tires", map[string]any{
			"brand": t.Brand, "model": t.Model, "dimension": t.Dimension, "season": t.Season,
			"purchase_date": day(t.PurchaseDate), "purchase_price": t.PriceCents, "current_position": t.Position,
			"initial_depth_mm": 8.0, "min_legal_depth_mm": 1.6, "mounted_odometer": t.MountedOdometer,
			"estimated_lifespan_km": t.LifespanKm,
		}, nil, "")
		if err != nil {
			return err
		}
	}
	for _, x := range ds.Expenses {
		var err error
		if x.Kind == "toll" {
			err = c.do("POST", "/api/vehicles/"+v.ID+"/expenses", map[string]any{
				"type": "TOLL", "amount": x.AmountCents, "currency": ds.Vehicle.Currency,
				"date": day(x.Date), "notes": x.Description,
			}, nil, "")
		} else {
			err = c.do("POST", "/api/vehicles/"+v.ID+"/maintenance", map[string]any{
				"category": x.Category, "amount": x.AmountCents, "currency": ds.Vehicle.Currency,
				"date": day(x.Date), "description": x.Description, "amortization_mode": "NONE",
			}, nil, "")
		}
		if err != nil {
			return err
		}
	}
	for _, r := range ds.Reminders {
		err := c.do("POST", "/api/vehicles/"+v.ID+"/reminders", map[string]any{
			"title": r.Title, "due_date": day(r.Due),
		}, nil, "")
		if err != nil {
			return err
		}
	}
	log.Printf("%s: %d events, %d tires, %d expenses, %d reminders", ds.Vehicle.Name, len(ds.Events), len(ds.Tires), len(ds.Expenses), len(ds.Reminders))
	return nil
}

func main() {
	url := flag.String("url", "http://localhost:8080", "base URL of the AutoLedger instance")
	email := flag.String("email", "demo@autoledger.example", "demo account email (created when absent)")
	password := flag.String("password", os.Getenv("DEMO_PASSWORD"), "demo account password (default: $DEMO_PASSWORD)")
	sets := flag.String("datasets", "ev,ice", "comma-separated datasets: ev, ice")
	flag.Parse()
	if *password == "" {
		log.Fatal("a password is required (-password or $DEMO_PASSWORD)")
	}

	jar, _ := cookiejar.New(nil)
	c := &client{base: strings.TrimRight(*url, "/"), http: &http.Client{Jar: jar, Timeout: 30 * time.Second}}
	if err := c.login(*email, *password); err != nil {
		log.Fatalf("login: %v", err)
	}
	var tok struct {
		Token string `json:"token"`
	}
	if err := c.do("POST", "/api/auth/tokens/", map[string]string{"name": "demo seed"}, &tok, ""); err != nil {
		log.Fatalf("api token: %v", err)
	}
	for _, key := range strings.Split(*sets, ",") {
		ds, err := demodata.Build(strings.TrimSpace(key), time.Now())
		if err != nil {
			log.Fatal(err)
		}
		if err := c.seed(ds, tok.Token); err != nil {
			log.Fatalf("%s: %v", key, err)
		}
	}
}
