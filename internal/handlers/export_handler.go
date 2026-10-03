package handlers

import (
	"fmt"
	"log/slog"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/teslacost/teslacost/internal/apierror"
	"github.com/teslacost/teslacost/internal/database"
	"github.com/teslacost/teslacost/internal/middleware"
	"github.com/teslacost/teslacost/internal/models"
	"github.com/teslacost/teslacost/internal/services"
)

type ExportHandler struct {
	repo   *database.Repository
	export *services.ExportService
}

func NewExportHandler(repo *database.Repository, export *services.ExportService) *ExportHandler {
	return &ExportHandler{repo: repo, export: export}
}

var unsafeFilenameChars = regexp.MustCompile(`[^a-zA-Z0-9_-]+`)

// parseExportDate reads a YYYY-MM-DD query value; the end date covers the whole day.
func parseExportDate(raw string, endOfDay bool) (*time.Time, error) {
	if raw == "" {
		return nil, nil
	}
	d, err := time.Parse("2006-01-02", raw)
	if err != nil {
		return nil, apierror.New("export.invalid_date", "Dates must be formatted YYYY-MM-DD")
	}
	if endOfDay {
		d = d.Add(24*time.Hour - time.Nanosecond)
	}
	return &d, nil
}

// Export downloads one kind of record of a vehicle as CSV or JSON. Any member of the vehicle may export.
func (h *ExportHandler) Export(w http.ResponseWriter, r *http.Request) {
	vehicle := requireVehicleAccess(w, r, h.repo, chi.URLParam(r, "vehicleId"), models.RoleViewer)
	if vehicle == nil {
		return
	}
	q := r.URL.Query()
	from, err := parseExportDate(q.Get("from"), false)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	to, err := parseExportDate(q.Get("to"), true)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	format := services.ExportFormat(q.Get("format"))
	if format == "" {
		format = services.ExportCSV
	}
	opts := services.ExportOptions{
		Type:   services.ExportType(q.Get("type")),
		Format: format,
		From:   from,
		To:     to,
		Tag:    strings.TrimSpace(q.Get("tag")),
		Unit:   "km",
		Lang:   requestLanguage(r, h.repo),
	}
	if user, uerr := h.repo.GetUserByID(r.Context(), middleware.GetUserID(r.Context())); uerr == nil && user != nil && user.DistanceUnit == "mi" {
		opts.Unit = "mi"
	}

	res, err := h.export.Export(r.Context(), vehicle, opts)
	if err != nil {
		if _, ok := apierror.As(err); ok {
			writeErr(w, http.StatusBadRequest, err)
			return
		}
		slog.Error("export failed", "vehicle_id", vehicle.ID, "type", opts.Type, "error", err)
		writeError(w, http.StatusInternalServerError, "Export failed")
		return
	}
	name := strings.Trim(unsafeFilenameChars.ReplaceAllString(vehicle.Name, "-"), "-")
	if name == "" {
		name = "vehicle"
	}
	w.Header().Set("Content-Type", res.ContentType)
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s-%s-%s.%s"`, name, opts.Type, time.Now().UTC().Format("2006-01-02"), res.Extension))
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write(res.Body)
}
