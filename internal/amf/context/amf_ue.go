// Copyright 2024 Ella Networks
// SPDX-FileCopyrightText: 2022-present Intel Corporation
// SPDX-FileCopyrightText: 2021 Open Networking Foundation <info@opennetworking.org>
// Copyright 2019 free5GC.org
//
// SPDX-License-Identifier: Apache-2.0

package context

import (
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"os"
	"reflect"
	"regexp"
	"sync"
	"time"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/util/fsm"
	"github.com/ellanetworks/core/internal/util/idgenerator"
	"github.com/ellanetworks/core/internal/util/ueauth"
	"github.com/omec-project/nas/nasMessage"
	"github.com/omec-project/nas/nasType"
	"github.com/omec-project/nas/security"
	"github.com/omec-project/ngap/ngapType"
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
	// Mutex sync.Mutex `json:"mutex,omitempty" yaml:"mutex" bson:"mutex,omitempty"`
	Mutex sync.Mutex `json:"-"`
	/* the AMF which serving this AmfUe now */
	ServingAMF *AMFContext `json:"servingAMF,omitempty"` // never nil

	/* Gmm State */
	State map[models.AccessType]*fsm.State `json:"-"`
	/* Registration procedure related context */
	RegistrationType5GS                uint8                           `json:"registrationType5GS,omitempty"`
	IdentityTypeUsedForRegistration    uint8                           `json:"identityTypeUsedForRegistration,omitempty"`
	RegistrationRequest                *nasMessage.RegistrationRequest `json:"registrationRequest,omitempty"`
	ServingAmfChanged                  bool                            `json:"servingAmfChanged,omitempty"`
	DeregistrationTargetAccessType     uint8                           `json:"deregistrationTargetAccessType,omitempty"` // only used when deregistration procedure is initialized by the network
	RegistrationAcceptForNon3GPPAccess []byte                          `json:"registrationAcceptForNon3GPPAccess,omitempty"`
	RetransmissionOfInitialNASMsg      bool                            `json:"retransmissionOfInitialNASMsg,omitempty"`
	/* Used for AMF relocation */
	TargetAmfURI string `json:"targetAmfUri,omitempty"`
	/* Ue Identity*/
	PlmnID models.PlmnID `json:"plmnID,omitempty"`
	Suci   string        `json:"suci,omitempty"`
	Supi   string        `json:"supi,omitempty"`
	// UnauthenticatedSupi bool          `json:"unauthenticatedSupi,omitempty"`
	Gpsi    string `json:"gpsi,omitempty"`
	Pei     string `json:"pei,omitempty"`
	Tmsi    int32  `json:"tmsi,omitempty"` // 5G-Tmsi
	Guti    string `json:"guti,omitempty"`
	GroupID string `json:"groupID,omitempty"`
	EBI     int32  `json:"ebi,omitempty"`
	/* Ue Identity*/
	/* User Location*/
	RatType                  models.RatType      `json:"ratType,omitempty"`
	Location                 models.UserLocation `json:"location,omitempty"`
	Tai                      models.Tai          `json:"tai,omitempty"`
	LocationChanged          bool                `json:"locationChanged,omitempty"`
	LastVisitedRegisteredTai models.Tai          `json:"lastVisitedRegisteredTai,omitempty"`
	TimeZone                 string              `json:"timezone,omitempty"`
	/* context about udm */
	// UdmId                             string                                    `json:"udmId,omitempty"`
	SubscriptionDataValid             bool                                      `json:"subscriptionDataValid,omitempty"`
	SmfSelectionData                  *models.SmfSelectionSubscriptionData      `json:"smfSelectionData,omitempty"`
	UeContextInSmfData                *models.UeContextInSmfData                `json:"ueContextInSmfData,omitempty"`
	TraceData                         *models.TraceData                         `json:"traceData,omitempty"`
	SubscribedNssai                   []models.SubscribedSnssai                 `json:"subscribeNssai,omitempty"`
	AccessAndMobilitySubscriptionData *models.AccessAndMobilitySubscriptionData `json:"accessAndMobilitySubscriptionData,omitempty"`
	RoutingIndicator                  string                                    `json:"routingIndicator,omitempty"`
	AuthenticationCtx                 *models.UeAuthenticationCtx               `json:"authenticationCtx,omitempty"`
	AuthFailureCauseSynchFailureTimes int                                       `json:"authFailureCauseSynchFailureTimes,omitempty"`
	ABBA                              []uint8                                   `json:"abba,omitempty"`
	Kseaf                             string                                    `json:"kseaf,omitempty"`
	Kamf                              string                                    `json:"kamf,omitempty"`
	/* context about PCF */
	PolicyAssociationID          string                    `json:"policyAssociationId,omitempty"`
	AmPolicyAssociation          *models.PolicyAssociation `json:"amPolicyAssociation,omitempty"`
	RequestTriggerLocationChange bool                      `json:"requestTriggerLocationChange,omitempty"` // true if AmPolicyAssociation.Trigger contains RequestTriggerLocCh
	ConfigurationUpdateMessage   []byte                    `json:"configurationUpdateMessage,omitempty"`
	/* N1N2Message */
	N1N2MessageIDGenerator          *idgenerator.IDGenerator `json:"n1n2MessageIDGenerator,omitempty"`
	N1N2Message                     *N1N2Message             `json:"-"`
	N1N2MessageSubscribeIDGenerator *idgenerator.IDGenerator `json:"n1n2MessageSubscribeIDGenerator,omitempty"`
	/* Pdu Sesseion context */
	SmContextList sync.Map `json:"-"` // map[int32]*SmContext, pdu session id as key
	/* Related Context*/
	//RanUe map[models.AccessType]*RanUe `json:"ranUe,omitempty" yaml:"ranUe" bson:"ranUe,omitempty"`
	RanUe map[models.AccessType]*RanUe `json:"ranUe,omitempty"`
	/* other */
	OnGoing                       map[models.AccessType]*OnGoingProcedureWithPrio `json:"onGoing,omitempty"`
	UeRadioCapability             string                                          `json:"ueRadioCapability,omitempty"` // OCTET string
	Capability5GMM                nasType.Capability5GMM                          `json:"capability5GMM,omitempty"`
	ConfigurationUpdateIndication nasType.ConfigurationUpdateIndication           `json:"configurationUpdateIndication,omitempty"`
	/* context related to Paging */
	UeRadioCapabilityForPaging                 *UERadioCapabilityForPaging                 `json:"ueRadioCapabilityForPaging,omitempty"`
	InfoOnRecommendedCellsAndRanNodesForPaging *InfoOnRecommendedCellsAndRanNodesForPaging `json:"infoOnRecommendedCellsAndRanNodesForPaging,omitempty"`
	UESpecificDRX                              uint8                                       `json:"ueSpecificDRX,omitempty"`
	/* Security Context */
	SecurityContextAvailable bool                         `json:"securityContextAvailable,omitempty"`
	UESecurityCapability     nasType.UESecurityCapability `json:"ueSecurityCapability,omitempty"` // for security command
	NgKsi                    models.NgKsi                 `json:"ngKsi,omitempty"`
	MacFailed                bool                         `json:"macFailed,omitempty"` // set to true if the integrity check of current NAS message is failed
	KnasInt                  [16]uint8                    `json:"knasInt,omitempty"`   // 16 byte
	KnasEnc                  [16]uint8                    `json:"knasEnc,omitempty"`   // 16 byte
	Kgnb                     []uint8                      `json:"kgnb,omitempty"`      // 32 byte
	Kn3iwf                   []uint8                      `json:"kn3iwf,omitempty"`    // 32 byte
	NH                       []uint8                      `json:"nh,omitempty"`        // 32 byte
	NCC                      uint8                        `json:"ncc,omitempty"`       // 0..7
	// ULCount                  security.Count               `json:"ulCount,omitempty" yaml:"ulCount" bson:"ulCount,omitempty"`
	// DLCount                  security.Count               `json:"dlCount,omitempty" yaml:"dlCount" bson:"dlCount,omitempty"`
	ULCount      security.Count `json:"-"`
	DLCount      security.Count `json:"-"`
	CipheringAlg uint8          `json:"cipheringAlg,omitempty"`
	IntegrityAlg uint8          `json:"integrityAlg,omitempty"`
	/* Registration Area */
	RegistrationArea map[models.AccessType][]models.Tai `json:"registrationArea,omitempty"`
	LadnInfo         []LADN                             `json:"ladnInfo,omitempty"`
	/* Network Slicing related context and Nssf */
	NetworkSliceInfo                  *models.AuthorizedNetworkSliceInfo           `json:"networkSliceInfo,omitempty"`
	AllowedNssai                      map[models.AccessType][]models.AllowedSnssai `json:"allowedNssai,omitempty"`
	NetworkSlicingSubscriptionChanged bool                                         `json:"networkSlicingSubscriptionChanged,omitempty"`
	/* T3513(Paging) */
	T3513 *Timer `json:"t3513Value,omitempty"` // for paging
	/* T3565(Notification) */
	T3565 *Timer `json:"t3565Value,omitempty"` // for NAS Notification
	/* T3560 (for authentication request/security mode command retransmission) */
	T3560 *Timer `json:"t3560Value,omitempty"`
	/* T3550 (for registration accept retransmission) */
	T3550 *Timer `json:"t3550Value,omitempty"`
	/* T3522 (for deregistration request) */
	T3522 *Timer `json:"t3522Value,omitempty"`
	/* Ue Context Release Cause */
	ReleaseCause map[models.AccessType]*CauseAll `json:"releaseCause,omitempty"`
	/* T3502 (Assigned by AMF, and used by UE to initialize registration procedure) */
	T3502Value                      int `json:"t3502Value,omitempty"`                      // Second
	T3512Value                      int `json:"t3512Value,omitempty"`                      // default 54 min
	Non3gppDeregistrationTimerValue int `json:"non3gppDeregistrationTimerValue,omitempty"` // default 54 min

	// AmfInstanceName and Ip
	AmfInstanceName string `json:"amfInstanceName,omitempty"`
	AmfInstanceIP   string `json:"amfInstanceIp,omitempty"`

	NASLog      *zap.Logger `json:"-"`
	GmmLog      *zap.Logger `json:"-"`
	TxLog       *zap.Logger `json:"-"`
	ProducerLog *zap.Logger `json:"-"`
}

