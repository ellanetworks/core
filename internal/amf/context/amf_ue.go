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
	"github.com/ellanetworks/core/internal/util/fsm"
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
	Deregistered            fsm.StateType = "Deregistered"
	DeregistrationInitiated fsm.StateType = "DeregistrationInitiated"
	Authentication          fsm.StateType = "Authentication"
	SecurityMode            fsm.StateType = "SecurityMode"
	ContextSetup            fsm.StateType = "ContextSetup"
	Registered              fsm.StateType = "Registered"
)

type AmfUe struct {
	Mutex sync.Mutex
	/* the AMF which serving this AmfUe now */
	ServingAMF *AMFContext // never nil

	/* Gmm State */
	State map[models.AccessType]*fsm.State
	/* Registration procedure related context */
	RegistrationType5GS                uint8
	IdentityTypeUsedForRegistration    uint8
	RegistrationRequest                *nasMessage.RegistrationRequest
	ServingAmfChanged                  bool
	DeregistrationTargetAccessType     uint8 // only used when deregistration procedure is initialized by the network
	RegistrationAcceptForNon3GPPAccess []byte
	RetransmissionOfInitialNASMsg      bool
	/* Used for AMF relocation */
	/* Ue Identity*/
	PlmnID  models.PlmnID
	Suci    string
	Supi    string
	Gpsi    string
	Pei     string
	Tmsi    int32
	OldTmsi int32
	Guti    string
	OldGuti string
	EBI     int32
	/* Ue Identity*/
	/* User Location*/
	RatType                  models.RatType
	Location                 models.UserLocation
	Tai                      models.Tai
	LastVisitedRegisteredTai models.Tai
	TimeZone                 string
	/* context about udm */
	SubscriptionDataValid             bool
	Dnn                               string
	TraceData                         *models.TraceData
	SubscribedNssai                   *models.Snssai
	Ambr                              *models.AmbrRm
	RoutingIndicator                  string
	AuthenticationCtx                 *models.UeAuthenticationCtx
	AuthFailureCauseSynchFailureTimes int
	ABBA                              []uint8
	Kseaf                             string
	Kamf                              string
	/* N1N2Message */
	N1N2Message *N1N2Message
	/* Pdu Sesseion context */
	SmContextList sync.Map // map[int32]*SmContext, pdu session id as key
	/* Related Context*/
	RanUe map[models.AccessType]*RanUe
	/* other */
	OnGoing                         map[models.AccessType]*OnGoingProcedureWithPrio
	UeRadioCapability               string // OCTET string
	Capability5GMM                  nasType.Capability5GMM
	ConfigurationUpdateIndication   nasType.ConfigurationUpdateIndication
	ConfigurationUpdateCommandFlags *ConfigurationUpdateCommandFlags
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
	Kn3iwf                   []uint8   // 32 byte
	NH                       []uint8   // 32 byte
	NCC                      uint8     // 0..7
	ULCount                  security.Count
	DLCount                  security.Count
	CipheringAlg             uint8
	IntegrityAlg             uint8
	/* Registration Area */
	RegistrationArea map[models.AccessType][]models.Tai
	/* Network Slicing related context and Nssf */
	AllowedNssai map[models.AccessType]*models.Snssai
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
	/* Ue Context Release Cause */
	ReleaseCause map[models.AccessType]*CauseAll
	/* T3502 (Assigned by AMF, and used by UE to initialize registration procedure) */
	T3502Value                      int // Second
	T3512Value                      int // default 54 min
	Non3gppDeregistrationTimerValue int // default 54 min

	NASLog      *zap.Logger
	GmmLog      *zap.Logger
	TxLog       *zap.Logger
	ProducerLog *zap.Logger
}

type N1N2Message struct {
	Request models.N1N2MessageTransferRequest
	Status  models.N1N2MessageTransferCause
}

type OnGoingProcedureWithPrio struct {
	Procedure OnGoingProcedure
	Ppi       int32 // Paging priority
}

type UERadioCapabilityForPaging struct {
	NR    string // OCTET string
	EUTRA string // OCTET string
}

// TS 38.413 9.3.1.100
type InfoOnRecommendedCellsAndRanNodesForPaging struct {
	RecommendedCells    []RecommendedCell  // RecommendedCellsForPaging
	RecommendedRanNodes []RecommendRanNode // RecommendedRanNodesForPaging
}

// TS 38.413 9.3.1.71
type RecommendedCell struct {
	NgRanCGI         NGRANCGI
	TimeStayedInCell *int64
}

