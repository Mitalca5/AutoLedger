package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/teslacost/teslacost/internal/apierror"
	"github.com/teslacost/teslacost/internal/database"
	"github.com/teslacost/teslacost/internal/middleware"
	"github.com/teslacost/teslacost/internal/models"
)

type VehicleMemberHandler struct {
	repo *database.Repository
}

func NewVehicleMemberHandler(repo *database.Repository) *VehicleMemberHandler {
	return &VehicleMemberHandler{repo: repo}
}

func (h *VehicleMemberHandler) getVehicleID(r *http.Request) string {
	id := chi.URLParam(r, "id")
	if id == "" {
		id = chi.URLParam(r, "vehicleId")
	}
	return id
}

// ListMembers lists all members of the specified vehicle.
func (h *VehicleMemberHandler) ListMembers(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	vehicleID := h.getVehicleID(r)

	if _, err := h.repo.GetVehicleByID(r.Context(), vehicleID, userID); err != nil {
		writeAPIError(w, http.StatusNotFound, apierror.New("vehicle.not_found", "Vehicle not found"))
		return
	}

	members, err := h.repo.ListVehicleMembers(r.Context(), vehicleID)
	if err != nil {
		writeRepoError(w, r, err, "Could not load the members")
		return
	}
	if members == nil {
		members = []models.VehicleMember{}
	}
	writeJSON(w, http.StatusOK, members)
}

// AddMember adds a user to the vehicle by email. Restricted to OWNER.
func (h *VehicleMemberHandler) AddMember(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	vehicleID := h.getVehicleID(r)

	v, err := h.repo.GetVehicleByID(r.Context(), vehicleID, userID)
	if err != nil {
		writeAPIError(w, http.StatusNotFound, apierror.New("vehicle.not_found", "Vehicle not found"))
		return
	}
	if v.Role != models.RoleOwner {
		writeAPIError(w, http.StatusForbidden, apierror.New("access.owner_only_add_member", "Only the owner can add members"))
		return
	}

	var req models.AddVehicleMemberRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAPIError(w, http.StatusBadRequest, apierror.New("request.invalid", "Invalid request"))
		return
	}

	req.Email = strings.TrimSpace(req.Email)
	if req.Email == "" {
		writeAPIError(w, http.StatusBadRequest, apierror.New("auth.email_required", "The email address is required"))
		return
	}
	if !req.Role.IsValid() {
		writeAPIError(w, http.StatusBadRequest, apierror.New("member.invalid_role", "Invalid role"))
		return
	}

	member, err := h.repo.AddVehicleMember(r.Context(), vehicleID, req.Email, req.Role, req.PersonID)
	if err != nil {
		if errors.Is(err, database.ErrNotFound) {
			writeAPIError(w, http.StatusNotFound, apierror.New("member.user_not_found", "No user found with this email address"))
			return
		}
		writeRepoError(w, r, err, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, member)
}

