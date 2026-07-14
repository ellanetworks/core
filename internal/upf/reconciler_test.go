// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package upf

import (
	"context"
	"errors"
	"net/netip"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/models"
)

type fakeStore struct {
	mu              sync.Mutex
	natEnabled      bool
	flowAccounting  bool
	n3External      string
	n3GetErr        error
	policies        []db.Policy
	rulesByPolicyID map[string][]*db.NetworkRule
}

func (f *fakeStore) IsNATEnabled(_ context.Context) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	return f.natEnabled, nil
}

func (f *fakeStore) IsFlowAccountingEnabled(_ context.Context) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	return f.flowAccounting, nil
}

func (f *fakeStore) GetN3Settings(_ context.Context) (*db.N3Settings, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.n3GetErr != nil {
		return nil, f.n3GetErr
	}

	return &db.N3Settings{ExternalAddress: f.n3External}, nil
}

func (f *fakeStore) ListPoliciesPage(_ context.Context, _ int, _ int) ([]db.Policy, int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	out := make([]db.Policy, len(f.policies))
	copy(out, f.policies)

	return out, len(out), nil
}

func (f *fakeStore) ListRulesForPolicy(_ context.Context, policyID string) ([]*db.NetworkRule, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	rules := f.rulesByPolicyID[policyID]
	out := make([]*db.NetworkRule, len(rules))
	copy(out, rules)

	return out, nil
}

type filterCall struct {
	policyID  string
	direction models.Direction
	rules     []models.FilterRule
}

type fakeUpdater struct {
	mu          sync.Mutex
	natCalls    []bool
	flowCalls   []bool
	n3Calls     []netip.Addr
	filterCalls []filterCall
	natErr      error
	flowErr     error
	// updateFiltersErr fails every UpdateFilters call; updateFiltersFunc, when
	// set, takes precedence and decides per (policyID, direction, rules).
	updateFiltersErr  error
	updateFiltersFunc func(policyID string, direction models.Direction, rules []models.FilterRule) error
}

func (f *fakeUpdater) ReloadNAT(enabled bool) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.natErr != nil {
		return f.natErr
	}

	f.natCalls = append(f.natCalls, enabled)

	return nil
}

func (f *fakeUpdater) ReloadFlowAccounting(enabled bool) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.flowErr != nil {
		return f.flowErr
	}

	f.flowCalls = append(f.flowCalls, enabled)

	return nil
}

func (f *fakeUpdater) UpdateAdvertisedN3Address(addr netip.Addr) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.n3Calls = append(f.n3Calls, addr)
}

func (f *fakeUpdater) UpdateFilters(_ context.Context, policyID string, direction models.Direction, rules []models.FilterRule) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	switch {
	case f.updateFiltersFunc != nil:
		if err := f.updateFiltersFunc(policyID, direction, rules); err != nil {
			return err
		}
	case f.updateFiltersErr != nil:
		return f.updateFiltersErr
	}

	cp := make([]models.FilterRule, len(rules))
	copy(cp, rules)
	f.filterCalls = append(f.filterCalls, filterCall{policyID: policyID, direction: direction, rules: cp})

	return nil
}

func newReconciler(updater Updater, store SettingsStore, fallback netip.Addr) *SettingsReconciler {
	return NewSettingsReconciler(updater, store, nil, fallback)
}

func TestReconcile_NATAppliesOnFirstTickAndSkipsWhenUnchanged(t *testing.T) {
	store := &fakeStore{natEnabled: true}
	updater := &fakeUpdater{}

	r := newReconciler(updater, store, netip.MustParseAddr("1.2.3.4"))

	if err := r.Reconcile(context.Background()); err != nil {
		t.Fatalf("first reconcile: %v", err)
	}

	if got := len(updater.natCalls); got != 1 || updater.natCalls[0] != true {
		t.Fatalf("expected one ReloadNAT(true), got %v", updater.natCalls)
	}

	if err := r.Reconcile(context.Background()); err != nil {
		t.Fatalf("second reconcile: %v", err)
	}

	if got := len(updater.natCalls); got != 1 {
		t.Fatalf("expected ReloadNAT to NOT fire on unchanged second tick, got %d total calls", got)
	}
}

func TestReconcile_NATFiresOnChange(t *testing.T) {
	store := &fakeStore{natEnabled: true}
	updater := &fakeUpdater{}

	r := newReconciler(updater, store, netip.MustParseAddr("1.2.3.4"))

	if err := r.Reconcile(context.Background()); err != nil {
		t.Fatalf("reconcile 1: %v", err)
	}

	store.mu.Lock()
	store.natEnabled = false
	store.mu.Unlock()

	if err := r.Reconcile(context.Background()); err != nil {
		t.Fatalf("reconcile 2: %v", err)
	}

	if !reflect.DeepEqual(updater.natCalls, []bool{true, false}) {
		t.Fatalf("expected NAT calls [true, false], got %v", updater.natCalls)
	}
}

