package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/teslacost/teslacost/internal/crypto"
	"github.com/teslacost/teslacost/internal/database"
	"github.com/teslacost/teslacost/internal/middleware"
	"github.com/teslacost/teslacost/internal/models"
	"github.com/teslacost/teslacost/internal/services"
)

type vehicleAPI struct {
	t       *testing.T
	router  chi.Router
	repo    *database.Repository
	ownerID string
	viewer  string
	editor  string
	other   string
	vid     string
}

func newVehicleAPI(t *testing.T, tag string) *vehicleAPI {
	t.Helper()
	repo := authTestRepo(t)
	ctx := context.Background()
	mk := func(name string) string {
		u, err := repo.CreateUser(ctx, name+"-"+tag+"@example.com", "hash")
		if err != nil {
			t.Fatal(err)
		}
		return u.ID
	}
	owner, viewer, editor, other := mk("owner"), mk("viewer"), mk("editor"), mk("other")
	v := &models.Vehicle{UserID: owner, Name: "Model 3", TeslaMateAuthType: models.AuthModeNone, CurrentOdometer: 1000}
	if err := repo.CreateVehicle(ctx, v); err != nil {
		t.Fatal(err)
	}
	for id, role := range map[string]models.VehicleRole{viewer: models.RoleViewer, editor: models.RoleEditor} {
		email := "viewer-" + tag + "@example.com"
		if id == editor {
			email = "editor-" + tag + "@example.com"
		}
		if _, err := repo.AddVehicleMember(ctx, v.ID, email, role, nil); err != nil {
			t.Fatal(err)
		}
	}
	enc, err := crypto.NewEncryptor("0123456789abcdef0123456789abcdef")
	if err != nil {
		t.Fatal(err)
	}
	h := NewVehicleHandler(repo, enc, services.NewSyncService(repo, enc))
	r := chi.NewRouter()
	r.Get("/", h.List)
	r.Post("/", h.Create)
	r.Post("/test-connection", h.TestTeslaMateRaw)
	r.Get("/{id}", h.Get)
	r.Put("/{id}", h.Update)
	r.Delete("/{id}", h.Delete)
	r.Post("/{id}/teslamate/test", h.TestTeslaMate)
	r.Post("/{id}/sync", h.Sync)
	r.Get("/{id}/sync", h.GetSyncStatus)
	r.Put("/{id}/estimated-energy", h.UpdateEstimatedEnergy)
	r.Get("/{id}/odometer-at", h.GetOdometerAtDate)
	r.Get("/{id}/odometer-estimate", h.GetOdometerEstimate)
	r.Get("/{id}/data-sources", h.GetDataSources)
	return &vehicleAPI{t: t, router: r, repo: repo, ownerID: owner, viewer: viewer, editor: editor, other: other, vid: v.ID}
}

func (a *vehicleAPI) do(userID string, want int, method, path, body string) []byte {
	a.t.Helper()
	req := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	req = req.WithContext(context.WithValue(req.Context(), middleware.UserIDKey, userID))
	rec := httptest.NewRecorder()
	a.router.ServeHTTP(rec, req)
	if rec.Code != want {
		a.t.Fatalf("%s %s %s: got %d, want %d (%s)", method, path, body, rec.Code, want, rec.Body.String())
	}
	return rec.Body.Bytes()
}

func TestVehicleCreateValidation(t *testing.T) {
	a := newVehicleAPI(t, "cv")
	cases := map[string]string{
		"bad json":         `{`,
		"no name":          `{"name":""}`,
		"bad powertrain":   `{"name":"x","powertrain":"STEAM"}`,
		"ice teslamate":    `{"name":"x","powertrain":"ICE","teslamate_api_url":"http://tm"}`,
		"bad currency":     `{"name":"x","currency":"EU1"}`,
		"grafana invalid":  `{"name":"x","teslamate_grafana_url":"ftp://g"}`,
		"grafana creds":    `{"name":"x","teslamate_grafana_url":"http://u:p@g"}`,
		"grafana query":    `{"name":"x","teslamate_grafana_url":"http://g/?a=1"}`,
		"grafana too long": `{"name":"x","teslamate_grafana_url":"http://` + strings.Repeat("a", 300) + `"}`,
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) { a.do(a.ownerID, http.StatusBadRequest, "POST", "/", body) })
	}
}

