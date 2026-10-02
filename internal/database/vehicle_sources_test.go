package database

import (
	"context"
	"os"
	"sort"
	"strings"
	"testing"

	neturl "net/url"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/teslacost/teslacost/internal/models"
	"github.com/teslacost/teslacost/migrations"
)

func sourcesTestPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	base := os.Getenv("TEST_DATABASE_URL")
	if base == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	ctx := context.Background()
	u, err := neturl.Parse(base)
	if err != nil {
		t.Fatal(err)
	}
	name := strings.TrimPrefix(u.Path, "/") + "_dbsources"
	admin, err := pgxpool.New(ctx, base)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	var exists bool
	if err := admin.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_database WHERE datname = $1)`, name).Scan(&exists); err != nil {
		t.Fatal(err)
	}
	if !exists {
		if _, err := admin.Exec(ctx, `CREATE DATABASE "`+name+`"`); err != nil {
			t.Fatal(err)
		}
	}
	u.Path = "/" + name
	pool, err := pgxpool.New(ctx, u.String())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	if _, err := pool.Exec(ctx, `DROP SCHEMA public CASCADE; CREATE SCHEMA public;`); err != nil {
		t.Fatal(err)
	}
	return pool
}

// migrateBefore applies the embedded migrations that come before the given prefix and records them.
func migrateBefore(t *testing.T, pool *pgxpool.Pool, prefix string) {
	t.Helper()
	ctx := context.Background()
	entries, err := migrations.FS.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	var ups []string
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".up.sql") && e.Name() < prefix {
			ups = append(ups, e.Name())
		}
	}
	sort.Strings(ups)
	if _, err := pool.Exec(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (version VARCHAR(255) PRIMARY KEY, applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW())`); err != nil {
		t.Fatal(err)
	}
	for _, f := range ups {
		sqlText, err := migrations.FS.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := pool.Exec(ctx, string(sqlText)); err != nil {
			t.Fatalf("%s: %v", f, err)
		}
		if _, err := pool.Exec(ctx, `INSERT INTO schema_migrations (version) VALUES ($1)`, strings.TrimSuffix(f, ".up.sql")); err != nil {
			t.Fatal(err)
		}
	}
}

