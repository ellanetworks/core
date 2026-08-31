// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package server

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/ellanetworks/core/internal/cluster/pkiissuer"
	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/pki"
)

const mintTestClusterID = "test-cluster"

func newMintTestIssuer(t *testing.T) (*pkiissuer.Service, func()) {
	t.Helper()

	ctx := context.Background()

	testdb, err := db.NewDatabaseWithoutRaft(ctx, filepath.Join(t.TempDir(), "ella.db"))
	if err != nil {
		t.Fatalf("new database: %v", err)
	}

	if err := testdb.UpdateOperatorClusterID(ctx, mintTestClusterID); err != nil {
		_ = testdb.Close()

		t.Fatalf("set cluster id: %v", err)
	}

	cert, _, err := pki.GenerateNodeCert(1, mintTestClusterID, time.Hour)
	if err != nil {
		_ = testdb.Close()

		t.Fatalf("generate node cert: %v", err)
	}

	fp := pki.Fingerprint(cert)
	svc := pkiissuer.New(testdb, func() string { return fp })

	leaderInit := func() {
		if err := svc.Bootstrap(ctx); err != nil {
			t.Errorf("bootstrap: %v", err)
			return
		}

		if _, _, err := svc.RegisterCert(ctx, 1, pki.EncodeCertPEM(cert)); err != nil {
			t.Errorf("register leader cert: %v", err)
		}
	}

	t.Cleanup(func() { _ = testdb.Close() })

	return svc, leaderInit
}

func TestMintWhenReady_WaitsForLeaderInit(t *testing.T) {
	svc, leaderInit := newMintTestIssuer(t)

	if _, err := svc.MintJoinToken(context.Background(), 5, 10*time.Minute); !errors.Is(err, pkiissuer.ErrNotReady) {
		t.Fatalf("mint before leader init: got %v, want ErrNotReady", err)
	}

	go func() {
		time.Sleep(300 * time.Millisecond)
		leaderInit()
	}()

	token, err := mintWhenReady(context.Background(), svc, 5, 10*time.Minute)
	if err != nil {
		t.Fatalf("mintWhenReady: %v", err)
	}

	if token == "" {
		t.Fatal("empty token")
	}
}

func TestMintWhenReady_GivesUpAsNotReady(t *testing.T) {
	svc, _ := newMintTestIssuer(t)

	previous := mintReadyWait
	mintReadyWait = 250 * time.Millisecond

	t.Cleanup(func() { mintReadyWait = previous })

	_, err := mintWhenReady(context.Background(), svc, 5, 10*time.Minute)
	if !errors.Is(err, pkiissuer.ErrNotReady) {
		t.Fatalf("mintWhenReady with no leader init: got %v, want ErrNotReady", err)
	}
}
