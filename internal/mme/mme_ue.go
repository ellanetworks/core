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
	"github.com/ellanetworks/core/s1ap"
)

// nasWriter is the subset of the SCTP connection the MME uses to send S1AP to an
// eNB. *sctp.SCTPConn satisfies it; tests substitute a capturing implementation.
type nasWriter interface {
	WriteMsg(b []byte, info *sctp.SndRcvInfo) (int, error)
}

// pdnConnection is one PDN connection: a default EPS bearer to an APN, its
// negotiated addressing as allocated by the SMF+PGW-C anchor, and the flags that
// serialise an in-flight reconfiguration (TS 24.301 §6.5). A UE holds one per
// active APN, keyed by EPS bearer identity.
type pdnConnection struct {
	ebi          uint8
	apn          string
	pdnType      uint8      // negotiated PDN type
	ueIP         netip.Addr // IPv4 address (for IPv4 / IPv4v6)
	ueIPv6Prefix netip.Addr // /64 prefix base (for IPv6 / IPv4v6)
	ueIPv6IID    [8]byte    // SLAAC interface identifier sent to the UE
	dns          netip.Addr // data-network DNS server, advertised to the UE via PCO
	dnConfig     string     // fingerprint of the data-network config the bearer was set up with; a change triggers reactivation
	// sessAmbrDLBps/ULBps are the per-APN Session-AMBR (bits/s), and qci/arp the
	// E-RAB QoS (QCI, ARP priority), the bearer was set up with; a policy change
	// triggers an in-place Modify EPS Bearer Context (QoS also an E-RAB Modify).
	sessAmbrDLBps uint64
	sessAmbrULBps uint64
	qci           uint8
	arp           uint8
	esmCause      uint8        // PDN-type downgrade cause (#50/#51), 0 when none
	sgwFTEID      models.FTEID // S-GW S1-U endpoint (anchor-assigned), sent to the eNB; Addr is the IPv4 N3
	sgwN3IPv6     netip.Addr   // S-GW S1-U IPv6 N3 endpoint, when the N3 has one
	enbFTEID      models.FTEID // eNB S1-U endpoint, learned from the ICS Response

	// deactivating is set while an EPS bearer deactivation (reactivation
	// requested) is in flight, so a duplicate reconcile does not re-send it.
	deactivating bool
	// disconnecting marks a deactivation triggered by a UE PDN disconnect: on the
	// DEACTIVATE ACCEPT only this PDN connection is released, leaving the UE
	// connected, rather than the whole UE being re-attached (TS 24.301 §6.5.2).
	disconnecting bool
	// modifying is set while a bearer modification (in-place DNS and/or Session-AMBR
	// update) is in flight, so a duplicate reconcile does not re-send it. The
	// pending* values are committed once the UE accepts, so an aborted modification
	// leaves the stored config stale for the backstop to retry.
	modifying            bool
	pendingDNConfig      string
	pendingSessAmbrDLBps uint64
	pendingSessAmbrULBps uint64
	pendingQCI           uint8
	pendingARP           uint8
}