func TestVehicleSourcesBackfillAndFacade(t *testing.T) {
	pool := sourcesTestPool(t)
	ctx := context.Background()
	migrateBefore(t, pool, "000045")

	var userID string
	if err := pool.QueryRow(ctx, `INSERT INTO users (email, password_hash) VALUES ('src@example.com', 'x') RETURNING id::text`).Scan(&userID); err != nil {
		t.Fatal(err)
	}
	insert := func(name, cols, vals string) string {
		t.Helper()
		var id string
		q := `INSERT INTO vehicles (user_id, name` + cols + `) VALUES ($1, $2` + vals + `) RETURNING id::text`
		if err := pool.QueryRow(ctx, q, userID, name).Scan(&id); err != nil {
			t.Fatal(err)
		}
		return id
	}
	bearer := insert("Bearer", `, teslamate_car_id, teslamate_api_url, teslamate_auth_type, teslamate_api_key_encrypted, teslamate_grafana_url`,
		`, 1, 'http://tm:8080', 'BEARER', 'cipher-key', 'http://grafana'`)
	basic := insert("Basic", `, teslamate_car_id, teslamate_api_url, teslamate_auth_type, teslamate_api_key_encrypted, teslamate_basic_user, teslamate_basic_pass_encrypted`,
		`, 2, 'http://tm2:8080', 'BASIC', 'old-key', 'bob', 'cipher-pass'`)
	grafanaOnly := insert("Grafana only", `, teslamate_grafana_url`, `, 'http://g2'`)
	manual := insert("Manual", ``, ``)

	if err := (&DB{Pool: pool}).Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	repo := NewRepository(pool)

	got, err := repo.GetVehicleByIDInternal(ctx, bearer)
	if err != nil {
		t.Fatal(err)
	}
	if got.TeslaMateAPIURL == nil || *got.TeslaMateAPIURL != "http://tm:8080" || got.TeslaMateAuthType != models.AuthModeBearer ||
		got.TeslaMateCarID == nil || *got.TeslaMateCarID != 1 || got.TeslaMateAPIKeyEncrypted == nil || *got.TeslaMateAPIKeyEncrypted != "cipher-key" ||
		got.TeslaMateGrafanaURL == nil || *got.TeslaMateGrafanaURL != "http://grafana" {
		t.Errorf("bearer vehicle lost data in the backfill: %+v", got)
	}

	got, err = repo.GetVehicleByIDInternal(ctx, basic)
	if err != nil {
		t.Fatal(err)
	}
	if got.TeslaMateAuthType != models.AuthModeBasic || got.TeslaMateBasicUser == nil || *got.TeslaMateBasicUser != "bob" ||
		got.TeslaMateBasicPassEnc == nil || *got.TeslaMateBasicPassEnc != "cipher-pass" ||
		got.TeslaMateAPIKeyEncrypted == nil || *got.TeslaMateAPIKeyEncrypted != "old-key" {
		t.Errorf("basic vehicle lost data in the backfill: %+v", got)
	}

	got, err = repo.GetVehicleByIDInternal(ctx, grafanaOnly)
	if err != nil {
		t.Fatal(err)
	}
	if got.TeslaMateGrafanaURL == nil || *got.TeslaMateGrafanaURL != "http://g2" || got.TeslaMateAPIURL != nil || got.TeslaMateAuthType != models.AuthModeNone {
		t.Errorf("grafana-only vehicle changed in the backfill: %+v", got)
	}

	got, err = repo.GetVehicleByIDInternal(ctx, manual)
	if err != nil {
		t.Fatal(err)
	}
	if got.TeslaMateAPIURL != nil || got.TeslaMateCarID != nil || got.TeslaMateAuthType != models.AuthModeNone {
		t.Errorf("manual vehicle gained TeslaMate settings: %+v", got)
	}
	var sources int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM vehicle_data_sources`).Scan(&sources); err != nil {
		t.Fatal(err)
	}
	if sources != 3 {
		t.Errorf("sources after the backfill: got %d, want 3 (the manual vehicle has none)", sources)
	}

	all, err := repo.ListAllVehiclesWithTeslaMate(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 2 {
		t.Errorf("vehicles synchronized: got %d, want the 2 with an API URL", len(all))
	}

	// Writes go to the source and to the legacy columns alike.
	v, _ := repo.GetVehicleByIDInternal(ctx, bearer)
	newURL := "http://moved:9000"
	v.TeslaMateAPIURL = &newURL
	if err := repo.UpdateVehicle(ctx, v); err != nil {
		t.Fatal(err)
	}
	var legacy string
	if err := pool.QueryRow(ctx, `SELECT teslamate_api_url FROM vehicles WHERE id::text = $1`, bearer).Scan(&legacy); err != nil || legacy != newURL {
		t.Errorf("legacy column after update: %q %v", legacy, err)
	}
	if again, _ := repo.GetVehicleByIDInternal(ctx, bearer); again.TeslaMateAPIURL == nil || *again.TeslaMateAPIURL != newURL {
		t.Errorf("source after update: %+v", again.TeslaMateAPIURL)
	}

	// Clearing every TeslaMate setting removes the source.
	v.TeslaMateAPIURL, v.TeslaMateGrafanaURL, v.TeslaMateCarID = nil, nil, nil
	v.TeslaMateAPIKeyEncrypted, v.TeslaMateAuthType = nil, models.AuthModeNone
	if err := repo.UpdateVehicle(ctx, v); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM vehicle_data_sources WHERE vehicle_id::text = $1`, bearer).Scan(&sources); err != nil || sources != 0 {
		t.Errorf("source after clearing the settings: %d %v", sources, err)
	}

	// Migrating again changes nothing.
	if err := (&DB{Pool: pool}).Migrate(ctx); err != nil {
		t.Fatal(err)
	}
}
