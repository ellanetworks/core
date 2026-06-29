// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package smf

import (
	"context"
	"errors"
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

// ErrDNNNotInSlice indicates that the requested slice is served, but no policy
// provides the requested DNN within it.
var ErrDNNNotInSlice = errors.New("data network not found in slice")

// ErrNoPolicyMatch indicates that no policy matches the session's slice (SST/SD)
// and DNN.
var ErrNoPolicyMatch = errors.New("no matching policy for slice and DNN")

// ErrUENotReachable indicates that the UE is in CM-IDLE state and the requested
// signaling cannot be delivered over the radio. AMFCallback implementations
// must return this error (wrapping is fine) when the UE has no active RAN
// connection.
var ErrUENotReachable = errors.New("UE is in CM-IDLE state")

// SessionQuerier provides read-only access to active sessions for external
// packages (API, AMF export, metrics), avoiding a package-level SMF singleton.
type SessionQuerier interface {
	GetSession(ref string) *SMContext
	SessionsByDNN(dnn string) []*SMContext
	SessionCount() int
}

// PCF abstracts the Policy Control Function (3GPP TS 23.503), backed by the local
// database. The interface keeps the SMF ↔ PCF boundary explicit.
type PCF interface {
	// GetSessionPolicy returns the PCC rules (QoS + traffic filters) and DNN
	// configuration for a subscriber, mirroring the 3GPP
	// Npcf_SMPolicyControl_Create operation that returns all session policy in
	// one call.
	GetSessionPolicy(ctx context.Context, imsi string, snssai *models.Snssai, dnn string) (*Policy, error)
}

// SessionStore is the minimal DB surface the SMF needs for session-level
// data operations (IP management, usage accounting, flow reports).
type SessionStore interface {
	AllocateIP(ctx context.Context, imsi string, dnn string, pduSessionID uint8) (netip.Addr, error)

	// ReleaseIP frees the session's lease and returns the freed IPv4 address so
	// the caller can withdraw the BGP route.
	ReleaseIP(ctx context.Context, imsi string, dnn string, pduSessionID uint8) (netip.Addr, error)

	// AllocateIPv6 assigns a /64 prefix from the data network's IPv6 pool and
	// returns its base address (lower 64 bits = 0).
	AllocateIPv6(ctx context.Context, imsi string, dnn string, pduSessionID uint8) (netip.Addr, error)

	ReleaseIPv6(ctx context.Context, imsi string, dnn string, pduSessionID uint8) (netip.Addr, error)

	IncrementDailyUsage(ctx context.Context, imsi string, uplinkBytes, downlinkBytes uint64) error

	// InsertFlowReports persists flow measurement records in one transaction.
	InsertFlowReports(ctx context.Context, reports []*models.FlowReportRequest) error
}

// UPFClient abstracts the session management interface toward the UPF.
type UPFClient interface {
	EstablishSession(ctx context.Context, req *models.EstablishRequest) (*models.EstablishResponse, error)
	ModifySession(ctx context.Context, req *models.ModifyRequest) error
	// FlushUsage delivers a final URR usage report for the given SEID before
	// the session is deleted, preventing loss of bytes accounted since the
	// last periodic poll.
	FlushUsage(ctx context.Context, remoteSEID uint64)
	DeleteSession(ctx context.Context, remoteSEID uint64) error

	UpdateFilters(ctx context.Context, policyID string, direction models.Direction, rules []models.FilterRule) error

	// RegisterIPv6Session tells the UPF's RA responder about a new IPv6
	// session so it can respond to Router Solicitations with an RA
	// containing the delegated /64 prefix.
	RegisterIPv6Session(ctx context.Context, reg *models.IPv6SessionRegistration) error

	UnregisterIPv6Session(ctx context.Context, ulTEID uint32) error
}

// AMFCallback abstracts the SMF → AMF communication.
// This breaks the circular dependency between the SMF and AMF packages.
type AMFCallback interface {
	TransferN1(ctx context.Context, supi etsi.SUPI, n1Msg []byte, pduSessionID uint8) error

	// TransferN1N2 delivers a combined N1+N2 message for PDU Session Setup.
	TransferN1N2(ctx context.Context, supi etsi.SUPI, pduSessionID uint8, snssai *models.Snssai, n1Msg, n2Msg []byte) error

	// ModifyN1N2 delivers a PDU Session Modification Command (N1) to the UE.
	// When n2Msg is non-nil (AMBR/QoS change), the AMF uses the NGAP
	// PDUSessionResourceModifyRequest procedure (TS 38.413 §9.2.1.5).
	// When n2Msg is nil (e.g. DNS-only change via Extended PCO), the AMF
	// delivers the NAS message via Downlink NAS Transport (TS 38.413 §8.6.2)
	// since no gNB resource modification is needed.
	ModifyN1N2(ctx context.Context, supi etsi.SUPI, pduSessionID uint8, n1Msg, n2Msg []byte) error

	// ReleaseSession sends a network-initiated PDU Session Release to the UE/gNB.
	// N1 (NAS PDU Session Release Command) is delivered piggy-backed on the
	// NGAP PDUSessionResourceReleaseCommand (TS 38.413 §9.2.1.3).
	// n2Transfer is the PDUSessionResourceReleaseCommandTransfer IE.
	ReleaseSession(ctx context.Context, supi etsi.SUPI, pduSessionID uint8, n1Msg, n2Transfer []byte) error

	// N2TransferOrPage sends an N2 message to the radio, paging the UE if needed.
	N2TransferOrPage(ctx context.Context, supi etsi.SUPI, pduSessionID uint8, snssai *models.Snssai, n2Msg []byte) error
}

// MMECallback abstracts the SMF → MME communication for 4G paging. It breaks the
// circular dependency between the SMF and MME packages: the MME (a 4G EPS session
// anchor) registers itself so the SMF can page an idle UE when downlink data
// arrives.
type MMECallback interface {
	// Page triggers an S1AP Paging for the idle UE identified by IMSI so it
	// re-establishes the bearer (TS 23.401 §5.3.4.3).
	Page(ctx context.Context, imsi string) error
}

// ResolvedNetworkRule represents a network rule attached to a policy for PDI/SDF filtering.
type ResolvedNetworkRule struct {
	Description  string
	PolicyID     string
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
	PolicyID     string // DB primary key (UUID)
	Ambr         models.Ambr
	QosData      models.QosData
	NetworkRules []*ResolvedNetworkRule
	DNS          net.IP
	MTU          uint16
	IPv4Pool     string // IPv4 pool CIDR (may be empty if only IPv6 is configured)
	IPv6Pool     string // IPv6 prefix delegation pool CIDR (may be empty if only IPv4 is configured)
}

// SMF implements the Session Management Function.
type SMF struct {
	mu   sync.RWMutex
	pool map[string]*SMContext // key: canonicalName(SUPI, PDUSessionID)

	pcf   PCF
	store SessionStore
	upf   UPFClient
	amf   AMFCallback
	mme   MMECallback // set after construction (the MME is built after the SMF)
	clock func() time.Time

	seidCounter uint64 // atomic; local SEID allocation

	pdrIDs *idgenerator.IDGenerator
	farIDs *idgenerator.IDGenerator
	qerIDs *idgenerator.IDGenerator
	urrIDs *idgenerator.IDGenerator

	t3591 time.Duration // network-requested modification command retransmission
	t3592 time.Duration // network-requested release command retransmission
}

// maxSMProcedureRetransmissions is the number of command retransmissions before
// the SMF aborts a network-requested procedure: the command is resent on each of
// the first four T3591/T3592 expiries and the procedure is aborted on the fifth
// (TS 24.501 §6.3.2.5, §6.3.3).
const maxSMProcedureRetransmissions = 4

// Option configures an SMF instance.
type Option func(*SMF)

// WithClock overrides the time source (useful for testing).
func WithClock(fn func() time.Time) Option { return func(s *SMF) { s.clock = fn } }

// WithT3591 overrides the network-requested modification retransmission interval.
func WithT3591(d time.Duration) Option { return func(s *SMF) { s.t3591 = d } }

// WithT3592 overrides the network-requested release retransmission interval.
func WithT3592(d time.Duration) Option { return func(s *SMF) { s.t3592 = d } }

// New creates a new SMF.
func New(pcf PCF, store SessionStore, upf UPFClient, amf AMFCallback, opts ...Option) *SMF {
	s := &SMF{
		pool:   make(map[string]*SMContext),
		pcf:    pcf,
		store:  store,
		upf:    upf,
		amf:    amf,
		clock:  time.Now,
		t3591:  16 * time.Second, // TS 24.501 table 10.3.2
		t3592:  16 * time.Second, // TS 24.501 table 10.3.2
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

// SetUPF binds the UPF client after the SMF and dispatcher are initialized.
func (s *SMF) SetUPF(upf UPFClient) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.upf = upf
}

// SetMME registers the 4G MME so the SMF can page idle EPS UEs. The MME uses the
// SMF as its session anchor, so it binds itself here once both exist.
func (s *SMF) SetMME(mme MMECallback) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.mme = mme
}

// AllocateLocalSEID returns the next available local SEID.
func (s *SMF) AllocateLocalSEID() uint64 {
	return atomic.AddUint64(&s.seidCounter, 1)
}

// NewSession creates a new SMContext and adds it to the pool.
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

// RemoveSession removes a session from the pool and releases its IP address(es).
func (s *SMF) RemoveSession(ctx context.Context, ref string) {
	s.mu.Lock()

	smCtx, ok := s.pool[ref]
	if !ok {
		s.mu.Unlock()
		return
	}

	delete(s.pool, ref)
	s.mu.Unlock()

	if smCtx.PDUIPV4Address != nil {
		_, err := s.store.ReleaseIP(ctx, smCtx.Supi.IMSI(), smCtx.Dnn, smCtx.PDUSessionID)
		if err != nil {
			logger.SmfLog.Error("release UE IP-Address failed", zap.Error(err), zap.String("smContextRef", ref))
		}
	}

	if smCtx.PDUIPV6Prefix != nil {
		_, err := s.store.ReleaseIPv6(ctx, smCtx.Supi.IMSI(), smCtx.Dnn, smCtx.PDUSessionID)
		if err != nil {
			logger.SmfLog.Error("release UE IPv6-Address failed", zap.Error(err), zap.String("smContextRef", ref))
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

// SessionCountByRAT returns the active session counts split by access technology:
// 4G EPS sessions and 5G PDU sessions.
func (s *SMF) SessionCountByRAT() (fourG, fiveG int) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, ctx := range s.pool {
		if ctx.IsEPS() {
			fourG++
		} else {
			fiveG++
		}
	}

	return fourG, fiveG
}

// GetSessionPolicy retrieves the PCC rules from the PCF for a subscriber.
func (s *SMF) GetSessionPolicy(ctx context.Context, supi etsi.SUPI, snssai *models.Snssai, dnn string) (*Policy, error) {
	ctx, span := tracer.Start(ctx, "smf/get_session_policy",
		trace.WithAttributes(attribute.String("ue.supi", supi.String())),
	)
	defer span.End()

	return s.pcf.GetSessionPolicy(ctx, supi.IMSI(), snssai, dnn)
}
