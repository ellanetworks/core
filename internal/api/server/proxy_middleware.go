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

// newClusterProxyClient returns an HTTP client that dials the specified
// leader via the mTLS listener, enforcing that the peer's certificate
// CN resolves to expectedLeaderID. The client is built per request
// because the expected leader node-id is part of the TLS dial contract
// and leadership can change between requests.
func newClusterProxyClient(ln *listener.Listener, expectedLeaderID int) *http.Client {
	return &http.Client{
		Transport: &http.Transport{
			DialTLSContext: func(ctx context.Context, _, addr string) (net.Conn, error) {
				return ln.Dial(ctx, addr, expectedLeaderID, listener.ALPNHTTP, 10*time.Second)
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

// leaderOnlyReadPaths enumerates the GET paths that must be served by the
// leader even though they do not mutate state. Autopilot state only
// exists on the leader; a follower has nothing useful to return, so the
// middleware proxies these requests the same way it proxies writes.
var leaderOnlyReadPaths = map[string]struct{}{
	"/api/v1/cluster/autopilot": {},
}

func isLeaderOnlyRead(r *http.Request) bool {
	if r.Method != http.MethodGet {
		return false
	}

	_, ok := leaderOnlyReadPaths[r.URL.Path]

	return ok
}

// isSelfRemoval reports whether r is a `DELETE /api/v1/cluster/members/{n}`
// where n matches the node id evaluating the request. Used to reject
// the request before proxying so an operator cannot delete the cluster
// member for the node they are currently connected to.
func isSelfRemoval(r *http.Request, localNodeID int) bool {
	if r.Method != http.MethodDelete {
		return false
	}

	const prefix = "/api/v1/cluster/members/"
	if !strings.HasPrefix(r.URL.Path, prefix) {
		return false
	}

	rest := r.URL.Path[len(prefix):]
	if rest == "" || strings.ContainsRune(rest, '/') {
		return false
	}

	id, err := strconv.Atoi(rest)
	if err != nil {
		return false
	}

	return id == localNodeID
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

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if isSelfRemoval(r, dbInstance.NodeID()) {
			writeError(r.Context(), w, http.StatusConflict,
				"Cannot remove the node you are currently connected to; issue the request against another node",
				nil, logger.APILog)

			return
		}

		mustProxy := isWriteMethod(r.Method) || isLeaderOnlyRead(r)
		if dbInstance.IsLeader() || !mustProxy {
			next.ServeHTTP(w, r)
			return
		}

		if r.Header.Get(headerForwarded) != "" {
			next.ServeHTTP(w, r)
			return
		}

		if ln == nil {
			writeError(r.Context(), w, http.StatusServiceUnavailable, "no leader available", nil, logger.APILog)
			return
		}

		proxyToLeaderCluster(w, r, ln, dbInstance)
	})
}

// resolveLeader looks up the current leader in the cluster_members table
// and returns the leader's API address and node-id. Either field is zero
// when no leader is known or the leader's row is not yet present.
func resolveLeader(dbInstance *db.Database) (apiAddress string, nodeID int) {
	raftAddr := dbInstance.LeaderAddress()
	if raftAddr == "" {
		return "", 0
	}

	members, err := dbInstance.ListClusterMembers(context.Background())
	if err != nil {
		return "", 0
	}

	for _, m := range members {
		if m.RaftAddress == raftAddr {
			return m.APIAddress, m.NodeID
		}
	}

	return "", 0
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
// cluster HTTP port over mTLS. The http.Client is built per request
// so its DialTLSContext can pin the expected leader node-id — leadership
// can change between requests, and an impersonator presenting a valid
// cluster leaf under a different node-id must not be able to terminate
// writes meant for the leader.
func proxyToLeaderCluster(w http.ResponseWriter, r *http.Request, ln *listener.Listener, dbInstance *db.Database) {
	leaderAddr, leaderID := dbInstance.LeaderAddressAndID()
	if leaderAddr == "" {
		writeError(r.Context(), w, http.StatusServiceUnavailable, "no leader available", nil, logger.APILog)
		return
	}

	if leaderID == 0 {
		writeError(r.Context(), w, http.StatusServiceUnavailable, "leader identity not yet established", nil, logger.APILog)
		return
	}

	client := newClusterProxyClient(ln, leaderID)
	doProxyToLeader(w, r, client, leaderAddr, dbInstance)
}

// doProxyToLeader performs the actual HTTP round-trip against the leader
// using the supplied client. Split from proxyToLeaderCluster so tests can
// inject a stub transport.
func doProxyToLeader(w http.ResponseWriter, r *http.Request, client *http.Client, leaderAddr string, dbInstance *db.Database) {
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

	// 410 Gone from the leader means this node has been removed from
	// cluster_members (removedNodeFence). The end-user caller did not
	// remove anything, so surface 502 Bad Gateway rather than forwarding
	// the 410 verbatim.
	if resp.StatusCode == http.StatusGone {
		logger.APILog.Error("proxy: this node has been removed from the cluster; operator must shut it down",
			zap.Int("nodeId", dbInstance.NodeID()))
		writeError(r.Context(), w, http.StatusBadGateway,
			"this node is no longer a cluster member", nil, logger.APILog)

		return
	}

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
			start := time.Now()

			localIdx, caughtUp := waitForLocalIndex(dbInstance, targetIdx)
			if !caughtUp {
				logger.APILog.Warn(
					"proxy: follower did not catch up to leader applied index before response; read-your-writes may be violated for an immediate subsequent read on this node",
					zap.Uint64("targetIdx", targetIdx),
					zap.Uint64("localIdx", localIdx),
					zap.Duration("waited", time.Since(start)),
				)
			}
		}
	}

	w.WriteHeader(resp.StatusCode)

	if _, err := io.Copy(w, resp.Body); err != nil {
		logger.APILog.Warn("Error copying proxy response body", zap.Error(err))
	}
}

const (
	proxyReadYourWritesMaxWait      = 2 * time.Second
	proxyReadYourWritesPollInterval = 5 * time.Millisecond
)

// waitForLocalIndex blocks until the local Raft applied index reaches
// targetIdx or the deadline elapses. Returns the last-observed local index
// and whether it caught up.
func waitForLocalIndex(dbInstance *db.Database, targetIdx uint64) (uint64, bool) {
	return waitForIndex(targetIdx, dbInstance.RaftAppliedIndex, proxyReadYourWritesMaxWait, proxyReadYourWritesPollInterval)
}

func waitForIndex(targetIdx uint64, get func() uint64, maxWait, poll time.Duration) (uint64, bool) {
	deadline := time.Now().Add(maxWait)

	for {
		local := get()
		if local >= targetIdx {
			return local, true
		}

		if !time.Now().Before(deadline) {
			return local, false
		}

		time.Sleep(poll)
	}
}
