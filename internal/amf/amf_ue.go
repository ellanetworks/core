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
	"encoding/binary"
	"fmt"
	"slices"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/amf/procedure"
	"github.com/ellanetworks/core/internal/guard"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/util/ueauth"
	"github.com/free5gc/nas"
	"github.com/free5gc/nas/nasMessage"
	"github.com/free5gc/nas/nasType"
	"github.com/free5gc/nas/security"
	"go.uber.org/zap"
)

type StateType string

const (
	Deregistered   StateType = "Deregistered"
	Authentication StateType = "Authentication"
	SecurityMode   StateType = "SecurityMode"
	ContextSetup   StateType = "ContextSetup"
	Registered     StateType = "Registered"
)

var validTransitions = map[StateType][]StateType{
	Deregistered:   {Authentication},
	Authentication: {SecurityMode, Deregistered},
	SecurityMode:   {ContextSetup, Deregistered},
	ContextSetup:   {Registered, Deregistered},
	Registered:     {Authentication, Deregistered},
}

type SmContext struct {
	Ref                string
	Snssai             *models.Snssai
	PduSessionInactive bool
}

type UeContext struct {
	mu sync.Mutex

	state StateType

	PlmnID  models.PlmnID
	Suci    string
	supi    etsi.SUPI
	Pei     string
	Tmsi    etsi.TMSI
	OldTmsi etsi.TMSI
	guti    etsi.GUTI
	OldGuti etsi.GUTI

	Location models.UserLocation
	Tai      models.Tai
	// Updated lock-free on every UE-specific NGAP message (hot path).
	lastSeen      atomic.Int64 // Unix nanoseconds; use lastSeenTime()/TouchLastSeen()
	lastSeenRadio atomic.Pointer[string]
	ranUe         *RanUe

	// handover is the in-flight N2 handover FSM (nil when none); see handover.go.
	// Guarded by mu.
	handover *handoverContext

	smf SmfSbi

	Log *zap.Logger

	// ctx is the per-registration context. NAS connection contexts derive from
	// it; cancelling it on RotateContext unwinds procedures and in-flight RPCs.
	ctx    context.Context
	cancel context.CancelFunc

	active atomic.Pointer[ActiveNasConnection]

	// NAS security context per TS 33.501.
	secured              bool
	ueSecurityCapability *nasType.UESecurityCapability
	ngKsi                models.NgKsi
	knasInt              [16]uint8
	knasEnc              [16]uint8
	kgnb                 []uint8
	nh                   [32]uint8 // AS key-chain Next Hop, 256 bits (TS 33.501)
	ncc                  uint8
	ulCount              security.Count
	dlCount              security.Count
	cipheringAlg         uint8
	integrityAlg         uint8
	kamf                 []uint8
	abba                 []uint8

	Ambr                       *models.Ambr
	AllowedNssai               []models.Snssai
	RegistrationArea           []models.Tai
	UeRadioCapability          []byte
	UeRadioCapabilityForPaging *models.UERadioCapabilityForPaging
	UESpecificDRX              uint8
	SmContextList              map[uint8]*SmContext

	T3502Value time.Duration
	T3512Value time.Duration

	// Idle-mode supervision (TS 24.501): the mobile reachable timer escalates to
	// implicit deregistration. idleGen bumps on every (re)arm/stop so an expiry
	// that fired just as the UE reconnected is ignored when it re-checks under
	// ue.mu. Both are one logical episode keyed by idleGen.
	mobileReachableTimer        guard.Guard
	implicitDeregistrationTimer guard.Guard
	idleGen                     uint64

	/* Radio measurements (E-CID) */
	radioMu           sync.RWMutex
	radioMeasurements *RadioMeasurements

	/* NRPPa messages (RAN → LMF) */
	nrppaMu       sync.RWMutex
	nrppaMessages []NRPPaMessage
}

func NewUeContext() *UeContext {
	ue := &UeContext{
		state:            Deregistered,
		SmContextList:    make(map[uint8]*SmContext),
		RegistrationArea: make([]models.Tai, 0),
	}

	ue.ctx, ue.cancel = context.WithCancel(context.Background())
	ue.active.Store(newActiveNasConnection(ue, nil))

	return ue
}

// Ctx returns the per-registration context. NAS connection contexts derive
// from it; it is cancelled and replaced when the 5GMM context is rotated.
func (ue *UeContext) Ctx() context.Context {
	return ue.ctx
}

