// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package raft

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"

	"github.com/ellanetworks/core/internal/cluster/listener"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/pki"
	"github.com/hashicorp/raft"
	"go.uber.org/zap"
)

const (
	discoveryPollInterval = 1 * time.Second
	defaultJoinTimeout    = 2 * time.Minute
	discoveryHTTPTimeout  = 5 * time.Second

	discoveryErrLogInterval = 30 * time.Second
)

var ErrDiscoveryFatal = errors.New("fatal cluster discovery error")

type peerState int

const (
	peerUnreachable peerState = iota
	peerForming
	peerFormed
)

// statusClusterBlock mirrors the cluster block of the status API response.
type statusClusterBlock struct {
	Role          string     `json:"role"`
	NodeID        pki.NodeID `json:"nodeId"`
	ClusterID     string     `json:"clusterId"`
	SchemaVersion int        `json:"schemaVersion"`
}

type statusResult struct {
	Cluster *statusClusterBlock `json:"cluster"`
}

type statusResponse struct {
	Result statusResult `json:"result"`
}

// StartDiscovery performs cluster formation for HA mode.
func (m *Manager) StartDiscovery(ctx context.Context) error {
	if !m.discoveryPending.Load() {
		return nil
	}

	if m.clusterListener == nil {
		return fmt.Errorf("%w: cluster discovery requires a cluster listener (mTLS)", ErrDiscoveryFatal)
	}

	if !m.config.HasJoinToken {
		if !m.config.Bootstrap {
			return fmt.Errorf("%w: this node has no cluster state and no way to join one; POST /api/v1/cluster/bootstrap to found a new cluster, or send a join token to POST /api/v1/cluster/join", ErrDiscoveryFatal)
		}

		logger.RaftLog.Info("Founding a new cluster",
			zap.String("node_id", m.raftID),
		)

		if err := m.bootstrapCluster(); err != nil {
			return fmt.Errorf("%w: %w", ErrDiscoveryFatal, err)
		}

		m.discoveryPending.Store(false)

		return nil
	}

	if err := m.joinExistingCluster(ctx); err != nil {
		return err
	}

	m.discoveryPending.Store(false)

	return nil
}

func (m *Manager) joinExistingCluster(parent context.Context) error {
	giveUpAfter := m.config.JoinTimeout
	if giveUpAfter == 0 {
		giveUpAfter = defaultJoinTimeout
	}

	logger.RaftLog.Info("Starting cluster discovery",
		zap.String("node_id", m.raftID),
		zap.Int("peer_count", len(m.config.Peers)),
		zap.Duration("give_up_after", giveUpAfter),
	)

	ctx, cancel := context.WithTimeout(parent, giveUpAfter)
	defer cancel()

	started := time.Now()

	var lastErr error

	var lastErrLog time.Time

	ticker := time.NewTicker(discoveryPollInterval)
	defer ticker.Stop()

	for {
		joined, err := m.discoveryTick(ctx)

		if errors.Is(err, ErrDiscoveryFatal) {
			return err
		}

		if err != nil {
			lastErr = err

			if time.Since(lastErrLog) >= discoveryErrLogInterval {
				lastErrLog = time.Now()

				logger.RaftLog.Error("Cluster discovery attempt failed; retrying",
					zap.String("node_id", m.raftID),
					zap.Error(err),
				)
			}
		}

		if err == nil && joined {
			logger.RaftLog.Info("Cluster formation complete",
				zap.String("node_id", m.raftID),
				zap.Duration("took", time.Since(started)),
			)

			return nil
		}

		select {
		case <-ctx.Done():
			if parent.Err() != nil {
				return parent.Err()
			}

			if lastErr != nil {
				return fmt.Errorf("%w: could not join a cluster within %s: %w", ErrDiscoveryFatal, giveUpAfter, lastErr)
			}

			return fmt.Errorf("%w: no peer in cluster.peers had formed a cluster within %s", ErrDiscoveryFatal, giveUpAfter)
		case <-ticker.C:
		}
	}
}

func (m *Manager) discoveryTick(ctx context.Context) (bool, error) {
	var skipped error

	for _, peerAddr := range m.config.Peers {
		if peerAddr == m.config.AdvertiseAddress {
			continue
		}

		state, nodeID, clusterID, peerSchema := m.probePeer(ctx, peerAddr)

		if state == peerUnreachable {
			continue
		}

		if nodeID != "" && nodeID == m.raftID {
			return false, fmt.Errorf("%w: peer %s advertises the same node-id (%s) as this node; its data directory was copied from this one", ErrDiscoveryFatal, peerAddr, nodeID)
		}

		if state != peerFormed {
			continue
		}

		if m.config.SchemaVersion < peerSchema {
			logger.RaftLog.Warn("Schema version lower than peer, skipping (downgrade)",
				zap.String("peer", peerAddr),
				zap.Int("local_schema", m.config.SchemaVersion),
				zap.Int("remote_schema", peerSchema),
			)

			skipped = fmt.Errorf("peer %s runs schema %d, newer than this node's %d", peerAddr, peerSchema, m.config.SchemaVersion)

			continue
		}

		if err := m.joinCluster(ctx, peerAddr, nodeID, clusterID); err != nil {
			logger.RaftLog.Warn("Failed to join cluster via peer",
				zap.String("peer", peerAddr),
				zap.Error(err),
			)

			skipped = fmt.Errorf("peer %s rejected the join: %w", peerAddr, err)

			continue
		}

		return true, nil
	}

	// No formed peer found this tick. The joiner keeps polling.
	return false, skipped
}

