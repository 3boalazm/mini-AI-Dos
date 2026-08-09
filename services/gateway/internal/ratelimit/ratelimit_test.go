package ratelimit

import (
	"testing"
	"time"

	"github.com/ai-dos/foundation/util"
)

// tickingClock advances a controllable amount per test step.
type tickingClock struct{ now time.Time }

func (c *tickingClock) Now() time.Time { return c.now }

func TestAllow_UnderLimit(t *testing.T) {
	clock := &tickingClock{now: time.Unix(1000, 0)}
	l := New(3, time.Minute, clock)

	for i := 0; i < 3; i++ {
		allowed, remaining, _ := l.Allow()
		if !allowed {
			t.Fatalf("request %d should be allowed under the limit", i+1)
		}
		if want := 3 - (i + 1); remaining != want {
			t.Errorf("request %d remaining: got %d, want %d", i+1, remaining, want)
		}
	}
}

func TestAllow_BlocksOverLimit_WithRetryAfter(t *testing.T) {
	clock := &tickingClock{now: time.Unix(1000, 0)}
	l := New(2, time.Minute, clock)

	l.Allow()
	l.Allow()
	clock.now = clock.now.Add(10 * time.Second)

	allowed, remaining, retryAfter := l.Allow()
	if allowed {
		t.Fatal("third request should be blocked")
	}
	if remaining != 0 {
		t.Errorf("blocked request remaining: got %d, want 0", remaining)
	}
	if retryAfter != 50*time.Second {
		t.Errorf("retryAfter: got %v, want 50s (window remainder)", retryAfter)
	}
}

func TestAllow_WindowResets(t *testing.T) {
	clock := &tickingClock{now: time.Unix(1000, 0)}
	l := New(1, time.Minute, clock)

	l.Allow()
	if allowed, _, _ := l.Allow(); allowed {
		t.Fatal("second request in the same window should be blocked")
	}

	clock.now = clock.now.Add(time.Minute)
	if allowed, _, _ := l.Allow(); !allowed {
		t.Fatal("request after window reset should be allowed")
	}
}

func TestAllow_RetryAfterNeverBelowOneSecond(t *testing.T) {
	clock := &tickingClock{now: time.Unix(1000, 0)}
	l := New(1, time.Minute, clock)

	l.Allow()
	clock.now = clock.now.Add(time.Minute - 100*time.Millisecond)

	allowed, _, retryAfter := l.Allow()
	if allowed {
		t.Fatal("should still be blocked just before the window edge")
	}
	if retryAfter < time.Second {
		t.Errorf("retryAfter must round up to at least 1s for the Retry-After header, got %v", retryAfter)
	}
}

// Compile-time check that the production clock satisfies the seam.
var _ util.Clock = util.RealClock{}