// NasConn returns the active NAS connection, or nil if none is established.
func (ue *UeContext) NasConn() *ActiveNasConnection {
	if ue == nil {
		return nil
	}

	return ue.active.Load()
}

// RotateContext discards the current 5GMM security and session state and starts
// a fresh per-registration context, cancelling the prior ctx to unwind its
// procedures and in-flight RPCs (TS 24.501: an initial registration arriving in
// 5GMM-REGISTERED). The NAS connection is left unset until a RanUe is attached.
func (ue *UeContext) RotateContext() {
	if old := ue.active.Swap(nil); old != nil {
		old.stopTimers()
		old.cancel()
	}

	ue.stopIdleTimers()

	if ue.cancel != nil {
		ue.cancel()
	}

	ue.ctx, ue.cancel = context.WithCancel(context.Background())

	ue.resetSecurityContext()
}

// AttachNasConnection installs a fresh NAS connection, replacing any prior one.
func (ue *UeContext) AttachNasConnection(ranUe *RanUe) *ActiveNasConnection {
	conn := newActiveNasConnection(ue, ranUe)
	if old := ue.active.Swap(conn); old != nil {
		old.stopTimers()
		old.cancel()
	}

	return conn
}

// resetSecurityContext clears the 5GMM security and session state, returning the
// field state to that of a freshly-constructed context.
func (ue *UeContext) resetSecurityContext() {
	ue.secured = false
	ue.ueSecurityCapability = nil
	ue.ngKsi = models.NgKsi{}
	ue.knasInt = [16]uint8{}
	ue.knasEnc = [16]uint8{}
	ue.kgnb = nil
	ue.nh = [32]uint8{}
	ue.ncc = 0
	ue.ulCount = security.Count{}
	ue.dlCount = security.Count{}
	ue.cipheringAlg = 0
	ue.integrityAlg = 0
	ue.kamf = nil
	ue.abba = nil
	ue.Ambr = nil
	ue.AllowedNssai = nil
	ue.RegistrationArea = make([]models.Tai, 0)
	ue.UeRadioCapability = nil
	ue.UeRadioCapabilityForPaging = nil
	ue.UESpecificDRX = 0
	ue.SmContextList = make(map[uint8]*SmContext)
	ue.T3502Value = 0
	ue.T3512Value = 0
}

// stopIdleTimers cancels both idle-mode timers and bumps idleGen so an expiry
// that has already fired becomes a no-op. Caller holds ue.mu.
func (ue *UeContext) stopIdleTimers() {
	ue.idleGen++
	ue.mobileReachableTimer.Stop()
	ue.implicitDeregistrationTimer.Stop()
}

// RanUe returns the currently attached RanUe, or nil. Callers must capture the
// result in a local and reuse it — never call RanUe() twice in the same code
// path, as the underlying pointer may change between calls.
func (ue *UeContext) RanUe() *RanUe {
	if ue == nil {
		return nil
	}

	ue.mu.Lock()
	r := ue.ranUe
	ue.mu.Unlock()

	return r
}

func (ue *UeContext) State() StateType {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	return ue.state
}

// ForceState sets the UE state unconditionally, bypassing transition validation.
// It exists for test precondition setup in external packages. Production code
// must use TransitionTo.
func (ue *UeContext) ForceState(s StateType) {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	ue.state = s
}

func (ue *UeContext) TransitionTo(target StateType) {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	ue.transitionToLocked(target)
}

// transitionToLocked enforces allowed state transitions and must only be called while ue.Mutex is held.
func (ue *UeContext) transitionToLocked(target StateType) {
	if ue.state == target {
		return
	}

	if slices.Contains(validTransitions[ue.state], target) {
		if ue.Log != nil {
			ue.Log.Debug("state transition",
				zap.String("from", string(ue.state)),
				zap.String("to", string(target)))
		}

		ue.state = target

		return
	}

	if ue.Log != nil {
		ue.Log.Error("invalid state transition",
			zap.String("from", string(ue.state)),
			zap.String("to", string(target)))
	}

	ue.state = Deregistered
}

