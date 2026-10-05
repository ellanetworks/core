// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package smsf

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
	"slices"
	"sync"
	"time"

	"github.com/ellanetworks/core/diameter"
	"github.com/ellanetworks/core/diameter/s6c"
	"github.com/ellanetworks/core/diameter/sgd"
	"github.com/ellanetworks/core/diameter/tgpp"
	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/diameternode"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

var tracer = otel.Tracer("ella-core/smsf")

const PeerRoleSMSC = "smsc"

var smscApplications = []diameter.Application{
	{ID: sgd.ApplicationID, VendorID: tgpp.VendorID},
	{ID: s6c.ApplicationID, VendorID: tgpp.VendorID},
}

var (
	ErrNotRegisteredForSMS = errors.New("smsf: UE is not registered for SMS")
	ErrUnreachable         = errors.New("smsf: UE did not become reachable")
	ErrNotClusterMember    = errors.New("smsf: node is not a cluster member")
	ErrSMSCUnavailable     = errors.New("smsf: SMSC not connected")
	ErrUnknownSCAddress    = errors.New("smsf: no SMSC peer serves the service centre address")
)

type Handler interface {
	AllowedEach(ctx context.Context, imsis []string) (map[string]bool, error)
	Uplink(ctx context.Context, imsi string, payload []byte)
	UEReachable(ctx context.Context, imsi string)
	TransactionPending(imsi string) bool
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
	IMSIsWithMSISDN(ctx context.Context, imsis []string) (map[string]bool, error)
	GetUERegistration(ctx context.Context, imsi, regType string) (*db.UERegistration, error)
	GetSMSWaiting(ctx context.Context, imsi string) (*db.SMSWaiting, error)
	RecordSMSWaiting(ctx context.Context, u db.SMSWaitingUpdate) error
	ClearSMSMemoryFull(ctx context.Context, imsi string) error
	RemoveSMSWaitingCentre(ctx context.Context, imsi, serviceCentre string) error
	DeleteSMSWaiting(ctx context.Context, imsi string) error
	ListSMSCPeers(ctx context.Context) ([]db.SMSCPeer, error)
}

type Directory interface {
	Identity(ctx context.Context, nodeID string) (diameternode.Identity, error)
}

type SMSC interface {
	Envelope(peer string) (tgpp.Envelope, error)
	Do(ctx context.Context, peer string, req *diameter.Message) (*diameter.Message, error)
	LocalHost() (string, bool)
}

type DiameterNode interface {
	Node() *diameter.Node
}

type Registrar interface {
	Handle(applicationID, commandCode uint32, h diameter.Handler)
}

type Timers struct {
	TC1                time.Duration
	Paging             time.Duration
	MoreMessages       time.Duration
	MaxRetransmissions int
	TR1N               time.Duration
	TR2N               time.Duration
	AlertTimeout       time.Duration
}

func DefaultTimers() Timers {
	return Timers{
		TC1:                6 * time.Second,
		Paging:             10 * time.Second,
		MoreMessages:       5 * time.Second,
		MaxRetransmissions: 2,
		TR1N:               25 * time.Second,
		TR2N:               25 * time.Second,
		AlertTimeout:       10 * time.Second,
	}
}

type SMSF struct {
	store     Store
	directory Directory
	smsc      SMSC
	logger    *zap.Logger
	timers    Timers

	transport Transport

	mu       sync.Mutex
	ues      map[string]*ueState
	alerting map[string]bool
}

func New(store Store, directory Directory, smsc SMSC, transport Transport, logger *zap.Logger, timers Timers) *SMSF {
	return &SMSF{
		store:     store,
		directory: directory,
		smsc:      smsc,
		transport: transport,
		logger:    logger,
		timers:    timers,
		ues:       make(map[string]*ueState),
		alerting:  make(map[string]bool),
	}
}

func SMSCPeerID(id string) string {
	return PeerRoleSMSC + "-" + id
}

func SMSCPeer(peer db.SMSCPeer) (diameternode.PeerConfig, error) {
	addr, err := netip.ParseAddr(peer.Address)
	if err != nil {
		return diameternode.PeerConfig{}, fmt.Errorf("invalid SMSC address %q: %w", peer.Address, err)
	}

	if peer.Port < 1 || peer.Port > 65535 {
		return diameternode.PeerConfig{}, fmt.Errorf("invalid SMSC port %d", peer.Port)
	}

	return diameternode.PeerConfig{
		ID:           SMSCPeerID(peer.ID),
		Role:         PeerRoleSMSC,
		Host:         peer.DiameterIdentity,
		Address:      netip.AddrPortFrom(addr, uint16(peer.Port)),
		Applications: smscApplications,
	}, nil
}

