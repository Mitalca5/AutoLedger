package auth

import (
	"testing"
	"time"
)

const linkTestSecret = "test-secret-test-secret-test-secret-00"

func TestOIDCLinkIntentRoundTripsForItsUser(t *testing.T) {
	now := time.Now()
	value := SignOIDCLinkIntent("user-1", linkTestSecret, now.Add(10*time.Minute))
	if id, ok := VerifyOIDCLinkIntent(value, linkTestSecret, now); !ok || id != "user-1" {
		t.Fatalf("got %q, %v", id, ok)
	}
}

func TestOIDCLinkIntentIsRefusedWhenExpiredTamperedOrSignedElsewhere(t *testing.T) {
	now := time.Now()
	value := SignOIDCLinkIntent("user-1", linkTestSecret, now.Add(10*time.Minute))
	if _, ok := VerifyOIDCLinkIntent(value, linkTestSecret, now.Add(11*time.Minute)); ok {
		t.Error("an expired proof must be refused")
	}
	if _, ok := VerifyOIDCLinkIntent(value, "another-secret-another-secret-0000000", now); ok {
		t.Error("a proof signed with another secret must be refused")
	}
	other := SignOIDCLinkIntent("user-2", linkTestSecret, now.Add(10*time.Minute))
	parts := splitIntent(value)
	forged := splitIntent(other)[0] + "." + parts[1] + "." + parts[2]
	if _, ok := VerifyOIDCLinkIntent(forged, linkTestSecret, now); ok {
		t.Error("swapping the user must break the signature")
	}
	for _, junk := range []string{"", "a.b", "a.b.c.d", "!!!.1.x", SignOIDCLinkIntent("", linkTestSecret, now.Add(time.Hour))} {
		if _, ok := VerifyOIDCLinkIntent(junk, linkTestSecret, now); ok {
			t.Errorf("%q must be refused", junk)
		}
	}
}

func TestOIDCLinkIntentIsNotAnAccessToken(t *testing.T) {
	value := SignOIDCLinkIntent("user-1", linkTestSecret, time.Now().Add(time.Hour))
	if _, err := ValidateToken(value, linkTestSecret); err == nil {
		t.Fatal("a link proof must never be accepted as an access token")
	}
}

func splitIntent(v string) []string {
	out := []string{}
	start := 0
	for i := 0; i < len(v); i++ {
		if v[i] == '.' {
			out = append(out, v[start:i])
			start = i + 1
		}
	}
	return append(out, v[start:])
}