// TS 38.413 9.3.1.101
type RecommendRanNode struct {
	Present         int32
	GlobalRanNodeID *models.GlobalRanNodeID
	Tai             *models.Tai
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
	ue.ServingAMF = AMFSelf()
	ue.State = make(map[models.AccessType]*fsm.State)
	ue.State[models.AccessType3GPPAccess] = fsm.NewState(Deregistered)
	ue.State[models.AccessTypeNon3GPPAccess] = fsm.NewState(Deregistered)
	ue.RanUe = make(map[models.AccessType]*RanUe)
	ue.RegistrationArea = make(map[models.AccessType][]models.Tai)
	ue.AllowedNssai = make(map[models.AccessType]*models.Snssai)
	ue.OnGoing = make(map[models.AccessType]*OnGoingProcedureWithPrio)
	ue.OnGoing[models.AccessTypeNon3GPPAccess] = new(OnGoingProcedureWithPrio)
	ue.OnGoing[models.AccessTypeNon3GPPAccess].Procedure = OnGoingProcedureNothing
	ue.OnGoing[models.AccessType3GPPAccess] = new(OnGoingProcedureWithPrio)
	ue.OnGoing[models.AccessType3GPPAccess].Procedure = OnGoingProcedureNothing
	ue.ReleaseCause = make(map[models.AccessType]*CauseAll)
}

func (ue *AmfUe) CmConnect(anType models.AccessType) bool {
	if _, ok := ue.RanUe[anType]; !ok {
		return false
	}
	return true
}

func (ue *AmfUe) CmIdle(anType models.AccessType) bool {
	return !ue.CmConnect(anType)
}

func (ue *AmfUe) Remove() {
	for _, ranUe := range ue.RanUe {
		if err := ranUe.Remove(); err != nil {
			logger.AmfLog.Error("Remove RanUe error", zap.Error(err))
		}
	}

	tmsiGenerator.FreeID(int64(ue.Tmsi))

	if len(ue.Supi) > 0 {
		AMFSelf().UePool.Delete(ue.Supi)
	}
}

func (ue *AmfUe) DetachRanUe(anType models.AccessType) {
	delete(ue.RanUe, anType)
}

func (ue *AmfUe) AttachRanUe(ranUe *RanUe) {
	if ranUe == nil || ranUe.Ran == nil {
		return
	}

	anType := ranUe.Ran.AnType

	oldRanUe := ue.RanUe[anType]

	ue.RanUe[anType] = ranUe
	ranUe.AmfUe = ue

	if oldRanUe != nil && oldRanUe != ranUe {
		if oldRanUe.AmfUe == ue {
			oldRanUe.Log.Info("Detached UeContext from previous RanUe")
			oldRanUe.AmfUe = nil
		}
	}

	// set log information
	ue.NASLog = logger.AmfLog.With(zap.String("AMF_UE_NGAP_ID", fmt.Sprintf("AMF_UE_NGAP_ID:%d", ranUe.AmfUeNgapID)))
	ue.GmmLog = logger.AmfLog.With(zap.String("AMF_UE_NGAP_ID", fmt.Sprintf("AMF_UE_NGAP_ID:%d", ranUe.AmfUeNgapID)))
	ue.TxLog = logger.AmfLog.With(zap.String("AMF_UE_NGAP_ID", fmt.Sprintf("AMF_UE_NGAP_ID:%d", ranUe.AmfUeNgapID)))
}

func (ue *AmfUe) GetAnType() models.AccessType {
	if ue.CmConnect(models.AccessType3GPPAccess) {
		return models.AccessType3GPPAccess
	} else if ue.CmConnect(models.AccessTypeNon3GPPAccess) {
		return models.AccessTypeNon3GPPAccess
	}
	return ""
}

func (ue *AmfUe) InAllowedNssai(targetSNssai models.Snssai, anType models.AccessType) bool {
	return reflect.DeepEqual(*ue.AllowedNssai[anType], targetSNssai)
}

func (ue *AmfUe) InSubscribedNssai(targetSNssai *models.Snssai) bool {
	return ue.SubscribedNssai.Sst == targetSNssai.Sst && ue.SubscribedNssai.Sd == targetSNssai.Sd
}