func (s *SMSF) smscFor(ctx context.Context, serviceCentre string) (string, error) {
	peers, err := s.store.ListSMSCPeers(ctx)
	if err != nil {
		return "", fmt.Errorf("list SMSC peers: %w", err)
	}

	i := slices.IndexFunc(peers, func(p db.SMSCPeer) bool { return p.Serves(serviceCentre) })
	if i < 0 {
		return "", ErrUnknownSCAddress
	}

	return SMSCPeerID(peers[i].ID), nil
}

func (s *SMSF) Register(r Registrar) {
	r.Handle(s6c.ApplicationID, s6c.CommandSendRoutingInfoForSM, traced("s6c/send-routing-info-for-sm", s.SendRoutingInfoForSM))
	r.Handle(s6c.ApplicationID, s6c.CommandReportSMDeliveryStatus, traced("s6c/report-sm-delivery-status", s.ReportSMDeliveryStatus))
	r.Handle(sgd.ApplicationID, sgd.CommandMTForwardShortMessage, traced("sgd/mt-forward-short-message", s.MTForwardShortMessage))
}

func traced(name string, h func(context.Context, diameter.Identity, *diameter.Message) *diameter.Message) diameter.Handler {
	return diameter.HandlerFunc(func(ctx context.Context, c *diameter.Conn, req *diameter.Message) *diameter.Message {
		ctx, span := tracer.Start(ctx, name,
			trace.WithSpanKind(trace.SpanKindServer),
			trace.WithAttributes(
				attribute.String("network.protocol.name", "diameter"),
				attribute.Int64("diameter.application_id", int64(req.ApplicationID)),
				attribute.Int64("diameter.command_code", int64(req.CommandCode)),
			),
		)
		defer span.End()

		ans := h(ctx, c.LocalIdentity(), req)

		if ans == nil {
			return nil
		}

		if r, err := tgpp.ParseResult(ans); err == nil {
			span.SetAttributes(attribute.Int64("diameter.result_code", int64(r.Code)))
		}

		return ans
	})
}

func (s *SMSF) AllowedEach(ctx context.Context, imsis []string) (map[string]bool, error) {
	_, ready, err := s.readySettings(ctx)
	if err != nil {
		return nil, err
	}

	if !ready {
		return map[string]bool{}, nil
	}

	allowed, err := s.store.IMSIsWithMSISDN(ctx, imsis)
	if err != nil {
		return nil, fmt.Errorf("get subscribers: %w", err)
	}

	return allowed, nil
}

func (s *SMSF) readySettings(ctx context.Context) (*db.SMSSettings, bool, error) {
	settings, err := s.store.GetSMSSettings(ctx)
	if err != nil {
		return nil, false, fmt.Errorf("get SMS settings: %w", err)
	}

	if !settings.Enabled || settings.SMSNumber == "" {
		return settings, false, nil
	}

	peers, err := s.store.ListSMSCPeers(ctx)
	if err != nil {
		return nil, false, fmt.Errorf("list SMSC peers: %w", err)
	}

	return settings, len(peers) > 0, nil
}

func SMSCOver(node DiameterNode) SMSC {
	return nodeSMSC{node: node}
}

type nodeSMSC struct {
	node DiameterNode
}

func (c nodeSMSC) Envelope(peer string) (tgpp.Envelope, error) {
	node := c.node.Node()
	if node == nil {
		return tgpp.Envelope{}, ErrSMSCUnavailable
	}

	p, ok := node.Peer(peer)
	if !ok || p.State != diameter.PeerOpen {
		return tgpp.Envelope{}, ErrSMSCUnavailable
	}

	return tgpp.Envelope{
		SessionID:        node.NewSessionID(),
		Origin:           node.Identity(),
		DestinationHost:  p.Host,
		DestinationRealm: p.Realm,
	}, nil
}

func (c nodeSMSC) Do(ctx context.Context, peer string, req *diameter.Message) (*diameter.Message, error) {
	node := c.node.Node()
	if node == nil {
		return nil, ErrSMSCUnavailable
	}

	ctx, span := tracer.Start(ctx, "diameter/request",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			attribute.String("network.protocol.name", "diameter"),
			attribute.String("diameter.peer", peer),
			attribute.Int64("diameter.application_id", int64(req.ApplicationID)),
			attribute.Int64("diameter.command_code", int64(req.CommandCode)),
		),
	)
	defer span.End()

	ans, err := node.Do(ctx, peer, req)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())

		return nil, err
	}

	if r, err := tgpp.ParseResult(ans); err == nil {
		span.SetAttributes(attribute.Int64("diameter.result_code", int64(r.Code)))
	}

	return ans, nil
}

func (c nodeSMSC) LocalHost() (string, bool) {
	node := c.node.Node()
	if node == nil {
		return "", false
	}

	return node.Identity().OriginHost, true
}
