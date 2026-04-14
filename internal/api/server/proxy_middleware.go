package server

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/logger"
	"go.uber.org/zap"
)

const (
	headerAppliedIndex = "X-Ella-Applied-Index"
	headerForwarded    = "X-Ella-Forwarded"
	proxyTimeout       = 30 * time.Second
)

var proxyClient = &http.Client{
	Timeout: proxyTimeout,
	CheckRedirect: func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	},
}

func isWriteMethod(method string) bool {
	switch method {
	case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
		return true
	}

	return false
}

// LeaderProxyMiddleware forwards write requests to the Raft leader when this
// node is a follower. Read requests are served locally. The middleware runs
// before authentication so the leader handles auth for proxied writes.
//
// Requests that already carry X-Ella-Forwarded are never proxied again,
// preventing infinite loops.
func LeaderProxyMiddleware(dbInstance *db.Database, next http.Handler) http.Handler {
	if dbInstance == nil || !dbInstance.ClusterEnabled() {
		return next
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if dbInstance.IsLeader() || !isWriteMethod(r.Method) {
			next.ServeHTTP(w, r)
			return
		}

		if r.Header.Get(headerForwarded) != "" {
			next.ServeHTTP(w, r)
			return
		}

		leaderAPI := resolveLeaderAPI(dbInstance)
		if leaderAPI == "" {
			writeError(r.Context(), w, http.StatusServiceUnavailable, "no leader available", nil, logger.APILog)
			return
		}

		proxyToLeader(w, r, leaderAPI, dbInstance)
	})
}

func resolveLeaderAPI(dbInstance *db.Database) string {
	raftAddr := dbInstance.LeaderAddress()
	if raftAddr == "" {
		return ""
	}

	members, err := dbInstance.ListClusterMembers(context.Background())
	if err != nil {
		return ""
	}

	for _, m := range members {
		if m.RaftAddress == raftAddr {
			return m.APIAddress
		}
	}

	return ""
}

// isHopByHopHeader returns true for headers that must not be forwarded by
// proxies per RFC 7230 §6.1.
func isHopByHopHeader(h string) bool {
	switch strings.ToLower(h) {
	case "connection", "keep-alive", "proxy-authenticate",
		"proxy-authorization", "te", "trailer", "transfer-encoding", "upgrade":
		return true
	}

	return false
}

func proxyToLeader(w http.ResponseWriter, r *http.Request, leaderAPI string, dbInstance *db.Database) {
	scheme := "https"
	if strings.HasPrefix(leaderAPI, "http://") || strings.HasPrefix(leaderAPI, "https://") {
		scheme = ""
	}

	var targetURL string
	if scheme != "" {
		targetURL = fmt.Sprintf("%s://%s%s", scheme, leaderAPI, r.RequestURI)
	} else {
		targetURL = fmt.Sprintf("%s%s", leaderAPI, r.RequestURI)
	}

	proxyReq, err := http.NewRequestWithContext(r.Context(), r.Method, targetURL, r.Body) // #nosec: G704 -- targetURL is built from the trusted Raft leader's cluster-member API address
	if err != nil {
		writeError(r.Context(), w, http.StatusBadGateway, "failed to create proxy request", err, logger.APILog)
		return
	}

	for key, values := range r.Header {
		if isHopByHopHeader(key) {
			continue
		}

		for _, v := range values {
			proxyReq.Header.Add(key, v)
		}
	}

	proxyReq.Header.Set(headerForwarded, "true")

	resp, err := proxyClient.Do(proxyReq) // #nosec: G704 -- proxyReq targets the trusted Raft leader's cluster-member API address
	if err != nil {
		logger.APILog.Warn("Leader proxy failed", zap.Error(err))
		writeError(r.Context(), w, http.StatusBadGateway, "leader unreachable", err, logger.APILog)

		return
	}

	defer func() { _ = resp.Body.Close() }()

	for key, values := range resp.Header {
		if isHopByHopHeader(key) {
			continue
		}

		for _, v := range values {
			w.Header().Add(key, v)
		}
	}

	if idxStr := resp.Header.Get(headerAppliedIndex); idxStr != "" {
		targetIdx, parseErr := strconv.ParseUint(idxStr, 10, 64)
		if parseErr == nil {
			waitForLocalIndex(dbInstance, targetIdx)
		}
	}

	w.WriteHeader(resp.StatusCode)

	if _, err := io.Copy(w, resp.Body); err != nil {
		logger.APILog.Warn("Error copying proxy response body", zap.Error(err))
	}
}

// waitForLocalIndex blocks briefly until the local Raft applied index catches
// up, implementing read-your-writes consistency for proxied requests.
func waitForLocalIndex(dbInstance *db.Database, targetIdx uint64) {
	const (
		pollInterval = 5 * time.Millisecond
		maxWait      = 2 * time.Second
	)

	deadline := time.Now().Add(maxWait)

	for time.Now().Before(deadline) {
		if dbInstance.RaftAppliedIndex() >= targetIdx {
			return
		}

		time.Sleep(pollInterval)
	}
}
