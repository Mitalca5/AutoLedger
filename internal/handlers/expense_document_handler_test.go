package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/teslacost/teslacost/internal/database"
	"github.com/teslacost/teslacost/internal/middleware"
	"github.com/teslacost/teslacost/internal/models"
	"github.com/teslacost/teslacost/internal/storage"
)

var testPDF = []byte("%PDF-1.7\n%\xe2\xe3\xcf\xd3\n1 0 obj\n<<>>\nendobj\n")

type documentAPI struct {
	t       *testing.T
	router  chi.Router
	repo    *database.Repository
	dir     string
	owner   string
	viewer  string
	outside string
	vid     string
}

func newDocumentAPI(t *testing.T, tag string) *documentAPI {
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
	owner, viewer, outside := mk("owner"), mk("viewer"), mk("outside")
	v := &models.Vehicle{UserID: owner.ID, Name: "Docs car", Powertrain: models.PowertrainEV}
	if err := repo.CreateVehicle(ctx, v); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.AddVehicleMember(ctx, v.ID, viewer.Email, models.RoleViewer, nil); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	fs, err := storage.NewFileStorageService(dir)
	if err != nil {
		t.Fatal(err)
	}
	h := NewExpenseHandler(repo, fs)
	r := chi.NewRouter()
	r.Post("/{vehicleId}/documents", h.UploadDocument)
	r.Get("/{vehicleId}/documents", h.ListDocuments)
	r.Get("/{vehicleId}/documents/{docId}", h.DownloadDocument)
	r.Delete("/{vehicleId}/documents/{docId}", h.DeleteDocument)
	return &documentAPI{t: t, router: r, repo: repo, dir: dir, owner: owner.ID, viewer: viewer.ID, outside: outside.ID, vid: v.ID}
}

func (a *documentAPI) serve(userID, method, path string, body []byte, contentType string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, bytes.NewReader(body))
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	req = req.WithContext(context.WithValue(req.Context(), middleware.UserIDKey, userID))
	rec := httptest.NewRecorder()
	a.router.ServeHTTP(rec, req)
	return rec
}

func (a *documentAPI) upload(userID, field, filename string, data []byte, description string) *httptest.ResponseRecorder {
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	if description != "" {
		_ = mw.WriteField("description", description)
	}
	fw, err := mw.CreateFormFile(field, filename)
	if err != nil {
		a.t.Fatal(err)
	}
	_, _ = fw.Write(data)
	_ = mw.Close()
	return a.serve(userID, http.MethodPost, "/"+a.vid+"/documents", buf.Bytes(), mw.FormDataContentType())
}

