// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

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

// fakeStore is a minimal in-memory db.Store stand-in.
type fakeStore struct {
	mu sync.Mutex

	leader  bool
	op      *db.Operator
	hmacKey []byte
	pins    map[int]*db.ClusterNodeCert
	tokens  map[string]*db.ClusterJoinToken
}

const testClusterID = "c"

func newFakeStore(clusterID string) *fakeStore {
	return &fakeStore{
		leader: true,
		op:     &db.Operator{ClusterID: clusterID},
		pins:   make(map[int]*db.ClusterNodeCert),
		tokens: make(map[string]*db.ClusterJoinToken),
	}
}

func (f *fakeStore) IsLeader() bool { return f.leader }

func (f *fakeStore) GetOperator(ctx context.Context) (*db.Operator, error) {
	return f.op, nil
}

func (f *fakeStore) GetClusterJoinHMACKey(ctx context.Context) ([]byte, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.hmacKey == nil {
		return nil, db.ErrNotFound
	}

	return f.hmacKey, nil
}

func (f *fakeStore) InitClusterJoinHMACKey(ctx context.Context, key []byte) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.hmacKey == nil {
		f.hmacKey = append([]byte(nil), key...)
	}

	return nil
}

func (f *fakeStore) UpsertClusterNodeCert(ctx context.Context, r *db.ClusterNodeCert) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	cp := *r
	f.pins[r.NodeID] = &cp

	return nil
}

func (f *fakeStore) ListClusterNodeCerts(ctx context.Context) ([]db.ClusterNodeCert, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	out := make([]db.ClusterNodeCert, 0, len(f.pins))
	for _, p := range f.pins {
		out = append(out, *p)
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

	t, ok := f.tokens[id]
	if !ok {
		return nil, db.ErrNotFound
	}

	cp := *t

	return &cp, nil
}

func (f *fakeStore) ConsumeJoinToken(ctx context.Context, id string, nodeID int) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	t, ok := f.tokens[id]
	if !ok {
		return db.ErrNotFound
	}

	if t.ConsumedAt != 0 {
		return db.ErrJoinTokenAlreadyConsumed
	}

	t.ConsumedAt = time.Now().Unix()
	t.ConsumedBy = nodeID

	return nil
}

func (f *fakeStore) RedeemJoinToken(ctx context.Context, tokenID string, nodeID int, fingerprint, certPEM string) ([]db.ClusterNodeCert, error) {
	f.mu.Lock()

	t, ok := f.tokens[tokenID]
	if !ok {
		f.mu.Unlock()
		return nil, db.ErrNotFound
	}

	if t.NodeID != nodeID {
		f.mu.Unlock()
		return nil, db.ErrJoinTokenNodeMismatch
	}

	if t.ExpiresAt <= time.Now().Unix() {
		f.mu.Unlock()
		return nil, db.ErrJoinTokenExpired
	}

	if t.ConsumedAt != 0 {
		existing, have := f.pins[nodeID]
		if t.ConsumedBy != nodeID || !have || existing.Fingerprint != fingerprint {
			f.mu.Unlock()
			return nil, db.ErrJoinTokenAlreadyConsumed
		}

		f.mu.Unlock()

		return f.ListClusterNodeCerts(ctx)
	}

	t.ConsumedAt = time.Now().Unix()
	t.ConsumedBy = nodeID
	f.pins[nodeID] = &db.ClusterNodeCert{
		NodeID:      nodeID,
		Fingerprint: fingerprint,
		CertPEM:     certPEM,
		AddedAt:     time.Now().Unix(),
	}

	f.mu.Unlock()

	return f.ListClusterNodeCerts(ctx)
}

// preregisterLeader inserts the leader's pin so MintJoinToken can
// embed it in a token's claims.
func preregisterLeader(t *testing.T, store *fakeStore, nodeID int) string {
	t.Helper()

	cert, _, err := pki.GenerateNodeCert(nodeID, testClusterID, time.Hour)
	if err != nil {
		t.Fatalf("generate leader cert: %v", err)
	}

	fp := pki.Fingerprint(cert)

	store.pins[nodeID] = &db.ClusterNodeCert{
		NodeID:      nodeID,
		Fingerprint: fp,
		CertPEM:     string(pki.EncodeCertPEM(cert)),
		AddedAt:     time.Now().Unix(),
	}

	return fp
}

