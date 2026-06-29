// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"net/netip"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/sctp"
	"github.com/ellanetworks/core/internal/udm"
	"github.com/ellanetworks/core/internal/util/timer"
	"github.com/ellanetworks/core/s1ap"
)

// NasWriter is the subset of the SCTP connection the MME uses to send S1AP to an
// eNB. *sctp.SCTPConn satisfies it; tests substitute a capturing implementation.
type NasWriter interface {
	WriteMsg(b []byte, info *sctp.SndRcvInfo) (int, error)
}

// PdnConnection is one PDN connection: a default EPS bearer to an APN, its
// negotiated addressing as allocated by the SMF+PGW-C anchor, and the flags that
// serialise an in-flight reconfiguration (TS 24.301 §6.5). A UE holds one per
// active APN, keyed by EPS bearer identity.
type PdnConnection struct {
	Ebi          uint8
	Apn          string
	PdnType      uint8      // negotiated PDN type
	UeIP         netip.Addr // IPv4 address (for IPv4 / IPv4v6)
	UeIPv6Prefix netip.Addr // /64 prefix base (for IPv6 / IPv4v6)
	UeIPv6IID    [8]byte    // SLAAC interface identifier sent to the UE
	Dns          netip.Addr // data-network DNS server, advertised to the UE via PCO
	DnConfig     string     // fingerprint of the data-network config the bearer was set up with; a change triggers reactivation
	// SessAmbrDLBps/ULBps are the per-APN Session-AMBR (bits/s), and qci/arp the
	// E-RAB QoS (QCI, ARP priority), the bearer was set up with; a policy change
	// triggers an in-place Modify EPS Bearer Context (QoS also an E-RAB Modify).
	SessAmbrDLBps uint64
	SessAmbrULBps uint64
	Qci           uint8
	Arp           uint8
	EsmCause      uint8        // PDN-type downgrade cause (#50/#51), 0 when none
	SgwFTEID      models.FTEID // S-GW S1-U endpoint (anchor-assigned), sent to the eNB; Addr is the IPv4 N3
	SgwN3IPv6     netip.Addr   // S-GW S1-U IPv6 N3 endpoint, when the N3 has one
	EnbFTEID      models.FTEID // eNB S1-U endpoint, learned from the ICS Response

	// Deactivating is set while an EPS bearer deactivation (reactivation
	// requested) is in flight, so a duplicate reconcile does not re-send it.
	Deactivating bool
	// Disconnecting marks a deactivation triggered by a UE PDN disconnect: on the
	// DEACTIVATE ACCEPT only this PDN connection is released, leaving the UE
	// connected, rather than the whole UE being re-attached (TS 24.301 §6.5.2).
	Disconnecting bool
	// Modifying is set while a bearer modification (in-place DNS and/or Session-AMBR
	// update) is in flight, so a duplicate reconcile does not re-send it. The
	// pending* values are committed once the UE accepts, so an aborted modification
	// leaves the stored config stale for the backstop to retry.
	Modifying            bool
	PendingDNConfig      string
	PendingSessAmbrDLBps uint64
	PendingSessAmbrULBps uint64
	PendingQCI           uint8
	PendingARP           uint8
}

// S1Conn is a UE's transient state for one UE-associated logical S1-connection
// (TS 36.413): the S1AP identities, the eNB association, the connection-scoped
// NAS-guard supervision, and any in-flight handover. A fresh one is bound
// on each idle→active transition; the persistent UeContext it belongs to survives
// across them. Fields are guarded by MME.mu unless noted.
type S1Conn struct {
	ENBUES1APID s1ap.ENBUES1APID
	MMEUES1APID s1ap.MMEUES1APID
	conn        NasWriter

	// ue is the persistent UE context bound to this connection, nil until a UE
	// is identified (a bare connection carries an Initial UE Message not yet
	// attached to a context). Guarded by MME.mu.
	ue *UeContext

	// BearersUp is set once the eNB confirms the radio bearers (Initial Context
	// Setup Response): it distinguishes a fully-established connection from one a
	// UE is still resuming on (e.g. a TAU resume that has not re-established
	// bearers).
	BearersUp bool

	// secureExchangeEstablished records that secure exchange of NAS messages has
	// been established on this connection (a NAS message has been successfully
	// integrity-checked, or the connection was created by a verified resume).
	// Once set, TS 24.301 §4.4.4.3 requires discarding any further message that
	// is not integrity protected or fails the check. It is per-connection (the
	// spec scopes it "for the NAS signalling connection"), matching the 5G AMF's
	// ActiveNasConnection.secureExchangeEstablished.
	secureExchangeEstablished bool

	// TauReleaseOnComplete defers the S1 release of a no-active TAU until the
	// GUTI reallocation it carried is acknowledged.
	TauReleaseOnComplete bool
	// releasing gates the registry op during a UE Context Release.
	releasing bool

	// NAS common-procedure guard (TS 24.301: T3450/T3460/T3470). At most
	// one common procedure is outstanding at a time, so a single guard suffices.
	// nasGuardPDU is the downlink message retransmitted on expiry; the shared
	// timer counts the retransmissions. nasGuardGen invalidates a stale callback
	// (a timer that fired just before the connection was released or re-armed).
	nasGuardTimer *timer.Timer
	nasGuardPDU   []byte
	nasGuardName  string
	nasGuardGen   uint64
	// nasGuardOnAbort, when non-nil, makes the guard abort-only: on exhausting
	// its retransmissions it runs this finalizer and leaves the UE connected,
	// rather than releasing the context. Used for non-critical procedures whose
	// failure must not drop the UE — a bearer modification (TS 24.301 §6.4.2.5:
	// on T3486 expiry the bearer stays active) or a single-PDN deactivation
	// (§6.4.4.5: on T3495 expiry the bearer is deactivated locally).
	nasGuardOnAbort func()
}

