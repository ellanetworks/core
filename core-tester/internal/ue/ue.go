package ue

import (
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"sync"
	"time"

	"github.com/ellanetworks/core/core-tester/internal/air"
	"github.com/ellanetworks/core/core-tester/internal/logger"
	"github.com/ellanetworks/core/core-tester/internal/ue/sidf"
	"github.com/free5gc/nas"
	"github.com/free5gc/nas/nasMessage"
	"github.com/free5gc/nas/nasType"
	"github.com/free5gc/nas/security"
	"github.com/free5gc/ngap/ngapType"
	"github.com/free5gc/openapi/models"
	"github.com/free5gc/util/milenage"
	"github.com/free5gc/util/ueauth"
	"go.uber.org/zap"
)

const (
	MM5G_NULL                 = 0x00
	MM5G_DEREGISTERED         = 0x01
	MM5G_REGISTERED_INITIATED = 0x02
	MM5G_REGISTERED           = 0x03
	MM5G_SERVICE_REQ_INIT     = 0x04
	MM5G_DEREGISTERED_INIT    = 0x05
	MM5G_IDLE                 = 0x06
)

type UESecurity struct {
	Supi                 string
	Msin                 string
	mcc                  string
	mnc                  string
	ULCount              security.Count
	DLCount              security.Count
	UeSecurityCapability *nasType.UESecurityCapability
	IntegrityAlg         uint8
	CipheringAlg         uint8
	NgKsi                models.NgKsi
	Snn                  string
	KnasEnc              [16]uint8
	KnasInt              [16]uint8
	Kamf                 []uint8
	AuthenticationSubs   models.AuthenticationSubscription
	Suci                 nasType.MobileIdentity5GS
	suciPublicKey        sidf.HomeNetworkPublicKey
	RoutingIndicator     string
	Guti                 *nasType.GUTI5G
}

type Amf struct {
	mcc string
	mnc string
}

type PDUSessionInfo struct {
	PDUSessionID uint8
	UEIP         string
	MTU          uint16
	QFI          uint8
}

type UE struct {
	UeSecurity             *UESecurity
	StateMM                int
	DNN                    string
	PDUSessionID           uint8
	PDUSessionType         uint8
	Snssai                 models.Snssai
	amfInfo                Amf
	IMEISV                 string
	Gnb                    air.UplinkSender
	mu                     sync.Mutex
	PDUSession             PDUSessionInfo
	receivedNASGMMMessages map[uint8][]*nas.Message // msgType -> gmm messages
	receivedNASGSMMessages map[uint8][]*nas.Message // msgType -> gsm messages
	receivedRRCRelease     bool
}

func (ue *UE) SetPDUSession(pduSession PDUSessionInfo) {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	ue.PDUSession = pduSession
}

func (ue *UE) GetPDUSession() PDUSessionInfo {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	return ue.PDUSession
}

func (ue *UE) WaitForPDUSession(timeout time.Duration) (PDUSessionInfo, error) {
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		ue.mu.Lock()
		pduSession := ue.PDUSession
		ue.mu.Unlock()

		if pduSession.PDUSessionID != 0 {
			return pduSession, nil
		}

		time.Sleep(10 * time.Millisecond)
	}

	return PDUSessionInfo{}, errors.New("timeout waiting for PDU session")
}

type UEOpts struct {
	PDUSessionID         uint8
	PDUSessionType       uint8
	Msin                 string
	UeSecurityCapability *nasType.UESecurityCapability
	K                    string
	OpC                  string
	Amf                  string
	Sqn                  string
	Mcc                  string
	Mnc                  string
	HomeNetworkPublicKey sidf.HomeNetworkPublicKey
	RoutingIndicator     string
	DNN                  string
	Sst                  int32
	Sd                   string
	IMEISV               string
	Guti                 *nasType.GUTI5G
	GnodeB               air.UplinkSender
}

