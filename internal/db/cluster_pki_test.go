// Copyright 2026 Ella Networks

package db_test

import (
	"bytes"
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/ellanetworks/core/internal/db"
)

func setupPKIDB(t *testing.T) *db.Database {
	t.Helper()

	ctx := context.Background()

	tmpDir := t.TempDir()

	dbInstance, err := db.NewDatabaseWithoutRaft(ctx, filepath.Join(tmpDir, "ella.db"))
	if err != nil {
		t.Fatalf("NewDatabaseWithoutRaft: %v", err)
	}

	t.Cleanup(func() { _ = dbInstance.Close() })

	return dbInstance
}

func TestPKIState_InitAndAllocate(t *testing.T) {
	database := setupPKIDB(t)
	ctx := context.Background()

	// Before init, GetPKIState returns not-found.
	if _, err := database.GetPKIState(ctx); err == nil {
		t.Fatal("expected ErrNotFound before init")
	}

	if err := database.InitializePKIState(ctx, []byte("0123456789abcdef0123456789abcdef")); err != nil {
		t.Fatalf("InitializePKIState: %v", err)
	}

	state, err := database.GetPKIState(ctx)
	if err != nil {
		t.Fatalf("GetPKIState: %v", err)
	}

	if string(state.HMACKey) != "0123456789abcdef0123456789abcdef" {
		t.Fatalf("hmac key mismatch")
	}

	// Init twice is a no-op (ON CONFLICT DO NOTHING).
	if err := database.InitializePKIState(ctx, []byte("xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx")); err != nil {
		t.Fatalf("InitializePKIState second call: %v", err)
	}

	state2, _ := database.GetPKIState(ctx)
	if string(state2.HMACKey) != "0123456789abcdef0123456789abcdef" {
		t.Fatal("InitializePKIState clobbered existing hmac key")
	}

	// Allocate three serials; they must be monotonic.
	var last int64

	for i := 0; i < 3; i++ {
		n, err := database.AllocatePKISerial(ctx)
		if err != nil {
			t.Fatalf("AllocatePKISerial: %v", err)
		}

		if n <= last {
			t.Fatalf("serial %d did not increase past %d", n, last)
		}

		last = n
	}
}

func TestPKIRoots_CRUD(t *testing.T) {
	database := setupPKIDB(t)
	ctx := context.Background()

	fakeKey := []byte("-----BEGIN PRIVATE KEY-----\nA\n-----END PRIVATE KEY-----\n")

	if err := database.InsertPKIRoot(ctx, &db.ClusterPKIRoot{
		Fingerprint: "sha256:aaaa",
		CertPEM:     "-----BEGIN CERTIFICATE-----\nA\n-----END CERTIFICATE-----",
		KeyPEM:      fakeKey,
		AddedAt:     time.Now().Unix(),
		Status:      db.PKIStatusActive,
	}); err != nil {
		t.Fatalf("InsertPKIRoot: %v", err)
	}

	roots, err := database.ListPKIRoots(ctx)
	if err != nil {
		t.Fatalf("ListPKIRoots: %v", err)
	}

	if len(roots) != 1 || roots[0].Fingerprint != "sha256:aaaa" {
		t.Fatalf("got roots %+v", roots)
	}

	if !bytes.Equal(roots[0].KeyPEM, fakeKey) {
		t.Fatalf("KeyPEM did not round-trip; got %q", roots[0].KeyPEM)
	}

	if err := database.SetPKIRootStatus(ctx, "sha256:aaaa", db.PKIStatusVerifyOnly); err != nil {
		t.Fatalf("SetPKIRootStatus: %v", err)
	}

	roots, _ = database.ListPKIRoots(ctx)
	if roots[0].Status != db.PKIStatusVerifyOnly {
		t.Fatalf("status = %q", roots[0].Status)
	}

	// Per CHECK constraint: non-active rows must have NULL keyPEM.
	if roots[0].KeyPEM != nil {
		t.Fatalf("KeyPEM should be cleared on status transition; got %q", roots[0].KeyPEM)
	}

	if err := database.DeletePKIRoot(ctx, "sha256:aaaa"); err != nil {
		t.Fatalf("DeletePKIRoot: %v", err)
	}

	roots, _ = database.ListPKIRoots(ctx)
	if len(roots) != 0 {
		t.Fatalf("expected empty, got %d", len(roots))
	}
}

