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

type AmfUe struct {
	Mutex sync.RWMutex

	/* Gmm State */
	state StateType
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
	ranUe         *RanUe

	smf SmfSbi

	Log *zap.Logger

	current atomic.Pointer[FivegmmContext]
}

func NewAmfUe() *AmfUe {
	ue := &AmfUe{
		state: Deregistered,
	}

	fc := newFivegmmContext(ue)
	fc.active.Store(newActiveNasConnection(fc, nil))
	ue.current.Store(fc)

	return ue
}

func (ue *AmfUe) Current() *FivegmmContext {
	if ue == nil {
		return nil
	}

	return ue.current.Load()
}

// NasConn returns the active NAS connection of the current 5GMM context,
// or nil if no NAS connection is established.
func (ue *AmfUe) NasConn() *ActiveNasConnection {
	fc := ue.Current()
	if fc == nil {
		return nil
	}

	return fc.active.Load()
}

// SwapContext atomically replaces the active 5GMM context. The previous
// context is closed: its ActiveNasConnection is released and its ctx is
// cancelled, unwinding procedures and in-flight RPCs derived from it.
func (ue *AmfUe) SwapContext(fresh *FivegmmContext) *FivegmmContext {
	old := ue.current.Swap(fresh)
	if old != nil {
		old.close()
	}

	return fresh
}

// RotateContext installs a fresh 5GMM context and returns it. Used when
// a UE re-initiates registration: the prior context is discarded per
// TS 24.501 handling of an initial registration arriving in 5GMM-REGISTERED.
// RotateContext installs a fresh 5GMM context, replacing the previous one.
// The NAS connection is intentionally left unset: AttachRanUe creates it once
// the RanUe is known.
func (ue *AmfUe) RotateContext() *FivegmmContext {
	fresh := newFivegmmContext(ue)
	ue.SwapContext(fresh)

	return fresh
}

// AttachNasConnection installs a fresh NAS connection on the current
// 5GMM context, replacing any prior one.
func (ue *AmfUe) AttachNasConnection(ranUe *RanUe) *ActiveNasConnection {
	fc := ue.current.Load()
	if fc == nil {
		fc = newFivegmmContext(ue)
		if !ue.current.CompareAndSwap(nil, fc) {
			fc = ue.current.Load()
		}
	}

	conn := newActiveNasConnection(fc, ranUe)
	if old := fc.active.Swap(conn); old != nil {
		old.stopTimers()
		old.cancel()
	}

	return conn
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

	if fc := ue.current.Load(); fc != nil && fc.active.Load() == nil {
		fc.active.Store(newActiveNasConnection(fc, ranUe))
	}
}

func (ue *AmfUe) AllocateRegistrationArea(supportedTais []models.Tai) {
	// clear the previous registration area if need
	if len(ue.Current().RegistrationArea) > 0 {
		ue.Current().RegistrationArea = nil
	}

	taiList := make([]models.Tai, len(supportedTais))
	copy(taiList, supportedTais)

	for _, supportTai := range taiList {
		if supportTai.Equal(ue.Tai) {
			ue.Current().RegistrationArea = append(ue.Current().RegistrationArea, supportTai)
			break
		}
	}
}

func (ue *AmfUe) IsAllowedNssai(targetSNssai *models.Snssai) bool {
	for _, s := range ue.Current().AllowedNssai {
		if s.Equal(*targetSNssai) {
			return true
		}
	}

	return false
}

func (ue *AmfUe) SecurityContextIsValid() bool {
	return ue.Current().SecurityContextAvailable && ue.Current().NgKsi.Ksi != nasMessage.NasKeySetIdentifierNoKeyIsAvailable
}

