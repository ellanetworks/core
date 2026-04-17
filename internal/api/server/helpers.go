package server

import (
	"fmt"
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

// getActorFromContext resolves the audit-log actor for a request. Requests
// arriving on the cluster mTLS port carry a peer node-id (set by
// peerNodeIDConnContext) and have no JWT email; attribute them to
// ella-node-<n>. Requests on the public API port fall back to the JWT email.
func getActorFromContext(r *http.Request) string {
	if nodeID, ok := peerNodeIDFromContext(r.Context()); ok {
		return fmt.Sprintf("ella-node-%d", nodeID)
	}

	return getEmailFromContext(r)
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
