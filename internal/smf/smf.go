// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package smf

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/smf/procedure"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

var tracer = otel.Tracer("ella-core/smf/session")

var ErrDNNNotFound = errors.New("data network not found")
var ErrDNNNotInSlice = errors.New("data network not found in slice")
var ErrNoPolicyMatch = errors.New("no matching policy for slice and DNN")
var ErrSMContextNotFound = errors.New("sm context not found")
var ErrUENotReachable = errors.New("UE is in CM-IDLE state")

type SessionQuerier interface {
	GetSession(ref string) *SMContext
	SessionsByDNN(dnn string) []*SMContext
	SessionCount() int
}

type PCF interface {
	GetSessionPolicy(ctx context.Context, imsi string, snssai *models.Snssai, dnn string) (*Policy, error)
}

type DNNStore interface {
	AllocateIP(ctx context.Context, imsi string, sessionKeyID uint8) (netip.Addr, error)
	ReleaseIP(ctx context.Context, imsi string, sessionKeyID uint8) (netip.Addr, error)
	AllocateIPv6(ctx context.Context, imsi string, sessionKeyID uint8) (netip.Addr, error)
	ReleaseIPv6(ctx context.Context, imsi string, sessionKeyID uint8) (netip.Addr, error)
	ListFramedRoutes(ctx context.Context, imsi string) ([]netip.Prefix, error)
	GetStaticIP(ctx context.Context, imsi string, ipv6 bool) (netip.Addr, bool, error)
}

type SessionStore interface {
	ResolveDNN(ctx context.Context, dnn string) (DNNStore, error)
	IncrementDailyUsage(ctx context.Context, imsi string, uplinkBytes, downlinkBytes uint64) error
	InsertFlowReports(ctx context.Context, reports []*models.FlowReportRequest) error
}

type UPFClient interface {
	EstablishSession(ctx context.Context, req *models.EstablishRequest) (*models.EstablishResponse, error)
	ModifySession(ctx context.Context, req *models.ModifyRequest) error
	FlushUsage(ctx context.Context, seid uint64)
	DeleteSession(ctx context.Context, seid uint64) error
	SuppressDownlinkDataNotification(ctx context.Context, seid uint64)
	ClearDownlinkDataNotification(ctx context.Context, seid uint64)
	UpdateFilters(ctx context.Context, policyID string, direction models.Direction, rules []models.FilterRule) error
	RegisterIPv6Session(ctx context.Context, reg *models.IPv6SessionRegistration) error
	UnregisterIPv6Session(ctx context.Context, ulTEID uint32) error
}

type AMFCallback interface {
	TransferN1(ctx context.Context, supi etsi.SUPI, n1Msg []byte, pduSessionID uint8) error
	TransferN1N2(ctx context.Context, supi etsi.SUPI, pduSessionID uint8, snssai *models.Snssai, n1Msg, n2Msg []byte) error
	ModifyN1N2(ctx context.Context, supi etsi.SUPI, pduSessionID uint8, n1Msg, n2Msg []byte) error
	ReleaseSession(ctx context.Context, supi etsi.SUPI, pduSessionID uint8, n1Msg, n2Transfer []byte) error
	N2TransferOrPage(ctx context.Context, supi etsi.SUPI, pduSessionID uint8, snssai *models.Snssai, n2Msg []byte) error
	SessionTransferred(ctx context.Context, supi etsi.SUPI, pduSessionID uint8, ref string, n2Transfer []byte)
}

type MMECallback interface {
	Page(ctx context.Context, imsi string) error
	SessionTransferred(ctx context.Context, imsi string, ebi uint8, ref string)
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
	mu          sync.RWMutex
	pool        map[string]*SMContext // key: SMContext.Ref (unique per session instance)=
	byKey       map[string]*SMContext
	refSeq      uint64 // guarded by mu; unique-Ref suffix counter
	pcf         PCF
	store       SessionStore
	upf         UPFClient
	amf         AMFCallback
	mme         MMECallback // set after construction
	clock       func() time.Time
	seidCounter uint64        // atomic; local SEID allocation
	t3591       time.Duration // network-requested modification command retransmission
	t3592       time.Duration // network-requested release command retransmission
}

const maxSMProcedureRetransmissions = 4

type Option func(*SMF)

func WithClock(fn func() time.Time) Option { return func(s *SMF) { s.clock = fn } }

