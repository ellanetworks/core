// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"context"
	"fmt"
	"net/netip"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/guard"
	lmfmodels "github.com/ellanetworks/core/internal/lmf/models"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/mme/procedure"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/nas/eps"
	"github.com/ellanetworks/core/nas/fgs"
	"github.com/ellanetworks/core/s1ap"
	"go.uber.org/zap"
)

// BeginKeyChainProc claims the {NH, NCC} key chain for a key-changing procedure of
// type t via the procedure registry, returning false if another is already
// advancing it. Nil-safe for bare test contexts.
func (ue *UeContext) BeginKeyChainProc(t procedure.Type) bool {
	if ue.procedures == nil {
		return true
	}

	return ue.procedures.Begin(t) == nil
}

// EndKeyChainProc releases a key-changing procedure claim. A no-op if t is not
// active, or on a bare test context.
func (ue *UeContext) EndKeyChainProc(t procedure.Type) {
	if ue.procedures != nil {
		ue.procedures.End(t)
	}
}

// SuperviseKeyChainProc arms the registry's supervision timeout on an already-begun
// key-chain procedure: cancel runs once at the deadline, and only while the procedure
// is still active. A no-op on a bare test context with no registry.
func (ue *UeContext) SuperviseKeyChainProc(t procedure.Type, deadline time.Time, cancel procedure.CancelFunc) {
	if ue.procedures != nil {
		_ = ue.procedures.Supervise(t, deadline, cancel)
	}
}

// clearKeyChainProc ends whichever key-changing procedure is active; they are
// mutually exclusive, so at most one is ended. For release paths that do not track
// which procedure holds the chain.
func (ue *UeContext) clearKeyChainProc() {
	ue.EndKeyChainProc(procedure.SecurityMode)
	ue.EndKeyChainProc(procedure.S1Handover)
	ue.EndKeyChainProc(procedure.PathSwitch)
}

// PdnConnection is one PDN connection: a default EPS bearer to an APN, its
// anchor-allocated addressing, and the flags serialising an in-flight
// reconfiguration (TS 24.301 §6.5). A UE holds one per active APN, keyed by EPS
// bearer identity.
type PdnConnection struct {
	SessionRef    string
	Ebi           uint8
	Apn           string
	PdnType       eps.PDNType
	UeIP          netip.Addr // IPv4 address (for IPv4 / IPv4v6)
	UeIPv6Prefix  netip.Addr // /64 prefix base (for IPv6 / IPv4v6)
	UeIPv6IID     [8]byte    // SLAAC interface identifier sent to the UE
	Dns           netip.Addr // data-network DNS server, advertised to the UE via PCO
	DnConfig      string     // fingerprint of the data-network config the bearer was set up with; a change triggers reactivation
	PDUSessionID  uint8
	Snssai        *models.Snssai
	Transferred   bool
	SessAmbrDLBps uint64
	SessAmbrULBps uint64
	Qci           uint8
	Arp           uint8
	EsmCause      eps.ESMCause // PDN-type downgrade cause (#50/#51), 0 when none
	SgwFTEID      models.FTEID // S-GW S1-U endpoint (anchor-assigned), sent to the eNB; Addr is the IPv4 N3
	SgwN3IPv6     netip.Addr   // S-GW S1-U IPv6 N3 endpoint, when the N3 has one
	EnbFTEID      models.FTEID // eNB S1-U endpoint, learned from the ICS Response

	// Deactivating is set while an EPS bearer deactivation (reactivation
	// requested) is in flight, so a duplicate reconcile does not re-send it.
	Deactivating bool
	// Disconnecting marks a deactivation triggered by a UE PDN disconnect: on the
	// DEACTIVATE ACCEPT only this PDN connection is released, leaving the UE
	// connected (TS 24.301 §6.5.2).
	Disconnecting bool

	Modifying            bool
	PendingDNConfig      string
	PendingSessAmbrDLBps uint64
	PendingSessAmbrULBps uint64
	PendingQCI           uint8
	PendingARP           uint8

	// guard supervises this bearer's outstanding ESM procedure (Modify/Deactivate,
	// T3486/T3495). It is per-bearer because a UE with several PDN connections can
	// have an ESM procedure outstanding on each at once; the guard invalidates a
	// callback whose firing races a stop, release or re-arm.
	guard guard.Guard
}

