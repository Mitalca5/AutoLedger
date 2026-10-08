package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/teslacost/teslacost/internal/database"
	"github.com/teslacost/teslacost/internal/middleware"
	"github.com/teslacost/teslacost/internal/models"
	"github.com/teslacost/teslacost/internal/services"
)

type importAPI struct {
	t      *testing.T
	router chi.Router
	repo   *database.Repository
	owner  string
	viewer string
	vid    string
}

func newImportAPI(t *testing.T, tag string) *importAPI {
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
	v := &models.Vehicle{UserID: owner.ID, Name: "Import car", Powertrain: models.PowertrainEV}
	if err := repo.CreateVehicle(ctx, v); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.AddVehicleMember(ctx, v.ID, viewer.Email, models.RoleViewer, nil); err != nil {
		t.Fatal(err)
	}
	h := NewImportHandler(repo, services.NewCSVImportService(repo, "Europe/Paris"))
	r := chi.NewRouter()
	r.Post("/{vehicleId}/import/preview", h.Preview)
	r.Post("/{vehicleId}/import", h.Execute)
	r.Get("/{vehicleId}/import/batches", h.ListBatches)
	r.Delete("/{vehicleId}/import/batches/{batchId}", h.UndoBatch)
	return &importAPI{t: t, router: r, repo: repo, owner: owner.ID, viewer: viewer.ID, vid: v.ID}
}

func (a *importAPI) serve(userID, method, path string, body []byte, contentType string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, bytes.NewReader(body))
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	req = req.WithContext(context.WithValue(req.Context(), middleware.UserIDKey, userID))
	rec := httptest.NewRecorder()
	a.router.ServeHTTP(rec, req)
	return rec
}

func (a *importAPI) multipart(userID, path, csv string, fields map[string]string) *httptest.ResponseRecorder {
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	for k, v := range fields {
		_ = mw.WriteField(k, v)
	}
	if csv != "" {
		fw, _ := mw.CreateFormFile("file", "data.csv")
		_, _ = fw.Write([]byte(csv))
	}
	_ = mw.Close()
	return a.serve(userID, http.MethodPost, path, buf.Bytes(), mw.FormDataContentType())
}

