// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ausf

import (
	"context"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/udm"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

var tracer = otel.Tracer("ella-core/ausf")

// SubscriberStore, Subscriber, and KeyResolver are the credential-authority
// types, defined in package udm; aliased here so existing callers (and the AMF
// wiring) are unaffected by the extraction.
type (
	SubscriberStore = udm.SubscriberStore
	Subscriber      = udm.Subscriber
	KeyResolver     = udm.KeyResolver
)

// AuthResult is returned by Authenticate to the AMF.
type AuthResult struct {
	Rand      string // hex
	Autn      string // hex
	HxresStar string // hex
}

// ResyncInfo carries the UE's re-synchronization data.
type ResyncInfo struct {
	Auts string // hex — AUTS from the UE
}

type authContext struct {
	supi      etsi.SUPI
	kseaf     string
	xresStar  string
	rand      string
	createdAt time.Time
}

// AUSF implements the 5G-AKA authentication server function. It is a consumer of
// the udm credential authority (which generates the HE AV and owns SQN); the
// AUSF does the 5G-specific transform (K_SEAF, HXRES*) and RES* verification.
type AUSF struct {
	mu    sync.RWMutex
	pool  map[string]*authContext // key: SUCI
	udm   *udm.Service
	clock func() time.Time
	ttl   time.Duration
}

// Option configures an AUSF instance.
type Option func(*AUSF)

// WithClock overrides the time source (useful for testing).
func WithClock(fn func() time.Time) Option { return func(a *AUSF) { a.clock = fn } }

// WithTTL overrides the auth context time-to-live (useful for testing).
func WithTTL(d time.Duration) Option { return func(a *AUSF) { a.ttl = d } }

// New creates a new AUSF. The caller must run a.Run(ctx) in a goroutine.
func New(store SubscriberStore, keys KeyResolver, opts ...Option) *AUSF {
	a := &AUSF{
		pool:  make(map[string]*authContext),
		udm:   udm.New(store, keys),
		clock: time.Now,
		ttl:   60 * time.Second,
	}
	for _, o := range opts {
		o(a)
	}

	return a
}

// Run starts the background context cleanup loop. It blocks until ctx is cancelled.
func (a *AUSF) Run(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			a.evictExpired()
		}
	}
}

func (a *AUSF) evictExpired() {
	a.mu.Lock()
	defer a.mu.Unlock()

	now := a.clock()
	for suci, ac := range a.pool {
		if now.Sub(ac.createdAt) > a.ttl {
			delete(a.pool, suci)
		}
	}
}

// Authenticate performs the 5G-AKA authentication procedure for a UE.
// It returns the authentication vector to send to the UE and caches
// the pending context for later confirmation via Confirm.
func (a *AUSF) Authenticate(ctx context.Context, suci string, plmn models.PlmnID, resync *ResyncInfo) (*AuthResult, error) {
	servingNetwork, err := plmn.ServingNetworkName()
	if err != nil {
		return nil, fmt.Errorf("invalid PLMN for serving network name: %w", err)
	}

	ctx, span := tracer.Start(ctx, "ausf/authenticate",
		trace.WithSpanKind(trace.SpanKindInternal),
		trace.WithAttributes(
			attribute.String("ue.suci", suci),
			attribute.String("ausf.serving_network", servingNetwork),
		),
	)
	defer span.End()

	// If resync, recover the original RAND from the cached context.
	var resyncAuts, resyncRand string

	if resync != nil {
		a.mu.RLock()
		cached := a.pool[suci]
		a.mu.RUnlock()

		if cached == nil {
			err := fmt.Errorf("ue context not found for suci: %s", suci)
			span.RecordError(err)
			span.SetStatus(codes.Error, "ue context not found")

			return nil, err
		}

		resyncAuts = resync.Auts
		resyncRand = cached.rand
	}

	// The credential authority (UDM/ARPF) deconceals the SUCI, advances the SQN,
	// and produces the 5G HE AV (TS 33.501 §6.1.3.2.0).
	heav, err := a.udm.Generate5GHEAV(ctx, suci, servingNetwork, resyncAuts, resyncRand)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to generate 5G HE AV")

		return nil, err
	}

	span.AddEvent("auth_vector_generated")

	// AUSF transform: HXRES* and the anchor key K_SEAF (TS 33.501 §6.2.2.1).
	hxresStar, err := deriveHxresStar(heav.RAND, heav.XresStar)
	if err != nil {
		return nil, fmt.Errorf("HXRES* derivation failed: %w", err)
	}

	kseaf, err := deriveKseaf(heav.Kausf, servingNetwork)
	if err != nil {
		return nil, fmt.Errorf("kseaf derivation failed: %w", err)
	}

	// Cache context for Confirm.
	a.mu.Lock()
	a.pool[suci] = &authContext{
		supi:      heav.SUPI,
		kseaf:     hex.EncodeToString(kseaf),
		xresStar:  heav.XresStar,
		rand:      heav.RAND,
		createdAt: a.clock(),
	}
	a.mu.Unlock()
	span.AddEvent("context_pooled")

	return &AuthResult{
		Rand:      heav.RAND,
		Autn:      heav.AUTN,
		HxresStar: hxresStar,
	}, nil
}

// Confirm verifies the UE's RES* against the cached XRES*.
// On success it returns the SUPI and Kseaf. The cached context is
// deleted regardless of outcome.
func (a *AUSF) Confirm(ctx context.Context, resStar, suci string) (etsi.SUPI, string, error) {
	_, span := tracer.Start(ctx, "ausf/confirm",
		trace.WithSpanKind(trace.SpanKindInternal),
		trace.WithAttributes(
			attribute.String("ue.suci", suci),
		),
	)
	defer span.End()

	a.mu.Lock()
	cached, ok := a.pool[suci]
	delete(a.pool, suci)
	a.mu.Unlock()

	if !ok {
		return etsi.InvalidSUPI, "", fmt.Errorf("ausf ue context not found for suci: %s", suci)
	}

	if subtle.ConstantTimeCompare([]byte(resStar), []byte(cached.xresStar)) != 1 {
		return etsi.InvalidSUPI, "", fmt.Errorf("RES* mismatch for suci: %s", suci)
	}

	return cached.supi, cached.kseaf, nil
}