// TestPKIRoots_CHECKConstraintRejectsActiveWithoutKey asserts the DDL
// CHECK constraint: an active row must carry a non-NULL keyPEM.
func TestPKIRoots_CHECKConstraintRejectsActiveWithoutKey(t *testing.T) {
	database := setupPKIDB(t)
	ctx := context.Background()

	err := database.InsertPKIRoot(ctx, &db.ClusterPKIRoot{
		Fingerprint: "sha256:bbbb",
		CertPEM:     "x",
		KeyPEM:      nil,
		AddedAt:     time.Now().Unix(),
		Status:      db.PKIStatusActive,
	})
	if err == nil {
		t.Fatal("InsertPKIRoot(active, keyPEM=nil) must be rejected by CHECK constraint")
	}
}

// TestPKIRoots_CHECKConstraintRejectsRetiredWithKey asserts the other
// half of the constraint: non-active rows must have NULL keyPEM.
func TestPKIRoots_CHECKConstraintRejectsRetiredWithKey(t *testing.T) {
	database := setupPKIDB(t)
	ctx := context.Background()

	err := database.InsertPKIRoot(ctx, &db.ClusterPKIRoot{
		Fingerprint: "sha256:cccc",
		CertPEM:     "x",
		KeyPEM:      []byte("should-not-be-here"),
		AddedAt:     time.Now().Unix(),
		Status:      db.PKIStatusRetired,
	})
	if err == nil {
		t.Fatal("InsertPKIRoot(retired, keyPEM=set) must be rejected by CHECK constraint")
	}
}

func TestIssuedAndRevokedCerts(t *testing.T) {
	database := setupPKIDB(t)
	ctx := context.Background()

	now := time.Now()

	// Issue three certs: two active, one already expired.
	for i, c := range []struct {
		serial   int64
		nodeID   int
		notAfter time.Time
	}{
		{1, 3, now.Add(time.Hour)},
		{2, 3, now.Add(time.Hour)},
		{3, 4, now.Add(-time.Hour)},
	} {
		if err := database.RecordIssuedCert(ctx, &db.ClusterIssuedCert{
			Serial:                  c.serial,
			NodeID:                  c.nodeID,
			NotAfter:                c.notAfter.Unix(),
			IntermediateFingerprint: "sha256:int",
			IssuedAt:                now.Unix(),
		}); err != nil {
			t.Fatalf("RecordIssuedCert[%d]: %v", i, err)
		}
	}

	active3, err := database.ListActiveIssuedCertsByNode(ctx, 3)
	if err != nil {
		t.Fatal(err)
	}

	if len(active3) != 2 {
		t.Fatalf("node 3 active = %d, want 2", len(active3))
	}

	active4, _ := database.ListActiveIssuedCertsByNode(ctx, 4)
	if len(active4) != 0 {
		t.Fatalf("node 4 has %d active, want 0 (one expired, one n/a)", len(active4))
	}

	// Revoke node 3's certs.
	for _, c := range active3 {
		if err := database.InsertRevokedCert(ctx, &db.ClusterRevokedCert{
			Serial:     c.Serial,
			NodeID:     c.NodeID,
			RevokedAt:  now.Unix(),
			Reason:     "test",
			PurgeAfter: now.Add(2 * time.Hour).Unix(),
		}); err != nil {
			t.Fatalf("InsertRevokedCert: %v", err)
		}
	}

	revoked, err := database.ListRevokedCerts(ctx)
	if err != nil {
		t.Fatal(err)
	}

	if len(revoked) != 2 {
		t.Fatalf("got %d revocations, want 2", len(revoked))
	}

	// Tidy: delete issued certs that are already expired.
	if err := database.DeleteExpiredIssuedCerts(ctx, now.Add(-30*time.Minute)); err != nil {
		t.Fatal(err)
	}

	// Tidy: delete revoked rows whose purge window has passed.
	if err := database.DeletePurgedRevocations(ctx, now.Add(3*time.Hour)); err != nil {
		t.Fatal(err)
	}

	revoked, _ = database.ListRevokedCerts(ctx)
	if len(revoked) != 0 {
		t.Fatalf("revocations should have been purged, got %d", len(revoked))
	}
}

func TestInsertRevokedCert_Idempotent(t *testing.T) {
	database := setupPKIDB(t)
	ctx := context.Background()

	now := time.Now().Unix()
	row := &db.ClusterRevokedCert{
		Serial:     100,
		NodeID:     5,
		RevokedAt:  now,
		Reason:     "first",
		PurgeAfter: now + 3600,
	}

	if err := database.InsertRevokedCert(ctx, row); err != nil {
		t.Fatal(err)
	}

	// Second insert with same serial must not error (ON CONFLICT DO NOTHING).
	if err := database.InsertRevokedCert(ctx, row); err != nil {
		t.Fatalf("second insert should be idempotent: %v", err)
	}
}