type ESMInfoWait struct {
	PTI        uint8
	Standalone *PendingPDNConnectivity
}

type PendingPDNConnectivity struct {
	PTI     uint8
	PDNType uint8
}

// UeContext is the MME's persistent per-UE EMM context: subscriber identity, the
// EPS NAS security context, and the bearer state.
type UeContext struct {
	active atomic.Pointer[UeConn]

	supi                     etsi.SUPI
	Imei                     etsi.IMEI // IMEI/IMEISV equipment identity (TS 24.301; 5G PEI, TS 23.501 §5.9.3)
	registrationArea         []models.Tai
	ueNetCap                 eps.UENetworkCapability
	msNetCap                 *eps.MSNetworkCapability
	DRXParameter             []byte
	RadioCapability          []byte // UE Radio Capability (S1AP UE Capability Info Indication), replayed in Initial Context Setup (TS 23.401)
	RadioCapabilityForPaging []byte
	RequestedPTI             nas.ProcedureTransactionIdentity
	kenbCount                uint32
	esmInfoWait              atomic.Pointer[ESMInfoWait]

	CombinedAttach bool // UE requested combined EPS/IMSI attach (TS 24.301)
	HashmmeInput   []byte

	lastSeen atomic.Int64

	Pdns                  map[uint8]*PdnConnection
	Ambr                  *models.Ambr // UE-AMBR (profile UE-AMBR), shared model; nil until set at attach
	RequestedPDNType      uint8        // UE-requested PDN type (1 IPv4 / 2 IPv6 / 3 IPv4v6)
	RequestedAPN          string       // UE-requested APN at attach ("" = use the default policy, TS 24.301 §6.5.1.3)
	RequestedPDUSessionID uint8
	RequestedType         eps.RequestType
	tmsi                  etsi.TMSI
	oldTmsi               etsi.TMSI
	Location              models.UserLocation

	mu sync.Mutex

	// EPS NAS security context (TS 33.401).
	kasme                  []byte
	knasEnc                [16]byte
	knasInt                [16]byte
	cipheringAlg           nas.CipheringAlgorithm
	integrityAlg           nas.IntegrityAlgorithm
	ulCount                nas.UplinkCounter
	dl                     *nas.DownlinkSender
	dlOnce                 sync.Once
	sc                     *nas.SecurityContext
	secured                bool
	eksi                   nas.KeySetIdentifier
	ue5GSecurityCapability *fgs.UESecurityCapability

	// X2-handover key chain (TS 33.401)
	nh  [32]byte
	ncc uint8

	procedures *procedure.Registry

	handover *handoverContext

	emmState EMMState

	idleMobilityFrom5GS bool
	idleMobilityTo5GS   bool

	allow5G bool

	localBearerDeactivation bool

	releasing bool

	regStep RegStep

	mobileReachableTimer guard.Guard
	implicitDetachTimer  guard.Guard
	idleGen              uint64

	pagingTimer guard.Guard

	lppaMu            sync.RWMutex
	lppaMessages      []LPPaMessage
	radioMu           sync.RWMutex
	radioMeasurements *lmfmodels.RadioMeasurements
}

// TouchLastSeen records the current time as the UE's most recent uplink NAS
// activity. Safe for concurrent use.
func (ue *UeContext) TouchLastSeen() {
	ue.lastSeen.Store(time.Now().UnixNano())
}

// lastSeenTime returns the UE's most recent uplink NAS activity, or the zero
// time if none has been recorded. Safe for concurrent use.
func (ue *UeContext) lastSeenTime() time.Time {
	ns := ue.lastSeen.Load()
	if ns == 0 {
		return time.Time{}
	}

	return time.Unix(0, ns)
}

