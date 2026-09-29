// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package smsf

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
	"sync"
	"time"

	"github.com/ellanetworks/core/diameter"
	"github.com/ellanetworks/core/diameter/s6c"
	"github.com/ellanetworks/core/diameter/sgd"
	"github.com/ellanetworks/core/diameter/tgpp"
	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/diameternode"
	"go.uber.org/zap"
)

const PeerRoleSMSC = "smsc"

var smscApplications = []diameter.Application{
	{ID: sgd.ApplicationID, VendorID: tgpp.VendorID},
	{ID: s6c.ApplicationID, VendorID: tgpp.VendorID},
}

var (
	ErrUserUnknown         = errors.New("smsf: UE not known on this node")
	ErrNotRegisteredForSMS = errors.New("smsf: UE is not registered for SMS")
	ErrSMSCUnavailable     = errors.New("smsf: SMSC not connected")
)

type AbsentError struct {
	Diagnostic uint32
}

func (e *AbsentError) Error() string {
	return fmt.Sprintf("smsf: UE absent (diagnostic %d)", e.Diagnostic)
}

type Access uint8

const (
	AccessEPS Access = iota + 1
	Access5GS
)

func (a Access) String() string {
	switch a {
	case AccessEPS:
		return "EPS"
	case Access5GS:
		return "5GS"
	default:
		return "unknown"
	}
}

type Transport interface {
	EnableUEReachability(ctx context.Context, imsi string, access Access) error
	SendSMS(ctx context.Context, imsi string, access Access, payload []byte) error
	SignallingSettled(ctx context.Context, imsi string, access Access)
}

type Store interface {
	GetSMSSettings(ctx context.Context) (*db.SMSSettings, error)
	GetSubscriber(ctx context.Context, imsi string) (*db.Subscriber, error)
	GetSubscriberByMSISDN(ctx context.Context, msisdn string) (*db.Subscriber, error)
	GetUERegistration(ctx context.Context, imsi, regType string) (*db.UERegistration, error)
}

type Directory interface {
	Identity(ctx context.Context, nodeID string) (diameternode.Identity, error)
}

type DiameterNode interface {
	Node() *diameter.Node
	Peers() []diameternode.PeerStatus
}

type Registrar interface {
	Handle(applicationID, commandCode uint32, h diameter.Handler)
}

type Timers struct {
	TC1                time.Duration
	MaxRetransmissions int
	TR1N               time.Duration
	TR2N               time.Duration
	AlertTimeout       time.Duration
}

func DefaultTimers() Timers {
	return Timers{
		TC1:                6 * time.Second,
		MaxRetransmissions: 2,
		TR1N:               25 * time.Second,
		TR2N:               25 * time.Second,
		AlertTimeout:       10 * time.Second,
	}
}

type SMSF struct {
	store     Store
	directory Directory
	diameter  DiameterNode
	logger    *zap.Logger
	timers    Timers

	transport Transport

	mu         sync.Mutex
	ues        map[string]*ueState
	waiting    map[string]waiting
	registered map[string]registration
}

type registration struct {
	access Access
	owner  any
}

func New(store Store, directory Directory, node DiameterNode, logger *zap.Logger, timers Timers) *SMSF {
	return &SMSF{
		store:      store,
		directory:  directory,
		diameter:   node,
		logger:     logger,
		timers:     timers,
		ues:        make(map[string]*ueState),
		waiting:    make(map[string]waiting),
		registered: make(map[string]registration),
	}
}

func (s *SMSF) SetTransport(t Transport) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.transport = t
}

func (s *SMSF) currentTransport() Transport {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.transport
}

func (s *SMSF) route(imsi string) (Transport, Access, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	r, ok := s.registered[imsi]
	if !ok {
		return nil, 0, ErrNotRegisteredForSMS
	}

	if s.transport == nil {
		return nil, 0, ErrUserUnknown
	}

	return s.transport, r.access, nil
}

func (s *SMSF) Activate(imsi string, access Access, owner any) {
	s.mu.Lock()
	previous, ok := s.registered[imsi]
	s.registered[imsi] = registration{access: access, owner: owner}
	s.mu.Unlock()

	if !ok || previous.access != access {
		s.logger.Info("UE registered for SMS", zap.String("imsi", imsi), zap.Stringer("access", access))
	}
}

func (s *SMSF) Deactivate(imsi string, access Access, owner any) {
	s.mu.Lock()

	current, ok := s.registered[imsi]
	if !ok || current.access != access || current.owner != owner {
		s.mu.Unlock()
		return
	}

	delete(s.registered, imsi)

	if u, ok := s.ues[imsi]; ok && u.mt == nil && len(u.mo) == 0 {
		delete(s.ues, imsi)
	}

	s.mu.Unlock()

	s.logger.Info("UE deregistered from SMS", zap.String("imsi", imsi), zap.Stringer("access", access))
}

func (s *SMSF) RegisteredAccess(imsi string) (Access, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	r, ok := s.registered[imsi]

	return r.access, ok
}

func SMSCPeer(address netip.AddrPort) diameternode.PeerConfig {
	return diameternode.PeerConfig{Role: PeerRoleSMSC, Address: address, Applications: smscApplications}
}

func (s *SMSF) Register(r Registrar) {
	r.Handle(s6c.ApplicationID, s6c.CommandSendRoutingInfoForSM, diameter.HandlerFunc(func(ctx context.Context, c *diameter.Conn, req *diameter.Message) *diameter.Message {
		return s.SendRoutingInfoForSM(ctx, c.LocalIdentity(), req)
	}))
	r.Handle(s6c.ApplicationID, s6c.CommandReportSMDeliveryStatus, diameter.HandlerFunc(func(ctx context.Context, c *diameter.Conn, req *diameter.Message) *diameter.Message {
		return s.ReportSMDeliveryStatus(ctx, c.LocalIdentity(), req)
	}))
	r.Handle(sgd.ApplicationID, sgd.CommandMTForwardShortMessage, diameter.HandlerFunc(func(ctx context.Context, c *diameter.Conn, req *diameter.Message) *diameter.Message {
		return s.MTForwardShortMessage(ctx, c.LocalIdentity(), req)
	}))
}

func (s *SMSF) Allowed(ctx context.Context, imsi string) (bool, error) {
	settings, err := s.store.GetSMSSettings(ctx)
	if err != nil {
		return false, fmt.Errorf("get SMS settings: %w", err)
	}

	if !settings.Enabled() {
		return false, nil
	}

	sub, err := s.store.GetSubscriber(ctx, imsi)
	if errors.Is(err, db.ErrNotFound) {
		return false, nil
	}

	if err != nil {
		return false, fmt.Errorf("get subscriber: %w", err)
	}

	return sub.Msisdn != "", nil
}

type smscEndpoint struct {
	node  *diameter.Node
	host  string
	realm string
}

func (s *SMSF) smsc() (smscEndpoint, error) {
	node := s.diameter.Node()
	if node == nil {
		return smscEndpoint{}, ErrSMSCUnavailable
	}

	for _, p := range s.diameter.Peers() {
		if p.Role == PeerRoleSMSC && p.State == diameter.PeerOpen {
			return smscEndpoint{node: node, host: p.Host, realm: p.Realm}, nil
		}
	}

	return smscEndpoint{}, ErrSMSCUnavailable
}

func (e smscEndpoint) envelope() tgpp.Envelope {
	return tgpp.Envelope{
		SessionID:        e.node.NewSessionID(),
		Origin:           e.node.Identity(),
		DestinationHost:  e.host,
		DestinationRealm: e.realm,
	}
}
