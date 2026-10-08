package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/teslacost/teslacost/internal/database"
	"github.com/teslacost/teslacost/internal/middleware"
	"github.com/teslacost/teslacost/internal/models"
)

type memberAPI struct {
	t       *testing.T
	router  chi.Router
	repo    *database.Repository
	vid     string
	ownerID string
	guestID string
	guest   string
}

func newMemberAPI(t *testing.T, tag string) *memberAPI {
	t.Helper()
	repo := authTestRepo(t)
	ctx := context.Background()
	owner, err := repo.CreateUser(ctx, "owner-"+tag+"@example.com", "hash")
	if err != nil {
		t.Fatal(err)
	}
	guest, err := repo.CreateUser(ctx, "guest-"+tag+"@example.com", "hash")
	if err != nil {
		t.Fatal(err)
	}
	v := &models.Vehicle{UserID: owner.ID, Name: "Car", TeslaMateAuthType: models.AuthModeNone, CurrentOdometer: 1000}
	if err := repo.CreateVehicle(ctx, v); err != nil {
		t.Fatal(err)
	}
	h := NewVehicleMemberHandler(repo)
	r := chi.NewRouter()
	r.Get("/{id}/members", h.ListMembers)
	r.Post("/{id}/members", h.AddMember)
	r.Put("/{id}/members/{memberId}", h.UpdateMemberRole)
	r.Delete("/{id}/members/{memberId}", h.RemoveMember)
	r.Get("/{id}/people", h.ListPeople)
	r.Post("/{id}/people", h.CreatePerson)
	r.Put("/{id}/people/{personId}", h.UpdatePerson)
	r.Delete("/{id}/people/{personId}", h.DeletePerson)
	r.Put("/{id}/people/{personId}/link", h.LinkPerson)
	r.Put("/{id}/people/{personId}/default", h.SetDefaultPerson)
	return &memberAPI{t: t, router: r, repo: repo, vid: v.ID, ownerID: owner.ID, guestID: guest.ID, guest: guest.Email}
}

func (a *memberAPI) as(userID string, want int, method, path, body string) []byte {
	a.t.Helper()
	req := httptest.NewRequest(method, "/"+a.vid+path, bytes.NewBufferString(body))
	req = req.WithContext(context.WithValue(req.Context(), middleware.UserIDKey, userID))
	rec := httptest.NewRecorder()
	a.router.ServeHTTP(rec, req)
	if rec.Code != want {
		a.t.Fatalf("%s %s %s: got %d, want %d (%s)", method, path, body, rec.Code, want, rec.Body.String())
	}
	return rec.Body.Bytes()
}

func decodeObj(t *testing.T, raw []byte) map[string]any {
	t.Helper()
	var obj map[string]any
	if err := json.Unmarshal(raw, &obj); err != nil {
		t.Fatalf("decode %s: %v", raw, err)
	}
	return obj
}

