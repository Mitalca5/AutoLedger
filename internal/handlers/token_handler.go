package handlers

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/teslacost/teslacost/internal/apierror"
	"github.com/teslacost/teslacost/internal/auth"
	"github.com/teslacost/teslacost/internal/database"
	"github.com/teslacost/teslacost/internal/middleware"
	"github.com/teslacost/teslacost/internal/models"
)

type TokenHandler struct {
	repo *database.Repository
}

func NewTokenHandler(repo *database.Repository) *TokenHandler {
	return &TokenHandler{repo: repo}
}

func (h *TokenHandler) ListTokens(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	tokens, err := h.repo.ListAPITokens(r.Context(), userID)
	if err != nil {
		writeRepoError(w, r, err, "Failed to list API tokens")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"tokens": tokens})
}

func (h *TokenHandler) CreateToken(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())

	var req models.CreateAPITokenRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAPIError(w, http.StatusBadRequest, apierror.New("request.invalid_body", "Invalid request body"))
		return
	}

	name := strings.TrimSpace(req.Name)
	if name == "" {
		writeAPIError(w, http.StatusBadRequest, apierror.New("token.name_required", "Token name is required"))
		return
	}
	if len(name) > 100 {
		writeAPIError(w, http.StatusBadRequest, apierror.New("token.name_too_long", "Token name must be at most 100 characters"))
		return
	}

	rawToken, prefix, tokenHash, err := auth.GenerateAPIToken()
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, apierror.New("internal", "Failed to generate token"))
		return
	}

	tokenInfo, err := h.repo.CreateAPIToken(r.Context(), userID, name, tokenHash, prefix, req.ExpiresAt)
	if err != nil {
		writeRepoError(w, r, err, "Failed to store token")
		return
	}

	writeJSON(w, http.StatusCreated, models.APITokenCreatedResponse{
		Token: rawToken,
		Info:  *tokenInfo,
	})
}

func (h *TokenHandler) RevokeToken(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	tokenID := chi.URLParam(r, "tokenId")

	if err := h.repo.RevokeAPIToken(r.Context(), userID, tokenID); err != nil {
		writeRepoError(w, r, err, "Failed to revoke token")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"success": true})
}
