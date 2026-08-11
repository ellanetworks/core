// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-FileCopyrightText: 2022-present Intel Corporation
// SPDX-FileCopyrightText: 2021 Open Networking Foundation <info@opennetworking.org>
// Copyright 2019 free5GC.org
//
// Modified by Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package amf

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/amf/procedure"
	"github.com/ellanetworks/core/internal/fivegskeys"
	"github.com/ellanetworks/core/internal/guard"
	"github.com/ellanetworks/core/internal/interworking"
	lmfmodels "github.com/ellanetworks/core/internal/lmf/models"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/util/ueauth"
	"github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/nas/eps"
	"github.com/ellanetworks/core/nas/fgs"
	"github.com/ellanetworks/core/ngap"
	"go.uber.org/zap"
)

type SmContext struct {
	Ref                string
	Snssai             *models.Snssai
	Dnn                string
	PduSessionInactive bool
}

type UeContext struct {
	mu sync.Mutex

	state   StateType
	regStep RegStep

	PlmnID  models.PlmnID
	Suci    string
	supi    etsi.SUPI
	Imei    etsi.IMEI // PEI equipment identity, carrying the IMEI/IMEISV (TS 23.501 §5.9.3)
	tmsi    etsi.TMSI
	oldTmsi etsi.TMSI

	Location models.UserLocation
	Tai      models.Tai
	// Written on every UE-specific NGAP message; guarded by mu.
	lastSeen atomic.Int64 // Unix nanoseconds; use lastSeenTime()/TouchLastSeen()

	// handover is the in-flight N2 handover FSM (nil when none).
	// Guarded by AMF.mu (the registry lock), not ue.mu.
	handover *handoverContext

	smf SmfSbi

	// active is the UE's single connection object (UeConn = NAS N1 + RAN N2), nil in
	// CM-IDLE. Atomic so the hot-path read is lock-free; swapped on bind/release/handover.
	active atomic.Pointer[UeConn]

	// procedures is the key-chain mutual-exclusion registry (SecurityMode/N2Handover;
	// TS 33.501 §6.9). It lives on the persistent UeContext — not the connection — so
	// a claim survives the UeConn swap on N2 handover.
	procedures *procedure.Registry

	// NAS security context per TS 33.501.
	secured              bool
	ueSecurityCapability *fgs.UESecurityCapability // TS 24.501 §9.11.3.54, nil until an authenticated path installs one

	// TS 24.501 §5.5.1.2.4 and §5.5.1.3.4 require all received octets to be kept:
	// SECURITY MODE COMMAND replays the S1 element byte-for-byte. Guarded by ue.mu.
	gmmCapability         *fgs.GMMCapability
	s1UENetworkCapability []byte

	epsNASAlgorithmsOffered *interworking.EPSNASAlgorithms
	epsNASAlgorithms        *interworking.EPSNASAlgorithms
	epsSecurityCapability   *eps.UESecurityCapability

	ngKsi        models.NgKsi
	knasInt      [16]uint8
	knasEnc      [16]uint8
	kgnb         []uint8
	nh           [32]uint8 // AS key-chain Next Hop, 256 bits (TS 33.501)
	ncc          uint8
	ulCount      nas.UplinkCounter
	dlCount      nas.DownlinkCounter
	sc           *nas.SecurityContext
	cipheringAlg nas.CipheringAlgorithm
	integrityAlg nas.IntegrityAlgorithm
	kamf         []uint8
	abba         []uint8

	Ambr                     *models.Ambr
	AllowedNssai             []models.Snssai
	RegistrationArea         []models.Tai
	RadioCapability          []byte
	RadioCapabilityForPaging *models.UERadioCapabilityForPaging
	DRXParameter             fgs.DRXValue // 5GS DRX cycle (TS 24.501 §9.11.3.2A); the 4G MME's DRXParameter is the 2-octet IE (TS 24.301 §9.9.3.8)
	SmContextList            map[uint8]*SmContext
	epsBearerIdentities      map[uint8]uint8

	allow4G bool

	mobileReachableTimer        guard.Guard
	implicitDeregistrationTimer guard.Guard
	idleGen                     uint64

	radioMu           sync.RWMutex
	radioMeasurements *lmfmodels.RadioMeasurements

	nrppaMu       sync.RWMutex
	nrppaMessages []NRPPaMessage

	nrppaRoutingIDs map[int64]struct{}

	pagingTimer guard.Guard

	n1n2Message atomic.Pointer[models.N1N2MessageTransferRequest]
}

