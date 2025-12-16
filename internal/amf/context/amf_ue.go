// Copyright 2024 Ella Networks
// SPDX-FileCopyrightText: 2022-present Intel Corporation
// SPDX-FileCopyrightText: 2021 Open Networking Foundation <info@opennetworking.org>
// Copyright 2019 free5GC.org
//
// SPDX-License-Identifier: Apache-2.0

package context

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"reflect"
	"regexp"
	"sync"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/util/ueauth"
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
	OnGoingProcedureAbort        OnGoingProcedure = "Abort"
)

const (
	NgRanCgiPresentNRCGI    int32 = 0
	NgRanCgiPresentEUTRACGI int32 = 1
)

const (
	RecommendRanNodePresentRanNode int32 = 0
	RecommendRanNodePresentTAI     int32 = 1
)

// GMM state for UE
const (
	Deregistered            StateType = "Deregistered"
	DeregistrationInitiated StateType = "DeregistrationInitiated"
	Authentication          StateType = "Authentication"
	SecurityMode            StateType = "SecurityMode"
	ContextSetup            StateType = "ContextSetup"
	Registered              StateType = "Registered"
)

type AmfUe struct {
	Mutex sync.Mutex

	/* Gmm State */
	State *State
	/* Registration procedure related context */
	RegistrationType5GS             uint8
	IdentityTypeUsedForRegistration uint8
	RegistrationRequest             *nasMessage.RegistrationRequest
	RetransmissionOfInitialNASMsg   bool
	/* Ue Identity*/
	PlmnID  models.PlmnID
	Suci    string
	Supi    string
	Pei     string
	Tmsi    int32
	OldTmsi int32
	Guti    string
	OldGuti string
	/* User Location*/
	Location                 models.UserLocation
	Tai                      models.Tai
	LastVisitedRegisteredTai models.Tai
	TimeZone                 string
	/* context about udm */
	Dnn                               string
	Ambr                              *models.Ambr
	AuthenticationCtx                 *models.Av5gAka
	AuthFailureCauseSynchFailureTimes int
	ABBA                              []uint8
	Kseaf                             string
	Kamf                              string
	N1N2Message                       *models.N1N2MessageTransferRequest
	SmContextList                     map[uint8]*SmContext // Key: pdu session id
	RanUe                             *RanUe
	OnGoing                           *OnGoingProcedureWithPrio
	UeRadioCapability                 string // OCTET string

	/* context related to Paging */
	UeRadioCapabilityForPaging                 *UERadioCapabilityForPaging
	InfoOnRecommendedCellsAndRanNodesForPaging *InfoOnRecommendedCellsAndRanNodesForPaging
	UESpecificDRX                              uint8

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

	AllowedNssai *models.Snssai

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
	T3502Value int // Second
	T3512Value int // default 54 min

	Log *zap.Logger
}

type OnGoingProcedureWithPrio struct {
	Procedure OnGoingProcedure
}

type UERadioCapabilityForPaging struct {
	NR    string // OCTET string
	EUTRA string // OCTET string
}

// TS 38.413 9.3.1.100
type InfoOnRecommendedCellsAndRanNodesForPaging struct {
	RecommendedCells []RecommendedCell // RecommendedCellsForPaging
}

// TS 38.413 9.3.1.71
type RecommendedCell struct {
	NgRanCGI         NGRANCGI
	TimeStayedInCell *int64
}

type NGRANCGI struct {
	Present  int32
	NRCGI    *models.Ncgi
	EUTRACGI *models.Ecgi
}

// TS 24.501 8.2.19
type ConfigurationUpdateCommandFlags struct {
	NeedGUTI                                     bool
	NeedNITZ                                     bool
	NeedTaiList                                  bool
	NeedRejectNSSAI                              bool
	NeedAllowedNSSAI                             bool
	NeedSmsIndication                            bool
	NeedMicoIndication                           bool
	NeedLadnInformation                          bool
	NeedServiceAreaList                          bool
	NeedConfiguredNSSAI                          bool
	NeedNetworkSlicingIndication                 bool
	NeedOperatordefinedAccessCategoryDefinitions bool
}

func (ue *AmfUe) init() {
	ue.State = NewState(Deregistered)
	ue.RegistrationArea = make([]models.Tai, 0)
	ue.OnGoing = new(OnGoingProcedureWithPrio)
	ue.OnGoing.Procedure = OnGoingProcedureNothing
	ue.SmContextList = make(map[uint8]*SmContext)
}

