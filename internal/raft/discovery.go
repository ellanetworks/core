// Copyright 2026 Ella Networks

package raft

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

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
	Role      string `json:"role"`
	NodeID    int    `json:"nodeId"`
	ClusterID string `json:"clusterId"`
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
// the HTTP server starts so peers can reach this node's API. In standalone
// mode or when resuming existing Raft state, it returns immediately.
func (m *Manager) RunDiscovery(ctx context.Context) error {
	if !m.needsDiscovery {
		return nil
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

	client := &http.Client{
		Timeout: discoveryHTTPTimeout,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true, // #nosec G402 — peers use self-signed certs during cluster formation
			},
		},
	}

	ticker := time.NewTicker(discoveryPollInterval)
	defer ticker.Stop()

	for {
		joined, err := m.discoveryTick(ctx, client)
		if err != nil {
			return err
		}

		if joined {
			m.needsDiscovery = false
			return m.waitForLeader(ctx)
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
func (m *Manager) discoveryTick(ctx context.Context, client *http.Client) (bool, error) {
	reachableCount := 1 // count ourselves
	lowestReachableNodeID := m.nodeID

	for _, peerURL := range m.config.Peers {
		if peerURL == m.config.AdvertiseAPIAddress {
			continue
		}

		state, nodeID, clusterID := m.probePeer(ctx, client, peerURL)

		switch state {
		case peerFormed:
			if err := m.joinCluster(ctx, client, peerURL, clusterID); err != nil {
				logger.RaftLog.Warn("Failed to join cluster via peer",
					zap.String("peer", peerURL),
					zap.Error(err),
				)

				continue
			}

			return true, nil

		case peerForming:
			reachableCount++

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

// probePeer queries a peer's status endpoint and returns its state and node ID.
func (m *Manager) probePeer(ctx context.Context, client *http.Client, peerURL string) (peerState, int, string) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, peerURL+"/api/v1/status", nil)
	if err != nil {
		return peerUnreachable, 0, ""
	}

	resp, err := client.Do(req)
	if err != nil {
		return peerUnreachable, 0, ""
	}

	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return peerForming, 0, ""
	}

	var status statusResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, 4096)).Decode(&status); err != nil {
		return peerForming, 0, ""
	}

	if status.Result.Cluster == nil {
		return peerForming, 0, ""
	}

	nodeID := status.Result.Cluster.NodeID
	role := status.Result.Cluster.Role
	clusterID := status.Result.Cluster.ClusterID

	if role == "Leader" || role == "Follower" {
		return peerFormed, nodeID, clusterID
	}

	return peerForming, nodeID, clusterID
}

// joinCluster POSTs our membership to a peer (proxied to leader via the
// LeaderProxyMiddleware).
func (m *Manager) joinCluster(ctx context.Context, client *http.Client, peerURL string, clusterID string) error {
	body := fmt.Sprintf(`{"nodeId":%d,"raftAddress":%q,"apiAddress":%q,"clusterId":%q}`,
		m.nodeID,
		string(m.transport.LocalAddr()),
		m.config.AdvertiseAPIAddress,
		clusterID,
	)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		peerURL+"/api/v1/cluster/members", strings.NewReader(body))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Ella-Cluster-Token", m.config.JoinToken)

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}

	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("server returned %d: %s", resp.StatusCode, string(respBody))
	}

	logger.RaftLog.Info("Joined existing cluster via peer",
		zap.String("peer", peerURL),
		zap.Int("node_id", m.nodeID),
	)

	return nil
}

// bootstrapCluster creates the initial Raft cluster with this node as the
// sole voter. Other nodes will join via AddVoter as they discover the leader.
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