func NewUeContext() *UeContext {
	ue := &UeContext{
		state:            Deregistered,
		SmContextList:    make(map[uint8]*SmContext),
		RegistrationArea: make([]models.Tai, 0),
		procedures:       procedure.NewRegistry(logger.AmfLog),
		tmsi:             etsi.InvalidTMSI,
		oldTmsi:          etsi.InvalidTMSI,
	}

	return ue
}

// Tmsi returns the UE's current 5G-TMSI (InvalidTMSI until a GUTI is allocated).
// Writers hold AMF.mu and ue.mu; callers outside the registry lock read here.
func (ue *UeContext) Tmsi() etsi.TMSI {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	return ue.tmsi
}

// OldTmsi returns the in-flight previous 5G-TMSI during a reallocation window.
func (ue *UeContext) OldTmsi() etsi.TMSI {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	return ue.oldTmsi
}

// Procedures returns the UE's key-chain mutual-exclusion registry.
func (ue *UeContext) Procedures() *procedure.Registry {
	return ue.procedures
}

// BeginKeyChainProc claims a key-changing procedure via the registry, returning false if
// a conflicting one is active. Nil-safe for bare test contexts.
func (ue *UeContext) BeginKeyChainProc(t procedure.Type) bool {
	if ue.procedures == nil {
		return true
	}

	return ue.procedures.Begin(t) == nil
}

// EndKeyChainProc releases a key-changing procedure claim. A no-op if t is not active,
// or on a bare test context.
func (ue *UeContext) EndKeyChainProc(t procedure.Type) {
	if ue.procedures != nil {
		ue.procedures.End(t)
	}
}

// endKeyChainProcs ends whichever key-changing procedure is active, aborting it on
// lower-layer failure (TS 24.501 §5.3.1.2: NAS procedures are aborted when the N1 NAS
// signalling connection is released). They are mutually exclusive, so at most one is
// ended. For release paths that do not track which procedure holds the chain.
func (ue *UeContext) endKeyChainProcs() {
	ue.EndKeyChainProc(procedure.SecurityMode)
	ue.EndKeyChainProc(procedure.N2Handover)
	ue.EndKeyChainProc(procedure.PathSwitch)
}

// SuperviseKeyChainProc arms the registry's supervision timeout on an already-begun
// key-chain procedure; cancel runs once at the deadline while the procedure is still
// active. A no-op on a bare test context.
func (ue *UeContext) SuperviseKeyChainProc(t procedure.Type, deadline time.Time, cancel func(context.Context) error) {
	if ue.procedures != nil {
		_ = ue.procedures.Supervise(t, deadline, cancel)
	}
}

// Conn returns the UE's currently attached connection (NAS N1 + RAN N2 in one
// object), or nil when the UE is in CM-IDLE. Callers must capture the result in a
// local and reuse it — never call Conn() twice in the same code path, as the
// underlying pointer may change between calls.
func (ue *UeContext) Conn() *UeConn {
	if ue == nil {
		return nil
	}

	return ue.active.Load()
}

// N1N2Message returns the SMF-pushed N1N2 message buffered for an idle UE, or nil.
// Capture the result in a local and reuse it — it may be cleared concurrently.
func (ue *UeContext) N1N2Message() *models.N1N2MessageTransferRequest {
	return ue.n1n2Message.Load()
}

// SetN1N2Message buffers an SMF-pushed N1N2 message for delivery when the idle UE reconnects.
func (ue *UeContext) SetN1N2Message(m *models.N1N2MessageTransferRequest) {
	ue.n1n2Message.Store(m)
}

// ClearN1N2Message discards the buffered N1N2 message once delivered or no longer deliverable.
func (ue *UeContext) ClearN1N2Message() {
	ue.n1n2Message.Store(nil)
}

