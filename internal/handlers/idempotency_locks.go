package handlers

import (
	"context"
	"sync"
)

type idempotencyRequestKey struct {
	userID string
	key    string
}

type idempotencyLock struct {
	available chan struct{}
	refs      int
}

// idempotencyLocks serializes matching requests in this application instance without
// holding a database connection while waiting. Idle entries are removed immediately.
type idempotencyLocks struct {
	mu      sync.Mutex
	entries map[idempotencyRequestKey]*idempotencyLock
}

func (l *idempotencyLocks) acquire(ctx context.Context, key idempotencyRequestKey) (func(), error) {
	l.mu.Lock()
	if l.entries == nil {
		l.entries = make(map[idempotencyRequestKey]*idempotencyLock)
	}
	entry := l.entries[key]
	if entry == nil {
		entry = &idempotencyLock{available: make(chan struct{}, 1)}
		entry.available <- struct{}{}
		l.entries[key] = entry
	}
	entry.refs++
	l.mu.Unlock()

	drop := func() {
		l.mu.Lock()
		defer l.mu.Unlock()
		entry.refs--
		if entry.refs == 0 {
			delete(l.entries, key)
		}
	}
	select {
	case <-ctx.Done():
		drop()
		return nil, ctx.Err()
	case <-entry.available:
		return func() { entry.available <- struct{}{}; drop() }, nil
	}
}
