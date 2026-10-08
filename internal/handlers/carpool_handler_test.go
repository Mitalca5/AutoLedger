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

type carpoolAPI struct {
	t        *testing.T
	router   chi.Router
	userID   string
	viewerID string
	vid      string
}

func newCarpoolAPI(t *testing.T, tag string) *carpoolAPI {
	t.Helper()
	repo := authTestRepo(t)
	ctx := context.Background()
	owner, err := repo.CreateUser(ctx, "carpool-"+tag+"@example.com", "hash")
	if err != nil {
		t.Fatal(err)
	}
	viewer, err := repo.CreateUser(ctx, "carpool-viewer-"+tag+"@example.com", "hash")
	if err != nil {
		t.Fatal(err)
	}
	v := &models.Vehicle{UserID: owner.ID, Name: "Car", TeslaMateAuthType: models.AuthModeNone, CurrentOdometer: 1000}
	if err := repo.CreateVehicle(ctx, v); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.AddVehicleMember(ctx, v.ID, viewer.Email, models.RoleViewer, nil); err != nil {
		t.Fatal(err)
	}
	h := NewCarpoolHandler(repo, services.NewCarpoolService(repo.Pool(), repo))
	r := chi.NewRouter()
	r.Get("/vehicles/{vehicleId}/carpools", h.List)
	r.Post("/vehicles/{vehicleId}/carpools", h.Create)
	r.Post("/vehicles/{vehicleId}/carpools/recalculate", h.Recalculate)
	r.Get("/vehicles/{vehicleId}/carpools/estimate", h.Estimate)
	r.Get("/vehicles/{vehicleId}/carpools/{id}", h.Get)
	r.Put("/vehicles/{vehicleId}/carpools/{id}", h.Update)
	r.Delete("/vehicles/{vehicleId}/carpools/{id}", h.Delete)
	return &carpoolAPI{t: t, router: r, userID: owner.ID, viewerID: viewer.ID, vid: v.ID}
}

func (a *carpoolAPI) as(userID string, want int, method, path, body string) map[string]any {
	a.t.Helper()
	req := httptest.NewRequest(method, "/vehicles/"+a.vid+path, bytes.NewBufferString(body))
	req = req.WithContext(context.WithValue(req.Context(), middleware.UserIDKey, userID))
	rec := httptest.NewRecorder()
	a.router.ServeHTTP(rec, req)
	if rec.Code != want {
		a.t.Fatalf("%s %s %s: got %d, want %d (%s)", method, path, body, rec.Code, want, rec.Body.String())
	}
	var obj map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &obj)
	return obj
}

func (a *carpoolAPI) expect(want int, method, path, body string) map[string]any {
	a.t.Helper()
	return a.as(a.userID, want, method, path, body)
}

const twoLegCarpool = `{"title":"Weekend","date":"2026-03-14","legs":[` +
	`{"start_label":"Paris","end_label":"Lyon","distance_km":460,"electricity_cost":20,"tolls_cost":30},` +
	`{"start_label":"Lyon","end_label":"Nice","distance_km":470,"electricity_cost":20}],` +
	`"passengers":[{"passenger_name":"Zoe","amount_paid":10},` +
	`{"passenger_name":"Max","board_stop_index":1,"alight_stop_index":2,"seats":2}]}`

func TestCarpoolValidation(t *testing.T) {
	a := newCarpoolAPI(t, "validation")
	legs31 := `{"distance_km":1}`
	for i := 0; i < 30; i++ {
		legs31 += `,{"distance_km":1}`
	}
	cases := map[string]string{
		"bad json":            `{`,
		"title required":      `{"title":"  "}`,
		"bad date":            `{"title":"T","date":"not-a-date"}`,
		"too many legs":       `{"title":"T","legs":[` + legs31 + `]}`,
		"negative distance":   `{"title":"T","legs":[{"distance_km":-1}]}`,
		"huge distance":       `{"title":"T","legs":[{"distance_km":99999}]}`,
		"negative leg cost":   `{"title":"T","legs":[{"distance_km":1,"tolls_cost":-5}]}`,
		"negative paid":       `{"title":"T","distance_km":1,"passengers":[{"passenger_name":"A","amount_paid":-1}]}`,
		"alight before board": `{"title":"T","distance_km":1,"passengers":[{"passenger_name":"A","board_stop_index":1,"alight_stop_index":1}]}`,
		"alight beyond legs":  `{"title":"T","distance_km":1,"passengers":[{"passenger_name":"A","alight_stop_index":5}]}`,
		"negative board":      `{"title":"T","distance_km":1,"passengers":[{"passenger_name":"A","board_stop_index":-1}]}`,
		"too many seats":      `{"title":"T","distance_km":1,"passengers":[{"passenger_name":"A","seats":7},{"passenger_name":"B"}]}`,
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) { a.expect(http.StatusBadRequest, "POST", "/carpools", body) })
	}
}

