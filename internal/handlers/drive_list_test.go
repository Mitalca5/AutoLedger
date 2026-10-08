package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/teslacost/teslacost/internal/database"
	"github.com/teslacost/teslacost/internal/middleware"
	"github.com/teslacost/teslacost/internal/models"
	"github.com/teslacost/teslacost/internal/services"
)

type driveListResponse struct {
	Drives []struct {
		ID    string             `json:"id"`
		Tags  []string           `json:"tags"`
		Costs DriveCostBreakdown `json:"costs"`
	} `json:"drives"`
	Total            int `json:"total"`
	Page             int `json:"page"`
	Limit            int `json:"limit"`
	UnqualifiedCount int `json:"unqualified_count"`
}

type driveAPI struct {
	t      *testing.T
	router chi.Router
	repo   *database.Repository
	owner  string
	viewer string
	vid    string
}

func newDriveAPI(t *testing.T, tag string) *driveAPI {
	t.Helper()
	repo := authTestRepo(t)
	ctx := context.Background()
	mk := func(name string) *models.User {
		u, err := repo.CreateUser(ctx, name+"-"+tag+"@example.com", "hash")
		if err != nil {
			t.Fatal(err)
		}
		return u
	}
	owner, viewer := mk("owner"), mk("viewer")
	kwh100 := 20.0
	v := &models.Vehicle{UserID: owner.ID, Name: "Drive car", Powertrain: models.PowertrainEV, TeslaMateAuthType: models.AuthModeNone, EstimatedKwh100km: &kwh100}
	if err := repo.CreateVehicle(ctx, v); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.AddVehicleMember(ctx, v.ID, viewer.Email, models.RoleViewer, nil); err != nil {
		t.Fatal(err)
	}
	h := NewDriveHandler(repo, services.NewCarpoolService(repo.Pool(), repo), nil)
	r := chi.NewRouter()
	r.Get("/{vehicleId}/drives", h.List)
	r.Post("/{vehicleId}/drives", h.Create)
	r.Get("/{vehicleId}/drives/{driveId}/expenses", h.GetDriveExpenses)
	r.Put("/{vehicleId}/drives/{driveId}/tags", h.UpdateTags)
	return &driveAPI{t: t, router: r, repo: repo, owner: owner.ID, viewer: viewer.ID, vid: v.ID}
}

func (a *driveAPI) do(userID, method, path string, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, "/"+a.vid+path, bytes.NewBufferString(body))
	req = req.WithContext(context.WithValue(req.Context(), middleware.UserIDKey, userID))
	rec := httptest.NewRecorder()
	a.router.ServeHTTP(rec, req)
	return rec
}

func (a *driveAPI) list(userID, query string) driveListResponse {
	a.t.Helper()
	rec := a.do(userID, http.MethodGet, "/drives"+query, "")
	if rec.Code != http.StatusOK {
		a.t.Fatalf("list %s: %d %s", query, rec.Code, rec.Body.String())
	}
	var out driveListResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		a.t.Fatal(err)
	}
	return out
}

