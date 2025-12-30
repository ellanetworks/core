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
	OnGoingProcedureAbort        OnGoingProcedure = "Abort"
)

const (
	RecommendRanNodePresentRanNode int32 = 0
	RecommendRanNodePresentTAI     int32 = 1
)

type StateType string

const (
	Deregistered   StateType = "Deregistered"
	Authentication StateType = "Authentication"
	SecurityMode   StateType = "SecurityMode"
	ContextSetup   StateType = "ContextSetup"
	Registered     StateType = "Registered"
)

type SmContext struct {
	Ref                string
	Snssai             *models.Snssai
	PduSessionInactive bool
}

type AmfUe struct {
	Mutex sync.Mutex

	/* Gmm State */
	State StateType
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
	Location models.UserLocation
	Tai      models.Tai
	TimeZone string
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
	UeRadioCapabilityForPaging                 *models.UERadioCapabilityForPaging
	InfoOnRecommendedCellsAndRanNodesForPaging *models.InfoOnRecommendedCellsAndRanNodesForPaging
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

func NewAmfUe() *AmfUe {
	return &AmfUe{
		State:            Deregistered,
		RegistrationArea: make([]models.Tai, 0),
		OnGoing: &OnGoingProcedureWithPrio{
			Procedure: OnGoingProcedureNothing,
		},
		SmContextList: make(map[uint8]*SmContext),
	}
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

func (ue *AmfUe) IsAllowedNssai(targetSNssai *models.Snssai) bool {
	return reflect.DeepEqual(*ue.AllowedNssai, *targetSNssai)
}

func (ue *AmfUe) SecurityContextIsValid() bool {
	return ue.SecurityContextAvailable && ue.NgKsi.Ksi != nasMessage.NasKeySetIdentifierNoKeyIsAvailable && !ue.MacFailed
}

// Kamf Derivation function defined in TS 33.501 Annex A.7
func (ue *AmfUe) DerivateKamf() error {
	supiRegexp, err := regexp.Compile("([0-9]{5,15})")
	if err != nil {
		return fmt.Errorf("could not compile supi regexp: %v", err)
	}

	groups := supiRegexp.FindStringSubmatch(ue.Supi)
	if groups == nil {
		return fmt.Errorf("supi is not correct")
	}

	P0 := []byte(groups[1])
	L0 := ueauth.KDFLen(P0)
	P1 := ue.ABBA
	L1 := ueauth.KDFLen(P1)

	kSeafDecode, err := hex.DecodeString(ue.Kseaf)
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
	ue.NCC++

	err := ue.DerivateNH(ue.NH)
	if err != nil {
		return fmt.Errorf("error deriving NH: %v", err)
	}

	return nil
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

func (ue *AmfUe) CreateSmContext(pduSessionID uint8, ref string, snssai *models.Snssai) {
	ue.SmContextList[pduSessionID] = &SmContext{
		Ref:    ref,
		Snssai: snssai,
	}
}

func (ue *AmfUe) SmContextFindByPDUSessionID(pduSessionID uint8) (*SmContext, bool) {
	smContext, ok := ue.SmContextList[pduSessionID]
	return smContext, ok
}

func (ue *AmfUe) HasActivePduSessions() bool {
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

	// Plain NAS message
	if !ue.SecurityContextAvailable {
		return msg.PlainNasEncode()
	}

	// Security protected NAS Message
	// a security protected NAS message must be integrity protected, and ciphering is optional
	needCiphering := false
	switch msg.SecurityHeader.SecurityHeaderType {
	case nas.SecurityHeaderTypeIntegrityProtected:
	case nas.SecurityHeaderTypeIntegrityProtectedAndCiphered:
		needCiphering = true
	case nas.SecurityHeaderTypeIntegrityProtectedWithNew5gNasSecurityContext:
		ue.ULCount.Set(0, 0)
		ue.DLCount.Set(0, 0)
	default:
		return nil, fmt.Errorf("wrong security header type: 0x%0x", msg.SecurityHeader.SecurityHeaderType)
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
	msgSecurityHeader := []byte{msg.SecurityHeader.ProtocolDiscriminator, msg.SecurityHeader.SecurityHeaderType}
	payload = append(msgSecurityHeader, payload[:]...)

	// Increase DL Count
	ue.DLCount.AddOne()

	return payload, nil
}

/*
payload either a security protected 5GS NAS message or a plain 5GS NAS message which
format is followed TS 24.501 9.1.1
*/
func (ue *AmfUe) DecodeNASMessage(payload []byte) (*nas.Message, error) {
	if payload == nil {
		return nil, fmt.Errorf("nas payload is empty")
	}

	if len(payload) < 2 {
		return nil, fmt.Errorf("nas payload is too short")
	}

	msg := new(nas.Message)
	msg.SecurityHeaderType = nas.GetSecurityHeaderType(payload) & 0x0f
	if msg.SecurityHeaderType == nas.SecurityHeaderTypePlainNas {
		// RRCEstablishmentCause 0 is for emergency service
		if ue.SecurityContextAvailable && ue.RanUe.RRCEstablishmentCause != "0" {
			ue.Log.Warn("Received Plain NAS message")
			ue.MacFailed = false
			ue.SecurityContextAvailable = false
			if err := msg.PlainNasDecode(&payload); err != nil {
				return nil, err
			}

			if msg.GmmMessage == nil {
				return nil, fmt.Errorf("gmm message is nil")
			}

			// TS 24.501 4.4.4.3: Except the messages listed below, no NAS signalling messages shall be processed
			// by the receiving 5GMM entity in the AMF or forwarded to the 5GSM entity, unless the secure exchange
			// of NAS messages has been established for the NAS signalling connection
			switch msg.GmmHeader.GetMessageType() {
			case nas.MsgTypeRegistrationRequest:
				return msg, nil
			case nas.MsgTypeIdentityResponse:
				return msg, nil
			case nas.MsgTypeAuthenticationResponse:
				return msg, nil
			case nas.MsgTypeAuthenticationFailure:
				return msg, nil
			case nas.MsgTypeSecurityModeReject:
				return msg, nil
			case nas.MsgTypeDeregistrationRequestUEOriginatingDeregistration:
				return msg, nil
			case nas.MsgTypeDeregistrationAcceptUETerminatedDeregistration:
				return msg, nil
			default:
				return nil, fmt.Errorf(
					"UE can not send plain nas for non-emergency service when there is a valid security context")
			}
		} else {
			ue.MacFailed = false
			err := msg.PlainNasDecode(&payload)
			return msg, err
		}
	} else { // Security protected NAS message
		if len(payload) < 7 {
			return nil, fmt.Errorf("nas payload is too short")
		}
		securityHeader := payload[0:6]
		sequenceNumber := payload[6]

		receivedMac32 := securityHeader[2:]
		// remove security Header except for sequece Number
		payload = payload[6:]

		// a security protected NAS message must be integrity protected, and ciphering is optional
		ciphered := false
		switch msg.SecurityHeaderType {
		case nas.SecurityHeaderTypeIntegrityProtected:
		case nas.SecurityHeaderTypeIntegrityProtectedAndCiphered:
			ciphered = true
		case nas.SecurityHeaderTypeIntegrityProtectedAndCipheredWithNew5gNasSecurityContext:
			ciphered = true
			ue.ULCount.Set(0, 0)
		default:
			return nil, fmt.Errorf("wrong security header type: 0x%0x", msg.SecurityHeader.SecurityHeaderType)
		}

		if ue.ULCount.SQN() > sequenceNumber {
			ue.Log.Debug("set ULCount overflow")
			ue.ULCount.SetOverflow(ue.ULCount.Overflow() + 1)
		}
		ue.ULCount.SetSQN(sequenceNumber)

		mac32, err := security.NASMacCalculate(ue.IntegrityAlg, ue.KnasInt, ue.ULCount.Get(), security.Bearer3GPP,
			security.DirectionUplink, payload)
		if err != nil {
			return nil, fmt.Errorf("error calculating mac: %+v", err)
		}

		if !reflect.DeepEqual(mac32, receivedMac32) {
			ue.Log.Warn("MAC verification failed", zap.String("received", hex.EncodeToString(receivedMac32)), zap.String("expected", hex.EncodeToString(mac32)))
			ue.MacFailed = true
		} else {
			ue.MacFailed = false
		}

		if ciphered {
			ue.Log.Debug("Decrypt NAS message", zap.Uint8("algorithm", ue.CipheringAlg), zap.Uint32("ULCount", ue.ULCount.Get()))
			// decrypt payload without sequence number (payload[1])
			if err = security.NASEncrypt(ue.CipheringAlg, ue.KnasEnc, ue.ULCount.Get(), security.Bearer3GPP,
				security.DirectionUplink, payload[1:]); err != nil {
				return nil, fmt.Errorf("error encrypting: %+v", err)
			}
		}

		// remove sequece Number
		payload = payload[1:]
		err = msg.PlainNasDecode(&payload)

		/*
			integrity check failed, as per spec 24501 section 4.4.4.3 AMF shouldnt process or forward to SMF
			except below message types
		*/
		if err == nil && ue.MacFailed {
			switch msg.GmmHeader.GetMessageType() {
			case nas.MsgTypeRegistrationRequest:
				return msg, nil
			case nas.MsgTypeIdentityResponse:
				return msg, nil
			case nas.MsgTypeAuthenticationResponse:
				return msg, nil
			case nas.MsgTypeAuthenticationFailure:
				return msg, nil
			case nas.MsgTypeSecurityModeReject:
				return msg, nil
			case nas.MsgTypeServiceRequest:
				return msg, nil
			case nas.MsgTypeDeregistrationRequestUEOriginatingDeregistration:
				return msg, nil
			case nas.MsgTypeDeregistrationAcceptUETerminatedDeregistration:
				return msg, nil
			default:
				return nil, fmt.Errorf("mac verification failed for the nas message: %v", msg.GmmHeader.GetMessageType())
			}
		}

		return msg, err
	}
}
