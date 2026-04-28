// Copyright 2026 Ella Networks

package bgp

import (
	"context"
	"fmt"
	"reflect"
	"sort"
	"sync"
	"time"

	"github.com/ellanetworks/core/internal/logger"
	"go.uber.org/zap"
)

const defaultSettingsReconcileBackstop = 5 * time.Minute

// SettingsStore is the narrow view the reconciler needs over the
// replicated settings tables. The api layer satisfies it with a
// concrete adapter that reads from the DB and converts DB types into
// BGP types so this package stays free of the db dependency.
type SettingsStore interface {
	// GetSettings returns the desired BGP configuration. Enabled=false
	// means the speaker should be stopped.
	GetSettings(ctx context.Context) (BGPSettings, error)

	// ListPeers returns the desired peer set ordered however the
	// caller chooses; the reconciler sorts internally before diffing.
	ListPeers(ctx context.Context) ([]BGPPeer, error)

	// IsNATEnabled reports whether NAT is on; the reconciler maps
	// NAT-on to advertising-off so UE /32s aren't advertised when
	// traffic is being source-rewritten on egress.
	IsNATEnabled(ctx context.Context) (bool, error)
}

// SettingsService is the narrow view the reconciler needs over a
// running BGP service. *BGPService satisfies it.
type SettingsService interface {
	IsRunning() bool
	Start(ctx context.Context, settings BGPSettings, peers []BGPPeer, advertising bool) error
	Stop() error
	Reconfigure(ctx context.Context, settings BGPSettings, peers []BGPPeer) error
	SetAdvertising(advertising bool)
	UpdateFilter(filter *RouteFilter)
}

// FilterBuilder returns the RouteFilter this node should apply,
// derived from the receiver's local config (N3 address, N6 interface
// name) plus the cluster-wide UE pool set. Supplied by the api layer
// at reconciler construction time because the local-config inputs
// don't live in the bgp package.
type FilterBuilder func(ctx context.Context) (*RouteFilter, error)

// SettingsReconciler drives this node's BGP daemon lifecycle and
// configuration from the replicated settings, peer, and NAT tables.
// Each tick reads the desired state, compares against an in-memory
// snapshot of last-applied state, and only calls into the BGP
// service when the desired state has changed — Start, Stop, and
// Reconfigure all touch live BGP sessions, so unconditional
// re-application would interrupt established peerings.
type SettingsReconciler struct {
	service       SettingsService
	store         SettingsStore
	filterBuilder FilterBuilder
	wakeup        <-chan struct{}
	backstop      time.Duration
	log           *zap.Logger

	mu     sync.Mutex
	cancel context.CancelFunc
	done   chan struct{}

	stateMu            sync.Mutex
	appliedSettings    BGPSettings
	appliedPeers       []BGPPeer
	appliedAdvertising *bool
	appliedFilter      *RouteFilter
	hasApplied         bool
}

// NewSettingsReconciler wires a reconciler. filterBuilder may be nil
// to disable filter reconciliation. wakeup is signalled by the caller
// when a relevant replicated change has applied; nil is fine (then
// only the backstop sweep fires).
func NewSettingsReconciler(service SettingsService, store SettingsStore, filterBuilder FilterBuilder, wakeup <-chan struct{}) *SettingsReconciler {
	return &SettingsReconciler{
		service:       service,
		store:         store,
		filterBuilder: filterBuilder,
		wakeup:        wakeup,
		backstop:      defaultSettingsReconcileBackstop,
		log:           logger.EllaLog.With(zap.String("component", "BGPSettingsReconciler")),
	}
}

// Start launches the background goroutine. Subsequent calls without a
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

// MarkApplied lets the caller seed the in-memory snapshot to match
// what was already pushed into the BGP service at process start.
// Without this, the reconciler's first tick would diff against zero
// values and unnecessarily call Reconfigure on a service that's
// already correctly configured.
func (r *SettingsReconciler) MarkApplied(settings BGPSettings, peers []BGPPeer, advertising bool) {
	r.stateMu.Lock()
	defer r.stateMu.Unlock()

	r.appliedSettings = settings
	r.appliedPeers = clonePeers(peers)
	v := advertising
	r.appliedAdvertising = &v
	r.hasApplied = true
}

