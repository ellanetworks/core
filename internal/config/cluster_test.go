// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package config_test

import (
	"errors"
	"net"
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

func TestCluster_RemovedJoinFieldsAreIgnored(t *testing.T) {
	core, logs := observer.New(zapcore.WarnLevel)

	restore := logger.EllaLog
	logger.EllaLog = zap.New(core)

	t.Cleanup(func() { logger.EllaLog = restore })

	cfg, err := config.Validate(writeConfig(t, baseConfigYAML+
		"  join-token: \""+strings.Repeat("a", 100)+"\"\n  initial-suffrage: \"nonvoter\"\n"))
	if err != nil {
		t.Fatalf("removed fields must not stop an upgraded node from starting: %v", err)
	}

	if len(cfg.Cluster.Join) != 0 {
		t.Errorf("cluster.peers must not turn into cluster.join, got %v", cfg.Cluster.Join)
	}

	for _, key := range []string{"cluster.peers", "cluster.join-token", "cluster.initial-suffrage"} {
		found := false

		for _, entry := range logs.All() {
			if strings.Contains(entry.Message, key+" was removed and is ignored") {
				found = true
			}
		}

		if !found {
			t.Errorf("expected a warning that %s is ignored", key)
		}
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

	if cfg.Cluster.Enabled || cfg.Cluster.NodeID != 0 || len(cfg.Cluster.Join) != 0 {
		t.Errorf("without a cluster address the block must be discarded, got %+v", cfg.Cluster)
	}
}

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

func writeSecret(t *testing.T, body string) string {
	t.Helper()

	p := filepath.Join(t.TempDir(), "cluster.secret")
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}

	return p
}

const validSecret = "MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY=\n"

func formationConfig(join, extra string) string {
	return strings.Replace(baseConfigYAML, `  peers:
    - "127.0.0.1:7000"
    - "127.0.0.1:7001"
    - "127.0.0.1:7002"
`, join, 1) + extra
}

func TestCluster_JoinParses(t *testing.T) {
	secret := writeSecret(t, validSecret)
	body := formationConfig(`  join: ["10.0.0.1", "10.0.0.2:7100", "core-2.example", "[fd00::3]"]
  bootstrap-expect: 3
  secret-file: "`+secret+`"
`, "")

	cfg, err := config.Validate(writeConfig(t, body))
	if err != nil {
		t.Fatalf("validate: %v", err)
	}

	want := []string{"10.0.0.1:7000", "10.0.0.2:7100", "core-2.example:7000", "[fd00::3]:7000"}
	if strings.Join(cfg.Cluster.Join, ",") != strings.Join(want, ",") {
		t.Fatalf("join = %v, want %v (a target without a port takes the bind port)", cfg.Cluster.Join, want)
	}

	if cfg.Cluster.BootstrapExpect != 3 || cfg.Cluster.SecretFile != secret {
		t.Fatalf("bootstrap-expect = %d, secret-file = %q", cfg.Cluster.BootstrapExpect, cfg.Cluster.SecretFile)
	}
}

func TestCluster_JoinValidation(t *testing.T) {
	secret := writeSecret(t, validSecret)
	short := writeSecret(t, "c2hvcnQ=\n")

	cases := []struct {
		name, block, want string
	}{
		{"join without secret", "  join: [\"10.0.0.1\"]\n", "cluster.join requires cluster.secret-file"},
		{"expect without join", "  bootstrap-expect: 3\n  secret-file: \"" + secret + "\"\n", "requires cluster.join"},
		{"expect of one", "  join: [\"10.0.0.1\"]\n  bootstrap-expect: 1\n  secret-file: \"" + secret + "\"\n", "2 or more"},
		{"url", "  join: [\"https://10.0.0.1:7000\"]\n  secret-file: \"" + secret + "\"\n", "looks like a URL"},
		{"unspecified", "  join: [\"0.0.0.0:7000\"]\n  secret-file: \"" + secret + "\"\n", "unspecified"},
		{"short secret", "  join: [\"10.0.0.1\"]\n  secret-file: \"" + short + "\"\n", "at least 32"},
		{"missing secret", "  join: [\"10.0.0.1\"]\n  secret-file: \"/nonexistent/secret\"\n", "cluster.secret-file"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := config.Validate(writeConfig(t, formationConfig(tc.block, "")))
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err = %v, want it to contain %q", err, tc.want)
			}
		})
	}
}

func TestCluster_AdvertiseDefaultsToRouteSource(t *testing.T) {
	restore := config.RouteSourceFunc

	t.Cleanup(func() { config.RouteSourceFunc = restore })

	config.RouteSourceFunc = func(ipv6 bool) (net.IP, error) {
		if ipv6 {
			return net.ParseIP("fd00::10"), nil
		}

		return net.ParseIP("10.128.0.10"), nil
	}

	cases := map[string]string{
		`":7000"`:         "10.128.0.10:7000",
		`"0.0.0.0:7000"`:  "10.128.0.10:7000",
		`"[::]:7000"`:     "[fd00::10]:7000",
		`"10.0.0.5:7000"`: "10.0.0.5:7000",
	}

	for bind, want := range cases {
		body := strings.Replace(baseConfigYAML, `bind-address: "127.0.0.1:7000"`, "bind-address: "+bind, 1)

		cfg, err := config.Validate(writeConfig(t, body))
		if err != nil {
			t.Fatalf("bind %s: %v", bind, err)
		}

		if cfg.Cluster.AdvertiseAddress != want {
			t.Errorf("bind %s: advertise = %q, want %q", bind, cfg.Cluster.AdvertiseAddress, want)
		}
	}
}

func TestCluster_AdvertiseWithoutRouteFails(t *testing.T) {
	restore := config.RouteSourceFunc

	t.Cleanup(func() { config.RouteSourceFunc = restore })

	config.RouteSourceFunc = func(bool) (net.IP, error) { return nil, errors.New("network is unreachable") }

	body := strings.Replace(baseConfigYAML, `bind-address: "127.0.0.1:7000"`, `bind-address: "0.0.0.0:7000"`, 1)

	_, err := config.Validate(writeConfig(t, body))
	if err == nil || !strings.Contains(err.Error(), "set cluster.advertise-address") {
		t.Fatalf("err = %v, want a hint to set cluster.advertise-address", err)
	}
}

func TestCluster_ExplicitAdvertiseMustBeSpecific(t *testing.T) {
	for _, adv := range []string{`"0.0.0.0:7000"`, `":7000"`} {
		body := baseConfigYAML + "  advertise-address: " + adv + "\n"

		if _, err := config.Validate(writeConfig(t, body)); err == nil {
			t.Errorf("advertise-address %s must be rejected", adv)
		}
	}
}
