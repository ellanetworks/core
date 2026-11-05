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
	"sync/atomic"
	"time"

	"github.com/ellanetworks/core/diameter"
	"github.com/ellanetworks/core/diameter/s6c"
	"github.com/ellanetworks/core/diameter/sgd"
	"github.com/ellanetworks/core/diameter/tgpp"
	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/diameternode"
	"go.opentelemetry.io/otel"
	"go.uber.org/zap"
)

var tracer = otel.Tracer("ella-core/smsf")

const PeerRoleSMSC = diameternode.PeerRoleSMSC

var smscApplications = []diameter.Application{
	{ID: sgd.ApplicationID, VendorID: tgpp.VendorID},
	{ID: s6c.ApplicationID, VendorID: tgpp.VendorID},
}

var (
	ErrNotRegisteredForSMS = errors.New("smsf: UE is not registered for SMS")
	ErrUnreachable         = errors.New("smsf: UE did not become reachable")
	ErrNotClusterMember    = errors.New("smsf: node is not a cluster member")
	ErrSMSCUnavailable     = errors.New("smsf: SMSC not connected")
	ErrUnknownSCAddress    = errors.New("smsf: no SMSC serves the service centre address")
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
	ListSMSCs(ctx context.Context) ([]db.SMSC, error)
	GetIPSMGWRegistration(ctx context.Context, imsi string) (*db.IPSMGWRegistration, error)
	PutIPSMGWRegistration(ctx context.Context, r db.IPSMGWRegistration) error
	DeleteIPSMGWRegistration(ctx context.Context, imsi string) error
}

type Directory interface {
	Identity(ctx context.Context, nodeID string) (diameternode.Identity, error)
}

type Peers interface {
	Envelope(peer string) (tgpp.Envelope, error)
	DoAny(ctx context.Context, peers []string, req *diameter.Message) (*diameter.Message, error)
	Relay(ctx context.Context, req *diameter.Message, from, host, realm string) (*diameter.Message, error)
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
	peers     Peers
	logger    *zap.Logger
	timers    Timers

	transport Transport

	mu       sync.Mutex
	ues      map[string]*ueState
	alerting map[string]bool

	rotation atomic.Uint32
}

func New(store Store, directory Directory, peers Peers, transport Transport, logger *zap.Logger, timers Timers) *SMSF {
	return &SMSF{
		store:     store,
		directory: directory,
		peers:     peers,
		transport: transport,
		logger:    logger,
		timers:    timers,
		ues:       make(map[string]*ueState),
		alerting:  make(map[string]bool),
	}
}

func SMSCPeerID(nodeID string) string {
	return PeerRoleSMSC + "-" + nodeID
}

func SMSCPeers(smsc db.SMSC) ([]diameternode.PeerConfig, error) {
	peers := make([]diameternode.PeerConfig, 0, len(smsc.Nodes))

	for _, n := range smsc.Nodes {
		addr, err := netip.ParseAddr(n.Address)
		if err != nil {
			return nil, fmt.Errorf("invalid address %q of SMSC %s: %w", n.Address, smsc.Name, err)
		}

		if n.Port < 1 || n.Port > 65535 {
			return nil, fmt.Errorf("invalid port %d of SMSC %s", n.Port, smsc.Name)
		}

		peers = append(peers, diameternode.PeerConfig{
			ID:           SMSCPeerID(n.ID),
			Role:         PeerRoleSMSC,
			Host:         n.DiameterIdentity,
			Address:      netip.AddrPortFrom(addr, uint16(n.Port)),
			Applications: smscApplications,
		})
	}

	return peers, nil
}

func (s *SMSF) smscServing(ctx context.Context, serviceCentre string) (*db.SMSC, error) {
	smscs, err := s.store.ListSMSCs(ctx)
	if err != nil {
		return nil, fmt.Errorf("list SMSCs: %w", err)
	}

	i := slices.IndexFunc(smscs, func(smsc db.SMSC) bool { return smsc.Serves(serviceCentre) })
	if i < 0 || len(smscs[i].Nodes) == 0 {
		return nil, ErrUnknownSCAddress
	}

	return &smscs[i], nil
}

func (s *SMSF) smscFor(ctx context.Context, serviceCentre string) ([]string, error) {
	smsc, err := s.smscServing(ctx, serviceCentre)
	if err != nil {
		return nil, err
	}

	nodes := smsc.Nodes
	start := int(s.rotation.Add(1)-1) % len(nodes)
	peers := make([]string, 0, len(nodes))

	for j := range nodes {
		peers = append(peers, SMSCPeerID(nodes[(start+j)%len(nodes)].ID))
	}

	return peers, nil
}

func (s *SMSF) envelope(peers []string) (tgpp.Envelope, error) {
	for _, peer := range peers {
		if envelope, err := s.peers.Envelope(peer); err == nil {
			return envelope, nil
		}
	}

	return tgpp.Envelope{}, ErrSMSCUnavailable
}

func (s *SMSF) Register(r Registrar) {
	r.Handle(s6c.ApplicationID, s6c.CommandSendRoutingInfoForSM, diameternode.HandlerFrom(s.SendRoutingInfoForSM))
	r.Handle(s6c.ApplicationID, s6c.CommandReportSMDeliveryStatus, diameternode.Handler(s.ReportSMDeliveryStatus))
	r.Handle(sgd.ApplicationID, sgd.CommandMTForwardShortMessage, diameternode.Handler(s.MTForwardShortMessage))
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

	if settings.SMSNumber == "" {
		return settings, false, nil
	}

	smscs, err := s.store.ListSMSCs(ctx)
	if err != nil {
		return nil, false, fmt.Errorf("list SMSCs: %w", err)
	}

	return settings, len(smscs) > 0, nil
}

func PeersOver(node DiameterNode) Peers {
	return nodePeers{node: node}
}

type nodePeers struct {
	node DiameterNode
}

func (c nodePeers) Envelope(peer string) (tgpp.Envelope, error) {
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

func (c nodePeers) DoAny(ctx context.Context, peers []string, req *diameter.Message) (*diameter.Message, error) {
	node := c.node.Node()
	if node == nil {
		return nil, ErrSMSCUnavailable
	}

	ans, err := node.DoAny(ctx, peers, req)
	if errors.Is(err, diameter.ErrNotConnected) {
		return nil, fmt.Errorf("%w: %w", ErrSMSCUnavailable, err)
	}

	return ans, err
}

func (c nodePeers) Relay(ctx context.Context, req *diameter.Message, from, host, realm string) (*diameter.Message, error) {
	node := c.node.Node()
	if node == nil {
		return nil, diameter.ErrUnableToDeliver
	}

	return node.Relay(ctx, req, from, host, realm, diameter.FailFast())
}

func (c nodePeers) LocalHost() (string, bool) {
	node := c.node.Node()
	if node == nil {
		return "", false
	}

	return node.Identity().OriginHost, true
}