// SetIMSI records the UE's IMSI under ue.mu (so a concurrent lookupUeByIMSI scan
// never reads it mid-write). It does not index the UE by IMSI or supersede a
// prior context — that waits until the attach is authenticated, in
// CommitUEIdentity — so an unauthenticated attach citing a victim's cleartext IMSI
// cannot tear down the victim's context. TS 24.301 §4.4.4.3 has the MME process the
// plain ATTACH REQUEST at all; §5.5.1.2.7 f) keeps the victim's EMM context unchanged
// until authentication proves the sender genuine.
func (m *MME) SetIMSI(ue *UeContext, imsi string) {
	supi, err := etsi.NewSUPIFromIMSI(imsi)
	if err != nil {
		logger.MmeLog.Warn("rejecting malformed IMSI", zap.String("imsi", imsi), zap.Error(err))
		return
	}

	ue.mu.Lock()
	ue.supi = supi
	ue.mu.Unlock()
}

// TS 24.301 §5.5.1.2.7 f
func (m *MME) CommitUEIdentity(ctx context.Context, ue *UeContext, _ AuthProof) error {
	ue.mu.Lock()
	supi := ue.supi
	ue.mu.Unlock()

	if !supi.IsIMSI() {
		return fmt.Errorf("imsi is empty")
	}

	m.mu.Lock()
	old, superseded := m.UEs[supi]
	superseded = superseded && old != ue

	if superseded {
		m.removeContextLocked(old)
	}

	m.UEs[supi] = ue
	m.mu.Unlock()

	// TS 24.301 §5.5.1.2.7 f): a genuine re-attach supersedes the old context and
	// its EPS bearer contexts are deleted. The anchor sessions are released outside
	// m.mu, since external calls cannot run under it.
	if superseded {
		logger.MmeLog.Info("CommitUEIdentity superseding prior UE context; releasing its EPS sessions",
			zap.String("imsi", supi.IMSI()))
		m.ReleaseAllSessions(ctx, old)
	}

	return nil
}

func (m *MME) ServesUeContext(ue *UeContext) bool {
	if ue == nil {
		return false
	}

	supi := ue.Supi()
	if !supi.IsIMSI() {
		return false
	}

	held, ok := m.LookupUeBySupi(supi)

	return ok && held == ue
}

// Connected reports whether the UE has an active UE-associated S1-connection
// (ECM-CONNECTED); an idle UE has none (TS 23.401).
func (ue *UeContext) Connected() bool {
	return ue.Conn() != nil
}

// MarkSecured records the IMEI (when reported) and flags the NAS security context established.
func (ue *UeContext) MarkSecured(imei etsi.IMEI) {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	if imei.IsSet() {
		ue.Imei = imei
	}

	ue.secured = true
}

// UESnapshot is a read-only, point-in-time copy of a UE's identity and NAS
// security view for the status API. It is safe to read without holding a lock.
type UESnapshot struct {
	Imei               string
	LastSeenAt         time.Time
	CipheringAlgorithm string // EPS NAS ciphering, e.g. "EEA2" (TS 33.401)
	IntegrityAlgorithm string // EPS NAS integrity, e.g. "EIA2"
}

func (ue *UeContext) Snapshot() UESnapshot {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	return UESnapshot{
		Imei:               ue.Imei.IMEI(),
		LastSeenAt:         ue.lastSeenTime(),
		CipheringAlgorithm: cipheringAlgName(ue.cipheringAlg),
		IntegrityAlgorithm: integrityAlgName(ue.integrityAlg),
	}
}

func (ue *UeContext) defaultPDNLocked() *PdnConnection {
	var lowest *PdnConnection

	for _, p := range ue.Pdns {
		if lowest == nil || p.Ebi < lowest.Ebi {
			lowest = p
		}
	}

	return lowest
}

// DefaultPDN returns the UE's default PDN connection under ue.mu, so a caller on
// another goroutine does not read the pdns map while it is mutated.
func (m *MME) DefaultPDN(ue *UeContext) *PdnConnection {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	return ue.defaultPDNLocked()
}

// allocateEBI returns the lowest free EPS bearer identity in [5,15] for a new
// PDN connection's default bearer, or 0 if all are in use (TS 24.301: EBI 0-4
// are reserved, 5-15 are assignable).
func (ue *UeContext) allocateEBI() uint8 {
	for ebi := DefaultERABID; ebi <= 15; ebi++ {
		if _, ok := ue.Pdns[ebi]; !ok {
			return ebi
		}
	}

	return 0
}