func TestCarpoolLifecycle(t *testing.T) {
	a := newCarpoolAPI(t, "lifecycle")

	trip := a.expect(http.StatusCreated, "POST", "/carpools", twoLegCarpool)
	id := trip["id"].(string)
	if len(trip["legs"].([]any)) != 2 || len(trip["passengers"].([]any)) != 2 {
		t.Fatalf("legs/passengers: %v", trip)
	}
	passengers := trip["passengers"].([]any)
	zoe := passengers[0].(map[string]any)
	if zoe["origin"] != "Paris" || zoe["destination"] != "Nice" || zoe["seats"].(float64) != 1 {
		t.Fatalf("passenger defaults should come from the stop labels: %v", zoe)
	}
	max := passengers[1].(map[string]any)
	if max["origin"] != "Lyon" || max["destination"] != "Nice" {
		t.Fatalf("second passenger stops: %v", max)
	}
	if trip["driver_cost_share"] == nil || trip["passengers_cost_share"] == nil {
		t.Fatalf("cost shares missing: %v", trip)
	}

	single := a.expect(http.StatusCreated, "POST", "/carpools",
		`{"title":"  Solo  ","distance_km":50,"electricity_cost":5,"passengers":[{"passenger_name":"  ","origin":"A","destination":"B"}]}`)
	if single["title"] != "Solo" || len(single["legs"].([]any)) != 1 {
		t.Fatalf("single leg trip: %v", single)
	}
	if p := single["passengers"].([]any)[0].(map[string]any); p["passenger_name"] != "Passager" {
		t.Fatalf("blank passenger name should default: %v", p)
	}

	list := a.as(a.viewerID, http.StatusOK, "GET", "/carpools", "")
	if len(list["trips"].([]any)) != 2 || list["summary"] == nil {
		t.Fatalf("list: %v", list)
	}
	got := a.as(a.viewerID, http.StatusOK, "GET", "/carpools/"+id, "")
	if got["title"] != "Weekend" {
		t.Fatalf("get: %v", got)
	}
	a.expect(http.StatusNotFound, "GET", "/carpools/00000000-0000-0000-0000-000000000000", "")

	upd := a.expect(http.StatusOK, "PUT", "/carpools/"+id,
		`{"title":"Weekend 2","date":"2026-03-14","distance_km":100,"passengers":[]}`)
	if upd["title"] != "Weekend 2" || len(upd["legs"].([]any)) != 1 {
		t.Fatalf("update: %v", upd)
	}
	a.expect(http.StatusBadRequest, "PUT", "/carpools/"+id, `{"title":""}`)
	a.expect(http.StatusNotFound, "PUT", "/carpools/00000000-0000-0000-0000-000000000000", `{"title":"Ghost","distance_km":1}`)

	a.expect(http.StatusOK, "DELETE", "/carpools/"+id, "")
	a.expect(http.StatusNotFound, "GET", "/carpools/"+id, "")
	a.expect(http.StatusNotFound, "DELETE", "/carpools/"+id, "")
}

func TestCarpoolAccessControl(t *testing.T) {
	a := newCarpoolAPI(t, "access")

	a.as(a.viewerID, http.StatusForbidden, "POST", "/carpools", twoLegCarpool)
	a.as(a.viewerID, http.StatusForbidden, "DELETE", "/carpools/00000000-0000-0000-0000-000000000000", "")
	a.as(a.viewerID, http.StatusForbidden, "POST", "/carpools/recalculate", "")

	stranger := newCarpoolAPI(t, "stranger")
	a.as(stranger.userID, http.StatusNotFound, "GET", "/carpools", "")
	a.as(stranger.userID, http.StatusNotFound, "GET", "/carpools/estimate", "")
}

func TestCarpoolEstimateAndRecalculate(t *testing.T) {
	a := newCarpoolAPI(t, "estimate")

	est := a.expect(http.StatusOK, "GET", "/carpools/estimate?distance_km=120", "")
	if est == nil {
		t.Fatal("empty estimate")
	}
	a.expect(http.StatusNotFound, "GET", "/carpools/estimate?drive_id=00000000-0000-0000-0000-000000000000", "")
	a.expect(http.StatusNotFound, "GET", "/carpools/estimate?drive_ids=00000000-0000-0000-0000-000000000000", "")

	trip := a.expect(http.StatusCreated, "POST", "/carpools", twoLegCarpool)
	none := a.expect(http.StatusOK, "POST", "/carpools/recalculate", "")
	if none["updated_count"] == nil {
		t.Fatalf("recalculate all: %v", none)
	}
	some := a.expect(http.StatusOK, "POST", "/carpools/recalculate", `{"trip_ids":["`+trip["id"].(string)+`"]}`)
	if some["trips"] == nil {
		t.Fatalf("recalculate ids: %v", some)
	}
	a.expect(http.StatusBadRequest, "POST", "/carpools/recalculate", `{`)
}
