// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ue

import (
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"sync"
	"time"

	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/tester/air"
	"github.com/ellanetworks/core/internal/tester/logger"
	"github.com/ellanetworks/core/internal/tester/ue/sidf"
	"github.com/ellanetworks/core/internal/util/milenage"
	"github.com/ellanetworks/core/internal/util/ueauth"
	"github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/nas/eps"
	"github.com/ellanetworks/core/nas/fgs"
	"github.com/ellanetworks/core/ngap"
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

// ngKSINoKey is the ngKSI "no key is available" value (TS 24.501 §9.11.3.32):
// the UE holds no current 5G NAS security context.
const ngKSINoKey = 7

type UESecurity struct {
	Supi    string
	Msin    string
	mcc     string
	mnc     string
	ULCount nas.Count
	// DLRecv is the receive-side counter of the downlink direction. It is a
	// nas.UplinkCounter because that type is the receiver side of a NAS COUNT and is
	// direction-agnostic: the direction feeds the crypto, not the counter.
	DLRecv                    nas.UplinkCounter
	UeSecurityCapability      fgs.UESecurityCapability
	IntegrityAlg              uint8
	CipheringAlg              uint8
	NgKsi                     models.NgKsi
	Snn                       string
	KnasEnc                   [16]uint8
	KnasInt                   [16]uint8
	Kamf                      []uint8
	AuthenticationSubs        AuthenticationSubscription
	Suci                      fgs.MobileIdentity // the UE's SUCI
	suciPublicKey             sidf.HomeNetworkPublicKey
	RoutingIndicator          string
	Guti                      *fgs.MobileIdentity // the assigned 5G-GUTI, nil until the network allocates one
	S1UENetworkCapability     *eps.UENetworkCapability
	EPSNASAlgorithms          *fgs.SelectedEPSNASSecurityAlgorithms
	contextFromAuthentication bool
}

type Amf struct {
	mcc string
	mnc string
}

type PDUSessionInfo struct {
	PDUSessionID uint8
	UEIPv4       string
	UEIPv6       string
	MTU          uint16
	QFI          uint8
}

type LPPRequest struct {
	TransactionID byte
}

type UE struct {
	UeSecurity             *UESecurity
	StateMM                int
	DNN                    string
	PDUSessionID           uint8
	PDUSessionType         fgs.PDUSessionType
	NoAutoPDUSession       bool
	Snssai                 models.Snssai
	amfInfo                Amf
	IMEISV                 string
	replayRegistration     []byte
	Gnb                    air.UplinkSender
	mu                     sync.Mutex
	cond                   *sync.Cond
	pduSessions            map[uint8]PDUSessionInfo
	receivedNASGMMMessages map[uint8][][]byte // msgType -> plaintext GMM messages
	receivedNASGSMMessages map[uint8][][]byte // msgType -> plaintext GSM messages
	receivedRRCRelease     bool
	lppRequests            []*LPPRequest // queue of received LPP requests
	lppCapsSent            bool          // true after first ProvideLocationCapabilities
	requestedReactivation  bool
	lastPTI                uint8
}

func (ue *UE) SetPDUSession(pduSession PDUSessionInfo) {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	ue.pduSessions[pduSession.PDUSessionID] = pduSession
	ue.cond.Broadcast()
}

// DropPDUSession forgets a PDU session the network released, so the PDU
// session status the UE reports on its next Service Request or Registration
// Request (TS 24.501 §9.11.3.44) no longer claims it.
func (ue *UE) DropPDUSession(pduSessionID uint8) {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	delete(ue.pduSessions, pduSessionID)
	ue.cond.Broadcast()
}

// nextPTI allocates the procedure transaction identity for a UE-requested 5GSM
// procedure, skipping the unassigned (0) and reserved (255) values of
// TS 24.007 §11.2.3.1a.
func (ue *UE) nextPTI() uint8 {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	ue.lastPTI++
	if ue.lastPTI == 0 || ue.lastPTI == 0xff {
		ue.lastPTI = 1
	}

	return ue.lastPTI
}

func (ue *UE) ActivePDUSessionIDs() []uint8 {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	ids := make([]uint8, 0, len(ue.pduSessions))
	for id := range ue.pduSessions {
		ids = append(ids, id)
	}

	slices.Sort(ids)

	return ids
}

func (ue *UE) GetPDUSession(pduSessionID uint8) PDUSessionInfo {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	return ue.pduSessions[pduSessionID]
}

func (ue *UE) WaitForPDUSession(pduSessionID uint8, timeout time.Duration) (PDUSessionInfo, error) {
	deadline := time.Now().Add(timeout)

	timer := time.AfterFunc(timeout, func() {
		ue.cond.Broadcast()
	})
	defer timer.Stop()

	ue.mu.Lock()
	defer ue.mu.Unlock()

	for {
		if session, ok := ue.pduSessions[pduSessionID]; ok {
			return session, nil
		}

		if time.Now().After(deadline) {
			return PDUSessionInfo{}, fmt.Errorf("timeout waiting for PDU session %d", pduSessionID)
		}

		ue.cond.Wait()
	}
}

type UEOpts struct {
	PDUSessionID          uint8
	PDUSessionType        fgs.PDUSessionType
	Msin                  string
	UeSecurityCapability  fgs.UESecurityCapability
	K                     string
	OpC                   string
	Amf                   string
	Sqn                   string
	Mcc                   string
	Mnc                   string
	HomeNetworkPublicKey  sidf.HomeNetworkPublicKey
	RoutingIndicator      string
	DNN                   string
	Sst                   int32
	Sd                    string
	IMEISV                string
	Guti                  *fgs.MobileIdentity
	GnodeB                air.UplinkSender
	NoAutoPDUSession      bool
	S1UENetworkCapability *eps.UENetworkCapability
}

var DefaultS1UENetworkCapability = eps.UENetworkCapability{EEA: 0xf0, EIA: 0x70}

func NewUE(opts *UEOpts) (*UE, error) {
	ue := UE{}
	ue.UeSecurity = &UESecurity{}
	ue.UeSecurity.Msin = opts.Msin
	ue.Gnb = opts.GnodeB
	ue.PDUSessionID = opts.PDUSessionID
	ue.PDUSessionType = opts.PDUSessionType
	ue.NoAutoPDUSession = opts.NoAutoPDUSession

	ue.UeSecurity.UeSecurityCapability = opts.UeSecurityCapability

	s1Capability := DefaultS1UENetworkCapability
	if opts.S1UENetworkCapability != nil {
		s1Capability = *opts.S1UENetworkCapability
	}

	ue.UeSecurity.S1UENetworkCapability = &s1Capability

	integAlg, CipherAlg := SelectAlgorithms(ue.UeSecurity.UeSecurityCapability)

	ue.UeSecurity.IntegrityAlg = integAlg
	ue.UeSecurity.CipheringAlg = CipherAlg
	ue.UeSecurity.NgKsi.Ksi = ngKSINoKey
	ue.UeSecurity.NgKsi.Tsc = models.ScTypeNative

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

	ue.pduSessions = make(map[uint8]PDUSessionInfo)
	ue.receivedNASGMMMessages = make(map[uint8][][]byte)
	ue.receivedNASGSMMessages = make(map[uint8][][]byte)
	ue.lppRequests = make([]*LPPRequest, 0)

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
	ue.cond = sync.NewCond(&ue.mu)

	return &ue, nil
}

func (ue *UE) SetAuthSubscription(k, opc, amf, sqn string) {
	ue.UeSecurity.AuthenticationSubs.EncPermanentKey = k
	ue.UeSecurity.AuthenticationSubs.EncOpcKey = opc

	ue.UeSecurity.AuthenticationSubs.AuthenticationManagementField = amf

	ue.UeSecurity.AuthenticationSubs.SequenceNumber = &SequenceNumber{
		Sqn: sqn,
	}
	ue.UeSecurity.AuthenticationSubs.AuthenticationMethod = AuthMethod5GAKA
}

// EncodeSuci returns the UE's SUCI as a 5GS mobile identity (TS 24.501
// §9.11.3.4).
func (ue *UE) EncodeSuci() (fgs.MobileIdentity, error) {
	protScheme, err := strconv.ParseUint(ue.UeSecurity.suciPublicKey.ProtectionScheme, 10, 8)
	if err != nil {
		return fgs.MobileIdentity{}, fmt.Errorf("unable to parse protection scheme: %v", err)
	}

	hnPubKeyID, err := strconv.ParseUint(ue.UeSecurity.suciPublicKey.PublicKeyID, 10, 8)
	if err != nil {
		return fgs.MobileIdentity{}, fmt.Errorf("unable to parse home network public key ID: %v", err)
	}

	var schemeOutput []byte

	if protScheme == 0 {
		schemeOutput, err = hex.DecodeString(sidf.Tbcd(ue.UeSecurity.Msin))
		if err != nil {
			return fgs.MobileIdentity{}, fmt.Errorf("unable to decode msin to tbcd: %v", err)
		}
	} else {
		suci, err := sidf.CipherSuci(ue.UeSecurity.Msin, ue.UeSecurity.mcc, ue.UeSecurity.mnc, ue.UeSecurity.RoutingIndicator, ue.UeSecurity.suciPublicKey)
		if err != nil {
			return fgs.MobileIdentity{}, fmt.Errorf("unable to cipher SUCI: %v", err)
		}

		schemeOutput, err = hex.DecodeString(suci.SchemeOutput)
		if err != nil {
			return fgs.MobileIdentity{}, fmt.Errorf("unable to decode scheme output to bytes: %v", err)
		}
	}

	return fgs.SUCIIdentity(fgs.SUCI{
		PLMN:             nas.PLMN{MCC: ue.UeSecurity.mcc, MNC: ue.UeSecurity.mnc},
		RoutingIndicator: ue.UeSecurity.RoutingIndicator,
		ProtectionScheme: fgs.ProtectionScheme(protScheme),
		HomeNetworkPKID:  uint8(hnPubKeyID),
		SchemeOutput:     schemeOutput,
	}), nil
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

// TS 24.501 §9.11.3.4.1: the routing indicator is 1-4 BCD digits; unused digits
// are coded as "1111" (0xF) to fill the 4-digit field.
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

	for i := 1; i < len(routingIndicator); i += 2 {
		tmp := routingIndicator[i-1]
		routingIndicator[i-1] = routingIndicator[i]
		routingIndicator[i] = tmp
	}

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

// ErrMACFailure and ErrSQNOutOfRange are the two AUTN checks a UE runs on an
// AUTHENTICATION REQUEST (TS 33.501 §6.1.3.2). Each has its own 5GMM cause in
// the AUTHENTICATION FAILURE that answers it, and the sequence-number one
// carries an AUTS for the network to resynchronise from.
var (
	ErrMACFailure    = errors.New("milenage MAC failure")
	ErrSQNOutOfRange = errors.New("sequence number out of range")
)

func (ue *UE) DeriveRESstarAndSetKey(authSubs AuthenticationSubscription, RAND []byte, snName string, AUTN []byte) ([]byte, error) {
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
		return nil, ErrMACFailure
	}

	if bytes.Compare(sqnUe, sqnHn) > 0 {
		auts, err := milenage.GenerateAUTS(OPC, K, RAND, sqnUe)
		if err != nil {
			return nil, fmt.Errorf("AUTS generation error: %v", err)
		}

		return auts, ErrSQNOutOfRange
	}

	authSubs.SequenceNumber = &SequenceNumber{
		Sqn: fmt.Sprintf("%08x", sqnHn),
	}

	key := append(CK, IK...)
	FC := ueauth.FCForResStarXresStarDerivation
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
	ue.UeSecurity.contextFromAuthentication = true

	FC := ueauth.FCForKausfDerivation
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

	Kseaf, err := ueauth.GetKDFValue(Kausf, ueauth.FCForKseafDerivation, P0, ueauth.KDFLen(P0))
	if err != nil {
		return fmt.Errorf("error while deriving Kseaf: %v", err)
	}

	supiRegexp, _ := regexp.Compile("(?:imsi|supi)-([0-9]{5,15})")
	groups := supiRegexp.FindStringSubmatch(ue.UeSecurity.Supi)

	P0 = []byte(groups[1])
	L0 := ueauth.KDFLen(P0)
	P1 = []byte{0x00, 0x00}
	L1 := ueauth.KDFLen(P1)

	ue.UeSecurity.Kamf, err = ueauth.GetKDFValue(Kseaf, ueauth.FCForKamfDerivation, P0, L0, P1, L1)
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

func (ue *UE) deriveSNN() string {
	var resu string
	if len(ue.amfInfo.mnc) == 2 {
		resu = "5G:mnc0" + ue.amfInfo.mnc + ".mcc" + ue.amfInfo.mcc + ".3gppnetwork.org"
	} else {
		resu = "5G:mnc" + ue.amfInfo.mnc + ".mcc" + ue.amfInfo.mcc + ".3gppnetwork.org"
	}

	return resu
}

func (ue *UE) Set5gGuti(guti *fgs.MobileIdentity) {
	ue.UeSecurity.Guti = guti
}

func (ue *UE) Get5gGuti() *fgs.MobileIdentity {
	return ue.UeSecurity.Guti
}

func (ue *UE) guti() *fgs.GUTI {
	if ue.UeSecurity.Guti == nil {
		return nil
	}

	return ue.UeSecurity.Guti.GUTI
}

func (ue *UE) GetAmfSetId() uint16 {
	if g := ue.guti(); g != nil {
		return g.AMFSetID
	}

	return 0
}

func (ue *UE) GetAmfPointer() uint8 {
	if g := ue.guti(); g != nil {
		return g.AMFPointer
	}

	return 0
}

func (ue *UE) GetTMSI5G() [4]uint8 {
	if g := ue.guti(); g != nil {
		return g.TMSI
	}

	return [4]uint8{}
}

func (ue *UE) GetSuci() fgs.MobileIdentity {
	return ue.UeSecurity.Suci
}

func (ue *UE) SendDownlinkNAS(msg []byte, amfUENGAPID int64, ranUENGAPID int64) error {
	plain, err := ue.DecodeNAS(msg)
	if err != nil {
		return fmt.Errorf("could not decode NAS message: %v", err)
	}

	if len(plain) < 3 {
		return fmt.Errorf("decoded NAS message too short: %d bytes", len(plain))
	}

	msgType := plain[2]

	switch fgs.MessageType(msgType) {
	case fgs.MsgAuthenticationReject:
		err := handleAuthenticationReject(ue, plain)
		if err != nil {
			return fmt.Errorf("could not handle Authentication Reject: %v", err)
		}
	case fgs.MsgAuthenticationRequest:
		err := handleAuthenticationRequest(ue, plain, amfUENGAPID, ranUENGAPID)
		if err != nil {
			return fmt.Errorf("could not handle Authentication Request: %v", err)
		}
	case fgs.MsgSecurityModeCommand:
		err := handleSecurityModeCommand(ue, plain, amfUENGAPID, ranUENGAPID)
		if err != nil {
			return fmt.Errorf("could not handle Security Mode Command: %v", err)
		}
	case fgs.MsgRegistrationAccept:
		err := handleRegistrationAccept(ue, plain, amfUENGAPID, ranUENGAPID)
		if err != nil {
			return fmt.Errorf("could not handle Registration Accept: %v", err)
		}
	case fgs.MsgRegistrationReject:
		err := handleRegistrationReject(ue, plain)
		if err != nil {
			return fmt.Errorf("could not handle Registration Reject: %v", err)
		}
	case fgs.MsgDeregistrationRequestUETerm:
		err := handleDeregistrationRequestUETerminated(ue, plain, amfUENGAPID, ranUENGAPID)
		if err != nil {
			return fmt.Errorf("could not handle Deregistration Request UE Terminated: %v", err)
		}
	case fgs.MsgIdentityRequest:
		err := handleIdentityRequest(ue, amfUENGAPID, ranUENGAPID)
		if err != nil {
			return fmt.Errorf("could not handle Identity Request: %v", err)
		}
	case fgs.MsgServiceAccept:
		err := handleServiceAccept(ue, plain)
		if err != nil {
			return fmt.Errorf("could not handle Service Accept: %v", err)
		}
	case fgs.MsgDLNASTransport:
		err := handleDLNASTransport(ue, plain, amfUENGAPID, ranUENGAPID)
		if err != nil {
			return fmt.Errorf("could not handle DL NAS Transport: %v", err)
		}
	case fgs.MsgConfigurationUpdateCommand:
		err := handleConfigurationUpdateCommand(ue, plain, amfUENGAPID, ranUENGAPID)
		if err != nil {
			return fmt.Errorf("could not handle Configuration Update Command: %v", err)
		}
	default:
		logger.UeLogger.Warn("NAS message type not implemented", zap.Uint8("msgType", msgType))
	}

	updateReceivedGMMMessages(ue, plain)

	return nil
}

func (ue *UE) RRCRelease() {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	ue.receivedRRCRelease = true
	ue.cond.Broadcast()
}

// updateReceivedGMMMessages archives a plaintext 5GMM message keyed by its type
// (octet 3 of a plain 5GMM message: EPD, security header, message type).
func updateReceivedGMMMessages(ue *UE, plain []byte) {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	msgType := plain[2]
	ue.receivedNASGMMMessages[msgType] = append(ue.receivedNASGMMMessages[msgType], plain)

	logger.UeLogger.Debug("Stored received NAS GMM Message", zap.String("msgType", getGMMMessageName(msgType)), zap.Int("totalFrames", len(ue.receivedNASGMMMessages[msgType])))
	ue.cond.Broadcast()
}

// updateReceivedGSMMessages archives a plaintext 5GSM message keyed by its type
// (octet 4 of a 5GSM message: EPD, PDU session ID, PTI, message type).
func updateReceivedGSMMessages(ue *UE, plain []byte) {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	msgType := plain[3]
	ue.receivedNASGSMMessages[msgType] = append(ue.receivedNASGSMMessages[msgType], plain)

	logger.UeLogger.Debug("Stored received NAS GSM Message", zap.String("msgType", getGSMMessageName(msgType)), zap.Int("totalFrames", len(ue.receivedNASGSMMessages[msgType])))
	ue.cond.Broadcast()
}

func (ue *UE) ReceivedNASGMMCount(msgType uint8) int {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	return len(ue.receivedNASGMMMessages[msgType])
}

func (ue *UE) WaitForNASGMMMessage(msgType uint8, timeout time.Duration) ([]byte, error) {
	deadline := time.Now().Add(timeout)

	timer := time.AfterFunc(timeout, func() {
		ue.cond.Broadcast()
	})
	defer timer.Stop()

	ue.mu.Lock()
	defer ue.mu.Unlock()

	for {
		msgs, ok := ue.receivedNASGMMMessages[msgType]
		if ok && len(msgs) > 0 {
			if len(msgs) == 1 {
				delete(ue.receivedNASGMMMessages, msgType)
			} else {
				ue.receivedNASGMMMessages[msgType] = msgs[1:]
			}

			return msgs[0], nil
		}

		if time.Now().After(deadline) {
			return nil, fmt.Errorf("timeout waiting for NAS message %v", msgType)
		}

		ue.cond.Wait()
	}
}

func (ue *UE) WaitForNASGSMMessage(msgType uint8, timeout time.Duration) ([]byte, error) {
	deadline := time.Now().Add(timeout)

	timer := time.AfterFunc(timeout, func() {
		ue.cond.Broadcast()
	})
	defer timer.Stop()

	ue.mu.Lock()
	defer ue.mu.Unlock()

	for {
		msgs, ok := ue.receivedNASGSMMessages[msgType]
		if ok && len(msgs) > 0 {
			msg := msgs[0]

			if len(msgs) == 1 {
				delete(ue.receivedNASGSMMessages, msgType)
			} else {
				ue.receivedNASGSMMessages[msgType] = msgs[1:]
			}

			return msg, nil
		}

		if time.Now().After(deadline) {
			return nil, fmt.Errorf("timeout waiting for NAS message %v", msgType)
		}

		ue.cond.Wait()
	}
}

func (ue *UE) WaitForRRCRelease(timeout time.Duration) error {
	deadline := time.Now().Add(timeout)

	timer := time.AfterFunc(timeout, func() {
		ue.cond.Broadcast()
	})
	defer timer.Stop()

	ue.mu.Lock()
	defer ue.mu.Unlock()

	for {
		if ue.receivedRRCRelease {
			ue.receivedRRCRelease = false
			return nil
		}

		if time.Now().After(deadline) {
			return errors.New("timeout waiting for RRC Release")
		}

		ue.cond.Wait()
	}
}

func (ue *UE) SendRegistrationRequest(ranUENGAPID int64, regType uint8) error {
	return ue.sendRegistrationRequest(ranUENGAPID, regType, nil)
}

func (ue *UE) SendMobilityRegistrationRequest(ranUENGAPID int64, reactivate []uint8) error {
	var uplinkDataStatus [16]bool

	for _, pduSessionID := range reactivate {
		if int(pduSessionID) >= len(uplinkDataStatus) {
			return fmt.Errorf("PDU session ID %d is out of range", pduSessionID)
		}

		uplinkDataStatus[pduSessionID] = true
	}

	return ue.sendRegistrationRequest(ranUENGAPID, uint8(fgs.RegistrationTypeMobilityUpdating), &uplinkDataStatus)
}

func (ue *UE) setRequestedReactivation(requested bool) {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	ue.requestedReactivation = requested
}

// followOnRequestPending reports what the UE puts in the Follow-On Request bit
// of a REGISTRATION REQUEST (TS 24.501 §9.11.3.14): the bit claims the UE has
// more NAS signalling to send, which for this UE means the PDU SESSION
// ESTABLISHMENT REQUEST it follows a registration with. A UE that establishes
// no session, and one that is asking for its user plane back through the uplink
// data status instead, both leave it clear; with it clear and nothing else
// pending the AMF releases the connection on the Registration Complete
// (TS 24.501 §5.5.1.2.4).
//
// The handsets on the reference network agree: initial registrations set the
// bit, mobility registration updates — which carry their PDU session status
// instead — clear it.
func (ue *UE) followOnRequestPending(uplinkDataStatus *[16]bool) bool {
	return !ue.NoAutoPDUSession && uplinkDataStatus == nil
}

func (ue *UE) skipAutoPDUSession() bool {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	return ue.NoAutoPDUSession || ue.requestedReactivation
}

func (ue *UE) sendRegistrationRequest(ranUENGAPID int64, regType uint8, uplinkDataStatus *[16]bool) error {
	if ue.Gnb == nil {
		return fmt.Errorf("GNB is not set for UE")
	}

	var err error

	var s1Capability []byte

	if ue.UeSecurity.S1UENetworkCapability != nil {
		if s1Capability, err = ue.UeSecurity.S1UENetworkCapability.MarshalBinary(); err != nil {
			return fmt.Errorf("could not encode the S1 UE network capability: %w", err)
		}
	}

	nasPDU, complete, err := buildRegistrationRequest(&RegistrationRequestOpts{
		RegistrationType:      regType,
		FollowOnRequest:       ue.followOnRequestPending(uplinkDataStatus),
		RequestedNSSAI:        nil,
		UplinkDataStatus:      uplinkDataStatus,
		IncludeCapability:     regType != uint8(fgs.RegistrationTypePeriodicUpdating),
		S1UENetworkCapability: s1Capability,
		UESecurity:            ue.UeSecurity,
		InitialNASMessage:     true,
	})
	if err != nil {
		return fmt.Errorf("could not build Registration Request NAS PDU: %v", err)
	}

	ue.replayRegistration = complete

	// TS 24.501 §4.4.6: a UE with a current 5G NAS security context integrity
	// protects the initial NAS message of a new connection, so the AMF can
	// verify it and reuse the context; without one the message stays plain.
	if ue.UeSecurity.NgKsi.Ksi != ngKSINoKey {
		nasPDU, err = ue.EncodeNasPduWithSecurity(nasPDU, uint8(fgs.SHTIntegrityProtected))
		if err != nil {
			return fmt.Errorf("could not integrity-protect Registration Request NAS PDU: %v", err)
		}
	}

	var gutiIE []byte
	if ue.UeSecurity.Guti != nil {
		if gutiIE, err = ue.UeSecurity.Guti.MarshalBinary(); err != nil {
			return fmt.Errorf("could not encode 5G-GUTI: %v", err)
		}
	}

	err = ue.Gnb.SendInitialUEMessage(nasPDU, ranUENGAPID, gutiIE, ngap.RRCCauseMOSignalling)
	if err != nil {
		return fmt.Errorf("could not send UplinkNASTransport: %v", err)
	}

	ue.setRequestedReactivation(uplinkDataStatus != nil)

	logger.UeLogger.Debug(
		"Sent Registration Request NAS message",
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

	encodedPdu, err := ue.EncodeNasPduWithSecurity(serviceRequest, uint8(fgs.SHTIntegrityProtected))
	if err != nil {
		return fmt.Errorf("error encoding %s IMSI UE  NAS Security Mode Complete message: %v", ue.UeSecurity.Supi, err)
	}

	establishmentCause := ngap.RRCCauseMOData

	switch fgs.ServiceType(serviceType) {
	case fgs.ServiceTypeMobileTerminatedServices:
		establishmentCause = ngap.RRCCauseMTAccess
	case fgs.ServiceTypeSignalling, fgs.ServiceTypeElevatedSignalling:
		establishmentCause = ngap.RRCCauseMOSignalling
	}

	var gutiIE []byte
	if ue.UeSecurity.Guti != nil {
		if gutiIE, err = ue.UeSecurity.Guti.MarshalBinary(); err != nil {
			return fmt.Errorf("could not encode 5G-GUTI: %v", err)
		}
	}

	err = ue.Gnb.SendInitialUEMessage(encodedPdu, ranUENGAPID, gutiIE, establishmentCause)
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

	encodedPdu, err := ue.EncodeNasPduWithSecurity(deregBytes, uint8(fgs.SHTIntegrityProtectedCiphered))
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

func (ue *UE) SendPDUSessionEstablishmentRequest(amfUENGAPID int64, ranUENGAPID int64, pduSessionID uint8, dnn string, snssai models.Snssai) error {
	return ue.sendPDUSessionRequest(amfUENGAPID, ranUENGAPID, pduSessionID, dnn, snssai, fgs.RequestTypeInitialRequest)
}

func (ue *UE) MovePDUSessionFromEPC(amfUENGAPID int64, ranUENGAPID int64, pduSessionID uint8, dnn string, snssai models.Snssai) error {
	return ue.sendPDUSessionRequest(amfUENGAPID, ranUENGAPID, pduSessionID, dnn, snssai, fgs.RequestTypeExistingPDUSession)
}

// SendPDUSessionReleaseRequest starts a UE-requested PDU session release
// (TS 23.502 §4.3.4.2, TS 24.501 §6.4.3.2). The network answers with a PDU
// SESSION RELEASE COMMAND carrying the PTI allocated here, which
// handlePDUSessionReleaseCommand completes.
func (ue *UE) SendPDUSessionReleaseRequest(amfUENGAPID int64, ranUENGAPID int64, pduSessionID uint8) (uint8, error) {
	pti := ue.nextPTI()

	release, err := BuildPDUSessionReleaseRequest(&PDUSessionReleaseRequestOpts{
		PDUSessionID: pduSessionID,
		PTI:          pti,
		Cause:        fgs.GSMCauseRegularDeactivation,
	})
	if err != nil {
		return 0, fmt.Errorf("could not build PDU Session Release Request: %v", err)
	}

	uplink, err := BuildUplinkNasTransportSM(pduSessionID, release)
	if err != nil {
		return 0, fmt.Errorf("could not build Uplink NAS Transport for PDU Session Release: %v", err)
	}

	encodedPdu, err := ue.EncodeNasPduWithSecurity(uplink, uint8(fgs.SHTIntegrityProtectedCiphered))
	if err != nil {
		return 0, fmt.Errorf("error encoding %s IMSI UE NAS Uplink NAS Transport for PDU Session Release Msg", ue.UeSecurity.Supi)
	}

	if err := ue.Gnb.SendUplinkNAS(encodedPdu, amfUENGAPID, ranUENGAPID); err != nil {
		return 0, fmt.Errorf("could not send UplinkNASTransport for PDU Session Release: %v", err)
	}

	logger.UeLogger.Debug(
		"Sent PDU Session Release Request",
		zap.String("IMSI", ue.UeSecurity.Supi),
		zap.Uint8("PDU Session ID", pduSessionID),
		zap.Uint8("PTI", pti),
	)

	return pti, nil
}

func (ue *UE) sendPDUSessionRequest(amfUENGAPID int64, ranUENGAPID int64, pduSessionID uint8, dnn string, snssai models.Snssai, requestType fgs.RequestType) error {
	pduReq, err := BuildPduSessionEstablishmentRequest(&PduSessionEstablishmentRequestOpts{
		PDUSessionID:   pduSessionID,
		PDUSessionType: ue.PDUSessionType,
	})
	if err != nil {
		return fmt.Errorf("could not build PDU Session Establishment Request: %v", err)
	}

	pduUplink, err := BuildUplinkNasTransport(&UplinkNasTransportOpts{
		PDUSessionID:     pduSessionID,
		PayloadContainer: pduReq,
		DNN:              dnn,
		SNSSAI:           snssai,
		RequestType:      requestType,
	})
	if err != nil {
		return fmt.Errorf("could not build Uplink NAS Transport for PDU Session: %v", err)
	}

	encodedPdu, err := ue.EncodeNasPduWithSecurity(pduUplink, uint8(fgs.SHTIntegrityProtectedCiphered))
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

// AuthenticationSubscription is the subscriber credential material this
// simulator authenticates with (TS 33.501 §6.1). internal/tester/s1enb keeps
// the 4G equivalent as plain K/OPc fields on its UE; the 5G UE needs the
// sequence number and AMF too, so they travel together.
type AuthenticationSubscription struct {
	// EncPermanentKey and EncOpcKey are K and OPc as hex strings.
	EncPermanentKey               string
	EncOpcKey                     string
	AuthenticationManagementField string
	SequenceNumber                *SequenceNumber
	AuthenticationMethod          AuthMethod
}

// SequenceNumber is the SQN the UE tracks for the milenage f5 check.
type SequenceNumber struct {
	Sqn string
}

// AuthMethod names the authentication method a subscription uses.
type AuthMethod string

// AuthMethod5GAKA is the only method this simulator drives (TS 33.501 §6.1.3.2).
const AuthMethod5GAKA AuthMethod = "5G_AKA"
