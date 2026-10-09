package database

import (
	"errors"
	"strings"
	"testing"
)

func TestUpsertOIDCUserDoesNotLinkALocalAccountUnlessAllowed(t *testing.T) {
	repo, ctx, owner, _, _ := vehicleRepoFixture(t, "oidcupsert")

	_, err := repo.UpsertOIDCUser(ctx, owner.Email, "sub-1", "https://idp.example", "Owner", false)
	if !errors.Is(err, ErrLocalAccountExists) {
		t.Fatalf("a local account is not linked by its address: %v", err)
	}
	if u, _ := repo.GetUserByEmail(ctx, owner.Email); u.OIDCSubject != nil {
		t.Fatal("the account must be untouched")
	}

	linked, err := repo.UpsertOIDCUser(ctx, owner.Email, "sub-1", "https://idp.example", "Owner", true)
	if err != nil || linked.ID != owner.ID || linked.OIDCSubject == nil {
		t.Fatalf("with local sign-in off the account is linked: %+v, %v", linked, err)
	}

	// An account tied to an identity is never re-pointed, whatever the setting.
	if _, err := repo.UpsertOIDCUser(ctx, owner.Email, "sub-2", "https://idp.example", "Owner", true); err == nil {
		t.Fatal("a linked account must not be re-pointed to another identity")
	}

	// A new address simply creates its account.
	fresh, err := repo.UpsertOIDCUser(ctx, "brand-new@example.org", "sub-3", "https://idp.example", "New", false)
	if err != nil || fresh.OIDCSubject == nil {
		t.Fatalf("new identity: %+v, %v", fresh, err)
	}
}

func TestLinkOIDCIdentityOutcomes(t *testing.T) {
	repo, ctx, owner, guest, _ := vehicleRepoFixture(t, "oidclink")
	const idp = "https://idp.example"

	if _, err := repo.LinkOIDCIdentity(ctx, owner.ID, "someone.else@example.org", "sub-a", idp); !errors.Is(err, ErrOIDCEmailMismatch) {
		t.Fatalf("another address: %v", err)
	}
	if _, err := repo.LinkOIDCIdentity(ctx, "00000000-0000-0000-0000-000000000000", owner.Email, "sub-a", idp); !errors.Is(err, ErrNotFound) {
		t.Fatalf("unknown user: %v", err)
	}
	linked, err := repo.LinkOIDCIdentity(ctx, owner.ID, strings.ToUpper(owner.Email), "sub-a", idp)
	if err != nil || linked.OIDCSubject == nil || *linked.OIDCSubject != "sub-a" {
		t.Fatalf("link, address compared without case: %+v, %v", linked, err)
	}
	if _, err := repo.LinkOIDCIdentity(ctx, owner.ID, owner.Email, "sub-b", idp); !errors.Is(err, ErrOIDCAlreadyLinked) {
		t.Fatalf("already linked: %v", err)
	}
	if _, err := repo.LinkOIDCIdentity(ctx, guest.ID, guest.Email, "sub-a", idp); !errors.Is(err, ErrOIDCIdentityInUse) {
		t.Fatalf("an identity belongs to one account: %v", err)
	}
	if u, _ := repo.GetUserByEmail(ctx, guest.Email); u.OIDCSubject != nil {
		t.Fatal("the second account must not have been touched")
	}
}
