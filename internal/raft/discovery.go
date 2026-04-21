// Copyright 2026 Ella Networks

package raft

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"

	"github.com/ellanetworks/core/internal/cluster/listener"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/hashicorp/raft"
	"go.uber.org/zap"
)

const (
	discoveryPollInterval = 1 * time.Second
	defaultJoinTimeout    = 2 * time.Minute
	discoveryHTTPTimeout  = 5 * time.Second
)

type peerState int

const (
	peerUnreachable peerState = iota
	peerForming
	peerFormed
)

// statusClusterBlock mirrors the cluster block of the status API response.
type statusClusterBlock struct {
	Role          string `json:"role"`
	NodeID        int    `json:"nodeId"`
	ClusterID     string `json:"clusterId"`
	SchemaVersion int    `json:"schemaVersion"`
}

type statusResult struct {
	Cluster *statusClusterBlock `json:"cluster"`
}

type statusResponse struct {
	Result statusResult `json:"result"`
}

// NeedsDiscovery reports whether this manager requires the discovery loop
// to form or join a cluster before it can serve writes.
func (m *Manager) NeedsDiscovery() bool {
	return m.needsDiscovery
}

// RunDiscovery performs cluster formation for HA mode. It must be called after
// the cluster listener starts so peers can reach this node's cluster port. In
// standalone mode or when resuming existing Raft state, it returns immediately.
func (m *Manager) RunDiscovery(ctx context.Context) error {
	if !m.needsDiscovery {
		return nil
	}

	if m.clusterListener == nil {
		return fmt.Errorf("cluster discovery requires a cluster listener (mTLS)")
	}

	timeout := m.config.JoinTimeout
	if timeout == 0 {
		timeout = defaultJoinTimeout
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	logger.RaftLog.Info("Starting cluster discovery",
		zap.Int("node_id", m.nodeID),
		zap.Bool("has_join_token", m.config.HasJoinToken),
		zap.Int("peers", len(m.config.Peers)),
		zap.Duration("join_timeout", timeout),
	)

	ticker := time.NewTicker(discoveryPollInterval)
	defer ticker.Stop()

	for {
		joined, err := m.discoveryTick(ctx)
		if err != nil {
			return err
		}

		if joined {
			m.needsDiscovery = false

			if err := m.waitForLeader(ctx); err != nil {
				return err
			}

			m.restoreHATimeouts()

			return nil
		}

		select {
		case <-ctx.Done():
			return fmt.Errorf("cluster discovery timed out after %s", timeout)
		case <-ticker.C:
		}
	}
}

// discoveryTick runs one iteration of the discovery poll. Returns true when
// the cluster has been joined or bootstrapped.
//
// The tick has two modes, selected by the join-token:
//
//   - A node with a join-token is a joiner. It only returns success when it
//     finds a formed peer and POSTs its membership to it. Solo-bootstrap is
//     never taken, even if no peers are reachable.
//   - A node without a join-token is the founder. It bootstraps immediately
//     on the first tick, without probing peers — the founder has nothing
//     to learn from them and the probe cost (5 s × non-self peers) would
//     otherwise delay startup. PostInitClusterSetup on the first leader
//     mints the CA and issues this node's leaf so joiners can connect.
func (m *Manager) discoveryTick(ctx context.Context) (bool, error) {
	if !m.config.HasJoinToken {
		logger.RaftLog.Info("Bootstrapping new cluster (no join-token configured)",
			zap.Int("node_id", m.nodeID),
		)

		return true, m.bootstrapCluster()
	}

	for _, peerAddr := range m.config.Peers {
		if peerAddr == m.config.AdvertiseAddress {
			continue
		}

		state, nodeID, clusterID, peerSchema := m.probePeer(ctx, peerAddr)

		if state == peerUnreachable {
			continue
		}

		// Duplicate node-id is a hard misconfiguration: two nodes can't
		// share an ID without risking split-brain during bootstrap or
		// clobbering an existing cluster member at join time. Fail loud
		// so the operator fixes cluster.node-id rather than letting the
		// cluster form silently wrong.
		if nodeID > 0 && nodeID == m.nodeID {
			return false, fmt.Errorf("peer %s advertises the same node-id (%d) as this node; check cluster.node-id configuration", peerAddr, nodeID)
		}

		if state != peerFormed {
			continue
		}

		// Schema handshake (follower side): allow joining a peer whose
		// cluster schema is <= our local schema, since post-baseline
		// migrations are proposed through Raft by the leader. Reject the
		// reverse (we'd be downgrading). The leader side of this check
		// lives in api_cluster.go:AddClusterMember and has the complementary
		// rule: reject joiners with schema < leader.
		if m.config.SchemaVersion < peerSchema {
			logger.RaftLog.Warn("Schema version lower than peer, skipping (downgrade)",
				zap.String("peer", peerAddr),
				zap.Int("local", m.config.SchemaVersion),
				zap.Int("remote", peerSchema),
			)

			continue
		}

		if err := m.joinCluster(ctx, peerAddr, nodeID, clusterID); err != nil {
			logger.RaftLog.Warn("Failed to join cluster via peer",
				zap.String("peer", peerAddr),
				zap.Error(err),
			)

			continue
		}

		return true, nil
	}

	// No formed peer found this tick. The joiner keeps polling.
	return false, nil
}

// clusterHTTPDo dials a peer's cluster port over mTLS and performs a
// single HTTP request. Pass nil body for GET-style requests. When
// expectedPeerID is non-zero the dial verifies the peer's leaf CN
// resolves to that node-id; pass 0 only from discovery paths that are
// still learning the peer's identity.
func (m *Manager) clusterHTTPDo(ctx context.Context, method, peerAddr string, expectedPeerID int, path string, body io.Reader) (*http.Response, error) {
	var (
		conn net.Conn
		err  error
	)

	if expectedPeerID == 0 {
		conn, err = m.clusterListener.DialAnyPeer(ctx, peerAddr, listener.ALPNHTTP, discoveryHTTPTimeout)
	} else {
		conn, err = m.clusterListener.Dial(ctx, peerAddr, expectedPeerID, listener.ALPNHTTP, discoveryHTTPTimeout)
	}

	if err != nil {
		return nil, err
	}

	// conn ownership: we close it on any error path. On success the
	// response body holds the conn via the transport; the caller closes
	// it by closing resp.Body.
	connUsed := false

	defer func() {
		if !connUsed {
			_ = conn.Close()
		}
	}()

	transport := &http.Transport{
		DialTLSContext: func(context.Context, string, string) (net.Conn, error) {
			if connUsed {
				return nil, fmt.Errorf("cluster HTTP transport: connection already consumed")
			}

			connUsed = true

			return conn, nil
		},
	}

	client := &http.Client{Transport: transport, Timeout: discoveryHTTPTimeout}

	req, err := http.NewRequestWithContext(ctx, method, "https://"+peerAddr+path, body)
	if err != nil {
		return nil, err
	}

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := client.Do(req) // #nosec G107 -- peerAddr comes from the operator-configured cluster.peers list
	if err != nil {
		return nil, err
	}

	return resp, nil
}

// probePeer queries a peer's cluster status endpoint and returns its state,
// node ID, cluster ID, and schema version. This is the one discovery path
// that cannot pin an expected peer-id — learning the peer's identity is
// the purpose of the probe.
func (m *Manager) probePeer(ctx context.Context, peerAddr string) (peerState, int, string, int) {
	resp, err := m.clusterHTTPDo(ctx, http.MethodGet, peerAddr, 0, "/cluster/status", nil)
	if err != nil {
		return peerUnreachable, 0, "", 0
	}

	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return peerUnreachable, 0, "", 0
	}

	var status statusResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, 4096)).Decode(&status); err != nil {
		return peerUnreachable, 0, "", 0
	}

	if status.Result.Cluster == nil {
		return peerForming, 0, "", 0
	}

	nodeID := status.Result.Cluster.NodeID
	role := status.Result.Cluster.Role
	clusterID := status.Result.Cluster.ClusterID
	schemaVersion := status.Result.Cluster.SchemaVersion

	if clusterID != "" && (role == "Leader" || role == "Follower") {
		return peerFormed, nodeID, clusterID, schemaVersion
	}

	return peerForming, nodeID, clusterID, schemaVersion
}

