// Copyright 2026 Ella Networks

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
	GetN3Settings(ctx context.Context) (*db.N3Settings, error)
	ListPoliciesPage(ctx context.Context, page int, perPage int) ([]db.Policy, int, error)
	ListRulesForPolicy(ctx context.Context, policyID int64) ([]*db.NetworkRule, error)
}

// Updater is the narrow view the reconciler needs over the UPF runtime.
// *UPF satisfies it.
type Updater interface {
	ReloadNAT(enabled bool) error
	ReloadFlowAccounting(enabled bool) error
	UpdateAdvertisedN3Address(addr netip.Addr)
	UpdateFilters(ctx context.Context, policyID int64, direction models.Direction, rules []models.FilterRule) error
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

	stateMu               sync.Mutex
	appliedNAT            *bool
	appliedFlowAccounting *bool
	appliedN3Address      netip.Addr
	appliedFilters        map[int64]filterSnapshot
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
		appliedFilters: make(map[int64]filterSnapshot),
	}
}

// Start launches the reconciler goroutine. Subsequent calls without a
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

	go r.loop(ctx, r.done)
}

// Stop signals the reconciler to exit and blocks until the goroutine
// has drained.
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

func (r *SettingsReconciler) loop(ctx context.Context, done chan struct{}) {
	defer close(done)

	var (
		events  <-chan db.Event
		dropped <-chan struct{}
	)

	if r.changefeed != nil {
		sub := r.changefeed.Subscribe(
			db.TopicNATSettings,
			db.TopicFlowAccountingSettings,
			db.TopicN3Settings,
			db.TopicPolicies,
			db.TopicNetworkRules,
		)
		defer sub.Close()

		events = sub.Events
		dropped = sub.Dropped
	}

	if err := r.Reconcile(ctx); err != nil {
		logger.UpfLog.Warn("upf settings reconcile failed", zap.Error(err))
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

		if err := r.Reconcile(ctx); err != nil {
			logger.UpfLog.Warn("upf settings reconcile failed", zap.Error(err))
		}
	}
}

// Reconcile performs one reconcile pass. Exposed for tests and for
// callers that want to force convergence after a known change.
func (r *SettingsReconciler) Reconcile(ctx context.Context) error {
	if err := r.reconcileNAT(ctx); err != nil {
		return fmt.Errorf("nat: %w", err)
	}

	if err := r.reconcileFlowAccounting(ctx); err != nil {
		return fmt.Errorf("flow accounting: %w", err)
	}

	if err := r.reconcileN3Address(ctx); err != nil {
		return fmt.Errorf("n3 address: %w", err)
	}

	if err := r.reconcileFilters(ctx); err != nil {
		return fmt.Errorf("policy filters: %w", err)
	}

	return nil
}

func (r *SettingsReconciler) reconcileNAT(ctx context.Context) error {
	desired, err := r.store.IsNATEnabled(ctx)
	if err != nil {
		return err
	}

	r.stateMu.Lock()
	current := r.appliedNAT
	r.stateMu.Unlock()

	if current != nil && *current == desired {
		return nil
	}

	if err := r.updater.ReloadNAT(desired); err != nil {
		return err
	}

	r.stateMu.Lock()
	v := desired
	r.appliedNAT = &v
	r.stateMu.Unlock()

	logger.UpfLog.Info("applied NAT setting", zap.Bool("enabled", desired))

	return nil
}

func (r *SettingsReconciler) reconcileFlowAccounting(ctx context.Context) error {
	desired, err := r.store.IsFlowAccountingEnabled(ctx)
	if err != nil {
		return err
	}

	r.stateMu.Lock()
	current := r.appliedFlowAccounting
	r.stateMu.Unlock()

	if current != nil && *current == desired {
		return nil
	}

	if err := r.updater.ReloadFlowAccounting(desired); err != nil {
		return err
	}

	r.stateMu.Lock()
	v := desired
	r.appliedFlowAccounting = &v
	r.stateMu.Unlock()

	logger.UpfLog.Info("applied flow accounting setting", zap.Bool("enabled", desired))

	return nil
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

	desired := make(map[int64]filterSnapshot, len(policies))

	for _, p := range policies {
		rules, err := r.store.ListRulesForPolicy(ctx, int64(p.ID))
		if err != nil {
			return fmt.Errorf("list rules for policy %d: %w", p.ID, err)
		}

		desired[int64(p.ID)] = filterSnapshot{
			uplink:   networkRulesToFilterRules(rules, directionUplinkString),
			downlink: networkRulesToFilterRules(rules, directionDownlinkString),
		}
	}

	r.stateMu.Lock()
	applied := r.appliedFilters
	r.stateMu.Unlock()

	for policyID, desiredSnap := range desired {
		appliedSnap, hadApplied := applied[policyID]

		if !hadApplied || !reflect.DeepEqual(appliedSnap.uplink, desiredSnap.uplink) {
			if err := r.updater.UpdateFilters(ctx, policyID, models.DirectionUplink, desiredSnap.uplink); err != nil {
				logger.UpfLog.Warn("failed to update uplink filters",
					zap.Int64("policyID", policyID), zap.Error(err))

				continue
			}
		}

		if !hadApplied || !reflect.DeepEqual(appliedSnap.downlink, desiredSnap.downlink) {
			if err := r.updater.UpdateFilters(ctx, policyID, models.DirectionDownlink, desiredSnap.downlink); err != nil {
				logger.UpfLog.Warn("failed to update downlink filters",
					zap.Int64("policyID", policyID), zap.Error(err))

				continue
			}
		}
	}

	for policyID := range applied {
		if _, ok := desired[policyID]; ok {
			continue
		}

		// Policy deleted: clear filters so the eBPF slot is freed.
		if err := r.updater.UpdateFilters(ctx, policyID, models.DirectionUplink, nil); err != nil {
			logger.UpfLog.Warn("failed to clear uplink filters for deleted policy",
				zap.Int64("policyID", policyID), zap.Error(err))
		}

		if err := r.updater.UpdateFilters(ctx, policyID, models.DirectionDownlink, nil); err != nil {
			logger.UpfLog.Warn("failed to clear downlink filters for deleted policy",
				zap.Int64("policyID", policyID), zap.Error(err))
		}
	}

	r.stateMu.Lock()
	r.appliedFilters = desired
	r.stateMu.Unlock()

	return nil
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
