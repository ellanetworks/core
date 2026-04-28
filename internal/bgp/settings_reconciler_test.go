// Copyright 2026 Ella Networks

package bgp

import (
	"context"
	"errors"
	"net/netip"
	"sync"
	"testing"
)

type fakeSettingsStore struct {
	mu         sync.Mutex
	settings   BGPSettings
	peers      []BGPPeer
	natEnabled bool

	settingsErr error
	peersErr    error
	natErr      error
}

func (f *fakeSettingsStore) GetSettings(_ context.Context) (BGPSettings, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.settingsErr != nil {
		return BGPSettings{}, f.settingsErr
	}

	return f.settings, nil
}

func (f *fakeSettingsStore) ListPeers(_ context.Context) ([]BGPPeer, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.peersErr != nil {
		return nil, f.peersErr
	}

	out := make([]BGPPeer, len(f.peers))
	copy(out, f.peers)

	return out, nil
}

func (f *fakeSettingsStore) IsNATEnabled(_ context.Context) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.natErr != nil {
		return false, f.natErr
	}

	return f.natEnabled, nil
}

type fakeService struct {
	mu             sync.Mutex
	running        bool
	startCalls     int
	stopCalls      int
	reconfigCalls  int
	advertisingSet []bool
	filterUpdates  int
	startErr       error
}

func (f *fakeService) IsRunning() bool {
	f.mu.Lock()
	defer f.mu.Unlock()

	return f.running
}

func (f *fakeService) Start(_ context.Context, _ BGPSettings, _ []BGPPeer, _ bool) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.startErr != nil {
		return f.startErr
	}

	f.startCalls++
	f.running = true

	return nil
}

func (f *fakeService) Stop() error {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.stopCalls++
	f.running = false

	return nil
}

func (f *fakeService) Reconfigure(_ context.Context, _ BGPSettings, _ []BGPPeer) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.reconfigCalls++

	return nil
}

func (f *fakeService) SetAdvertising(advertising bool) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.advertisingSet = append(f.advertisingSet, advertising)
}

func (f *fakeService) UpdateFilter(_ *RouteFilter) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.filterUpdates++
}

func TestSettingsReconcile_StartsWhenEnabled(t *testing.T) {
	store := &fakeSettingsStore{settings: BGPSettings{Enabled: true, LocalAS: 65000}}
	svc := &fakeService{running: false}

	r := NewSettingsReconciler(svc, store, nil, nil)

	if err := r.Reconcile(context.Background()); err != nil {
		t.Fatalf("reconcile: %v", err)
	}

	if svc.startCalls != 1 {
		t.Fatalf("expected 1 Start, got %d", svc.startCalls)
	}
}

func TestSettingsReconcile_StopsWhenDisabledAndRunning(t *testing.T) {
	store := &fakeSettingsStore{settings: BGPSettings{Enabled: false}}
	svc := &fakeService{running: true}

	r := NewSettingsReconciler(svc, store, nil, nil)
	r.MarkApplied(BGPSettings{Enabled: true, LocalAS: 65000}, nil, true)

	if err := r.Reconcile(context.Background()); err != nil {
		t.Fatalf("reconcile: %v", err)
	}

	if svc.stopCalls != 1 {
		t.Fatalf("expected 1 Stop, got %d", svc.stopCalls)
	}
}

func TestSettingsReconcile_ReconfiguresOnSettingsChange(t *testing.T) {
	store := &fakeSettingsStore{settings: BGPSettings{Enabled: true, LocalAS: 65000, RouterID: "1.1.1.1"}}
	svc := &fakeService{running: true}

	r := NewSettingsReconciler(svc, store, nil, nil)
	r.MarkApplied(BGPSettings{Enabled: true, LocalAS: 65000, RouterID: "1.1.1.1"}, nil, true)

	if err := r.Reconcile(context.Background()); err != nil {
		t.Fatalf("first reconcile: %v", err)
	}

	if svc.reconfigCalls != 0 {
		t.Fatalf("Reconfigure should not fire when settings unchanged, got %d calls", svc.reconfigCalls)
	}

	store.mu.Lock()
	store.settings.RouterID = "2.2.2.2"
	store.mu.Unlock()

	if err := r.Reconcile(context.Background()); err != nil {
		t.Fatalf("second reconcile: %v", err)
	}

	if svc.reconfigCalls != 1 {
		t.Fatalf("expected Reconfigure on settings change, got %d", svc.reconfigCalls)
	}
}

