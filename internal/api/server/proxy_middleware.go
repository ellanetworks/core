package server

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/ellanetworks/core/internal/cluster/listener"
	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/logger"
	"go.uber.org/zap"
)

const (
	headerAppliedIndex = "X-Ella-Applied-Index"
	headerForwarded    = "X-Ella-Forwarded"
)

// newClusterProxyClient returns an HTTP client that dials the leader's
// cluster port via the mTLS listener.
func newClusterProxyClient(ln *listener.Listener) *http.Client {
	return &http.Client{
		Transport: &http.Transport{
			DialTLSContext: func(ctx context.Context, _, addr string) (net.Conn, error) {
				return ln.Dial(ctx, addr, listener.ALPNHTTP, 10*time.Second)
			},
		},
		Timeout: 0,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
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
// Writes are forwarded over the cluster mTLS port to /cluster/proxy<requestURI>.
// Requests that already carry X-Ella-Forwarded are never proxied again,
// preventing infinite loops.
func LeaderProxyMiddleware(dbInstance *db.Database, ln *listener.Listener, next http.Handler) http.Handler {
	if dbInstance == nil || !dbInstance.ClusterEnabled() {
		return next
	}

	var clusterClient *http.Client
	if ln != nil {
		clusterClient = newClusterProxyClient(ln)
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

		if clusterClient == nil {
			writeError(r.Context(), w, http.StatusServiceUnavailable, "no leader available", nil, logger.APILog)
			return
		}

		proxyToLeaderCluster(w, r, clusterClient, dbInstance)
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

// proxyToLeaderCluster forwards a write request to the Raft leader's
// cluster HTTP port over mTLS.
func proxyToLeaderCluster(w http.ResponseWriter, r *http.Request, client *http.Client, dbInstance *db.Database) {
	leaderAddr := dbInstance.LeaderAddress()
	if leaderAddr == "" {
		writeError(r.Context(), w, http.StatusServiceUnavailable, "no leader available", nil, logger.APILog)
		return
	}

	targetURL := fmt.Sprintf("https://%s/cluster/proxy%s", leaderAddr, r.RequestURI)

	proxyReq, err := http.NewRequestWithContext(r.Context(), r.Method, targetURL, r.Body) // #nosec G704 -- targetURL is built from the trusted Raft leader address, not user input
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

	resp, err := client.Do(proxyReq) // #nosec G704 -- targetURL is built from the trusted Raft leader address, not user input
	if err != nil {
		logger.APILog.Warn("Leader cluster proxy failed", zap.Error(err))
		writeError(r.Context(), w, http.StatusBadGateway, "leader unreachable", err, logger.APILog)

		return
	}

	defer func() { _ = resp.Body.Close() }()

	copyProxyResponse(w, resp, dbInstance)
}

// copyProxyResponse writes the proxied response back to the original
// client, applying read-your-writes consistency when the leader includes
// an applied-index header.
func copyProxyResponse(w http.ResponseWriter, resp *http.Response, dbInstance *db.Database) {
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
