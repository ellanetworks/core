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
	"encoding/hex"
	"fmt"
	"slices"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/amf/procedure"
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
	Mutex sync.RWMutex

	/* Gmm State */
	state StateType
	/* Ue Identity*/
	PlmnID  models.PlmnID
	Suci    string
	supi    etsi.SUPI
	Pei     string
	Tmsi    etsi.TMSI
	OldTmsi etsi.TMSI
	Guti    etsi.GUTI
	OldGuti etsi.GUTI
	/* User Location*/
	Location models.UserLocation
	Tai      models.Tai
	/* Last Seen — updated on every UE-specific NGAP message */
	LastSeenAt    time.Time
	LastSeenRadio string
	ranUe         *RanUe

	smf SmfSbi

	Log *zap.Logger

	// ctx is the per-registration context. NAS connection contexts derive from
	// it; cancelling it on RotateContext unwinds procedures and in-flight RPCs.
	ctx    context.Context
	cancel context.CancelFunc

	active atomic.Pointer[ActiveNasConnection]

	// NAS security context per TS 33.501.
	SecurityContextAvailable bool
	ueSecurityCapability     *nasType.UESecurityCapability
	NgKsi                    models.NgKsi
	KnasInt                  [16]uint8
	KnasEnc                  [16]uint8
	Kgnb                     []uint8
	NH                       []uint8
	NCC                      uint8
	ULCount                  security.Count
	DLCount                  security.Count
	CipheringAlg             uint8
	IntegrityAlg             uint8
	Kamf                     string
	ABBA                     []uint8

	Ambr                       *models.Ambr
	AllowedNssai               []models.Snssai
	RegistrationArea           []models.Tai
	UeRadioCapability          string
	UeRadioCapabilityForPaging *models.UERadioCapabilityForPaging
	UESpecificDRX              uint8
	SmContextList              map[uint8]*SmContext

	T3502Value time.Duration
	T3512Value time.Duration

	MobileReachableTimer        *Timer
	ImplicitDeregistrationTimer *Timer
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
	ue.SecurityContextAvailable = false
	ue.ueSecurityCapability = nil
	ue.NgKsi = models.NgKsi{}
	ue.KnasInt = [16]uint8{}
	ue.KnasEnc = [16]uint8{}
	ue.Kgnb = nil
	ue.NH = nil
	ue.NCC = 0
	ue.ULCount = security.Count{}
	ue.DLCount = security.Count{}
	ue.CipheringAlg = 0
	ue.IntegrityAlg = 0
	ue.Kamf = ""
	ue.ABBA = nil
	ue.Ambr = nil
	ue.AllowedNssai = nil
	ue.RegistrationArea = make([]models.Tai, 0)
	ue.UeRadioCapability = ""
	ue.UeRadioCapabilityForPaging = nil
	ue.UESpecificDRX = 0
	ue.SmContextList = make(map[uint8]*SmContext)
	ue.T3502Value = 0
	ue.T3512Value = 0
}

// stopIdleTimers stops and clears the registered-but-idle timers (mobile
// reachable and implicit deregistration).
func (ue *UeContext) stopIdleTimers() {
	if ue.MobileReachableTimer != nil {
		ue.MobileReachableTimer.Stop()
		ue.MobileReachableTimer = nil
	}

	if ue.ImplicitDeregistrationTimer != nil {
		ue.ImplicitDeregistrationTimer.Stop()
		ue.ImplicitDeregistrationTimer = nil
	}
}

// RanUe returns the currently attached RanUe, or nil.
// The read is synchronized via ue.Mutex so the returned pointer is a
// consistent snapshot.  Callers must capture the result in a local
// variable and reuse it — never call RanUe() twice in the same
// code path, as the underlying pointer may change between calls.
func (ue *UeContext) RanUe() *RanUe {
	if ue == nil {
		return nil
	}

	ue.Mutex.RLock()
	r := ue.ranUe
	ue.Mutex.RUnlock()

	return r
}

func (ue *UeContext) GetState() StateType {
	ue.Mutex.Lock()
	defer ue.Mutex.Unlock()

	return ue.state
}

// ForceState sets the UE state unconditionally, bypassing transition validation.
// It exists for test precondition setup in external packages. Production code
// must use TransitionTo.
func (ue *UeContext) ForceState(s StateType) {
	ue.Mutex.Lock()
	defer ue.Mutex.Unlock()

	ue.state = s
}