func (ue *AmfUe) TaiListInRegistrationArea(taiList []models.Tai, accessType models.AccessType) bool {
	for _, tai := range taiList {
		if !InTaiList(tai, ue.RegistrationArea[accessType]) {
			return false
		}
	}
	return true
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
func (ue *AmfUe) DerivateAlgKey() {
	// Security Key
	P0 := []byte{security.NNASEncAlg}
	L0 := ueauth.KDFLen(P0)
	P1 := []byte{ue.CipheringAlg}
	L1 := ueauth.KDFLen(P1)

	KamfBytes, err := hex.DecodeString(ue.Kamf)
	if err != nil {
		logger.AmfLog.Error("decode kamf error", zap.Error(err))
		return
	}
	kenc, err := ueauth.GetKDFValue(KamfBytes, ueauth.FCForAlgorithmKeyDerivation, P0, L0, P1, L1)
	if err != nil {
		logger.AmfLog.Error("get kdf value error", zap.Error(err))
		return
	}
	copy(ue.KnasEnc[:], kenc[16:32])

	// Integrity Key
	P0 = []byte{security.NNASIntAlg}
	L0 = ueauth.KDFLen(P0)
	P1 = []byte{ue.IntegrityAlg}
	L1 = ueauth.KDFLen(P1)

	kint, err := ueauth.GetKDFValue(KamfBytes, ueauth.FCForAlgorithmKeyDerivation, P0, L0, P1, L1)
	if err != nil {
		logger.AmfLog.Error("get kdf value error", zap.Error(err))
		return
	}
	copy(ue.KnasInt[:], kint[16:32])
}

// Access Network key Derivation function defined in TS 33.501 Annex A.9
func (ue *AmfUe) DerivateAnKey(anType models.AccessType) {
	accessType := security.AccessType3GPP // Defalut 3gpp
	P0 := make([]byte, 4)
	binary.BigEndian.PutUint32(P0, ue.ULCount.Get())
	L0 := ueauth.KDFLen(P0)
	if anType == models.AccessTypeNon3GPPAccess {
		accessType = security.AccessTypeNon3GPP
	}
	P1 := []byte{accessType}
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
	switch accessType {
	case security.AccessType3GPP:
		ue.Kgnb = key
	case security.AccessTypeNon3GPP:
		ue.Kn3iwf = key
	}
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

func (ue *AmfUe) UpdateSecurityContext(anType models.AccessType) {
	ue.DerivateAnKey(anType)
	switch anType {
	case models.AccessType3GPPAccess:
		ue.DerivateNH(ue.Kgnb)
	case models.AccessTypeNon3GPPAccess:
		ue.DerivateNH(ue.Kn3iwf)
	}
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
func (ue *AmfUe) ClearRegistrationRequestData(accessType models.AccessType) {
	ue.RegistrationRequest = nil
	ue.RegistrationType5GS = 0
	ue.IdentityTypeUsedForRegistration = 0
	ue.AuthFailureCauseSynchFailureTimes = 0
	ue.ServingAmfChanged = false
	ue.RegistrationAcceptForNon3GPPAccess = nil
	if ue.RanUe != nil && ue.RanUe[accessType] != nil {
		ue.RanUe[accessType].UeContextRequest = false
		ue.RanUe[accessType].RecvdInitialContextSetupResponse = false
	}
	ue.RetransmissionOfInitialNASMsg = false
	ue.OnGoing[accessType].Procedure = OnGoingProcedureNothing
}

// this method called when we are reusing the same uecontext during the registration procedure
func (ue *AmfUe) ClearRegistrationData() {
	// Allowed Nssai should be cleared first as it is a new Registration
	ue.SubscribedNssai = nil
	ue.AllowedNssai = make(map[models.AccessType]*models.Snssai)
	ue.SubscriptionDataValid = false
	// Clearing SMContextList locally
	ue.SmContextList.Range(func(key, _ interface{}) bool {
		ue.SmContextList.Delete(key)
		return true
	})
}

func (ue *AmfUe) SetOnGoing(anType models.AccessType, onGoing *OnGoingProcedureWithPrio) {
	prevOnGoing := ue.OnGoing[anType]
	ue.OnGoing[anType] = onGoing
	ue.GmmLog.Debug("set ongoing procedure", zap.Any("ongoingProcedure", onGoing.Procedure), zap.Any("previousOnGoingProcedure", prevOnGoing.Procedure), zap.Any("OnGoingPPi", onGoing.Ppi), zap.Any("PreviousOnGoingPPi", prevOnGoing.Ppi))
}

func (ue *AmfUe) GetOnGoing(anType models.AccessType) OnGoingProcedureWithPrio {
	return *ue.OnGoing[anType]
}

// SM Context realted function

func (ue *AmfUe) StoreSmContext(pduSessionID int32, smContext *SmContext) {
	ue.SmContextList.Store(pduSessionID, smContext)
}

func (ue *AmfUe) SmContextFindByPDUSessionID(pduSessionID int32) (*SmContext, bool) {
	if value, ok := ue.SmContextList.Load(pduSessionID); ok {
		return value.(*SmContext), true
	} else {
		return nil, false
	}
}

func (ue *AmfUe) HasActivePduSessions() bool {
	hasActive := false
	ue.SmContextList.Range(func(key, value any) bool {
		smContext := value.(*SmContext)
		if smContext.IsPduSessionActive() {
			hasActive = true
			return false
		}
		return true
	})
	return hasActive
}
