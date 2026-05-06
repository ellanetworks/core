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

// fakeStore is a minimal in-memory db.Store stand-in.
type fakeStore struct {
	mu sync.Mutex

	leader  bool
	op      *db.Operator
	hmacKey []byte
	pins    map[int]*db.ClusterNodeCert
	tokens  map[string]*db.ClusterJoinToken
}

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

// preregisterLeader inserts the leader's pin so MintJoinToken can
// embed it in a token's claims.
func preregisterLeader(t *testing.T, store *fakeStore, nodeID int, clusterID string) string {
	t.Helper()

	cert, _, err := pki.GenerateNodeCert(nodeID, clusterID, time.Hour)
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

func TestService_Bootstrap_SeedsHMACKey(t *testing.T) {
	store := newFakeStore("c")

	svc := pkiissuer.New(store)

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

	svc := pkiissuer.New(store)

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

	svc := pkiissuer.New(store)

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

	svc := pkiissuer.New(store)

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

	leaderFP := preregisterLeader(t, store, 1, "c")

	svc := pkiissuer.New(store)

	if err := svc.Bootstrap(context.Background()); err != nil {
		t.Fatal(err)
	}

	token, err := svc.MintJoinToken(context.Background(), 5, time.Minute*30, 1)
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

	verified, err := svc.VerifyAndConsumeJoinToken(context.Background(), token)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}

	if verified.TokenID != claims.TokenID {
		t.Fatal("verify returned different token id")
	}

	// Replay: second consume must fail.
	if _, err := svc.VerifyAndConsumeJoinToken(context.Background(), token); err == nil {
		t.Fatal("replay should be rejected")
	}
}

func TestService_MintJoinToken_RejectsInvalidTTL(t *testing.T) {
	store := newFakeStore("c")
	preregisterLeader(t, store, 1, "c")

	svc := pkiissuer.New(store)
	_ = svc.Bootstrap(context.Background())

	if _, err := svc.MintJoinToken(context.Background(), 5, time.Second, 1); err == nil {
		t.Fatal("expected ttl < min to be rejected")
	}

	if _, err := svc.MintJoinToken(context.Background(), 5, 48*time.Hour, 1); err == nil {
		t.Fatal("expected ttl > max to be rejected")
	}
}

func TestService_NotLeader_RejectsMutations(t *testing.T) {
	store := newFakeStore("c")
	store.leader = false

	svc := pkiissuer.New(store)

	if err := svc.Bootstrap(context.Background()); err == nil {
		t.Fatal("Bootstrap should fail on non-leader")
	}

	if _, err := svc.MintJoinToken(context.Background(), 5, time.Hour, 1); err == nil {
		t.Fatal("MintJoinToken should fail on non-leader")
	}

	if _, _, err := svc.RegisterCert(context.Background(), 1, []byte("not pem")); err == nil {
		t.Fatal("RegisterCert should fail on non-leader")
	}
}

// Smoke test that ErrJoinTokenAlreadyConsumed is preserved through
// the wrapping VerifyAndConsumeJoinToken does.
func TestService_DoubleConsume_PreservesErrPath(t *testing.T) {
	store := newFakeStore("c")
	preregisterLeader(t, store, 1, "c")

	svc := pkiissuer.New(store)
	_ = svc.Bootstrap(context.Background())

	tok, _ := svc.MintJoinToken(context.Background(), 5, time.Minute*10, 1)

	_, err := svc.VerifyAndConsumeJoinToken(context.Background(), tok)
	if err != nil {
		t.Fatal(err)
	}

	_, err = svc.VerifyAndConsumeJoinToken(context.Background(), tok)
	if err == nil || (!errors.Is(err, db.ErrJoinTokenAlreadyConsumed) && err.Error() != "token already consumed") {
		t.Fatalf("unexpected second-consume error: %v", err)
	}
}
