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
// database.
type PCF interface {
	// GetSessionPolicy returns the PCC rules (QoS + traffic filters) and DNN
	// configuration for a subscriber in one call (3GPP Npcf_SMPolicyControl_Create).
	GetSessionPolicy(ctx context.Context, imsi string, snssai *models.Snssai, dnn string) (*Policy, error)
}

// DNNStore is the session-data surface bound to one data network resolved once
// via SessionStore.ResolveDNN. Leases are keyed by the converged session key,
// which is stable across an access change.
type DNNStore interface {
	AllocateIP(ctx context.Context, imsi string, sessionKeyID uint8) (netip.Addr, error)

	// ReleaseIP frees the session's lease and returns the freed IPv4 address so
	// the caller can withdraw the BGP route.
	ReleaseIP(ctx context.Context, imsi string, sessionKeyID uint8) (netip.Addr, error)

	// AllocateIPv6 assigns a /64 prefix from the data network's IPv6 pool and
	// returns its base address (lower 64 bits = 0).
	AllocateIPv6(ctx context.Context, imsi string, sessionKeyID uint8) (netip.Addr, error)

	ReleaseIPv6(ctx context.Context, imsi string, sessionKeyID uint8) (netip.Addr, error)

	ListFramedRoutes(ctx context.Context, imsi string) ([]netip.Prefix, error)

	// GetStaticIP returns the reserved static address for the family (ipv6
	// selects the IPv6 pool), and whether one exists.
	GetStaticIP(ctx context.Context, imsi string, ipv6 bool) (netip.Addr, bool, error)
}

// SessionStore is the minimal DB surface the SMF needs for session-level
// data operations (IP management, usage accounting, flow reports).
type SessionStore interface {
	ResolveDNN(ctx context.Context, dnn string) (DNNStore, error)

	IncrementDailyUsage(ctx context.Context, imsi string, uplinkBytes, downlinkBytes uint64) error

	// InsertFlowReports persists flow measurement records in one transaction.
	InsertFlowReports(ctx context.Context, reports []*models.FlowReportRequest) error
}

// UPFClient abstracts the session management interface toward the UPF.
type UPFClient interface {
	// Apply states a session's whole intended user plane. The first call for a
	// SEID creates the session and later ones converge it, so the SMF never has
	// to say which rules changed. It answers with the resources the UPF owns.
	Apply(ctx context.Context, desired *models.SessionState) (*models.SessionApplied, error)

	// Delete tears the session down, reporting the usage accounted since the
	// last poll before the counters go (TS 29.244 §5.2.2.4).
	Delete(ctx context.Context, seid uint64) error

	SuppressDownlinkDataNotification(ctx context.Context, seid uint64)
	ClearDownlinkDataNotification(ctx context.Context, seid uint64)

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
	// A non-nil n2Msg (AMBR/QoS change) is carried by NGAP
	// PDUSessionResourceModifyRequest (TS 38.413 §9.2.1.5); a nil n2Msg
	// (e.g. DNS-only change via Extended PCO) uses Downlink NAS Transport
	// (TS 38.413 §8.6.2).
	ModifyN1N2(ctx context.Context, supi etsi.SUPI, pduSessionID uint8, n1Msg, n2Msg []byte) error

	// ReleaseSession sends a network-initiated PDU Session Release to the UE/gNB.
	// N1 (NAS PDU Session Release Command) is delivered piggy-backed on the
	// NGAP PDUSessionResourceReleaseCommand (TS 38.413 §9.2.1.3).
	// n2Transfer is the PDUSessionResourceReleaseCommandTransfer IE.
	ReleaseSession(ctx context.Context, supi etsi.SUPI, pduSessionID uint8, n1Msg, n2Transfer []byte) error

	// N2TransferOrPage sends an N2 message to the radio, paging the UE if needed.
	N2TransferOrPage(ctx context.Context, supi etsi.SUPI, pduSessionID uint8, snssai *models.Snssai, n2Msg []byte) error

	// SessionTransferred reports a PDU session moved to EPS (TS 23.502 §4.11.2.2
	// step 14). The session and the UE address live on. n2Release is a
	// PDUSessionResourceReleaseCommandTransfer, carrying no N1 container.
	SessionTransferred(ctx context.Context, supi etsi.SUPI, pduSessionID uint8, ref string, n2Release []byte)

	// SessionReleased reports a PDU session whose anchor session the SMF
	// released, so the AMF drops the routing context naming it.
	SessionReleased(ctx context.Context, supi etsi.SUPI, pduSessionID uint8, ref string)
}