func (ue *UeContext) AttachRanUe(ranUe *RanUe) {
	if ranUe == nil {
		return
	}

	ue.mu.Lock()

	oldRanUe := ue.ranUe

	ue.ranUe = ranUe

	ranUe.amfUe = ue

	if oldRanUe != nil && oldRanUe != ranUe {
		if oldRanUe.amfUe == ue {
			oldRanUe.Log.Info("Detached UeContext from previous RanUe")
			oldRanUe.amfUe = nil
		}
	}

	ue.lastSeen.Store(time.Now().UnixNano())

	if ranUe.radio != nil {
		name := ranUe.radio.Name
		ue.lastSeenRadio.Store(&name)
	}

	ue.Log = logger.AmfLog.With(logger.AmfUeNgapID(ranUe.AmfUeNgapID))

	ue.mu.Unlock()

	if ue.active.Load() == nil {
		ue.active.Store(newActiveNasConnection(ue, ranUe))
	}
}

func (ue *UeContext) AllocateRegistrationArea(supportedTais []models.Tai) {
	if len(ue.RegistrationArea) > 0 {
		ue.RegistrationArea = nil
	}

	taiList := make([]models.Tai, len(supportedTais))
	copy(taiList, supportedTais)

	for _, supportTai := range taiList {
		if supportTai.Equal(ue.Tai) {
			ue.RegistrationArea = append(ue.RegistrationArea, supportTai)
			break
		}
	}
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
	return ue.secured && ue.ngKsi.Ksi != nasMessage.NasKeySetIdentifierNoKeyIsAvailable
}

// cipheringAlgName must be called while holding ue.Mutex.
func (ue *UeContext) cipheringAlgName() string {
	switch ue.cipheringAlg {
	case security.AlgCiphering128NEA0:
		return "NEA0"
	case security.AlgCiphering128NEA1:
		return "NEA1"
	case security.AlgCiphering128NEA2:
		return "NEA2"
	case security.AlgCiphering128NEA3:
		return "NEA3"
	default:
		return ""
	}
}

// integrityAlgName must be called while holding ue.Mutex.
func (ue *UeContext) integrityAlgName() string {
	switch ue.integrityAlg {
	case security.AlgIntegrity128NIA0:
		return "NIA0"
	case security.AlgIntegrity128NIA1:
		return "NIA1"
	case security.AlgIntegrity128NIA2:
		return "NIA2"
	case security.AlgIntegrity128NIA3:
		return "NIA3"
	default:
		return ""
	}
}