func TestReconcile_FlowAccountingDiff(t *testing.T) {
	store := &fakeStore{flowAccounting: true}
	updater := &fakeUpdater{}

	r := newReconciler(updater, store, netip.MustParseAddr("1.2.3.4"))

	if err := r.Reconcile(context.Background()); err != nil {
		t.Fatalf("first reconcile: %v", err)
	}

	if err := r.Reconcile(context.Background()); err != nil {
		t.Fatalf("second reconcile: %v", err)
	}

	if !reflect.DeepEqual(updater.flowCalls, []bool{true}) {
		t.Fatalf("expected one ReloadFlowAccounting(true), got %v", updater.flowCalls)
	}
}

func TestReconcile_N3UsesFallbackWhenExternalEmpty(t *testing.T) {
	store := &fakeStore{n3External: ""}
	updater := &fakeUpdater{}
	fallback := netip.MustParseAddr("10.0.0.5")

	r := newReconciler(updater, store, fallback)

	if err := r.Reconcile(context.Background()); err != nil {
		t.Fatalf("reconcile: %v", err)
	}

	if got := len(updater.n3Calls); got != 1 || updater.n3Calls[0] != fallback {
		t.Fatalf("expected N3 call with fallback %s, got %v", fallback, updater.n3Calls)
	}
}

func TestReconcile_N3PrefersExternalWhenSet(t *testing.T) {
	external := "172.16.1.1"
	store := &fakeStore{n3External: external}
	updater := &fakeUpdater{}

	r := newReconciler(updater, store, netip.MustParseAddr("10.0.0.5"))

	if err := r.Reconcile(context.Background()); err != nil {
		t.Fatalf("reconcile: %v", err)
	}

	if got := len(updater.n3Calls); got != 1 || updater.n3Calls[0].String() != external {
		t.Fatalf("expected N3 call with external %s, got %v", external, updater.n3Calls)
	}
}

func TestReconcile_N3RejectsInvalidExternal(t *testing.T) {
	store := &fakeStore{n3External: "not-an-ip"}
	updater := &fakeUpdater{}

	r := newReconciler(updater, store, netip.MustParseAddr("10.0.0.5"))

	err := r.Reconcile(context.Background())
	if err == nil {
		t.Fatal("expected error on invalid external address")
	}
}

func TestReconcile_N3HandlesGetN3SettingsNotFound(t *testing.T) {
	store := &fakeStore{n3GetErr: db.ErrNotFound}
	updater := &fakeUpdater{}

	r := newReconciler(updater, store, netip.MustParseAddr("10.0.0.5"))

	if err := r.Reconcile(context.Background()); err != nil {
		t.Fatalf("ErrNotFound should be tolerated, got %v", err)
	}

	if len(updater.n3Calls) != 0 {
		t.Fatalf("no N3 call expected when settings missing, got %v", updater.n3Calls)
	}
}