// SelfAnnounce POSTs this node's capability record (node-id, addresses,
// binary version, max schema version, suffrage) to the current leader's
// cluster port. Callers invoke this on startup after Raft is up so
// follower rows in `cluster_members` reflect the running binary. When
// this node is the leader, callers should write directly via
// db.UpsertClusterMember rather than calling this method.
func (m *Manager) SelfAnnounce(ctx context.Context, payload any) error {
	leaderAddr, leaderID := m.LeaderAddressAndID()
	if leaderAddr == "" {
		return fmt.Errorf("no known leader")
	}

	if leaderID == 0 {
		return fmt.Errorf("leader at %s has unrecognized server id; refusing to self-announce", leaderAddr)
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal self-announce: %w", err)
	}

	resp, err := m.clusterHTTPDo(ctx, http.MethodPost, leaderAddr, leaderID, "/cluster/members/self", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("dial leader %s: %w", leaderAddr, err)
	}

	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("leader returned %d: %s", resp.StatusCode, string(respBody))
	}

	return nil
}

// joinCluster POSTs our membership to a peer's cluster port. peerNodeID
// is the node-id learned from the prior probePeer call; it pins the mTLS
// dial so we only send the join payload to the node we intended.
func (m *Manager) joinCluster(ctx context.Context, peerAddr string, peerNodeID int, clusterID string) error {
	payload := struct {
		NodeID           int    `json:"nodeId"`
		RaftAddress      string `json:"raftAddress"`
		APIAddress       string `json:"apiAddress"`
		ClusterID        string `json:"clusterId"`
		SchemaVersion    int    `json:"schemaVersion"`
		MaxSchemaVersion int    `json:"maxSchemaVersion"`
		Suffrage         string `json:"suffrage,omitempty"`
	}{
		NodeID:           m.nodeID,
		RaftAddress:      string(m.transport.LocalAddr()),
		APIAddress:       m.config.APIAddress,
		ClusterID:        clusterID,
		SchemaVersion:    m.config.SchemaVersion,
		MaxSchemaVersion: m.config.SchemaVersion,
		Suffrage:         m.config.InitialSuffrage,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal join request: %w", err)
	}

	resp, err := m.clusterHTTPDo(ctx, http.MethodPost, peerAddr, peerNodeID, "/cluster/members", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}

	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("server returned %d: %s", resp.StatusCode, string(respBody))
	}

	logger.RaftLog.Info("Joined existing cluster via peer",
		zap.String("peer", peerAddr),
		zap.Int("node_id", m.nodeID),
	)

	return nil
}

// bootstrapCluster creates the initial Raft cluster with this node as the
// sole voter. Other nodes will join via AddVoter as they discover the leader.
// Fast self-election is guaranteed by applyTimeouts which sets standalone
// timeouts for fresh HA nodes; restoreHATimeouts upgrades them after the
// cluster forms.
func (m *Manager) bootstrapCluster() error {
	cfg := raft.Configuration{
		Servers: []raft.Server{{
			ID:      raft.ServerID(fmt.Sprintf("%d", m.nodeID)),
			Address: m.transport.LocalAddr(),
		}},
	}

	if err := m.raft.BootstrapCluster(cfg).Error(); err != nil {
		return fmt.Errorf("bootstrap cluster: %w", err)
	}

	return nil
}