// attachUeConnLocked binds ueConn as ue's active connection, defuses idle-mode
// supervision, and returns the displaced previous connection (nil if none) so the caller
// can release it outside the registry lock. Caller holds amf.mu: the whole connection
// lifecycle (bind, detach, ue.active) is serialized on it, so bind and release cannot
// race; ue.mu guards only per-UE security/session data.
func (a *AMF) attachUeConnLocked(ue *UeContext, ueConn *UeConn) *UeConn {
	oldUeConn := ue.active.Load()

	ueConn.ue.Store(ue)

	var displaced *UeConn

	if oldUeConn != nil && oldUeConn != ueConn {
		if oldUeConn.ue.Load() == ue {
			oldUeConn.Log.Info("Detached UeContext from previous UeConn")
			oldUeConn.ue.Store(nil)
			displaced = oldUeConn
		}
	}

	ue.lastSeen.Store(time.Now().UnixNano())

	ue.active.Store(ueConn)

	// The idle-mode supervision timers live under the registry lock held here
	// (TS 24.501 §5.3.7).
	a.stopIdleTimersLocked(ue)

	return displaced
}

// AttachUeConn binds ueConn to ue as the UE's active connection under the registry lock.
func (a *AMF) AttachUeConn(ue *UeContext, ueConn *UeConn) {
	if ueConn == nil {
		return
	}

	a.mu.Lock()
	displaced := a.attachUeConnLocked(ue, ueConn)
	a.mu.Unlock()

	// Release the displaced connection toward the gNB (TS 23.502 §4.2.6, TS 38.413
	// §8.3.3.1). It stays registered with its AMF-UE-NGAP-ID reserved until the Release
	// Complete reaps it, so the gNB can reference it until then.
	if displaced != nil {
		displaced.SendUEContextReleaseCommand(context.Background(),
			ngap.Cause{Group: ngap.CauseGroupNAS, Value: ngap.CauseNASNormalRelease})
	}

	a.clearPagingSuppression(context.Background(), ue)
}

// clearPagingSuppression runs outside the registry lock: the anchor must not be
// called while amf.mu is held.
func (a *AMF) clearPagingSuppression(ctx context.Context, ue *UeContext) {
	if a.Session == nil {
		return
	}

	supi := ue.Supi()

	for id := range ue.SmContextSnapshot() {
		if err := a.Session.ClearPagingSuppression(ctx, supi, id); err != nil {
			logger.AmfLog.Warn("failed to clear paging suppression on reconnect",
				logger.SUPI(supi.String()), zap.Error(err))
		}
	}
}

// AllocateRegistrationArea assigns the UE's registered tracking area: the whole served
// area as a single registration area, which TS 23.501 §5.3.4 permits (the AMF may allocate
// up to the served area, which always contains the UE's serving TAI).
func (ue *UeContext) AllocateRegistrationArea(supportedTais []models.Tai) {
	ue.RegistrationArea = append([]models.Tai(nil), supportedTais...)
}

func (ue *UeContext) IsAllowedNssai(targetSNssai *models.Snssai) bool {
	for _, s := range ue.AllowedNssai {
		if s.Equal(*targetSNssai) {
			return true
		}
	}

	return false
}

func (ue *UeContext) SecurityContextIsValid() bool {
	return ue.secured && ue.ngKsi.Ksi != int32(nas.NoKeyAvailable)
}

// TouchLastSeen updates the UE's last-seen timestamp lock-free — it is on the uplink hot
// path (every UE-specific NGAP message).
func (ue *UeContext) TouchLastSeen() {
	ue.lastSeen.Store(time.Now().UnixNano())
}

// lastSeenTime returns the UE's most recent last-seen time, or the zero time
// if it has never been seen. Safe for concurrent use.
func (ue *UeContext) lastSeenTime() time.Time {
	ns := ue.lastSeen.Load()
	if ns == 0 {
		return time.Time{}
	}

	return time.Unix(0, ns)
}

// SetLastSeenForTest sets the UE's last-seen time. For tests only.
func (ue *UeContext) SetLastSeenForTest(t time.Time) {
	ue.lastSeen.Store(t.UnixNano())
}

