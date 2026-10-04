package geocode

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func newTestClient(t *testing.T, handler http.HandlerFunc) (*Client, *atomic.Int32) {
	t.Helper()
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		handler(w, r)
	}))
	t.Cleanup(srv.Close)
	c := New(srv.URL+"/", "test-agent")
	c.interval = time.Millisecond
	return c, &calls
}

func TestReverseBuildsShortAddressAndCaches(t *testing.T) {
	var gotAgent, gotLat string
	c, calls := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotAgent = r.Header.Get("User-Agent")
		gotLat = r.URL.Query().Get("lat")
		_, _ = w.Write([]byte(`{"display_name":"long name","address":{"house_number":"12","road":"Rue Example","city":"Annecy"}}`))
	})

	addr, err := c.Reverse(context.Background(), 45.89921, 6.12941)
	if err != nil || addr != "12 Rue Example, Annecy" {
		t.Fatalf("got %q, %v", addr, err)
	}
	if gotAgent != "test-agent" || gotLat != "45.899210" {
		t.Errorf("request: agent %q, lat %q", gotAgent, gotLat)
	}
	// About 5 m away: same cache cell, no second request.
	if addr, _ := c.Reverse(context.Background(), 45.89924, 6.12944); addr != "12 Rue Example, Annecy" {
		t.Errorf("cached: got %q", addr)
	}
	if calls.Load() != 1 {
		t.Errorf("requests: got %d, want 1", calls.Load())
	}
	// Far away: a new request.
	if _, err := c.Reverse(context.Background(), 45.7, 4.8); err != nil || calls.Load() != 2 {
		t.Errorf("far position: err %v, requests %d", err, calls.Load())
	}
}

func TestLabel(t *testing.T) {
	cases := map[string]reverseResponse{
		"Rue Example, Annecy": {Address: map[string]string{"road": "Rue Example", "town": "Annecy"}},
		"Rue Example":         {Address: map[string]string{"road": "Rue Example"}},
		"Gare, Lyon":          {Address: map[string]string{"amenity": "Gare", "city": "Lyon"}},
		"Lyon":                {Address: map[string]string{"city": "Lyon"}},
		"Somewhere, far away": {DisplayName: " Somewhere, far away "},
		"":                    {},
	}
	for want, in := range cases {
		if got := label(in); got != want {
			t.Errorf("%+v: got %q, want %q", in, got, want)
		}
	}
}

func TestReverseFailuresAreNotCached(t *testing.T) {
	status := http.StatusInternalServerError
	c, calls := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if status != http.StatusOK {
			w.WriteHeader(status)
			return
		}
		_, _ = w.Write([]byte(`{"address":{"road":"Rue Example","city":"Annecy"}}`))
	})
	if _, err := c.Reverse(context.Background(), 45.9, 6.1); err == nil {
		t.Error("a 500 must be an error")
	}
	status = http.StatusOK
	if addr, err := c.Reverse(context.Background(), 45.9, 6.1); err != nil || addr != "Rue Example, Annecy" {
		t.Errorf("after recovery: %q, %v", addr, err)
	}
	if calls.Load() != 2 {
		t.Errorf("requests: got %d, want 2", calls.Load())
	}
}

func TestReverseNoAddressAndBadInput(t *testing.T) {
	c, calls := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"error":"Unable to geocode"}`))
	})
	if _, err := c.Reverse(context.Background(), 0, 0); err == nil {
		t.Error("an empty answer must be an error")
	}
	if _, err := c.Reverse(context.Background(), 91, 0); err == nil {
		t.Error("latitude out of range must be refused")
	}
	if calls.Load() != 1 {
		t.Errorf("requests: got %d, want 1 (invalid input never leaves the process)", calls.Load())
	}
}

func TestReverseSpacesRequests(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"address":{"city":"Annecy"}}`))
	})
	c.interval = 60 * time.Millisecond
	start := time.Now()
	for i := range 3 {
		if _, err := c.Reverse(context.Background(), 45.0+float64(i), 6.0); err != nil {
			t.Fatal(err)
		}
	}
	if elapsed := time.Since(start); elapsed < 110*time.Millisecond {
		t.Errorf("three requests took %v, want at least two intervals", elapsed)
	}
}

func TestReverseHonoursCancellation(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"address":{"city":"Annecy"}}`))
	})
	c.interval = time.Hour
	_, _ = c.Reverse(context.Background(), 1, 1)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if _, err := c.Reverse(ctx, 2, 2); err == nil {
		t.Error("a waiting lookup must stop with its context")
	}
}