func responseCode(rec *httptest.ResponseRecorder) string {
	var body struct {
		Code string `json:"code"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	return body.Code
}

const chargesCSV = "date,kwh,cost\n2026-09-15 10:00,10,3\n2026-09-16 10:00,12,4\n"

func TestImportRequestValidation(t *testing.T) {
	a := newImportAPI(t, "validation")
	preview := "/" + a.vid + "/import/preview"

	if rec := a.serve(a.viewer, http.MethodPost, preview+"?type=CHARGES", []byte(chargesCSV), ""); rec.Code != http.StatusForbidden {
		t.Fatalf("a viewer cannot import: %d", rec.Code)
	}
	if rec := a.serve(a.owner, http.MethodPost, preview, nil, ""); rec.Code != http.StatusBadRequest || responseCode(rec) != "import.empty" {
		t.Fatalf("empty body: %d %s", rec.Code, rec.Body.String())
	}
	if rec := a.serve(a.owner, http.MethodPost, preview, []byte("x"), "multipart/form-data; boundary=nope"); rec.Code != http.StatusBadRequest || responseCode(rec) != "import.too_large" {
		t.Fatalf("broken multipart: %d %s", rec.Code, rec.Body.String())
	}
	if rec := a.multipart(a.owner, preview, "", map[string]string{"type": "CHARGES"}); rec.Code != http.StatusBadRequest || responseCode(rec) != "import.missing_file" {
		t.Fatalf("missing file field: %d %s", rec.Code, rec.Body.String())
	}
	if rec := a.multipart(a.owner, preview, chargesCSV, map[string]string{"mapping": "{not json"}); rec.Code != http.StatusBadRequest || responseCode(rec) != "import.invalid_mapping" {
		t.Fatalf("invalid mapping: %d %s", rec.Code, rec.Body.String())
	}
	if rec := a.serve(a.owner, http.MethodPost, preview+"?type=NOPE", []byte(chargesCSV), ""); rec.Code != http.StatusBadRequest {
		t.Fatalf("unknown type: %d %s", rec.Code, rec.Body.String())
	}

	execute := "/" + a.vid + "/import"
	if rec := a.multipart(a.owner, execute, chargesCSV, map[string]string{"mapping": "{not json"}); rec.Code != http.StatusBadRequest || responseCode(rec) != "import.invalid_mapping" {
		t.Fatalf("execute with an invalid mapping: %d %s", rec.Code, rec.Body.String())
	}
	if rec := a.serve(a.owner, http.MethodPost, execute+"?type=NOPE", []byte(chargesCSV), ""); rec.Code != http.StatusBadRequest {
		t.Fatalf("execute with an unknown type: %d %s", rec.Code, rec.Body.String())
	}
}

func TestImportPreviewExecuteListAndUndo(t *testing.T) {
	a := newImportAPI(t, "flow")
	base := "/" + a.vid + "/import"

	var preview services.CSVPreviewResult
	rec := a.serve(a.owner, http.MethodPost, base+"/preview?type=CHARGES", []byte(chargesCSV), "text/csv")
	if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &preview) != nil || preview.ValidRows != 2 || preview.Type != services.ImportTypeCharges {
		t.Fatalf("raw preview: %d %s", rec.Code, rec.Body.String())
	}

	// The multipart form carries the options: type, decimal separator, date order and a column mapping.
	semicolon := "Date;Energie;Montant\n15/09/2026 10:00;42,5;18,5\n"
	rec = a.multipart(a.owner, base+"/preview", semicolon, map[string]string{
		"type": "CHARGES", "decimal_separator": ",", "date_order": "dmy", "skip_duplicates": "false",
		"mapping": `{"1":"kwh","2":"cost"}`,
	})
	preview = services.CSVPreviewResult{}
	if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &preview) != nil || preview.ValidRows != 1 {
		t.Fatalf("multipart preview: %d %s", rec.Code, rec.Body.String())
	}

	var result services.CSVExecuteResult
	rec = a.serve(a.owner, http.MethodPost, base+"?type=CHARGES", []byte(chargesCSV), "text/csv")
	if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &result) != nil || !result.Committed || result.ImportedCount != 2 || result.BatchID == "" {
		t.Fatalf("execute: %d %s", rec.Code, rec.Body.String())
	}

	// skip_duplicates=false stores the same rows again; the default skips them.
	rec = a.serve(a.owner, http.MethodPost, base+"?type=CHARGES", []byte(chargesCSV), "text/csv")
	result = services.CSVExecuteResult{}
	_ = json.Unmarshal(rec.Body.Bytes(), &result)
	if rec.Code != http.StatusOK || result.ImportedCount != 0 || result.SkippedCount != 2 {
		t.Fatalf("duplicates must be skipped: %d %+v", rec.Code, result)
	}

	// The all-duplicates run is recorded as an empty batch, newest first.
	rec = a.serve(a.viewer, http.MethodGet, base+"/batches", nil, "")
	var batches []models.ImportBatch
	if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &batches) != nil || len(batches) != 2 || batches[0].RowCount != 0 || batches[1].Remaining != 2 {
		t.Fatalf("batches: %d %s", rec.Code, rec.Body.String())
	}
	if rec := a.serve(a.outsider(), http.MethodGet, base+"/batches", nil, ""); rec.Code == http.StatusOK {
		t.Fatal("a stranger must not list the batches")
	}

	if rec := a.serve(a.viewer, http.MethodDelete, base+"/batches/"+batches[1].ID, nil, ""); rec.Code != http.StatusForbidden {
		t.Fatalf("a viewer cannot undo: %d", rec.Code)
	}
	rec = a.serve(a.owner, http.MethodDelete, base+"/batches/"+batches[1].ID, nil, "")
	var undone map[string]int
	if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &undone) != nil || undone["removed"] != 2 {
		t.Fatalf("undo: %d %s", rec.Code, rec.Body.String())
	}
	if rec := a.serve(a.owner, http.MethodDelete, base+"/batches/"+batches[1].ID, nil, ""); rec.Code != http.StatusNotFound || responseCode(rec) != "import.batch_not_found" {
		t.Fatalf("second undo: %d %s", rec.Code, rec.Body.String())
	}
}

func (a *importAPI) outsider() string {
	u, err := a.repo.CreateUser(context.Background(), "outsider-"+strings.ReplaceAll(a.vid, "-", "")[:8]+"@example.com", "hash")
	if err != nil {
		a.t.Fatal(err)
	}
	return u.ID
}

func TestImportReadsDistancesInTheAccountUnit(t *testing.T) {
	a := newImportAPI(t, "miles")
	if _, err := a.repo.Pool().Exec(context.Background(), `UPDATE users SET distance_unit = 'mi' WHERE id = $1`, a.owner); err != nil {
		t.Fatal(err)
	}
	csv := "date,distance\n2026-09-15 08:00,10\n"
	rec := a.serve(a.owner, http.MethodPost, "/"+a.vid+"/import/preview?type=DRIVES", []byte(csv), "text/csv")
	var preview services.CSVPreviewResult
	if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &preview) != nil || preview.ValidRows != 1 {
		t.Fatalf("preview: %d %s", rec.Code, rec.Body.String())
	}
	rec = a.serve(a.owner, http.MethodPost, "/"+a.vid+"/import?type=DRIVES", []byte(csv), "text/csv")
	if rec.Code != http.StatusOK {
		t.Fatalf("execute: %d %s", rec.Code, rec.Body.String())
	}
	var km float64
	if err := a.repo.Pool().QueryRow(context.Background(), `SELECT distance_km FROM drives WHERE vehicle_id = $1`, a.vid).Scan(&km); err != nil {
		t.Fatal(err)
	}
	if km < 16 || km > 16.2 {
		t.Fatalf("10 miles must be stored as about 16.09 km, got %v", km)
	}
}
