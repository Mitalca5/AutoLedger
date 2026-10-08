package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/teslacost/teslacost/internal/middleware"
	"github.com/teslacost/teslacost/internal/models"
	"github.com/teslacost/teslacost/internal/services"
)

type tariffAPI struct {
	t      *testing.T
	router chi.Router
	userID string
	vid    string
}

func newTariffAPI(t *testing.T, email string) *tariffAPI {
	t.Helper()
	repo := authTestRepo(t)
	ctx := context.Background()
	u, err := repo.CreateUser(ctx, email, "hash")
	if err != nil {
		t.Fatal(err)
	}
	v := &models.Vehicle{UserID: u.ID, Name: "Car", TeslaMateAuthType: models.AuthModeNone, CurrentOdometer: 1000}
	if err := repo.CreateVehicle(ctx, v); err != nil {
		t.Fatal(err)
	}
	h := NewTariffHandler(repo, services.NewTariffService())
	r := chi.NewRouter()
	r.Get("/plans", h.List)
	r.Post("/plans", h.Create)
	r.Put("/plans/{id}", h.Update)
	r.Delete("/plans/{id}", h.Delete)
	r.Post("/calculate-session", h.Calculate)
	r.Get("/public-presets", h.ListPublicPresets)
	r.Post("/public-presets", h.CreatePublicPreset)
	r.Delete("/public-presets/{id}", h.DeletePublicPreset)
	r.Post("/calculate-public", h.CalculatePublic)
	return &tariffAPI{t: t, router: r, userID: u.ID, vid: v.ID}
}

func (a *tariffAPI) expect(want int, method, path, body string) map[string]any {
	a.t.Helper()
	req := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	req = req.WithContext(context.WithValue(req.Context(), middleware.UserIDKey, a.userID))
	rec := httptest.NewRecorder()
	a.router.ServeHTTP(rec, req)
	if rec.Code != want {
		a.t.Fatalf("%s %s %s: got %d, want %d (%s)", method, path, body, rec.Code, want, rec.Body.String())
	}
	var obj map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &obj)
	return obj
}

func TestTariffPlanLifecycle(t *testing.T) {
	a := newTariffAPI(t, "tariff-plan@example.com")

	a.expect(http.StatusBadRequest, "POST", "/plans", `{`)
	a.expect(http.StatusBadRequest, "POST", "/plans", `{"name":"  "}`)

	plan := a.expect(http.StatusCreated, "POST", "/plans", `{"name":" Home ","plan_type":"bogus","flat_rate_cents":0.2,"is_default":true}`)
	if plan["name"] != "Home" || plan["plan_type"] != models.TariffTypeTimeOfUse || plan["currency"] != "EUR" {
		t.Fatalf("defaults not applied: %v", plan)
	}
	id := plan["id"].(string)

	flat := a.expect(http.StatusCreated, "POST", "/plans", `{"name":"Flat","plan_type":"FLAT","currency":"usd","flat_rate_cents":0.3}`)
	if flat["currency"] != "USD" || flat["plan_type"] != models.TariffTypeFlat {
		t.Fatalf("flat plan: %v", flat)
	}

	list := a.expect(http.StatusOK, "GET", "/plans", "")
	if len(list["plans"].([]any)) != 2 {
		t.Fatalf("expected 2 plans, got %v", list["plans"])
	}

	upd := a.expect(http.StatusOK, "PUT", "/plans/"+id, `{"name":"Home 2","currency":"gbp","flat_rate_cents":0.25,"valid_from":"2026-01-01","valid_to":"2026-12-31"}`)
	if upd["name"] != "Home 2" || upd["currency"] != "GBP" || upd["valid_from"] != "2026-01-01" {
		t.Fatalf("update not applied: %v", upd)
	}
	a.expect(http.StatusBadRequest, "PUT", "/plans/"+id, `{"valid_from":"nope"}`)
	a.expect(http.StatusBadRequest, "PUT", "/plans/"+id, `{`)
	a.expect(http.StatusNotFound, "PUT", "/plans/00000000-0000-0000-0000-000000000000", `{"name":"x"}`)

	a.expect(http.StatusOK, "DELETE", "/plans/"+flat["id"].(string), "")
	list = a.expect(http.StatusOK, "GET", "/plans", "")
	if len(list["plans"].([]any)) != 1 {
		t.Fatalf("expected 1 plan after delete, got %v", list["plans"])
	}
}

