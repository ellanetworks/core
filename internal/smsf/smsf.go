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
	ErrNotClusterMember    = errors.New("smsf: node is not a cluster member")
	ErrSMSCUnavailable     = errors.New("smsf: SMSC not connected")
)

type AbsentError struct {
	Diagnostic uint32
}

func (e *AbsentError) Error() string {
	return fmt.Sprintf("smsf: UE absent (diagnostic %d)", e.Diagnostic)
}

type Transport interface {
	EnableUEReachability(ctx context.Context, imsi string) error
	SendSMS(ctx context.Context, imsi string, payload []byte) error
	SignallingSettled(ctx context.Context, imsi string)
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

	mu      sync.Mutex
	ues     map[string]*ueState
	waiting map[string]waiting
}

func New(store Store, directory Directory, node DiameterNode, transport Transport, logger *zap.Logger, timers Timers) *SMSF {
	return &SMSF{
		store:     store,
		directory: directory,
		diameter:  node,
		transport: transport,
		logger:    logger,
		timers:    timers,
		ues:       make(map[string]*ueState),
		waiting:   make(map[string]waiting),
	}
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
	allowed, err := s.AllowedEach(ctx, []string{imsi})

	return allowed[imsi], err
}

func (s *SMSF) AllowedEach(ctx context.Context, imsis []string) (map[string]bool, error) {
	settings, err := s.store.GetSMSSettings(ctx)
	if err != nil {
		return nil, fmt.Errorf("get SMS settings: %w", err)
	}

	allowed := make(map[string]bool, len(imsis))

	if !settings.Enabled() {
		return allowed, nil
	}

	for _, imsi := range imsis {
		ok, err := s.subscriberHasMSISDN(ctx, imsi)
		if err != nil {
			return nil, err
		}

		allowed[imsi] = ok
	}

	return allowed, nil
}

func (s *SMSF) subscriberHasMSISDN(ctx context.Context, imsi string) (bool, error) {
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
