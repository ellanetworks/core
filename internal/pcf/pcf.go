// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package pcf

import (
	"context"
	"net/netip"
	"sync"
	"time"

	"github.com/ellanetworks/core/diameter"
	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/smf"
	"go.opentelemetry.io/otel"
	"go.uber.org/zap"
)

var tracer = otel.Tracer("ella-core/pcf")

var _ smf.PCF = (*PCF)(nil)

type Store interface {
	GetSessionPolicy(ctx context.Context, imsi string, sst int32, sd string, dnn string) (*db.Policy, error)
}

type Diameter interface {
	Node() *diameter.Node
	Handle(applicationID, commandCode uint32, h diameter.Handler)
}

type PCF struct {
	store Store
	log   *zap.Logger

	abortTimeout     time.Duration
	abortedRetention time.Duration
	notifyTimeout    time.Duration

	mu           sync.Mutex
	diameter     Diameter
	enforcer     Enforcer
	revision     uint64
	associations map[string]*association
	byAddress    map[netip.Addr]*association
	rxSessions   map[string]*rxSession
}

func New(store Store, log *zap.Logger) *PCF {
	return &PCF{
		store:            store,
		log:              log,
		abortTimeout:     10 * time.Second,
		abortedRetention: time.Minute,
		notifyTimeout:    time.Minute,
		associations:     make(map[string]*association),
		byAddress:        make(map[netip.Addr]*association),
		rxSessions:       make(map[string]*rxSession),
	}
}

func (p *PCF) Attach(d Diameter) {
	p.mu.Lock()
	p.diameter = d
	p.mu.Unlock()

	p.registerRx(d)
}

func (p *PCF) node() *diameter.Node {
	p.mu.Lock()
	d := p.diameter
	p.mu.Unlock()

	if d == nil {
		return nil
	}

	return d.Node()
}