func (ue *AmfUe) Remove() {
	if ue.RanUe != nil {
		err := ue.RanUe.Remove()
		if err != nil {
			logger.AmfLog.Error("failed to remove RAN UE", zap.Error(err))
		}
	}

	tmsiGenerator.FreeID(int64(ue.Tmsi))

	if len(ue.Supi) > 0 {
		amfCtxt := AMFSelf()
		amfCtxt.Mutex.Lock()
		delete(amfCtxt.UePool, ue.Supi)
		amfCtxt.Mutex.Unlock()
	}
}

func (ue *AmfUe) DetachRanUe() {
	ue.RanUe = nil
}

func (ue *AmfUe) AttachRanUe(ranUe *RanUe) {
	if ranUe == nil || ranUe.Ran == nil {
		return
	}
	ue.Mutex.Lock()
	defer ue.Mutex.Unlock()

	oldRanUe := ue.RanUe

	ue.RanUe = ranUe

	ranUe.AmfUe = ue

	if oldRanUe != nil && oldRanUe != ranUe {
		if oldRanUe.AmfUe == ue {
			oldRanUe.Log.Info("Detached UeContext from previous RanUe")
			oldRanUe.AmfUe = nil
		}
	}

	ue.Log = logger.AmfLog.With(zap.String("AMF_UE_NGAP_ID", fmt.Sprintf("AMF_UE_NGAP_ID:%d", ranUe.AmfUeNgapID)))
}

func (ue *AmfUe) InAllowedNssai(targetSNssai *models.Snssai) bool {
	return reflect.DeepEqual(*ue.AllowedNssai, *targetSNssai)
}

func (ue *AmfUe) SecurityContextIsValid() bool {
	return ue.SecurityContextAvailable && ue.NgKsi.Ksi != nasMessage.NasKeySetIdentifierNoKeyIsAvailable && !ue.MacFailed
}

// Kamf Derivation function defined in TS 33.501 Annex A.7
func (ue *AmfUe) DerivateKamf() {
	supiRegexp, err := regexp.Compile("([0-9]{5,15})")
	if err != nil {
		logger.AmfLog.Error("compile supi regexp error", zap.Error(err))
		return
	}
	groups := supiRegexp.FindStringSubmatch(ue.Supi)
	if groups == nil {
		logger.AmfLog.Error("supi is not correct")
		return
	}

	P0 := []byte(groups[1])
	L0 := ueauth.KDFLen(P0)
	P1 := ue.ABBA
	L1 := ueauth.KDFLen(P1)

	KseafDecode, err := hex.DecodeString(ue.Kseaf)
	if err != nil {
		logger.AmfLog.Error("decode kseaf error", zap.Error(err))
		return
	}
	KamfBytes, err := ueauth.GetKDFValue(KseafDecode, ueauth.FCForKamfDerivation, P0, L0, P1, L1)
	if err != nil {
		logger.AmfLog.Error("get kdf value error", zap.Error(err))
		return
	}
	ue.Kamf = hex.EncodeToString(KamfBytes)
}

// Algorithm key Derivation function defined in TS 33.501 Annex A.9
func (ue *AmfUe) DerivateAlgKey() error {
	// Security Key
	P0 := []byte{security.NNASEncAlg}
	L0 := ueauth.KDFLen(P0)
	P1 := []byte{ue.CipheringAlg}
	L1 := ueauth.KDFLen(P1)

	KamfBytes, err := hex.DecodeString(ue.Kamf)
	if err != nil {
		return fmt.Errorf("decode kamf error: %v", err)
	}

	kenc, err := ueauth.GetKDFValue(KamfBytes, ueauth.FCForAlgorithmKeyDerivation, P0, L0, P1, L1)
	if err != nil {
		return fmt.Errorf("get kdf value error: %v", err)
	}

	copy(ue.KnasEnc[:], kenc[16:32])

	// Integrity Key
	P0 = []byte{security.NNASIntAlg}
	L0 = ueauth.KDFLen(P0)
	P1 = []byte{ue.IntegrityAlg}
	L1 = ueauth.KDFLen(P1)

	kint, err := ueauth.GetKDFValue(KamfBytes, ueauth.FCForAlgorithmKeyDerivation, P0, L0, P1, L1)
	if err != nil {
		return fmt.Errorf("get kdf value error: %v", err)
	}

	copy(ue.KnasInt[:], kint[16:32])

	return nil
}

