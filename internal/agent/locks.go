package agent

import (
	"context"
	"sync"
)

type sessionSemaphore struct {
	token chan struct{}
	users int
}

type sessionLocks struct {
	mu      sync.Mutex
	entries map[string]*sessionSemaphore
}

func (locks *sessionLocks) acquire(ctx context.Context, key string) (func(), error) {
	locks.mu.Lock()
	if locks.entries == nil {
		locks.entries = make(map[string]*sessionSemaphore)
	}
	entry := locks.entries[key]
	if entry == nil {
		entry = &sessionSemaphore{token: make(chan struct{}, 1)}
		entry.token <- struct{}{}
		locks.entries[key] = entry
	}
	entry.users++
	locks.mu.Unlock()

	select {
	case <-ctx.Done():
		locks.releaseReference(key, entry)
		return nil, ctx.Err()
	case <-entry.token:
		return func() {
			entry.token <- struct{}{}
			locks.releaseReference(key, entry)
		}, nil
	}
}

func (locks *sessionLocks) releaseReference(key string, entry *sessionSemaphore) {
	locks.mu.Lock()
	defer locks.mu.Unlock()
	entry.users--
	if entry.users == 0 {
		delete(locks.entries, key)
	}
}
