package database

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestAPITokens(t *testing.T) {
	pool := sourcesTestPool(t)
	ctx := context.Background()
	if err := (&DB{Pool: pool}).Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	repo := NewRepository(pool)

	user, err := repo.CreateUser(ctx, "tokens-owner@example.com", "x")
	if err != nil {
		t.Fatal(err)
	}
	other, err := repo.CreateUser(ctx, "tokens-other@example.com", "x")
	if err != nil {
		t.Fatal(err)
	}

	list, err := repo.ListAPITokens(ctx, user.ID)
	if err != nil || list == nil || len(list) != 0 {
		t.Fatalf("a user without token must get an empty, non-nil list: %+v, %v", list, err)
	}

	live, err := repo.CreateAPIToken(ctx, user.ID, "home assistant", "hash-live", "al_live", nil)
	if err != nil || live.ID == "" || live.CreatedAt.IsZero() {
		t.Fatalf("create = %+v, %v", live, err)
	}
	past := time.Now().Add(-time.Hour)
	expired, err := repo.CreateAPIToken(ctx, user.ID, "old", "hash-expired", "al_old", &past)
	if err != nil {
		t.Fatal(err)
	}

	list, err = repo.ListAPITokens(ctx, user.ID)
	if err != nil || len(list) != 2 {
		t.Fatalf("list = %+v, %v", list, err)
	}
	if list[0].ID != expired.ID {
		t.Fatalf("tokens must be listed newest first, got %s first", list[0].Name)
	}

	// Only a live token validates, and it resolves to its owner.
	id, email, err := repo.ValidateAPIToken(ctx, "hash-live")
	if err != nil || id != user.ID || email != "tokens-owner@example.com" {
		t.Fatalf("validate = %q %q %v", id, email, err)
	}
	if _, _, err := repo.ValidateAPIToken(ctx, "hash-expired"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expired token: err = %v, want ErrNotFound", err)
	}
	if _, _, err := repo.ValidateAPIToken(ctx, "hash-unknown"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("unknown token: err = %v, want ErrNotFound", err)
	}

	// Validation records the last use, asynchronously.
	deadline := time.Now().Add(3 * time.Second)
	for {
		list, _ = repo.ListAPITokens(ctx, user.ID)
		var used bool
		for _, tok := range list {
			if tok.ID == live.ID && tok.LastUsedAt != nil {
				used = true
			}
		}
		if used {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("last_used_at was not recorded")
		}
		time.Sleep(50 * time.Millisecond)
	}

	// A token can only be revoked by its owner.
	if err := repo.RevokeAPIToken(ctx, other.ID, live.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("revoke by another user: err = %v, want ErrNotFound", err)
	}
	if err := repo.RevokeAPIToken(ctx, user.ID, live.ID); err != nil {
		t.Fatal(err)
	}
	if _, _, err := repo.ValidateAPIToken(ctx, "hash-live"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("revoked token: err = %v, want ErrNotFound", err)
	}
}
