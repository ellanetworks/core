// Copyright 2026 Ella Networks

package server

import (
	"fmt"
	"net/http"
	"strings"
	"time"
)

// responseWriter wraps http.ResponseWriter to capture status code
type responseWriter struct {
	http.ResponseWriter
	statusCode int
	written    bool
}

func (rw *responseWriter) WriteHeader(code int) {
	if !rw.written {
		rw.statusCode = code
		rw.written = true
		rw.ResponseWriter.WriteHeader(code)
	}
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	if !rw.written {
		rw.WriteHeader(http.StatusOK)
	}

	return rw.ResponseWriter.Write(b)
}

// MetricsMiddleware wraps HTTP handlers to collect request metrics
func MetricsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Wrap response writer to capture status code
		rw := &responseWriter{
			ResponseWriter: w,
			statusCode:     http.StatusOK,
			written:        false,
		}

		start := time.Now()

		endpoint := normalizeEndpoint(r.URL.Path)

		next.ServeHTTP(rw, r)

		duration := time.Since(start)

		trackRequest(r.Method, endpoint, rw.statusCode, duration)
	})
}

// normalizeEndpoint extracts the resource name from API paths.
// Examples:
//   - /api/v1/subscribers/123456789012345 -> /api/v1/subscribers
//   - /api/v1/users/user@example.com -> /api/v1/users
//   - /api/v1/networking/data-networks/my-network -> /api/v1/networking
//   - /api/v1/status -> /api/v1/status
func normalizeEndpoint(path string) string {
	if path == "" || path == "/" {
		return path
	}

	// Split path into segments
	segments := splitPath(path)

	// Keep first 3 segments: /api/v1/resource
	if len(segments) > 3 {
		segments = segments[:3]
	}

	return "/" + joinPath(segments)
}

func splitPath(path string) []string {
	segments := make([]string, 0, 8)
	start := 0

	for i := 0; i < len(path); i++ {
		if path[i] == '/' {
			if i > start {
				segments = append(segments, path[start:i])
			}

			start = i + 1
		}
	}

	if start < len(path) {
		segments = append(segments, path[start:])
	}

	return segments
}

func joinPath(segments []string) string {
	if len(segments) == 0 {
		return ""
	}

	result := segments[0]

	for i := 1; i < len(segments); i++ {
		result += "/" + segments[i]
	}

	return result
}

// trackRequest records a completed HTTP request using Prometheus metrics
// Only tracks /api/v1/* routes to avoid UI route pollution
func trackRequest(method, endpoint string, statusCode int, duration time.Duration) {
	// Only track API routes
	if !strings.HasPrefix(endpoint, "/api/v1/") {
		return
	}

	APIRequestsTotal.WithLabelValues(method, endpoint, fmt.Sprintf("%d", statusCode)).Inc()
	APIRequestDuration.WithLabelValues(method, endpoint).Observe(duration.Seconds())
}

func trackAuthAttempt(authType string, success bool) {
	result := "failure"
	if success {
		result = "success"
	}

	APIAuthAttempts.WithLabelValues(authType, result).Inc()
}