// UESnapshot is a read-only, point-in-time copy of the UE's identity and NAS security
// state, safe to use from any goroutine without holding AMF or UE locks.
type UESnapshot struct {
	Imei               string
	LastSeenAt         time.Time
	CipheringAlgorithm string
	IntegrityAlgorithm string
}

// Snapshot returns a point-in-time copy of the UE's identity and NAS security state.
func (ue *UeContext) Snapshot() UESnapshot {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	snap := UESnapshot{
		Imei:               ue.Imei.IMEI(),
		LastSeenAt:         ue.lastSeenTime(),
		CipheringAlgorithm: cipheringAlgName(ue.cipheringAlg),
		IntegrityAlgorithm: integrityAlgName(ue.integrityAlg),
	}

	return snap
}

// DeriveKamf derives Kamf per TS 33.501.
func (ue *UeContext) DeriveKamf(kseaf []byte) error {
	if !ue.supi.IsValid() || !ue.supi.IsIMSI() {
		return fmt.Errorf("supi is not a valid IMSI")
	}

	P0 := []byte(ue.supi.IMSI())
	L0 := ueauth.KDFLen(P0)
	P1 := ue.abba
	L1 := ueauth.KDFLen(P1)

	kAmfBytes, err := ueauth.GetKDFValue(kseaf, ueauth.FCForKamfDerivation, P0, L0, P1, L1)
	if err != nil {
		return fmt.Errorf("could not get kdf value: %v", err)
	}

	ue.kamf = kAmfBytes

	return nil
}

// InstallNASSecurityContext commits the negotiated NAS algorithms and derives the NAS
// algorithm keys from kamf, installing the 5G NAS security context under ue.mu
// (TS 33.501). The AuthProof witnesses that authentication has succeeded. The NAS COUNTs
// are reset separately in the downlink encode path, keyed off the new-security-context
// header type.
func (ue *UeContext) InstallNASSecurityContext(nea nas.CipheringAlgorithm, nia nas.IntegrityAlgorithm, _ AuthProof) error {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	ue.cipheringAlg, ue.integrityAlg = nea, nia

	return ue.deriveAlgKeyLocked()
}

// deriveAlgKeyLocked derives the NAS algorithm keys per TS 33.501. Caller holds ue.mu.
func (ue *UeContext) deriveAlgKeyLocked() error {
	kenc, err := fivegskeys.DeriveKNASEnc(ue.kamf, ue.cipheringAlg)
	if err != nil {
		return err
	}

	kint, err := fivegskeys.DeriveKNASInt(ue.kamf, ue.integrityAlg)
	if err != nil {
		return err
	}

	ue.knasEnc, ue.knasInt = kenc, kint

	return ue.installSecurityContextLocked()
}

// installSecurityContextLocked builds the NAS security context from the
// algorithms and keys currently held. Caller holds ue.mu.
func (ue *UeContext) installSecurityContextLocked() error {
	sc, err := nas.NewSecurityContext(nas.SecurityContextOptions{
		Integrity:    ue.integrityAlg,
		Ciphering:    ue.cipheringAlg,
		IntegrityKey: ue.knasInt,
		CipherKey:    ue.knasEnc,
		// The operator may select NIA0, which the API and UI expose deliberately.
		AllowNullIntegrity: ue.integrityAlg == nas.IntegrityNull,
	})
	if err != nil {
		// Leave no usable context behind: a security context that cannot be built
		// must not fall back to the previous one.
		ue.sc = nil

		return err
	}

	ue.sc = sc

	return nil
}

// deriveAnKeyLocked derives the access network key. Caller holds ue.mu, which
// the uplink NAS COUNT and K_AMF read here are written under.
func (ue *UeContext) deriveAnKeyLocked() error {
	// The AN key is derived from the uplink NAS COUNT of the most recently
	// accepted uplink NAS message (TS 33.501 §A.9).
	key, err := fivegskeys.DeriveKgNB(ue.kamf, ue.ulCount.LastAccepted().Value())
	if err != nil {
		return err
	}

	ue.kgnb = key[:]

	return nil
}

// DeriveNH derives the AS key-chain Next Hop per TS 33.501.
func (ue *UeContext) DeriveNH(syncInput []byte) error {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	return ue.deriveNHLocked(syncInput)
}

