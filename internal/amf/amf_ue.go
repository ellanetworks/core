// Copyright 2024 Ella Networks
// SPDX-FileCopyrightText: 2022-present Intel Corporation
// SPDX-FileCopyrightText: 2021 Open Networking Foundation <info@opennetworking.org>
// Copyright 2019 free5GC.org
//
// SPDX-License-Identifier: Apache-2.0

package amf

import (
	"context"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"slices"
	"sync"
	"time"

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/ausf"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/util/ueauth"
	"github.com/free5gc/nas"
	"github.com/free5gc/nas/nasMessage"
	"github.com/free5gc/nas/nasType"
	"github.com/free5gc/nas/security"
	"go.uber.org/zap"
)

type OnGoingProcedure string

const (
	OnGoingProcedureNothing      OnGoingProcedure = "Nothing"
	OnGoingProcedurePaging       OnGoingProcedure = "Paging"
	OnGoingProcedureN2Handover   OnGoingProcedure = "N2Handover"
	OnGoingProcedureRegistration OnGoingProcedure = "Registration"
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

type AmfUe struct {
	Mutex sync.RWMutex

	/* Gmm State */
	state StateType
	/* Registration procedure related context */
	RegistrationType5GS             uint8
	IdentityTypeUsedForRegistration uint8
	RegistrationRequest             *nasMessage.RegistrationRequest
	RetransmissionOfInitialNASMsg   bool
	/* Ue Identity*/
	PlmnID  models.PlmnID
	Suci    string
	Supi    etsi.SUPI
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
	/* context about udm */
	Ambr                              *models.Ambr
	AuthenticationCtx                 *ausf.AuthResult
	AuthFailureCauseSynchFailureTimes int
	ABBA                              []uint8
	Kamf                              string
	N1N2Message                       *models.N1N2MessageTransferRequest
	SmContextList                     map[uint8]*SmContext // Key: pdu session id
	ranUe                             *RanUe
	OnGoing                           OnGoingProcedure
	UeRadioCapability                 string // OCTET string

	/* context related to Paging */
	UeRadioCapabilityForPaging *models.UERadioCapabilityForPaging
	UESpecificDRX              uint8

	/* Security Context */
	SecurityContextAvailable bool
	UESecurityCapability     *nasType.UESecurityCapability // for security command
	NgKsi                    models.NgKsi
	MacFailed                bool      // set to true if the integrity check of current NAS message is failed
	KnasInt                  [16]uint8 // 16 byte
	KnasEnc                  [16]uint8 // 16 byte
	Kgnb                     []uint8   // 32 byte
	NH                       []uint8   // 32 byte
	NCC                      uint8     // 0..7
	ULCount                  security.Count
	DLCount                  security.Count
	CipheringAlg             uint8
	IntegrityAlg             uint8

	RegistrationArea []models.Tai

	AllowedNssai []models.Snssai

	/* T3513(Paging) */
	T3513 *Timer // for paging
	/* T3565(Notification) */
	T3565 *Timer // for NAS Notification
	/* T3560 (for authentication request/security mode command retransmission) */
	T3560 *Timer
	/* T3550 (for registration accept retransmission) */
	T3550 *Timer
	/* T3555 (for configuration update command retransmission) */
	T3555 *Timer
	/* T3522 (for deregistration request) */
	T3522 *Timer
	/* T3502 (Assigned by AMF, and used by UE to initialize registration procedure) */
	T3502Value time.Duration
	T3512Value time.Duration
	/* Linked to T3512, should be 4 minutes longer */
	mobileReachableTimer        *Timer
	implicitDeregistrationTimer *Timer

	smf SmfSbi // set by AMF.AddAmfUeToUePool; used by releaseSmContexts

	Log *zap.Logger
}

func NewAmfUe() *AmfUe {
	return &AmfUe{
		state:            Deregistered,
		RegistrationArea: make([]models.Tai, 0),
		OnGoing:          OnGoingProcedureNothing,
		SmContextList:    make(map[uint8]*SmContext),
	}
}

// RanUe returns the currently attached RanUe, or nil.
// The read is synchronized via ue.Mutex so the returned pointer is a
// consistent snapshot.  Callers must capture the result in a local
// variable and reuse it — never call RanUe() twice in the same
// code path, as the underlying pointer may change between calls.
func (ue *AmfUe) RanUe() *RanUe {
	if ue == nil {
		return nil
	}

	ue.Mutex.RLock()
	r := ue.ranUe
	ue.Mutex.RUnlock()

	return r
}

func (ue *AmfUe) GetState() StateType {
	ue.Mutex.Lock()
	defer ue.Mutex.Unlock()

	return ue.state
}

// ForceState sets the UE state unconditionally, bypassing transition validation.
// It exists for test precondition setup in external packages. Production code
// must use TransitionTo.
func (ue *AmfUe) ForceState(s StateType) {
	ue.Mutex.Lock()
	defer ue.Mutex.Unlock()

	ue.state = s
}

func (ue *AmfUe) TransitionTo(target StateType) {
	ue.Mutex.Lock()
	defer ue.Mutex.Unlock()

	ue.transitionToLocked(target)
}

// transitionToLocked enforces allowed state transitions and must only be called while ue.Mutex is held.
func (ue *AmfUe) transitionToLocked(target StateType) {
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

func (ue *AmfUe) AttachRanUe(ranUe *RanUe) {
	if ranUe == nil {
		return
	}

	ue.Mutex.Lock()
	defer ue.Mutex.Unlock()

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
	if ranUe.Radio != nil {
		ue.LastSeenRadio = ranUe.Radio.Name
	}

	ue.Log = logger.AmfLog.With(logger.AmfUeNgapID(ranUe.AmfUeNgapID))
}

// DetachRanUe detaches the given RanUe from this AmfUe. If target is non-nil,
// the detach only proceeds when ue.ranUe still points to target, preventing a
// stale RanUe cleanup from accidentally severing a newer association.
func (ue *AmfUe) DetachRanUe(target *RanUe) {
	if ue == nil {
		return
	}

	ue.Mutex.Lock()
	defer ue.Mutex.Unlock()

	if ue.ranUe == nil {
		return
	}

	if target != nil && ue.ranUe != target {
		logger.AmfLog.Warn("DetachRanUe: current RanUe does not match target, skipping",
			logger.SUPI(ue.Supi.String()),
			zap.Int64("currentAmfUeNgapID", ue.ranUe.AmfUeNgapID),
			zap.Int64("targetAmfUeNgapID", target.AmfUeNgapID),
		)

		return
	}

	if ue.ranUe.amfUe == ue {
		ue.ranUe.amfUe = nil
	}

	ue.ranUe = nil
}

func (ue *AmfUe) AllocateRegistrationArea(supportedTais []models.Tai) {
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

func (ue *AmfUe) IsAllowedNssai(targetSNssai *models.Snssai) bool {
	for _, s := range ue.AllowedNssai {
		if s.Equal(*targetSNssai) {
			return true
		}
	}

	return false
}

func (ue *AmfUe) SecurityContextIsValid() bool {
	return ue.SecurityContextAvailable && ue.NgKsi.Ksi != nasMessage.NasKeySetIdentifierNoKeyIsAvailable && !ue.MacFailed
}

// cipheringAlgName returns the human-readable name for the negotiated NAS ciphering algorithm.
// Must be called while holding ue.Mutex.
func (ue *AmfUe) cipheringAlgName() string {
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
func (ue *AmfUe) integrityAlgName() string {
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
func (ue *AmfUe) TouchLastSeen(radioName string) {
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
func (ue *AmfUe) Snapshot() UESnapshot {
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
func (ue *AmfUe) DerivateKamf(kseaf string) error {
	if !ue.Supi.IsValid() || !ue.Supi.IsIMSI() {
		return fmt.Errorf("supi is not a valid IMSI")
	}

	P0 := []byte(ue.Supi.IMSI())
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
func (ue *AmfUe) DerivateAlgKey() error {
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
func (ue *AmfUe) DerivateAnKey() error {
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
func (ue *AmfUe) DerivateNH(syncInput []byte) error {
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

func (ue *AmfUe) UpdateSecurityContext() error {
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

func (ue *AmfUe) UpdateNH() error {
	ue.NCC = (ue.NCC + 1) % 8

	err := ue.DerivateNH(ue.NH)
	if err != nil {
		return fmt.Errorf("error deriving NH: %v", err)
	}

	return nil
}

func (ue *AmfUe) SelectSecurityAlg(intOrder, encOrder []uint8) error {
	if ue.UESecurityCapability == nil {
		return fmt.Errorf("UE security capability not available, cannot negotiate NAS security algorithms")
	}

	intFound := false
	ueSupported := uint8(0)

	for _, intAlg := range intOrder {
		switch intAlg {
		case security.AlgIntegrity128NIA0:
			ueSupported = ue.UESecurityCapability.GetIA0_5G()
		case security.AlgIntegrity128NIA1:
			ueSupported = ue.UESecurityCapability.GetIA1_128_5G()
		case security.AlgIntegrity128NIA2:
			ueSupported = ue.UESecurityCapability.GetIA2_128_5G()
		case security.AlgIntegrity128NIA3:
			ueSupported = ue.UESecurityCapability.GetIA3_128_5G()
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
			ueSupported = ue.UESecurityCapability.GetEA0_5G()
		case security.AlgCiphering128NEA1:
			ueSupported = ue.UESecurityCapability.GetEA1_128_5G()
		case security.AlgCiphering128NEA2:
			ueSupported = ue.UESecurityCapability.GetEA2_128_5G()
		case security.AlgCiphering128NEA3:
			ueSupported = ue.UESecurityCapability.GetEA3_128_5G()
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

// this is clearing the transient data of registration request, this is called entrypoint of Deregistration and Registration state
func (ue *AmfUe) ClearRegistrationRequestData() {
	ue.RegistrationRequest = nil
	ue.RegistrationType5GS = 0
	ue.IdentityTypeUsedForRegistration = 0

	ue.AuthFailureCauseSynchFailureTimes = 0
	if ue.ranUe != nil {
		ue.ranUe.UeContextRequest = false
		ue.ranUe.RecvdInitialContextSetupResponse = false
	}

	ue.RetransmissionOfInitialNASMsg = false
	ue.OnGoing = OnGoingProcedureNothing
}

func (ue *AmfUe) ClearRegistrationData(ctx context.Context) {
	ue.releaseSmContexts(ctx)

	ue.SmContextList = make(map[uint8]*SmContext)
}

func (ue *AmfUe) SetOnGoing(onGoing OnGoingProcedure) {
	prevOnGoing := ue.OnGoing
	ue.OnGoing = onGoing
	ue.Log.Debug("set ongoing procedure", zap.Any("ongoingProcedure", onGoing), zap.Any("previousOnGoingProcedure", prevOnGoing))
}

func (ue *AmfUe) GetOnGoing() OnGoingProcedure {
	return ue.OnGoing
}

func (ue *AmfUe) CreateSmContext(pduSessionID uint8, ref string, snssai *models.Snssai) error {
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

func (ue *AmfUe) DeleteSmContext(pduSessionID uint8) {
	ue.Mutex.Lock()
	defer ue.Mutex.Unlock()

	delete(ue.SmContextList, pduSessionID)
}

func (ue *AmfUe) SmContextFindByPDUSessionID(pduSessionID uint8) (*SmContext, bool) {
	ue.Mutex.Lock()
	defer ue.Mutex.Unlock()

	smContext, ok := ue.SmContextList[pduSessionID]

	return smContext, ok
}

func (ue *AmfUe) SetSmContextInactive(pduSessionID uint8) {
	ue.Mutex.Lock()
	defer ue.Mutex.Unlock()

	if sc, ok := ue.SmContextList[pduSessionID]; ok {
		sc.PduSessionInactive = true
	}
}

func (ue *AmfUe) HasActivePduSessions() bool {
	ue.Mutex.Lock()
	defer ue.Mutex.Unlock()

	for _, smContext := range ue.SmContextList {
		if !smContext.PduSessionInactive {
			return true
		}
	}

	return false
}

func (ue *AmfUe) EncodeNASMessage(msg *nas.Message) ([]byte, error) {
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

func (ue *AmfUe) ResetMobileReachableTimer() {
	ue.Mutex.Lock()
	defer ue.Mutex.Unlock()

	if ue.implicitDeregistrationTimer != nil {
		ue.implicitDeregistrationTimer.Stop()
		ue.implicitDeregistrationTimer = nil
	}

	ue.Log.Debug("starting mobile reachable timer", logger.SUPI(ue.Supi.String()))

	ue.mobileReachableTimer = NewTimer(
		ue.T3512Value+(4*time.Minute),
		1,
		func(expireTimes int32) {
			ue.Log.Debug("mobile reachable timer expired", logger.SUPI(ue.Supi.String()))
			ue.startImplicitDeregistrationTimer()
		},
		func() {},
	)
}

func (ue *AmfUe) StopMobileReachableTimer() {
	ue.Mutex.Lock()
	defer ue.Mutex.Unlock()

	ue.Log.Debug("stopping mobile reachable timer")

	if ue.mobileReachableTimer != nil {
		ue.mobileReachableTimer.Stop()
		ue.mobileReachableTimer = nil
	}
}

func (ue *AmfUe) StopImplicitDeregistrationTimer() {
	ue.Mutex.Lock()
	defer ue.Mutex.Unlock()

	ue.Log.Debug("stopping implicit deregistration timer")

	if ue.implicitDeregistrationTimer != nil {
		ue.implicitDeregistrationTimer.Stop()
		ue.implicitDeregistrationTimer = nil
	}
}

func (ue *AmfUe) startImplicitDeregistrationTimer() {
	ue.Mutex.Lock()
	defer ue.Mutex.Unlock()

	ue.Log.Debug("starting implicit deregistration timer")

	ue.implicitDeregistrationTimer = NewTimer(2*time.Minute, 1, func(expireTimes int32) { ue.Deregister(context.Background()) }, func() {})
}

// stopAllTimersLocked stops every timer on the UE. Caller must hold ue.Mutex.
func (ue *AmfUe) stopAllTimersLocked() {
	for _, t := range []*Timer{ue.T3513, ue.T3565, ue.T3560, ue.T3550, ue.T3555, ue.T3522} {
		if t != nil {
			t.Stop()
		}
	}

	if ue.implicitDeregistrationTimer != nil {
		ue.implicitDeregistrationTimer.Stop()
		ue.implicitDeregistrationTimer = nil
	}

	if ue.mobileReachableTimer != nil {
		ue.mobileReachableTimer.Stop()
		ue.mobileReachableTimer = nil
	}
}

func (ue *AmfUe) Deregister(ctx context.Context) {
	ue.Mutex.Lock()

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

	ue.Log.Debug("ue deregistered", logger.SUPI(ue.Supi.String()))
}

func (ue *AmfUe) releaseSmContexts(ctx context.Context) {
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