func TestVehicleLifecycle(t *testing.T) {
	a := newVehicleAPI(t, "lc")

	created := decodeObj(t, a.do(a.other, http.StatusCreated, "POST", "/",
		`{"name":" Ioniq ","currency":"usd","powertrain":"EV","make":" Hyundai ","model":"Ioniq 5","teslamate_car_id":0,`+
			`"teslamate_api_key":"secret","teslamate_basic_pass":"pw","teslamate_grafana_url":"https://g.example.com/"}`))
	if created["currency"] != "USD" || created["make"] != "Hyundai" || created["teslamate_grafana_url"] != "https://g.example.com" {
		t.Fatalf("unexpected created vehicle: %v", created)
	}
	if created["teslamate_car_id"] != nil {
		t.Fatalf("a non-positive car id must be dropped: %v", created["teslamate_car_id"])
	}
	id := created["id"].(string)

	ice := decodeObj(t, a.do(a.other, http.StatusCreated, "POST", "/",
		`{"name":"Clio","powertrain":"ICE","teslamate_car_id":3,"teslamate_api_key":"k","teslamate_auth_type":"API_KEY"}`))
	if ice["teslamate_car_id"] != nil || ice["teslamate_auth_type"] != string(models.AuthModeNone) {
		t.Fatalf("an ICE vehicle must not keep TeslaMate settings: %v", ice)
	}

	var list []map[string]any
	if err := json.Unmarshal(a.do(a.other, http.StatusOK, "GET", "/", ""), &list); err != nil || len(list) != 2 {
		t.Fatalf("list: %v %v", list, err)
	}
	if err := json.Unmarshal(a.do(a.ownerID, http.StatusOK, "GET", "/", ""), &list); err != nil || len(list) != 1 {
		t.Fatalf("owner list: %v %v", list, err)
	}

	a.do(a.other, http.StatusOK, "GET", "/"+id, "")
	a.do(a.ownerID, http.StatusNotFound, "GET", "/"+id, "")

	// Update: access, validation, merge rules.
	a.do(a.ownerID, http.StatusNotFound, "PUT", "/"+id, `{"name":"x"}`)
	a.do(a.viewer, http.StatusForbidden, "PUT", "/"+a.vid, `{"name":"x"}`)
	a.do(a.editor, http.StatusForbidden, "PUT", "/"+a.vid, `{"name":"x"}`)
	a.do(a.other, http.StatusBadRequest, "PUT", "/"+id, `{`)
	a.do(a.other, http.StatusBadRequest, "PUT", "/"+id, `{"name":"x","powertrain":"WAT"}`)
	a.do(a.other, http.StatusBadRequest, "PUT", "/"+id, `{"name":"x","teslamate_grafana_url":"nope"}`)
	a.do(a.other, http.StatusBadRequest, "PUT", "/"+id, `{"name":"x","default_driver_id":"00000000-0000-0000-0000-000000000000"}`)

	updated := decodeObj(t, a.do(a.other, http.StatusOK, "PUT", "/"+id,
		`{"name":"Ioniq 6","current_odometer":4200,"teslamate_api_url":"http://tm:8080","teslamate_car_id":2,`+
			`"teslamate_auth_type":"API_KEY","teslamate_api_key":"new","teslamate_basic_user":"u","teslamate_basic_pass":"p",`+
			`"estimated_kwh_100km":15.5,"estimated_price_per_kwh":0.2,"make":"","model":"Six","is_home_charger_default":true,`+
			`"teslamate_grafana_url":""}`))
	if updated["name"] != "Ioniq 6" || updated["current_odometer"].(float64) != 4200 || updated["make"] != "" || updated["model"] != "Six" {
		t.Fatalf("unexpected updated vehicle: %v", updated)
	}
	if updated["teslamate_grafana_url"] != nil {
		t.Fatalf("an empty Grafana URL clears it: %v", updated["teslamate_grafana_url"])
	}
	// Non-positive car id clears the link; an omitted odometer keeps the stored one.
	cleared := decodeObj(t, a.do(a.other, http.StatusOK, "PUT", "/"+id, `{"name":"Ioniq 6","teslamate_car_id":-1}`))
	if cleared["teslamate_car_id"] != nil || cleared["current_odometer"].(float64) != 4200 {
		t.Fatalf("unexpected cleared vehicle: %v", cleared)
	}

	// An ICE vehicle drops every TeslaMate setting on update.
	iceID := ice["id"].(string)
	a.do(a.other, http.StatusBadRequest, "PUT", "/"+iceID, `{"name":"Clio","teslamate_api_url":"http://tm"}`)
	a.do(a.other, http.StatusOK, "PUT", "/"+iceID, `{"name":"Clio 5","teslamate_car_id":4}`)

	// Delete: only the owner.
	a.do(a.viewer, http.StatusForbidden, "DELETE", "/"+a.vid, "")
	a.do(a.ownerID, http.StatusNotFound, "DELETE", "/"+id, "")
	a.do(a.other, http.StatusOK, "DELETE", "/"+id, "")
	a.do(a.other, http.StatusNotFound, "GET", "/"+id, "")
}

