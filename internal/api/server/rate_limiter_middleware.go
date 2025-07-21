package server

import (
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/ellanetworks/core/internal/logger"
	"golang.org/x/time/rate"
)

const (
	NumRequests     = 100
	RequestsPerTime = time.Second
)

type Visitor struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

var (
	visitors = make(map[string]*Visitor)
	mu       sync.Mutex
)

// getVisitor retrieves the rate limiter for a given IP, or creates a new one if none exists.
func getVisitor(ip string) *rate.Limiter {
	mu.Lock()
	defer mu.Unlock()

	v, exists := visitors[ip]
	if !exists {
		limiter := rate.NewLimiter(rate.Limit(NumRequests), NumRequests)
		visitors[ip] = &Visitor{limiter: limiter, lastSeen: time.Now()}
		return limiter
	}
	v.lastSeen = time.Now()
	return v.limiter
}

// cleanupVisitors runs periodically to remove visitors that haven't been seen for over 3 minutes.
func cleanupVisitors() {
	for {
		time.Sleep(time.Minute)
		mu.Lock()
		for ip, v := range visitors {
			if time.Since(v.lastSeen) > 1*time.Minute {
				delete(visitors, ip)
			}
		}
		mu.Unlock()
	}
}

// RateLimitMiddleware is a Gin middleware that rate limits incoming requests based on the client IP.
func RateLimitMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := getClientIP(r)
		limiter := getVisitor(ip)
		if !limiter.Allow() {
			writeError(w, http.StatusTooManyRequests, "Rate limit exceeded", fmt.Errorf("rate limit exceeded for IP %s", ip), logger.APILog)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func ResetVisitors() {
	mu.Lock()
	defer mu.Unlock()
	visitors = make(map[string]*Visitor)
}

func init() {
	// Start the cleanup goroutine for the rate limiter.
	go cleanupVisitors()
}