func (ue *UeContext) PdnForAPN(apn string) *PdnConnection {
	for _, p := range ue.Pdns {
		if p.Apn == apn {
			return p
		}
	}

	return nil
}

// EnsurePDN returns the PDN connection for the given EPS bearer identity,
// creating it if absent.
func (ue *UeContext) EnsurePDN(ebi uint8) *PdnConnection {
	if ue.Pdns == nil {
		ue.Pdns = make(map[uint8]*PdnConnection)
	}

	p, ok := ue.Pdns[ebi]
	if !ok {
		p = &PdnConnection{Ebi: ebi}
		ue.Pdns[ebi] = p
	}

	return p
}

// AddDefaultPDN reserves the default bearer's PDN connection (EBI 5) at attach,
// under the lock so the map is safe against the reconciler.
func (m *MME) AddDefaultPDN(ue *UeContext) *PdnConnection {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	return ue.EnsurePDN(DefaultERABID)
}

// fillBearerLocked populates a PDN connection's addressing/QoS from a created EPS
// bearer. The caller holds ue.mu, so a concurrent status snapshot or reconcile never
// observes a half-written bearer (TS 24.301 §6.4; the PDN state is ue.mu-guarded).
func fillBearerLocked(p *PdnConnection, qos *EpsQoS, bearer models.EPSBearer) {
	p.SessionRef = bearer.Ref
	p.Apn = qos.APN
	p.DnConfig = qos.DnFingerprint()
	p.SessAmbrDLBps = qos.SessAmbrDL.Bps()
	p.SessAmbrULBps = qos.SessAmbrUL.Bps()
	p.Qci = qos.QCI
	p.Arp = qos.ARP
	p.PdnType = bearer.PDNType
	p.UeIP = bearer.IPv4
	p.UeIPv6Prefix = bearer.IPv6Prefix
	p.UeIPv6IID = bearer.IPv6IID
	p.Dns = bearer.DNS
	p.EsmCause = bearer.ESMCause
	p.SgwFTEID = bearer.SGW
	p.SgwN3IPv6 = bearer.SGWN3IPv6
	p.PDUSessionID = bearer.PDUSessionID
	p.Snssai = bearer.Snssai
}

// InstallDefaultBearer publishes the UE-AMBR and the default PDN connection's
// addressing/QoS atomically under ue.mu, so a status read or reconcile never sees a
// half-populated bearer. It returns the negotiated PDN type, DNS, and ESM cause
// (captured under the lock) for the caller to log.
func (m *MME) InstallDefaultBearer(ue *UeContext, qos *EpsQoS, bearer models.EPSBearer) (pdnType uint8, dns string, esmCause eps.ESMCause) {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	// The UE-AMBR, not the per-APN Session-AMBR: this is what the S1 handover
	// signals to the target eNB, matching the ICS and E-RAB Setup paths. The
	// Session-AMBR is per PDN connection and lives on PdnConnection.
	ue.Ambr = &models.Ambr{Uplink: qos.AMBRUL, Downlink: qos.AMBRDL}

	p := ue.publishPDNLocked(DefaultERABID, qos, bearer)

	return uint8(p.PdnType), p.Dns.String(), p.EsmCause
}

func (ue *UeContext) UsesEPCO(p *PdnConnection) bool {
	return p != nil && p.Transferred && ue.UeNetCap().SupportsEPCO()
}

func (m *MME) FillBearer(ue *UeContext, p *PdnConnection, qos *EpsQoS, bearer models.EPSBearer) *PdnConnection {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	return ue.publishPDNLocked(p.Ebi, qos, bearer)
}

func (ue *UeContext) publishPDNLocked(ebi uint8, qos *EpsQoS, bearer models.EPSBearer) *PdnConnection {
	if ue.Pdns == nil {
		ue.Pdns = make(map[uint8]*PdnConnection)
	}

	p := &PdnConnection{
		Ebi:         ebi,
		Transferred: ue.RequestedType == eps.RequestTypeHandover,
	}

	fillBearerLocked(p, qos, bearer)

	ue.Pdns[ebi] = p

	return p
}