func NewUE(opts *UEOpts) (*UE, error) {
	ue := UE{}
	ue.UeSecurity = &UESecurity{}
	ue.UeSecurity.Msin = opts.Msin
	ue.UeSecurity.UeSecurityCapability = opts.UeSecurityCapability
	ue.Gnb = opts.GnodeB
	ue.PDUSessionID = opts.PDUSessionID
	ue.PDUSessionType = opts.PDUSessionType

	integAlg, cipherAlg, err := SelectAlgorithms(ue.UeSecurity.UeSecurityCapability)
	if err != nil {
		return nil, fmt.Errorf("could not select security algorithms: %v", err)
	}

	ue.UeSecurity.IntegrityAlg = integAlg
	ue.UeSecurity.CipheringAlg = cipherAlg
	ue.UeSecurity.NgKsi.Ksi = 7
	ue.UeSecurity.NgKsi.Tsc = models.ScType_NATIVE

	ue.SetAuthSubscription(opts.K, opts.OpC, opts.Amf, opts.Sqn)

	ue.UeSecurity.mcc = opts.Mcc
	ue.UeSecurity.mnc = opts.Mnc

	ue.UeSecurity.RoutingIndicator = opts.RoutingIndicator
	ue.UeSecurity.suciPublicKey = opts.HomeNetworkPublicKey

	ue.UeSecurity.Supi = fmt.Sprintf("imsi-%s%s%s", opts.Mcc, opts.Mnc, opts.Msin)

	ue.Snssai.Sd = opts.Sd
	ue.Snssai.Sst = opts.Sst

	ue.DNN = opts.DNN

	ue.IMEISV = opts.IMEISV

	ue.receivedNASGMMMessages = make(map[uint8][]*nas.Message)
	ue.receivedNASGSMMessages = make(map[uint8][]*nas.Message)

	suci, err := ue.EncodeSuci()
	if err != nil {
		return nil, fmt.Errorf("failed to encode SUCI: %v", err)
	}

	ue.SetAmfMccAndMnc(opts.Mcc, opts.Mnc)

	ue.UeSecurity.Suci = suci

	if opts.Guti != nil {
		ue.Set5gGuti(opts.Guti)
	}

	ue.StateMM = MM5G_NULL

	return &ue, nil
}

func (ue *UE) SetAuthSubscription(k, opc, amf, sqn string) {
	ue.UeSecurity.AuthenticationSubs.EncPermanentKey = k
	ue.UeSecurity.AuthenticationSubs.EncOpcKey = opc

	ue.UeSecurity.AuthenticationSubs.AuthenticationManagementField = amf

	ue.UeSecurity.AuthenticationSubs.SequenceNumber = &models.SequenceNumber{
		Sqn: sqn,
	}
	ue.UeSecurity.AuthenticationSubs.AuthenticationMethod = models.AuthMethod__5_G_AKA
}

func (ue *UE) EncodeSuci() (nasType.MobileIdentity5GS, error) {
	protScheme, err := strconv.ParseUint(ue.UeSecurity.suciPublicKey.ProtectionScheme, 10, 8)
	if err != nil {
		return nasType.MobileIdentity5GS{}, fmt.Errorf("unable to parse protection scheme: %v", err)
	}

	buf6 := byte(protScheme)

	hnPubKeyId, err := strconv.ParseUint(ue.UeSecurity.suciPublicKey.PublicKeyID, 10, 8)
	if err != nil {
		return nasType.MobileIdentity5GS{}, fmt.Errorf("unable to parse home network public key ID: %v", err)
	}

	buf7 := byte(hnPubKeyId)

	var schemeOutput []byte

	if protScheme == 0 {
		schemeOutput, err = hex.DecodeString(sidf.Tbcd(ue.UeSecurity.Msin))
		if err != nil {
			return nasType.MobileIdentity5GS{}, fmt.Errorf("unable to decode msin to tbcd: %v", err)
		}
	} else {
		suci, err := sidf.CipherSuci(ue.UeSecurity.Msin, ue.UeSecurity.mcc, ue.UeSecurity.mnc, ue.UeSecurity.RoutingIndicator, ue.UeSecurity.suciPublicKey)
		if err != nil {
			return nasType.MobileIdentity5GS{}, fmt.Errorf("unable to cipher SUCI: %v", err)
		}

		schemeOutput, err = hex.DecodeString(suci.SchemeOutput)
		if err != nil {
			return nasType.MobileIdentity5GS{}, fmt.Errorf("unable to decode scheme output to bytes: %v", err)
		}
	}

	buffer := make([]byte, 8+len(schemeOutput))

	buffer[0] = 1

	plmnID, err := ue.GetMccAndMncInOctets()
	if err != nil {
		return nasType.MobileIdentity5GS{}, fmt.Errorf("unable to get mcc and mnc in octets: %v", err)
	}

	copy(buffer[1:], plmnID)

	routingInd, err := ue.GetRoutingIndicatorInOctets()
	if err != nil {
		return nasType.MobileIdentity5GS{}, fmt.Errorf("unable to get routing indicator: %v", err)
	}

	copy(buffer[4:], routingInd)
	buffer[6] = buf6
	buffer[7] = buf7
	copy(buffer[8:], schemeOutput)

	suci := nasType.MobileIdentity5GS{
		Buffer: buffer,
		Len:    uint16(len(buffer)),
	}

	return suci, nil
}

