// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ausf

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/tracing/attrs"
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

	AdvancedCredentials = udm.AdvancedCredentials
)

// UEAuthenticationCtx is returned by Authenticate to the AMF.
type UEAuthenticationCtx struct {
	AuthCtxID string
	Av5gAka
}

type Av5gAka struct {
	Rand      string // hex
	HxresStar string // hex
	Autn      string // hex
}

// ResynchronizationInfo carries the UE's re-synchronization data.
type ResynchronizationInfo struct {
	Rand string
	Auts string // hex — AUTS from the UE
}

var ErrAuthContextNotFound = errors.New("authentication context not found")

type authContext struct {
	supi      etsi.SUPI
	kseaf     []byte
	xresStar  string
	createdAt time.Time
}

// AUSF implements the 5G-AKA authentication server function. It is a consumer of
// the udm credential authority (which generates the HE AV and owns SQN); the
// AUSF does the 5G-specific transform (K_SEAF, HXRES*) and RES* verification.
type AUSF struct {
	mu    sync.RWMutex
	pool  map[string]*authContext
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
	for id, ac := range a.pool {
		if now.Sub(ac.createdAt) > a.ttl {
			delete(a.pool, id)
		}
	}
}

// Authenticate performs the 5G-AKA authentication procedure for a UE
// identified by its SUCI or SUPI. It returns the authentication vector to send
// to the UE and caches the pending context under the returned AuthCtxID for
// later confirmation via Confirm.
func (a *AUSF) Authenticate(ctx context.Context, supiOrSuci string, plmn models.PlmnID, resync *ResynchronizationInfo) (*UEAuthenticationCtx, error) {
	servingNetwork, err := plmn.ServingNetworkName()
	if err != nil {
		return nil, fmt.Errorf("invalid PLMN for serving network name: %w", err)
	}

	ctx, span := tracer.Start(ctx, "ausf/authenticate",
		trace.WithSpanKind(trace.SpanKindInternal),
		trace.WithAttributes(
			identityAttribute(supiOrSuci),
			attribute.String("ausf.serving_network", servingNetwork),
		),
	)
	defer span.End()

	var resyncAuts, resyncRand string

	if resync != nil {
		if resync.Rand == "" || resync.Auts == "" {
			err := errors.New("re-synchronisation requires both RAND and AUTS")
			span.RecordError(err)
			span.SetStatus(codes.Error, "incomplete re-synchronisation info")

			return nil, err
		}

		resyncAuts = resync.Auts
		resyncRand = resync.Rand
	}

	// The credential authority (UDM/ARPF) resolves the SUPI (deconcealing a SUCI),
	// advances the SQN, and produces the 5G HE AV (TS 33.501 §6.1.3.2.0).
	heav, err := a.udm.Generate5GHEAV(ctx, supiOrSuci, servingNetwork, resyncAuts, resyncRand)
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

	id, err := newAuthCtxID()
	if err != nil {
		return nil, err
	}

	// Cache context for Confirm.
	a.mu.Lock()
	a.pool[id] = &authContext{
		supi:      heav.SUPI,
		kseaf:     kseaf,
		xresStar:  heav.XresStar,
		createdAt: a.clock(),
	}
	a.mu.Unlock()
	span.AddEvent("context_pooled")

	return &UEAuthenticationCtx{
		AuthCtxID: id,
		Av5gAka: Av5gAka{
			Rand:      heav.RAND,
			HxresStar: hxresStar,
			Autn:      heav.AUTN,
		},
	}, nil
}

// Confirm verifies the UE's RES* against the XRES* of the authentication
// context authCtxID. On success it returns the SUPI and Kseaf. The context is
// deleted regardless of outcome.
func (a *AUSF) Confirm(ctx context.Context, authCtxID, resStar string) (etsi.SUPI, []byte, error) {
	_, span := tracer.Start(ctx, "ausf/confirm",
		trace.WithSpanKind(trace.SpanKindInternal),
	)
	defer span.End()

	a.mu.Lock()
	cached, ok := a.pool[authCtxID]
	delete(a.pool, authCtxID)
	a.mu.Unlock()

	if !ok {
		return etsi.InvalidSUPI, nil, fmt.Errorf("%w: %s", ErrAuthContextNotFound, authCtxID)
	}

	span.SetAttributes(attrs.SUPI(cached.supi.String()))

	if subtle.ConstantTimeCompare([]byte(resStar), []byte(cached.xresStar)) != 1 {
		return etsi.InvalidSUPI, nil, fmt.Errorf("RES* mismatch for supi: %s", cached.supi)
	}

	return cached.supi, cached.kseaf, nil
}

func (a *AUSF) ReportAuthenticationFailure(ctx context.Context, authCtxID string) {
	_, span := tracer.Start(ctx, "ausf/report_authentication_failure",
		trace.WithSpanKind(trace.SpanKindInternal),
	)
	defer span.End()

	a.mu.Lock()
	_, ok := a.pool[authCtxID]
	delete(a.pool, authCtxID)
	a.mu.Unlock()

	span.SetAttributes(attribute.Bool("ausf.context_found", ok))
}

func newAuthCtxID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("authentication context id: %w", err)
	}

	return hex.EncodeToString(b), nil
}

func identityAttribute(supiOrSuci string) attribute.KeyValue {
	if strings.HasPrefix(supiOrSuci, "suci-") {
		return attrs.SUCI(supiOrSuci)
	}

	return attrs.SUPI(supiOrSuci)
}
