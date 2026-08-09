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
// remaining is how many further requests the window still accepts
// after this one — suitable for an X-RateLimit-Remaining header. When
// blocked, retryAfter is how long until the window resets — suitable
// for a Retry-After header, rounded up to a whole second.
func (l *Limiter) Allow() (allowed bool, remaining int, retryAfter time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.clock.Now()
	if l.windowStart.IsZero() || now.Sub(l.windowStart) >= l.window {
		l.windowStart = now
		l.count = 0
	}

	if l.count >= l.limit {
		left := l.window - now.Sub(l.windowStart)
		if left < time.Second {
			left = time.Second
		}
		return false, 0, left
	}

	l.count++
	return true, l.limit - l.count, 0
}