func TestDriveListPaginationFiltersAndCosts(t *testing.T) {
	a := newDriveAPI(t, "list")
	for _, body := range []string{
		`{"start_time":"2026-09-01T08:00:00Z","distance_km":50}`,
		`{"start_time":"2026-09-02T08:00:00Z","distance_km":20}`,
		`{"start_time":"2026-09-03T08:00:00Z","distance_km":10}`,
	} {
		if rec := a.do(a.owner, http.MethodPost, "/drives", body); rec.Code != http.StatusCreated {
			t.Fatalf("create: %d %s", rec.Code, rec.Body.String())
		}
	}

	all := a.list(a.viewer, "")
	if all.Total != 3 || len(all.Drives) != 3 || all.Page != 1 || all.Limit != 50 {
		t.Fatalf("defaults: %+v", all)
	}
	// The newest drive comes first; the cost breakdown is computed from the vehicle's estimates.
	first := all.Drives[0]
	if first.Costs.CostPerKm <= 0 || first.Costs.TotalCost <= 0 || first.Costs.ElectricityKwh != 2 || !first.Costs.HasEstimates {
		t.Fatalf("costs of the newest drive (10 km at 20 kWh/100 km): %+v", first.Costs)
	}

	if got := a.list(a.owner, "?limit=2&page=2"); got.Total != 3 || len(got.Drives) != 1 || got.Page != 2 || got.Limit != 2 {
		t.Fatalf("second page: %+v", got)
	}
	if got := a.list(a.owner, "?limit=999&page=-3"); got.Limit != 50 || got.Page != 1 {
		t.Fatalf("out of range paging falls back to the defaults: %+v", got)
	}
	if got := a.list(a.owner, "?from=2026-09-02&to=2026-09-02"); got.Total != 1 {
		t.Fatalf("one-day filter: %+v", got)
	}

	// Tags: set on one drive, the filter finds exactly that one.
	target := all.Drives[1].ID
	rec := a.do(a.owner, http.MethodPut, "/drives/"+target+"/tags", `{"tags":["Pro"]}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("tags: %d %s", rec.Code, rec.Body.String())
	}
	if got := a.list(a.owner, "?tag=Pro"); got.Total != 1 || got.Drives[0].ID != target {
		t.Fatalf("tag filter: %+v", got)
	}
	rec = a.do(a.owner, http.MethodPut, "/drives/"+target+"/tags", `{}`)
	var cleared struct {
		Tags []string `json:"tags"`
	}
	if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &cleared) != nil || cleared.Tags == nil || len(cleared.Tags) != 0 {
		t.Fatalf("missing tags must clear to an empty list: %d %s", rec.Code, rec.Body.String())
	}

	if rec := a.do(a.owner, http.MethodGet, "/drives/"+target+"/expenses", ""); rec.Code != http.StatusOK || rec.Body.String() == "null\n" {
		t.Fatalf("expenses: %d %s", rec.Code, rec.Body.String())
	}
}

func TestDriveTagsAndExpensesAccess(t *testing.T) {
	a := newDriveAPI(t, "access")
	rec := a.do(a.owner, http.MethodPost, "/drives", `{"start_time":"2026-09-01T08:00:00Z","distance_km":50}`)
	var d models.Drive
	_ = json.Unmarshal(rec.Body.Bytes(), &d)

	if rec := a.do(a.owner, http.MethodPut, "/drives/"+d.ID+"/tags", `{`); rec.Code != http.StatusBadRequest || responseCode(rec) != "request.invalid_body" {
		t.Fatalf("invalid body: %d %s", rec.Code, rec.Body.String())
	}
	if rec := a.do(a.viewer, http.MethodPut, "/drives/"+d.ID+"/tags", `{"tags":["x"]}`); rec.Code != http.StatusForbidden {
		t.Fatalf("a viewer cannot tag: %d", rec.Code)
	}
	if rec := a.do(a.owner, http.MethodPut, "/drives/00000000-0000-0000-0000-000000000000/tags", `{"tags":["x"]}`); rec.Code != http.StatusNotFound {
		t.Fatalf("unknown drive: %d", rec.Code)
	}

	rec = a.do(a.viewer, http.MethodGet, "/drives/"+d.ID+"/expenses", "")
	var expenses []models.DriveExpense
	if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &expenses) != nil || expenses == nil || len(expenses) != 0 {
		t.Fatalf("a drive without expense gives an empty list: %d %s", rec.Code, rec.Body.String())
	}

	stranger, err := a.repo.CreateUser(context.Background(), "stranger-drives@example.com", "hash")
	if err != nil {
		t.Fatal(err)
	}
	if rec := a.do(stranger.ID, http.MethodGet, "/drives", ""); rec.Code == http.StatusOK {
		t.Fatal("a stranger must not list the drives")
	}
	if rec := a.do(stranger.ID, http.MethodGet, "/drives/"+d.ID+"/expenses", ""); rec.Code == http.StatusOK {
		t.Fatal("a stranger must not read the expenses")
	}
}