func TestVehicleUpdateDefaultDriver(t *testing.T) {
	a := newVehicleAPI(t, "dd")
	ctx := context.Background()
	var personID string
	if err := a.repo.Pool().QueryRow(ctx,
		`INSERT INTO vehicle_people (vehicle_id, name) VALUES ($1, 'Alex') RETURNING id`, a.vid).Scan(&personID); err != nil {
		t.Fatal(err)
	}
	out := decodeObj(t, a.do(a.ownerID, http.StatusOK, "PUT", "/"+a.vid, `{"name":"Model 3","default_driver_id":"`+personID+`"}`))
	if out["default_driver_id"] != personID {
		t.Fatalf("default driver not set: %v", out["default_driver_id"])
	}
}

func TestVehicleEstimatedEnergy(t *testing.T) {
	a := newVehicleAPI(t, "ee")
	path := "/" + a.vid + "/estimated-energy"
	a.do(a.ownerID, http.StatusBadRequest, "PUT", path, `{`)
	a.do(a.ownerID, http.StatusBadRequest, "PUT", path, `{"estimated_kwh_100km":0}`)
	a.do(a.ownerID, http.StatusBadRequest, "PUT", path, `{"estimated_kwh_100km":101}`)
	a.do(a.ownerID, http.StatusBadRequest, "PUT", path, `{"estimated_price_per_kwh":11}`)
	a.do(a.other, http.StatusNotFound, "PUT", path, `{"estimated_kwh_100km":15}`)
	a.do(a.viewer, http.StatusForbidden, "PUT", path, `{"estimated_kwh_100km":15}`)
	out := decodeObj(t, a.do(a.editor, http.StatusOK, "PUT", path, `{"estimated_kwh_100km":16.5,"estimated_price_per_kwh":0.25}`))
	if out["estimated_kwh_100km"].(float64) != 16.5 {
		t.Fatalf("unexpected vehicle: %v", out)
	}
}

