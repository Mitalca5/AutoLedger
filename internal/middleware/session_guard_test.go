package middleware

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

type fakeSessions struct {
	active bool
	err    error
	asked  [2]string
}

func (f *fakeSessions) IsSessionActive(_ context.Context, userID, sessionID string) (bool, error) {
	f.asked = [2]string{userID, sessionID}
	return f.active, f.err
}

func guarded(checker SessionChecker, userID, sessionID string) *httptest.ResponseRecorder {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNoContent) })
	req := httptest.NewRequest(http.MethodPost, "/api/auth/tokens", nil)
	ctx := context.WithValue(req.Context(), UserIDKey, userID)
	ctx = context.WithValue(ctx, SessionIDKey, sessionID)
	rec := httptest.NewRecorder()
	RequireActiveSession(checker)(next).ServeHTTP(rec, req.WithContext(ctx))
	return rec
}

func TestRequireActiveSessionLetsAnActiveSessionThrough(t *testing.T) {
	checker := &fakeSessions{active: true}
	if rec := guarded(checker, "u1", "s1"); rec.Code != http.StatusNoContent {
		t.Fatalf("got %d", rec.Code)
	}
	if checker.asked != [2]string{"u1", "s1"} {
		t.Fatalf("the session of the token must be checked for its user: %v", checker.asked)
	}
}

func TestRequireActiveSessionRefusesAnEndedSession(t *testing.T) {
	if rec := guarded(&fakeSessions{active: false}, "u1", "s1"); rec.Code != http.StatusUnauthorized {
		t.Fatalf("a revoked session must be refused with 401, got %d", rec.Code)
	}
}

func TestRequireActiveSessionRefusesATokenThatNamesNoSession(t *testing.T) {
	checker := &fakeSessions{active: true}
	if rec := guarded(checker, "u1", ""); rec.Code != http.StatusUnauthorized {
		t.Fatalf("a token without a session must be renewed first, got %d", rec.Code)
	}
	if checker.asked != ([2]string{}) {
		t.Fatal("nothing to look up without a session")
	}
	if rec := guarded(checker, "", "s1"); rec.Code != http.StatusUnauthorized {
		t.Fatalf("no user, got %d", rec.Code)
	}
}

func TestRequireActiveSessionFailsClosedWhenTheLookupFails(t *testing.T) {
	if rec := guarded(&fakeSessions{active: true, err: errors.New("db down")}, "u1", "s1"); rec.Code != http.StatusInternalServerError {
		t.Fatalf("got %d", rec.Code)
	}
}
