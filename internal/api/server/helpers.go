package server

import (
	"net"
	"net/http"
	"strconv"
)

func getEmailFromContext(r *http.Request) string {
	if email, ok := r.Context().Value(contextKeyEmail).(string); ok {
		return email
	}

	return ""
}

// getClientIP extracts the client IP address from the direct connection.
func getClientIP(r *http.Request) string {
	ip, _, _ := net.SplitHostPort(r.RemoteAddr)
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
