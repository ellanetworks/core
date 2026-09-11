// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package upf

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
	"reflect"
	"sync"
	"time"

	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	"go.uber.org/zap"
)

const (
	// upfReconcileBackstop is the periodic invariant-checking sweep
	// when no change events have fired. Primary trigger is the
	// changefeed; this exists only to recover from missed signals.
	upfReconcileBackstop    = 5 * time.Minute
	directionUplinkString   = "uplink"
	directionDownlinkString = "downlink"
)

// SettingsStore is the narrow view the reconciler needs over the DB.
// *db.Database satisfies it; a fake satisfies it in tests.
type SettingsStore interface {
	IsNATEnabled(ctx context.Context) (bool, error)
	IsFlowAccountingEnabled(ctx context.Context) (bool, error)
	IsLocalSwitchEnabled(ctx context.Context) (bool, error)
	GetN3Settings(ctx context.Context) (*db.N3Settings, error)
	ListPoliciesPage(ctx context.Context, page int, perPage int) ([]db.Policy, int, error)
	ListRulesForPolicy(ctx context.Context, policyID string) ([]*db.NetworkRule, error)
}

// DatapathSettings is the set of load-time datapath toggles applied together.
type DatapathSettings struct {
	NAT            bool
	FlowAccounting bool
	LocalSwitch    bool
}

// Updater is the narrow view the reconciler needs over the UPF runtime.
// *UPF satisfies it.
type Updater interface {
	ApplyDatapathSettings(settings DatapathSettings) error
	UpdateAdvertisedN3Address(addr netip.Addr)
	UpdateFilters(ctx context.Context, policyID string, direction models.Direction, rules []models.FilterRule) error
}

// SettingsReconciler drives this node's UPF runtime from replicated DB
// settings: NAT toggle, flow accounting toggle, advertised N3 address,
// and per-policy SDF filters. Each tick reads the desired state from
// the DB and applies it to the local UPF only when it differs from the
// last-applied snapshot — the underlying Reload* and UpdateFilters
// calls re-attach XDP / re-write eBPF maps, so calling them
// unconditionally on every tick would disrupt the data plane.
type SettingsReconciler struct {
	updater      Updater
	store        SettingsStore
	changefeed   *db.Changefeed
	fallbackN3IP netip.Addr
	backstop     time.Duration

	mu     sync.Mutex
	cancel context.CancelFunc
	done   chan struct{}

	stateMu          sync.Mutex
	appliedSettings  *DatapathSettings
	appliedN3Address netip.Addr
	appliedFilters   map[string]filterSnapshot
}

type filterSnapshot struct {
	uplink   []models.FilterRule
	downlink []models.FilterRule
}

// NewSettingsReconciler wires a reconciler. fallbackN3IP is the local
// node's configured N3 address used when n3_settings.external_address
// is empty. changefeed may be nil in tests that drive Reconcile()
// directly; production callers always pass a non-nil broker.
func NewSettingsReconciler(updater Updater, store SettingsStore, changefeed *db.Changefeed, fallbackN3IP netip.Addr) *SettingsReconciler {
	return &SettingsReconciler{
		updater:        updater,
		store:          store,
		changefeed:     changefeed,
		fallbackN3IP:   fallbackN3IP,
		backstop:       upfReconcileBackstop,
		appliedFilters: make(map[string]filterSnapshot),
	}
}