// deriveNHLocked derives the AS key-chain Next Hop. Caller holds ue.mu.
func (ue *UeContext) deriveNHLocked(syncInput []byte) error {
	nh, err := fivegskeys.DeriveNH(ue.kamf, syncInput)
	if err != nil {
		return err
	}

	ue.nh = nh

	return nil
}

func (ue *UeContext) UpdateSecurityContext() error {
	// One critical section for the whole derivation: it reads the uplink NAS
	// COUNT and K_AMF, which the receive path writes under this lock, and the
	// AS keys it derives have to come from one consistent view of them.
	ue.mu.Lock()
	defer ue.mu.Unlock()

	if err := ue.deriveAnKeyLocked(); err != nil {
		return fmt.Errorf("error deriving AnKey: %v", err)
	}

	if err := ue.deriveNHLocked(ue.kgnb); err != nil {
		return fmt.Errorf("error deriving NH: %v", err)
	}

	ue.ncc = 1

	return nil
}

// AdvancePathSwitchNH derives the next {NH, NCC} of the AS key chain for a path
// switch (TS 33.501 §6.9.2.1.1) without committing them to the UE, so the chain
// is advanced only once the switch is confirmed (see AMF.CommitPathSwitch).
func (ue *UeContext) AdvancePathSwitchNH() (nh [32]uint8, ncc uint8, err error) {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	return ue.deriveNextNHLocked()
}

// deriveNextNHLocked returns the next {NH, NCC} of the AS key chain (TS 33.501
// §6.9.2.1.1) without committing them to the UE. Caller holds ue.mu.
func (ue *UeContext) deriveNextNHLocked() ([32]uint8, uint8, error) {
	nh, err := fivegskeys.DeriveNH(ue.kamf, ue.nh[:])
	if err != nil {
		return [32]uint8{}, 0, err
	}

	return nh, (ue.ncc + 1) % 8, nil
}

// ClearRegistrationRequestData clears transient registration fields and
// cancels any active procedures on the current NAS connection.
func (ue *UeContext) ClearRegistrationRequestData() {
	conn := ue.Conn()
	if conn == nil {
		return
	}

	ue.mu.Lock()
	defer ue.mu.Unlock()

	conn.RegistrationRequest = nil
	conn.RegistrationRequestPlain = nil
	conn.RegistrationType5GS = 0
	conn.IdentityTypeUsedForRegistration = 0
	conn.resyncTried = false
	conn.RetransmissionOfInitialNASMsg = false
	conn.RegistrationAcceptPdu = nil

	if r := ue.active.Load(); r != nil {
		r.UeContextRequest = false
	}

	for _, t := range ue.procedures.ActiveTypes() {
		ue.procedures.End(procedure.Type(t))
	}
}

func (ue *UeContext) ClearRegistrationData(ctx context.Context) {
	// releaseSmContexts clears SmContextList under ue.mu; clearing it again here
	// would be an unguarded write.
	ue.releaseSmContexts(ctx)
}

func (ue *UeContext) CreateSmContext(pduSessionID uint8, ref string, snssai *models.Snssai, dnn string) error {
	if pduSessionID < 1 || pduSessionID > 15 {
		return fmt.Errorf("invalid PDU session ID %d: must be in range 1-15 per TS 24.501", pduSessionID)
	}

	ue.mu.Lock()
	defer ue.mu.Unlock()

	ue.SmContextList[pduSessionID] = &SmContext{
		Ref:    ref,
		Snssai: snssai,
		Dnn:    dnn,
	}

	return nil
}

func (ue *UeContext) DeleteSmContext(pduSessionID uint8) {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	delete(ue.SmContextList, pduSessionID)
	delete(ue.epsBearerIdentities, pduSessionID)
}

func (ue *UeContext) SmContextFindByPDUSessionID(pduSessionID uint8) (*SmContext, bool) {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	smContext, ok := ue.SmContextList[pduSessionID]

	return smContext, ok
}

func (ue *UeContext) SetSmContextInactive(pduSessionID uint8) {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	if sc, ok := ue.SmContextList[pduSessionID]; ok {
		sc.PduSessionInactive = true
	}
}

