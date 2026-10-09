package auth

import "testing"

const sessionTestSecret = "test-secret-test-secret-test-secret-00"

func TestAccessTokenCanBeBoundToItsSession(t *testing.T) {
	token, err := GenerateSessionAccessToken("user-1", "u@example.org", "session-9", sessionTestSecret, 15)
	if err != nil {
		t.Fatal(err)
	}
	claims, err := ValidateToken(token, sessionTestSecret)
	if err != nil || claims.SessionID != "session-9" || claims.UserID != "user-1" {
		t.Fatalf("claims = %+v, %v", claims, err)
	}
}

func TestAccessTokenWithoutASessionStillValidatesButNamesNone(t *testing.T) {
	token, err := GenerateAccessToken("user-1", "u@example.org", sessionTestSecret, 15)
	if err != nil {
		t.Fatal(err)
	}
	claims, err := ValidateToken(token, sessionTestSecret)
	if err != nil || claims.SessionID != "" {
		t.Fatalf("a token issued before sessions were named must still read data: %+v, %v", claims, err)
	}
}

func TestSessionBindingIsPartOfTheSignature(t *testing.T) {
	token, _ := GenerateSessionAccessToken("user-1", "u@example.org", "session-9", sessionTestSecret, 15)
	other, _ := GenerateSessionAccessToken("user-1", "u@example.org", "session-10", sessionTestSecret, 15)
	if token == other {
		t.Fatal("tokens of two sessions must differ")
	}
	if _, err := ValidateToken(token, "another-secret-another-secret-0000000"); err == nil {
		t.Fatal("a token signed with another secret must be refused")
	}
}
