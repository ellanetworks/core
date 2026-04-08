// Copyright 2026 Ella Networks
// SPDX-License-Identifier: Apache-2.0

package smf

import (
	"context"
	"errors"
	"fmt"
	"math"
	"net"
	"net/netip"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/util/idgenerator"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

var tracer = otel.Tracer("ella-core/smf/session")

// ErrDNNNotFound indicates that the requested data network (DNN) does not exist.
var ErrDNNNotFound = errors.New("data network not found")

// SessionQuerier provides read-only access to active sessions.
// External packages (API, AMF export, metrics) use this interface
// instead of a package-level SMF singleton.
type SessionQuerier interface {
	GetSession(ref string) *SMContext
	SessionsByDNN(dnn string) []*SMContext
	SessionCount() int
}

// PCF abstracts the Policy Control Function (3GPP TS 23.503).
// In a full 3GPP deployment, this would be the Npcf_SMPolicyControl service.
// Here it is backed by the local database, but the interface keeps the
// SMF ↔ PCF boundary explicit.
type PCF interface {
	// GetSessionPolicy returns the PCC rules (QoS + traffic filters) and DNN
	// configuration for a subscriber identified by IMSI, in the given network
	// slice and DNN. This mirrors the 3GPP Npcf_SMPolicyControl_Create
	// service operation which returns all session policy data in one call.
	GetSessionPolicy(ctx context.Context, imsi string, snssai *models.Snssai, dnn string) (*Policy, error)
}

// SessionStore is the minimal DB surface the SMF needs for session-level
// data operations (IP management, usage accounting, flow reports).
type SessionStore interface {
	// AllocateIP assigns an IPv4 address from the given data network's pool.
	AllocateIP(ctx context.Context, imsi string, dnn string, pduSessionID uint8) (netip.Addr, error)

	// ReleaseIP frees the lease associated with a session.
	// Returns the released IPv4 address so the caller can withdraw the BGP route.
	ReleaseIP(ctx context.Context, imsi string, dnn string, pduSessionID uint8) (netip.Addr, error)

	// IncrementDailyUsage adds uplink/downlink byte counts to a subscriber's daily usage.
	IncrementDailyUsage(ctx context.Context, imsi string, uplinkBytes, downlinkBytes uint64) error

	// InsertFlowReports persists multiple flow measurement records in a single transaction.
	InsertFlowReports(ctx context.Context, reports []*models.FlowReportRequest) error
}

// UPFClient abstracts the session management interface toward the UPF.
type UPFClient interface {
	EstablishSession(ctx context.Context, req *models.EstablishRequest) (*models.EstablishResponse, error)
	ModifySession(ctx context.Context, req *models.ModifyRequest) error
	DeleteSession(ctx context.Context, remoteSEID uint64) error

	UpdateFilters(ctx context.Context, policyID int64, direction models.Direction, rules []models.FilterRule) error
}

// BGPAnnouncer is the interface used by the SMF to announce/withdraw subscriber routes.
type BGPAnnouncer interface {
	Announce(ip netip.Addr, owner string) error
	Withdraw(ip netip.Addr) error
	IsRunning() bool
	IsAdvertising() bool
}

// AMFCallback abstracts the SMF → AMF communication.
// This breaks the circular dependency between the SMF and AMF packages.
type AMFCallback interface {
	// TransferN1 delivers a NAS message to the UE.
	TransferN1(ctx context.Context, supi etsi.SUPI, n1Msg []byte, pduSessionID uint8) error

	// TransferN1N2 delivers a combined N1+N2 message.
	TransferN1N2(ctx context.Context, supi etsi.SUPI, pduSessionID uint8, snssai *models.Snssai, n1Msg, n2Msg []byte) error

	// N2TransferOrPage sends an N2 message to the radio, paging the UE if needed.
	N2TransferOrPage(ctx context.Context, supi etsi.SUPI, pduSessionID uint8, snssai *models.Snssai, n2Msg []byte) error
}

// ResolvedNetworkRule represents a network rule attached to a policy for PDI/SDF filtering.
type ResolvedNetworkRule struct {
	Description  string
	PolicyID     int64
	Direction    models.Direction
	RemotePrefix *string
	Protocol     int32
	PortLow      int32
	PortHigh     int32
	Action       string
	Precedence   int32
}

// Policy contains the QoS parameters, network rules, and DNN configuration
// the SMF needs for a session.
type Policy struct {
	PolicyID     int64 // DB primary key; populated by GetSessionPolicy
	Ambr         models.Ambr
	QosData      models.QosData
	NetworkRules []*ResolvedNetworkRule
	DNS          net.IP
	MTU          uint16
}

// SMF implements the Session Management Function.
type SMF struct {
	mu   sync.RWMutex
	pool map[string]*SMContext // key: canonicalName(SUPI, PDUSessionID)

	pcf   PCF
	store SessionStore
	upf   UPFClient
	amf   AMFCallback
	bgp   BGPAnnouncer
	clock func() time.Time

	seidCounter uint64 // atomic; local SEID allocation

	pdrIDs *idgenerator.IDGenerator
	farIDs *idgenerator.IDGenerator
	qerIDs *idgenerator.IDGenerator
	urrIDs *idgenerator.IDGenerator
}

// Option configures an SMF instance.
type Option func(*SMF)

// WithClock overrides the time source (useful for testing).
func WithClock(fn func() time.Time) Option { return func(s *SMF) { s.clock = fn } }

// WithBGP sets the BGP announcer for advertising subscriber routes.
func WithBGP(bgp BGPAnnouncer) Option { return func(s *SMF) { s.bgp = bgp } }

// New creates a new SMF.
func New(pcf PCF, store SessionStore, upf UPFClient, amf AMFCallback, opts ...Option) *SMF {
	s := &SMF{
		pool:   make(map[string]*SMContext),
		pcf:    pcf,
		store:  store,
		upf:    upf,
		amf:    amf,
		clock:  time.Now,
		pdrIDs: idgenerator.NewGenerator(1, math.MaxUint16),
		farIDs: idgenerator.NewGenerator(1, math.MaxUint32),
		qerIDs: idgenerator.NewGenerator(1, math.MaxUint32),
		urrIDs: idgenerator.NewGenerator(1, math.MaxUint32),
	}
	for _, o := range opts {
		o(s)
	}

	return s
}

// SetUPF sets the UPF client adapter. This allows late binding of the UPF adapter
// after the SMF instance and dispatcher have been initialized.
func (s *SMF) SetUPF(upf UPFClient) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.upf = upf
}