// cipheringAlgName returns the human-readable name for the negotiated NAS ciphering algorithm.
// Must be called while holding ue.Mutex.
func (ue *AmfUe) cipheringAlgName() string {
	switch ue.Current().CipheringAlg {
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
	switch ue.Current().IntegrityAlg {
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
	P1 := ue.Current().ABBA
	L1 := ueauth.KDFLen(P1)

	kSeafDecode, err := hex.DecodeString(kseaf)
	if err != nil {
		return fmt.Errorf("could not decode kseaf: %v", err)
	}

	kAmfBytes, err := ueauth.GetKDFValue(kSeafDecode, ueauth.FCForKamfDerivation, P0, L0, P1, L1)
	if err != nil {
		return fmt.Errorf("could not get kdf value: %v", err)
	}

	ue.Current().Kamf = hex.EncodeToString(kAmfBytes)

	return nil
}

// Algorithm key Derivation function defined in TS 33.501 Annex A.9
func (ue *AmfUe) DerivateAlgKey() error {
	// Security Key
	P0 := []byte{security.NNASEncAlg}
	L0 := ueauth.KDFLen(P0)
	P1 := []byte{ue.Current().CipheringAlg}
	L1 := ueauth.KDFLen(P1)

	kAmfBytes, err := hex.DecodeString(ue.Current().Kamf)
	if err != nil {
		return fmt.Errorf("decode kamf error: %v", err)
	}

	kenc, err := ueauth.GetKDFValue(kAmfBytes, ueauth.FCForAlgorithmKeyDerivation, P0, L0, P1, L1)
	if err != nil {
		return fmt.Errorf("get kdf value error: %v", err)
	}

	copy(ue.Current().KnasEnc[:], kenc[16:32])

	// Integrity Key
	P0 = []byte{security.NNASIntAlg}
	L0 = ueauth.KDFLen(P0)
	P1 = []byte{ue.Current().IntegrityAlg}
	L1 = ueauth.KDFLen(P1)

	kint, err := ueauth.GetKDFValue(kAmfBytes, ueauth.FCForAlgorithmKeyDerivation, P0, L0, P1, L1)
	if err != nil {
		return fmt.Errorf("get kdf value error: %v", err)
	}

	copy(ue.Current().KnasInt[:], kint[16:32])

	return nil
}

// Access Network key Derivation function defined in TS 33.501 Annex A.9
func (ue *AmfUe) DerivateAnKey() error {
	P0 := make([]byte, 4)
	binary.BigEndian.PutUint32(P0, ue.Current().ULCount.Get())
	L0 := ueauth.KDFLen(P0)
	P1 := []byte{security.AccessType3GPP}
	L1 := ueauth.KDFLen(P1)

	kAmfBytes, err := hex.DecodeString(ue.Current().Kamf)
	if err != nil {
		return fmt.Errorf("could not decode kamf: %v", err)
	}

	key, err := ueauth.GetKDFValue(kAmfBytes, ueauth.FCForKgnbKn3iwfDerivation, P0, L0, P1, L1)
	if err != nil {
		return fmt.Errorf("could not get kdf value: %v", err)
	}

	ue.Current().Kgnb = key

	return nil
}

// NH Derivation function defined in TS 33.501 Annex A.10
func (ue *AmfUe) DerivateNH(syncInput []byte) error {
	P0 := syncInput
	L0 := ueauth.KDFLen(P0)

	kAmfBytes, err := hex.DecodeString(ue.Current().Kamf)
	if err != nil {
		return fmt.Errorf("could not decode kamf: %v", err)
	}

	nh, err := ueauth.GetKDFValue(kAmfBytes, ueauth.FCForNhDerivation, P0, L0)
	if err != nil {
		return fmt.Errorf("could not get kdf value: %v", err)
	}

	ue.Current().NH = nh

	return nil
}

func (ue *AmfUe) UpdateSecurityContext() error {
	err := ue.DerivateAnKey()
	if err != nil {
		return fmt.Errorf("error deriving AnKey: %v", err)
	}

	err = ue.DerivateNH(ue.Current().Kgnb)
	if err != nil {
		return fmt.Errorf("error deriving NH: %v", err)
	}

	ue.Current().NCC = 1

	return nil
}

func (ue *AmfUe) UpdateNH() error {
	ue.Current().NCC = (ue.Current().NCC + 1) % 8

	err := ue.DerivateNH(ue.Current().NH)
	if err != nil {
		return fmt.Errorf("error deriving NH: %v", err)
	}

	return nil
}

func (ue *AmfUe) SelectSecurityAlg(intOrder, encOrder []uint8) error {
	if ue.Current().UESecurityCapability == nil {
		return fmt.Errorf("UE security capability not available, cannot negotiate NAS security algorithms")
	}

	intFound := false
	ueSupported := uint8(0)

	for _, intAlg := range intOrder {
		switch intAlg {
		case security.AlgIntegrity128NIA0:
			ueSupported = ue.Current().UESecurityCapability.GetIA0_5G()
		case security.AlgIntegrity128NIA1:
			ueSupported = ue.Current().UESecurityCapability.GetIA1_128_5G()
		case security.AlgIntegrity128NIA2:
			ueSupported = ue.Current().UESecurityCapability.GetIA2_128_5G()
		case security.AlgIntegrity128NIA3:
			ueSupported = ue.Current().UESecurityCapability.GetIA3_128_5G()
		}

		if ueSupported == 1 {
			ue.Current().IntegrityAlg = intAlg
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
			ueSupported = ue.Current().UESecurityCapability.GetEA0_5G()
		case security.AlgCiphering128NEA1:
			ueSupported = ue.Current().UESecurityCapability.GetEA1_128_5G()
		case security.AlgCiphering128NEA2:
			ueSupported = ue.Current().UESecurityCapability.GetEA2_128_5G()
		case security.AlgCiphering128NEA3:
			ueSupported = ue.Current().UESecurityCapability.GetEA3_128_5G()
		}

		if ueSupported == 1 {
			ue.Current().CipheringAlg = encAlg
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
func (ue *AmfUe) ClearRegistrationRequestData() {
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

func (ue *AmfUe) ClearRegistrationData(ctx context.Context) {
	ue.releaseSmContexts(ctx)

	ue.Current().SmContextList = make(map[uint8]*SmContext)
}

func (ue *AmfUe) CreateSmContext(pduSessionID uint8, ref string, snssai *models.Snssai) error {
	if pduSessionID < 1 || pduSessionID > 15 {
		return fmt.Errorf("invalid PDU session ID %d: must be in range 1-15 per TS 24.501", pduSessionID)
	}

	ue.Mutex.Lock()
	defer ue.Mutex.Unlock()

	ue.Current().SmContextList[pduSessionID] = &SmContext{
		Ref:    ref,
		Snssai: snssai,
	}

	return nil
}

func (ue *AmfUe) DeleteSmContext(pduSessionID uint8) {
	ue.Mutex.Lock()
	defer ue.Mutex.Unlock()

	delete(ue.Current().SmContextList, pduSessionID)
}

func (ue *AmfUe) SmContextFindByPDUSessionID(pduSessionID uint8) (*SmContext, bool) {
	ue.Mutex.Lock()
	defer ue.Mutex.Unlock()

	smContext, ok := ue.Current().SmContextList[pduSessionID]

	return smContext, ok
}

func (ue *AmfUe) SetSmContextInactive(pduSessionID uint8) {
	ue.Mutex.Lock()
	defer ue.Mutex.Unlock()

	if sc, ok := ue.Current().SmContextList[pduSessionID]; ok {
		sc.PduSessionInactive = true
	}
}

func (ue *AmfUe) HasActivePduSessions() bool {
	ue.Mutex.Lock()
	defer ue.Mutex.Unlock()

	for _, smContext := range ue.Current().SmContextList {
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
	if !ue.Current().SecurityContextAvailable {
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
		ue.Current().ULCount.Set(0, 0)
		ue.Current().DLCount.Set(0, 0)
	default:
		return nil, fmt.Errorf("wrong security header type: 0x%0x", msg.SecurityHeaderType)
	}

	// encode plain nas first
	payload, err := msg.PlainNasEncode()
	if err != nil {
		return nil, fmt.Errorf("error encoding plain nas: %+v", err)
	}

	if needCiphering {
		if err = security.NASEncrypt(ue.Current().CipheringAlg, ue.Current().KnasEnc, ue.Current().DLCount.Get(), security.Bearer3GPP, security.DirectionDownlink, payload); err != nil {
			return nil, fmt.Errorf("error encrypting: %+v", err)
		}
	}

	// add sequece number
	payload = append([]byte{ue.Current().DLCount.SQN()}, payload[:]...)

	mac32, err := security.NASMacCalculate(ue.Current().IntegrityAlg, ue.Current().KnasInt, ue.Current().DLCount.Get(), security.Bearer3GPP, security.DirectionDownlink, payload)
	if err != nil {
		return nil, fmt.Errorf("MAC calcuate error: %+v", err)
	}

	// Add mac value
	payload = append(mac32, payload[:]...)

	// Add EPD and Security Type
	msgSecurityHeader := []byte{msg.ProtocolDiscriminator, msg.SecurityHeaderType}
	payload = append(msgSecurityHeader, payload[:]...)

	// Increase DL Count
	ue.Current().DLCount.AddOne()

	return payload, nil
}

func (ue *AmfUe) ResetMobileReachableTimer() {
	ue.Mutex.Lock()
	defer ue.Mutex.Unlock()

	if ue.Current().ImplicitDeregistrationTimer != nil {
		ue.Current().ImplicitDeregistrationTimer.Stop()
		ue.Current().ImplicitDeregistrationTimer = nil
	}

	ue.Log.Debug("starting mobile reachable timer", logger.SUPI(ue.Supi.String()))

	ue.Current().MobileReachableTimer = NewTimer(
		ue.Current().T3512Value+(4*time.Minute),
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

	if ue.Current().MobileReachableTimer != nil {
		ue.Current().MobileReachableTimer.Stop()
		ue.Current().MobileReachableTimer = nil
	}
}

func (ue *AmfUe) StopImplicitDeregistrationTimer() {
	ue.Mutex.Lock()
	defer ue.Mutex.Unlock()

	ue.Log.Debug("stopping implicit deregistration timer")

	if ue.Current().ImplicitDeregistrationTimer != nil {
		ue.Current().ImplicitDeregistrationTimer.Stop()
		ue.Current().ImplicitDeregistrationTimer = nil
	}
}

func (ue *AmfUe) startImplicitDeregistrationTimer() {
	ue.Mutex.Lock()
	defer ue.Mutex.Unlock()

	ue.Log.Debug("starting implicit deregistration timer")

	ue.Current().ImplicitDeregistrationTimer = NewTimer(2*time.Minute, 1, func(expireTimes int32) { ue.Deregister(context.Background()) }, func() {})
}

func (ue *AmfUe) StopProcedureTimers() {
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
func (ue *AmfUe) ReleaseNasConnection(target *RanUe) {
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

func (ue *AmfUe) detachRanUeIfMatch(target *RanUe) bool {
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
func (ue *AmfUe) stopAllTimersLocked() {
	fc := ue.Current()
	if fc == nil {
		return
	}

	if fc.ImplicitDeregistrationTimer != nil {
		fc.ImplicitDeregistrationTimer.Stop()
		fc.ImplicitDeregistrationTimer = nil
	}

	if fc.MobileReachableTimer != nil {
		fc.MobileReachableTimer.Stop()
		fc.MobileReachableTimer = nil
	}
}

func (ue *AmfUe) Deregister(ctx context.Context) {
	ue.Mutex.Lock()

	if conn := ue.NasConn(); conn != nil {
		conn.Release()
	}

	ue.stopAllTimersLocked()

	ue.transitionToLocked(Deregistered)

	// Copy refs and clear map while protected by UE lock.
	smContextRefs := make([]string, 0, len(ue.Current().SmContextList))
	for _, smContext := range ue.Current().SmContextList {
		smContextRefs = append(smContextRefs, smContext.Ref)
	}

	ue.Current().SmContextList = make(map[uint8]*SmContext)
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

	smContextRefs := make([]string, 0, len(ue.Current().SmContextList))
	for _, smContext := range ue.Current().SmContextList {
		smContextRefs = append(smContextRefs, smContext.Ref)
	}

	ue.Current().SmContextList = make(map[uint8]*SmContext)
	ue.Mutex.Unlock()

	for _, smContextRef := range smContextRefs {
		err := ue.smf.ReleaseSmContext(ctx, smContextRef)
		if err != nil {
			ue.Log.Error("Release SmContext Error", zap.Error(err))
		}
	}
}