// AddPDN allocates the lowest free EPS bearer identity and reserves a PDN
// connection for it, returning nil when none is free (TS 24.301). Locked so the
// allocate-and-insert is atomic against the reconciler.
func (m *MME) AddPDN(ue *UeContext) *PdnConnection {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	ebi := ue.allocateEBI()
	if ebi == 0 {
		return nil
	}

	return ue.EnsurePDN(ebi)
}

// LookupPDN returns the UE's PDN connection for an EPS bearer identity under the
// lock (nil if absent), so a NAS handler does not read the map while the
// reconciler mutates it.
func (m *MME) LookupPDN(ue *UeContext, ebi uint8) *PdnConnection {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	return ue.Pdns[ebi]
}

// FindPDNByAPN returns the UE's PDN connection to the given APN under the lock.
func (m *MME) FindPDNByAPN(ue *UeContext, apn string) *PdnConnection {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	return ue.PdnForAPN(apn)
}

func (ue *UeContext) AwaitESMInformation(pti uint8, standalone *PendingPDNConnectivity) {
	ue.esmInfoWait.Store(&ESMInfoWait{PTI: pti, Standalone: standalone})
}

func (ue *UeContext) PendingESMInfo() *ESMInfoWait {
	return ue.esmInfoWait.Load()
}

func (ue *UeContext) TakeESMInfoWait() *ESMInfoWait {
	return ue.esmInfoWait.Swap(nil)
}

func (ue *UeContext) TakeESMInfoWaitFor(pti uint8) *ESMInfoWait {
	w := ue.esmInfoWait.Load()
	if w == nil || w.PTI != pti {
		return nil
	}

	if ue.esmInfoWait.CompareAndSwap(w, nil) {
		return w
	}

	return nil
}

// DropPDN removes a PDN connection from the UE without releasing a session, for
// rolling back a connection reserved but never established.
func (m *MME) DropPDN(ue *UeContext, ebi uint8) {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	delete(ue.Pdns, ebi)
}

// NewUeConn allocates an MME-UE-S1AP-ID and registers a bare UE-associated
// S1-connection — one carrying an Initial UE Message not yet bound to a UE
// context (TS 36.413). An unidentified peer holds at most a bare connection.
func (m *MME) NewUeConn(conn S1APWriter, enbUEID s1ap.ENBUES1APID) *UeConn {
	m.mu.Lock()
	defer m.mu.Unlock()

	id, ok := m.allocConnIDLocked()
	if !ok {
		return nil
	}

	c := &UeConn{m: m, ENBUES1APID: enbUEID, MMEUES1APID: s1ap.MMEUES1APID(id)}
	c.setConn(conn)
	c.Log = m.nodeLogLocked(conn).With(logger.MMEUeS1apID(uint32(c.MMEUES1APID)))
	m.conns[id] = c

	return c
}

// allocConnIDLocked reserves an MME-UE-S1AP-ID from the recycling allocator,
// which does not reuse a released identifier immediately (TS 36.413). The caller
// holds m.mu. It returns false only when every identifier in the S1AP range is
// concurrently in use — unreachable in practice — in which case the connection
// is refused, never colliding with a live one.
func (m *MME) allocConnIDLocked() (uint32, bool) {
	id, err := m.connIDs.Allocate()
	if err != nil {
		logger.MmeLog.Error("cannot allocate MME-UE-S1AP-ID: identifier space exhausted", zap.Error(err))
		return 0, false
	}

	return uint32(id), true
}

// releaseConnIDLocked drops a connection from the active table and returns its
// MME-UE-S1AP-ID to the allocator for later reuse. The caller holds m.mu.
func (m *MME) releaseConnIDLocked(id uint32) {
	// A connection leaving the registry can no longer receive a Release Complete,
	// so the guard would fire later and run ReleaseUEContextLocally on a context
	// nothing references, re-arming the mobile-reachable chain behind it. Guard
	// callbacks run without the guard's lock, so this cannot deadlock.
	if c, ok := m.conns[id]; ok {
		c.releaseGuard.Stop()
	}

	delete(m.conns, id)
	m.connIDs.FreeID(int64(id))
}

