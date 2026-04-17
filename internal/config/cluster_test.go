package config

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// testPKI holds file paths for a cluster CA and one node leaf certificate.
type testPKI struct {
	CAPath   string
	CertPath string
	KeyPath  string
}

// genTestPKI creates a self-signed CA and a leaf certificate with
// CN=ella-node-<nodeID> in dir. Files are written as PEM.
func genTestPKI(t *testing.T, dir string, nodeID int) testPKI {
	t.Helper()

	caKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate CA key: %v", err)
	}

	caTemplate := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "ella-test-ca"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(24 * time.Hour),
		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
	}

	caDER, err := x509.CreateCertificate(rand.Reader, caTemplate, caTemplate, &caKey.PublicKey, caKey)
	if err != nil {
		t.Fatalf("create CA cert: %v", err)
	}

	caPath := filepath.Join(dir, "ca.pem")
	writePEM(t, caPath, "CERTIFICATE", caDER)

	caCert, err := x509.ParseCertificate(caDER)
	if err != nil {
		t.Fatalf("parse CA cert: %v", err)
	}

	leafKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate leaf key: %v", err)
	}

	leafTemplate := &x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject:      pkix.Name{CommonName: fmt.Sprintf("ella-node-%d", nodeID)},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
	}

	leafDER, err := x509.CreateCertificate(rand.Reader, leafTemplate, caCert, &leafKey.PublicKey, caKey)
	if err != nil {
		t.Fatalf("create leaf cert: %v", err)
	}

	certPath := filepath.Join(dir, "node.pem")
	writePEM(t, certPath, "CERTIFICATE", leafDER)

	keyDER, err := x509.MarshalECPrivateKey(leafKey)
	if err != nil {
		t.Fatalf("marshal leaf key: %v", err)
	}

	keyPath := filepath.Join(dir, "node-key.pem")
	writePEM(t, keyPath, "EC PRIVATE KEY", keyDER)

	return testPKI{CAPath: caPath, CertPath: certPath, KeyPath: keyPath}
}

func writePEM(t *testing.T, path, blockType string, data []byte) {
	t.Helper()

	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create %s: %v", path, err)
	}

	defer func() { _ = f.Close() }()

	if err := pem.Encode(f, &pem.Block{Type: blockType, Bytes: data}); err != nil {
		t.Fatalf("write PEM %s: %v", path, err)
	}
}