// s1Conn is a UE's transient state for one UE-associated logical S1-connection
// (TS 36.413): the S1AP identities, the eNB association, the connection and
// idle-mode supervision timers, and any in-flight handover. A fresh one is bound
// on each idle→active transition; the persistent UeContext it belongs to survives
// across them. Fields are guarded by MME.mu unless noted.
type s1Conn struct {
	ENBUES1APID s1ap.ENBUES1APID
	MMEUES1APID s1ap.MMEUES1APID
	conn        nasWriter

	// EPS Connection Management state (TS 23.401): ECM-CONNECTED on an active S1
	// connection, ECM-IDLE between them.
	ecmState ecmStateAtomic

	// tauReleaseOnComplete defers the S1 release of a no-active TAU until the
	// GUTI reallocation it carried is acknowledged.
	tauReleaseOnComplete bool
	// releasing gates the registry op during a UE Context Release.
	releasing bool

	// handover is the in-flight S1 handover for this UE (nil when none); the UE is
	// reachable on both the source and target associations until HANDOVER NOTIFY
	// moves conn/ENBUES1APID to the target (TS 36.413 §8.4). handoverGen invalidates
	// a guard-timer callback that fired just as the handover was cleared or
	// replaced.
	handover    *handoverContext
	handoverGen uint64

	// Idle-mode reachability supervision (TS 24.301), armed in ECM-IDLE
	// and cancelled on reconnect. idleGen invalidates a timer callback that fired
	// just as a reconnect or re-arm ran.
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

	// NAS common-procedure guard (TS 24.301: T3450/T3460/T3470). At most
	// one common procedure is outstanding at a time, so a single guard suffices;
	// nasGuardPDU is the downlink message retransmitted on expiry. nasGuardGen
	// invalidates a stale callback.
	nasGuardTimer *time.Timer
	nasGuardPDU   []byte
	nasGuardName  string
	nasGuardTries int
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
// EPS NAS security context, and the bearer state. The embedded *s1Conn carries
// the transient state of its current UE-associated S1-connection.
type UeContext struct {
	*s1Conn

	imsi            string
	imei            string // 15-digit IMEI from the UE's IMEISV (TS 24.301)
	ueNetCap        []byte // raw UE network capability (algorithm selection + replay)
	msNetCap        []byte // raw MS network capability value part; source of the replayed GERAN (GEA) capabilities (TS 24.301)
	radioCapability []byte // UE Radio Capability (S1AP UE Capability Info Indication), replayed in Initial Context Setup (TS 23.401)
	esmContainer    []byte // PDN Connectivity Request, kept for default-bearer activation
	authVector      *udm.EPSAV
	combinedAttach  bool // UE requested combined EPS/IMSI attach (TS 24.301)
	// hashmmeInput is the plain Attach Request to hash into the SECURITY MODE
	// COMMAND HashMME IE, set when the Attach arrived without integrity protection;
	// nil when the Attach verified (TS 24.301 §5.4.3.2).
	hashmmeInput []byte

	// lastSeen is the Unix-nanosecond time of the UE's most recent uplink NAS
	// activity, updated on the hot path and read concurrently by the status API.
	lastSeen atomic.Int64

	// PDN connections (default EPS bearers), each to one APN, keyed by EPS bearer
	// identity (TS 24.301 §6.5). defaultEBI is the EBI of the bearer established at
	// attach (0 = none yet); it is the linked bearer of the UE's first PDN.
	pdns       map[uint8]*pdnConnection
	defaultEBI uint8

	ambrUplink       string // UE-AMBR (profile UE-AMBR), raw "<n> <unit>" form
	ambrDownlink     string
	requestedPDNType uint8  // UE-requested PDN type (1 IPv4 / 2 IPv6 / 3 IPv4v6)
	requestedAPN     string // UE-requested APN at attach ("" = use the default policy, TS 24.301 §6.5.1.3)

	// mtmsi is the M-TMSI of the GUTI assigned at attach (0 = none); it indexes
	// the UE for S-TMSI-addressed procedures (Service Request, paging).
	mtmsi uint32
	// oldMTMSI is the M-TMSI being replaced during a GUTI reallocation at TAU
	// (0 = none): both stay resolvable, and the UE is paged with the old one,
	// until TRACKING AREA UPDATE COMPLETE commits the new GUTI (TS 24.301).
	oldMTMSI uint32

	// mu is the per-UE lock guarding this UE's data state — the EPS NAS security
	// context below (dlCount, knasEnc, knasInt, eea, eia, imei, secured), the PDN
	// modification state (the pdns map, defaultEBI, and each pdnConnection's
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

	// emmState is the EPS Mobility Management state (TS 23.401), independent of the
	// ECM state on s1Conn: an S1 release moves the UE to ECM-IDLE while leaving the
	// EMM state untouched, so the release-complete handler deletes the context only
	// if the UE is also EMM-DEREGISTERED (detach), else it is retained in ECM-IDLE.
	emmState emmStateAtomic
}

// touchLastSeen records the current time as the UE's most recent uplink NAS
// activity. Safe for concurrent use.
func (ue *UeContext) touchLastSeen() {
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

// setIMSI sets the IMSI under ue.mu, so a concurrent lookupUeByIMSI scan never reads it mid-write.
func (m *MME) setIMSI(ue *UeContext, imsi string) {
	ue.mu.Lock()
	ue.imsi = imsi
	ue.mu.Unlock()
}

// downlinkSecCtx reserves the next downlink NAS COUNT and returns the security context to protect with (TS 24.301).
func (ue *UeContext) downlinkSecCtx() (count uint32, knasInt, knasEnc [16]byte, eia, eea byte) {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	count = ue.dlCount
	ue.dlCount++

	return count, ue.knasInt, ue.knasEnc, ue.eia, ue.eea
}

// nextDownlinkCount reserves and returns the next downlink NAS COUNT (TS 24.301).
func (ue *UeContext) nextDownlinkCount() uint32 {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	count := ue.dlCount
	ue.dlCount++

	return count
}

// setEPSSecurityContext installs the negotiated NAS algorithms and derived keys (TS 33.401).
func (ue *UeContext) setEPSSecurityContext(eea, eia byte, knasEnc, knasInt [16]byte) {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	ue.eea, ue.eia = eea, eia
	ue.knasEnc, ue.knasInt = knasEnc, knasInt
}

// markSecured records the IMEI (when reported) and flags the NAS security context established.
func (ue *UeContext) markSecured(imei string) {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	if imei != "" {
		ue.imei = imei
	}

	ue.secured = true
}

// securitySnapshot returns the IMEI and selected NAS algorithms for the status API.
func (ue *UeContext) securitySnapshot() (imei string, eea, eia byte) {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	return ue.imei, ue.eea, ue.eia
}

// defaultPDNLocked returns the UE's default PDN connection (the bearer
// established at attach), or nil if no PDN is active. The caller holds ue.mu.
func (ue *UeContext) defaultPDNLocked() *pdnConnection {
	if ue.defaultEBI == 0 {
		return nil
	}

	return ue.pdns[ue.defaultEBI]
}

// defaultPDN returns the UE's default PDN connection under ue.mu, so a caller on
// another goroutine does not read the pdns map while it is mutated.
func (m *MME) defaultPDN(ue *UeContext) *pdnConnection {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	return ue.defaultPDNLocked()
}

// allocateEBI returns the lowest free EPS bearer identity in [5,15] for a new
// PDN connection's default bearer, or 0 if all are in use (TS 24.301: EBI 0-4
// are reserved, 5-15 are assignable).
func (ue *UeContext) allocateEBI() uint8 {
	for ebi := defaultERABID; ebi <= 15; ebi++ {
		if _, ok := ue.pdns[ebi]; !ok {
			return ebi
		}
	}

	return 0
}

// pdnForAPN returns the PDN connection to the given APN, or nil if the UE has no
// connection to it.
func (ue *UeContext) pdnForAPN(apn string) *pdnConnection {
	for _, p := range ue.pdns {
		if p.apn == apn {
			return p
		}
	}

	return nil
}

// ensurePDN returns the PDN connection for the given EPS bearer identity,
// creating it if absent.
func (ue *UeContext) ensurePDN(ebi uint8) *pdnConnection {
	if ue.pdns == nil {
		ue.pdns = make(map[uint8]*pdnConnection)
	}

	p, ok := ue.pdns[ebi]
	if !ok {
		p = &pdnConnection{ebi: ebi}
		ue.pdns[ebi] = p
	}

	return p
}

// addDefaultPDN reserves the default bearer's PDN connection (EBI 5) at attach,
// under the lock so the map is safe against the reconciler.
func (m *MME) addDefaultPDN(ue *UeContext) *pdnConnection {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	p := ue.ensurePDN(defaultERABID)
	ue.defaultEBI = defaultERABID

	return p
}

// addPDN allocates the lowest free EPS bearer identity and reserves a PDN
// connection for it, returning nil when none is free (TS 24.301). Locked so the
// allocate-and-insert is atomic against the reconciler.
func (m *MME) addPDN(ue *UeContext) *pdnConnection {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	ebi := ue.allocateEBI()
	if ebi == 0 {
		return nil
	}

	return ue.ensurePDN(ebi)
}

// lookupPDN returns the UE's PDN connection for an EPS bearer identity under the
// lock (nil if absent), so a NAS handler does not read the map while the
// reconciler mutates it.
func (m *MME) lookupPDN(ue *UeContext, ebi uint8) *pdnConnection {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	return ue.pdns[ebi]
}

// findPDNByAPN returns the UE's PDN connection to the given APN under the lock.
func (m *MME) findPDNByAPN(ue *UeContext, apn string) *pdnConnection {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	return ue.pdnForAPN(apn)
}

// dropPDN removes a PDN connection from the UE without releasing a session, for
// rolling back a connection reserved but never established.
func (m *MME) dropPDN(ue *UeContext, ebi uint8) {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	delete(ue.pdns, ebi)

	if ue.defaultEBI == ebi {
		ue.defaultEBI = 0
	}
}

// newUe allocates an MME-UE-S1AP-ID and registers a UE context for the eNB
// association.
func (m *MME) newUe(conn nasWriter, enbUEID s1ap.ENBUES1APID) *UeContext {
	m.mu.Lock()
	defer m.mu.Unlock()

	id := m.nextMMEUEID
	m.nextMMEUEID++

	ue := &UeContext{s1Conn: &s1Conn{ENBUES1APID: enbUEID, MMEUES1APID: s1ap.MMEUES1APID(id), conn: conn}}
	ue.ecmState.store(ECMConnected)
	m.ues[id] = ue

	return ue
}

// establishS1Connection binds an existing UE context to a new UE-associated
// logical S1-connection: it allocates a fresh MME-UE-S1AP-ID (the one from the
// released connection must not be reused, TS 36.413), re-keys the
// active-connection index, and records the new eNB association, for a UE
// returning from ECM-IDLE (Service Request, paging response, tracking area
// update with the active flag). ECM-CONNECTED is set by
// the caller once the procedure succeeds, so a rejected request leaves the UE in
// ECM-IDLE.
func (m *MME) establishS1Connection(ue *UeContext, conn nasWriter, enbUEID s1ap.ENBUES1APID) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// A NAS signalling connection is being established for the UE, so the prior
	// connection's idle, paging, and NAS-guard supervision are stopped before its
	// s1Conn is replaced, leaving no timer that could fire against the new one
	// (TS 24.301).
	m.stopIdleTimersLocked(ue)
	m.stopPagingLocked(ue)
	m.stopNASGuardLocked(ue)

	delete(m.ues, uint32(ue.MMEUES1APID))

	id := m.nextMMEUEID
	m.nextMMEUEID++

	ue.s1Conn = &s1Conn{MMEUES1APID: s1ap.MMEUES1APID(id), ENBUES1APID: enbUEID, conn: conn}
	m.ues[id] = ue
}

// removeUe deletes a UE context. It is idempotent (deleting an absent context
// is a no-op), which absorbs the detach/RLF release race.
func (m *MME) removeUe(mmeUEID s1ap.MMEUES1APID) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if ue, ok := m.ues[uint32(mmeUEID)]; ok {
		m.stopIdleTimersLocked(ue)
		m.stopNASGuardLocked(ue)
		m.stopPagingLocked(ue)
		m.releaseMTMSIsLocked(ue)
	}

	delete(m.ues, uint32(mmeUEID))
}

