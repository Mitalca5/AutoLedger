package handlers

import (
	"encoding/json"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/go-chi/chi/v5"

	"github.com/teslacost/teslacost/internal/apierror"
	"github.com/teslacost/teslacost/internal/database"
	"github.com/teslacost/teslacost/internal/middleware"
	"github.com/teslacost/teslacost/internal/models"
	"github.com/teslacost/teslacost/internal/services"
)

const (
	maxImportProfiles       = 50
	maxImportProfileColumns = 100
)

// ImportProfileHandler manages the CSV column mappings a user saved.
type ImportProfileHandler struct {
	repo *database.Repository
}

func NewImportProfileHandler(repo *database.Repository) *ImportProfileHandler {
	return &ImportProfileHandler{repo: repo}
}

type saveImportProfileRequest struct {
	Name             string            `json:"name"`
	ImportType       string            `json:"import_type"`
	Columns          map[string]string `json:"columns"`
	DateOrder        string            `json:"date_order"`
	DecimalSeparator string            `json:"decimal_separator"`
}

// validateImportProfile returns the profile a request describes, or the reason it cannot be saved.
func validateImportProfile(req *saveImportProfileRequest) (*models.ImportProfile, error) {
	name := strings.TrimSpace(req.Name)
	if name == "" || utf8.RuneCountInString(name) > 60 {
		return nil, apierror.New("import.profile_invalid_name", "The profile name must have 1 to 60 characters")
	}
	importType := services.ImportType(req.ImportType)
	switch importType {
	case services.ImportTypeCharges, services.ImportTypeDrives, services.ImportTypeFuel, services.ImportTypeOdometer:
	default:
		return nil, apierror.Newf("import.unsupported_type", "Unrecognized or unsupported import type %q", req.ImportType)
	}
	if len(req.Columns) > maxImportProfileColumns {
		return nil, apierror.New("import.invalid_mapping", "The column mapping is not valid")
	}
	for _, field := range req.Columns {
		if !services.ValidImportField(importType, field) {
			return nil, apierror.Newf("import.unknown_field", "Field %q cannot be imported as %s", field, req.ImportType)
		}
	}
	if !services.ValidDateOrder(req.DateOrder) {
		return nil, apierror.Newf("import.invalid_date_order", "Unknown date order %q", req.DateOrder)
	}
	if !services.ValidDecimalSeparator(req.DecimalSeparator) {
		return nil, apierror.Newf("import.invalid_decimal_separator", "Unknown decimal separator %q", req.DecimalSeparator)
	}
	columns := req.Columns
	if columns == nil {
		columns = map[string]string{}
	}
	return &models.ImportProfile{
		Name: name, ImportType: req.ImportType, Columns: columns,
		DateOrder: req.DateOrder, DecimalSeparator: req.DecimalSeparator,
	}, nil
}

func (h *ImportProfileHandler) List(w http.ResponseWriter, r *http.Request) {
	profiles, err := h.repo.ListImportProfiles(r.Context(), middleware.GetUserID(r.Context()))
	if err != nil {
		writeRepoError(w, r, err, "Failed to list import profiles")
		return
	}
	writeJSON(w, http.StatusOK, profiles)
}

// Save creates a profile, or replaces the one of the same name.
func (h *ImportProfileHandler) Save(w http.ResponseWriter, r *http.Request) {
	var req saveImportProfileRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAPIError(w, http.StatusBadRequest, apierror.New("request.invalid_body", "Invalid request body"))
		return
	}
	profile, err := validateImportProfile(&req)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	userID := middleware.GetUserID(r.Context())
	existing, err := h.repo.ListImportProfiles(r.Context(), userID)
	if err != nil {
		writeRepoError(w, r, err, "Failed to save the import profile")
		return
	}
	replaces := false
	for _, p := range existing {
		replaces = replaces || p.Name == profile.Name
	}
	if !replaces && len(existing) >= maxImportProfiles {
		writeAPIError(w, http.StatusBadRequest, apierror.Newf("import.profile_limit", "At most %d import profiles can be saved", maxImportProfiles))
		return
	}
	if err := h.repo.SaveImportProfile(r.Context(), userID, profile); err != nil {
		writeRepoError(w, r, err, "Failed to save the import profile")
		return
	}
	writeJSON(w, http.StatusOK, profile)
}

func (h *ImportProfileHandler) Delete(w http.ResponseWriter, r *http.Request) {
	if err := h.repo.DeleteImportProfile(r.Context(), middleware.GetUserID(r.Context()), chi.URLParam(r, "profileId")); err != nil {
		writeRepoError(w, r, err, "Failed to delete the import profile")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"message": "Profile deleted"})
}
