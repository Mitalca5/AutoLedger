package handlers

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/teslacost/teslacost/internal/apierror"
	"github.com/teslacost/teslacost/internal/database"
	"github.com/teslacost/teslacost/internal/middleware"
	"github.com/teslacost/teslacost/internal/models"
	"github.com/teslacost/teslacost/internal/services"
)

type ImportHandler struct {
	repo          *database.Repository
	importService *services.CSVImportService
}

func NewImportHandler(repo *database.Repository, importService *services.CSVImportService) *ImportHandler {
	return &ImportHandler{
		repo:          repo,
		importService: importService,
	}
}

// readCSVBody extracts CSV bytes whether sent via multipart/form-data or raw request body.
func readCSVBody(w http.ResponseWriter, r *http.Request) ([]byte, error) {
	const maxUploadSize = 10 << 20 // 10 MB
	r.Body = http.MaxBytesReader(w, r.Body, maxUploadSize)

	contentType := r.Header.Get("Content-Type")
	if strings.HasPrefix(contentType, "multipart/form-data") {
		if err := r.ParseMultipartForm(maxUploadSize); err != nil {
			return nil, apierror.New("import.too_large", "File too large or invalid multipart form")
		}
		file, _, err := r.FormFile("file")
		if err != nil {
			return nil, apierror.New("import.missing_file", "Missing file in multipart form (field 'file')")
		}
		defer file.Close()
		data, err := io.ReadAll(file)
		if err != nil {
			return nil, apierror.New("import.read_failed", "Failed to read uploaded file")
		}
		return data, nil
	}

	data, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, apierror.New("import.read_failed", "Failed to read request body")
	}
	return data, nil
}

// importOptions reads the import type and the duplicate policy from the query string or the multipart form,
// and the distance unit from the signed-in user's setting.
func (h *ImportHandler) importOptions(r *http.Request) (services.CSVImportOptions, error) {
	opts := services.CSVImportOptions{
		Type:           services.ImportType(r.URL.Query().Get("type")),
		SkipDuplicates: r.URL.Query().Get("skip_duplicates") != "false", // default: true
		DistanceUnit:   "km",
		UserID:         middleware.GetUserID(r.Context()),
	}
	if r.MultipartForm != nil {
		if t := r.MultipartForm.Value["type"]; len(t) > 0 && t[0] != "" {
			opts.Type = services.ImportType(t[0])
		}
		if s := r.MultipartForm.Value["skip_duplicates"]; len(s) > 0 {
			opts.SkipDuplicates = s[0] != "false"
		}
		opts.DecimalSeparator = multipartValue(r, "decimal_separator")
		opts.DateOrder = multipartValue(r, "date_order")
		if raw := multipartValue(r, "mapping"); raw != "" {
			if err := json.Unmarshal([]byte(raw), &opts.Mapping); err != nil {
				return opts, apierror.New("import.invalid_mapping", "The column mapping is not valid")
			}
		}
	}
	if user, err := h.repo.GetUserByID(r.Context(), middleware.GetUserID(r.Context())); err == nil && user != nil && user.DistanceUnit == "mi" {
		opts.DistanceUnit = "mi"
	}
	return opts, nil
}

func multipartValue(r *http.Request, name string) string {
	if v := r.MultipartForm.Value[name]; len(v) > 0 {
		return strings.TrimSpace(v[0])
	}
	return ""
}

// prepare checks access and reads the file shared by Preview and Execute; it answers and returns nil on failure.
func (h *ImportHandler) prepare(w http.ResponseWriter, r *http.Request) (*models.Vehicle, []byte) {
	vehicle := requireVehicleAccess(w, r, h.repo, chi.URLParam(r, "vehicleId"), models.RoleEditor)
	if vehicle == nil {
		return nil, nil
	}
	data, err := readCSVBody(w, r)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return nil, nil
	}
	if len(data) == 0 {
		writeAPIError(w, http.StatusBadRequest, apierror.New("import.empty", "The CSV file is empty"))
		return nil, nil
	}
	return vehicle, data
}

// Preview dry-runs the import: the counts it returns are those Execute would produce, nothing is written.
func (h *ImportHandler) Preview(w http.ResponseWriter, r *http.Request) {
	vehicle, data := h.prepare(w, r)
	if vehicle == nil {
		return
	}
	opts, err := h.importOptions(r)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	preview, err := h.importService.Preview(r.Context(), vehicle, data, opts)
	if err != nil {
		writeImportError(w, err, "import.invalid_csv")
		return
	}
	writeJSON(w, http.StatusOK, preview)
}

// Execute imports the file all or nothing.
func (h *ImportHandler) Execute(w http.ResponseWriter, r *http.Request) {
	vehicle, data := h.prepare(w, r)
	if vehicle == nil {
		return
	}
	opts, err := h.importOptions(r)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	result, err := h.importService.Execute(r.Context(), vehicle, data, opts)
	if err != nil {
		writeImportError(w, err, "import.execute_failed")
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// writeImportError passes coded errors through and hides the details of any other failure.
func writeImportError(w http.ResponseWriter, err error, fallbackCode string) {
	if apiErr, ok := apierror.As(err); ok {
		writeAPIError(w, http.StatusBadRequest, apiErr)
		return
	}
	slog.Error("CSV import failed", "code", fallbackCode, "error", err)
	writeAPIError(w, http.StatusInternalServerError, apierror.New(fallbackCode, "The import failed"))
}

// ListBatches returns the vehicle's CSV imports, newest first.
func (h *ImportHandler) ListBatches(w http.ResponseWriter, r *http.Request) {
	vehicle := requireVehicleAccess(w, r, h.repo, chi.URLParam(r, "vehicleId"), models.RoleViewer)
	if vehicle == nil {
		return
	}
	batches, err := h.repo.ListImportBatches(r.Context(), vehicle.ID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, batches)
}

// UndoBatch deletes the rows of one CSV import that are still in place.
func (h *ImportHandler) UndoBatch(w http.ResponseWriter, r *http.Request) {
	vehicle := requireVehicleAccess(w, r, h.repo, chi.URLParam(r, "vehicleId"), models.RoleEditor)
	if vehicle == nil {
		return
	}
	removed, err := h.repo.UndoImportBatch(r.Context(), vehicle.ID, chi.URLParam(r, "batchId"))
	if errors.Is(err, database.ErrNotFound) {
		writeAPIError(w, http.StatusNotFound, apierror.New("import.batch_not_found", "This import does not exist"))
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]int{"removed": removed})
}