func validClusterYaml(t *testing.T) ClusterYaml {
	t.Helper()

	dir := t.TempDir()
	pki := genTestPKI(t, dir, 1)

	return ClusterYaml{
		Enabled:         true,
		NodeID:          1,
		BindAddress:     "10.0.0.1:7000",
		BootstrapExpect: 3,
		Peers: []string{
			"10.0.0.1:7000",
			"10.0.0.2:7000",
			"10.0.0.3:7000",
		},
		TLS: ClusterTLSYaml{
			CA:   pki.CAPath,
			Cert: pki.CertPath,
			Key:  pki.KeyPath,
		},
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
	c := validClusterYaml(t)

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

	// advertise-address defaults to bind-address
	if got.AdvertiseAddress != "10.0.0.1:7000" {
		t.Fatalf("expected AdvertiseAddress=10.0.0.1:7000, got %s", got.AdvertiseAddress)
	}

	if got.BootstrapExpect != 3 {
		t.Fatalf("expected BootstrapExpect=3, got %d", got.BootstrapExpect)
	}

	if len(got.Peers) != 3 {
		t.Fatalf("expected 3 peers, got %d", len(got.Peers))
	}

	if got.TLS.CA == "" || got.TLS.Cert == "" || got.TLS.Key == "" {
		t.Fatal("expected TLS paths to be populated")
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

func TestValidateCluster_ExplicitAdvertiseAddress(t *testing.T) {
	c := validClusterYaml(t)
	c.AdvertiseAddress = "10.0.0.1:7000"

	got, err := validateCluster(c)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got.AdvertiseAddress != "10.0.0.1:7000" {
		t.Fatalf("expected AdvertiseAddress=10.0.0.1:7000, got %s", got.AdvertiseAddress)
	}
}

func TestValidateCluster_OptionalDurationsOmitted(t *testing.T) {
	c := validClusterYaml(t)
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
	c := validClusterYaml(t)
	c.NodeID = maxClusterNodeID

	// Re-generate PKI with the correct CN for node 63.
	dir := t.TempDir()
	pki := genTestPKI(t, dir, maxClusterNodeID)
	c.TLS.CA = pki.CAPath
	c.TLS.Cert = pki.CertPath
	c.TLS.Key = pki.KeyPath

	_, err := validateCluster(c)
	if err != nil {
		t.Fatalf("node-id=%d should be valid: %v", maxClusterNodeID, err)
	}
}

func TestValidateCluster_Errors(t *testing.T) {
	tests := []struct {
		name    string
		modify  func(t *testing.T, c *ClusterYaml)
		wantErr string
	}{
		{
			name:    "node-id zero",
			modify:  func(_ *testing.T, c *ClusterYaml) { c.NodeID = 0 },
			wantErr: "cluster.node-id must be between 1 and 63",
		},
		{
			name:    "node-id too large",
			modify:  func(_ *testing.T, c *ClusterYaml) { c.NodeID = 64 },
			wantErr: "cluster.node-id must be between 1 and 63",
		},
		{
			name:    "negative node-id",
			modify:  func(_ *testing.T, c *ClusterYaml) { c.NodeID = -1 },
			wantErr: "cluster.node-id must be between 1 and 63",
		},
		{
			name:    "empty bind-address",
			modify:  func(_ *testing.T, c *ClusterYaml) { c.BindAddress = "" },
			wantErr: "cluster.bind-address is required",
		},
		{
			name:    "invalid bind-address",
			modify:  func(_ *testing.T, c *ClusterYaml) { c.BindAddress = "not-host-port" },
			wantErr: "cluster.bind-address",
		},
		{
			name: "invalid advertise-address",
			modify: func(_ *testing.T, c *ClusterYaml) {
				c.AdvertiseAddress = "not-host-port"
			},
			wantErr: "cluster.advertise-address",
		},
		{
			name:    "bootstrap-expect zero",
			modify:  func(_ *testing.T, c *ClusterYaml) { c.BootstrapExpect = 0 },
			wantErr: "cluster.bootstrap-expect must be >= 1",
		},
		{
			name:    "empty peers",
			modify:  func(_ *testing.T, c *ClusterYaml) { c.Peers = nil },
			wantErr: "cluster.peers must not be empty",
		},
		{
			name:    "peers less than bootstrap-expect",
			modify:  func(_ *testing.T, c *ClusterYaml) { c.BootstrapExpect = 5 },
			wantErr: "peers must be >= bootstrap-expect",
		},
		{
			name: "peer is URL not host:port",
			modify: func(_ *testing.T, c *ClusterYaml) {
				c.Peers = append(c.Peers, "https://10.0.0.4:5002")
			},
			wantErr: "looks like a URL",
		},
		{
			name: "invalid peer",
			modify: func(_ *testing.T, c *ClusterYaml) {
				c.Peers = append(c.Peers, "bad-peer")
			},
			wantErr: "cluster.peers[3]",
		},
		{
			name: "peers missing self",
			modify: func(_ *testing.T, c *ClusterYaml) {
				c.Peers = []string{
					"10.0.0.2:7000",
					"10.0.0.3:7000",
					"10.0.0.4:7000",
				}
			},
			wantErr: "must include this node's advertise-address",
		},
		{
			name:    "missing tls.ca",
			modify:  func(_ *testing.T, c *ClusterYaml) { c.TLS.CA = "" },
			wantErr: "cluster.tls.ca is required",
		},
		{
			name:    "missing tls.cert",
			modify:  func(_ *testing.T, c *ClusterYaml) { c.TLS.Cert = "" },
			wantErr: "cluster.tls.cert is required",
		},
		{
			name:    "missing tls.key",
			modify:  func(_ *testing.T, c *ClusterYaml) { c.TLS.Key = "" },
			wantErr: "cluster.tls.key is required",
		},
		{
			name:    "ca file does not exist",
			modify:  func(_ *testing.T, c *ClusterYaml) { c.TLS.CA = "/nonexistent/ca.pem" },
			wantErr: "cluster.tls.ca file",
		},
		{
			name:    "cert file does not exist",
			modify:  func(_ *testing.T, c *ClusterYaml) { c.TLS.Cert = "/nonexistent/cert.pem" },
			wantErr: "cluster.tls.cert file",
		},
		{
			name:    "key file does not exist",
			modify:  func(_ *testing.T, c *ClusterYaml) { c.TLS.Key = "/nonexistent/key.pem" },
			wantErr: "cluster.tls.key file",
		},
		{
			name: "ca bundle has no valid certs",
			modify: func(t *testing.T, c *ClusterYaml) {
				t.Helper()

				bad := filepath.Join(t.TempDir(), "bad-ca.pem")
				if err := os.WriteFile(bad, []byte("not a cert"), 0o644); err != nil {
					t.Fatal(err)
				}

				c.TLS.CA = bad
			},
			wantErr: "no valid certificates found",
		},
		{
			name: "cert/key mismatch",
			modify: func(t *testing.T, c *ClusterYaml) {
				t.Helper()
				// Generate a second key that doesn't match the cert.
				dir := t.TempDir()
				otherPKI := genTestPKI(t, dir, 1)
				c.TLS.Key = otherPKI.KeyPath
			},
			wantErr: "cert/key pair invalid",
		},
		{
			name: "leaf CN does not match node-id",
			modify: func(t *testing.T, c *ClusterYaml) {
				t.Helper()
				// Generate PKI for node 2 but keep node-id as 1.
				dir := t.TempDir()
				wrongPKI := genTestPKI(t, dir, 2)
				c.TLS.CA = wrongPKI.CAPath
				c.TLS.Cert = wrongPKI.CertPath
				c.TLS.Key = wrongPKI.KeyPath
			},
			wantErr: "leaf CN is",
		},
		{
			name:    "invalid join-timeout",
			modify:  func(_ *testing.T, c *ClusterYaml) { c.JoinTimeout = "notaduration" },
			wantErr: "cluster.join-timeout",
		},
		{
			name:    "invalid propose-timeout",
			modify:  func(_ *testing.T, c *ClusterYaml) { c.ProposeTimeout = "xyz" },
			wantErr: "cluster.propose-timeout",
		},
		{
			name:    "invalid snapshot-interval",
			modify:  func(_ *testing.T, c *ClusterYaml) { c.SnapshotInterval = "bad" },
			wantErr: "cluster.snapshot-interval",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c := validClusterYaml(t)
			tc.modify(t, &c)

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