// clusterHTTPDo dials a peer's cluster port over mTLS and performs a
// single HTTP request. Pass nil body for GET-style requests. When
// expectedPeerID is non-empty the dial verifies the peer's leaf CN
// resolves to that node-id; pass an empty string only from discovery
// paths that are still learning the peer's identity.
func (m *Manager) clusterHTTPDo(ctx context.Context, method, peerAddr string, expectedPeerID string, path string, body io.Reader) (*http.Response, error) {
	var (
		conn net.Conn
		err  error
	)

	if expectedPeerID == "" {
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
		DisableKeepAlives: true,
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

func (m *Manager) probePeer(ctx context.Context, peerAddr string) (peerState, string, string, int) {
	resp, err := m.clusterHTTPDo(ctx, http.MethodGet, peerAddr, "", "/cluster/status", nil)
	if err != nil {
		return peerUnreachable, "", "", 0
	}

	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return peerUnreachable, "", "", 0
	}

	var status statusResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, 4096)).Decode(&status); err != nil {
		return peerUnreachable, "", "", 0
	}

	if status.Result.Cluster == nil {
		return peerForming, "", "", 0
	}

	nodeID := string(status.Result.Cluster.NodeID)
	role := status.Result.Cluster.Role
	clusterID := status.Result.Cluster.ClusterID
	schemaVersion := status.Result.Cluster.SchemaVersion

	if clusterID != "" && (role == "Leader" || role == "Follower") {
		return peerFormed, nodeID, clusterID, schemaVersion
	}

	return peerForming, nodeID, clusterID, schemaVersion
}

// ProbePeerSchemaVersion reads the peer's reported Schema Version from
// its /cluster/status endpoint.
func (m *Manager) ProbePeerSchemaVersion(ctx context.Context, peerNodeID string, peerAddr string) (int, error) {
	if peerNodeID == "" {
		return 0, fmt.Errorf("peer node id required")
	}

	if peerAddr == "" {
		return 0, fmt.Errorf("peer raft address required")
	}

	resp, err := m.clusterHTTPDo(ctx, http.MethodGet, peerAddr, peerNodeID, "/cluster/status", nil)
	if err != nil {
		return 0, fmt.Errorf("dial peer %s: %w", peerAddr, err)
	}

	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return 0, fmt.Errorf("peer %s returned %d: %s", peerAddr, resp.StatusCode, string(body))
	}

	var status statusResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, 4096)).Decode(&status); err != nil {
		return 0, fmt.Errorf("decode status from peer %s: %w", peerAddr, err)
	}

	if status.Result.Cluster == nil {
		return 0, fmt.Errorf("peer %s reported no cluster status", peerAddr)
	}

	v := status.Result.Cluster.SchemaVersion
	if v < 1 {
		return 0, fmt.Errorf("peer %s reported invalid schema version %d", peerAddr, v)
	}

	return v, nil
}

func (m *Manager) joinCluster(ctx context.Context, peerAddr string, peerNodeID string, clusterID string) error {
	payload := struct {
		NodeID        pki.NodeID `json:"nodeId"`
		RaftAddress   string     `json:"raftAddress"`
		APIAddress    string     `json:"apiAddress"`
		ClusterID     string     `json:"clusterId"`
		SchemaVersion int        `json:"schemaVersion"`
		BinaryVersion string     `json:"binaryVersion,omitempty"`
		Suffrage      string     `json:"suffrage,omitempty"`
	}{
		NodeID:        pki.NodeID(m.raftID),
		RaftAddress:   string(m.transport.LocalAddr()),
		APIAddress:    m.config.APIAddress,
		ClusterID:     clusterID,
		SchemaVersion: m.config.SchemaVersion,
		BinaryVersion: m.config.BinaryVersion,
		Suffrage:      m.config.InitialSuffrage,
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
		zap.String("node_id", m.raftID),
	)

	return nil
}

// bootstrapCluster creates the initial Raft cluster with this node as the
// sole voter. Other nodes will join via AddVoter as they discover the leader.
func (m *Manager) bootstrapCluster() error {
	cfg := raft.Configuration{
		Servers: []raft.Server{{
			ID:      raft.ServerID(m.raftID),
			Address: m.transport.LocalAddr(),
		}},
	}

	if err := m.raft.BootstrapCluster(cfg).Error(); err != nil {
		return fmt.Errorf("bootstrap cluster: %w", err)
	}

	return nil
}
