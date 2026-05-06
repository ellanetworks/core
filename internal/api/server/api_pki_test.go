// Copyright 2026 Ella Networks

package server_test

import (
	"context"
	"encoding/json"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ellanetworks/core/internal/api/server"
	"github.com/ellanetworks/core/internal/cluster/pkiissuer"
	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/pki"
)

// TestPKIAdminEndpoints_Gated verifies that admin PKI endpoints reject
// non-admin callers and return 503 before the issuer service is
// installed.
func TestPKIAdminEndpoints_NoIssuer503(t *testing.T) {
	env, err := setupServer(filepath.Join(t.TempDir(), "ella.db"))
	if err != nil {
		t.Fatal(err)
	}

	defer func() { _ = env.DB.Close() }()

	client := env.Server.Client()

	admin, err := initializeAndRefresh(env.Server.URL, client)
	if err != nil {
		t.Fatal(err)
	}

	// No issuer installed yet — expect 503.
	body := strings.NewReader(`{"nodeID": 2}`)

	req, _ := http.NewRequestWithContext(context.Background(), http.MethodPost,
		env.Server.URL+"/api/v1/cluster/pki/join-tokens", body)
	req.Header.Set("Authorization", "Bearer "+admin)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}

	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("got %d, want 503", resp.StatusCode)
	}
}

// TestPKIAdminEndpoints_MintToken verifies the happy path once an
// issuer is installed.
func TestPKIAdminEndpoints_MintToken(t *testing.T) {
	env, err := setupServer(filepath.Join(t.TempDir(), "ella.db"))
	if err != nil {
		t.Fatal(err)
	}

	defer func() { _ = env.DB.Close() }()

	// Seed a cluster ID so the issuer's Bootstrap succeeds.
	if err := env.DB.UpdateOperatorClusterID(context.Background(), "test-cluster"); err != nil {
		t.Fatal(err)
	}

	issuer := pkiissuer.New(env.DB)
	if err := issuer.Bootstrap(context.Background()); err != nil {
		t.Fatalf("Bootstrap: %v", err)
	}

	// Register the leader's pin (NodeID()==0 in standalone, so seed
	// node 1 as the "leader" the test API uses).
	leaderCert, _, err := pki.GenerateNodeCert(1, "test-cluster", time.Hour)
	if err != nil {
		t.Fatal(err)
	}

	if err := env.DB.UpsertClusterNodeCert(context.Background(), &db.ClusterNodeCert{
		NodeID:      1,
		Fingerprint: pki.Fingerprint(leaderCert),
		CertPEM:     string(pki.EncodeCertPEM(leaderCert)),
		AddedAt:     time.Now().Unix(),
	}); err != nil {
		t.Fatal(err)
	}

	server.SetPKIIssuer(issuer)
	t.Cleanup(func() { server.SetPKIIssuer(nil) })

	client := env.Server.Client()

	admin, err := initializeAndRefresh(env.Server.URL, env.Server.Client())
	if err != nil {
		t.Fatal(err)
	}

	body := strings.NewReader(`{"nodeID": 5}`)

	req, _ := http.NewRequestWithContext(context.Background(), http.MethodPost,
		env.Server.URL+"/api/v1/cluster/pki/join-tokens", body)
	req.Header.Set("Authorization", "Bearer "+admin)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}

	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("got %d, want 201", resp.StatusCode)
	}

	var env2 struct {
		Result server.MintJoinTokenResponse `json:"result"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&env2); err != nil {
		t.Fatal(err)
	}

	if env2.Result.Token == "" {
		t.Fatal("empty token")
	}

	if env2.Result.ExpiresAtUnixSecs == 0 {
		t.Fatal("empty expiresAt")
	}
}