// MMECallback abstracts the SMF → MME communication for 4G paging, breaking the
// circular dependency between the SMF and MME packages.
type MMECallback interface {
	// Page triggers an S1AP Paging for the idle UE identified by IMSI so it
	// re-establishes the bearer (TS 23.401 §5.3.4.3).
	Page(ctx context.Context, imsi string) error

	// SessionTransferred reports a PDN connection moved to 5GS (TS 23.502
	// §4.11.2.3 step 10). The session and the UE address live on.
	SessionTransferred(ctx context.Context, imsi string, ebi uint8, ref string)

	// SessionReleased reports a PDN connection whose anchor session the SMF
	// released, so the MME drops the connection naming it.
	SessionReleased(ctx context.Context, imsi string, ebi uint8, ref string)
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
	pool map[string]*SMContext // key: SMContext.Ref (unique per session instance)
	// Indexed under every identity that names the session (TS 23.501 §5.17.2). A
	// superseded session keeps its pool entry but not its byKey entries.
	byKey map[string]*SMContext

	// bySEID resolves a session from a UPF report without walking the pool, so
	// the receive path does not wait on the lock of every unrelated session.
	bySEID map[uint64]*SMContext
	refSeq uint64 // guarded by mu; unique-Ref suffix counter

	pcf   PCF
	store SessionStore
	upf   UPFClient
	amf   AMFCallback
	mme   MMECallback // set after construction
	clock func() time.Time

	seidCounter uint64 // atomic; SEID allocation

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
		byKey:  make(map[string]*SMContext),
		bySEID: make(map[uint64]*SMContext),
		pcf:    pcf,
		store:  store,
		upf:    upf,
		amf:    amf,
		clock:  time.Now,
		t3591:  16 * time.Second, // TS 24.501 table 10.3.2
		t3592:  16 * time.Second, // TS 24.501 table 10.3.2
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

// NewSession creates a new SMContext with a unique Ref and adds it to the pool,
// making it current for every identity that names it. A prior session for the
// same slot keeps its Ref and pool entry until it is explicitly released.
//
// The keys are tested and claimed in one critical section: the session key is
// also the UE IP lease key, and two sessions holding one key are handed one
// address. A PDU session identity a live session already holds is dropped, and
// the PDN connection is then not transferable to 5GS (TS 23.502 §4.11.1.1
// NOTE 5); an identity the session cannot do without is an error.
func (s *SMF) NewSession(supi etsi.SUPI, access AccessType, id SessionIdentity, dnn string, snssai *models.Snssai) (*SMContext, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if id.PDUSessionID != 0 && s.byKey[canonicalName(supi, id.PDUSessionID)] != nil {
		if id.EBI == 0 {
			return nil, fmt.Errorf("PDU session identity %d is already in use", id.PDUSessionID)
		}

		logger.SmfLog.Warn("ignoring PDU session id from PCO already held by a live session",
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

// A release must target an instance by Ref: the current session for a slot may
// already be a newer one.
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

// epsBearerIdentityAvailable reports whether ebi names a default bearer this
// session may take, without claiming it: a claim held for a move the target
// access abandons strands the identity. Caller holds sc.mu.
func (s *SMF) epsBearerIdentityAvailable(sc *SMContext, ebi uint8) error {
	if !(SessionIdentity{EBI: ebi}).valid() {
		return fmt.Errorf("EPS bearer identity %d is not a default bearer's", ebi)
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	if held := s.byKey[canonicalName(sc.Supi, epsBearerKey(ebi))]; held != nil && held != sc {
		return fmt.Errorf("EPS bearer identity %d is already in use", ebi)
	}

	return nil
}

// Only the EPS half of the identity moves (TS 23.501 §5.17.2): the PDU session
// identity stays the primary key, leaving the Ref and the UE IP lease alone.
// Caller holds sc.mu.
func (s *SMF) setEPSBearerIdentity(sc *SMContext, ebi uint8) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.pool[sc.Ref] != sc {
		return fmt.Errorf("session %q is not in the pool", sc.Ref)
	}

	if ebi != 0 {
		if held := s.byKey[canonicalName(sc.Supi, epsBearerKey(ebi))]; held != nil && held != sc {
			return fmt.Errorf("EPS bearer identity %d is already in use", ebi)
		}
	}

	if sc.EBI != 0 {
		key := canonicalName(sc.Supi, epsBearerKey(sc.EBI))
		if s.byKey[key] == sc {
			delete(s.byKey, key)
		}
	}

	sc.EBI = ebi

	if ebi != 0 {
		s.byKey[canonicalName(sc.Supi, epsBearerKey(ebi))] = sc
	}

	return nil
}

// dropFromPool removes sc from the pool by its unique Ref, and from the secondary
// index only under the keys sc is still current for — so releasing a superseded
// session cannot evict the newer one that replaced it. Caller must not hold s.mu.
func (s *SMF) dropFromPool(sc *SMContext) {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.pool, sc.Ref)

	for seid, indexed := range s.bySEID {
		if indexed == sc {
			delete(s.bySEID, seid)
		}
	}

	for _, k := range sc.sessionKeys() {
		key := canonicalName(sc.Supi, k)
		if s.byKey[key] == sc {
			delete(s.byKey, key)
		}
	}
}

// AssignUPFSession gives a session its UPF SEID and indexes it under that
// SEID, so a report can be resolved without walking the pool.
func (s *SMF) AssignUPFSession(sc *SMContext, seid uint64) {
	sc.SetUPFSession(seid)

	s.mu.Lock()
	defer s.mu.Unlock()

	s.bySEID[seid] = sc
}

func (s *SMF) GetSessionBySEID(seid uint64) *SMContext {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.bySEID[seid]
}

// RemoveSession tears down a session's user plane, releases its addresses, and
// removes it from the pool. Caller holds the session's mu.
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
	// The lock order is session then registry, so the pool is snapshotted before
	// the accesses are read.
	s.mu.RLock()

	sessions := make([]*SMContext, 0, len(s.pool))
	for _, ctx := range s.pool {
		sessions = append(sessions, ctx)
	}

	s.mu.RUnlock()

	for _, ctx := range sessions {
		ctx.mu.Lock()
		isEPS := ctx.IsEPS()
		ctx.mu.Unlock()

		if isEPS {
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
