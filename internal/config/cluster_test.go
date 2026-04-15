package config

import (
	"strings"
	"testing"
	"time"
)

func validClusterYaml() ClusterYaml {
	return ClusterYaml{
		Enabled:             true,
		NodeID:              1,
		BindAddress:         "10.0.0.1:7000",
		AdvertiseAPIAddress: "https://10.0.0.1:5002",
		BootstrapExpect:     3,
		Peers: []string{
			"https://10.0.0.1:5002",
			"https://10.0.0.2:5002",
			"https://10.0.0.3:5002",
		},
		JoinToken:        "my-secret-token-that-is-at-least-32-chars",
		JoinTimeout:      "30s",
		ProposeTimeout:   "5s",
		SnapshotInterval: "2m",
	}
}

func TestValidateCluster_Disabled(t *testing.T) {
	c := ClusterYaml{Enabled: false}

	got, err := validateCluster(c)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got.Enabled {
		t.Fatal("expected Enabled=false for disabled cluster")
	}
}

func TestValidateCluster_ValidHA(t *testing.T) {
	c := validClusterYaml()

	got, err := validateCluster(c)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !got.Enabled {
		t.Fatal("expected Enabled=true")
	}

	if got.NodeID != 1 {
		t.Fatalf("expected NodeID=1, got %d", got.NodeID)
	}

	if got.BindAddress != "10.0.0.1:7000" {
		t.Fatalf("expected BindAddress=10.0.0.1:7000, got %s", got.BindAddress)
	}

	if got.AdvertiseAPIAddress != "https://10.0.0.1:5002" {
		t.Fatalf("expected AdvertiseAPIAddress=https://10.0.0.1:5002, got %s", got.AdvertiseAPIAddress)
	}

	if got.BootstrapExpect != 3 {
		t.Fatalf("expected BootstrapExpect=3, got %d", got.BootstrapExpect)
	}

	if len(got.Peers) != 3 {
		t.Fatalf("expected 3 peers, got %d", len(got.Peers))
	}

	if got.JoinToken != "my-secret-token-that-is-at-least-32-chars" {
		t.Fatalf("expected JoinToken=my-secret-token-that-is-at-least-32-chars, got %s", got.JoinToken)
	}

	if got.JoinTimeout != 30*time.Second {
		t.Fatalf("expected JoinTimeout=30s, got %v", got.JoinTimeout)
	}

	if got.ProposeTimeout != 5*time.Second {
		t.Fatalf("expected ProposeTimeout=5s, got %v", got.ProposeTimeout)
	}

	if got.SnapshotInterval != 2*time.Minute {
		t.Fatalf("expected SnapshotInterval=2m, got %v", got.SnapshotInterval)
	}
}

func TestValidateCluster_OptionalDurationsOmitted(t *testing.T) {
	c := validClusterYaml()
	c.JoinTimeout = ""
	c.ProposeTimeout = ""
	c.SnapshotInterval = ""

	got, err := validateCluster(c)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got.JoinTimeout != 0 {
		t.Fatalf("expected zero JoinTimeout, got %v", got.JoinTimeout)
	}

	if got.ProposeTimeout != 0 {
		t.Fatalf("expected zero ProposeTimeout, got %v", got.ProposeTimeout)
	}

	if got.SnapshotInterval != 0 {
		t.Fatalf("expected zero SnapshotInterval, got %v", got.SnapshotInterval)
	}
}

func TestValidateCluster_MaxNodeID(t *testing.T) {
	c := validClusterYaml()
	c.NodeID = maxClusterNodeID

	_, err := validateCluster(c)
	if err != nil {
		t.Fatalf("node-id=%d should be valid: %v", maxClusterNodeID, err)
	}
}

func TestValidateCluster_Errors(t *testing.T) {
	tests := []struct {
		name    string
		modify  func(*ClusterYaml)
		wantErr string
	}{
		{
			name:    "node-id zero",
			modify:  func(c *ClusterYaml) { c.NodeID = 0 },
			wantErr: "cluster.node-id must be between 1 and 63",
		},
		{
			name:    "node-id too large",
			modify:  func(c *ClusterYaml) { c.NodeID = 64 },
			wantErr: "cluster.node-id must be between 1 and 63",
		},
		{
			name:    "negative node-id",
			modify:  func(c *ClusterYaml) { c.NodeID = -1 },
			wantErr: "cluster.node-id must be between 1 and 63",
		},
		{
			name:    "empty bind-address",
			modify:  func(c *ClusterYaml) { c.BindAddress = "" },
			wantErr: "cluster.bind-address is required",
		},
		{
			name:    "invalid bind-address",
			modify:  func(c *ClusterYaml) { c.BindAddress = "not-host-port" },
			wantErr: "cluster.bind-address",
		},
		{
			name:    "empty advertise-api-address",
			modify:  func(c *ClusterYaml) { c.AdvertiseAPIAddress = "" },
			wantErr: "cluster.advertise-api-address is required",
		},
		{
			name: "invalid advertise-api-address",
			modify: func(c *ClusterYaml) {
				c.AdvertiseAPIAddress = "://bad"
				c.Peers[0] = "://bad"
			},
			wantErr: "cluster.advertise-api-address",
		},
		{
			name:    "bootstrap-expect zero",
			modify:  func(c *ClusterYaml) { c.BootstrapExpect = 0 },
			wantErr: "cluster.bootstrap-expect must be >= 1",
		},
		{
			name:    "empty peers",
			modify:  func(c *ClusterYaml) { c.Peers = nil },
			wantErr: "cluster.peers must not be empty",
		},
		{
			name:    "peers less than bootstrap-expect",
			modify:  func(c *ClusterYaml) { c.BootstrapExpect = 5 },
			wantErr: "peers must be >= bootstrap-expect",
		},
		{
			name: "invalid peer URL",
			modify: func(c *ClusterYaml) {
				c.Peers = append(c.Peers, "://bad-url")
			},
			wantErr: "cluster.peers[3]",
		},
		{
			name: "peers missing self",
			modify: func(c *ClusterYaml) {
				c.Peers = []string{
					"https://10.0.0.2:5002",
					"https://10.0.0.3:5002",
					"https://10.0.0.4:5002",
				}
			},
			wantErr: "must include this node's advertise-api-address",
		},
		{
			name:    "empty join-token",
			modify:  func(c *ClusterYaml) { c.JoinToken = "" },
			wantErr: "cluster.join-token is required",
		},
		{
			name:    "invalid join-timeout",
			modify:  func(c *ClusterYaml) { c.JoinTimeout = "notaduration" },
			wantErr: "cluster.join-timeout",
		},
		{
			name:    "invalid propose-timeout",
			modify:  func(c *ClusterYaml) { c.ProposeTimeout = "xyz" },
			wantErr: "cluster.propose-timeout",
		},
		{
			name:    "invalid snapshot-interval",
			modify:  func(c *ClusterYaml) { c.SnapshotInterval = "bad" },
			wantErr: "cluster.snapshot-interval",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c := validClusterYaml()
			tc.modify(&c)

			_, err := validateCluster(c)
			if err == nil {
				t.Fatal("expected error, got nil")
			}

			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("error %q does not contain %q", err.Error(), tc.wantErr)
			}
		})
	}
}