func (ue *UeContext) TransitionTo(target StateType) {
	ue.Mutex.Lock()
	defer ue.Mutex.Unlock()

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

	ue.Mutex.Lock()

	oldRanUe := ue.ranUe

	ue.ranUe = ranUe

	ranUe.amfUe = ue

	if oldRanUe != nil && oldRanUe != ranUe {
		if oldRanUe.amfUe == ue {
			oldRanUe.Log.Info("Detached UeContext from previous RanUe")
			oldRanUe.amfUe = nil
		}
	}

	ue.LastSeenAt = time.Now()
	if ranUe.radio != nil {
		ue.LastSeenRadio = ranUe.radio.Name
	}

	ue.Log = logger.AmfLog.With(logger.AmfUeNgapID(ranUe.AmfUeNgapID))

	ue.Mutex.Unlock()

	if ue.active.Load() == nil {
		ue.active.Store(newActiveNasConnection(ue, ranUe))
	}
}

func (ue *UeContext) AllocateRegistrationArea(supportedTais []models.Tai) {
	// clear the previous registration area if need
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
	return ue.SecurityContextAvailable && ue.NgKsi.Ksi != nasMessage.NasKeySetIdentifierNoKeyIsAvailable
}

// cipheringAlgName returns the human-readable name for the negotiated NAS ciphering algorithm.
// Must be called while holding ue.Mutex.
func (ue *UeContext) cipheringAlgName() string {
	switch ue.CipheringAlg {
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

// integrityAlgName returns the human-readable name for the negotiated NAS integrity algorithm.
// Must be called while holding ue.Mutex.
func (ue *UeContext) integrityAlgName() string {
	switch ue.IntegrityAlg {
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

// TouchLastSeen updates the UE's last-seen timestamp and radio name.
// Must be called while the UE mutex is NOT held (it acquires the lock).
func (ue *UeContext) TouchLastSeen(radioName string) {
	ue.Mutex.Lock()
	defer ue.Mutex.Unlock()

	ue.LastSeenAt = time.Now()
	if radioName != "" {
		ue.LastSeenRadio = radioName
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
	ue.Mutex.Lock()
	defer ue.Mutex.Unlock()

	snap := UESnapshot{
		State:              ue.state,
		Pei:                ue.Pei,
		CipheringAlgorithm: ue.cipheringAlgName(),
		IntegrityAlgorithm: ue.integrityAlgName(),
		LastSeenAt:         ue.LastSeenAt,
		LastSeenRadio:      ue.LastSeenRadio,
	}

	return snap
}

// Kamf Derivation function defined in TS 33.501 Annex A.7
func (ue *UeContext) DerivateKamf(kseaf string) error {
	if !ue.supi.IsValid() || !ue.supi.IsIMSI() {
		return fmt.Errorf("supi is not a valid IMSI")
	}

	P0 := []byte(ue.supi.IMSI())
	L0 := ueauth.KDFLen(P0)
	P1 := ue.ABBA
	L1 := ueauth.KDFLen(P1)

	kSeafDecode, err := hex.DecodeString(kseaf)
	if err != nil {
		return fmt.Errorf("could not decode kseaf: %v", err)
	}

	kAmfBytes, err := ueauth.GetKDFValue(kSeafDecode, ueauth.FCForKamfDerivation, P0, L0, P1, L1)
	if err != nil {
		return fmt.Errorf("could not get kdf value: %v", err)
	}

	ue.Kamf = hex.EncodeToString(kAmfBytes)

	return nil
}

// Algorithm key Derivation function defined in TS 33.501 Annex A.9
func (ue *UeContext) DerivateAlgKey() error {
	// Security Key
	P0 := []byte{security.NNASEncAlg}
	L0 := ueauth.KDFLen(P0)
	P1 := []byte{ue.CipheringAlg}
	L1 := ueauth.KDFLen(P1)

	kAmfBytes, err := hex.DecodeString(ue.Kamf)
	if err != nil {
		return fmt.Errorf("decode kamf error: %v", err)
	}

	kenc, err := ueauth.GetKDFValue(kAmfBytes, ueauth.FCForAlgorithmKeyDerivation, P0, L0, P1, L1)
	if err != nil {
		return fmt.Errorf("get kdf value error: %v", err)
	}

	copy(ue.KnasEnc[:], kenc[16:32])

	// Integrity Key
	P0 = []byte{security.NNASIntAlg}
	L0 = ueauth.KDFLen(P0)
	P1 = []byte{ue.IntegrityAlg}
	L1 = ueauth.KDFLen(P1)

	kint, err := ueauth.GetKDFValue(kAmfBytes, ueauth.FCForAlgorithmKeyDerivation, P0, L0, P1, L1)
	if err != nil {
		return fmt.Errorf("get kdf value error: %v", err)
	}

	copy(ue.KnasInt[:], kint[16:32])

	return nil
}

// Access Network key Derivation function defined in TS 33.501 Annex A.9
func (ue *UeContext) DerivateAnKey() error {
	P0 := make([]byte, 4)
	binary.BigEndian.PutUint32(P0, ue.ULCount.Get())
	L0 := ueauth.KDFLen(P0)
	P1 := []byte{security.AccessType3GPP}
	L1 := ueauth.KDFLen(P1)

	kAmfBytes, err := hex.DecodeString(ue.Kamf)
	if err != nil {
		return fmt.Errorf("could not decode kamf: %v", err)
	}

	key, err := ueauth.GetKDFValue(kAmfBytes, ueauth.FCForKgnbKn3iwfDerivation, P0, L0, P1, L1)
	if err != nil {
		return fmt.Errorf("could not get kdf value: %v", err)
	}

	ue.Kgnb = key

	return nil
}

// NH Derivation function defined in TS 33.501 Annex A.10
func (ue *UeContext) DerivateNH(syncInput []byte) error {
	P0 := syncInput
	L0 := ueauth.KDFLen(P0)

	kAmfBytes, err := hex.DecodeString(ue.Kamf)
	if err != nil {
		return fmt.Errorf("could not decode kamf: %v", err)
	}

	nh, err := ueauth.GetKDFValue(kAmfBytes, ueauth.FCForNhDerivation, P0, L0)
	if err != nil {
		return fmt.Errorf("could not get kdf value: %v", err)
	}

	ue.NH = nh

	return nil
}

func (ue *UeContext) UpdateSecurityContext() error {
	err := ue.DerivateAnKey()
	if err != nil {
		return fmt.Errorf("error deriving AnKey: %v", err)
	}

	err = ue.DerivateNH(ue.Kgnb)
	if err != nil {
		return fmt.Errorf("error deriving NH: %v", err)
	}

	ue.NCC = 1

	return nil
}

func (ue *UeContext) UpdateNH() error {
	ue.NCC = (ue.NCC + 1) % 8

	err := ue.DerivateNH(ue.NH)
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
			ue.IntegrityAlg = intAlg
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
			ue.CipheringAlg = encAlg
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

	ue.Mutex.Lock()
	defer ue.Mutex.Unlock()

	ue.SmContextList[pduSessionID] = &SmContext{
		Ref:    ref,
		Snssai: snssai,
	}

	return nil
}

func (ue *UeContext) DeleteSmContext(pduSessionID uint8) {
	ue.Mutex.Lock()
	defer ue.Mutex.Unlock()

	delete(ue.SmContextList, pduSessionID)
}

func (ue *UeContext) SmContextFindByPDUSessionID(pduSessionID uint8) (*SmContext, bool) {
	ue.Mutex.Lock()
	defer ue.Mutex.Unlock()

	smContext, ok := ue.SmContextList[pduSessionID]

	return smContext, ok
}

func (ue *UeContext) SetSmContextInactive(pduSessionID uint8) {
	ue.Mutex.Lock()
	defer ue.Mutex.Unlock()

	if sc, ok := ue.SmContextList[pduSessionID]; ok {
		sc.PduSessionInactive = true
	}
}

func (ue *UeContext) HasActivePduSessions() bool {
	ue.Mutex.Lock()
	defer ue.Mutex.Unlock()

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

	ue.Mutex.Lock()
	defer ue.Mutex.Unlock()

	// Plain NAS message
	if !ue.SecurityContextAvailable {
		return msg.PlainNasEncode()
	}

	// Security protected NAS Message
	// a security protected NAS message must be integrity protected, and ciphering is optional
	needCiphering := false

	switch msg.SecurityHeaderType {
	case nas.SecurityHeaderTypeIntegrityProtected:
	case nas.SecurityHeaderTypeIntegrityProtectedAndCiphered:
		needCiphering = true
	case nas.SecurityHeaderTypeIntegrityProtectedWithNew5gNasSecurityContext:
		ue.ULCount.Set(0, 0)
		ue.DLCount.Set(0, 0)
	default:
		return nil, fmt.Errorf("wrong security header type: 0x%0x", msg.SecurityHeaderType)
	}

	// encode plain nas first
	payload, err := msg.PlainNasEncode()
	if err != nil {
		return nil, fmt.Errorf("error encoding plain nas: %+v", err)
	}

	if needCiphering {
		if err = security.NASEncrypt(ue.CipheringAlg, ue.KnasEnc, ue.DLCount.Get(), security.Bearer3GPP, security.DirectionDownlink, payload); err != nil {
			return nil, fmt.Errorf("error encrypting: %+v", err)
		}
	}

	// add sequece number
	payload = append([]byte{ue.DLCount.SQN()}, payload[:]...)

	mac32, err := security.NASMacCalculate(ue.IntegrityAlg, ue.KnasInt, ue.DLCount.Get(), security.Bearer3GPP, security.DirectionDownlink, payload)
	if err != nil {
		return nil, fmt.Errorf("MAC calcuate error: %+v", err)
	}

	// Add mac value
	payload = append(mac32, payload[:]...)

	// Add EPD and Security Type
	msgSecurityHeader := []byte{msg.ProtocolDiscriminator, msg.SecurityHeaderType}
	payload = append(msgSecurityHeader, payload[:]...)

	// Increase DL Count
	ue.DLCount.AddOne()

	return payload, nil
}

func (ue *UeContext) ResetMobileReachableTimer() {
	ue.Mutex.Lock()
	defer ue.Mutex.Unlock()

	if ue.ImplicitDeregistrationTimer != nil {
		ue.ImplicitDeregistrationTimer.Stop()
		ue.ImplicitDeregistrationTimer = nil
	}

	ue.Log.Debug("starting mobile reachable timer", logger.SUPI(ue.supi.String()))

	ue.MobileReachableTimer = NewTimer(
		ue.T3512Value+(4*time.Minute),
		1,
		func(expireTimes int32) {
			ue.Log.Debug("mobile reachable timer expired", logger.SUPI(ue.supi.String()))
			ue.startImplicitDeregistrationTimer()
		},
		func() {},
	)
}

func (ue *UeContext) StopMobileReachableTimer() {
	ue.Mutex.Lock()
	defer ue.Mutex.Unlock()

	ue.Log.Debug("stopping mobile reachable timer")

	if ue.MobileReachableTimer != nil {
		ue.MobileReachableTimer.Stop()
		ue.MobileReachableTimer = nil
	}
}

func (ue *UeContext) StopImplicitDeregistrationTimer() {
	ue.Mutex.Lock()
	defer ue.Mutex.Unlock()

	ue.Log.Debug("stopping implicit deregistration timer")

	if ue.ImplicitDeregistrationTimer != nil {
		ue.ImplicitDeregistrationTimer.Stop()
		ue.ImplicitDeregistrationTimer = nil
	}
}

func (ue *UeContext) startImplicitDeregistrationTimer() {
	ue.Mutex.Lock()
	defer ue.Mutex.Unlock()

	ue.Log.Debug("starting implicit deregistration timer")

	ue.ImplicitDeregistrationTimer = NewTimer(2*time.Minute, 1, func(expireTimes int32) { ue.Deregister(context.Background()) }, func() {})
}

func (ue *UeContext) StopProcedureTimers() {
	ue.Mutex.Lock()
	defer ue.Mutex.Unlock()

	conn := ue.NasConn()
	if conn == nil {
		return
	}

	for _, t := range []*Timer{conn.T3513, conn.T3565, conn.T3560, conn.T3550, conn.T3555, conn.T3522} {
		if t != nil {
			t.Stop()
		}
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
	ue.Mutex.Lock()
	defer ue.Mutex.Unlock()

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
	ue.Mutex.Lock()

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
	ue.Mutex.Unlock()

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
	ue.Mutex.Lock()

	smContextRefs := make([]string, 0, len(ue.SmContextList))
	for _, smContext := range ue.SmContextList {
		smContextRefs = append(smContextRefs, smContext.Ref)
	}

	ue.SmContextList = make(map[uint8]*SmContext)
	ue.Mutex.Unlock()

	for _, smContextRef := range smContextRefs {
		err := ue.smf.ReleaseSmContext(ctx, smContextRef)
		if err != nil {
			ue.Log.Error("Release SmContext Error", zap.Error(err))
		}
	}
}