func (ue *UE) GetMccAndMncInOctets() ([]byte, error) {
	var res string

	mcc := reverse(ue.UeSecurity.mcc)
	mnc := reverse(ue.UeSecurity.mnc)

	if len(mnc) == 2 {
		res = fmt.Sprintf("%c%cf%c%c%c", mcc[1], mcc[2], mcc[0], mnc[0], mnc[1])
	} else {
		res = fmt.Sprintf("%c%c%c%c%c%c", mcc[1], mcc[2], mnc[0], mcc[0], mnc[1], mnc[2])
	}

	resu, err := hex.DecodeString(res)
	if err != nil {
		return nil, fmt.Errorf("could not decode string: %v", err)
	}

	return resu, nil
}

// TS 24.501 9.11.3.4.1
// Routing Indicator shall consist of 1 to 4 digits. The coding of this field is the
// responsibility of home network operator but BCD coding shall be used. If a network
// operator decides to assign less than 4 digits to Routing Indicator, the remaining digits
// shall be coded as "1111" to fill the 4 digits coding of Routing Indicator (see NOTE 2). If
// no Routing Indicator is configured in the USIM, the UE shall coxde bits 1 to 4 of octet 8
// of the Routing Indicator as "0000" and the remaining digits as “1111".
func (ue *UE) GetRoutingIndicatorInOctets() ([]byte, error) {
	if len(ue.UeSecurity.RoutingIndicator) == 0 {
		ue.UeSecurity.RoutingIndicator = "0"
	}

	if len(ue.UeSecurity.RoutingIndicator) > 4 {
		return nil, fmt.Errorf("routing indicator must be 4 digits maximum, %s is invalid", ue.UeSecurity.RoutingIndicator)
	}

	routingIndicator := []byte(ue.UeSecurity.RoutingIndicator)
	for len(routingIndicator) < 4 {
		routingIndicator = append(routingIndicator, 'F')
	}

	// Reverse the bytes in group of two
	for i := 1; i < len(routingIndicator); i += 2 {
		tmp := routingIndicator[i-1]
		routingIndicator[i-1] = routingIndicator[i]
		routingIndicator[i] = tmp
	}

	// BCD conversion
	encodedRoutingIndicator, err := hex.DecodeString(string(routingIndicator))
	if err != nil {
		return nil, fmt.Errorf("unable to encode routing indicator %s", err)
	}

	return encodedRoutingIndicator, nil
}

func reverse(s string) string {
	var aux string
	for _, valor := range s {
		aux = string(valor) + aux
	}

	return aux
}