// NewUeContext creates a fresh persistent UE context, unattached to any connection.
// AttachUeConn binds it to a connection.
func NewUeContext() *UeContext {
	return &UeContext{
		// The reserved all-ones value is the "no valid M-TMSI" sentinel (TS 23.003
		// §2.4); 0 is a legal allocatable M-TMSI, so it must not double as "unset".
		tmsi:       etsi.InvalidTMSI,
		oldTmsi:    etsi.InvalidTMSI,
		procedures: procedure.NewRegistry(logger.MmeLog),
	}
}

// ReleaseBareConn drops a connection that never bound a UE context.
func (m *MME) ReleaseBareConn(c *UeConn) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if c.ue != nil {
		return
	}

	m.releaseConnIDLocked(uint32(c.MMEUES1APID))
}

// NewUe registers a bare connection and immediately binds a fresh UE context to it.
func (m *MME) NewUe(conn S1APWriter, enbUEID s1ap.ENBUES1APID) *UeContext {
	c := m.NewUeConn(conn, enbUEID)
	if c == nil {
		return nil
	}

	ue := NewUeContext()
	m.AttachUeConn(ue, c)

	return ue
}

// attachUeConnLocked binds the bare connection c to a held UE context — a returning
// UE resuming from ECM-IDLE (S-TMSI) or reusing a native GUTI onto the connection its
// Initial UE Message created — detaching any connection ue still holds (returned as
// superseded) and stopping its idle/paging supervision. The caller holds m.mu. Secure
// exchange is established by the subsequent decode, not here (the bare connection
// carries the message that establishes it).
func (m *MME) attachUeConnLocked(ue *UeContext, c *UeConn) (superseded *UeConn) {
	m.stopIdleTimersLocked(ue)
	m.stopPagingLocked(ue)

	// A superseding connection detaches the old one but keeps its MME-UE-S1AP-ID
	// reserved in m.conns: the eNB can reference it until it is released (TS 36.413 §8.3.3.1).
	superseded = m.detachConnLocked(ue)

	// If c was bound to a transient context — a fresh Attach context superseded by a
	// native-GUTI reuse — detach it there so that discarded context does not appear
	// connected on c.
	if prev := c.ue; prev != nil && prev != ue {
		prev.active.CompareAndSwap(c, nil)
	}

	ue.active.Store(c)
	c.ue = ue

	// Becoming connected is activity; refresh liveness at the bind point.
	ue.TouchLastSeen()

	return superseded
}

// AttachUeConn binds a bare connection to a held UE context under the registry lock,
// releasing any superseded connection.
func (m *MME) AttachUeConn(ue *UeContext, c *UeConn) {
	m.mu.Lock()
	superseded := m.attachUeConnLocked(ue, c)
	m.mu.Unlock()

	if superseded != nil {
		m.releaseSupersededConn(context.Background(), superseded)
	}

	m.clearPagingSuppression(context.Background(), ue)
}

// clearPagingSuppression runs outside the registry lock: the anchor must not be
// called while m.mu is held.
func (m *MME) clearPagingSuppression(ctx context.Context, ue *UeContext) {
	if m.Session == nil {
		return
	}

	imsi := ue.imsiOrEmpty()
	if imsi == "" {
		return
	}

	for _, p := range m.SnapshotPDNs(ue) {
		if err := m.Session.ClearEPSPagingSuppression(ctx, imsi, p.Ebi); err != nil {
			logger.MmeLog.Warn("failed to clear paging suppression on reconnect",
				zap.String("imsi", imsi), zap.Uint8("ebi", p.Ebi), zap.Error(err))
		}
	}
}