func TestReconcile_FiltersAddRemoveModify(t *testing.T) {
	store := &fakeStore{
		policies: []db.Policy{{ID: "policy-1"}},
		rulesByPolicyID: map[string][]*db.NetworkRule{
			"policy-1": {
				{Direction: directionUplinkString, Protocol: 6, PortLow: 80, PortHigh: 80, Action: "allow"},
			},
		},
	}
	updater := &fakeUpdater{}

	r := newReconciler(updater, store, netip.MustParseAddr("10.0.0.5"))

	if err := r.Reconcile(context.Background()); err != nil {
		t.Fatalf("reconcile 1: %v", err)
	}

	uplinkCalls, downlinkCalls := splitFilterCalls(updater.filterCalls)

	if len(uplinkCalls) != 1 || len(uplinkCalls[0].rules) != 1 {
		t.Fatalf("expected one uplink call with one rule, got %v", uplinkCalls)
	}

	if len(downlinkCalls) != 1 || len(downlinkCalls[0].rules) != 0 {
		t.Fatalf("expected one downlink call with zero rules, got %v", downlinkCalls)
	}

	updater.mu.Lock()
	updater.filterCalls = nil
	updater.mu.Unlock()

	if err := r.Reconcile(context.Background()); err != nil {
		t.Fatalf("reconcile 2: %v", err)
	}

	if len(updater.filterCalls) != 0 {
		t.Fatalf("expected no filter calls on unchanged second reconcile, got %v", updater.filterCalls)
	}

	store.mu.Lock()
	store.rulesByPolicyID["policy-1"] = []*db.NetworkRule{
		{Direction: directionUplinkString, Protocol: 17, PortLow: 53, PortHigh: 53, Action: "allow"},
	}
	store.mu.Unlock()

	updater.mu.Lock()
	updater.filterCalls = nil
	updater.mu.Unlock()

	if err := r.Reconcile(context.Background()); err != nil {
		t.Fatalf("reconcile 3: %v", err)
	}

	uplinkCalls, _ = splitFilterCalls(updater.filterCalls)
	if len(uplinkCalls) != 1 || uplinkCalls[0].rules[0].Protocol != 17 {
		t.Fatalf("expected uplink reapplied with new rules, got %v", uplinkCalls)
	}

	store.mu.Lock()
	store.policies = nil
	store.rulesByPolicyID = nil
	store.mu.Unlock()

	updater.mu.Lock()
	updater.filterCalls = nil
	updater.mu.Unlock()

	if err := r.Reconcile(context.Background()); err != nil {
		t.Fatalf("reconcile 4: %v", err)
	}

	uplinkCalls, downlinkCalls = splitFilterCalls(updater.filterCalls)

	if len(uplinkCalls) != 1 || len(uplinkCalls[0].rules) != 0 {
		t.Fatalf("expected uplink cleared (empty rules) on policy delete, got %v", uplinkCalls)
	}

	if len(downlinkCalls) != 1 || len(downlinkCalls[0].rules) != 0 {
		t.Fatalf("expected downlink cleared (empty rules) on policy delete, got %v", downlinkCalls)
	}
}

func TestReconcile_FilterUpdateFailureRetried(t *testing.T) {
	store := &fakeStore{
		policies: []db.Policy{{ID: "policy-1"}},
		rulesByPolicyID: map[string][]*db.NetworkRule{
			"policy-1": {
				{Direction: directionUplinkString, Protocol: 6, PortLow: 80, PortHigh: 80, Action: "allow"},
			},
		},
	}
	updater := &fakeUpdater{updateFiltersErr: errors.New("map write failed")}

	r := newReconciler(updater, store, netip.MustParseAddr("10.0.0.5"))

	if err := r.Reconcile(context.Background()); err == nil {
		t.Fatal("expected reconcile to return an error when UpdateFilters fails")
	}

	updater.mu.Lock()
	updater.updateFiltersErr = nil
	updater.filterCalls = nil
	updater.mu.Unlock()

	if err := r.Reconcile(context.Background()); err != nil {
		t.Fatalf("reconcile after recovery: %v", err)
	}

	uplinkCalls, downlinkCalls := splitFilterCalls(updater.filterCalls)
	if len(uplinkCalls) != 1 || len(downlinkCalls) != 1 {
		t.Fatalf("expected both directions retried after failure, got uplink=%v downlink=%v", uplinkCalls, downlinkCalls)
	}

	updater.mu.Lock()
	updater.filterCalls = nil
	updater.mu.Unlock()

	if err := r.Reconcile(context.Background()); err != nil {
		t.Fatalf("reconcile once applied: %v", err)
	}

	if len(updater.filterCalls) != 0 {
		t.Fatalf("expected no filter calls once applied, got %v", updater.filterCalls)
	}
}

func TestReconcile_FilterUplinkFailureDoesNotSkipDownlink(t *testing.T) {
	store := &fakeStore{
		policies: []db.Policy{{ID: "policy-1"}},
		rulesByPolicyID: map[string][]*db.NetworkRule{
			"policy-1": {
				{Direction: directionUplinkString, Protocol: 6, PortLow: 80, PortHigh: 80, Action: "allow"},
				{Direction: directionDownlinkString, Protocol: 17, PortLow: 53, PortHigh: 53, Action: "allow"},
			},
		},
	}

	var attempted []models.Direction

	updater := &fakeUpdater{
		updateFiltersFunc: func(_ string, dir models.Direction, _ []models.FilterRule) error {
			attempted = append(attempted, dir)
			if dir == models.DirectionUplink {
				return errors.New("uplink map full")
			}

			return nil
		},
	}

	r := newReconciler(updater, store, netip.MustParseAddr("10.0.0.5"))

	if err := r.Reconcile(context.Background()); err == nil {
		t.Fatal("expected error when uplink update fails")
	}

	var sawUplink, sawDownlink bool

	for _, d := range attempted {
		switch d {
		case models.DirectionUplink:
			sawUplink = true
		case models.DirectionDownlink:
			sawDownlink = true
		}
	}

	if !sawUplink || !sawDownlink {
		t.Fatalf("expected both directions attempted despite uplink failure, got %v", attempted)
	}
}