func (ue *UE) DeriveRESstarAndSetKey(authSubs models.AuthenticationSubscription, RAND []byte, snName string, AUTN []byte) ([]byte, error) {
	OPC, err := hex.DecodeString(authSubs.EncOpcKey)
	if err != nil {
		return nil, fmt.Errorf("could not decode OPC: %v", err)
	}

	K, err := hex.DecodeString(authSubs.EncPermanentKey)
	if err != nil {
		return nil, fmt.Errorf("could not decode K: %v", err)
	}

	sqnUe, err := hex.DecodeString(authSubs.SequenceNumber.Sqn)
	if err != nil {
		return nil, fmt.Errorf("could not decode SQN: %v", err)
	}

	sqnHn, AK, IK, CK, RES, err := milenage.GenerateKeysWithAUTN(OPC, K, RAND, AUTN)
	if err != nil {
		return nil, errors.New("milenage MAC failure")
	}

	if bytes.Compare(sqnUe, sqnHn) > 0 {
		auts, err := milenage.GenerateAUTS(OPC, K, RAND, sqnUe)
		if err != nil {
			return auts, fmt.Errorf("AUTS generation error: %v", err)
		}

		return auts, errors.New("sequence number out of range")
	}

	authSubs.SequenceNumber = &models.SequenceNumber{
		Sqn: fmt.Sprintf("%08x", sqnHn),
	}

	key := append(CK, IK...)
	FC := ueauth.FC_FOR_RES_STAR_XRES_STAR_DERIVATION
	P0 := []byte(snName)
	P1 := RAND
	P2 := RES

	err = ue.DerivateKamf(key, snName, sqnHn, AK)
	if err != nil {
		return nil, fmt.Errorf("error while deriving Kamf: %v", err)
	}

	kdfVal_for_resStar, err := ueauth.GetKDFValue(key, FC, P0, ueauth.KDFLen(P0), P1, ueauth.KDFLen(P1), P2, ueauth.KDFLen(P2))
	if err != nil {
		return nil, fmt.Errorf("error while deriving KDF: %v", err)
	}

	return kdfVal_for_resStar[len(kdfVal_for_resStar)/2:], nil
}

func (ue *UE) DerivateKamf(key []byte, snName string, SQN, AK []byte) error {
	FC := ueauth.FC_FOR_KAUSF_DERIVATION
	P0 := []byte(snName)
	SQNxorAK := make([]byte, 6)

	for i := range SQN {
		SQNxorAK[i] = SQN[i] ^ AK[i]
	}

	P1 := SQNxorAK

	Kausf, err := ueauth.GetKDFValue(key, FC, P0, ueauth.KDFLen(P0), P1, ueauth.KDFLen(P1))
	if err != nil {
		return fmt.Errorf("error while deriving Kausf: %v", err)
	}

	P0 = []byte(snName)

	Kseaf, err := ueauth.GetKDFValue(Kausf, ueauth.FC_FOR_KSEAF_DERIVATION, P0, ueauth.KDFLen(P0))
	if err != nil {
		return fmt.Errorf("error while deriving Kseaf: %v", err)
	}

	supiRegexp, _ := regexp.Compile("(?:imsi|supi)-([0-9]{5,15})")
	groups := supiRegexp.FindStringSubmatch(ue.UeSecurity.Supi)

	P0 = []byte(groups[1])
	L0 := ueauth.KDFLen(P0)
	P1 = []byte{0x00, 0x00}
	L1 := ueauth.KDFLen(P1)

	ue.UeSecurity.Kamf, err = ueauth.GetKDFValue(Kseaf, ueauth.FC_FOR_KAMF_DERIVATION, P0, L0, P1, L1)
	if err != nil {
		return fmt.Errorf("error while deriving Kamf: %v", err)
	}

	return nil
}

func (ue *UE) SetAmfMccAndMnc(mcc string, mnc string) {
	ue.amfInfo.mcc = mcc
	ue.amfInfo.mnc = mnc
	ue.UeSecurity.Snn = ue.deriveSNN()
}

// Build SNN (// 5G:mnc093.mcc208.3gppnetwork.org)
func (ue *UE) deriveSNN() string {
	var resu string
	if len(ue.amfInfo.mnc) == 2 {
		resu = "5G:mnc0" + ue.amfInfo.mnc + ".mcc" + ue.amfInfo.mcc + ".3gppnetwork.org"
	} else {
		resu = "5G:mnc" + ue.amfInfo.mnc + ".mcc" + ue.amfInfo.mcc + ".3gppnetwork.org"
	}

	return resu
}

func (ue *UE) Set5gGuti(guti *nasType.GUTI5G) {
	ue.UeSecurity.Guti = guti
}

