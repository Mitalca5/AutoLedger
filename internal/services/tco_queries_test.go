package services

import (
	"context"
	"os"
	"reflect"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/teslacost/teslacost/internal/database"
	"github.com/teslacost/teslacost/internal/models"
)

func TestDriveDistanceIndexSumsTheDrivesOfAHalfOpenInterval(t *testing.T) {
	base := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	at := func(h int) time.Time { return base.Add(time.Duration(h) * time.Hour) }
	idx := &driveDistanceIndex{
		starts: []time.Time{at(0), at(2), at(2), at(5), at(9)},
		km:     []float64{10, 20, 5, 40, 80},
		synced: []bool{true, false, true, true, false},
	}
	for _, c := range []struct {
		name       string
		from, to   time.Time
		tracked    float64
		syncedOnly float64
	}{
		{"from is inclusive, to is exclusive", at(2), at(5), 25, 5},
		{"the whole history", at(-1), at(10), 155, 55},
		{"an interval with no drive", at(6), at(8), 0, 0},
		{"a single instant", at(5), at(5), 0, 0},
		{"before the first drive", at(-5), at(0), 0, 0},
		{"ends right after the last start", at(9), at(10), 80, 0},
	} {
		gotTracked, gotSynced := idx.between(c.from, c.to)
		if gotTracked != c.tracked || gotSynced != c.syncedOnly {
			t.Errorf("%s: got %v / %v, want %v / %v", c.name, gotTracked, gotSynced, c.tracked, c.syncedOnly)
		}
	}
	empty := &driveDistanceIndex{}
	if a, b := empty.between(at(0), at(5)); a != 0 || b != 0 {
		t.Errorf("no drives: got %v / %v", a, b)
	}
}

// queryCounter counts the statements a pool sends, to keep the number of queries of a TCO independent of the history.
type queryCounter struct{ n atomic.Int64 }

func (c *queryCounter) TraceQueryStart(ctx context.Context, _ *pgx.Conn, _ pgx.TraceQueryStartData) context.Context {
	c.n.Add(1)
	return ctx
}

func (c *queryCounter) TraceQueryEnd(context.Context, *pgx.Conn, pgx.TraceQueryEndData) {}

func countingTCOService(t *testing.T) (*TCOService, *queryCounter) {
	t.Helper()
	cfg, err := pgxpool.ParseConfig(os.Getenv("TEST_DATABASE_URL"))
	if err != nil {
		t.Fatal(err)
	}
	counter := &queryCounter{}
	cfg.ConnConfig.Tracer = counter
	pool, err := pgxpool.NewWithConfig(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	return NewTCOService(pool, "Europe/Paris"), counter
}

func addCheckpoints(t *testing.T, repo *database.Repository, vehicleID string, from, to int) {
	t.Helper()
	now := time.Now()
	for i := from; i < to; i++ {
		cp := &models.OdometerCheckpoint{VehicleID: vehicleID, Date: now.AddDate(0, -(30 - i), 0), Odometer: float64(1000 * (i + 1)), Source: "MANUAL"}
		if err := repo.CreateOdometerCheckpoint(context.Background(), cp); err != nil {
			t.Fatal(err)
		}
	}
}

func TestIntegrationTCOQueryCountDoesNotGrowWithTheOdometerHistory(t *testing.T) {
	db, repo := setupIntegrationDB(t, false)
	ctx := context.Background()
	v := mustVehicle(t, repo, "query-count@example.com")
	_ = db
	svc, counter := countingTCOService(t)

	addCheckpoints(t, repo, v.ID, 0, 3)
	mustDrive(t, repo, v.ID, 1, time.Now().AddDate(0, -28, 0), 1500, 40)
	counter.n.Store(0)
	if _, err := svc.ComputeVehicleTCO(ctx, v.ID); err != nil {
		t.Fatal(err)
	}
	few := counter.n.Load()

	addCheckpoints(t, repo, v.ID, 3, 20)
	counter.n.Store(0)
	if _, err := svc.ComputeVehicleTCO(ctx, v.ID); err != nil {
		t.Fatal(err)
	}
	many := counter.n.Load()

	if few == 0 || many != few {
		t.Fatalf("queries per TCO: %d with 3 readings, %d with 20 readings; the count must not depend on the history", few, many)
	}
}

func TestIntegrationMonthlyTimelineMatchesTheTCOAndSkipsTheRest(t *testing.T) {
	_, repo := setupIntegrationDB(t, false)
	ctx := context.Background()
	v := mustVehicle(t, repo, "timeline@example.com")
	svc, counter := countingTCOService(t)

	addCheckpoints(t, repo, v.ID, 0, 8)
	mustDrive(t, repo, v.ID, 1, time.Now().AddDate(0, -20, 0), 11000, 120)
	mustDrive(t, repo, v.ID, 2, time.Now().AddDate(0, -12, 0), 19000, 60)

	counter.n.Store(0)
	sum, err := svc.ComputeVehicleTCO(ctx, v.ID)
	if err != nil {
		t.Fatal(err)
	}
	full := counter.n.Load()

	counter.n.Store(0)
	timeline, err := svc.monthlyTimeline(ctx, v.ID)
	if err != nil {
		t.Fatal(err)
	}
	light := counter.n.Load()

	if !reflect.DeepEqual(timeline, sum.MonthlyCosts) {
		t.Fatalf("the drill-down timeline differs from the TCO months:\n%+v\n%+v", timeline, sum.MonthlyCosts)
	}
	if light >= full {
		t.Fatalf("the drill-down must not recompute the whole TCO: %d queries against %d", light, full)
	}
}