// detachConnLocked unbinds the UE's current S1-connection and stops its connection-scoped
// supervision, returning the connection (nil if none). The connection stays in m.conns;
// the caller frees or reserves its MME-UE-S1AP-ID. The caller holds m.mu.
func (m *MME) detachConnLocked(ue *UeContext) *UeConn {
	old := ue.Conn()
	if old == nil {
		return nil
	}

	// Abort any in-flight handover so its supervision does not outlive ue.active and
	// fire on a detached connection.
	m.clearHandoverLocked(ue)
	m.stopNASGuardLocked(ue)
	// The response can only arrive over this connection, so a surviving wait could
	// only be concluded by an unrelated one.
	old.esmInfoGuard.Stop()
	ue.esmInfoWait.Store(nil)
	// Detaching the connection ends any in-flight key-changing procedure on it
	// (e.g. a security mode whose Complete never arrived), so the {NH, NCC} chain
	// claim must not outlive it and block a later procedure (TS 33.401 §7.2.8).
	ue.clearKeyChainProc()

	old.ue = nil
	ue.active.Store(nil)

	return old
}

func (m *MME) freeUeConnLocked(ue *UeContext) {
	ue.releasing = false

	if old := m.detachConnLocked(ue); old != nil {
		m.releaseConnIDLocked(uint32(old.MMEUES1APID))
	}
}

// FreeUeConn releases the UE's S1-connection under m.mu, moving it to ECM-IDLE.
func (m *MME) FreeUeConn(ue *UeContext) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.freeUeConnLocked(ue)
}

// RemoveUe deletes a UE context entirely: its S1-connection, idle/paging
// supervision, and all registry indexes. Idempotent, absorbing the detach/RLF
// release race.
func (m *MME) RemoveUe(ue *UeContext) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.removeContextLocked(ue)
}

// removeContextLocked deletes the UE from every registry index and stops all its
// supervision. The caller holds m.mu.
func (m *MME) removeContextLocked(ue *UeContext) {
	m.stopIdleTimersLocked(ue)
	m.stopPagingLocked(ue)
	m.releaseMTMSIsLocked(ue)
	m.freeUeConnLocked(ue)
	m.endRelocationLocked(ue.supi, ue)

	if supi := ue.supi; supi.IsIMSI() && m.UEs[supi] == ue {
		delete(m.UEs, supi)
	}
}

// UeConnected reports whether the UE currently holds a UE-associated S1
// connection, read under the registry lock.
func (m *MME) UeConnected(ue *UeContext) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return ue.Connected()
}

// ConnectedUEs returns a snapshot of every UE with a bound S1 connection.
func (m *MME) ConnectedUEs() []*UeContext {
	m.mu.Lock()
	defer m.mu.Unlock()

	ues := make([]*UeContext, 0, len(m.conns))
	for _, c := range m.conns {
		if c.ue != nil {
			ues = append(ues, c.ue)
		}
	}

	return ues
}

// ReconcileReady reports whether a UE may receive bearer-reconciliation
// signalling: registered, S1-connected, and not mid-handover (an E-RAB Modify or
// Release would collide with handover bearer signalling, TS 36.413 §8.4.1.2).
//
// The returned connection must be carried rather than re-read: the reconciler
// runs off the dispatch goroutine and makes database calls between deciding and
// signalling, and a release in that window nils ue.Conn().
func (m *MME) ReconcileReady(ue *UeContext) (*UeConn, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	conn := ue.Conn()
	if ue.EMMState() != EMMRegistered || conn == nil || ue.handover != nil {
		return nil, false
	}

	return conn, true
}

// claimRelease atomically marks the UE's S1 connection as releasing, returning
// false when there is no connection or a release is already in progress (a NAS
// guard timeout and an eNB-initiated release can race for the same UE).
func (m *MME) claimRelease(ue *UeContext) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	if ue.releasing {
		return false
	}

	ue.releasing = true

	return true
}

// releaseContextLockedPart performs, under the registry lock, the registry side
// of a local release: a registered UE keeps its context and is moved to ECM-IDLE
// (its S1 connection freed), an unregistered one is removed. It returns whether
// the UE was registered, plus its IMSI and MME-UE-S1AP-ID for post-release logging.
func (m *MME) releaseContextLockedPart(ue *UeContext) (registered bool, imsi string, mmeUEID s1ap.MMEUES1APID) {
	m.mu.Lock()
	defer m.mu.Unlock()

	registered = ue.EMMState() == EMMRegistered
	imsi = ue.imsiOrEmpty()

	if ue.Conn() != nil {
		mmeUEID = ue.Conn().MMEUES1APID
	}

	if registered {
		m.freeUeConnLocked(ue)
	} else {
		m.removeContextLocked(ue)
	}

	return registered, imsi, mmeUEID
}