// leaderLeaf mirrors the runtime accessor: the pin of the cert this
// node presents. The fake leader is always node 1.
func leaderLeaf(store *fakeStore) pkiissuer.LocalLeafFunc {
	return func() string {
		store.mu.Lock()
		defer store.mu.Unlock()

		if p := store.pins[1]; p != nil {
			return p.Fingerprint
		}

		return ""
	}
}

func TestService_Bootstrap_SeedsHMACKey(t *testing.T) {
	store := newFakeStore("c")

	svc := pkiissuer.New(store, leaderLeaf(store))

	if err := svc.Bootstrap(context.Background()); err != nil {
		t.Fatalf("bootstrap: %v", err)
	}

	if !svc.Ready(context.Background()) {
		t.Fatal("expected Ready after Bootstrap")
	}

	first, _ := store.GetClusterJoinHMACKey(context.Background())

	// Idempotent.
	if err := svc.Bootstrap(context.Background()); err != nil {
		t.Fatalf("second bootstrap: %v", err)
	}

	second, _ := store.GetClusterJoinHMACKey(context.Background())
	if string(first) != string(second) {
		t.Fatal("Bootstrap is not idempotent: HMAC key changed")
	}
}

func TestService_RegisterCert_HappyPath(t *testing.T) {
	store := newFakeStore("c")

	svc := pkiissuer.New(store, leaderLeaf(store))

	cert, _, err := pki.GenerateNodeCert(7, "c", time.Hour)
	if err != nil {
		t.Fatal(err)
	}

	fp, pins, err := svc.RegisterCert(context.Background(), 7, pki.EncodeCertPEM(cert))
	if err != nil {
		t.Fatalf("register: %v", err)
	}

	if fp != pki.Fingerprint(cert) {
		t.Fatal("returned fingerprint mismatch")
	}

	if got := store.pins[7]; got == nil || got.Fingerprint != fp {
		t.Fatal("pin not stored")
	}

	if len(pins) != 1 || pins[0].NodeID != 7 || pins[0].Fingerprint != fp {
		t.Fatalf("post-commit pin snapshot: got %+v", pins)
	}
}

func TestService_RegisterCert_RejectsCrossCluster(t *testing.T) {
	store := newFakeStore("c-a")

	svc := pkiissuer.New(store, leaderLeaf(store))

	cert, _, err := pki.GenerateNodeCert(7, "c-b", time.Hour)
	if err != nil {
		t.Fatal(err)
	}

	if _, _, err := svc.RegisterCert(context.Background(), 7, pki.EncodeCertPEM(cert)); err == nil {
		t.Fatal("expected register to reject cross-cluster cert")
	}
}

func TestService_RegisterCert_RejectsNodeIDMismatch(t *testing.T) {
	store := newFakeStore("c")

	svc := pkiissuer.New(store, leaderLeaf(store))

	cert, _, err := pki.GenerateNodeCert(7, "c", time.Hour)
	if err != nil {
		t.Fatal(err)
	}

	if _, _, err := svc.RegisterCert(context.Background(), 8, pki.EncodeCertPEM(cert)); err == nil {
		t.Fatal("expected register to reject when URI nodeID != path nodeID")
	}
}

func TestService_MintAndVerifyJoinToken_RoundTrip(t *testing.T) {
	store := newFakeStore("c")

	leaderFP := preregisterLeader(t, store, 1)

	svc := pkiissuer.New(store, leaderLeaf(store))

	if err := svc.Bootstrap(context.Background()); err != nil {
		t.Fatal(err)
	}

	token, err := svc.MintJoinToken(context.Background(), 5, time.Minute*30)
	if err != nil {
		t.Fatalf("mint: %v", err)
	}

	claims, err := pki.ExtractClaimsUnverified(token)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}

	if claims.LeaderCertPin != leaderFP {
		t.Fatalf("leader pin mismatch: got %s want %s", claims.LeaderCertPin, leaderFP)
	}

	if claims.NodeID != 5 {
		t.Fatalf("nodeID mismatch")
	}

	joinerPEM := nodeCertPEM(t, 5)

	fp, pins, err := svc.RedeemJoinToken(context.Background(), token, 5, joinerPEM)
	if err != nil {
		t.Fatalf("redeem: %v", err)
	}

	if fp == "" || len(pins) != 2 {
		t.Fatalf("redeem returned fp=%q pins=%d, want non-empty fp and 2 pins", fp, len(pins))
	}

	otherPEM := nodeCertPEM(t, 6)
	if _, _, err := svc.RedeemJoinToken(context.Background(), token, 6, otherPEM); err == nil {
		t.Fatal("replay for a different node should be rejected")
	}
}

