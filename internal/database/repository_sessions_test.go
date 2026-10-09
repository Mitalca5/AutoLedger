package database

import (
	"testing"
	"time"
)

func TestIsSessionActive(t *testing.T) {
	repo, ctx, owner, other, _ := vehicleRepoFixture(t, "sessions")
	in := func(d time.Duration) time.Time { return time.Now().Add(d) }
	mk := func(userID, hash, family string, expires time.Time) {
		t.Helper()
		if _, err := repo.CreateRefreshToken(ctx, userID, hash, family, expires, nil, nil); err != nil {
			t.Fatal(err)
		}
	}
	const live, revoked, expired = "00000000-0000-0000-0000-0000000000a1", "00000000-0000-0000-0000-0000000000a2", "00000000-0000-0000-0000-0000000000a3"
	mk(owner.ID, "h-live", live, in(time.Hour))
	mk(owner.ID, "h-revoked", revoked, in(time.Hour))
	mk(owner.ID, "h-expired", expired, in(-time.Hour))
	if err := repo.RevokeRefreshToken(ctx, "h-revoked"); err != nil {
		t.Fatal(err)
	}

	for name, c := range map[string]struct {
		user, family string
		want         bool
	}{
		"a live session":                     {owner.ID, live, true},
		"a revoked session":                  {owner.ID, revoked, false},
		"an expired session":                 {owner.ID, expired, false},
		"an unknown session":                 {owner.ID, "00000000-0000-0000-0000-0000000000ff", false},
		"the session of another user":        {other.ID, live, false},
		"an identifier that is not a family": {owner.ID, "not-a-uuid", false},
	} {
		got, err := repo.IsSessionActive(ctx, c.user, c.family)
		if err != nil || got != c.want {
			t.Errorf("%s: got %v, %v; want %v", name, got, err, c.want)
		}
	}

	// A session stays active across a rotation: the new token joins the family, the old one is revoked.
	mk(owner.ID, "h-rotated-1", "00000000-0000-0000-0000-0000000000b1", in(time.Hour))
	if err := repo.RevokeRefreshToken(ctx, "h-rotated-1"); err != nil {
		t.Fatal(err)
	}
	mk(owner.ID, "h-rotated-2", "00000000-0000-0000-0000-0000000000b1", in(time.Hour))
	if got, _ := repo.IsSessionActive(ctx, owner.ID, "00000000-0000-0000-0000-0000000000b1"); !got {
		t.Error("a rotated session is still active")
	}
}
