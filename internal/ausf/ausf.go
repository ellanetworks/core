// Copyright 2026 Ella Networks
// SPDX-License-Identifier: Apache-2.0

package ausf

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"regexp"
	"sync"
	"time"

	"github.com/ellanetworks/core/etsi"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

var tracer = otel.Tracer("ella-core/ausf")

// SubscriberStore is the minimal DB surface the AUSF needs.
type SubscriberStore interface {
	GetSubscriber(ctx context.Context, imsi string) (*Subscriber, error)
	UpdateSequenceNumber(ctx context.Context, imsi string, sqn string) error
}

// Subscriber contains the authentication material the AUSF needs.
type Subscriber struct {
	PermanentKey   string
	Opc            string
	SequenceNumber string
}

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

// AUSF implements the 5G-AKA authentication server function.
type AUSF struct {
	mu                  sync.RWMutex
	pool                map[string]*authContext // key: SUCI
	store               SubscriberStore
	keys                KeyResolver
	clock               func() time.Time
	ttl                 time.Duration
	servingNetworkRegex *regexp.Regexp
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
		pool:                make(map[string]*authContext),
		store:               store,
		keys:                keys,
		clock:               time.Now,
		ttl:                 60 * time.Second,
		servingNetworkRegex: regexp.MustCompile(`^5G:mnc[0-9]{3}\.mcc[0-9]{3}\.3gppnetwork\.org$`),
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

func (a *AUSF) isServingNetworkAuthorized(lookup string) bool {
	return a.servingNetworkRegex.MatchString(lookup)
}

// Authenticate performs the 5G-AKA authentication procedure for a UE.
// It returns the authentication vector to send to the UE and caches
// the pending context for later confirmation via Confirm.
func (a *AUSF) Authenticate(ctx context.Context, suci, servingNetwork string, resync *ResyncInfo) (*AuthResult, error) {
	ctx, span := tracer.Start(ctx, "ausf/authenticate",
		trace.WithSpanKind(trace.SpanKindInternal),
		trace.WithAttributes(
			attribute.String("ue.suci", suci),
			attribute.String("ausf.serving_network", servingNetwork),
		),
	)
	defer span.End()

	if !a.isServingNetworkAuthorized(servingNetwork) {
		err := fmt.Errorf("serving network not authorized: %s", servingNetwork)
		span.RecordError(err)
		span.SetStatus(codes.Error, "serving network not authorized")

		return nil, err
	}

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

	// Convert SUCI → SUPI
	supi, err := ToSupi(suci, a.keys)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to convert suci to supi")

		return nil, fmt.Errorf("couldn't convert suci to supi: %w", err)
	}

	// Fetch subscriber auth material
	sub, err := a.store.GetSubscriber(ctx, supi.IMSI())
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to get subscriber")

		return nil, fmt.Errorf("couldn't get subscriber %s: %w", supi, err)
	}

	if sub.PermanentKey == "" {
		err := fmt.Errorf("permanent key is empty")
		span.RecordError(err)
		span.SetStatus(codes.Error, "permanent key is empty")

		return nil, err
	}

	if sub.Opc == "" {
		err := fmt.Errorf("opc is empty")
		span.RecordError(err)
		span.SetStatus(codes.Error, "opc is empty")

		return nil, err
	}

	k, err := hex.DecodeString(sub.PermanentKey)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to decode permanent key")

		return nil, fmt.Errorf("failed to decode k: %w", err)
	}

	opc, err := hex.DecodeString(sub.Opc)
	if err != nil {
		return nil, fmt.Errorf("failed to decode opc: %w", err)
	}

	sqnStr := strictHex(sub.SequenceNumber, 12)

	// Handle SQN re-synchronization
	var nextSQN string

	if resync != nil {
		auts, err := hex.DecodeString(resyncAuts)
		if err != nil {
			return nil, fmt.Errorf("could not decode auts: %w", err)
		}

		randBytes, err := hex.DecodeString(resyncRand)
		if err != nil {
			return nil, fmt.Errorf("could not decode rand: %w", err)
		}

		sqnMsHex, err := resyncSQN(opc, k, auts, randBytes)
		if err != nil {
			return nil, fmt.Errorf("SQN resync failed for %s: %w", supi, err)
		}

		// TS 33.102 §C.3.4: after resync, advance by IND+1 (=33) to
		// move to the next IND slot.
		nextSQN, err = advanceSQN(sqnMsHex, indStep+1)
		if err != nil {
			return nil, fmt.Errorf("SQN advance failed: %w", err)
		}
	} else {
		var err error

		nextSQN, err = advanceSQN(sqnStr, indStep)
		if err != nil {
			return nil, fmt.Errorf("SQN increment failed: %w", err)
		}
	}

	// Use the incremented SQN for the authentication vector.
	sqn, err := hex.DecodeString(nextSQN)
	if err != nil {
		return nil, fmt.Errorf("error decoding sqn: %w", err)
	}

	err = a.store.UpdateSequenceNumber(ctx, supi.IMSI(), nextSQN)
	if err != nil {
		return nil, fmt.Errorf("couldn't update subscriber %s: %w", supi, err)
	}

	// Generate RAND
	RAND := make([]byte, 16)
	if _, err = rand.Read(RAND); err != nil {
		return nil, fmt.Errorf("rand read error: %w", err)
	}

	// Run Milenage
	macA, macS := make([]byte, 8), make([]byte, 8)
	CK, IK := make([]byte, 16), make([]byte, 16)
	RES := make([]byte, 8)
	AK := make([]byte, 6)

	amf, err := hex.DecodeString("8000")
	if err != nil {
		return nil, fmt.Errorf("amf decode error: %w", err)
	}

	if err = F1(opc, k, RAND, sqn, amf, macA, macS); err != nil {
		return nil, fmt.Errorf("milenage F1 err: %w", err)
	}

	if err = F2345(opc, k, RAND, RES, CK, IK, AK, nil); err != nil {
		return nil, fmt.Errorf("milenage F2345 err: %w", err)
	}

	// Build AUTN = (SQN ⊕ AK) || AMF || MAC-A
	sqnXorAK := make([]byte, 6)
	for i := range sqn {
		sqnXorAK[i] = sqn[i] ^ AK[i]
	}

	AUTN := append(append(sqnXorAK, amf...), macA...)

	// Derive XRES*
	xresStar, err := deriveXresStar(CK, IK, servingNetwork, RAND, RES)
	if err != nil {
		return nil, fmt.Errorf("XRES* derivation failed: %w", err)
	}

	// Derive Kausf
	kausf, err := deriveKausf(CK, IK, servingNetwork, sqnXorAK)
	if err != nil {
		return nil, fmt.Errorf("kausf derivation failed: %w", err)
	}

	randHex := hex.EncodeToString(RAND)
	xresStarHex := hex.EncodeToString(xresStar)

	// Derive HXRES*
	hxresStar, err := deriveHxresStar(randHex, xresStarHex)
	if err != nil {
		return nil, fmt.Errorf("HXRES* derivation failed: %w", err)
	}

	// Derive Kseaf
	kseaf, err := deriveKseaf(kausf, servingNetwork)
	if err != nil {
		return nil, fmt.Errorf("kseaf derivation failed: %w", err)
	}

	// Cache context for Confirm
	a.mu.Lock()
	a.pool[suci] = &authContext{
		supi:      supi,
		kseaf:     hex.EncodeToString(kseaf),
		xresStar:  xresStarHex,
		rand:      randHex,
		createdAt: a.clock(),
	}
	a.mu.Unlock()

	return &AuthResult{
		Rand:      randHex,
		Autn:      hex.EncodeToString(AUTN),
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