func TestVehicleTeslaMateAndSync(t *testing.T) {
	a := newVehicleAPI(t, "tm")
	a.do(a.ownerID, http.StatusBadRequest, "POST", "/test-connection", `{`)
	// An unusable URL is reported as a bad request without any network access.
	a.do(a.ownerID, http.StatusBadRequest, "POST", "/test-connection", `{"teslamate_api_url":"","teslamate_auth_type":"NONE","teslamate_car_id":-3}`)
	a.do(a.ownerID, http.StatusBadRequest, "POST", "/test-connection",
		`{"teslamate_api_url":"","teslamate_auth_type":"API_KEY","teslamate_api_key":"k","teslamate_basic_user":"u","teslamate_basic_pass":"p","teslamate_car_id":2}`)

	tm := "/" + a.vid + "/teslamate/test"
	a.do(a.other, http.StatusNotFound, "POST", tm, "")
	a.do(a.editor, http.StatusForbidden, "POST", tm, "")
	a.do(a.ownerID, http.StatusBadRequest, "POST", tm, "")

	syncPath := "/" + a.vid + "/sync"
	a.do(a.other, http.StatusNotFound, "POST", syncPath, "")
	a.do(a.viewer, http.StatusForbidden, "POST", syncPath, "")
	a.do(a.other, http.StatusNotFound, "GET", syncPath, "")
	none := decodeObj(t, a.do(a.viewer, http.StatusOK, "GET", syncPath, ""))
	if none["status"] != "NONE" {
		t.Fatalf("no job yet: %v", none)
	}
	a.do(a.ownerID, http.StatusAccepted, "POST", syncPath, "")
	a.do(a.ownerID, http.StatusOK, "GET", syncPath, "")
}

func TestVehicleOdometerAndDataSources(t *testing.T) {
	a := newVehicleAPI(t, "od")
	at := "/" + a.vid + "/odometer-at"
	est := "/" + a.vid + "/odometer-estimate"

	a.do(a.other, http.StatusNotFound, "GET", at, "")
	a.do(a.other, http.StatusNotFound, "GET", est, "")
	a.do(a.ownerID, http.StatusBadRequest, "GET", at+"?date=garbage", "")
	a.do(a.ownerID, http.StatusBadRequest, "GET", est+"?date=garbage", "")
	a.do(a.ownerID, http.StatusBadRequest, "GET", est, "")

	// Only the odometer the vehicle was created with: it bounds any earlier date.
	early := decodeObj(t, a.do(a.ownerID, http.StatusOK, "GET", est+"?date=2020-01-01", ""))
	if early["source"] != "bounded" || early["odometer"].(float64) != 1000 {
		t.Fatalf("unexpected estimate: %v", early)
	}
	a.do(a.ownerID, http.StatusOK, "GET", at+"?date=2020-01-01", "")
	a.do(a.ownerID, http.StatusOK, "GET", at, "")
	a.do(a.ownerID, http.StatusOK, "GET", at+"?date=2020-01-01T10:30:00Z", "")

	ctx := context.Background()
	if _, err := a.repo.Pool().Exec(ctx,
		`INSERT INTO odometer_checkpoints (vehicle_id, date, odometer) VALUES ($1, '2024-01-01', 1000), ($1, '2024-03-01', 2000)`, a.vid); err != nil {
		t.Fatal(err)
	}
	got := decodeObj(t, a.do(a.ownerID, http.StatusOK, "GET", est+"?date=2024-02-01", ""))
	if got["odometer"] == nil {
		t.Fatalf("expected an interpolated odometer: %v", got)
	}
	a.do(a.ownerID, http.StatusOK, "GET", at+"?date=2024-02-01", "")

	ds := "/" + a.vid + "/data-sources"
	a.do(a.other, http.StatusNotFound, "GET", ds, "")
	if out := decodeObj(t, a.do(a.ownerID, http.StatusOK, "GET", ds, "")); out["teslamate_configured"] != false {
		t.Fatalf("not configured yet: %v", out)
	}
	a.do(a.ownerID, http.StatusOK, "PUT", "/"+a.vid, `{"name":"Model 3","teslamate_api_url":"http://tm:8080"}`)
	if out := decodeObj(t, a.do(a.ownerID, http.StatusOK, "GET", ds, "")); out["teslamate_configured"] != true {
		t.Fatalf("configured for the owner: %v", out)
	}
	if out := decodeObj(t, a.do(a.viewer, http.StatusOK, "GET", ds, "")); out["teslamate_configured"] != false {
		t.Fatalf("hidden from a viewer: %v", out)
	}
}