// dropStaleUe removes any context bound to the same eNB association and
// ENB-UE-S1AP-ID, so a fresh Initial UE Message (e.g. a re-attach reusing the
// eNB UE id) does not leak the previous context.
func (m *MME) dropStaleUe(conn nasWriter, enbUEID s1ap.ENBUES1APID) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for id, ue := range m.ues {
		if ue.conn == conn && ue.ENBUES1APID == enbUEID {
			m.stopIdleTimersLocked(ue)
			m.stopNASGuardLocked(ue)
			m.stopPagingLocked(ue)
			m.releaseMTMSIsLocked(ue)

			delete(m.ues, id)
		}
	}
}

// s1Identity snapshots the UE's S1 association — the eNB connection and the
// MME/ENB-UE-S1AP-IDs — under m.mu, so an off-dispatch send reads a consistent
// identity even while the UE rebinds it on an ECM-IDLE resume or a path switch
// (TS 36.413). The caller then does I/O with the snapshot, never under the lock.
func (m *MME) s1Identity(ue *UeContext) (nasWriter, s1ap.MMEUES1APID, s1ap.ENBUES1APID) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return ue.conn, ue.MMEUES1APID, ue.ENBUES1APID
}

// lookupUe finds a UE context by its MME-UE-S1AP-ID.
func (m *MME) lookupUe(mmeUEID s1ap.MMEUES1APID) (*UeContext, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	ue, ok := m.ues[uint32(mmeUEID)]

	return ue, ok
}

// lookupUeByMTMSI finds a UE context by the M-TMSI of its assigned GUTI, used to
// resolve an S-TMSI (e.g. in a Service Request or paging response).
func (m *MME) lookupUeByMTMSI(mtmsi uint32) (*UeContext, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	ue, ok := m.byMTMSI[mtmsi]

	return ue, ok
}

// lookupUeByIMSI finds the attached UE context for imsi by scanning the
// registry, used by the detach and paging paths that key on the subscriber.
func (m *MME) lookupUeByIMSI(imsi string) (*UeContext, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, ue := range m.ues {
		ue.mu.Lock()
		match := ue.imsi == imsi
		ue.mu.Unlock()

		if match {
			return ue, true
		}
	}

	return nil, false
}