// ConnsOnConn returns every UE-associated connection on the given eNB association.
func (m *MME) ConnsOnConn(conn S1APWriter) []*UeConn {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var out []*UeConn

	for _, c := range m.conns {
		if c.Conn() == conn {
			out = append(out, c)
		}
	}

	return out
}

// ConnsForConnectionList resolves the UE-associated connections named by a
// part-of-interface reset list, scoped to the association the reset arrived on.
// Each item is matched by its MME-UE-S1AP-ID, else by its eNB-UE-S1AP-ID; an item
// naming no known connection is skipped (it is still echoed in the acknowledge).
func (m *MME) ConnsForConnectionList(conn S1APWriter, items []s1ap.UEAssociatedLogicalS1ConnectionItem) []*UeConn {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var out []*UeConn

	for _, it := range items {
		switch {
		case it.MMEUES1APID != nil:
			if c, ok := m.conns[uint32(*it.MMEUES1APID)]; ok && c.Conn() == conn {
				out = append(out, c)
			}
		case it.ENBUES1APID != nil:
			for _, c := range m.conns {
				if c.Conn() == conn && c.ENBUES1APID == *it.ENBUES1APID {
					out = append(out, c)
					break
				}
			}
		}
	}

	return out
}

// DropStaleUe removes any context bound to the same eNB association and
// ENB-UE-S1AP-ID, so a fresh Initial UE Message (e.g. a re-attach reusing the
// eNB UE id) does not leak the previous context.
func (m *MME) DropStaleUe(conn S1APWriter, enbUEID s1ap.ENBUES1APID) {
	m.mu.Lock()
	defer m.mu.Unlock()

	var stale []*UeContext

	for _, c := range m.conns {
		if c.ue != nil && c.ue.Conn() == c && c.Conn() == conn && c.ENBUES1APID == enbUEID {
			stale = append(stale, c.ue)
		}
	}

	for _, ue := range stale {
		m.removeContextLocked(ue)
	}
}

// S1Identity snapshots the UE's S1 association — the eNB connection and the
// MME/ENB-UE-S1AP-IDs — under m.mu, so an off-dispatch send reads a consistent
// identity even while the UE rebinds it on an ECM-IDLE resume or a path switch
// (TS 36.413). It returns a nil writer for an idle UE (no connection), which the
// caller treats as undeliverable. The caller does I/O with the snapshot, never
// under the lock.
func (m *MME) S1Identity(ue *UeContext) (S1APWriter, s1ap.MMEUES1APID, s1ap.ENBUES1APID) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if ue.Conn() == nil {
		return nil, 0, 0
	}

	return ue.Conn().Conn(), ue.Conn().MMEUES1APID, ue.Conn().ENBUES1APID
}

// LookupUe finds the UE context bound to a connection by its MME-UE-S1AP-ID. A
// bare connection (no UE context yet) reports not found.
func (m *MME) LookupUe(mmeUEID s1ap.MMEUES1APID) (*UeContext, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	c, ok := m.conns[uint32(mmeUEID)]
	if !ok || c.ue == nil {
		return nil, false
	}

	return c.ue, true
}

// LookupUeByMTMSI finds a UE context by the M-TMSI of its assigned GUTI,
// resolving an S-TMSI.
func (m *MME) LookupUeByMTMSI(mtmsi uint32) (*UeContext, bool) {
	tmsi, err := etsi.NewTMSI(mtmsi)
	if err != nil {
		return nil, false
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	ue, ok := m.uesByTmsi[tmsi]

	return ue, ok
}

func (m *MME) LookupUeBySupi(supi etsi.SUPI) (*UeContext, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	ue, ok := m.UEs[supi]

	return ue, ok
}

func (m *MME) LookupUeByIMSI(imsi string) (*UeContext, bool) {
	supi, err := etsi.NewSUPIFromIMSI(imsi)
	if err != nil {
		return nil, false
	}

	return m.LookupUeBySupi(supi)
}