func (a *documentAPI) code(rec *httptest.ResponseRecorder) string {
	var body struct {
		Code string `json:"code"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	return body.Code
}

func TestDocumentUploadValidation(t *testing.T) {
	a := newDocumentAPI(t, "up")

	if rec := a.serve(a.owner, http.MethodPost, "/"+a.vid+"/documents", []byte("not multipart"), "text/plain"); rec.Code != http.StatusBadRequest || a.code(rec) != "document.too_large" {
		t.Fatalf("invalid form: %d %s", rec.Code, rec.Body.String())
	}
	if rec := a.upload(a.owner, "other", "x.pdf", testPDF, ""); rec.Code != http.StatusBadRequest || a.code(rec) != "document.file_required" {
		t.Fatalf("missing file field: %d %s", rec.Code, rec.Body.String())
	}
	if rec := a.upload(a.owner, "file", "empty.pdf", nil, ""); rec.Code != http.StatusBadRequest || a.code(rec) != "document.empty" {
		t.Fatalf("empty file: %d %s", rec.Code, rec.Body.String())
	}
	if rec := a.upload(a.owner, "file", "receipt.pdf", []byte("<html><script>alert(1)</script></html>"), ""); rec.Code != http.StatusBadRequest || a.code(rec) != "document.unsupported_format" {
		t.Fatalf("html named .pdf: %d %s", rec.Code, rec.Body.String())
	}
	if rec := a.upload(a.viewer, "file", "x.pdf", testPDF, ""); rec.Code != http.StatusForbidden {
		t.Fatalf("a viewer cannot upload: %d", rec.Code)
	}
	if rec := a.upload(a.outside, "file", "x.pdf", testPDF, ""); rec.Code != http.StatusNotFound && rec.Code != http.StatusForbidden {
		t.Fatalf("a stranger cannot upload: %d", rec.Code)
	}
}

func TestDocumentLifecycle(t *testing.T) {
	a := newDocumentAPI(t, "life")

	rec := a.upload(a.owner, "file", "dir/../receipt.pdf", testPDF, "  Invoice 2026  ")
	if rec.Code != http.StatusCreated {
		t.Fatalf("upload: %d %s", rec.Code, rec.Body.String())
	}
	var created models.ExpenseDocumentHeader
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	if created.Filename != "receipt.pdf" || created.MimeType != "application/pdf" || created.FileSize != int64(len(testPDF)) ||
		created.Description == nil || *created.Description != "Invoice 2026" {
		t.Fatalf("created = %+v", created)
	}
	if _, err := os.Stat(filepath.Join(a.dir, a.vid, created.ID)); err != nil {
		t.Fatalf("the file must be on the volume: %v", err)
	}

	long := strings.Repeat("n", 300) + ".pdf"
	rec = a.upload(a.owner, "file", long, testPDF, "")
	var second models.ExpenseDocumentHeader
	_ = json.Unmarshal(rec.Body.Bytes(), &second)
	if rec.Code != http.StatusCreated || len(second.Filename) != 200 || second.Description != nil {
		t.Fatalf("long name: %d %+v", rec.Code, second)
	}

	rec = a.serve(a.viewer, http.MethodGet, "/"+a.vid+"/documents", nil, "")
	var list []models.ExpenseDocumentHeader
	if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &list) != nil || len(list) != 2 {
		t.Fatalf("list: %d %s", rec.Code, rec.Body.String())
	}

	rec = a.serve(a.viewer, http.MethodGet, "/"+a.vid+"/documents/"+created.ID, nil, "")
	if rec.Code != http.StatusOK || !bytes.Equal(rec.Body.Bytes(), testPDF) ||
		rec.Header().Get("Content-Type") != "application/pdf" || rec.Header().Get("X-Content-Type-Options") != "nosniff" ||
		!strings.HasPrefix(rec.Header().Get("Content-Disposition"), "inline") {
		t.Fatalf("download: %d headers=%v", rec.Code, rec.Header())
	}
	if rec := a.serve(a.outside, http.MethodGet, "/"+a.vid+"/documents/"+created.ID, nil, ""); rec.Code == http.StatusOK {
		t.Fatal("a stranger must not download the document")
	}
	if rec := a.serve(a.owner, http.MethodGet, "/"+a.vid+"/documents/00000000-0000-0000-0000-000000000000", nil, ""); rec.Code != http.StatusNotFound {
		t.Fatalf("unknown document: %d", rec.Code)
	}

	if rec := a.serve(a.viewer, http.MethodDelete, "/"+a.vid+"/documents/"+created.ID, nil, ""); rec.Code != http.StatusForbidden {
		t.Fatalf("a viewer cannot delete: %d", rec.Code)
	}
	if rec := a.serve(a.owner, http.MethodDelete, "/"+a.vid+"/documents/00000000-0000-0000-0000-000000000000", nil, ""); rec.Code != http.StatusNotFound {
		t.Fatalf("delete unknown: %d", rec.Code)
	}
	if rec := a.serve(a.owner, http.MethodDelete, "/"+a.vid+"/documents/"+created.ID, nil, ""); rec.Code != http.StatusOK {
		t.Fatalf("delete: %d %s", rec.Code, rec.Body.String())
	}
	if _, err := os.Stat(filepath.Join(a.dir, a.vid, created.ID)); !os.IsNotExist(err) {
		t.Fatalf("the file must be removed from the volume, stat err = %v", err)
	}
}

func TestDocumentDownloadRecoversLegacyAndMissingFiles(t *testing.T) {
	a := newDocumentAPI(t, "legacy")
	ctx := context.Background()
	insert := func(storagePath *string, data []byte) string {
		var id string
		err := a.repo.Pool().QueryRow(ctx, `INSERT INTO expense_documents
			(user_id, vehicle_id, filename, mime_type, file_size, storage_path, data)
			VALUES ($1, $2, 'old.pdf', 'application/pdf', $3, $4, $5) RETURNING id::text`,
			a.owner, a.vid, len(testPDF), storagePath, data).Scan(&id)
		if err != nil {
			t.Fatal(err)
		}
		return id
	}
	get := func(id string) *httptest.ResponseRecorder {
		return a.serve(a.owner, http.MethodGet, "/"+a.vid+"/documents/"+id, nil, "")
	}

	// Stored only in the database: served, then written to the volume and its path recorded.
	legacy := insert(nil, testPDF)
	if rec := get(legacy); rec.Code != http.StatusOK || !bytes.Equal(rec.Body.Bytes(), testPDF) {
		t.Fatalf("legacy download: %d", rec.Code)
	}
	if _, err := os.Stat(filepath.Join(a.dir, a.vid, legacy)); err != nil {
		t.Fatalf("legacy document must be migrated to the volume: %v", err)
	}
	var path *string
	if err := a.repo.Pool().QueryRow(ctx, `SELECT storage_path FROM expense_documents WHERE id::text = $1`, legacy).Scan(&path); err != nil || path == nil {
		t.Fatalf("storage path not recorded: %v %v", path, err)
	}

	// No path and no data: nothing to serve.
	empty := insert(nil, nil)
	if rec := get(empty); rec.Code != http.StatusNotFound || a.code(rec) != "document.file_missing" {
		t.Fatalf("no file: %d %s", rec.Code, rec.Body.String())
	}

	// A path whose file vanished from the volume is healed from the database copy.
	gone := "gone/path"
	healed := insert(&gone, testPDF)
	if rec := get(healed); rec.Code != http.StatusOK || !bytes.Equal(rec.Body.Bytes(), testPDF) {
		t.Fatalf("heal from database: %d", rec.Code)
	}

	// A path whose file vanished and no database copy: an error, not an empty file.
	lost := insert(&gone, nil)
	if rec := get(lost); rec.Code != http.StatusInternalServerError || a.code(rec) != "document.read_failed" {
		t.Fatalf("lost file: %d %s", rec.Code, rec.Body.String())
	}
}