// Without this the inactive mark latches, and HasActivePduSessions releases a UE
// that still has a live session to CM-IDLE.
func (ue *UeContext) SetSmContextActive(pduSessionID uint8) {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	if sc, ok := ue.SmContextList[pduSessionID]; ok {
		sc.PduSessionInactive = false
	}
}

func (ue *UeContext) HasActivePduSessions() bool {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	for _, smContext := range ue.SmContextList {
		if !smContext.PduSessionInactive {
			return true
		}
	}

	return false
}

// EncodeNASMessagePlain wraps an already-encoded plain 5GMM message with NAS
// security (or returns it unchanged when no security context exists). It is the
// byte-boundary entry point for home-built (nas/fgs) message builders.
func (ue *UeContext) EncodeNASMessagePlain(plain []byte, securityHeaderType uint8) ([]byte, error) {
	if ue == nil {
		return nil, fmt.Errorf("amf ue is nil")
	}

	ue.mu.Lock()
	defer ue.mu.Unlock()

	if !ue.secured {
		return plain, nil
	}

	return ue.wrapSecuredLocked(plain, securityHeaderType)
}

func (ue *UeContext) ResetNASCounts() {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	ue.ulCount.Reset()
	ue.dlCount.Reset()
}

// wrapSecuredLocked ciphers (when the header type requires it), integrity
// protects, and frames a plain 5GMM message as a security-protected 5GS NAS
// message (TS 24.501 §4.4.4, §9.1.1). The caller must hold ue.mu and have a
// security context. It advances the downlink NAS COUNT.
func (ue *UeContext) wrapSecuredLocked(plain []byte, sht uint8) ([]byte, error) {
	headerType := fgs.SecurityHeaderType(sht)

	switch headerType {
	case fgs.SHTIntegrityProtected, fgs.SHTIntegrityProtectedCiphered, fgs.SHTIntegrityProtectedNewContext:
	default:
		return nil, fmt.Errorf("wrong security header type: 0x%0x", sht)
	}

	// Protect with the current NAS COUNT and advance only once the message is
	// protected, so a protection failure does not consume a downlink COUNT
	// (TS 24.501 §4.4.3.1).
	count, err := ue.dlCount.Use()
	if err != nil {
		return nil, err
	}

	wire, err := fgs.Protect(plain, headerType, count, nas.DirectionDownlink, ue.sc)
	if err != nil {
		return nil, err
	}

	return wire, nil
}

func (ue *UeContext) StopProcedureTimers() {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	conn := ue.Conn()
	if conn == nil {
		return
	}

	conn.StopNASGuard()
}

// ReleaseNasConnection releases the NAS signalling connection between
// this AMF UE and a RAN UE. Target, when non-nil, makes the release a
// no-op if a newer RAN UE has already taken over.
func (a *AMF) ReleaseNasConnection(ue *UeContext, target *UeConn) {
	if ue == nil {
		return
	}

	conn := ue.Conn()
	if conn == nil {
		return
	}

	a.mu.Lock()
	detached := a.detachUeConnLocked(ue, target)
	a.mu.Unlock()

	if detached == nil {
		return
	}

	ue.endKeyChainProcs()
	ue.StopProcedureTimers()

	detached.Release()
}

// detachUeConnLocked clears ue.active when it matches target (or unconditionally when
// target is nil), clears the connection's back-pointer, and returns the detached
// connection (nil if nothing matched). Caller holds amf.mu.
func (a *AMF) detachUeConnLocked(ue *UeContext, target *UeConn) *UeConn {
	cur := ue.active.Load()
	if cur == nil {
		return nil
	}

	if target != nil && cur != target {
		return nil
	}

	if cur.ue.Load() == ue {
		cur.ue.Store(nil)
	}

	ue.active.Store(nil)

	return cur
}

// Caller must hold ue.mu. Not "stop everything": the idle-mode timers live under
// the registry lock and the NAS/release guards on the connection.
// AMF.StopAllTimers covers all three.
func (ue *UeContext) stopUeMuTimersLocked() {
	ue.pagingTimer.Stop()
}