func TestSettingsReconcile_ReconfiguresOnPeerChange(t *testing.T) {
	peers := []BGPPeer{{ID: 1, Address: "10.0.0.1", RemoteAS: 100}}
	store := &fakeSettingsStore{settings: BGPSettings{Enabled: true, LocalAS: 65000}, peers: peers}
	svc := &fakeService{running: true}

	r := NewSettingsReconciler(svc, store, nil, nil)
	r.MarkApplied(BGPSettings{Enabled: true, LocalAS: 65000}, peers, true)

	if err := r.Reconcile(context.Background()); err != nil {
		t.Fatalf("first reconcile: %v", err)
	}

	if svc.reconfigCalls != 0 {
		t.Fatalf("Reconfigure should not fire when peers unchanged, got %d", svc.reconfigCalls)
	}

	store.mu.Lock()
	store.peers = append(store.peers, BGPPeer{ID: 2, Address: "10.0.0.2", RemoteAS: 200})
	store.mu.Unlock()

	if err := r.Reconcile(context.Background()); err != nil {
		t.Fatalf("second reconcile: %v", err)
	}

	if svc.reconfigCalls != 1 {
		t.Fatalf("expected Reconfigure on peer change, got %d", svc.reconfigCalls)
	}
}

func TestSettingsReconcile_AdvertisingFollowsNAT(t *testing.T) {
	store := &fakeSettingsStore{settings: BGPSettings{Enabled: true, LocalAS: 65000}, natEnabled: false}
	svc := &fakeService{running: true}

	r := NewSettingsReconciler(svc, store, nil, nil)
	r.MarkApplied(BGPSettings{Enabled: true, LocalAS: 65000}, nil, true)

	if err := r.Reconcile(context.Background()); err != nil {
		t.Fatalf("first reconcile: %v", err)
	}

	if got := len(svc.advertisingSet); got != 0 {
		t.Fatalf("SetAdvertising should not fire when unchanged, got %d", got)
	}

	store.mu.Lock()
	store.natEnabled = true
	store.mu.Unlock()

	if err := r.Reconcile(context.Background()); err != nil {
		t.Fatalf("second reconcile: %v", err)
	}

	if !equalBoolSlices(svc.advertisingSet, []bool{false}) {
		t.Fatalf("expected SetAdvertising(false) when NAT enabled, got %v", svc.advertisingSet)
	}
}

func TestSettingsReconcile_FilterRebuiltOnDiff(t *testing.T) {
	store := &fakeSettingsStore{settings: BGPSettings{Enabled: true, LocalAS: 65000}}
	svc := &fakeService{running: true}

	currentFilter := &RouteFilter{
		RejectPrefixes: []netip.Prefix{netip.MustParsePrefix("10.0.0.0/8")},
	}
	called := 0
	builder := func(_ context.Context) (*RouteFilter, error) {
		called++

		return currentFilter, nil
	}

	r := NewSettingsReconciler(svc, store, builder, nil)
	r.MarkApplied(BGPSettings{Enabled: true, LocalAS: 65000}, nil, true)

	if err := r.Reconcile(context.Background()); err != nil {
		t.Fatalf("first reconcile: %v", err)
	}

	if svc.filterUpdates != 1 {
		t.Fatalf("expected first filter apply, got %d", svc.filterUpdates)
	}

	if err := r.Reconcile(context.Background()); err != nil {
		t.Fatalf("second reconcile: %v", err)
	}

	if svc.filterUpdates != 1 {
		t.Fatalf("filter should not re-apply when unchanged, got %d", svc.filterUpdates)
	}

	currentFilter = &RouteFilter{
		RejectPrefixes: []netip.Prefix{netip.MustParsePrefix("172.16.0.0/12")},
	}

	if err := r.Reconcile(context.Background()); err != nil {
		t.Fatalf("third reconcile: %v", err)
	}

	if svc.filterUpdates != 2 {
		t.Fatalf("expected filter re-apply on change, got %d", svc.filterUpdates)
	}
}

func TestSettingsReconcile_StorePropagatesError(t *testing.T) {
	wantErr := errors.New("db down")
	store := &fakeSettingsStore{settingsErr: wantErr}
	svc := &fakeService{}

	r := NewSettingsReconciler(svc, store, nil, nil)

	if err := r.Reconcile(context.Background()); err == nil {
		t.Fatal("expected store error to surface")
	}
}

func equalBoolSlices(a, b []bool) bool {
	if len(a) != len(b) {
		return false
	}

	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}

	return true
}
