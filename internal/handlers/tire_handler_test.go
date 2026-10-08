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

type tireAPI struct {
	t      *testing.T
	router chi.Router
	userID string
	vid    string
}

func newTireAPI(t *testing.T, email string) *tireAPI {
	t.Helper()
	repo := authTestRepo(t)
	ctx := context.Background()
	u, err := repo.CreateUser(ctx, email, "hash")
	if err != nil {
		t.Fatal(err)
	}
	v := &models.Vehicle{UserID: u.ID, Name: "Car", TeslaMateAuthType: models.AuthModeNone, CurrentOdometer: 30000}
	if err := repo.CreateVehicle(ctx, v); err != nil {
		t.Fatal(err)
	}
	h := NewTireHandler(repo, services.NewTireWearService(repo))
	r := chi.NewRouter()
	r.Get("/vehicles/{vehicleId}/tires", h.List)
	r.Post("/vehicles/{vehicleId}/tires", h.Create)
	r.Post("/vehicles/{vehicleId}/tires/batch", h.BatchCreate)
	r.Patch("/vehicles/{vehicleId}/tires/batch", h.BatchUpdate)
	r.Post("/vehicles/{vehicleId}/tires/batch-dispose", h.BatchDispose)
	r.Put("/vehicles/{vehicleId}/tires/{tireId}", h.Update)
	r.Delete("/vehicles/{vehicleId}/tires/{tireId}", h.Delete)
	r.Post("/vehicles/{vehicleId}/tires/{tireId}/dispose", h.Dispose)
	r.Post("/vehicles/{vehicleId}/tires/{tireId}/logs", h.AddLog)
	r.Put("/vehicles/{vehicleId}/tires/{tireId}/logs/{logId}", h.UpdateLog)
	r.Delete("/vehicles/{vehicleId}/tires/{tireId}/logs/{logId}", h.DeleteLog)
	return &tireAPI{t: t, router: r, userID: u.ID, vid: v.ID}
}

func (a *tireAPI) do(method, path, body string) (int, map[string]any, []byte) {
	a.t.Helper()
	req := httptest.NewRequest(method, "/vehicles/"+a.vid+path, bytes.NewBufferString(body))
	req = req.WithContext(context.WithValue(req.Context(), middleware.UserIDKey, a.userID))
	rec := httptest.NewRecorder()
	a.router.ServeHTTP(rec, req)
	var obj map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &obj)
	return rec.Code, obj, rec.Body.Bytes()
}

func (a *tireAPI) expect(want int, method, path, body string) map[string]any {
	a.t.Helper()
	code, obj, raw := a.do(method, path, body)
	if code != want {
		a.t.Fatalf("%s %s %s: got %d, want %d (%s)", method, path, body, code, want, raw)
	}
	return obj
}

const newTire = `{"brand":"Michelin","model":"Pilot","dimension":"235/45R18","purchase_date":"2026-01-10","purchase_price":120}`

func TestTireCreateAppliesDefaultsAndValidates(t *testing.T) {
	a := newTireAPI(t, "tire-create@example.com")

	tire := a.expect(http.StatusCreated, "POST", "/tires", newTire)
	if tire["current_position"] != "STORAGE" || tire["season"] != "SUMMER" || tire["initial_depth_mm"] != 8.0 ||
		tire["min_legal_depth_mm"] != 1.6 || tire["estimated_lifespan_km"] != 40000.0 {
		t.Errorf("defaults not applied: %v", tire)
	}

	a.expect(http.StatusBadRequest, "POST", "/tires", `{`)
	a.expect(http.StatusBadRequest, "POST", "/tires", `{"brand":"x","model":"y"}`)
	a.expect(http.StatusBadRequest, "POST", "/tires", `{"brand":"x","model":"y","dimension":"z","purchase_date":"not-a-date"}`)
	a.expect(http.StatusBadRequest, "POST", "/tires", `{"brand":"x","model":"y","dimension":"z","purchase_date":"2026-01-10","purchase_price":-5}`)
}

func TestTireListComputesWear(t *testing.T) {
	a := newTireAPI(t, "tire-list@example.com")
	tire := a.expect(http.StatusCreated, "POST", "/tires", newTire)
	id := tire["id"].(string)
	a.expect(http.StatusCreated, "POST", "/tires/"+id+"/logs", `{"depth_mm":6,"odometer":20000,"date":"2026-03-01"}`)

	_, _, raw := a.do("GET", "/tires", "")
	var stats []map[string]any
	if err := json.Unmarshal(raw, &stats); err != nil || len(stats) != 1 {
		t.Fatalf("list: %v %s", err, raw)
	}
	if stats[0]["current_depth_mm"] != 6.0 || stats[0]["logs_count"] != 1.0 {
		t.Errorf("unexpected wear stats: %v", stats[0])
	}
}