func (r *SettingsReconciler) loop(ctx context.Context, done chan struct{}) {
	defer close(done)

	if err := r.Reconcile(ctx); err != nil {
		r.log.Warn("initial bgp settings reconcile failed", zap.Error(err))
	}

	backstop := time.NewTicker(r.backstop)
	defer backstop.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-r.wakeup:
		case <-backstop.C:
		}

		if err := r.Reconcile(ctx); err != nil {
			r.log.Warn("bgp settings reconcile failed", zap.Error(err))
		}
	}
}

// Reconcile performs one pass. Exposed for tests and explicit triggers.
func (r *SettingsReconciler) Reconcile(ctx context.Context) error {
	desiredSettings, err := r.store.GetSettings(ctx)
	if err != nil {
		return fmt.Errorf("get settings: %w", err)
	}

	desiredPeers, err := r.store.ListPeers(ctx)
	if err != nil {
		return fmt.Errorf("list peers: %w", err)
	}

	natEnabled, err := r.store.IsNATEnabled(ctx)
	if err != nil {
		return fmt.Errorf("read nat setting: %w", err)
	}

	advertising := !natEnabled
	desiredPeers = sortPeers(desiredPeers)

	r.stateMu.Lock()
	hadApplied := r.hasApplied
	prevSettings := r.appliedSettings
	prevPeers := clonePeers(r.appliedPeers)
	prevAdvertising := r.appliedAdvertising
	r.stateMu.Unlock()

	switch {
	case !desiredSettings.Enabled && r.service.IsRunning():
		if err := r.service.Stop(); err != nil {
			return fmt.Errorf("stop: %w", err)
		}

		r.log.Info("stopped BGP service from reconcile")

	case desiredSettings.Enabled && !r.service.IsRunning():
		if err := r.service.Start(ctx, desiredSettings, desiredPeers, advertising); err != nil {
			return fmt.Errorf("start: %w", err)
		}

		r.log.Info("started BGP service from reconcile",
			zap.Int("peers", len(desiredPeers)),
			zap.Bool("advertising", advertising))

	case desiredSettings.Enabled && r.service.IsRunning():
		settingsChanged := !hadApplied || prevSettings != desiredSettings
		peersChanged := !hadApplied || !peersEqual(prevPeers, desiredPeers)

		if settingsChanged || peersChanged {
			if err := r.service.Reconfigure(ctx, desiredSettings, desiredPeers); err != nil {
				return fmt.Errorf("reconfigure: %w", err)
			}

			r.log.Info("reconfigured BGP service from reconcile",
				zap.Bool("settings_changed", settingsChanged),
				zap.Bool("peers_changed", peersChanged))
		}
	}

	if r.service.IsRunning() {
		if prevAdvertising == nil || *prevAdvertising != advertising {
			r.service.SetAdvertising(advertising)
			r.log.Info("set BGP advertising flag from reconcile", zap.Bool("advertising", advertising))
		}
	}

	var nextFilter *RouteFilter

	if r.filterBuilder != nil {
		f, err := r.filterBuilder(ctx)
		if err != nil {
			return fmt.Errorf("build filter: %w", err)
		}

		nextFilter = f

		r.stateMu.Lock()
		prevFilter := r.appliedFilter
		r.stateMu.Unlock()

		if !filtersEqual(prevFilter, nextFilter) {
			r.service.UpdateFilter(nextFilter)
			r.log.Info("updated BGP route filter from reconcile")
		}
	}

	r.stateMu.Lock()
	r.appliedSettings = desiredSettings
	r.appliedPeers = clonePeers(desiredPeers)
	v := advertising

	r.appliedAdvertising = &v
	if nextFilter != nil {
		r.appliedFilter = nextFilter
	}

	r.hasApplied = true
	r.stateMu.Unlock()

	return nil
}

func filtersEqual(a, b *RouteFilter) bool {
	switch {
	case a == nil && b == nil:
		return true
	case a == nil || b == nil:
		return false
	}

	return reflect.DeepEqual(a.RejectPrefixes, b.RejectPrefixes)
}

func sortPeers(peers []BGPPeer) []BGPPeer {
	out := clonePeers(peers)
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })

	return out
}

func clonePeers(peers []BGPPeer) []BGPPeer {
	if peers == nil {
		return nil
	}

	out := make([]BGPPeer, len(peers))
	copy(out, peers)

	return out
}

func peersEqual(a, b []BGPPeer) bool {
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
