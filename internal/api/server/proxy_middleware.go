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

// targetedEndpointNodeID returns the node-id when r targets a per-member
// endpoint that must execute on the target node itself (not the leader).
// Currently this covers drain and resume. The proxy forwards such requests
// directly to the target's cluster port, bypassing the leader, because the
// side-effects are local runtime state (BGP speaker, connected RANs).
func targetedEndpointNodeID(r *http.Request) (int, bool) {
	if !isWriteMethod(r.Method) {
		return 0, false
	}

	const prefix = "/api/v1/cluster/members/"
	if !strings.HasPrefix(r.URL.Path, prefix) {
		return 0, false
	}

	rest := r.URL.Path[len(prefix):]

	slash := strings.IndexByte(rest, '/')
	if slash <= 0 {
		return 0, false
	}

	idStr, tail := rest[:slash], rest[slash+1:]
	if tail != "drain" && tail != "resume" {
		return 0, false
	}

	id, err := strconv.Atoi(idStr)
	if err != nil || id <= 0 {
		return 0, false
	}

	return id, true
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

	var clusterClient *http.Client
	if ln != nil {
		clusterClient = newClusterProxyClient(ln)
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if isSelfRemoval(r, dbInstance.NodeID()) {
			writeError(r.Context(), w, http.StatusConflict,
				"Cannot remove the node you are currently connected to; issue the request against another node",
				nil, logger.APILog)

			return
		}

		// Per-member drain/resume must execute on the target node itself;
		// forward directly to that node's cluster port (bypassing the
		// leader). If we are already the target, serve locally. An
		// already-forwarded request on the wrong node is a routing bug
		// rather than something to silently loop on.
		if targetID, ok := targetedEndpointNodeID(r); ok {
			if targetID == dbInstance.NodeID() {
				next.ServeHTTP(w, r)
				return
			}

			if r.Header.Get(headerForwarded) != "" {
				writeError(r.Context(), w, http.StatusBadGateway,
					"per-member request forwarded to wrong node", nil, logger.APILog)

				return
			}

			if clusterClient == nil {
				writeError(r.Context(), w, http.StatusServiceUnavailable,
					"cluster client unavailable", nil, logger.APILog)

				return
			}

			proxyToMemberCluster(w, r, clusterClient, dbInstance, targetID)

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

		if clusterClient == nil {
			writeError(r.Context(), w, http.StatusServiceUnavailable, "no leader available", nil, logger.APILog)
			return
		}

		proxyToLeaderCluster(w, r, clusterClient, dbInstance)
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

// proxyToMemberCluster forwards r to a specific cluster member's HTTP cluster
// port over mTLS. Used for per-member endpoints (drain, resume) whose
// side-effects must execute on the target node itself.
func proxyToMemberCluster(w http.ResponseWriter, r *http.Request, client *http.Client, dbInstance *db.Database, targetNodeID int) {
	target, err := dbInstance.GetClusterMember(r.Context(), targetNodeID)
	if err != nil {
		writeError(r.Context(), w, http.StatusNotFound, "target cluster member not found", err, logger.APILog)
		return
	}

	if target.RaftAddress == "" {
		writeError(r.Context(), w, http.StatusServiceUnavailable,
			"target cluster member has no raft address", nil, logger.APILog)

		return
	}

	targetURL := fmt.Sprintf("https://%s/cluster/proxy%s", target.RaftAddress, r.RequestURI)

	proxyReq, err := http.NewRequestWithContext(r.Context(), r.Method, targetURL, r.Body) // #nosec G704 -- built from cluster_members row (Raft-replicated)
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

	resp, err := client.Do(proxyReq) // #nosec G704 -- built from cluster_members row (Raft-replicated)
	if err != nil {
		logger.APILog.Warn("Member cluster proxy failed",
			zap.Int("targetNodeId", targetNodeID), zap.Error(err))
		writeError(r.Context(), w, http.StatusBadGateway, "target node unreachable", err, logger.APILog)

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
