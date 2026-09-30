package handlers

import (
	"io"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/teslacost/teslacost/internal/apierror"
	"github.com/teslacost/teslacost/internal/database"
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

// Preview parses the uploaded CSV and returns diagnostic info without inserting records.
func (h *ImportHandler) Preview(w http.ResponseWriter, r *http.Request) {
	vehicleID := chi.URLParam(r, "vehicleId")
	vehicle := requireVehicleAccess(w, r, h.repo, vehicleID, models.RoleEditor)
	if vehicle == nil {
		return
	}

	data, err := readCSVBody(w, r)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	if len(data) == 0 {
		writeAPIError(w, http.StatusBadRequest, apierror.New("import.empty", "The CSV file is empty"))
		return
	}

	preview, err := h.importService.Preview(data)
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, apierror.New("import.invalid_csv", err.Error()))
		return
	}

	writeJSON(w, http.StatusOK, preview)
}

// Execute parses the uploaded CSV and commits the records to the database.
func (h *ImportHandler) Execute(w http.ResponseWriter, r *http.Request) {
	vehicleID := chi.URLParam(r, "vehicleId")
	vehicle := requireVehicleAccess(w, r, h.repo, vehicleID, models.RoleEditor)
	if vehicle == nil {
		return
	}

	targetType := services.ImportType(r.URL.Query().Get("type"))
	skipDuplicates := r.URL.Query().Get("skip_duplicates") != "false" // default: true

	data, err := readCSVBody(w, r)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	if len(data) == 0 {
		writeAPIError(w, http.StatusBadRequest, apierror.New("import.empty", "The CSV file is empty"))
		return
	}

	// Also check if type was passed in multipart form
	if r.MultipartForm != nil {
		if t := r.MultipartForm.Value["type"]; len(t) > 0 && t[0] != "" {
			targetType = services.ImportType(t[0])
		}
		if s := r.MultipartForm.Value["skip_duplicates"]; len(s) > 0 {
			skipDuplicates = s[0] != "false"
		}
	}

	result, err := h.importService.Execute(r.Context(), vehicle, data, targetType, skipDuplicates)
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, apierror.New("import.execute_failed", err.Error()))
		return
	}

	writeJSON(w, http.StatusOK, result)
}