func TestReconcile_FilterClearFailureRetried(t *testing.T) {
	store := &fakeStore{
		policies: []db.Policy{{ID: "policy-1"}},
		rulesByPolicyID: map[string][]*db.NetworkRule{
			"policy-1": {
				{Direction: directionUplinkString, Protocol: 6, PortLow: 80, PortHigh: 80, Action: "allow"},
			},
		},
	}
	updater := &fakeUpdater{}

	r := newReconciler(updater, store, netip.MustParseAddr("10.0.0.5"))

	if err := r.Reconcile(context.Background()); err != nil {
		t.Fatalf("initial apply: %v", err)
	}

	store.mu.Lock()
	store.policies = nil
	store.rulesByPolicyID = nil
	store.mu.Unlock()

	updater.mu.Lock()
	updater.updateFiltersFunc = func(_ string, _ models.Direction, _ []models.FilterRule) error {
		return errors.New("clear failed")
	}
	updater.filterCalls = nil
	updater.mu.Unlock()

	if err := r.Reconcile(context.Background()); err == nil {
		t.Fatal("expected error when clearing a deleted policy's filters fails")
	}

	updater.mu.Lock()
	updater.updateFiltersFunc = nil
	updater.filterCalls = nil
	updater.mu.Unlock()

	if err := r.Reconcile(context.Background()); err != nil {
		t.Fatalf("reconcile after clear recovery: %v", err)
	}

	uplinkCalls, downlinkCalls := splitFilterCalls(updater.filterCalls)
	if len(uplinkCalls) != 1 || len(uplinkCalls[0].rules) != 0 {
		t.Fatalf("expected uplink clear retried, got %v", uplinkCalls)
	}

	if len(downlinkCalls) != 1 || len(downlinkCalls[0].rules) != 0 {
		t.Fatalf("expected downlink clear retried, got %v", downlinkCalls)
	}
}

// TestReconcile_LoopWakesOnChangefeedEvent verifies the end-to-end
// path: starting the reconciler with a real changefeed wakeup, then
// publishing an event, drives Reconcile within milliseconds — not
// "next backstop tick."
func TestReconcile_LoopWakesOnChangefeedEvent(t *testing.T) {
	feed := db.NewChangefeed()

	store := &fakeStore{natEnabled: true}
	updater := &fakeUpdater{}

	r := NewSettingsReconciler(updater, store, feed, netip.MustParseAddr("1.2.3.4"))
	r.backstop = time.Hour // disable backstop so the test only passes via events

	r.Start()
	defer r.Stop()

	// Wait for the initial reconcile (synchronous in loop()).
	deadline := time.Now().Add(time.Second)

	for time.Now().Before(deadline) {
		updater.mu.Lock()
		count := len(updater.natCalls)
		updater.mu.Unlock()

		if count == 1 {
			break
		}

		time.Sleep(time.Millisecond)
	}

	updater.mu.Lock()
	initialNATCalls := len(updater.natCalls)
	updater.mu.Unlock()

	if initialNATCalls != 1 {
		t.Fatalf("expected 1 NAT call after initial reconcile, got %d", initialNATCalls)
	}

	// Flip desired state and publish an event; the loop must observe
	// the change without waiting for the (disabled) backstop tick.
	store.mu.Lock()
	store.natEnabled = false
	store.mu.Unlock()

	feed.Publish(db.TopicNATSettings, 0)

	deadline = time.Now().Add(time.Second)

	for time.Now().Before(deadline) {
		updater.mu.Lock()
		count := len(updater.natCalls)
		updater.mu.Unlock()

		if count == 2 {
			return
		}

		time.Sleep(time.Millisecond)
	}

	updater.mu.Lock()
	finalNATCalls := len(updater.natCalls)
	updater.mu.Unlock()

	t.Fatalf("expected reconcile to fire from changefeed event; got %d total NAT calls", finalNATCalls)
}

func TestReconcile_PropagatesNATError(t *testing.T) {
	store := &fakeStore{natEnabled: true}
	updater := &fakeUpdater{natErr: errors.New("xdp attach failed")}

	r := newReconciler(updater, store, netip.MustParseAddr("10.0.0.5"))

	err := r.Reconcile(context.Background())
	if err == nil {
		t.Fatal("expected error from NAT updater to propagate")
	}
}

func splitFilterCalls(calls []filterCall) (uplink, downlink []filterCall) {
	for _, c := range calls {
		switch c.direction {
		case models.DirectionUplink:
			uplink = append(uplink, c)
		case models.DirectionDownlink:
			downlink = append(downlink, c)
		}
	}

	return uplink, downlink
}
