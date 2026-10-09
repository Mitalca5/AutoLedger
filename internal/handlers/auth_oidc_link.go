package handlers

import (
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/teslacost/teslacost/internal/apierror"
	"github.com/teslacost/teslacost/internal/auth"
	"github.com/teslacost/teslacost/internal/database"
	"github.com/teslacost/teslacost/internal/middleware"
)

// oidcLinkCookie carries the proof that a signed-in user asked to link an SSO identity (see auth.SignOIDCLinkIntent).
const oidcLinkCookie = "teslacost_oidc_link"

// OIDCLink starts linking the SSO identity of the provider to the account of the signed-in user. It runs the usual
// login round trip, with a proof of who asked, so the callback links the identity instead of signing anyone in. An
// account is linked this way, not by sharing an email address with an identity, because the address of a local
// account is not proof of who owns it.
func (h *AuthHandler) OIDCLink(w http.ResponseWriter, r *http.Request) {
	if h.oidcService == nil {
		writeAPIError(w, http.StatusNotFound, apierror.New("auth.oidc_not_configured", "OIDC is not configured on this instance"))
		return
	}
	user, err := h.repo.GetUserByID(r.Context(), middleware.GetUserID(r.Context()))
	if err != nil {
		writeAPIError(w, http.StatusUnauthorized, apierror.New("auth.unauthorized", "Unauthorized"))
		return
	}
	if user.OIDCSubject != nil {
		http.Redirect(w, r, "/account?sso=already_linked", http.StatusFound)
		return
	}
	expires := time.Now().Add(oidcCookieTTL)
	http.SetCookie(w, &http.Cookie{ // NOSONAR - Secure is config-driven (cfg.CookieSecure), see comment on setRefreshTokenCookie.
		Name:     oidcLinkCookie,
		Value:    auth.SignOIDCLinkIntent(user.ID, h.cfg.JWTSecret, expires),
		Expires:  expires,
		HttpOnly: true,
		Secure:   h.cfg.CookieSecure,
		SameSite: http.SameSiteLaxMode,
		Path:     "/",
	})
	h.OIDCLogin(w, r)
}

// finishOIDCLink completes a link started by OIDCLink and reports whether the callback was one. The link is made
// for the user the proof names, never for whoever the provider's answer is about: an identity that is not the
// account's own address is refused.
func (h *AuthHandler) finishOIDCLink(w http.ResponseWriter, r *http.Request, identity *auth.UserInfo) bool {
	cookie, err := r.Cookie(oidcLinkCookie)
	if err != nil {
		return false
	}
	h.clearCookie(w, oidcLinkCookie)
	userID, ok := auth.VerifyOIDCLinkIntent(cookie.Value, h.cfg.JWTSecret, time.Now())
	if !ok {
		http.Redirect(w, r, "/account?sso=failed", http.StatusFound)
		return true
	}
	_, err = h.repo.LinkOIDCIdentity(r.Context(), userID, identity.Email, identity.Subject, h.cfg.OIDCIssuerURL)
	outcome := "linked"
	switch {
	case err == nil:
	case errors.Is(err, database.ErrOIDCAlreadyLinked):
		outcome = "already_linked"
	case errors.Is(err, database.ErrOIDCIdentityInUse):
		outcome = "identity_in_use"
	case errors.Is(err, database.ErrOIDCEmailMismatch):
		outcome = "email_mismatch"
	default:
		slog.WarnContext(r.Context(), "linking an SSO identity failed", "component", "auth", "error", err)
		outcome = "failed"
	}
	http.Redirect(w, r, "/account?sso="+outcome, http.StatusFound)
	return true
}
