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

func TestVehicleSourcesBackfill(t *testing.T) {
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

	// Updates are written to the source.
	v, _ := repo.GetVehicleByIDInternal(ctx, bearer)
	newURL := "http://moved:9000"
	v.TeslaMateAPIURL = &newURL
	if err := repo.UpdateVehicle(ctx, v); err != nil {
		t.Fatal(err)
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

func TestDropLegacyTeslaMateColumns(t *testing.T) {
	pool := sourcesTestPool(t)
	ctx := context.Background()
	migrateBefore(t, pool, "000049")

	var userID string
	if err := pool.QueryRow(ctx, `INSERT INTO users (email, password_hash) VALUES ('drop@example.com', 'x') RETURNING id::text`).Scan(&userID); err != nil {
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
	// Written by the previous version after the sources existed: the columns are ahead of the source.
	moved := insert("Moved", `, teslamate_car_id, teslamate_api_url, teslamate_auth_type, teslamate_api_key_encrypted, teslamate_grafana_url`,
		`, 1, 'http://old:8080', 'BEARER', 'cipher', 'http://grafana'`)
	cleared := insert("Cleared", `, teslamate_car_id, teslamate_api_url`, `, 2, 'http://gone:8080'`)
	basic := insert("Basic", `, teslamate_car_id, teslamate_api_url, teslamate_auth_type, teslamate_basic_user, teslamate_basic_pass_encrypted`,
		`, 3, 'http://tm3:8080', 'BASIC', 'bob', 'cipher-pass'`)
	manual := insert("Manual", ``, ``)

	// The sources as the previous version left them: one stale, one that should no longer exist.
	if _, err := pool.Exec(ctx, `
		INSERT INTO vehicle_data_sources (vehicle_id, provider, config) VALUES
		  ($1, 'TESLAMATE', '{"api_url":"http://stale:1","car_id":9}'),
		  ($2, 'TESLAMATE', '{"api_url":"http://gone:8080","car_id":2}')`, moved, cleared); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `UPDATE vehicles SET teslamate_api_url = NULL, teslamate_car_id = NULL WHERE id::text = $1`, cleared); err != nil {
		t.Fatal(err)
	}

	if err := (&DB{Pool: pool}).Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	repo := NewRepository(pool)

	var columns int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM information_schema.columns WHERE table_name = 'vehicles' AND column_name LIKE 'teslamate_%'`).Scan(&columns); err != nil || columns != 0 {
		t.Fatalf("legacy columns left on vehicles: %d %v", columns, err)
	}

	got, err := repo.GetVehicleByIDInternal(ctx, moved)
	if err != nil {
		t.Fatal(err)
	}
	if got.TeslaMateAPIURL == nil || *got.TeslaMateAPIURL != "http://old:8080" || got.TeslaMateCarID == nil || *got.TeslaMateCarID != 1 ||
		got.TeslaMateAuthType != models.AuthModeBearer || got.TeslaMateAPIKeyEncrypted == nil || *got.TeslaMateAPIKeyEncrypted != "cipher" ||
		got.TeslaMateGrafanaURL == nil || *got.TeslaMateGrafanaURL != "http://grafana" {
		t.Errorf("the columns did not win over the stale source: %+v", got)
	}
	got, err = repo.GetVehicleByIDInternal(ctx, cleared)
	if err != nil {
		t.Fatal(err)
	}
	if got.TeslaMateAPIURL != nil || got.TeslaMateCarID != nil {
		t.Errorf("a connection cleared before the upgrade came back: %+v", got)
	}
	got, err = repo.GetVehicleByIDInternal(ctx, basic)
	if err != nil {
		t.Fatal(err)
	}
	if got.TeslaMateAuthType != models.AuthModeBasic || got.TeslaMateBasicUser == nil || *got.TeslaMateBasicUser != "bob" ||
		got.TeslaMateBasicPassEnc == nil || *got.TeslaMateBasicPassEnc != "cipher-pass" || got.TeslaMateCarID == nil || *got.TeslaMateCarID != 3 {
		t.Errorf("a vehicle without a source before the upgrade lost its connection: %+v", got)
	}
	got, err = repo.GetVehicleByIDInternal(ctx, manual)
	if err != nil {
		t.Fatal(err)
	}
	if got.TeslaMateAPIURL != nil || got.TeslaMateAuthType != models.AuthModeNone {
		t.Errorf("manual vehicle gained a connection: %+v", got)
	}

	// A new vehicle gets the next free car id from the sources.
	nv := &models.Vehicle{UserID: userID, Name: "Next", Powertrain: models.PowertrainEV}
	url := "http://next:8080"
	nv.TeslaMateAPIURL = &url
	if err := repo.CreateVehicle(ctx, nv); err != nil {
		t.Fatal(err)
	}
	if nv.TeslaMateCarID == nil || *nv.TeslaMateCarID != 4 {
		t.Errorf("next car id: %v, want 4", nv.TeslaMateCarID)
	}

	// Rolling the schema back restores the columns from the sources.
	down, err := migrations.FS.ReadFile("000049_drop_legacy_teslamate_columns.down.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, string(down)); err != nil {
		t.Fatal(err)
	}
	var apiURL string
	var carID int
	if err := pool.QueryRow(ctx, `SELECT teslamate_api_url, teslamate_car_id FROM vehicles WHERE id::text = $1`, basic).Scan(&apiURL, &carID); err != nil || apiURL != "http://tm3:8080" || carID != 3 {
		t.Errorf("columns after the rollback: %q %d %v", apiURL, carID, err)
	}
}
