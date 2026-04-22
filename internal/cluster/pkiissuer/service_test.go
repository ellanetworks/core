// Copyright 2026 Ella Networks

package pkiissuer_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/ellanetworks/core/internal/cluster/pkiissuer"
	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/pki"
)

// fakeStore is an in-memory stub of pkiissuer.Store. Good enough for
// issuer unit tests; the real DB is exercised by cluster_pki_test.go in
// the db package.
type fakeStore struct {
	mu sync.Mutex

	leader        bool
	operator      *db.Operator
	state         *db.ClusterPKIState
	roots         map[string]*db.ClusterPKIRoot
	intermediates map[string]*db.ClusterPKIIntermediate
	issued        map[int64]*db.ClusterIssuedCert
	revoked       map[int64]*db.ClusterRevokedCert
	tokens        map[string]*db.ClusterJoinToken
	serialCounter int64
}

func newFakeStore(clusterID string) *fakeStore {
	return &fakeStore{
		leader:        true,
		operator:      &db.Operator{ClusterID: clusterID},
		roots:         make(map[string]*db.ClusterPKIRoot),
		intermediates: make(map[string]*db.ClusterPKIIntermediate),
		issued:        make(map[int64]*db.ClusterIssuedCert),
		revoked:       make(map[int64]*db.ClusterRevokedCert),
		tokens:        make(map[string]*db.ClusterJoinToken),
	}
}

func (f *fakeStore) GetOperator(ctx context.Context) (*db.Operator, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.operator == nil {
		return nil, db.ErrNotFound
	}

	cp := *f.operator

	return &cp, nil
}

func (f *fakeStore) GetPKIState(ctx context.Context) (*db.ClusterPKIState, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.state == nil {
		return nil, db.ErrNotFound
	}

	cp := *f.state

	return &cp, nil
}

func (f *fakeStore) BootstrapPKI(ctx context.Context, p *db.PKIBootstrap) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	// Matches the atomic semantics of the real BootstrapPKI: either
	// all three rows persist or none. For the fake we just write them
	// sequentially under the mutex; tests never interleave a retry.
	f.state = &db.ClusterPKIState{HMACKey: p.HMACKey}

	rootCopy := *p.Root
	f.roots[p.Root.Fingerprint] = &rootCopy

	intCopy := *p.Intermediate
	f.intermediates[p.Intermediate.Fingerprint] = &intCopy

	return nil
}

func (f *fakeStore) AllocatePKISerial(ctx context.Context) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.serialCounter++

	return f.serialCounter, nil
}

func (f *fakeStore) ListPKIRoots(ctx context.Context) ([]db.ClusterPKIRoot, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	out := make([]db.ClusterPKIRoot, 0, len(f.roots))
	for _, r := range f.roots {
		out = append(out, *r)
	}

	return out, nil
}

func (f *fakeStore) ListPKIIntermediates(ctx context.Context) ([]db.ClusterPKIIntermediate, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	out := make([]db.ClusterPKIIntermediate, 0, len(f.intermediates))
	for _, r := range f.intermediates {
		out = append(out, *r)
	}

	return out, nil
}

func (f *fakeStore) RecordIssuedCert(ctx context.Context, r *db.ClusterIssuedCert) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	cp := *r
	f.issued[r.Serial] = &cp

	return nil
}

func (f *fakeStore) ListRevokedCerts(ctx context.Context) ([]db.ClusterRevokedCert, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	out := make([]db.ClusterRevokedCert, 0, len(f.revoked))
	for _, r := range f.revoked {
		out = append(out, *r)
	}

	return out, nil
}

func (f *fakeStore) MintJoinTokenRecord(ctx context.Context, r *db.ClusterJoinToken) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	cp := *r
	f.tokens[r.ID] = &cp

	return nil
}

func (f *fakeStore) GetJoinToken(ctx context.Context, id string) (*db.ClusterJoinToken, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if t, ok := f.tokens[id]; ok {
		cp := *t

		return &cp, nil
	}

	return nil, db.ErrNotFound
}

func (f *fakeStore) ConsumeJoinToken(ctx context.Context, id string, nodeID int) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	t, ok := f.tokens[id]
	if !ok {
		return errors.New("not found")
	}

	if t.ConsumedAt != 0 {
		return nil
	}

	t.ConsumedAt = time.Now().Unix()
	t.ConsumedBy = nodeID

	return nil
}

func (f *fakeStore) IsLeader() bool { return f.leader }

func TestBootstrap_PopulatesKeysAndState(t *testing.T) {
	store := newFakeStore("cluster-xyz")

	svc := pkiissuer.New(store)
	ctx := context.Background()

	if err := svc.Bootstrap(ctx); err != nil {
		t.Fatalf("Bootstrap: %v", err)
	}

	if len(store.roots) != 1 {
		t.Fatalf("roots = %d, want 1", len(store.roots))
	}

	if len(store.intermediates) != 1 {
		t.Fatalf("intermediates = %d, want 1", len(store.intermediates))
	}

	if store.state == nil || len(store.state.HMACKey) == 0 {
		t.Fatal("pki state not initialised")
	}

	if !svc.Ready() {
		t.Fatal("issuer should be ready post-bootstrap")
	}

	// Second Bootstrap is a no-op.
	if err := svc.Bootstrap(ctx); err != nil {
		t.Fatalf("second Bootstrap: %v", err)
	}

	if len(store.roots) != 1 {
		t.Fatal("second Bootstrap should not add another root")
	}
}