// Start launches the reconciler goroutines. Subsequent calls without a
// paired Stop are no-ops.
func (r *SettingsReconciler) Start() {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.cancel != nil {
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	r.cancel = cancel
	r.done = make(chan struct{})

	done := r.done

	settingsDone := make(chan struct{})
	filtersDone := make(chan struct{})

	go r.loop(ctx, settingsDone, "upf settings reconcile failed", r.reconcileSettings,
		db.TopicNATSettings,
		db.TopicFlowAccountingSettings,
		db.TopicLocalSwitchSettings,
		db.TopicN3Settings,
	)

	go r.loop(ctx, filtersDone, "upf policy filter reconcile failed", r.reconcileFilters,
		db.TopicPolicies,
		db.TopicNetworkRules,
	)

	go func() {
		defer close(done)

		<-settingsDone
		<-filtersDone
	}()
}

// Stop signals the reconcilers to exit and blocks until both goroutines
// have drained.
func (r *SettingsReconciler) Stop() {
	r.mu.Lock()
	cancel := r.cancel
	done := r.done
	r.cancel = nil
	r.done = nil
	r.mu.Unlock()

	if cancel == nil {
		return
	}

	cancel()
	<-done
}

func (r *SettingsReconciler) loop(ctx context.Context, done chan struct{}, failMsg string, reconcile func(context.Context) error, topics ...db.Topic) {
	defer close(done)

	var (
		events  <-chan db.Event
		dropped <-chan struct{}
	)

	if r.changefeed != nil {
		sub := r.changefeed.Subscribe(topics...)
		defer sub.Close()

		events = sub.Events
		dropped = sub.Dropped
	}

	if err := reconcile(ctx); err != nil {
		logger.UpfLog.Warn(failMsg, zap.Error(err))
	}

	backstop := time.NewTicker(r.backstop)
	defer backstop.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-events:
		case <-dropped:
		case <-backstop.C:
		}

		if err := reconcile(ctx); err != nil {
			logger.UpfLog.Warn(failMsg, zap.Error(err))
		}
	}
}

// Reconcile performs one full reconcile pass. Exposed for tests and for
// callers that want to force convergence after a known change.
func (r *SettingsReconciler) Reconcile(ctx context.Context) error {
	if err := r.reconcileSettings(ctx); err != nil {
		return err
	}

	if err := r.reconcileFilters(ctx); err != nil {
		return fmt.Errorf("policy filters: %w", err)
	}

	return nil
}

func (r *SettingsReconciler) reconcileSettings(ctx context.Context) error {
	if err := r.reconcileDatapathSettings(ctx); err != nil {
		return err
	}

	if err := r.reconcileN3Address(ctx); err != nil {
		return fmt.Errorf("n3 address: %w", err)
	}

	return nil
}

func (r *SettingsReconciler) reconcileDatapathSettings(ctx context.Context) error {
	desired, err := r.desiredDatapathSettings(ctx)
	if err != nil {
		return err
	}

	r.stateMu.Lock()
	current := r.appliedSettings
	r.stateMu.Unlock()

	if current != nil && *current == desired {
		return nil
	}

	if err := r.updater.ApplyDatapathSettings(desired); err != nil {
		return fmt.Errorf("apply datapath settings: %w", err)
	}

	r.stateMu.Lock()
	applied := desired
	r.appliedSettings = &applied
	r.stateMu.Unlock()

	logger.UpfLog.Info("applied datapath settings",
		zap.Bool("nat", desired.NAT),
		zap.Bool("flow_accounting", desired.FlowAccounting),
		zap.Bool("local_switch", desired.LocalSwitch),
	)

	return nil
}

func (r *SettingsReconciler) desiredDatapathSettings(ctx context.Context) (DatapathSettings, error) {
	var desired DatapathSettings

	nat, err := r.store.IsNATEnabled(ctx)
	if err != nil {
		return desired, fmt.Errorf("nat: %w", err)
	}

	flowAccounting, err := r.store.IsFlowAccountingEnabled(ctx)
	if err != nil {
		return desired, fmt.Errorf("flow accounting: %w", err)
	}

	localSwitch, err := r.store.IsLocalSwitchEnabled(ctx)
	if err != nil {
		return desired, fmt.Errorf("local switch: %w", err)
	}

	desired.NAT = nat
	desired.FlowAccounting = flowAccounting
	desired.LocalSwitch = localSwitch

	return desired, nil
}