type InterfaceType uint8

const (
	NgapMessage InterfaceType = iota
	SbiMessage
	NasMessage
)

type InterfaceMsg interface{}

/*type InterfaceMsg struct {
	AnType        models.AccessType
	NasMsg        []byte
	ProcedureCode int64
	NgapMsg       *ngapType.NGAPPDU
	Ran           *AmfRan
	//MsgType is Nas or Sbi interface msg
	IntfType InterfaceType
}*/

type NasMsg struct {
	AnType        models.AccessType
	NasMsg        []byte
	ProcedureCode int64
}

type NgapMsg struct {
	NgapMsg *ngapType.NGAPPDU
	Ran     *AmfRan
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

func (ue *AmfUe) init() {
	ue.ServingAMF = AMFSelf()
	ue.State = make(map[models.AccessType]*fsm.State)
	ue.State[models.AccessType3GPPAccess] = fsm.NewState(Deregistered)
	ue.State[models.AccessTypeNon3GPPAccess] = fsm.NewState(Deregistered)
	ue.RanUe = make(map[models.AccessType]*RanUe)
	ue.RegistrationArea = make(map[models.AccessType][]models.Tai)
	ue.AllowedNssai = make(map[models.AccessType][]models.AllowedSnssai)
	ue.N1N2MessageIDGenerator = idgenerator.NewGenerator(1, 2147483647)
	ue.N1N2MessageSubscribeIDGenerator = idgenerator.NewGenerator(1, 2147483647)
	ue.OnGoing = make(map[models.AccessType]*OnGoingProcedureWithPrio)
	ue.OnGoing[models.AccessTypeNon3GPPAccess] = new(OnGoingProcedureWithPrio)
	ue.OnGoing[models.AccessTypeNon3GPPAccess].Procedure = OnGoingProcedureNothing
	ue.OnGoing[models.AccessType3GPPAccess] = new(OnGoingProcedureWithPrio)
	ue.OnGoing[models.AccessType3GPPAccess].Procedure = OnGoingProcedureNothing
	ue.ReleaseCause = make(map[models.AccessType]*CauseAll)
	ue.AmfInstanceName = os.Getenv("HOSTNAME")
	ue.AmfInstanceIP = os.Getenv("POD_IP")
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
	/* detach any RanUe associated to it */
	oldRanUe := ue.RanUe[ranUe.Ran.AnType]
	ue.RanUe[ranUe.Ran.AnType] = ranUe
	ranUe.AmfUe = ue

	go func() {
		time.Sleep(time.Second * 2)
		if oldRanUe != nil {
			oldRanUe.Log.Info("Detached UeContext from OldRanUe")
			oldRanUe.AmfUe = nil
		}
	}()

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
	for _, allowedSnssai := range ue.AllowedNssai[anType] {
		if reflect.DeepEqual(*allowedSnssai.AllowedSnssai, targetSNssai) {
			return true
		}
	}
	return false
}

func (ue *AmfUe) InSubscribedNssai(targetSNssai *models.Snssai) bool {
	for _, sNssai := range ue.SubscribedNssai {
		if sNssai.SubscribedSnssai.Sst == targetSNssai.Sst && sNssai.SubscribedSnssai.Sd == targetSNssai.Sd {
			return true
		}
	}
	return false
}

func (ue *AmfUe) GetNsiInformationFromSnssai(anType models.AccessType, snssai models.Snssai) *models.NsiInformation {
	for _, allowedSnssai := range ue.AllowedNssai[anType] {
		if reflect.DeepEqual(*allowedSnssai.AllowedSnssai, snssai) {
			if len(allowedSnssai.NsiInformationList) != 0 {
				return &allowedSnssai.NsiInformationList[0]
			}
		}
	}
	return nil
}

func (ue *AmfUe) TaiListInRegistrationArea(taiList []models.Tai, accessType models.AccessType) bool {
	for _, tai := range taiList {
		if !InTaiList(tai, ue.RegistrationArea[accessType]) {
			return false
		}
	}
	return true
}

func (ue *AmfUe) HasWildCardSubscribedDNN() bool {
	for _, snssaiInfo := range ue.SmfSelectionData.SubscribedSnssaiInfos {
		for _, dnnInfo := range snssaiInfo.DnnInfos {
			if dnnInfo.Dnn == "*" {
				return true
			}
		}
	}
	return false
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
	ue.AllowedNssai = make(map[models.AccessType][]models.AllowedSnssai)
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

func (ue *AmfUe) RemoveAmPolicyAssociation() {
	ue.AmPolicyAssociation = nil
	ue.PolicyAssociationID = ""
}

func (ue *AmfUe) CopyDataFromUeContextModel(ueContext models.UeContext) {
	if ueContext.Supi != "" {
		ue.Supi = ueContext.Supi
	}

	if ueContext.Pei != "" {
		ue.Pei = ueContext.Pei
	}

	if ueContext.RoutingIndicator != "" {
		ue.RoutingIndicator = ueContext.RoutingIndicator
	}

	if ueContext.SubUeAmbr != nil {
		if ue.AccessAndMobilitySubscriptionData == nil {
			ue.AccessAndMobilitySubscriptionData = new(models.AccessAndMobilitySubscriptionData)
		}
		if ue.AccessAndMobilitySubscriptionData.SubscribedUeAmbr == nil {
			ue.AccessAndMobilitySubscriptionData.SubscribedUeAmbr = new(models.AmbrRm)
		}

		subAmbr := ue.AccessAndMobilitySubscriptionData.SubscribedUeAmbr
		subAmbr.Uplink = ueContext.SubUeAmbr.Uplink
		subAmbr.Downlink = ueContext.SubUeAmbr.Downlink
	}

	if ueContext.SubRfsp != 0 {
		if ue.AccessAndMobilitySubscriptionData == nil {
			ue.AccessAndMobilitySubscriptionData = new(models.AccessAndMobilitySubscriptionData)
		}
		ue.AccessAndMobilitySubscriptionData.RfspIndex = ueContext.SubRfsp
	}

	if len(ueContext.RestrictedRatList) > 0 {
		if ue.AccessAndMobilitySubscriptionData == nil {
			ue.AccessAndMobilitySubscriptionData = new(models.AccessAndMobilitySubscriptionData)
		}
		ue.AccessAndMobilitySubscriptionData.RatRestrictions = []models.RatType{}
		ue.AccessAndMobilitySubscriptionData.RatRestrictions = append(ue.AccessAndMobilitySubscriptionData.RatRestrictions, ueContext.RestrictedRatList...)
	}

	if len(ueContext.ForbiddenAreaList) > 0 {
		if ue.AccessAndMobilitySubscriptionData == nil {
			ue.AccessAndMobilitySubscriptionData = new(models.AccessAndMobilitySubscriptionData)
		}
		// ue.AccessAndMobilitySubscriptionData.ForbiddenAreas = ueContext.ForbiddenAreaList
		ue.AccessAndMobilitySubscriptionData.ForbiddenAreas = []models.Area{}
		for _, area := range ueContext.ForbiddenAreaList {
			ue.AccessAndMobilitySubscriptionData.ForbiddenAreas = append(ue.AccessAndMobilitySubscriptionData.ForbiddenAreas, models.Area{
				Tacs:      []string{},
				AreaCodes: area.AreaCodes,
			})
			ue.AccessAndMobilitySubscriptionData.ForbiddenAreas[len(ue.AccessAndMobilitySubscriptionData.ForbiddenAreas)-1].Tacs = append(ue.AccessAndMobilitySubscriptionData.ForbiddenAreas[len(ue.AccessAndMobilitySubscriptionData.ForbiddenAreas)-1].Tacs, area.Tacs...)
		}
	}

	if ueContext.ServiceAreaRestriction != nil {
		if ue.AccessAndMobilitySubscriptionData == nil {
			ue.AccessAndMobilitySubscriptionData = new(models.AccessAndMobilitySubscriptionData)
		}
		ue.AccessAndMobilitySubscriptionData.ServiceAreaRestriction = &models.ServiceAreaRestriction{
			RestrictionType: ueContext.ServiceAreaRestriction.RestrictionType,
			Areas:           []models.Area{},
			MaxNumOfTAs:     ueContext.ServiceAreaRestriction.MaxNumOfTAs,
		}
		for _, area := range ueContext.ServiceAreaRestriction.Areas {
			ue.AccessAndMobilitySubscriptionData.ServiceAreaRestriction.Areas = append(ue.AccessAndMobilitySubscriptionData.ServiceAreaRestriction.Areas, models.Area{
				Tacs:      []string{},
				AreaCodes: area.AreaCodes,
			})
			ue.AccessAndMobilitySubscriptionData.ServiceAreaRestriction.Areas[len(ue.AccessAndMobilitySubscriptionData.ServiceAreaRestriction.Areas)-1].Tacs = append(ue.AccessAndMobilitySubscriptionData.ServiceAreaRestriction.Areas[len(ue.AccessAndMobilitySubscriptionData.ServiceAreaRestriction.Areas)-1].Tacs, area.Tacs...)
		}
	}

	if ueContext.SeafData != nil {
		seafData := ueContext.SeafData

		ue.NgKsi = *seafData.NgKsi
		if seafData.KeyAmf != nil {
			if seafData.KeyAmf.KeyType == models.KeyAmfTypeKamf {
				ue.Kamf = seafData.KeyAmf.KeyVal
			}
		}
		if nh, err := hex.DecodeString(seafData.Nh); err != nil {
			logger.AmfLog.Error("decode nh error", zap.Error(err))
			return
		} else {
			ue.NH = nh
		}
		ue.NCC = uint8(seafData.Ncc)
	}

	if len(ueContext.AmPolicyReqTriggerList) > 0 {
		if ue.AmPolicyAssociation == nil {
			ue.AmPolicyAssociation = new(models.PolicyAssociation)
		}
		for _, trigger := range ueContext.AmPolicyReqTriggerList {
			switch trigger {
			case models.AmPolicyReqTriggerLocationChange:
				ue.AmPolicyAssociation.Triggers = append(ue.AmPolicyAssociation.Triggers, models.RequestTriggerLocCh)
			case models.AmPolicyReqTriggerPraChange:
				ue.AmPolicyAssociation.Triggers = append(ue.AmPolicyAssociation.Triggers, models.RequestTriggerPraCh)
			case models.AmPolicyReqTriggerSariChange:
				ue.AmPolicyAssociation.Triggers = append(ue.AmPolicyAssociation.Triggers, models.RequestTriggerServAreaCh)
			case models.AmPolicyReqTriggerRfspIndexChange:
				ue.AmPolicyAssociation.Triggers = append(ue.AmPolicyAssociation.Triggers, models.RequestTriggerRfspCh)
			}
		}
	}

	if len(ueContext.SessionContextList) > 0 {
		for _, pduSessionContext := range ueContext.SessionContextList {
			smContext := SmContext{
				Mu:              new(sync.RWMutex),
				PduSessionIDVal: pduSessionContext.PduSessionID,
				SmContextRefVal: pduSessionContext.SmContextRef,
				SnssaiVal:       *pduSessionContext.SNssai,
				DnnVal:          pduSessionContext.Dnn,
				AccessTypeVal:   pduSessionContext.AccessType,
				HSmfIDVal:       pduSessionContext.HsmfID,
				VSmfIDVal:       pduSessionContext.VsmfID,
				NsInstanceVal:   pduSessionContext.NsInstance,
			}
			ue.StoreSmContext(pduSessionContext.PduSessionID, &smContext)
		}
	}

	if len(ueContext.MmContextList) > 0 {
		for _, mmContext := range ueContext.MmContextList {
			if mmContext.AccessType == models.AccessType3GPPAccess {
				if nasSecurityMode := mmContext.NasSecurityMode; nasSecurityMode != nil {
					switch nasSecurityMode.IntegrityAlgorithm {
					case models.IntegrityAlgorithmNIA0:
						ue.IntegrityAlg = security.AlgIntegrity128NIA0
					case models.IntegrityAlgorithmNIA1:
						ue.IntegrityAlg = security.AlgIntegrity128NIA1
					case models.IntegrityAlgorithmNIA2:
						ue.IntegrityAlg = security.AlgIntegrity128NIA2
					case models.IntegrityAlgorithmNIA3:
						ue.IntegrityAlg = security.AlgIntegrity128NIA3
					}

					switch nasSecurityMode.CipheringAlgorithm {
					case models.CipheringAlgorithmNEA0:
						ue.CipheringAlg = security.AlgCiphering128NEA0
					case models.CipheringAlgorithmNEA1:
						ue.CipheringAlg = security.AlgCiphering128NEA1
					case models.CipheringAlgorithmNEA2:
						ue.CipheringAlg = security.AlgCiphering128NEA2
					case models.CipheringAlgorithmNEA3:
						ue.CipheringAlg = security.AlgCiphering128NEA3
					}

					if mmContext.NasDownlinkCount != 0 {
						overflow := uint16((uint32(mmContext.NasDownlinkCount) & 0x00ffff00) >> 8)
						sqn := uint8(uint32(mmContext.NasDownlinkCount & 0x000000ff))
						ue.DLCount.Set(overflow, sqn)
					}

					if mmContext.NasUplinkCount != 0 {
						overflow := uint16((uint32(mmContext.NasUplinkCount) & 0x00ffff00) >> 8)
						sqn := uint8(uint32(mmContext.NasUplinkCount & 0x000000ff))
						ue.ULCount.Set(overflow, sqn)
					}

					// TS 29.518 Table 6.1.6.3.2.1
					if mmContext.UeSecurityCapability != "" {
						// ue.SecurityCapabilities
						buf, err := base64.StdEncoding.DecodeString(mmContext.UeSecurityCapability)
						if err != nil {
							logger.AmfLog.Error("decode ueSecurityCapability error", zap.Error(err))
							return
						}
						ue.UESecurityCapability.Buffer = buf
						ue.UESecurityCapability.SetLen(uint8(len(buf)))
					}
				}
			}

			if mmContext.AllowedNssai != nil {
				for _, snssai := range mmContext.AllowedNssai {
					allowedSnssai := models.AllowedSnssai{
						AllowedSnssai: &snssai,
					}
					ue.AllowedNssai[mmContext.AccessType] = append(ue.AllowedNssai[mmContext.AccessType], allowedSnssai)
				}
			}
		}
	}
	if ueContext.TraceData != nil {
		ue.TraceData = ueContext.TraceData
	}
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
