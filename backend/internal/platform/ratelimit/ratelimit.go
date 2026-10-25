// Package ratelimit provides a small in-memory fixed-window limiter. The MVP
// runs a single API instance, so a local limiter is enough (PRD 6.1).
package ratelimit

import (
	"sync"
	"time"
)

type window struct {
	count   int
	resetAt time.Time
}

// Limiter tracks request counts per key inside a rolling fixed window.
type Limiter struct {
	mu      sync.Mutex
	windows map[string]*window
	limit   int
	period  time.Duration
}

// New creates a limiter allowing `limit` requests per `period` per key.
func New(limit int, period time.Duration) *Limiter {
	l := &Limiter{
		windows: make(map[string]*window),
		limit:   limit,
		period:  period,
	}
	go l.reap()
	return l
}

// Allow records a request for key and reports whether it is permitted, plus
// the remaining allowance and the time the window resets.
func (l *Limiter) Allow(key string) (allowed bool, remaining int, resetAt time.Time) {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	w, ok := l.windows[key]
	if !ok || now.After(w.resetAt) {
		w = &window{count: 0, resetAt: now.Add(l.period)}
		l.windows[key] = w
	}
	w.count++
	if w.count > l.limit {
		return false, 0, w.resetAt
	}
	return true, l.limit - w.count, w.resetAt
}

// reap drops expired windows so the map does not grow without bound.
func (l *Limiter) reap() {
	t := time.NewTicker(5 * time.Minute)
	defer t.Stop()
	for range t.C {
		now := time.Now()
		l.mu.Lock()
		for k, w := range l.windows {
			if now.After(w.resetAt) {
				delete(l.windows, k)
			}
		}
		l.mu.Unlock()
	}
}