// UeContext is the MME's persistent per-UE EMM context: subscriber identity, the
// EPS NAS security context, and the bearer state. s1 is the UE's current
// UE-associated S1-connection, nil while the UE is in ECM-IDLE.
type UeContext struct {
	S1 *S1Conn

	imsi            string
	Imei            string // 15-digit IMEI from the UE's IMEISV (TS 24.301)
	UeNetCap        []byte // raw UE network capability (algorithm selection + replay)
	MsNetCap        []byte // raw MS network capability value part; source of the replayed GERAN (GEA) capabilities (TS 24.301)
	RadioCapability []byte // UE Radio Capability (S1AP UE Capability Info Indication), replayed in Initial Context Setup (TS 23.401)
	EsmContainer    []byte // PDN Connectivity Request, kept for default-bearer activation
	AuthVector      *udm.EPSAV
	CombinedAttach  bool // UE requested combined EPS/IMSI attach (TS 24.301)
	// HashmmeInput is the plain Attach Request to hash into the SECURITY MODE
	// COMMAND HashMME IE, set when the Attach arrived without integrity protection;
	// nil when the Attach verified (TS 24.301 §5.4.3.2).
	HashmmeInput []byte

	// lastSeen is the Unix-nanosecond time of the UE's most recent uplink NAS
	// activity, updated on the hot path and read concurrently by the status API.
	lastSeen atomic.Int64

	// PDN connections (default EPS bearers), each to one APN, keyed by EPS bearer
	// identity (TS 24.301 §6.5). defaultEBI is the EBI of the bearer established at
	// attach (0 = none yet); it is the linked bearer of the UE's first PDN.
	Pdns       map[uint8]*PdnConnection
	DefaultEBI uint8

	AmbrUplink       string // UE-AMBR (profile UE-AMBR), raw "<n> <unit>" form
	AmbrDownlink     string
	RequestedPDNType uint8  // UE-requested PDN type (1 IPv4 / 2 IPv6 / 3 IPv4v6)
	RequestedAPN     string // UE-requested APN at attach ("" = use the default policy, TS 24.301 §6.5.1.3)

	// mtmsi is the M-TMSI of the GUTI assigned at attach (0 = none); it indexes
	// the UE for S-TMSI-addressed procedures (Service Request, paging).
	mtmsi uint32
	// oldMTMSI is the M-TMSI being replaced during a GUTI reallocation at TAU
	// (0 = none): both stay resolvable, and the UE is paged with the old one,
	// until TRACKING AREA UPDATE COMPLETE commits the new GUTI (TS 24.301).
	oldMTMSI uint32

	// mu is the per-UE lock guarding this UE's data state — the EPS NAS security
	// context below (dlCount, knasEnc, knasInt, eea, eia, imei, secured), the PDN
	// modification state (the pdns map, defaultEBI, and each PdnConnection's
	// in-flight flags), and imsi. See the MME concurrency model. resyncTried is
	// dispatch-confined; releasing is guarded by MME.mu (it gates a registry op).
	mu sync.Mutex

	// EPS NAS security context (TS 33.401).
	kasme       []byte
	knasEnc     [16]byte
	knasInt     [16]byte
	eea         byte
	eia         byte
	ulCount     uint32
	dlCount     uint32
	secured     bool
	resyncTried bool

	// X2-handover key chain (TS 33.401): nh is the Next Hop the next path
	// switch hands to the target eNB, ncc its chaining counter. Seeded at initial
	// context setup (NCC=1) and advanced on each Path Switch or S1 handover.
	nh  [32]byte
	ncc uint8

	// keyChainBusy is set while a Path Switch or S1 handover is advancing the
	// {NH, NCC} chain. Both procedures refuse to start while it is set, so they
	// cannot derive a fresh NH from the same base for two targets (TS 33.401
	// §7.2.8). Guarded by MME.mu.
	keyChainBusy bool

	// handover is the in-flight S1 handover (nil when none). It holds the source
	// and target connections, each a distinct s1Conn with its own MME-UE-S1AP-ID;
	// s1 stays the source until HANDOVER NOTIFY switches it to the target (TS 36.413
	// §8.4). handoverGen invalidates a guard-timer callback that fired just as the
	// handover was cleared or replaced. Guarded by MME.mu.
	handover    *handoverContext
	handoverGen uint64

	// emmState is the EPS Mobility Management state (TS 23.401), independent of the
	// ECM state on s1Conn: an S1 release moves the UE to ECM-IDLE while leaving the
	// EMM state untouched, so the release-complete handler deletes the context only
	// if the UE is also EMM-DEREGISTERED (detach), else it is retained in ECM-IDLE.
	emmState emmStateAtomic

	// Idle-mode supervision lives on the persistent context because it runs while
	// the UE has no S1-connection (TS 24.301), armed in ECM-IDLE and cancelled on
	// reconnect. idleGen invalidates a timer callback that fired just as a reconnect
	// or re-arm ran.
	mobileReachableTimer *time.Timer
	implicitDetachTimer  *time.Timer
	idleGen              uint64

	// Paging supervision (T3413, TS 24.301 §5.6.2): armed when the MME pages an
	// idle UE for buffered downlink data, retransmitted a bounded number of times,
	// and cancelled when the UE reconnects. pagingPDU is the S1AP Paging message
	// retransmitted on expiry; pagingGen invalidates a stale callback.
	pagingTimer *time.Timer
	pagingPDU   []byte
	pagingTries int
	pagingGen   uint64
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

// SetIMSI records the UE's IMSI (under ue.mu, so a concurrent lookupUeByIMSI
// scan never reads it mid-write) so authentication and subscriber-data lookups
// can use it. It deliberately does NOT index the UE by IMSI or supersede a prior
// context for the same subscriber — that happens only once the attach is
// authenticated, in commitUEIdentity. Deferring the supersede keeps an
// unauthenticated attach citing a victim's (cleartext) IMSI from tearing down
// the victim's context (TS 24.301 §4.4.4.3).
func (m *MME) SetIMSI(ue *UeContext, imsi string) {
	ue.mu.Lock()
	ue.imsi = imsi
	ue.mu.Unlock()
}

// CommitUEIdentity indexes the UE by IMSI and supersedes any prior context for
// the same subscriber (a re-attach), so a subscriber maps to exactly one UE
// context. It runs only after the attach is authenticated and accepted —
// mirroring the 5G AMF, which adds the UE to its pool only once security is
// established — so an unauthenticated attach cannot disturb a registered UE
// (TS 24.301 §4.4.4.3).
func (m *MME) CommitUEIdentity(ue *UeContext, _ AuthProof) {
	ue.mu.Lock()
	imsi := ue.imsi
	ue.mu.Unlock()

	if imsi == "" {
		return
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if old, ok := m.ues[imsi]; ok && old != ue {
		m.removeContextLocked(old)
	}

	m.ues[imsi] = ue
}

// Connected reports whether the UE has an active UE-associated S1-connection
// (ECM-CONNECTED); an idle UE has none (TS 23.401).
func (ue *UeContext) Connected() bool {
	return ue.S1 != nil
}

// MarkSecured records the IMEI (when reported) and flags the NAS security context established.
func (ue *UeContext) MarkSecured(imei string) {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	if imei != "" {
		ue.Imei = imei
	}

	ue.secured = true
}

// securitySnapshot returns the IMEI and selected NAS algorithms for the status API.
func (ue *UeContext) securitySnapshot() (imei string, eea, eia byte) {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	return ue.Imei, ue.eea, ue.eia
}

// defaultPDNLocked returns the UE's default PDN connection (the bearer
// established at attach), or nil if no PDN is active. The caller holds ue.mu.
func (ue *UeContext) defaultPDNLocked() *PdnConnection {
	if ue.DefaultEBI == 0 {
		return nil
	}

	return ue.Pdns[ue.DefaultEBI]
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

// PdnForAPN returns the PDN connection to the given APN, or nil if the UE has no
// connection to it.
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

	p := ue.EnsurePDN(DefaultERABID)
	ue.DefaultEBI = DefaultERABID

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

// DropPDN removes a PDN connection from the UE without releasing a session, for
// rolling back a connection reserved but never established.
func (m *MME) DropPDN(ue *UeContext, ebi uint8) {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	delete(ue.Pdns, ebi)

	if ue.DefaultEBI == ebi {
		ue.DefaultEBI = 0
	}
}

// NewConn allocates an MME-UE-S1AP-ID and registers a bare UE-associated
// S1-connection — one carrying an Initial UE Message not yet bound to a UE
// context (TS 36.413). An unidentified peer holds at most a bare connection.
func (m *MME) NewConn(conn NasWriter, enbUEID s1ap.ENBUES1APID) *S1Conn {
	m.mu.Lock()
	defer m.mu.Unlock()

	id := m.nextMMEUEID
	m.nextMMEUEID++

	c := &S1Conn{ENBUES1APID: enbUEID, MMEUES1APID: s1ap.MMEUES1APID(id), conn: conn}
	m.conns[id] = c

	return c
}

// BindConn attaches a fresh persistent UE context to a bare connection, once the
// connection's first NAS message warrants one (an Attach Request).
func (m *MME) BindConn(c *S1Conn) *UeContext {
	m.mu.Lock()
	defer m.mu.Unlock()

	ue := &UeContext{S1: c}
	c.ue = ue

	return ue
}

// ReleaseBareConn drops a connection that never bound a UE context.
func (m *MME) ReleaseBareConn(c *S1Conn) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if c.ue != nil {
		return
	}

	delete(m.conns, uint32(c.MMEUES1APID))
}

// NewUe registers a bare connection and immediately binds a UE context to it.
func (m *MME) NewUe(conn NasWriter, enbUEID s1ap.ENBUES1APID) *UeContext {
	return m.BindConn(m.NewConn(conn, enbUEID))
}

// EstablishS1Connection binds a UE returning from ECM-IDLE to a fresh
// UE-associated logical S1-connection, allocating a new MME-UE-S1AP-ID (the
// released one must not be reused, TS 36.413). Any prior connection and its
// idle/paging supervision are released first.
func (m *MME) EstablishS1Connection(ue *UeContext, conn NasWriter, enbUEID s1ap.ENBUES1APID) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.stopIdleTimersLocked(ue)
	m.stopPagingLocked(ue)
	m.freeS1ConnLocked(ue)

	id := m.nextMMEUEID
	m.nextMMEUEID++

	// A resume only reaches here after its message was integrity-verified against
	// the held context (service request short-MAC, TAU/initial-message unprotect),
	// so secure exchange is established on the new connection from the outset.
	c := &S1Conn{MMEUES1APID: s1ap.MMEUES1APID(id), ENBUES1APID: enbUEID, conn: conn, ue: ue, secureExchangeEstablished: true}
	ue.S1 = c
	m.conns[id] = c
}

// AdoptConn moves the connection an Initial UE Message created onto a held
// persistent context, superseding the transient context that first received it.
// A returning UE whose native GUTI resolves to a held EPS security context reuses
// that context whole — keys, algorithms, and NAS COUNTs continue in place — so its
// connection, not its security state, is what is rebound (TS 24.301 §4.4.3,
// §5.4.3.3). Any prior connection and idle supervision of the held context are
// released first.
func (m *MME) AdoptConn(existing, transient *UeContext) {
	m.mu.Lock()
	defer m.mu.Unlock()

	c := transient.S1
	transient.S1 = nil

	m.stopIdleTimersLocked(existing)
	m.stopPagingLocked(existing)
	m.freeS1ConnLocked(existing)

	existing.S1 = c
	c.ue = existing

	// The GUTI re-attach was integrity-verified against the held context before
	// adoption, so secure exchange is established on the adopted connection.
	c.secureExchangeEstablished = true
}

// freeS1ConnLocked releases the UE's current S1-connection (moving it to
// ECM-IDLE) and stops the connection-scoped NAS-guard supervision. The
// persistent context, its idle timers, and its registry indexes are left intact.
// The caller holds m.mu.
func (m *MME) freeS1ConnLocked(ue *UeContext) {
	if ue.S1 == nil {
		return
	}

	// Abort any in-flight handover so its guard timer does not outlive ue.s1 and
	// fire on a freed connection.
	m.clearHandoverLocked(ue)
	m.stopNASGuardLocked(ue)
	delete(m.conns, uint32(ue.S1.MMEUES1APID))
	ue.S1 = nil
}

// FreeS1Conn releases the UE's S1-connection under m.mu, moving it to ECM-IDLE.
func (m *MME) FreeS1Conn(ue *UeContext) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.freeS1ConnLocked(ue)
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
	m.freeS1ConnLocked(ue)

	if ue.imsi != "" && m.ues[ue.imsi] == ue {
		delete(m.ues, ue.imsi)
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
func (m *MME) ReconcileReady(ue *UeContext) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return ue.emmState.load() == EMMRegistered && ue.S1 != nil && ue.handover == nil
}

// claimRelease atomically marks the UE's S1 connection as releasing, returning
// false when there is no connection or a release is already in progress (a NAS
// guard timeout and an eNB-initiated release can race for the same UE).
func (m *MME) claimRelease(ue *UeContext) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	if ue.S1 == nil || ue.S1.releasing {
		return false
	}

	ue.S1.releasing = true

	return true
}