func TestJoinToken_SingleUse(t *testing.T) {
	database := setupPKIDB(t)
	ctx := context.Background()

	now := time.Now()
	tok := &db.ClusterJoinToken{
		ID:         "token-abc",
		NodeID:     2,
		ClaimsJSON: `{"id":"token-abc","node_id":2,"iat":1,"exp":2}`,
		ExpiresAt:  now.Add(time.Hour).Unix(),
	}

	if err := database.MintJoinTokenRecord(ctx, tok); err != nil {
		t.Fatal(err)
	}

	got, err := database.GetJoinToken(ctx, "token-abc")
	if err != nil {
		t.Fatalf("GetJoinToken: %v", err)
	}

	if got.ConsumedAt != 0 {
		t.Fatalf("new token should be unconsumed")
	}

	// Consume.
	if err := database.ConsumeJoinToken(ctx, "token-abc", 2); err != nil {
		t.Fatalf("Consume: %v", err)
	}

	got, _ = database.GetJoinToken(ctx, "token-abc")
	if got.ConsumedAt == 0 || got.ConsumedBy != 2 {
		t.Fatalf("token should be consumed by 2, got %+v", got)
	}

	// Second consume surfaces ErrJoinTokenAlreadyConsumed so the caller
	// can distinguish a lost race from a successful single-use.
	// consumedAt/consumedBy must not change.
	firstConsumedAt := got.ConsumedAt

	time.Sleep(1 * time.Second)

	err = database.ConsumeJoinToken(ctx, "token-abc", 9)
	if !errors.Is(err, db.ErrJoinTokenAlreadyConsumed) {
		t.Fatalf("second Consume: want ErrJoinTokenAlreadyConsumed, got %v", err)
	}

	got, _ = database.GetJoinToken(ctx, "token-abc")
	if got.ConsumedAt != firstConsumedAt || got.ConsumedBy != 2 {
		t.Fatalf("second consume must not replace state, got %+v", got)
	}
}

func TestJoinTokenCleanup(t *testing.T) {
	database := setupPKIDB(t)
	ctx := context.Background()

	now := time.Now()

	// Token A: expired, unconsumed.
	if err := database.MintJoinTokenRecord(ctx, &db.ClusterJoinToken{
		ID:         "tok-expired",
		NodeID:     2,
		ClaimsJSON: "{}",
		ExpiresAt:  now.Add(-time.Hour).Unix(),
	}); err != nil {
		t.Fatal(err)
	}

	// Token B: expires far in the future — should survive.
	if err := database.MintJoinTokenRecord(ctx, &db.ClusterJoinToken{
		ID:         "tok-live",
		NodeID:     3,
		ClaimsJSON: "{}",
		ExpiresAt:  now.Add(24 * time.Hour).Unix(),
	}); err != nil {
		t.Fatal(err)
	}

	// Token C: consumed > 1h before cleanup clock — should be purged.
	if err := database.MintJoinTokenRecord(ctx, &db.ClusterJoinToken{
		ID:         "tok-consumed",
		NodeID:     4,
		ClaimsJSON: "{}",
		ExpiresAt:  now.Add(24 * time.Hour).Unix(),
	}); err != nil {
		t.Fatal(err)
	}

	if err := database.ConsumeJoinToken(ctx, "tok-consumed", 4); err != nil {
		t.Fatal(err)
	}

	// Run cleanup at now+90min: tok-expired is expired relative to this
	// clock; tok-consumed was consumed >1h ago; tok-live is still valid.
	if err := database.DeleteStaleJoinTokens(ctx, now.Add(90*time.Minute)); err != nil {
		t.Fatal(err)
	}

	// tok-expired: expiresAt < now+2h → purged.
	// tok-consumed: consumedAt < now+1h → purged.
	// tok-live: neither condition met → survives.
	if _, err := database.GetJoinToken(ctx, "tok-expired"); err == nil {
		t.Fatal("tok-expired should be purged")
	}

	if _, err := database.GetJoinToken(ctx, "tok-consumed"); err == nil {
		t.Fatal("tok-consumed should be purged")
	}

	if _, err := database.GetJoinToken(ctx, "tok-live"); err != nil {
		t.Fatalf("tok-live should survive: %v", err)
	}
}
