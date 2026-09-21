// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package config_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ellanetworks/core/internal/config"
	"github.com/ellanetworks/core/internal/logger"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

// Cluster config v2 drops the operator-supplied TLS fields (cluster.tls.*)
// in favour of the in-band PKI bootstrapped at first-leader election, and
// leaves cluster formation to the API. The tests below exercise the
// remaining surface: node-id range, bind address format, peers list,
// suffrage, timeouts.

const baseConfigYAML = `
db:
  path: /tmp/ella.db
interfaces:
  n2: { address: "127.0.0.1", port: 38412 }
  n3: { name: "lo" }
  n6: { name: "lo" }
  api: { address: "127.0.0.1", port: 5002 }
datapath:
  attach-mode: xdp-native
logging:
  system: { level: info, output: stdout }
  audit: { output: stdout }
cluster:
  enabled: true
  node-id: 1
  bind-address: "127.0.0.1:7000"
  peers:
    - "127.0.0.1:7000"
    - "127.0.0.1:7001"
    - "127.0.0.1:7002"
`

func writeConfig(t *testing.T, body string) string {
	t.Helper()

	p := filepath.Join(t.TempDir(), "core.yaml")
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}

	return p
}

func TestCluster_ValidateBase(t *testing.T) {
	p := writeConfig(t, baseConfigYAML)

	cfg, err := config.Validate(p)
	if err != nil {
		t.Fatalf("validate: %v", err)
	}

	if !cfg.Cluster.Enabled {
		t.Fatal("cluster should be enabled")
	}

	if cfg.Cluster.NodeID != 1 {
		t.Fatalf("node-id = %d", cfg.Cluster.NodeID)
	}

	if cfg.Cluster.BindAddress != "127.0.0.1:7000" {
		t.Fatalf("bind-address = %q", cfg.Cluster.BindAddress)
	}

	if len(cfg.Cluster.Peers) != 3 {
		t.Fatalf("peers = %d", len(cfg.Cluster.Peers))
	}
}

func TestCluster_NodeIDRange(t *testing.T) {
	for _, bad := range []int{64, -1, 100} {
		p := writeConfig(t, strings.Replace(baseConfigYAML, "node-id: 1", "node-id: "+itoa(bad), 1))

		_, err := config.Validate(p)
		if err == nil {
			t.Fatalf("node-id %d should be rejected", bad)
		}
	}
}

// node-id no longer names the node: a config that omits it is valid, and
// one that carries it is accepted so an upgrade does not fail at boot.
func TestCluster_NodeIDOptional(t *testing.T) {
	withoutNodeID := strings.Replace(baseConfigYAML, "  node-id: 1\n", "", 1)

	cfg, err := config.Validate(writeConfig(t, withoutNodeID))
	if err != nil {
		t.Fatalf("a config without node-id must validate: %v", err)
	}

	if cfg.Cluster.NodeID != 0 {
		t.Fatalf("cluster.node-id = %d, want 0 when absent", cfg.Cluster.NodeID)
	}

	cfg, err = config.Validate(writeConfig(t, baseConfigYAML))
	if err != nil {
		t.Fatalf("a config carrying the deprecated node-id must still validate: %v", err)
	}

	if cfg.Cluster.NodeID != 1 {
		t.Fatalf("cluster.node-id = %d, want the parsed 1", cfg.Cluster.NodeID)
	}
}

func TestCluster_BindAddressSwitchesClusteringOn(t *testing.T) {
	body := strings.Replace(baseConfigYAML, `bind-address: "127.0.0.1:7000"`, `bind-address: ""`, 1)

	cfg, err := config.Validate(writeConfig(t, body))
	if err != nil {
		t.Fatalf("a node without a cluster address must still start: %v", err)
	}

	if cfg.Cluster.Enabled {
		t.Error("no cluster address means no clustering")
	}

	cfg, err = config.Validate(writeConfig(t, baseConfigYAML))
	if err != nil {
		t.Fatalf("validate: %v", err)
	}

	if !cfg.Cluster.Enabled {
		t.Error("a cluster address is what turns clustering on")
	}
}

func TestCluster_SnapshotTunablesOptional(t *testing.T) {
	cfg, err := config.Validate(writeConfig(t, baseConfigYAML))
	if err != nil {
		t.Fatalf("validate: %v", err)
	}

	if cfg.Cluster.SnapshotInterval != 0 {
		t.Fatalf("snapshot-interval default = %s, want 0", cfg.Cluster.SnapshotInterval)
	}

	if cfg.Cluster.SnapshotThreshold != 0 {
		t.Fatalf("snapshot-threshold default = %d, want 0", cfg.Cluster.SnapshotThreshold)
	}

	if cfg.Cluster.TrailingLogs != 0 {
		t.Fatalf("trailing-logs default = %d, want 0", cfg.Cluster.TrailingLogs)
	}
}