func TestService_MintJoinToken_RejectsInvalidTTL(t *testing.T) {
	store := newFakeStore("c")
	preregisterLeader(t, store, 1)

	svc := pkiissuer.New(store, leaderLeaf(store))
	_ = svc.Bootstrap(context.Background())

	if _, err := svc.MintJoinToken(context.Background(), 5, time.Second); err == nil {
		t.Fatal("expected ttl < min to be rejected")
	}

	if _, err := svc.MintJoinToken(context.Background(), 5, 48*time.Hour); err == nil {
		t.Fatal("expected ttl > max to be rejected")
	}
}

func TestService_NotLeader_RejectsMutations(t *testing.T) {
	store := newFakeStore("c")
	store.leader = false

	svc := pkiissuer.New(store, leaderLeaf(store))

	if err := svc.Bootstrap(context.Background()); err == nil {
		t.Fatal("Bootstrap should fail on non-leader")
	}

	if _, err := svc.MintJoinToken(context.Background(), 5, time.Hour); err == nil {
		t.Fatal("MintJoinToken should fail on non-leader")
	}
}

func TestService_RegisterCert_WorksOnNonLeader(t *testing.T) {
	store := newFakeStore("c")
	store.leader = false

	svc := pkiissuer.New(store, leaderLeaf(store))

	if _, _, err := svc.RegisterCert(context.Background(), 5, nodeCertPEM(t, 5)); err != nil {
		t.Fatalf("RegisterCert on a follower: %v", err)
	}
}

func TestService_Redeem_ReplayWithDifferentCertRejected(t *testing.T) {
	store := newFakeStore("c")
	preregisterLeader(t, store, 1)

	svc := pkiissuer.New(store, leaderLeaf(store))
	_ = svc.Bootstrap(context.Background())

	tok, _ := svc.MintJoinToken(context.Background(), 5, time.Minute*10)

	if _, _, err := svc.RedeemJoinToken(context.Background(), tok, 5, nodeCertPEM(t, 5)); err != nil {
		t.Fatal(err)
	}

	_, _, err := svc.RedeemJoinToken(context.Background(), tok, 5, nodeCertPEM(t, 5))
	if !errors.Is(err, db.ErrJoinTokenAlreadyConsumed) {
		t.Fatalf("second redeem with a fresh cert: got %v, want ErrJoinTokenAlreadyConsumed", err)
	}
}

func TestService_Redeem_SameNodeSameCertIsIdempotent(t *testing.T) {
	store := newFakeStore("c")
	preregisterLeader(t, store, 1)

	svc := pkiissuer.New(store, leaderLeaf(store))
	_ = svc.Bootstrap(context.Background())

	tok, _ := svc.MintJoinToken(context.Background(), 5, time.Minute*10)
	joinerPEM := nodeCertPEM(t, 5)

	fp1, pins1, err := svc.RedeemJoinToken(context.Background(), tok, 5, joinerPEM)
	if err != nil {
		t.Fatal(err)
	}

	fp2, pins2, err := svc.RedeemJoinToken(context.Background(), tok, 5, joinerPEM)
	if err != nil {
		t.Fatalf("retry after a burnt token must succeed: %v", err)
	}

	if fp1 != fp2 || len(pins1) != len(pins2) {
		t.Fatalf("replay returned a different result: %s/%d vs %s/%d", fp1, len(pins1), fp2, len(pins2))
	}
}

func TestService_MintJoinToken_EmbedsEveryVoterPin(t *testing.T) {
	store := newFakeStore("c")
	leaderFP := preregisterLeader(t, store, 1)
	peerFP := preregisterLeader(t, store, 2)

	svc := pkiissuer.New(store, leaderLeaf(store))
	_ = svc.Bootstrap(context.Background())

	tok, err := svc.MintJoinToken(context.Background(), 5, time.Minute*10)
	if err != nil {
		t.Fatal(err)
	}

	claims, err := pki.ExtractClaimsUnverified(tok)
	if err != nil {
		t.Fatal(err)
	}

	set := claims.PinSet()
	if len(set) != 2 {
		t.Fatalf("PinSet has %d entries, want 2", len(set))
	}

	for _, want := range []string{leaderFP, peerFP} {
		found := false

		for _, got := range set {
			if got == want {
				found = true
			}
		}

		if !found {
			t.Fatalf("PinSet is missing %s", want)
		}
	}
}