func TestIssue_HappyPath(t *testing.T) {
	store := newFakeStore("c")

	svc := pkiissuer.New(store)
	ctx := context.Background()

	if err := svc.Bootstrap(ctx); err != nil {
		t.Fatal(err)
	}

	_, csrPEM, err := pki.GenerateKeyAndCSR(5, "c")
	if err != nil {
		t.Fatal(err)
	}

	csr, _ := pki.ParseCSRPEM(csrPEM)

	leafPEM, err := svc.Issue(ctx, csr, 5, time.Hour)
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}

	leaf, err := pki.ParseCertPEM(leafPEM)
	if err != nil {
		t.Fatal(err)
	}

	if leaf.Subject.CommonName != "ella-node-5" {
		t.Fatalf("CN = %q", leaf.Subject.CommonName)
	}

	// Chain-verify through the issuer's bundle.
	bundle, err := svc.CurrentBundle(ctx)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := bundle.Verify(leaf, time.Now()); err != nil {
		t.Fatalf("issued leaf must verify through issuer bundle: %v", err)
	}

	if len(store.issued) != 1 {
		t.Fatalf("issued = %d, want 1", len(store.issued))
	}

	if leaf.SerialNumber.Uint64() != 1 {
		t.Fatalf("serial = %d, want 1", leaf.SerialNumber.Uint64())
	}
}

func TestIssue_NotReady(t *testing.T) {
	store := newFakeStore("c")

	svc := pkiissuer.New(store)
	ctx := context.Background()

	_, csrPEM, _ := pki.GenerateKeyAndCSR(1, "c")
	csr, _ := pki.ParseCSRPEM(csrPEM)

	if _, err := svc.Issue(ctx, csr, 1, time.Hour); err == nil {
		t.Fatal("Issue before Bootstrap must fail")
	}
}

func TestIssue_NotLeader(t *testing.T) {
	store := newFakeStore("c")

	svc := pkiissuer.New(store)
	ctx := context.Background()

	_ = svc.Bootstrap(ctx)

	store.leader = false

	_, csrPEM, _ := pki.GenerateKeyAndCSR(1, "c")
	csr, _ := pki.ParseCSRPEM(csrPEM)

	if _, err := svc.Issue(ctx, csr, 1, time.Hour); err == nil {
		t.Fatal("Issue on follower must fail")
	}
}

func TestJoinToken_MintVerifyConsume(t *testing.T) {
	store := newFakeStore("c")
	svc := pkiissuer.New(store)
	ctx := context.Background()

	_ = svc.Bootstrap(ctx)

	tok, err := svc.MintJoinToken(ctx, 3, 10*time.Minute)
	if err != nil {
		t.Fatalf("MintJoinToken: %v", err)
	}

	claims, err := svc.VerifyAndConsumeJoinToken(ctx, tok)
	if err != nil {
		t.Fatalf("VerifyAndConsumeJoinToken: %v", err)
	}

	if claims.NodeID != 3 {
		t.Fatalf("claims.NodeID = %d, want 3", claims.NodeID)
	}

	// Second consume must fail (single-use).
	if _, err := svc.VerifyAndConsumeJoinToken(ctx, tok); err == nil {
		t.Fatal("second consume must fail")
	}
}

func TestJoinToken_TTLBounds(t *testing.T) {
	store := newFakeStore("c")
	svc := pkiissuer.New(store)
	ctx := context.Background()

	_ = svc.Bootstrap(ctx)

	if _, err := svc.MintJoinToken(ctx, 1, time.Second); err == nil {
		t.Fatal("too-short TTL must be rejected")
	}

	if _, err := svc.MintJoinToken(ctx, 1, 100*time.Hour); err == nil {
		t.Fatal("too-long TTL must be rejected")
	}
}

func TestLoadKeys_ReloadsAfterLeadership(t *testing.T) {
	store := newFakeStore("c")

	svc := pkiissuer.New(store)
	ctx := context.Background()

	_ = svc.Bootstrap(ctx)

	svc.UnloadKeys()

	if svc.Ready() {
		t.Fatal("should not be ready after UnloadKeys")
	}

	if err := svc.LoadKeys(ctx); err != nil {
		t.Fatalf("LoadKeys: %v", err)
	}

	if !svc.Ready() {
		t.Fatal("should be ready after LoadKeys")
	}

	// Issue still works.
	_, csrPEM, _ := pki.GenerateKeyAndCSR(9, "c")
	csr, _ := pki.ParseCSRPEM(csrPEM)

	if _, err := svc.Issue(ctx, csr, 9, time.Hour); err != nil {
		t.Fatalf("Issue post-reload: %v", err)
	}
}

func TestLoadKeys_MissingFilesIsNoop(t *testing.T) {
	// Fresh dataDir with no keys on disk; LoadKeys should not error.
	svc := pkiissuer.New(newFakeStore("c"))

	if err := svc.LoadKeys(context.Background()); err != nil {
		t.Fatalf("LoadKeys on empty dir: %v", err)
	}

	if svc.Ready() {
		t.Fatal("should not be ready without keys")
	}
}

func TestActiveRootFingerprint(t *testing.T) {
	store := newFakeStore("c")
	svc := pkiissuer.New(store)
	ctx := context.Background()

	_ = svc.Bootstrap(ctx)

	fp, err := svc.ActiveRootFingerprint(ctx)
	if err != nil {
		t.Fatal(err)
	}

	if fp == "" {
		t.Fatal("fingerprint empty")
	}
}