// AllocateLocalSEID returns the next available local SEID.
func (s *SMF) AllocateLocalSEID() uint64 {
	return atomic.AddUint64(&s.seidCounter, 1)
}

// NewSession creates a new SMContext, adds it to the pool, and returns it.
func (s *SMF) NewSession(supi etsi.SUPI, pduSessionID uint8, dnn string, snssai *models.Snssai) *SMContext {
	s.mu.Lock()
	defer s.mu.Unlock()

	ctx := &SMContext{
		PDUSessionID: pduSessionID,
		Supi:         supi,
		Dnn:          dnn,
		Snssai:       snssai,
	}

	ref := CanonicalName(supi, pduSessionID)
	s.pool[ref] = ctx

	return ctx
}

// GetSession retrieves a session by its canonical reference.
func (s *SMF) GetSession(ref string) *SMContext {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.pool[ref]
}

// GetSessionBySEID finds a session by its local PFCP SEID.
func (s *SMF) GetSessionBySEID(seid uint64) *SMContext {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, ctx := range s.pool {
		if ctx.PFCPContext != nil && ctx.PFCPContext.LocalSEID == seid {
			return ctx
		}
	}

	return nil
}

// RemoveSession removes a session from the pool and releases its IP address.
func (s *SMF) RemoveSession(ctx context.Context, ref string) {
	s.mu.Lock()

	smCtx, ok := s.pool[ref]
	if !ok {
		s.mu.Unlock()
		return
	}

	delete(s.pool, ref)
	s.mu.Unlock()

	if smCtx.PDUAddress != nil {
		released, err := s.store.ReleaseIP(ctx, smCtx.Supi.IMSI(), smCtx.Dnn, smCtx.PDUSessionID)
		if err != nil {
			logger.SmfLog.Error("release UE IP-Address failed", zap.Error(err), zap.String("smContextRef", ref))
		} else if released.IsValid() {
			s.withdrawRoute(released)
		}
	}

	logger.SmfLog.Info("SM Context removed", zap.String("smContextRef", ref))
}

// SessionsByDNN returns all active sessions for a specific DNN.
func (s *SMF) SessionsByDNN(dnn string) []*SMContext {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var out []*SMContext

	for _, ctx := range s.pool {
		if ctx.Dnn == dnn {
			out = append(out, ctx)
		}
	}

	return out
}

