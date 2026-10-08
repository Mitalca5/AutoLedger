package database

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/teslacost/teslacost/migrations"
)

// DB encapsulates the pgx connection pool.
type DB struct {
	Pool *pgxpool.Pool
}

// SupportedServerMajor is the PostgreSQL major version the docker-compose.yml of this release runs.
const SupportedServerMajor = 18

// UnreachableHint tells an operator what to check when PostgreSQL stays unreachable: after a PostgreSQL major
// upgrade, a docker-compose.yml that predates the db-upgrade and db-restore services leaves the database container
// crashing on the old data directory.
const UnreachableHint = "if the postgres container keeps stopping (docker compose logs postgres), your docker-compose.yml is probably " +
	"out of date: a database volume written by an older PostgreSQL major needs the db-upgrade and db-restore services " +
	"of the current docker-compose.yml (see the README, section \"Upgrading PostgreSQL 16 to 18\")"

// connectRetryInterval is the pause between two attempts to reach PostgreSQL.
var connectRetryInterval = 3 * time.Second

// Connect initializes a connection pool to PostgreSQL, retrying at a fixed interval until it succeeds or ctx is
// done. Give ctx a deadline covering how long a deployment can tolerate waiting for PostgreSQL to become reachable:
// after an unclean shutdown (e.g. a power outage), PostgreSQL's own crash recovery (WAL replay, fsync of the data
// directory) can take minutes on a database of any real size, well past a container's first few restart attempts.
func Connect(ctx context.Context, databaseURL string) (*DB, error) {
	config, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("unable to parse database config: %w", err)
	}

	config.MaxConns = 25
	config.MinConns = 2
	config.MaxConnLifetime = 1 * time.Hour
	config.MaxConnIdleTime = 30 * time.Minute

	start := time.Now()
	var lastErr error
	for attempt := 1; ; attempt++ {
		pool, err := dialAndPing(ctx, config)
		if err == nil {
			slog.Info("connected successfully to PostgreSQL", "component", "database", "attempt", attempt, "waited", time.Since(start).Round(time.Second))
			return &DB{Pool: pool}, nil
		}
		lastErr = err

		// Logged on the first attempt and then roughly every 30s, so a long wait (crash recovery) does not flood
		// the log with one line every connectRetryInterval.
		if attempt == 1 || attempt%10 == 0 {
			slog.Warn("waiting for PostgreSQL to be ready", "component", "database", "attempt", attempt, "waited", time.Since(start).Round(time.Second), "error", err, "hint", UnreachableHint)
		}

		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("database unreachable after %d attempt(s) over %s: %w", attempt, time.Since(start).Round(time.Second), lastErr)
		case <-time.After(connectRetryInterval):
		}
	}
}

// OutdatedServerWarning returns the message to log when the server runs an older major than SupportedServerMajor,
// or "" when it is up to date. serverVersionNum is PostgreSQL's server_version_num (e.g. 160004).
func OutdatedServerWarning(serverVersionNum int) string {
	major := serverVersionNum / 10000
	if major >= SupportedServerMajor {
		return ""
	}
	return fmt.Sprintf("PostgreSQL %d is running but this release targets PostgreSQL %d: update your docker-compose.yml "+
		"(it upgrades the database volume automatically with a backup first, see the README)", major, SupportedServerMajor)
}

// WarnIfServerOutdated logs a warning when PostgreSQL is older than the supported major. It never fails startup:
// an older server keeps working.
func (db *DB) WarnIfServerOutdated(ctx context.Context) {
	var num int
	if err := db.Pool.QueryRow(ctx, "SELECT current_setting('server_version_num')::int").Scan(&num); err != nil {
		return
	}
	if msg := OutdatedServerWarning(num); msg != "" {
		slog.Warn(msg, "component", "database", "server_version_num", num)
	}
}

// dialAndPing opens a pool and confirms PostgreSQL actually answers, closing the pool on any failure.
func dialAndPing(ctx context.Context, config *pgxpool.Config) (*pgxpool.Pool, error) {
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, err
	}
	pingCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, err
	}
	return pool, nil
}

// migrationLockID is the advisory lock key serializing concurrent migration runs.
const migrationLockID = 7474_2026

// Migrate applies pending embedded SQL migrations, each one inside its own transaction,
// and records applied versions in schema_migrations.
func (db *DB) Migrate(ctx context.Context) error {
	entries, err := migrations.FS.ReadDir(".")
	if err != nil {
		return fmt.Errorf("failed to read embedded migrations directory: %w", err)
	}

	var upFiles []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".up.sql") {
			upFiles = append(upFiles, entry.Name())
		}
	}
	sort.Strings(upFiles)

	conn, err := db.Pool.Acquire(ctx)
	if err != nil {
		return fmt.Errorf("failed to acquire connection for migrations: %w", err)
	}
	defer conn.Release()

	if _, err := conn.Exec(ctx, `SELECT pg_advisory_lock($1)`, migrationLockID); err != nil {
		return fmt.Errorf("failed to acquire migration lock: %w", err)
	}
	defer conn.Exec(ctx, `SELECT pg_advisory_unlock($1)`, migrationLockID)

	if _, err := conn.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version VARCHAR(255) PRIMARY KEY,
			applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);
	`); err != nil {
		return fmt.Errorf("failed to create schema_migrations table: %w", err)
	}

	applied := make(map[string]bool)
	rows, err := conn.Query(ctx, `SELECT version FROM schema_migrations`)
	if err != nil {
		return fmt.Errorf("failed to read applied migrations: %w", err)
	}
	for rows.Next() {
		var v string
		if err := rows.Scan(&v); err != nil {
			rows.Close()
			return err
		}
		applied[v] = true
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}

	for _, file := range upFiles {
		version := strings.TrimSuffix(file, ".up.sql")
		if applied[version] {
			continue
		}

		schemaSQL, err := migrations.FS.ReadFile(file)
		if err != nil {
			return fmt.Errorf("failed to read embedded schema migration %s: %w", file, err)
		}

		tx, err := conn.Begin(ctx)
		if err != nil {
			return fmt.Errorf("failed to begin migration %s: %w", file, err)
		}
		if _, err := tx.Exec(ctx, string(schemaSQL)); err != nil {
			_ = tx.Rollback(ctx)
			return fmt.Errorf("failed to execute schema migration %s: %w", file, err)
		}
		if _, err := tx.Exec(ctx, `INSERT INTO schema_migrations (version) VALUES ($1)`, version); err != nil {
			_ = tx.Rollback(ctx)
			return fmt.Errorf("failed to record migration %s: %w", file, err)
		}
		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("failed to commit migration %s: %w", file, err)
		}
		slog.Info("applied migration", "component", "database", "file", file)
	}

	slog.Info("schema is up-to-date", "component", "database")
	return nil
}

// Close closes the connection pool.
func (db *DB) Close() {
	if db.Pool != nil {
		db.Pool.Close()
		slog.Info("connection pool closed", "component", "database")
	}
}