// Access Network key Derivation function defined in TS 33.501 Annex A.9
func (ue *AmfUe) DerivateAnKey() {
	P0 := make([]byte, 4)
	binary.BigEndian.PutUint32(P0, ue.ULCount.Get())
	L0 := ueauth.KDFLen(P0)
	P1 := []byte{security.AccessType3GPP}
	L1 := ueauth.KDFLen(P1)

	KamfBytes, err := hex.DecodeString(ue.Kamf)
	if err != nil {
		logger.AmfLog.Error("decode kamf error", zap.Error(err))
		return
	}

	key, err := ueauth.GetKDFValue(KamfBytes, ueauth.FCForKgnbKn3iwfDerivation, P0, L0, P1, L1)
	if err != nil {
		logger.AmfLog.Error("get kdf value error", zap.Error(err))
		return
	}

	ue.Kgnb = key
}

// NH Derivation function defined in TS 33.501 Annex A.10
func (ue *AmfUe) DerivateNH(syncInput []byte) {
	P0 := syncInput
	L0 := ueauth.KDFLen(P0)

	KamfBytes, err := hex.DecodeString(ue.Kamf)
	if err != nil {
		logger.AmfLog.Error("decode kamf error", zap.Error(err))
		return
	}
	ue.NH, err = ueauth.GetKDFValue(KamfBytes, ueauth.FCForNhDerivation, P0, L0)
	if err != nil {
		logger.AmfLog.Error("get kdf value error", zap.Error(err))
		return
	}
}

func (ue *AmfUe) UpdateSecurityContext() {
	ue.DerivateAnKey()
	ue.DerivateNH(ue.Kgnb)
	ue.NCC = 1
}

func (ue *AmfUe) UpdateNH() {
	ue.NCC++
	ue.DerivateNH(ue.NH)
}

func (ue *AmfUe) SelectSecurityAlg(intOrder, encOrder []uint8) {
	ue.CipheringAlg = security.AlgCiphering128NEA0
	ue.IntegrityAlg = security.AlgIntegrity128NIA0

	if ue.UESecurityCapability == nil {
		logger.AmfLog.Debug("AMF UE Security Capability is not available")
		return
	}

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
			break
		}
	}

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
			break
		}
	}
}

// this is clearing the transient data of registration request, this is called entrypoint of Deregistration and Registration state
func (ue *AmfUe) ClearRegistrationRequestData() {
	ue.RegistrationRequest = nil
	ue.RegistrationType5GS = 0
	ue.IdentityTypeUsedForRegistration = 0
	ue.AuthFailureCauseSynchFailureTimes = 0
	if ue.RanUe != nil {
		ue.RanUe.UeContextRequest = false
		ue.RanUe.RecvdInitialContextSetupResponse = false
	}
	ue.RetransmissionOfInitialNASMsg = false
	ue.OnGoing.Procedure = OnGoingProcedureNothing
}

// this method called when we are reusing the same uecontext during the registration procedure
func (ue *AmfUe) ClearRegistrationData() {
	ue.SmContextList = make(map[uint8]*SmContext)
}

func (ue *AmfUe) SetOnGoing(onGoing *OnGoingProcedureWithPrio) {
	prevOnGoing := ue.OnGoing
	ue.OnGoing = onGoing
	ue.Log.Debug("set ongoing procedure", zap.Any("ongoingProcedure", onGoing.Procedure), zap.Any("previousOnGoingProcedure", prevOnGoing.Procedure))
}

func (ue *AmfUe) GetOnGoing() OnGoingProcedureWithPrio {
	return *ue.OnGoing
}

func (ue *AmfUe) StoreSmContext(pduSessionID uint8, smContext *SmContext) {
	ue.SmContextList[pduSessionID] = smContext
}

func (ue *AmfUe) SmContextFindByPDUSessionID(pduSessionID uint8) (*SmContext, bool) {
	smContext, ok := ue.SmContextList[pduSessionID]
	return smContext, ok
}

func (ue *AmfUe) HasActivePduSessions() bool {
	for _, smContext := range ue.SmContextList {
		if smContext.IsPduSessionActive() {
			return true
		}
	}

	return false
}