// SessionCount returns the number of active sessions.
func (s *SMF) SessionCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return len(s.pool)
}

// GetSessionPolicy retrieves the PCC rules from the PCF for a subscriber.
func (s *SMF) GetSessionPolicy(ctx context.Context, supi etsi.SUPI, snssai *models.Snssai, dnn string) (*Policy, error) {
	ctx, span := tracer.Start(ctx, "smf/get_session_policy",
		trace.WithAttributes(attribute.String("ue.supi", supi.String())),
	)
	defer span.End()

	return s.pcf.GetSessionPolicy(ctx, supi.IMSI(), snssai, dnn)
}

// --- PDR/FAR/QER/URR allocation (delegated to the ID generators) ---

// NewPDR allocates a new Packet Detection Rule with an associated FAR.
func (s *SMF) NewPDR() (*PDR, error) {
	pdrID, err := s.pdrIDs.Allocate()
	if err != nil {
		return nil, fmt.Errorf("could not allocate PDR ID: %v", err)
	}

	far, err := s.NewFAR()
	if err != nil {
		return nil, err
	}

	return &PDR{
		PDRID: uint16(pdrID),
		FAR:   far,
	}, nil
}

// NewFAR allocates a new Forwarding Action Rule (default: drop).
func (s *SMF) NewFAR() (*FAR, error) {
	farID, err := s.farIDs.Allocate()
	if err != nil {
		return nil, fmt.Errorf("could not allocate FAR ID: %v", err)
	}

	return &FAR{
		FARID:       uint32(farID),
		ApplyAction: models.ApplyAction{Drop: true},
	}, nil
}

// NewQER allocates a new QoS Enhancement Rule from policy data.
func (s *SMF) NewQER(policy *Policy) (*QER, error) {
	qerID, err := s.qerIDs.Allocate()
	if err != nil {
		return nil, fmt.Errorf("could not allocate QER ID: %v", err)
	}

	return &QER{
		QERID: uint32(qerID),
		QFI:   policy.QosData.QFI,
		GateStatus: &models.GateStatus{
			ULGate: models.GateOpen,
			DLGate: models.GateOpen,
		},
		MBR: &models.MBR{
			ULMBR: bitRateTokbps(policy.Ambr.Uplink),
			DLMBR: bitRateTokbps(policy.Ambr.Downlink),
		},
	}, nil
}

// NewURR allocates a new Usage Reporting Rule.
func (s *SMF) NewURR() (*URR, error) {
	urrID, err := s.urrIDs.Allocate()
	if err != nil {
		return nil, fmt.Errorf("could not allocate URR ID: %v", err)
	}

	return &URR{
		URRID: uint32(urrID),
	}, nil
}

// announceRoute advertises a /32 route for the given UE IP via BGP.
// It is a no-op if no BGP announcer is configured or it is not advertising
// (BGP not running, or NAT enabled).
func (s *SMF) announceRoute(ip netip.Addr, owner string) {
	if s.bgp == nil || !s.bgp.IsAdvertising() {
		return
	}

	if err := s.bgp.Announce(ip, owner); err != nil {
		logger.SmfLog.Warn("failed to announce BGP route", zap.String("ip", ip.String()), zap.Error(err))
	}
}

// withdrawRoute removes a /32 route for the given UE IP from BGP.
// It is a no-op if no BGP announcer is configured or it is not advertising
// (BGP not running, or NAT enabled).
func (s *SMF) withdrawRoute(ip netip.Addr) {
	if s.bgp == nil || !s.bgp.IsAdvertising() {
		return
	}

	if err := s.bgp.Withdraw(ip); err != nil {
		logger.SmfLog.Warn("failed to withdraw BGP route", zap.String("ip", ip.String()), zap.Error(err))
	}
}

// RemovePDR frees a PDR ID.
func (s *SMF) RemovePDR(pdr *PDR) {
	s.pdrIDs.FreeID(int64(pdr.PDRID))
}

// RemoveFAR frees a FAR ID.
func (s *SMF) RemoveFAR(far *FAR) {
	s.farIDs.FreeID(int64(far.FARID))
}

// RemoveQER frees a QER ID.
func (s *SMF) RemoveQER(qer *QER) {
	s.qerIDs.FreeID(int64(qer.QERID))
}

// RemoveURR frees a URR ID.
func (s *SMF) RemoveURR(urr *URR) {
	s.urrIDs.FreeID(int64(urr.URRID))
}
