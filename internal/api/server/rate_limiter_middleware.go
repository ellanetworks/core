package server

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"
)

const (
	RequestsPerSecond = 5
	Burst             = 5
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
		limiter := rate.NewLimiter(RequestsPerSecond, Burst)
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
			if time.Since(v.lastSeen) > 3*time.Minute {
				delete(visitors, ip)
			}
		}
		mu.Unlock()
	}
}

// RateLimitMiddleware is a Gin middleware that rate limits incoming requests based on the client IP.
func RateLimitMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		ip := c.ClientIP()
		limiter := getVisitor(ip)
		if !limiter.Allow() {
			writeError(c.Writer, http.StatusTooManyRequests, "Too many requests")
			c.Abort()
			return
		}
		c.Next()
	}
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
