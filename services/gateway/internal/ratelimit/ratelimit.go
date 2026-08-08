// Package ratelimit is a fixed-window, in-process rate limiter — the
// smallest thing that genuinely protects an upstream provider from a
// runaway caller. Deliberately not distributed, not Redis-backed, and
// not per-key (Mini AI-DOS has a single API key): one process, one
// counter, one window.
package ratelimit

import (
	"sync"
	"time"

	"github.com/ai-dos/foundation/util"
)

// Limiter allows at most Limit requests per Window. The zero value is
// not usable; construct with New.
type Limiter struct {
	mu          sync.Mutex
	limit       int
	window      time.Duration
	clock       util.Clock
	windowStart time.Time
	count       int
}

// New builds a Limiter. clock is injectable for tests (util.FakeClock),
// following the foundation's established testability pattern.
func New(limit int, window time.Duration, clock util.Clock) *Limiter {
	return &Limiter{limit: limit, window: window, clock: clock}
}

// Allow reports whether one more request fits in the current window.
// When it does not, retryAfter is how long until the window resets —
// suitable for a Retry-After header, rounded up to a whole second.
func (l *Limiter) Allow() (allowed bool, retryAfter time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.clock.Now()
	if l.windowStart.IsZero() || now.Sub(l.windowStart) >= l.window {
		l.windowStart = now
		l.count = 0
	}

	if l.count >= l.limit {
		remaining := l.window - now.Sub(l.windowStart)
		if remaining < time.Second {
			remaining = time.Second
		}
		return false, remaining
	}

	l.count++
	return true, 0
}