func (ue *UE) Get5gGuti() *nasType.GUTI5G {
	return ue.UeSecurity.Guti
}

func (ue *UE) GetAmfSetId() uint16 {
	return ue.UeSecurity.Guti.GetAMFSetID()
}

func (ue *UE) GetAmfPointer() uint8 {
	return ue.UeSecurity.Guti.GetAMFPointer()
}

func (ue *UE) GetTMSI5G() [4]uint8 {
	if ue.UeSecurity.Guti != nil {
		return ue.UeSecurity.Guti.GetTMSI5G()
	}

	return [4]uint8{}
}

func (ue *UE) GetSuci() nasType.MobileIdentity5GS {
	return ue.UeSecurity.Suci
}

func (ue *UE) SendDownlinkNAS(msg []byte, amfUENGAPID int64, ranUENGAPID int64) error {
	decodedMsg, err := ue.DecodeNAS(msg)
	if err != nil {
		return fmt.Errorf("could not decode NAS message: %v", err)
	}

	msgType := decodedMsg.GmmMessage.GetMessageType()

	switch msgType {
	case nas.MsgTypeAuthenticationReject:
		err := handleAuthenticationReject(ue, decodedMsg)
		if err != nil {
			return fmt.Errorf("could not handle Authentication Reject: %v", err)
		}
	case nas.MsgTypeAuthenticationRequest:
		err := handleAuthenticationRequest(ue, decodedMsg, amfUENGAPID, ranUENGAPID)
		if err != nil {
			return fmt.Errorf("could not handle Authentication Request: %v", err)
		}
	case nas.MsgTypeSecurityModeCommand:
		err := handleSecurityModeCommand(ue, decodedMsg, amfUENGAPID, ranUENGAPID)
		if err != nil {
			return fmt.Errorf("could not handle Security Mode Command: %v", err)
		}
	case nas.MsgTypeRegistrationAccept:
		err := handleRegistrationAccept(ue, decodedMsg, amfUENGAPID, ranUENGAPID)
		if err != nil {
			return fmt.Errorf("could not handle Registration Accept: %v", err)
		}
	case nas.MsgTypeRegistrationReject:
		err := handleRegistrationReject(ue, decodedMsg)
		if err != nil {
			return fmt.Errorf("could not handle Registration Reject: %v", err)
		}
	case nas.MsgTypeDeregistrationRequestUETerminatedDeregistration:
		err := handleDeregistrationRequestUETerminated(ue, decodedMsg, amfUENGAPID, ranUENGAPID)
		if err != nil {
			return fmt.Errorf("could not handle Deregistration Request UE Terminated: %v", err)
		}
	case nas.MsgTypeIdentityRequest:
		err := handleIdentityRequest(ue, amfUENGAPID, ranUENGAPID)
		if err != nil {
			return fmt.Errorf("could not handle Identity Request: %v", err)
		}
	case nas.MsgTypeServiceAccept:
		err := handleServiceAccept(ue, decodedMsg)
		if err != nil {
			return fmt.Errorf("could not handle Service Accept: %v", err)
		}
	case nas.MsgTypeDLNASTransport:
		err := handleDLNASTransport(ue, decodedMsg)
		if err != nil {
			return fmt.Errorf("could not handle DL NAS Transport: %v", err)
		}
	case nas.MsgTypeConfigurationUpdateCommand:
		err := handleConfigurationUpdateCommand(ue, decodedMsg.ConfigurationUpdateCommand, amfUENGAPID, ranUENGAPID)
		if err != nil {
			return fmt.Errorf("could not handle Configuration Update Command: %v", err)
		}
	default:
		logger.UeLogger.Warn("NAS message type not implemented", zap.Uint8("msgType", msgType))
	}

	updateReceivedGMMMessages(ue, decodedMsg)

	return nil
}

func (ue *UE) RRCRelease() {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	ue.receivedRRCRelease = true
}