func TestTireUpdate(t *testing.T) {
	a := newTireAPI(t, "tire-update@example.com")
	id := a.expect(http.StatusCreated, "POST", "/tires", newTire)["id"].(string)

	got := a.expect(http.StatusOK, "PUT", "/tires/"+id,
		`{"brand":"Pirelli","model":"","season":"WINTER","purchase_date":"2026-02-01","purchase_price":99,"initial_depth_mm":9,"min_legal_depth_mm":2,"dot_code":"1224","initial_distance_km":500,"estimated_lifespan_km":50000}`)
	if got["brand"] != "Pirelli" || got["model"] != "Pilot" || got["season"] != "WINTER" || got["initial_depth_mm"] != 9.0 ||
		got["min_legal_depth_mm"] != 2.0 || got["dot_code"] != "1224" || got["initial_distance_km"] != 500.0 || got["estimated_lifespan_km"] != 50000.0 {
		t.Errorf("unexpected update result: %v", got)
	}
	// Zero or empty values are ignored rather than applied.
	got = a.expect(http.StatusOK, "PUT", "/tires/"+id, `{"initial_depth_mm":0,"estimated_lifespan_km":0,"season":""}`)
	if got["initial_depth_mm"] != 9.0 || got["estimated_lifespan_km"] != 50000.0 || got["season"] != "WINTER" {
		t.Errorf("ignored values changed the tire: %v", got)
	}

	a.expect(http.StatusNotFound, "PUT", "/tires/00000000-0000-0000-0000-000000000000", `{}`)
	a.expect(http.StatusBadRequest, "PUT", "/tires/"+id, `{`)
	a.expect(http.StatusBadRequest, "PUT", "/tires/"+id, `{"purchase_date":"nope"}`)
	a.expect(http.StatusBadRequest, "PUT", "/tires/"+id, `{"purchase_price":-1}`)
	a.expect(http.StatusBadRequest, "PUT", "/tires/"+id, `{"initial_distance_km":-1}`)
}

func TestTireBatchCreatePositionsAndPriceSplit(t *testing.T) {
	a := newTireAPI(t, "tire-batch@example.com")
	for _, tc := range []struct {
		typ       string
		count     int
		positions map[string]int
	}{
		{"", 4, map[string]int{"FL": 1, "FR": 1, "RL": 1, "RR": 1}},
		{"SET_2_FRONT", 2, map[string]int{"FL": 1, "FR": 1}},
		{"SET_2_REAR", 2, map[string]int{"RL": 1, "RR": 1}},
		{"SET_4_STORAGE", 4, map[string]int{"STORAGE": 4}},
		{"SET_2_STORAGE", 2, map[string]int{"STORAGE": 2}},
	} {
		body := `{"type":"` + tc.typ + `","brand":"B","model":"M","dimension":"D","purchase_date":"2026-01-01","total_price":100,"mounted_odometer":1000}`
		resp := a.expect(http.StatusCreated, "POST", "/tires/batch", body)
		tires := resp["tires"].([]any)
		if int(resp["count"].(float64)) != tc.count || len(tires) != tc.count {
			t.Fatalf("%s: wrong count %v", tc.typ, resp["count"])
		}
		positions := map[string]int{}
		var total float64
		var created []string
		for _, raw := range tires {
			tire := raw.(map[string]any)
			positions[tire["current_position"].(string)]++
			created = append(created, tire["id"].(string))
			total += tire["purchase_price"].(float64)
			if _, mounted := tire["mounted_odometer"]; mounted == (tire["current_position"] == "STORAGE") {
				t.Errorf("%s: mounted odometer should be set only on fitted tires: %v", tc.typ, tire)
			}
		}
		for pos, n := range tc.positions {
			if positions[pos] != n {
				t.Errorf("%s: positions %v, want %v", tc.typ, positions, tc.positions)
			}
		}
		if total != 100 {
			t.Errorf("%s: split total %v, want 100", tc.typ, total)
		}
		ids, _ := json.Marshal(created)
		a.expect(http.StatusOK, "POST", "/tires/batch-dispose", `{"tire_ids":`+string(ids)+`,"odometer":2000}`)
	}

	unit := a.expect(http.StatusCreated, "POST", "/tires/batch", `{"brand":"B","model":"M","dimension":"D","purchase_date":"2026-01-01","unit_price":30,"mounted_odometer":1000}`)
	for _, raw := range unit["tires"].([]any) {
		if raw.(map[string]any)["purchase_price"] != 30.0 {
			t.Errorf("unit price not applied: %v", raw)
		}
	}

	a.expect(http.StatusBadRequest, "POST", "/tires/batch", `{`)
	a.expect(http.StatusBadRequest, "POST", "/tires/batch", `{"brand":"B"}`)
	a.expect(http.StatusBadRequest, "POST", "/tires/batch", `{"brand":"B","model":"M","dimension":"D","purchase_date":"x"}`)
	a.expect(http.StatusBadRequest, "POST", "/tires/batch", `{"brand":"B","model":"M","dimension":"D","purchase_date":"2026-01-01","total_price":-1}`)
	a.expect(http.StatusBadRequest, "POST", "/tires/batch", `{"brand":"B","model":"M","dimension":"D","purchase_date":"2026-01-01","unit_price":-1}`)
}

