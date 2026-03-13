package server

import (
	"sync"
	"time"
)

// ipRateLimiter tracks per-IP request counts within a sliding window.
// It is safe for concurrent use.
type ipRateLimiter struct {
	mu      sync.Mutex
	entries map[string]*rateLimitEntry
	limit   int           // max requests per window
	window  time.Duration // window size

	stopOnce sync.Once
	done     chan struct{}
}

type rateLimitEntry struct {
	count       int
	windowStart time.Time
}

// newIPRateLimiter creates a rate limiter that allows `limit` requests per
// `window` duration for each IP address. It starts a background goroutine
// that cleans up stale entries every `window` interval.
func newIPRateLimiter(limit int, window time.Duration) *ipRateLimiter {
	rl := &ipRateLimiter{
		entries: make(map[string]*rateLimitEntry),
		limit:   limit,
		window:  window,
		done:    make(chan struct{}),
	}

	go rl.cleanup()

	return rl
}

// allow returns true if the request from the given IP should be permitted.
func (rl *ipRateLimiter) allow(ip string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()

	entry, ok := rl.entries[ip]
	if !ok || now.Sub(entry.windowStart) >= rl.window {
		rl.entries[ip] = &rateLimitEntry{count: 1, windowStart: now}
		return true
	}

	entry.count++

	return entry.count <= rl.limit
}

// cleanup removes expired entries periodically.
func (rl *ipRateLimiter) cleanup() {
	ticker := time.NewTicker(rl.window)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			rl.mu.Lock()

			now := time.Now()

			for ip, entry := range rl.entries {
				if now.Sub(entry.windowStart) >= rl.window {
					delete(rl.entries, ip)
				}
			}

			rl.mu.Unlock()
		case <-rl.done:
			return
		}
	}
}

// stop terminates the background cleanup goroutine.
func (rl *ipRateLimiter) stop() {
	rl.stopOnce.Do(func() {
		close(rl.done)
	})
}