func TestCluster_SnapshotTunablesParse(t *testing.T) {
	body := baseConfigYAML + `  snapshot-interval: "2s"
  snapshot-threshold: 50
  trailing-logs: 5
`

	cfg, err := config.Validate(writeConfig(t, body))
	if err != nil {
		t.Fatalf("validate: %v", err)
	}

	if cfg.Cluster.SnapshotInterval.String() != "2s" {
		t.Fatalf("snapshot-interval = %s, want 2s", cfg.Cluster.SnapshotInterval)
	}

	if cfg.Cluster.SnapshotThreshold != 50 {
		t.Fatalf("snapshot-threshold = %d, want 50", cfg.Cluster.SnapshotThreshold)
	}

	if cfg.Cluster.TrailingLogs != 5 {
		t.Fatalf("trailing-logs = %d, want 5", cfg.Cluster.TrailingLogs)
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}

	if n < 0 {
		return "-" + itoa(-n)
	}

	out := ""

	for n > 0 {
		out = string(rune('0'+n%10)) + out
		n /= 10
	}

	return out
}

func TestCluster_PeersOptionalWithoutJoinToken(t *testing.T) {
	body := strings.Replace(baseConfigYAML, `  peers:
    - "127.0.0.1:7000"
    - "127.0.0.1:7001"
    - "127.0.0.1:7002"
`, "", 1)

	cfg, err := config.Validate(writeConfig(t, body))
	if err != nil {
		t.Fatalf("peers must be optional when the node is not joining from config: %v", err)
	}

	if len(cfg.Cluster.Peers) != 0 {
		t.Errorf("expected no peers, got %v", cfg.Cluster.Peers)
	}
}

func TestCluster_JoinTokenRequiresPeers(t *testing.T) {
	body := strings.Replace(baseConfigYAML, `  peers:
    - "127.0.0.1:7000"
    - "127.0.0.1:7001"
    - "127.0.0.1:7002"
`, "", 1)

	_, err := config.Validate(writeConfig(t, body+"  join-token: \""+strings.Repeat("a", 100)+"\"\n"))
	if err == nil {
		t.Fatal("a config-file join token has nowhere to go without peers")
	}

	if !strings.Contains(err.Error(), "cluster.peers is empty") {
		t.Errorf("error should name the missing peers, got %q", err)
	}
}

func TestCluster_DeprecatedFieldsStillAccepted(t *testing.T) {
	token := strings.Repeat("a", 100)

	cfg, err := config.Validate(writeConfig(t, baseConfigYAML+
		"  join-token: \""+token+"\"\n  initial-suffrage: \"nonvoter\"\n"))
	if err != nil {
		t.Fatalf("deprecated fields must keep working for a release: %v", err)
	}

	if cfg.Cluster.JoinToken != token {
		t.Error("join-token must survive validation")
	}

	if cfg.Cluster.InitialSuffrage != "nonvoter" {
		t.Error("initial-suffrage must survive validation")
	}

	if len(cfg.Cluster.Peers) != 3 {
		t.Error("peers must survive validation")
	}
}

func TestCluster_EnabledIsIgnored(t *testing.T) {
	body := strings.Replace(baseConfigYAML, "  enabled: true\n", "  enabled: false\n", 1)

	cfg, err := config.Validate(writeConfig(t, body))
	if err != nil {
		t.Fatalf("validate: %v", err)
	}

	if !cfg.Cluster.Enabled {
		t.Error("cluster.enabled is deprecated; a node with a cluster address clusters regardless")
	}
}

func TestCluster_NoBindAddressDiscardsClusterSettings(t *testing.T) {
	body := strings.Replace(baseConfigYAML, `bind-address: "127.0.0.1:7000"`, `bind-address: ""`, 1)

	cfg, err := config.Validate(writeConfig(t, body))
	if err != nil {
		t.Fatalf("validate: %v", err)
	}

	if cfg.Cluster.Enabled || cfg.Cluster.NodeID != 0 || len(cfg.Cluster.Peers) != 0 {
		t.Errorf("without a cluster address the block must be discarded, got %+v", cfg.Cluster)
	}
}

// A config that asks for HA but omits the bind address runs standalone, and
// the operator has to be told which settings were ignored.
func TestCluster_EnabledWithoutBindAddressIsWarnedAbout(t *testing.T) {
	core, logs := observer.New(zapcore.WarnLevel)

	restore := logger.EllaLog
	logger.EllaLog = zap.New(core)

	t.Cleanup(func() { logger.EllaLog = restore })

	body := strings.Replace(baseConfigYAML, `bind-address: "127.0.0.1:7000"`, `bind-address: ""`, 1)

	if _, err := config.Validate(writeConfig(t, body)); err != nil {
		t.Fatalf("a node without a cluster address must still start: %v", err)
	}

	found := false

	for _, entry := range logs.All() {
		if strings.Contains(entry.Message, "cluster.bind-address is not set") {
			found = true

			if !strings.Contains(entry.Message, "cluster.enabled") {
				t.Errorf("the warning must name cluster.enabled as ignored, got %q", entry.Message)
			}

			if strings.Contains(entry.Message, "node ID 1") {
				t.Errorf("nodes mint their own identity; the warning must not promise node ID 1, got %q", entry.Message)
			}
		}
	}

	if !found {
		t.Fatal("a config with cluster.enabled and no bind-address must warn")
	}
}