func updateReceivedGMMMessages(ue *UE, msg *nas.Message) {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	msgType := msg.GmmMessage.GetMessageType()
	ue.receivedNASGMMMessages[msgType] = append(ue.receivedNASGMMMessages[msgType], msg)

	logger.UeLogger.Debug("Stored received NAS GMM Message", zap.String("msgType", getGMMMessageName(msgType)), zap.Int("totalFrames", len(ue.receivedNASGMMMessages[msgType])))
}

func updateReceivedGSMMessages(ue *UE, msg *nas.Message) {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	msgType := msg.GsmMessage.GetMessageType()
	ue.receivedNASGSMMessages[msgType] = append(ue.receivedNASGSMMessages[msgType], msg)

	logger.UeLogger.Debug("Stored received NAS GSM Message", zap.String("msgType", getGSMMessageName(msgType)), zap.Int("totalFrames", len(ue.receivedNASGSMMessages[msgType])))
}

func (ue *UE) WaitForNASGMMMessage(msgType uint8, timeout time.Duration) (*nas.Message, error) {
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		ue.mu.Lock()

		msgs, ok := ue.receivedNASGMMMessages[msgType]
		if !ok {
			ue.mu.Unlock()
			time.Sleep(1 * time.Millisecond)

			continue
		}

		if len(msgs) == 0 {
			ue.mu.Unlock()
			time.Sleep(1 * time.Millisecond)

			continue
		}

		if len(msgs) == 1 {
			delete(ue.receivedNASGMMMessages, msgType)
		} else {
			ue.receivedNASGMMMessages[msgType] = msgs[1:]
		}

		ue.mu.Unlock()

		return msgs[0], nil
	}

	return nil, fmt.Errorf("timeout waiting for NAS message %v", msgType)
}

func (ue *UE) WaitForNASGSMMessage(msgType uint8, timeout time.Duration) (*nas.Message, error) {
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		ue.mu.Lock()

		msgs, ok := ue.receivedNASGSMMessages[msgType]
		if !ok {
			ue.mu.Unlock()
			time.Sleep(1 * time.Millisecond)

			continue
		}

		msg := msgs[0]

		if len(msgs) == 1 {
			delete(ue.receivedNASGSMMessages, msgType)
		} else {
			ue.receivedNASGSMMessages[msgType] = msgs[1:]
		}

		ue.mu.Unlock()

		return msg, nil
	}

	return nil, fmt.Errorf("timeout waiting for NAS message %v", msgType)
}

func (ue *UE) WaitForRRCRelease(timeout time.Duration) error {
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		ue.mu.Lock()
		received := ue.receivedRRCRelease
		ue.receivedRRCRelease = false
		ue.mu.Unlock()

		if received {
			return nil
		}

		time.Sleep(10 * time.Millisecond)
	}

	return errors.New("timeout waiting for RRC Release")
}

func (ue *UE) SendRegistrationRequest(ranUENGAPID int64, regType uint8) error {
	if ue.Gnb == nil {
		return fmt.Errorf("GNB is not set for UE")
	}

	nasPDU, err := BuildRegistrationRequest(&RegistrationRequestOpts{
		RegistrationType:  regType,
		RequestedNSSAI:    nil,
		UplinkDataStatus:  nil,
		IncludeCapability: false,
		UESecurity:        ue.UeSecurity,
	})
	if err != nil {
		return fmt.Errorf("could not build Registration Request NAS PDU: %v", err)
	}

	err = ue.Gnb.SendInitialUEMessage(nasPDU, ranUENGAPID, ue.UeSecurity.Guti, ngapType.RRCEstablishmentCausePresentMoSignalling)
	if err != nil {
		return fmt.Errorf("could not send UplinkNASTransport: %v", err)
	}

	logger.UeLogger.Debug(
		"Sent Security Mode Complete NAS message",
		zap.String("IMSI", ue.UeSecurity.Supi),
	)

	return nil
}