func TestTariffBandsValidation(t *testing.T) {
	a := newTariffAPI(t, "tariff-bands@example.com")

	cases := map[string]string{
		"no bands":          `{"name":"B","plan_type":"BANDS"}`,
		"blank band name":   `{"name":"B","plan_type":"BANDS","bands":[{"name":" ","rate_cents":0.1}]}`,
		"duplicate band":    `{"name":"B","plan_type":"BANDS","bands":[{"name":"a","rate_cents":0.1},{"name":"a","rate_cents":0.2}]}`,
		"unknown default":   `{"name":"B","plan_type":"BANDS","default_band":"z","bands":[{"name":"a","rate_cents":0.1}]}`,
		"unknown rule band": `{"name":"B","plan_type":"BANDS","bands":[{"name":"a","rate_cents":0.1}],"rules":[{"start":"00:00","end":"06:00","band":"z"}]}`,
		"bad time":          `{"name":"B","plan_type":"BANDS","bands":[{"name":"a","rate_cents":0.1}],"rules":[{"start":"25:00","end":"06:00","band":"a"}]}`,
		"bad day":           `{"name":"B","plan_type":"BANDS","bands":[{"name":"a","rate_cents":0.1}],"rules":[{"days":[7],"start":"00:00","end":"06:00","band":"a"}]}`,
		"bad validity":      `{"name":"B","plan_type":"BANDS","bands":[{"name":"a","rate_cents":0.1}],"valid_from":"2026-05-01","valid_to":"2026-01-01"}`,
		"invalid day":       `{"name":"B","plan_type":"BANDS","bands":[{"name":"a","rate_cents":0.1}],"valid_to":"31/12/2026"}`,
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) { a.expect(http.StatusBadRequest, "POST", "/plans", body) })
	}

	ok := `{"name":"Grid","plan_type":"BANDS","default_band":"peak","standing_charge_cents":0.5,` +
		`"bands":[{"name":"peak","rate_cents":0.3},{"name":"night","rate_cents":0.1}],` +
		`"rules":[{"days":[1,2],"start":"22:00","end":"24:00","band":"night"}],"valid_from":""}`
	plan := a.expect(http.StatusCreated, "POST", "/plans", ok)
	if plan["valid_from"] != nil || len(plan["bands"].([]any)) != 2 {
		t.Fatalf("bands plan: %v", plan)
	}

	id := plan["id"].(string)
	a.expect(http.StatusBadRequest, "PUT", "/plans/"+id, `{"bands":[]}`)
	upd := a.expect(http.StatusOK, "PUT", "/plans/"+id, `{"default_band":"night","bands":[{"name":"peak","rate_cents":0.4},{"name":"night","rate_cents":0.1}]}`)
	if upd["default_band"] != "night" {
		t.Fatalf("bands update: %v", upd)
	}
}

func TestTariffCalculateSession(t *testing.T) {
	a := newTariffAPI(t, "tariff-calc@example.com")
	window := `"start_time":"2026-03-10T10:00:00Z","end_time":"2026-03-10T12:00:00Z","kwh":10`

	a.expect(http.StatusBadRequest, "POST", "/calculate-session", `{`)

	none := a.expect(http.StatusOK, "POST", "/calculate-session", `{`+window+`}`)
	if none["plan"] != nil {
		t.Fatalf("no default plan should leave plan null: %v", none)
	}

	plan := a.expect(http.StatusCreated, "POST", "/plans", `{"name":"Flat","plan_type":"FLAT","flat_rate_cents":0.2,"is_default":true}`)
	byDefault := a.expect(http.StatusOK, "POST", "/calculate-session", `{`+window+`}`)
	if byDefault["plan"] != "Flat" || byDefault["cost"].(float64) != 2.0 {
		t.Fatalf("default plan: %v", byDefault)
	}
	byID := a.expect(http.StatusOK, "POST", "/calculate-session", `{"plan_id":"`+plan["id"].(string)+`",`+window+`}`)
	if byID["plan"] != "Flat" {
		t.Fatalf("plan by id: %v", byID)
	}
	a.expect(http.StatusNotFound, "POST", "/calculate-session", `{"plan_id":"00000000-0000-0000-0000-000000000000",`+window+`}`)
	byVehicle := a.expect(http.StatusOK, "POST", "/calculate-session", `{"vehicle_id":"`+a.vid+`",`+window+`}`)
	if byVehicle["plan"] != "Flat" {
		t.Fatalf("vehicle without its own tariff falls back to the default plan: %v", byVehicle)
	}
	a.expect(http.StatusNotFound, "POST", "/calculate-session", `{"vehicle_id":"00000000-0000-0000-0000-000000000000",`+window+`}`)
}

func TestPublicChargingPresets(t *testing.T) {
	a := newTariffAPI(t, "tariff-presets@example.com")

	a.expect(http.StatusBadRequest, "POST", "/public-presets", `{`)
	a.expect(http.StatusBadRequest, "POST", "/public-presets", `{"name":" "}`)

	p := a.expect(http.StatusCreated, "POST", "/public-presets", `{"name":" Ionity ","price_per_kwh":0.69,"connection_fee":1}`)
	if p["name"] != "Ionity" || p["currency"] != "EUR" {
		t.Fatalf("preset defaults: %v", p)
	}
	list := a.expect(http.StatusOK, "GET", "/public-presets", "")
	if len(list["presets"].([]any)) != 1 {
		t.Fatalf("presets: %v", list)
	}

	calc := a.expect(http.StatusOK, "POST", "/calculate-public", `{"kwh":10,"charging_minutes":30,"total_plugged_minutes":30,"connection_fee":1,"price_per_kwh":0.5}`)
	if calc["energy_cost"].(float64) != 5.0 || calc["connection_cost"].(float64) != 1.0 {
		t.Fatalf("public breakdown: %v", calc)
	}
	a.expect(http.StatusBadRequest, "POST", "/calculate-public", `{`)

	a.expect(http.StatusOK, "DELETE", "/public-presets/"+p["id"].(string), "")
	list = a.expect(http.StatusOK, "GET", "/public-presets", "")
	if len(list["presets"].([]any)) != 0 {
		t.Fatalf("preset not deleted: %v", list)
	}
}
