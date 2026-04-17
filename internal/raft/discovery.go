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
		zap.Int("bootstrap_expect", m.config.BootstrapExpect),
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
func (m *Manager) discoveryTick(ctx context.Context) (bool, error) {
	reachableCount := 1 // count ourselves
	lowestReachableNodeID := m.nodeID

	for _, peerAddr := range m.config.Peers {
		if peerAddr == m.config.AdvertiseAddress {
			continue
		}

		state, nodeID, clusterID, peerSchema := m.probePeer(ctx, peerAddr)

		switch state {
		case peerFormed:
			// Schema handshake (follower side): allow joining a peer whose
			// cluster schema is <= our local schema, since post-baseline
			// migrations are proposed through Raft by the leader. Reject
			// the reverse (we'd be downgrading). The leader side of this
			// check lives in api_cluster.go:AddClusterMember and has the
			// complementary rule: reject joiners with schema < leader.
			if m.config.SchemaVersion < peerSchema {
				logger.RaftLog.Warn("Schema version lower than peer, skipping (downgrade)",
					zap.String("peer", peerAddr),
					zap.Int("local", m.config.SchemaVersion),
					zap.Int("remote", peerSchema),
				)

				continue
			}

			if err := m.joinCluster(ctx, peerAddr, clusterID); err != nil {
				logger.RaftLog.Warn("Failed to join cluster via peer",
					zap.String("peer", peerAddr),
					zap.Error(err),
				)

				continue
			}

			return true, nil

		case peerForming:
			reachableCount++

			if nodeID == m.nodeID {
				logger.RaftLog.Warn("Peer reports duplicate node-id during discovery",
					zap.String("peer", peerAddr),
					zap.Int("node_id", nodeID),
				)
			}

			if nodeID > 0 && nodeID < lowestReachableNodeID {
				lowestReachableNodeID = nodeID
			}
		}
	}

	if reachableCount >= m.config.BootstrapExpect && m.nodeID == lowestReachableNodeID {
		logger.RaftLog.Info("Bootstrapping new cluster",
			zap.Int("reachable", reachableCount),
			zap.Int("bootstrap_expect", m.config.BootstrapExpect),
		)

		return true, m.bootstrapCluster()
	}

	if reachableCount >= m.config.BootstrapExpect {
		logger.RaftLog.Debug("Waiting for lowest node-id to bootstrap",
			zap.Int("reachable", reachableCount),
			zap.Int("lowest_node_id", lowestReachableNodeID),
		)
	}

	return false, nil
}

// clusterHTTPDo dials a peer's cluster port over mTLS and performs a
// single HTTP request. Pass nil body for GET-style requests.
func (m *Manager) clusterHTTPDo(ctx context.Context, method, peerAddr, path string, body io.Reader) (*http.Response, error) {
	conn, err := m.clusterListener.Dial(ctx, peerAddr, listener.ALPNHTTP, discoveryHTTPTimeout)
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
// node ID, cluster ID, and schema version.
func (m *Manager) probePeer(ctx context.Context, peerAddr string) (peerState, int, string, int) {
	resp, err := m.clusterHTTPDo(ctx, http.MethodGet, peerAddr, "/cluster/status", nil)
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

// joinCluster POSTs our membership to a peer's cluster port.
func (m *Manager) joinCluster(ctx context.Context, peerAddr string, clusterID string) error {
	payload := struct {
		NodeID        int    `json:"nodeId"`
		RaftAddress   string `json:"raftAddress"`
		APIAddress    string `json:"apiAddress"`
		ClusterID     string `json:"clusterId"`
		SchemaVersion int    `json:"schemaVersion"`
		Suffrage      string `json:"suffrage,omitempty"`
	}{
		NodeID:        m.nodeID,
		RaftAddress:   string(m.transport.LocalAddr()),
		APIAddress:    m.config.APIAddress,
		ClusterID:     clusterID,
		SchemaVersion: m.config.SchemaVersion,
		Suffrage:      m.config.InitialSuffrage,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal join request: %w", err)
	}

	resp, err := m.clusterHTTPDo(ctx, http.MethodPost, peerAddr, "/cluster/members", bytes.NewReader(body))
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