// UpdateMemberRole modifies a member's role. Restricted to OWNER.
func (h *VehicleMemberHandler) UpdateMemberRole(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	vehicleID := h.getVehicleID(r)
	targetUserID := chi.URLParam(r, "memberId")

	v, err := h.repo.GetVehicleByID(r.Context(), vehicleID, userID)
	if err != nil {
		writeAPIError(w, http.StatusNotFound, apierror.New("vehicle.not_found", "Vehicle not found"))
		return
	}
	if v.Role != models.RoleOwner {
		writeAPIError(w, http.StatusForbidden, apierror.New("access.owner_only_roles", "Only the owner can change the roles"))
		return
	}

	var req models.UpdateVehicleMemberRoleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAPIError(w, http.StatusBadRequest, apierror.New("request.invalid", "Invalid request"))
		return
	}
	if !req.Role.IsValid() {
		writeAPIError(w, http.StatusBadRequest, apierror.New("member.invalid_role", "Invalid role"))
		return
	}

	if err := h.repo.UpdateVehicleMemberRole(r.Context(), vehicleID, targetUserID, req.Role); err != nil {
		if errors.Is(err, database.ErrNotFound) {
			writeAPIError(w, http.StatusNotFound, apierror.New("member.not_found", "Member not found"))
			return
		}
		writeRepoError(w, r, err, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": "Role updated"})
}

// RemoveMember removes a member's access. Restricted to OWNER or the member themselves.
func (h *VehicleMemberHandler) RemoveMember(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	vehicleID := h.getVehicleID(r)
	targetUserID := chi.URLParam(r, "memberId")

	v, err := h.repo.GetVehicleByID(r.Context(), vehicleID, userID)
	if err != nil {
		writeAPIError(w, http.StatusNotFound, apierror.New("vehicle.not_found", "Vehicle not found"))
		return
	}

	if v.Role != models.RoleOwner && targetUserID != userID {
		writeAPIError(w, http.StatusForbidden, apierror.New("access.owner_only_remove_member", "Only the owner can remove a member (or you can remove yourself)"))
		return
	}

	if err := h.repo.RemoveVehicleMember(r.Context(), vehicleID, targetUserID); err != nil {
		if errors.Is(err, database.ErrNotFound) {
			writeAPIError(w, http.StatusNotFound, apierror.New("member.not_found", "Member not found"))
			return
		}
		writeRepoError(w, r, err, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": "Member removed"})
}

// ListPeople lists the people who drive the vehicle, with or without an account.
func (h *VehicleMemberHandler) ListPeople(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	vehicleID := h.getVehicleID(r)
	if _, err := h.repo.GetVehicleByID(r.Context(), vehicleID, userID); err != nil {
		writeAPIError(w, http.StatusNotFound, apierror.New("vehicle.not_found", "Vehicle not found"))
		return
	}
	people, err := h.repo.ListVehiclePeople(r.Context(), vehicleID)
	if err != nil {
		writeRepoError(w, r, err, "Could not load the drivers")
		return
	}
	writeJSON(w, http.StatusOK, people)
}

func (h *VehicleMemberHandler) requireOwner(w http.ResponseWriter, r *http.Request) (string, bool) {
	userID := middleware.GetUserID(r.Context())
	vehicleID := h.getVehicleID(r)
	v, err := h.repo.GetVehicleByID(r.Context(), vehicleID, userID)
	if err != nil {
		writeAPIError(w, http.StatusNotFound, apierror.New("vehicle.not_found", "Vehicle not found"))
		return "", false
	}
	if v.Role != models.RoleOwner {
		writeAPIError(w, http.StatusForbidden, apierror.New("access.owner_only_people", "Only the owner can manage the drivers"))
		return "", false
	}
	return vehicleID, true
}

// CreatePerson adds a driver without an account. Restricted to OWNER.
func (h *VehicleMemberHandler) CreatePerson(w http.ResponseWriter, r *http.Request) {
	vehicleID, ok := h.requireOwner(w, r)
	if !ok {
		return
	}
	var req models.SaveVehiclePersonRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAPIError(w, http.StatusBadRequest, apierror.New("request.invalid", "Invalid request"))
		return
	}
	person, err := h.repo.CreateVehiclePerson(r.Context(), vehicleID, req.Name)
	if err != nil {
		writeRepoError(w, r, err, "Could not add the driver")
		return
	}
	writeJSON(w, http.StatusCreated, person)
}

// UpdatePerson renames a driver. Restricted to OWNER.
func (h *VehicleMemberHandler) UpdatePerson(w http.ResponseWriter, r *http.Request) {
	vehicleID, ok := h.requireOwner(w, r)
	if !ok {
		return
	}
	var req models.SaveVehiclePersonRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAPIError(w, http.StatusBadRequest, apierror.New("request.invalid", "Invalid request"))
		return
	}
	person, err := h.repo.RenameVehiclePerson(r.Context(), vehicleID, chi.URLParam(r, "personId"), req.Name)
	if err != nil {
		writePersonError(w, r, err, "Could not update the driver")
		return
	}
	writeJSON(w, http.StatusOK, person)
}

// LinkPerson gives a driver the account of one of the vehicle's members. Restricted to OWNER.
func (h *VehicleMemberHandler) LinkPerson(w http.ResponseWriter, r *http.Request) {
	vehicleID, ok := h.requireOwner(w, r)
	if !ok {
		return
	}
	var req models.LinkVehiclePersonRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.UserID == "" {
		writeAPIError(w, http.StatusBadRequest, apierror.New("request.invalid", "Invalid request"))
		return
	}
	person, err := h.repo.LinkVehiclePersonToMember(r.Context(), vehicleID, chi.URLParam(r, "personId"), req.UserID)
	if err != nil {
		writePersonError(w, r, err, "Could not link the account")
		return
	}
	writeJSON(w, http.StatusOK, person)
}

// SetDefaultPerson chooses the default driver of the vehicle. Restricted to OWNER.
func (h *VehicleMemberHandler) SetDefaultPerson(w http.ResponseWriter, r *http.Request) {
	vehicleID, ok := h.requireOwner(w, r)
	if !ok {
		return
	}
	if err := h.repo.SetVehicleDefaultDriver(r.Context(), vehicleID, chi.URLParam(r, "personId")); err != nil {
		writePersonError(w, r, err, "Could not set the default driver")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"success": true})
}

// DeletePerson removes a driver without an account. Restricted to OWNER.
func (h *VehicleMemberHandler) DeletePerson(w http.ResponseWriter, r *http.Request) {
	vehicleID, ok := h.requireOwner(w, r)
	if !ok {
		return
	}
	if err := h.repo.DeleteVehiclePerson(r.Context(), vehicleID, chi.URLParam(r, "personId")); err != nil {
		writePersonError(w, r, err, "Could not remove the driver")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func writePersonError(w http.ResponseWriter, r *http.Request, err error, fallback string) {
	if errors.Is(err, database.ErrNotFound) {
		writeAPIError(w, http.StatusNotFound, apierror.New("person.not_found", "Driver not found"))
		return
	}
	writeRepoError(w, r, err, fallback)
}
