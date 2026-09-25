package agent

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestSessionLocksSerializeAndAllowOtherSessions(t *testing.T) {
	var locks sessionLocks
	releaseFirst, err := locks.acquire(context.Background(), "user-a\x00window-1")
	if err != nil {
		t.Fatal(err)
	}
	releaseOther, err := locks.acquire(context.Background(), "user-a\x00window-2")
	if err != nil {
		t.Fatal(err)
	}
	releaseOther()

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if _, err := locks.acquire(ctx, "user-a\x00window-1"); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("same-session waiter was not blocked: %v", err)
	}
	releaseFirst()
	releaseAgain, err := locks.acquire(context.Background(), "user-a\x00window-1")
	if err != nil {
		t.Fatal(err)
	}
	releaseAgain()
}
