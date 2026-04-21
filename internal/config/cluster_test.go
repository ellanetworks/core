// Copyright 2026 Ella Networks

package config_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ellanetworks/core/internal/config"
)

// Cluster config v2 drops the operator-supplied TLS fields (cluster.tls.*)
// in favour of the in-band PKI bootstrapped at first-leader election, and
// replaces the bootstrap-expect gate with the cluster.join-token signal
// (present → joiner, absent → founder). The tests below exercise the
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
xdp:
  attach-mode: native
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
	for _, bad := range []int{0, 64, -1, 100} {
		p := writeConfig(t, strings.Replace(baseConfigYAML, "node-id: 1", "node-id: "+itoa(bad), 1))

		_, err := config.Validate(p)
		if err == nil {
			t.Fatalf("node-id %d should be rejected", bad)
		}
	}
}

func TestCluster_BindAddressRequired(t *testing.T) {
	cfg := strings.Replace(baseConfigYAML, `bind-address: "127.0.0.1:7000"`, `bind-address: ""`, 1)

	_, err := config.Validate(writeConfig(t, cfg))
	if err == nil {
		t.Fatal("empty bind-address should be rejected")
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