func TestTireBatchUpdateValidation(t *testing.T) {
	a := newTireAPI(t, "tire-batch-update@example.com")
	resp := a.expect(http.StatusCreated, "POST", "/tires/batch", `{"brand":"B","model":"M","dimension":"D","purchase_date":"2026-01-01","unit_price":10,"mounted_odometer":1000}`)
	var ids []string
	for _, raw := range resp["tires"].([]any) {
		ids = append(ids, raw.(map[string]any)["id"].(string))
	}
	idList, _ := json.Marshal(ids)

	a.expect(http.StatusOK, "PATCH", "/tires/batch", `{"tire_ids":`+string(idList)+`,"brand":" Goodyear ","season":"WINTER","purchase_price":25,"initial_depth_mm":9,"estimated_lifespan_km":45000}`)
	_, _, raw := a.do("GET", "/tires", "")
	var stats []struct {
		Tire struct {
			Brand string `json:"brand"`
		} `json:"tire"`
	}
	_ = json.Unmarshal(raw, &stats)
	for _, s := range stats {
		if s.Tire.Brand == "" {
			t.Errorf("brand lost after batch update: %s", raw)
		}
	}

	for name, body := range map[string]string{
		"bad json":       `{`,
		"bad season":     `{"tire_ids":["x"],"season":"MONSOON"}`,
		"bad date":       `{"tire_ids":["x"],"purchase_date":"nope"}`,
		"bad mount date": `{"tire_ids":["x"],"mounted_date":"nope"}`,
		"negative price": `{"tire_ids":["x"],"purchase_price":-1}`,
		"negative total": `{"tire_ids":["x"],"total_price":-1}`,
		"deep tread":     `{"tire_ids":["x"],"initial_depth_mm":50}`,
		"deep minimum":   `{"tire_ids":["x"],"min_legal_depth_mm":50}`,
		"far initial":    `{"tire_ids":["x"],"initial_distance_km":900000}`,
		"far odometer":   `{"tire_ids":["x"],"mounted_odometer":9000000}`,
		"zero lifespan":  `{"tire_ids":["x"],"estimated_lifespan_km":0}`,
		"huge lifespan":  `{"tire_ids":["x"],"estimated_lifespan_km":900000}`,
	} {
		if code, _, _ := a.do("PATCH", "/tires/batch", body); code != http.StatusBadRequest {
			t.Errorf("%s: got %d, want 400", name, code)
		}
	}
}

func TestTireDisposeAndDelete(t *testing.T) {
	a := newTireAPI(t, "tire-dispose@example.com")
	one := a.expect(http.StatusCreated, "POST", "/tires", newTire)["id"].(string)
	two := a.expect(http.StatusCreated, "POST", "/tires", newTire)["id"].(string)
	three := a.expect(http.StatusCreated, "POST", "/tires", newTire)["id"].(string)

	a.expect(http.StatusOK, "POST", "/tires/"+one+"/dispose", `{"date":"2026-05-01","odometer":31000}`)
	a.expect(http.StatusOK, "POST", "/tires/"+two+"/dispose", `{}`)
	a.expect(http.StatusBadRequest, "POST", "/tires/"+three+"/dispose", `{`)
	a.expect(http.StatusBadRequest, "POST", "/tires/"+three+"/dispose", `{"date":"nope"}`)
	a.expect(http.StatusBadRequest, "POST", "/tires/"+three+"/dispose", `{"odometer":-5}`)

	a.expect(http.StatusBadRequest, "POST", "/tires/batch-dispose", `{`)
	a.expect(http.StatusBadRequest, "POST", "/tires/batch-dispose", `{"tire_ids":[]}`)
	a.expect(http.StatusBadRequest, "POST", "/tires/batch-dispose", `{"tire_ids":["`+three+`"],"date":"nope"}`)
	a.expect(http.StatusOK, "POST", "/tires/batch-dispose", `{"tire_ids":["`+three+`"],"date":"2026-06-01"}`)

	a.expect(http.StatusOK, "DELETE", "/tires/"+one, "")
	if code, _, _ := a.do("DELETE", "/tires/"+one, ""); code != http.StatusNotFound {
		t.Errorf("deleting twice: got %d, want 404", code)
	}
}