// StopPaging cancels paging supervision once the UE answers or is released. The
// guard invalidates any in-flight callback, so a retransmission racing the stop
// is a no-op.
func (ue *UeContext) StopPaging() {
	if ue == nil {
		return
	}

	ue.pagingTimer.Stop()
}

// PagingActive reports whether paging supervision is in flight for the UE.
// Paging is per UE (TS 24.501 §5.6.2.2), so this per-UE timer is the paging state.
func (ue *UeContext) PagingActive() bool {
	if ue == nil {
		return false
	}

	return ue.pagingTimer.Active()
}

func (ue *UeContext) Deregister(ctx context.Context) {
	// Release takes the registry lock, so it runs before ue.mu to preserve the lock
	// order registry lock → ue.mu.
	if conn := ue.Conn(); conn != nil {
		conn.Release()
	}

	ue.mu.Lock()

	ue.stopUeMuTimersLocked()

	ue.transitionToLocked(Deregistered)

	smContextRefs := make([]string, 0, len(ue.SmContextList))
	for _, smContext := range ue.SmContextList {
		smContextRefs = append(smContextRefs, smContext.Ref)
	}

	ue.SmContextList = make(map[uint8]*SmContext)
	ue.mu.Unlock()

	// External SMF calls must happen without holding UE lock.
	if ue.smf != nil {
		for _, smContextRef := range smContextRefs {
			err := ue.smf.ReleaseSmContext(ctx, smContextRef)
			if err != nil {
				logger.From(ctx, logger.AmfLog).Error("Release SmContext Error", zap.Error(err))
			}
		}
	}

	logger.From(ctx, logger.AmfLog).Debug("ue deregistered", logger.SUPI(ue.supi.String()))
}

// deactivateSmContexts deactivates the user-plane connection of every PDU session
// without releasing it, so the UPF releases the N3 tunnel toward the RAN, buffers
// downlink, and paging reactivates the session (TS 23.501 §5.3.3.2.4 / §5.8.3). Used on
// abrupt NG-C loss where no UE Context Release Complete arrives.
func (ue *UeContext) deactivateSmContexts(ctx context.Context) {
	if ue == nil || ue.smf == nil {
		return
	}

	for _, ref := range ue.SmContextRefs() {
		if err := ue.smf.DeactivateSmContext(ctx, ref.Ref); err != nil {
			logger.From(ctx, logger.AmfLog).Warn("failed to deactivate SM context for paging", zap.Error(err), zap.Uint8("PduSessionID", ref.PduSessionID))
		}
	}
}

func (ue *UeContext) releaseSmContexts(ctx context.Context) {
	// Cleared under the lock even with no SMF wired up: ClearRegistrationData
	// relies on this, and clearing it there would be an unguarded write.
	// External SMF calls must not hold ue.mu.
	ue.mu.Lock()

	smContextRefs := make([]string, 0, len(ue.SmContextList))
	for _, smContext := range ue.SmContextList {
		smContextRefs = append(smContextRefs, smContext.Ref)
	}

	ue.SmContextList = make(map[uint8]*SmContext)
	ue.mu.Unlock()

	if ue.smf == nil {
		return
	}

	for _, smContextRef := range smContextRefs {
		err := ue.smf.ReleaseSmContext(ctx, smContextRef)
		if err != nil {
			logger.From(ctx, logger.AmfLog).Error("Release SmContext Error", zap.Error(err))
		}
	}
}

// GetUserLocation returns a copy of the UE's user location.
func (ue *UeContext) GetUserLocation() models.UserLocation {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	return ue.Location
}

// IsUserLocationEmpty returns true if the UE has no location information.
func (ue *UeContext) IsUserLocationEmpty() bool {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	loc := ue.Location

	return loc.EutraLocation == nil &&
		loc.NrLocation == nil &&
		loc.N3gaLocation == nil
}

func (ue *UeContext) DeleteSmContextRef(pduSessionID uint8, ref string) bool {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	sc, ok := ue.SmContextList[pduSessionID]
	if !ok || sc.Ref != ref {
		return false
	}

	delete(ue.SmContextList, pduSessionID)

	return true
}