// TouchLastSeen updates the UE's last-seen timestamp and radio name lock-free —
// it is on the uplink hot path (every UE-specific NGAP message).
func (ue *UeContext) TouchLastSeen(radioName string) {
	ue.lastSeen.Store(time.Now().UnixNano())

	if radioName != "" {
		ue.lastSeenRadio.Store(&radioName)
	}
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

// LastSeenRadioName returns the name of the radio the UE was last seen on, or ""
// if it has never been seen. Safe for concurrent use.
func (ue *UeContext) LastSeenRadioName() string {
	if p := ue.lastSeenRadio.Load(); p != nil {
		return *p
	}

	return ""
}

// SetLastSeenForTest sets the UE's last-seen time and radio. For tests only.
func (ue *UeContext) SetLastSeenForTest(t time.Time, radio string) {
	ue.lastSeen.Store(t.UnixNano())

	if radio != "" {
		ue.lastSeenRadio.Store(&radio)
	}
}

// UESnapshot is a read-only, point-in-time copy of the UE's connection
// state. It is safe to use from any goroutine without holding AMF or UE locks.
type UESnapshot struct {
	State              StateType
	Pei                string
	CipheringAlgorithm string
	IntegrityAlgorithm string
	LastSeenAt         time.Time
	LastSeenRadio      string
}

// Snapshot returns a point-in-time copy of the UE's connection state.
// The caller can safely read the returned value without holding any lock.
func (ue *UeContext) Snapshot() UESnapshot {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	snap := UESnapshot{
		State:              ue.state,
		Pei:                ue.Pei,
		CipheringAlgorithm: ue.cipheringAlgName(),
		IntegrityAlgorithm: ue.integrityAlgName(),
		LastSeenAt:         ue.lastSeenTime(),
		LastSeenRadio:      ue.LastSeenRadioName(),
	}

	return snap
}

// Kamf Derivation function defined in TS 33.501
func (ue *UeContext) DerivateKamf(kseaf []byte) error {
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

// Algorithm key Derivation function defined in TS 33.501
func (ue *UeContext) DerivateAlgKey() error {
	P0 := []byte{security.NNASEncAlg}
	L0 := ueauth.KDFLen(P0)
	P1 := []byte{ue.cipheringAlg}
	L1 := ueauth.KDFLen(P1)

	kenc, err := ueauth.GetKDFValue(ue.kamf, ueauth.FCForAlgorithmKeyDerivation, P0, L0, P1, L1)
	if err != nil {
		return fmt.Errorf("get kdf value error: %v", err)
	}

	copy(ue.knasEnc[:], kenc[16:32])

	P0 = []byte{security.NNASIntAlg}
	L0 = ueauth.KDFLen(P0)
	P1 = []byte{ue.integrityAlg}
	L1 = ueauth.KDFLen(P1)

	kint, err := ueauth.GetKDFValue(ue.kamf, ueauth.FCForAlgorithmKeyDerivation, P0, L0, P1, L1)
	if err != nil {
		return fmt.Errorf("get kdf value error: %v", err)
	}

	copy(ue.knasInt[:], kint[16:32])

	return nil
}

// Access Network key Derivation function defined in TS 33.501
func (ue *UeContext) DerivateAnKey() error {
	P0 := make([]byte, 4)
	binary.BigEndian.PutUint32(P0, ue.ulCount.Get())
	L0 := ueauth.KDFLen(P0)
	P1 := []byte{security.AccessType3GPP}
	L1 := ueauth.KDFLen(P1)

	key, err := ueauth.GetKDFValue(ue.kamf, ueauth.FCForKgnbKn3iwfDerivation, P0, L0, P1, L1)
	if err != nil {
		return fmt.Errorf("could not get kdf value: %v", err)
	}

	ue.kgnb = key

	return nil
}

// NH Derivation function defined in TS 33.501
func (ue *UeContext) DerivateNH(syncInput []byte) error {
	P0 := syncInput
	L0 := ueauth.KDFLen(P0)

	nh, err := ueauth.GetKDFValue(ue.kamf, ueauth.FCForNhDerivation, P0, L0)
	if err != nil {
		return fmt.Errorf("could not get kdf value: %v", err)
	}

	if len(nh) != len(ue.nh) {
		return fmt.Errorf("unexpected NH length %d, want %d", len(nh), len(ue.nh))
	}

	ue.nh = [32]uint8(nh)

	return nil
}

func (ue *UeContext) UpdateSecurityContext() error {
	err := ue.DerivateAnKey()
	if err != nil {
		return fmt.Errorf("error deriving AnKey: %v", err)
	}

	err = ue.DerivateNH(ue.kgnb)
	if err != nil {
		return fmt.Errorf("error deriving NH: %v", err)
	}

	ue.ncc = 1

	return nil
}

func (ue *UeContext) UpdateNH() error {
	ue.ncc = (ue.ncc + 1) % 8

	err := ue.DerivateNH(ue.nh[:])
	if err != nil {
		return fmt.Errorf("error deriving NH: %v", err)
	}

	return nil
}

func (ue *UeContext) SelectSecurityAlg(intOrder, encOrder []uint8) error {
	if ue.ueSecurityCapability == nil {
		return fmt.Errorf("UE security capability not available, cannot negotiate NAS security algorithms")
	}

	intFound := false
	ueSupported := uint8(0)

	for _, intAlg := range intOrder {
		switch intAlg {
		case security.AlgIntegrity128NIA0:
			ueSupported = ue.ueSecurityCapability.GetIA0_5G()
		case security.AlgIntegrity128NIA1:
			ueSupported = ue.ueSecurityCapability.GetIA1_128_5G()
		case security.AlgIntegrity128NIA2:
			ueSupported = ue.ueSecurityCapability.GetIA2_128_5G()
		case security.AlgIntegrity128NIA3:
			ueSupported = ue.ueSecurityCapability.GetIA3_128_5G()
		}

		if ueSupported == 1 {
			ue.integrityAlg = intAlg
			intFound = true

			break
		}
	}

	if !intFound {
		return fmt.Errorf("no common NAS integrity algorithm found")
	}

	encFound := false
	ueSupported = uint8(0)

	for _, encAlg := range encOrder {
		switch encAlg {
		case security.AlgCiphering128NEA0:
			ueSupported = ue.ueSecurityCapability.GetEA0_5G()
		case security.AlgCiphering128NEA1:
			ueSupported = ue.ueSecurityCapability.GetEA1_128_5G()
		case security.AlgCiphering128NEA2:
			ueSupported = ue.ueSecurityCapability.GetEA2_128_5G()
		case security.AlgCiphering128NEA3:
			ueSupported = ue.ueSecurityCapability.GetEA3_128_5G()
		}

		if ueSupported == 1 {
			ue.cipheringAlg = encAlg
			encFound = true

			break
		}
	}

	if !encFound {
		return fmt.Errorf("no common NAS ciphering algorithm found")
	}

	return nil
}

// ClearRegistrationRequestData clears transient registration fields and
// cancels any active procedures on the current NAS connection.
func (ue *UeContext) ClearRegistrationRequestData() {
	conn := ue.NasConn()
	if conn == nil {
		return
	}

	conn.RegistrationRequest = nil
	conn.RegistrationType5GS = 0
	conn.IdentityTypeUsedForRegistration = 0
	conn.AuthFailureCauseSynchFailureTimes = 0
	conn.RetransmissionOfInitialNASMsg = false

	if ue.ranUe != nil {
		ue.ranUe.UeContextRequest = false
	}

	for _, t := range conn.Procedures.ActiveTypes() {
		conn.Procedures.End(procedure.Type(t))
	}
}

func (ue *UeContext) ClearRegistrationData(ctx context.Context) {
	ue.releaseSmContexts(ctx)

	ue.SmContextList = make(map[uint8]*SmContext)
}

func (ue *UeContext) CreateSmContext(pduSessionID uint8, ref string, snssai *models.Snssai) error {
	if pduSessionID < 1 || pduSessionID > 15 {
		return fmt.Errorf("invalid PDU session ID %d: must be in range 1-15 per TS 24.501", pduSessionID)
	}

	ue.mu.Lock()
	defer ue.mu.Unlock()

	ue.SmContextList[pduSessionID] = &SmContext{
		Ref:    ref,
		Snssai: snssai,
	}

	return nil
}

func (ue *UeContext) DeleteSmContext(pduSessionID uint8) {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	delete(ue.SmContextList, pduSessionID)
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

func (ue *UeContext) EncodeNASMessage(msg *nas.Message) ([]byte, error) {
	if ue == nil {
		return nil, fmt.Errorf("amf ue is nil")
	}

	if msg == nil {
		return nil, fmt.Errorf("nas message is nil")
	}

	ue.mu.Lock()
	defer ue.mu.Unlock()

	if !ue.secured {
		return msg.PlainNasEncode()
	}

	// A security-protected NAS message must be integrity protected; ciphering is optional.
	needCiphering := false

	switch msg.SecurityHeaderType {
	case nas.SecurityHeaderTypeIntegrityProtected:
	case nas.SecurityHeaderTypeIntegrityProtectedAndCiphered:
		needCiphering = true
	case nas.SecurityHeaderTypeIntegrityProtectedWithNew5gNasSecurityContext:
		ue.ulCount.Set(0, 0)
		ue.dlCount.Set(0, 0)
	default:
		return nil, fmt.Errorf("wrong security header type: 0x%0x", msg.SecurityHeaderType)
	}

	payload, err := msg.PlainNasEncode()
	if err != nil {
		return nil, fmt.Errorf("error encoding plain nas: %+v", err)
	}

	if needCiphering {
		if err = security.NASEncrypt(ue.cipheringAlg, ue.knasEnc, ue.dlCount.Get(), security.Bearer3GPP, security.DirectionDownlink, payload); err != nil {
			return nil, fmt.Errorf("error encrypting: %+v", err)
		}
	}

	payload = append([]byte{ue.dlCount.SQN()}, payload[:]...)

	mac32, err := security.NASMacCalculate(ue.integrityAlg, ue.knasInt, ue.dlCount.Get(), security.Bearer3GPP, security.DirectionDownlink, payload)
	if err != nil {
		return nil, fmt.Errorf("MAC calcuate error: %+v", err)
	}

	payload = append(mac32, payload[:]...)

	msgSecurityHeader := []byte{msg.ProtocolDiscriminator, msg.SecurityHeaderType}
	payload = append(msgSecurityHeader, payload[:]...)

	ue.dlCount.AddOne()

	return payload, nil
}

func (ue *UeContext) ResetMobileReachableTimer() {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	ue.stopIdleTimers()
	gen := ue.idleGen

	ue.Log.Debug("starting mobile reachable timer", logger.SUPI(ue.supi.String()))

	ue.mobileReachableTimer.ArmOnce(ue.T3512Value+(4*time.Minute), func() {
		ue.onMobileReachableExpiry(gen)
	})
}

// StopMobileReachableTimer ends idle-mode supervision when the UE becomes
// reachable again.
func (ue *UeContext) StopMobileReachableTimer() {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	ue.Log.Debug("stopping idle-mode timers")
	ue.stopIdleTimers()
}

// StopImplicitDeregistrationTimer ends idle-mode supervision when the UE becomes
// reachable again.
func (ue *UeContext) StopImplicitDeregistrationTimer() {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	ue.Log.Debug("stopping idle-mode timers")
	ue.stopIdleTimers()
}

// onMobileReachableExpiry escalates to the implicit deregistration timer once the
// mobile reachable timer fires (TS 24.501). It no-ops if a reconnect bumped
// idleGen after this timer was armed.
func (ue *UeContext) onMobileReachableExpiry(gen uint64) {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	if ue.idleGen != gen {
		return
	}

	ue.Log.Debug("mobile reachable timer expired", logger.SUPI(ue.supi.String()))

	ue.implicitDeregistrationTimer.ArmOnce(2*time.Minute, func() {
		ue.onImplicitDeregistrationExpiry(gen)
	})
}

// onImplicitDeregistrationExpiry deregisters an unreachable UE (TS 24.501). It
// no-ops if a reconnect bumped idleGen after the implicit timer was armed.
func (ue *UeContext) onImplicitDeregistrationExpiry(gen uint64) {
	ue.mu.Lock()
	stale := ue.idleGen != gen
	ue.mu.Unlock()

	if stale {
		return
	}

	ue.Deregister(context.Background())
}

func (ue *UeContext) StopProcedureTimers() {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	conn := ue.NasConn()
	if conn == nil {
		return
	}

	for _, g := range []*guard.Guard{&conn.T3513, &conn.T3565, &conn.T3560, &conn.T3550, &conn.T3555, &conn.T3522} {
		g.Stop()
	}
}

// ReleaseNasConnection releases the NAS signalling connection between
// this AMF UE and a RAN UE. Target, when non-nil, makes the release a
// no-op if a newer RAN UE has already taken over.
func (ue *UeContext) ReleaseNasConnection(target *RanUe) {
	if ue == nil {
		return
	}

	if !ue.detachRanUeIfMatch(target) {
		return
	}

	ue.StopProcedureTimers()

	if conn := ue.NasConn(); conn != nil {
		conn.Release()
	}
}

func (ue *UeContext) detachRanUeIfMatch(target *RanUe) bool {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	if ue.ranUe == nil {
		return false
	}

	if target != nil && ue.ranUe != target {
		return false
	}

	if ue.ranUe.amfUe == ue {
		ue.ranUe.amfUe = nil
	}

	ue.ranUe = nil

	return true
}

// stopAllTimersLocked stops every idle timer on the 5GMM context. Caller
// must hold ue.Mutex. Procedure retransmission timers are torn down via
// NAS-connection ctx cancellation.
func (ue *UeContext) stopAllTimersLocked() {
	ue.stopIdleTimers()
}

func (ue *UeContext) Deregister(ctx context.Context) {
	ue.mu.Lock()

	if conn := ue.NasConn(); conn != nil {
		conn.Release()
	}

	ue.stopAllTimersLocked()

	ue.transitionToLocked(Deregistered)

	// Copy refs and clear map while protected by UE lock.
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
				ue.Log.Error("Release SmContext Error", zap.Error(err))
			}
		}
	}

	ue.Log.Debug("ue deregistered", logger.SUPI(ue.supi.String()))
}

func (ue *UeContext) releaseSmContexts(ctx context.Context) {
	if ue.smf == nil {
		return
	}

	// Copy refs under lock, then release lock before external SMF calls.
	ue.mu.Lock()

	smContextRefs := make([]string, 0, len(ue.SmContextList))
	for _, smContext := range ue.SmContextList {
		smContextRefs = append(smContextRefs, smContext.Ref)
	}

	ue.SmContextList = make(map[uint8]*SmContext)
	ue.mu.Unlock()

	for _, smContextRef := range smContextRefs {
		err := ue.smf.ReleaseSmContext(ctx, smContextRef)
		if err != nil {
			ue.Log.Error("Release SmContext Error", zap.Error(err))
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
