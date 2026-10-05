package handlers

import (
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/teslacost/teslacost/internal/database"
	"github.com/teslacost/teslacost/internal/middleware"
	"github.com/teslacost/teslacost/internal/models"
	"github.com/teslacost/teslacost/internal/services"
)

// ServiceBookHandler serves the printable service book of a vehicle.
type ServiceBookHandler struct {
	repo *database.Repository
	book *services.ServiceBookService
}

func NewServiceBookHandler(repo *database.Repository, book *services.ServiceBookService) *ServiceBookHandler {
	return &ServiceBookHandler{repo: repo, book: book}
}

// Download returns the service book as a PDF. Any member of the vehicle may download it.
// Query: from, to (YYYY-MM-DD, optional), attachments=1 to append the picture proofs.
func (h *ServiceBookHandler) Download(w http.ResponseWriter, r *http.Request) {
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
	userID := middleware.GetUserID(r.Context())
	opts := services.ServiceBookOptions{
		Lang:           requestLanguage(r, h.repo),
		Unit:           "km",
		From:           from,
		To:             to,
		WithAttachment: q.Get("attachments") == "1",
	}
	if user, uerr := h.repo.GetUserByID(r.Context(), userID); uerr == nil && user != nil && user.DistanceUnit == "mi" {
		opts.Unit = "mi"
	}
	pdf, err := h.book.Build(r.Context(), vehicle, userID, opts)
	if err != nil {
		slog.Error("service book failed", "vehicle_id", vehicle.ID, "error", err)
		writeError(w, http.StatusInternalServerError, "Service book generation failed")
		return
	}
	name := strings.Trim(unsafeFilenameChars.ReplaceAllString(vehicle.Name, "-"), "-")
	if name == "" {
		name = "vehicle"
	}
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s-service-book-%s.pdf"`, name, time.Now().UTC().Format("2006-01-02")))
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	_, _ = w.Write(pdf)
}
