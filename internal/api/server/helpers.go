package server

import (
	"encoding/json"
	"net"
	"net/http"
	"strings"

	"github.com/ellanetworks/core/internal/logger"
)

func pathParam(path, prefix string) string {
	if strings.HasPrefix(path, prefix) {
		return strings.TrimPrefix(path, prefix)
	}
	return ""
}

func getEmailFromContext(r *http.Request) string {
	if email, ok := r.Context().Value("email").(string); ok {
		return email
	}
	return ""
}

func decodeJSONBody(w http.ResponseWriter, r *http.Request, out any) error {
	if err := json.NewDecoder(r.Body).Decode(out); err != nil {
		writeErrorHTTP(w, http.StatusBadRequest, "Invalid JSON body", err, logger.APILog)
		return err
	}
	return nil
}

// GetClientIP extracts the client IP address from the request headers or remote address.
func GetClientIP(r *http.Request) string {
	// Check X-Forwarded-For header (may contain multiple IPs)
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