func (ue *UE) SendServiceRequest(ranUENGAPID int64, pduSessionStatus [16]bool, serviceType uint8) error {
	serviceRequest, err := BuildServiceRequest(&ServiceRequestOpts{
		ServiceType:      serviceType,
		AMFSetID:         ue.GetAmfSetId(),
		AMFPointer:       ue.GetAmfPointer(),
		TMSI5G:           ue.GetTMSI5G(),
		PDUSessionStatus: &pduSessionStatus,
		UESecurity:       ue.UeSecurity,
	})
	if err != nil {
		return fmt.Errorf("could not build Service Request NAS PDU: %v", err)
	}

	encodedPdu, err := ue.EncodeNasPduWithSecurity(serviceRequest, nas.SecurityHeaderTypeIntegrityProtected)
	if err != nil {
		return fmt.Errorf("error encoding %s IMSI UE  NAS Security Mode Complete message: %v", ue.UeSecurity.Supi, err)
	}

	establishmentCause := ngapType.RRCEstablishmentCausePresentMoData
	if serviceType == nasMessage.ServiceTypeMobileTerminatedServices {
		establishmentCause = ngapType.RRCEstablishmentCausePresentMtAccess
	}

	err = ue.Gnb.SendInitialUEMessage(encodedPdu, ranUENGAPID, ue.UeSecurity.Guti, establishmentCause)
	if err != nil {
		return fmt.Errorf("could not send UplinkNASTransport: %v", err)
	}

	logger.UeLogger.Debug(
		"Sent Service Request NAS message",
		zap.String("IMSI", ue.UeSecurity.Supi),
		zap.Int64("RAN UE NGAP ID", ranUENGAPID),
	)

	return nil
}

func (ue *UE) SendDeregistrationRequest(amfUENGAPID int64, ranUENGAPID int64) error {
	deregBytes, err := BuildDeregistrationRequest(&DeregistrationRequestOpts{
		Guti: ue.UeSecurity.Guti,
		Ksi:  ue.UeSecurity.NgKsi.Ksi,
	})
	if err != nil {
		return fmt.Errorf("could not build Deregistration Request NAS PDU: %v", err)
	}

	encodedPdu, err := ue.EncodeNasPduWithSecurity(deregBytes, nas.SecurityHeaderTypeIntegrityProtectedAndCiphered)
	if err != nil {
		return fmt.Errorf("error encoding %s IMSI UE NAS Deregistration Msg", ue.UeSecurity.Supi)
	}

	err = ue.Gnb.SendUplinkNAS(encodedPdu, amfUENGAPID, ranUENGAPID)
	if err != nil {
		return fmt.Errorf("could not send UplinkNASTransport: %v", err)
	}

	logger.UeLogger.Debug(
		"Sent Deregistration Request NAS message",
		zap.String("IMSI", ue.UeSecurity.Supi),
	)

	return nil
}

func (ue *UE) SendPDUSessionEstablishmentRequest(amfUENGAPID int64, ranUENGAPID int64) error {
	pduReq, err := BuildPduSessionEstablishmentRequest(&PduSessionEstablishmentRequestOpts{
		PDUSessionID:   ue.PDUSessionID,
		PDUSessionType: ue.PDUSessionType,
	})
	if err != nil {
		return fmt.Errorf("could not build PDU Session Establishment Request: %v", err)
	}

	pduUplink, err := BuildUplinkNasTransport(&UplinkNasTransportOpts{
		PDUSessionID:     ue.PDUSessionID,
		PayloadContainer: pduReq,
		DNN:              ue.DNN,
		SNSSAI:           ue.Snssai,
	})
	if err != nil {
		return fmt.Errorf("could not build Uplink NAS Transport for PDU Session: %v", err)
	}

	encodedPdu, err := ue.EncodeNasPduWithSecurity(pduUplink, nas.SecurityHeaderTypeIntegrityProtectedAndCiphered)
	if err != nil {
		return fmt.Errorf("error encoding %s IMSI UE NAS Uplink NAS Transport for PDU Session Msg", ue.UeSecurity.Supi)
	}

	err = ue.Gnb.SendUplinkNAS(encodedPdu, amfUENGAPID, ranUENGAPID)
	if err != nil {
		return fmt.Errorf("could not send UplinkNASTransport for PDU Session Establishment: %v", err)
	}

	logger.UeLogger.Debug(
		"Sent PDU Session Establishment Request",
		zap.String("IMSI", ue.UeSecurity.Supi),
	)

	return nil
}