// releaseContextLockedPart performs, under the registry lock, the registry side
// of a local release: a registered UE keeps its context and is moved to ECM-IDLE
// (its S1 connection freed), an unregistered one is removed. It returns whether
// the UE was registered, plus its IMSI and MME-UE-S1AP-ID for post-release logging.
func (m *MME) releaseContextLockedPart(ue *UeContext) (registered bool, imsi string, mmeUEID s1ap.MMEUES1APID) {
	m.mu.Lock()
	defer m.mu.Unlock()

	registered = ue.emmState.load() == EMMRegistered
	imsi = ue.imsi

	if ue.S1 != nil {
		mmeUEID = ue.S1.MMEUES1APID
	}

	if registered {
		m.freeS1ConnLocked(ue)
	} else {
		m.removeContextLocked(ue)
	}

	return registered, imsi, mmeUEID
}

// ConnsOnConn returns every UE-associated connection on the given eNB association.
func (m *MME) ConnsOnConn(conn NasWriter) []*S1Conn {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var out []*S1Conn

	for _, c := range m.conns {
		if c.conn == conn {
			out = append(out, c)
		}
	}

	return out
}

// ConnsForConnectionList resolves the UE-associated connections named by a
// part-of-interface reset list, scoped to the association the reset arrived on.
// Each item is matched by its MME-UE-S1AP-ID, else by its eNB-UE-S1AP-ID; an item
// naming no known connection is skipped (it is still echoed in the acknowledge).
func (m *MME) ConnsForConnectionList(conn NasWriter, items []s1ap.UEAssociatedLogicalS1ConnectionItem) []*S1Conn {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var out []*S1Conn

	for _, it := range items {
		switch {
		case it.MMEUES1APID != nil:
			if c, ok := m.conns[uint32(*it.MMEUES1APID)]; ok && c.conn == conn {
				out = append(out, c)
			}
		case it.ENBUES1APID != nil:
			for _, c := range m.conns {
				if c.conn == conn && c.ENBUES1APID == *it.ENBUES1APID {
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
func (m *MME) DropStaleUe(conn NasWriter, enbUEID s1ap.ENBUES1APID) {
	m.mu.Lock()
	defer m.mu.Unlock()

	var stale []*UeContext

	for _, c := range m.conns {
		if c.ue != nil && c.ue.S1 == c && c.conn == conn && c.ENBUES1APID == enbUEID {
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
func (m *MME) S1Identity(ue *UeContext) (NasWriter, s1ap.MMEUES1APID, s1ap.ENBUES1APID) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if ue.S1 == nil {
		return nil, 0, 0
	}

	return ue.S1.conn, ue.S1.MMEUES1APID, ue.S1.ENBUES1APID
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

// LookupUeByMTMSI finds a UE context by the M-TMSI of its assigned GUTI, used to
// resolve an S-TMSI (e.g. in a Service Request or paging response).
func (m *MME) LookupUeByMTMSI(mtmsi uint32) (*UeContext, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	ue, ok := m.byMTMSI[mtmsi]

	return ue, ok
}

// LookupUeByIMSI finds the persistent UE context for imsi, used by the detach
// and paging paths that key on the subscriber. It resolves a UE in ECM-IDLE as
// well as a connected one.
func (m *MME) LookupUeByIMSI(imsi string) (*UeContext, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	ue, ok := m.ues[imsi]

	return ue, ok
}
