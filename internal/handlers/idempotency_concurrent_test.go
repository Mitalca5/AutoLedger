package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/teslacost/teslacost/internal/middleware"
	"github.com/teslacost/teslacost/internal/models"
	"github.com/teslacost/teslacost/internal/money"
)

func TestIdempotencyConcurrentRetriesPersistOneCharge(t *testing.T) {
	repo := authTestRepo(t)
	ctx := context.Background()
	user, err := repo.CreateUser(ctx, "idem-concurrent@example.org", "hash")
	if err != nil {
		t.Fatal(err)
	}
	vehicle := &models.Vehicle{UserID: user.ID, Name: "Test car", Powertrain: models.PowertrainEV, Currency: "EUR"}
	if err := repo.CreateVehicle(ctx, vehicle); err != nil {
		t.Fatal(err)
	}
	var calls atomic.Int32
	entered, release := make(chan struct{}, 2), make(chan struct{})
	wrapped := Idempotency(repo)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		select {
		case entered <- struct{}{}:
		default:
		}
		<-release
		cost := money.FromFloat(5)
		charge := &models.ChargeLog{VehicleID: vehicle.ID, Date: time.Now(), KwhAdded: 20, Cost: &cost, Currency: "EUR"}
		if err := repo.CreateManualCharge(r.Context(), charge); err != nil {
			writeError(w, 500, err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, charge)
	}))
	const concurrent = 8
	responses := make([]*httptest.ResponseRecorder, concurrent)
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := range responses {
		responses[i] = httptest.NewRecorder()
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			req := httptest.NewRequest("POST", "/api/vehicles/"+vehicle.ID+"/charges", nil)
			req.Header.Set("Idempotency-Key", "same-offline-entry")
			req = req.WithContext(context.WithValue(ctx, middleware.UserIDKey, user.ID))
			wrapped.ServeHTTP(responses[i], req)
		}(i)
	}
	close(start)
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		close(release)
		wg.Wait()
		t.Fatal("handler did not start")
	}
	// Keep the first mutation in flight while the retries attempt to enter.
	select {
	case <-entered:
		t.Error("a concurrent retry entered the handler")
	case <-time.After(100 * time.Millisecond):
	}
	close(release)
	wg.Wait()
	_, count, err := repo.ListCharges(ctx, vehicle.ID, false, 10, 0)
	if err != nil || count != 1 || calls.Load() != 1 {
		t.Fatalf("count=%d, calls=%d, err=%v", count, calls.Load(), err)
	}
	for _, response := range responses {
		if response.Code != http.StatusCreated || response.Body.String() != responses[0].Body.String() {
			t.Fatalf("inconsistent replay: %d %s", response.Code, response.Body.String())
		}
	}
}

func TestIdempotencyLocksScopeCancellationAndCleanup(t *testing.T) {
	var locks idempotencyLocks
	key := idempotencyRequestKey{userID: "u1", key: "k1"}
	release, err := locks.acquire(context.Background(), key)
	if err != nil {
		t.Fatal(err)
	}
	for _, independent := range []idempotencyRequestKey{{userID: "u2", key: "k1"}, {userID: "u1", key: "k2"}} {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		unlock, err := locks.acquire(ctx, independent)
		cancel()
		if err != nil {
			t.Fatal(err)
		}
		unlock()
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := locks.acquire(ctx, key); err != context.Canceled {
		t.Fatalf("err=%v", err)
	}
	release()
	if len(locks.entries) != 0 {
		t.Fatalf("idle locks retained: %d", len(locks.entries))
	}
}