func TestTireLogs(t *testing.T) {
	a := newTireAPI(t, "tire-logs@example.com")
	id := a.expect(http.StatusCreated, "POST", "/tires", newTire)["id"].(string)

	log := a.expect(http.StatusCreated, "POST", "/tires/"+id+"/logs", `{"depth_mm":7,"odometer":1000,"notes":"first"}`)
	logID := log["id"].(string)
	a.expect(http.StatusCreated, "POST", "/tires/"+id+"/logs", `{"depth_mm":6,"odometer":5000,"date":"2026-04-01"}`)

	a.expect(http.StatusNotFound, "POST", "/tires/00000000-0000-0000-0000-000000000000/logs", `{"depth_mm":7,"odometer":1}`)
	a.expect(http.StatusBadRequest, "POST", "/tires/"+id+"/logs", `{`)
	a.expect(http.StatusBadRequest, "POST", "/tires/"+id+"/logs", `{"depth_mm":0,"odometer":10}`)
	a.expect(http.StatusBadRequest, "POST", "/tires/"+id+"/logs", `{"depth_mm":25,"odometer":10}`)
	a.expect(http.StatusBadRequest, "POST", "/tires/"+id+"/logs", `{"depth_mm":5}`)
	a.expect(http.StatusBadRequest, "POST", "/tires/"+id+"/logs", `{"depth_mm":5,"odometer":10,"date":"nope"}`)

	updated := a.expect(http.StatusOK, "PUT", "/tires/"+id+"/logs/"+logID, `{"depth_mm":6.5,"odometer":1200,"date":"2026-02-01","notes":"fixed"}`)
	if updated["depth_mm"] != 6.5 || updated["odometer"] != 1200.0 {
		t.Errorf("unexpected update: %v", updated)
	}
	a.expect(http.StatusBadRequest, "PUT", "/tires/"+id+"/logs/"+logID, `{`)
	a.expect(http.StatusBadRequest, "PUT", "/tires/"+id+"/logs/"+logID, `{"depth_mm":0,"odometer":1,"date":"2026-02-01"}`)
	a.expect(http.StatusBadRequest, "PUT", "/tires/"+id+"/logs/"+logID, `{"depth_mm":5,"odometer":0,"date":"2026-02-01"}`)
	a.expect(http.StatusBadRequest, "PUT", "/tires/"+id+"/logs/"+logID, `{"depth_mm":5,"odometer":10,"date":"nope"}`)
	a.expect(http.StatusNotFound, "PUT", "/tires/"+id+"/logs/00000000-0000-0000-0000-000000000000", `{"depth_mm":5,"odometer":10,"date":"2026-02-01"}`)

	a.expect(http.StatusOK, "DELETE", "/tires/"+id+"/logs/"+logID, "")
	a.expect(http.StatusNotFound, "DELETE", "/tires/"+id+"/logs/"+logID, "")
}

func TestTireRoutesRefuseAnotherUsersVehicle(t *testing.T) {
	a := newTireAPI(t, "tire-owner@example.com")
	id := a.expect(http.StatusCreated, "POST", "/tires", newTire)["id"].(string)
	a.userID = "00000000-0000-0000-0000-000000000099"
	for _, c := range []struct{ method, path, body string }{
		{"GET", "/tires", ""},
		{"POST", "/tires", newTire},
		{"PUT", "/tires/" + id, `{}`},
		{"DELETE", "/tires/" + id, ""},
		{"POST", "/tires/" + id + "/logs", `{"depth_mm":5,"odometer":10}`},
	} {
		if code, _, _ := a.do(c.method, c.path, c.body); code != http.StatusNotFound && code != http.StatusForbidden {
			t.Errorf("%s %s by a stranger: got %d, want 403/404", c.method, c.path, code)
		}
	}
}