func TestVehicleMemberLifecycle(t *testing.T) {
	a := newMemberAPI(t, "members")

	var members []map[string]any
	if err := json.Unmarshal(a.as(a.ownerID, http.StatusOK, "GET", "/members", ""), &members); err != nil || len(members) != 1 {
		t.Fatalf("owner should be the only member: %v %v", members, err)
	}

	a.as(a.ownerID, http.StatusBadRequest, "POST", "/members", `{`)
	a.as(a.ownerID, http.StatusBadRequest, "POST", "/members", `{"email":"  ","role":"VIEWER"}`)
	a.as(a.ownerID, http.StatusBadRequest, "POST", "/members", `{"email":"`+a.guest+`","role":"BOSS"}`)
	a.as(a.ownerID, http.StatusNotFound, "POST", "/members", `{"email":"nobody@example.com","role":"VIEWER"}`)
	a.as(a.guestID, http.StatusNotFound, "POST", "/members", `{"email":"`+a.guest+`","role":"VIEWER"}`)

	a.as(a.ownerID, http.StatusCreated, "POST", "/members", `{"email":" `+a.guest+` ","role":"VIEWER"}`)
	a.as(a.guestID, http.StatusOK, "GET", "/members", "")
	a.as(a.guestID, http.StatusForbidden, "POST", "/members", `{"email":"x@example.com","role":"VIEWER"}`)
	a.as(a.guestID, http.StatusForbidden, "PUT", "/members/"+a.ownerID, `{"role":"VIEWER"}`)
	a.as(a.guestID, http.StatusForbidden, "DELETE", "/members/"+a.ownerID, "")

	a.as(a.ownerID, http.StatusBadRequest, "PUT", "/members/"+a.guestID, `{`)
	a.as(a.ownerID, http.StatusBadRequest, "PUT", "/members/"+a.guestID, `{"role":"BOSS"}`)
	a.as(a.ownerID, http.StatusNotFound, "PUT", "/members/00000000-0000-0000-0000-000000000000", `{"role":"EDITOR"}`)
	a.as(a.ownerID, http.StatusOK, "PUT", "/members/"+a.guestID, `{"role":"EDITOR"}`)

	a.as(a.ownerID, http.StatusNotFound, "DELETE", "/members/00000000-0000-0000-0000-000000000000", "")
	a.as(a.guestID, http.StatusOK, "DELETE", "/members/"+a.guestID, "")
	a.as(a.guestID, http.StatusNotFound, "GET", "/members", "")

	a.as(a.ownerID, http.StatusCreated, "POST", "/members", `{"email":"`+a.guest+`","role":"EDITOR"}`)
	a.as(a.ownerID, http.StatusOK, "DELETE", "/members/"+a.guestID, "")
}

func TestVehiclePeopleLifecycle(t *testing.T) {
	a := newMemberAPI(t, "people")

	a.as(a.ownerID, http.StatusOK, "GET", "/people", "")
	a.as(a.ownerID, http.StatusCreated, "POST", "/members", `{"email":"`+a.guest+`","role":"EDITOR"}`)

	a.as(a.ownerID, http.StatusBadRequest, "POST", "/people", `{`)
	a.as(a.guestID, http.StatusForbidden, "POST", "/people", `{"name":"Zoe"}`)
	person := decodeObj(t, a.as(a.ownerID, http.StatusCreated, "POST", "/people", `{"name":"Zoe"}`))
	pid := person["id"].(string)
	if person["name"] != "Zoe" {
		t.Fatalf("person: %v", person)
	}
	a.as(a.guestID, http.StatusOK, "GET", "/people", "")

	a.as(a.ownerID, http.StatusBadRequest, "PUT", "/people/"+pid, `{`)
	renamed := decodeObj(t, a.as(a.ownerID, http.StatusOK, "PUT", "/people/"+pid, `{"name":"Zoé"}`))
	if renamed["name"] != "Zoé" {
		t.Fatalf("rename: %v", renamed)
	}
	a.as(a.ownerID, http.StatusNotFound, "PUT", "/people/00000000-0000-0000-0000-000000000000", `{"name":"Ghost"}`)

	a.as(a.ownerID, http.StatusBadRequest, "PUT", "/people/"+pid+"/link", `{}`)
	a.as(a.ownerID, http.StatusBadRequest, "PUT", "/people/"+pid+"/link", `{`)
	linked := decodeObj(t, a.as(a.ownerID, http.StatusOK, "PUT", "/people/"+pid+"/link", `{"user_id":"`+a.guestID+`"}`))
	if linked["user_id"] != a.guestID {
		t.Fatalf("link: %v", linked)
	}

	a.as(a.ownerID, http.StatusOK, "PUT", "/people/"+pid+"/default", "")
	a.as(a.ownerID, http.StatusNotFound, "PUT", "/people/00000000-0000-0000-0000-000000000000/default", "")

	a.as(a.guestID, http.StatusForbidden, "DELETE", "/people/"+pid, "")
	a.as(a.ownerID, http.StatusBadRequest, "DELETE", "/people/"+pid, "")
	other := decodeObj(t, a.as(a.ownerID, http.StatusCreated, "POST", "/people", `{"name":"Max"}`))["id"].(string)
	a.as(a.ownerID, http.StatusNoContent, "DELETE", "/people/"+other, "")
	a.as(a.ownerID, http.StatusNotFound, "DELETE", "/people/"+other, "")
}
