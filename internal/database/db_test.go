package database

import (
	"bytes"
	"context"
	"log/slog"
	"regexp"
	"strings"
	"testing"
	"time"
)

// port1 is reserved and nothing ever listens there: dialing it fails immediately (connection refused) instead of
// timing out, keeping these tests fast regardless of the environment they run in.
const unreachableDSN = "postgres://user:pass@127.0.0.1:1/db?sslmode=disable&connect_timeout=1"

func TestConnectGivesUpWhenContextExpires(t *testing.T) {
	withFastRetries(t, func() {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Millisecond)
		defer cancel()

		start := time.Now()
		_, err := Connect(ctx, unreachableDSN)
		elapsed := time.Since(start)

		if err == nil {
			t.Fatal("expected an error: nothing listens on port 1")
		}
		// Returns close to the deadline, not after a long fixed wait swallowing the context cancellation.
		if elapsed > 500*time.Millisecond {
			t.Fatalf("Connect took %s to give up after a 60ms deadline", elapsed)
		}
	})
}

func TestConnectRetriesUntilItGivesUp(t *testing.T) {
	withFastRetries(t, func() {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Millisecond)
		defer cancel()

		_, err := Connect(ctx, unreachableDSN)
		if err == nil {
			t.Fatal("expected an error")
		}

		m := regexp.MustCompile(`after (\d+) attempt`).FindStringSubmatch(err.Error())
		if m == nil {
			t.Fatalf("expected the error to report an attempt count, got: %v", err)
		}
		if m[1] == "1" {
			t.Fatalf("expected more than one attempt within the deadline, got: %v", err)
		}
	})
}

// withFastRetries lowers the pause between two connection attempts for the duration of fn, so tests exercising the
// retry loop do not each take connectRetryInterval (3s) per attempt.
func withFastRetries(t *testing.T, fn func()) {
	t.Helper()
	original := connectRetryInterval
	connectRetryInterval = time.Millisecond
	t.Cleanup(func() { connectRetryInterval = original })
	fn()
}

func TestOutdatedServerWarning(t *testing.T) {
	if msg := OutdatedServerWarning(180000); msg != "" {
		t.Fatalf("PostgreSQL 18 is supported, got %q", msg)
	}
	if msg := OutdatedServerWarning(190001); msg != "" {
		t.Fatalf("a newer major is not outdated, got %q", msg)
	}
	msg := OutdatedServerWarning(160004)
	if !strings.Contains(msg, "PostgreSQL 16") || !strings.Contains(msg, "docker-compose.yml") {
		t.Fatalf("an older major must point to the compose file, got %q", msg)
	}
}

func TestWarnIfServerOutdated(t *testing.T) {
	var logs bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logs, nil)))
	t.Cleanup(func() { slog.SetDefault(previous) })

	// A server that cannot be queried is never reported: the check must not fail startup.
	closed := sourcesTestPool(t)
	closed.Close()
	(&DB{Pool: closed}).WarnIfServerOutdated(context.Background())
	if logs.Len() != 0 {
		t.Fatalf("an unreachable server must stay silent, got %q", logs.String())
	}

	// The test server runs the supported major or a newer one, so it stays silent too.
	(&DB{Pool: sourcesTestPool(t)}).WarnIfServerOutdated(context.Background())
	var num int
	if err := sourcesTestPool(t).QueryRow(context.Background(), "SELECT current_setting('server_version_num')::int").Scan(&num); err != nil {
		t.Fatal(err)
	}
	if outdated := num/10000 < SupportedServerMajor; outdated != (logs.Len() > 0) {
		t.Fatalf("server_version_num %d: warning logged = %v, want %v", num, logs.Len() > 0, outdated)
	}
}