func nodeCertPEM(t *testing.T, nodeID int) []byte {
	t.Helper()

	cert, _, err := pki.GenerateNodeCert(nodeID, testClusterID, time.Hour)
	if err != nil {
		t.Fatalf("generate node cert: %v", err)
	}

	return pki.EncodeCertPEM(cert)
}

// After a restore the table still holds the pre-restore cluster's row
// for this nodeID. Minting from that row hands the joiner a pin no
// node will ever present, and the joiner retries it forever.
func TestService_MintJoinToken_RefusesStalePinFromRestoredTable(t *testing.T) {
	store := newFakeStore(testClusterID)

	stalePins := map[int]*db.ClusterNodeCert{}

	for _, nodeID := range []int{1, 2, 3} {
		preregisterLeader(t, store, nodeID)
		stalePins[nodeID] = store.pins[nodeID]
	}

	// This node self-signs a fresh cert on restore; the row for its
	// own nodeID has not been replaced yet.
	freshCert, _, err := pki.GenerateNodeCert(1, testClusterID, time.Hour)
	if err != nil {
		t.Fatal(err)
	}

	freshFP := pki.Fingerprint(freshCert)

	svc := pkiissuer.New(store, func() string { return freshFP })
	if err := svc.Bootstrap(context.Background()); err != nil {
		t.Fatal(err)
	}

	_, err = svc.MintJoinToken(context.Background(), 5, 10*time.Minute)
	if !errors.Is(err, pkiissuer.ErrNotReady) {
		t.Fatalf("mint before the fresh cert commits: got %v, want ErrNotReady", err)
	}

	if _, _, err := svc.RegisterCert(context.Background(), 1, pki.EncodeCertPEM(freshCert)); err != nil {
		t.Fatalf("register fresh leader cert: %v", err)
	}

	tok, err := svc.MintJoinToken(context.Background(), 5, 10*time.Minute)
	if err != nil {
		t.Fatalf("mint after the fresh cert commits: %v", err)
	}

	claims, err := pki.ExtractClaimsUnverified(tok)
	if err != nil {
		t.Fatal(err)
	}

	if claims.LeaderCertPin != freshFP {
		t.Fatalf("token pins %q, want the cert this node presents (%q)", claims.LeaderCertPin, freshFP)
	}

	if claims.LeaderCertPin == stalePins[1].Fingerprint {
		t.Fatal("token pins the pre-restore cert")
	}
}

func TestService_MintJoinToken_RefusesWithoutLocalLeaf(t *testing.T) {
	store := newFakeStore(testClusterID)
	preregisterLeader(t, store, 1)

	svc := pkiissuer.New(store, nil)
	if err := svc.Bootstrap(context.Background()); err != nil {
		t.Fatal(err)
	}

	if _, err := svc.MintJoinToken(context.Background(), 5, 10*time.Minute); !errors.Is(err, pkiissuer.ErrNotReady) {
		t.Fatalf("mint with no local leaf: got %v, want ErrNotReady", err)
	}
}

func TestService_MintJoinToken_NotReadyBeforeBootstrap(t *testing.T) {
	store := newFakeStore(testClusterID)
	preregisterLeader(t, store, 1)

	cert, _, err := pki.GenerateNodeCert(1, testClusterID, time.Hour)
	if err != nil {
		t.Fatal(err)
	}

	fp := pki.Fingerprint(cert)
	svc := pkiissuer.New(store, func() string { return fp })

	if _, err := svc.MintJoinToken(context.Background(), 5, 10*time.Minute); !errors.Is(err, pkiissuer.ErrNotReady) {
		t.Fatalf("mint before the join hmac key commits: got %v, want ErrNotReady", err)
	}

	if err := svc.Bootstrap(context.Background()); err != nil {
		t.Fatal(err)
	}

	if _, _, err := svc.RegisterCert(context.Background(), 1, pki.EncodeCertPEM(cert)); err != nil {
		t.Fatalf("register leader cert: %v", err)
	}

	if _, err := svc.MintJoinToken(context.Background(), 5, 10*time.Minute); err != nil {
		t.Fatalf("mint after bootstrap: %v", err)
	}
}
