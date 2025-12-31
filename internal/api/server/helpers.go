package server

import (
	"net"
	"net/http"
	"strconv"
	"strings"
)

func getEmailFromContext(r *http.Request) string {
	if email, ok := r.Context().Value(contextKeyEmail).(string); ok {
		return email
	}

	return ""
}

// getClientIP extracts the client IP address from the request headers or remote address.
func getClientIP(r *http.Request) string {
	xForwardedFor := r.Header.Get("X-Forwarded-For")
	if xForwardedFor != "" {
		ips := strings.Split(xForwardedFor, ",")
		if len(ips) > 0 {
			ip := strings.TrimSpace(ips[0])
			if ip != "" {
				return ip
			}
		}
	}

	// Check X-Real-IP header
	xRealIP := r.Header.Get("X-Real-IP")
	if xRealIP != "" {
		return strings.TrimSpace(xRealIP)
	}

	// Fallback to remote address
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr // as a fallback (may include port)
	}

	return ip
}

func atoiDefault(s string, def int) int {
	if s == "" {
		return def
	}

	if v, err := strconv.Atoi(s); err == nil {
		return v
	}

	return def
}