func WithT3591(d time.Duration) Option { return func(s *SMF) { s.t3591 = d } }

func WithT3592(d time.Duration) Option { return func(s *SMF) { s.t3592 = d } }

func New(pcf PCF, store SessionStore, upf UPFClient, amf AMFCallback, opts ...Option) *SMF {
	s := &SMF{
		pool:  make(map[string]*SMContext),
		byKey: make(map[string]*SMContext),
		pcf:   pcf,
		store: store,
		upf:   upf,
		amf:   amf,
		clock: time.Now,
		t3591: 16 * time.Second, // TS 24.501 table 10.3.2
		t3592: 16 * time.Second, // TS 24.501 table 10.3.2
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

// SetMME registers the 4G MME so the SMF can page idle EPS UEs.
func (s *SMF) SetMME(mme MMECallback) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.mme = mme
}

func (s *SMF) AllocateSEID() uint64 {
	return atomic.AddUint64(&s.seidCounter, 1)
}

func (s *SMF) NewSession(supi etsi.SUPI, access AccessType, id SessionIdentity, dnn string, snssai *models.Snssai) (*SMContext, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !id.valid() {
		return nil, fmt.Errorf("session identity %s names no session", id)
	}

	if id.PDUSessionID != 0 && s.byKey[canonicalName(supi, id.PDUSessionID)] != nil {
		if id.EBI == 0 {
			return nil, fmt.Errorf("PDU session identity %d is already in use", id.PDUSessionID)
		}

		logger.SmfLog.Warn("ignoring a PDU session identity a live session already holds",
			logger.SUPI(supi.String()), logger.PDUSessionID(id.PDUSessionID))

		id.PDUSessionID = 0
	}

	if id.EBI != 0 && s.byKey[canonicalName(supi, epsBearerKey(id.EBI))] != nil {
		return nil, fmt.Errorf("EPS bearer identity %d is already in use", id.EBI)
	}

	s.refSeq++

	ctx := &SMContext{
		SessionIdentity: id,
		procedures:      procedure.NewRegistry(logger.SmfLog),
		Supi:            supi,
		Access:          access,
		Dnn:             dnn,
		Snssai:          snssai,
		Ref:             fmt.Sprintf("%s#%d", canonicalName(supi, id.sessionKey()), s.refSeq),
	}

	s.pool[ctx.Ref] = ctx

	for _, key := range id.sessionKeys() {
		s.byKey[canonicalName(supi, key)] = ctx
	}

	return ctx, nil
}

// GetSession retrieves a session by its unique Ref.
func (s *SMF) GetSession(ref string) *SMContext {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.pool[ref]
}

func (s *SMF) currentSession(supi etsi.SUPI, sessionKey uint8) *SMContext {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.byKey[canonicalName(supi, sessionKey)]
}

func (s *SMF) currentPDUSession(supi etsi.SUPI, pduSessionID uint8) *SMContext {
	return s.currentSession(supi, SessionIdentity{PDUSessionID: pduSessionID}.sessionKey())
}

func (s *SMF) currentEPSSession(supi etsi.SUPI, ebi uint8) *SMContext {
	return s.currentSession(supi, SessionIdentity{EBI: ebi}.sessionKey())
}

func (s *SMF) dropFromPool(sc *SMContext) {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.pool, sc.Ref)

	for _, k := range sc.sessionKeys() {
		key := canonicalName(sc.Supi, k)
		if s.byKey[key] == sc {
			delete(s.byKey, key)
		}
	}
}

// GetSessionBySEID finds a session by its local PFCP SEID.
func (s *SMF) GetSessionBySEID(seid uint64) *SMContext {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, ctx := range s.pool {
		if ctx.PFCPContext != nil && ctx.PFCPContext.SEID == seid {
			return ctx
		}
	}

	return nil
}

// RemoveSession tears down a session's user plane, releases its addresses, and
// removes it from the pool. Caller holds the session's Mutex.
func (s *SMF) RemoveSession(ctx context.Context, ref string) {
	smCtx := s.GetSession(ref)
	if smCtx == nil {
		return
	}

	_ = s.releaseUserPlaneThenAddresses(ctx, smCtx)

	s.dropFromPool(smCtx)

	logger.SmfLog.Info("SM Context removed", zap.String("smContextRef", ref))
}

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