func (r *SettingsReconciler) reconcileN3Address(ctx context.Context) error {
	settings, err := r.store.GetN3Settings(ctx)
	if err != nil {
		// Initialize() seeds this row; absence is a transient race during boot.
		if errors.Is(err, db.ErrNotFound) {
			return nil
		}

		return err
	}

	desired := r.fallbackN3IP

	if settings.ExternalAddress != "" {
		parsed, err := netip.ParseAddr(settings.ExternalAddress)
		if err != nil {
			return fmt.Errorf("invalid external address %q: %w", settings.ExternalAddress, err)
		}

		desired = parsed
	}

	if !desired.IsValid() {
		return nil
	}

	r.stateMu.Lock()
	current := r.appliedN3Address
	r.stateMu.Unlock()

	if current == desired {
		return nil
	}

	r.updater.UpdateAdvertisedN3Address(desired)

	r.stateMu.Lock()
	r.appliedN3Address = desired
	r.stateMu.Unlock()

	logger.UpfLog.Info("applied advertised N3 address", zap.String("address", desired.String()))

	return nil
}

func (r *SettingsReconciler) reconcileFilters(ctx context.Context) error {
	policies, _, err := r.store.ListPoliciesPage(ctx, 1, 1000)
	if err != nil {
		return fmt.Errorf("list policies: %w", err)
	}

	desired := make(map[string]filterSnapshot, len(policies))

	for _, p := range policies {
		rules, err := r.store.ListRulesForPolicy(ctx, p.ID)
		if err != nil {
			return fmt.Errorf("list rules for policy %s: %w", p.ID, err)
		}

		desired[p.ID] = filterSnapshot{
			uplink:   networkRulesToFilterRules(rules, directionUplinkString),
			downlink: networkRulesToFilterRules(rules, directionDownlinkString),
		}
	}

	r.stateMu.Lock()
	applied := r.appliedFilters
	r.stateMu.Unlock()

	// Record only what actually reached the data plane, so a failed direction
	// still differs from desired on the next pass and is retried.
	nextApplied := make(map[string]filterSnapshot, len(desired))

	var errs []error

	for policyID, desiredSnap := range desired {
		appliedSnap, hadApplied := applied[policyID]
		result := appliedSnap

		if !hadApplied || !reflect.DeepEqual(appliedSnap.uplink, desiredSnap.uplink) {
			if err := r.updater.UpdateFilters(ctx, policyID, models.DirectionUplink, desiredSnap.uplink); err != nil {
				errs = append(errs, fmt.Errorf("update uplink filters for policy %s: %w", policyID, err))
			} else {
				result.uplink = desiredSnap.uplink
			}
		}

		if !hadApplied || !reflect.DeepEqual(appliedSnap.downlink, desiredSnap.downlink) {
			if err := r.updater.UpdateFilters(ctx, policyID, models.DirectionDownlink, desiredSnap.downlink); err != nil {
				errs = append(errs, fmt.Errorf("update downlink filters for policy %s: %w", policyID, err))
			} else {
				result.downlink = desiredSnap.downlink
			}
		}

		nextApplied[policyID] = result
	}

	for policyID, appliedSnap := range applied {
		if _, ok := desired[policyID]; ok {
			continue
		}

		// Policy deleted: clear its filters so the eBPF slot is freed.
		cleared := true

		if err := r.updater.UpdateFilters(ctx, policyID, models.DirectionUplink, nil); err != nil {
			errs = append(errs, fmt.Errorf("clear uplink filters for deleted policy %s: %w", policyID, err))
			cleared = false
		}

		if err := r.updater.UpdateFilters(ctx, policyID, models.DirectionDownlink, nil); err != nil {
			errs = append(errs, fmt.Errorf("clear downlink filters for deleted policy %s: %w", policyID, err))
			cleared = false
		}

		if !cleared {
			nextApplied[policyID] = appliedSnap
		}
	}

	r.stateMu.Lock()
	r.appliedFilters = nextApplied
	r.stateMu.Unlock()

	return errors.Join(errs...)
}

func networkRulesToFilterRules(rules []*db.NetworkRule, direction string) []models.FilterRule {
	out := make([]models.FilterRule, 0, len(rules))

	for _, rule := range rules {
		if rule.Direction != direction {
			continue
		}

		fr := models.FilterRule{
			Protocol: rule.Protocol,
			PortLow:  rule.PortLow,
			PortHigh: rule.PortHigh,
			Action:   models.ActionFromString(rule.Action),
		}

		if rule.RemotePrefix != nil {
			fr.RemotePrefix = *rule.RemotePrefix
		}

		out = append(out, fr)
	}

	return out
}
