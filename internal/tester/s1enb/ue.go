// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1enb

import (
	"fmt"

	"github.com/ellanetworks/core/internal/udm"
	"github.com/ellanetworks/core/internal/util/ueauth"
	nascommon "github.com/ellanetworks/core/nas/common"
	"github.com/ellanetworks/core/nas/eps"
	"github.com/ellanetworks/core/s1ap"
)

// EPS key-derivation FC values (TS 33.401 Annex A), as the hex strings
// ueauth.GetKDFValue expects.
const (
	fcKASME           = "10" // §A.2
	fcEPSAlgorithmKey = "15" // §A.7
)

const (
	nasEncAlgDistinguisher byte = 0x01
	nasIntAlgDistinguisher byte = 0x02
)

// defaultIMEISV is the UE's IMEISV mobile identity (TS 24.008 §10.5.1.4),
// returned in SECURITY MODE COMPLETE when the MME requests it.
var defaultIMEISV = []byte{0x03, 0x53, 0x60, 0x83, 0x12, 0x34, 0x56, 0x78, 0xf0}

// UE is a simulated 4G UE: its identity, USIM credentials, and the EPS NAS
// security state it derives during attach. It is single-threaded — used by one
// attach/procedure goroutine at a time.
type UE struct {
	IMSI string
	K    [16]byte
	OPc  [16]byte

	plmn []byte // serving-network PLMN octets, for K_ASME derivation

	netCapEEA byte // advertised EPS ciphering bitmap (UE network capability octet 3)
	netCapEIA byte // advertised EPS integrity bitmap (octet 4)

	pdnType    uint8                  // requested PDN type (eps.PDNTypeIPv4 / IPv6 / IPv4v6)
	apn        string                 // requested APN in the Attach Request ("" = subscriber default)
	attachGUTI *eps.EPSMobileIdentity // when set, the Attach Request presents this GUTI rather than the IMSI

	kasme   []byte
	knasEnc [16]byte
	knasInt [16]byte
	eea     uint8
	eia     uint8
	ulCount uint8 // uplink NAS COUNT for protected uplink messages
	pti     uint8 // last ESM procedure transaction identity used (attach uses 1)
}

// NewUE creates a UE bound to this eNB's serving PLMN (the network used in the
// K_ASME derivation, TS 33.401 §A.2).
func (e *ENB) NewUE(imsi string, k, opc [16]byte) *UE {
	// EEA0-3 supported; EIA1-3 supported (no EIA0) — drives an EEA2/EIA2 selection.
	return &UE{
		IMSI: imsi, K: k, OPc: opc, plmn: append([]byte(nil), e.plmn[:]...),
		netCapEEA: 0xf0, netCapEIA: 0x70, pdnType: eps.PDNTypeIPv4, pti: 1,
	}
}

// nextPTI allocates the next ESM procedure transaction identity for a UE-initiated
// procedure (the attach PDN connectivity used 1, TS 24.301 §9.6).
func (ue *UE) nextPTI() uint8 {
	ue.pti++
	if ue.pti == 0 {
		ue.pti = 1
	}

	return ue.pti
}

// protectUplink integrity-protects and ciphers a plain uplink NAS message,
// advancing the uplink NAS COUNT (TS 24.301).
func (ue *UE) protectUplink(plain []byte) ([]byte, error) {
	out, err := eps.Protect(plain, eps.SHTIntegrityProtectedCiphered,
		nascommon.NASCount(0, ue.ulCount), nascommon.DirectionUplink,
		ue.knasInt, ue.knasEnc, ue.integrityAlg(), ue.cipherAlg())
	if err != nil {
		return nil, err
	}

	ue.ulCount++

	return out, nil
}

// RequestPDNType sets the PDN type the UE requests in its Attach Request
// (eps.PDNTypeIPv4 / IPv6 / IPv4v6).
func (ue *UE) RequestPDNType(t uint8) { ue.pdnType = t }

// RequestAPN sets the APN the UE requests in its Attach Request's PDN Connectivity
// Request (TS 24.301 §6.5.1.2); empty selects the subscriber's default APN.
func (ue *UE) RequestAPN(apn string) { ue.apn = apn }

// UseUnknownGUTI makes the Attach Request present a GUTI the MME cannot resolve,
// so the MME must obtain the IMSI with an IDENTITY REQUEST (TS 24.301 §5.4.4).
func (ue *UE) UseUnknownGUTI() {
	ue.attachGUTI = &eps.EPSMobileIdentity{
		Type: eps.IdentityGUTI, MCC: "001", MNC: "01", MMEGroupID: 0xffff, MMECode: 0xff, MTMSI: 0xdeadbeef,
	}
}

// S1APSecurityCapabilities returns the UE's EPS algorithm capabilities in the
// S1AP UESecurityCapabilities encoding (the EEA0/EIA0 bit is dropped, so the
// octet is shifted left), matching how the MME stored them at attach. Used to
// replay capabilities in a Path Switch Request.
func (ue *UE) S1APSecurityCapabilities() s1ap.UESecurityCapabilities {
	return s1ap.UESecurityCapabilities{
		EncryptionAlgorithms:          uint16(ue.netCapEEA<<1) << 8,
		IntegrityProtectionAlgorithms: uint16(ue.netCapEIA<<1) << 8,
	}
}

func (ue *UE) buildAttachRequest() ([]byte, error) {
	pc := &eps.PDNConnectivityRequest{ProcedureTransactionIdentity: 1, RequestType: 1, PDNType: ue.pdnType}

	if ue.apn != "" {
		apnIE, err := eps.EncodeAPN(ue.apn)
		if err != nil {
			return nil, fmt.Errorf("encode APN: %w", err)
		}

		pc.AccessPointName = apnIE
	}

	esm, err := pc.Marshal()
	if err != nil {
		return nil, fmt.Errorf("build PDN Connectivity Request: %w", err)
	}

	identity := eps.EPSMobileIdentity{Type: eps.IdentityIMSI, Digits: ue.IMSI}
	if ue.attachGUTI != nil {
		identity = *ue.attachGUTI
	}

	attach := &eps.AttachRequest{
		EPSAttachType:       eps.AttachTypeEPS,
		NASKeySetIdentifier: 7,
		EPSMobileIdentity:   identity,
		UENetworkCapability: eps.UENetworkCapability{EEA: ue.netCapEEA, EIA: ue.netCapEIA}.Marshal(),
		ESMMessageContainer: esm,
	}

	return attach.Marshal()
}

// buildIdentityResponse returns the IDENTITY RESPONSE carrying the UE's IMSI as
// the mobile identity (TS 24.008 §10.5.1.4 coding), answering an MME IDENTITY
// REQUEST for the IMSI.
func (ue *UE) buildIdentityResponse() ([]byte, error) {
	id, err := imsiMobileIdentity(ue.IMSI)
	if err != nil {
		return nil, err
	}

	return (&eps.IdentityResponse{MobileIdentity: id}).Marshal()
}

// imsiMobileIdentity encodes an IMSI as a Mobile Identity value part (TS 24.008
// §10.5.1.4): the first digit shares the leading octet with the odd/even flag
// and type, the remaining digits are TBCD.
func imsiMobileIdentity(imsi string) ([]byte, error) {
	if len(imsi) == 0 {
		return nil, fmt.Errorf("empty IMSI")
	}

	rest, err := nascommon.EncodeTBCD(imsi[1:])
	if err != nil {
		return nil, fmt.Errorf("encode IMSI: %w", err)
	}

	oddEven := byte(len(imsi) & 1)
	head := (imsi[0]-'0')<<4 | oddEven<<3 | byte(eps.IdentityIMSI)

	return append([]byte{head}, rest...), nil
}

// handleAuthenticationRequest verifies nothing of the network (this is a test UE)
// but computes RES from the challenge and derives K_ASME (TS 33.401 §A.2), then
// returns the AUTHENTICATION RESPONSE NAS.
func (ue *UE) handleAuthenticationRequest(plain []byte) ([]byte, error) {
	req, err := eps.ParseAuthenticationRequest(plain)
	if err != nil {
		return nil, fmt.Errorf("parse Authentication Request: %w", err)
	}

	if len(req.AUTN) < 6 {
		return nil, fmt.Errorf("AUTN too short: %d", len(req.AUTN))
	}

	res := make([]byte, 8)
	ck := make([]byte, 16)
	ik := make([]byte, 16)
	ak := make([]byte, 6)

	if err := udm.F2345(ue.OPc[:], ue.K[:], req.RAND[:], res, ck, ik, ak, nil); err != nil {
		return nil, fmt.Errorf("milenage f2345: %w", err)
	}

	// SQN ⊕ AK is the first six octets of AUTN.
	sqnXorAK := append([]byte(nil), req.AUTN[:6]...)
	key := append(append([]byte(nil), ck...), ik...)

	kasme, err := ueauth.GetKDFValue(key, fcKASME, ue.plmn, ueauth.KDFLen(ue.plmn), sqnXorAK, ueauth.KDFLen(sqnXorAK))
	if err != nil {
		return nil, fmt.Errorf("derive K_ASME: %w", err)
	}

	ue.kasme = kasme

	return (&eps.AuthenticationResponse{RES: res}).Marshal()
}

// handleSecurityModeCommand reads the selected algorithms from the (integrity-
// protected, unciphered) SECURITY MODE COMMAND, derives the NAS keys, and returns
// the protected SECURITY MODE COMPLETE carrying the UE's IMEISV.
func (ue *UE) handleSecurityModeCommand(wire []byte) ([]byte, error) {
	if len(wire) < 6 {
		return nil, fmt.Errorf("security mode command too short: %d", len(wire))
	}

	// The command is integrity-protected with the new context but not ciphered,
	// so the inner message is readable after the 6-octet security header.
	smc, err := eps.ParseSecurityModeCommand(wire[6:])
	if err != nil {
		return nil, fmt.Errorf("parse Security Mode Command: %w", err)
	}

	ue.eea = smc.CipheringAlgorithm
	ue.eia = smc.IntegrityAlgorithm

	if err := ue.deriveNASKeys(); err != nil {
		return nil, err
	}

	complete, err := (&eps.SecurityModeComplete{IMEISV: defaultIMEISV}).Marshal()
	if err != nil {
		return nil, fmt.Errorf("build Security Mode Complete: %w", err)
	}

	out, err := eps.Protect(complete, eps.SHTIntegrityProtectedCiphered,
		nascommon.NASCount(0, ue.ulCount), nascommon.DirectionUplink,
		ue.knasInt, ue.knasEnc, ue.integrityAlg(), ue.cipherAlg())
	if err != nil {
		return nil, fmt.Errorf("protect Security Mode Complete: %w", err)
	}

	ue.ulCount++

	return out, nil
}

// unprotectDownlink deciphers and integrity-checks a protected downlink NAS PDU
// using the sequence number carried in the message.
func (ue *UE) unprotectDownlink(wire []byte) ([]byte, error) {
	if len(wire) < 6 {
		return nil, fmt.Errorf("protected downlink NAS too short: %d", len(wire))
	}

	return eps.Unprotect(wire, nascommon.NASCount(0, wire[5]), nascommon.DirectionDownlink,
		ue.knasInt, ue.knasEnc, ue.integrityAlg(), ue.cipherAlg())
}

// buildAttachComplete acknowledges the default EPS bearer carried in the Attach
// Accept's ESM container and returns the protected ATTACH COMPLETE.
func (ue *UE) buildAttachComplete(acceptESM []byte) ([]byte, error) {
	activate, err := eps.ParseActivateDefaultEPSBearerContextRequest(acceptESM)
	if err != nil {
		return nil, fmt.Errorf("parse Activate Default EPS Bearer Context Request: %w", err)
	}

	// Activate Default EPS Bearer Context Accept (TS 24.301 §8.3.7): PD/EBI octet,
	// the matching PTI, and the message type.
	esm := []byte{0x02, activate.ProcedureTransactionIdentity, 0xc2}

	complete, err := (&eps.AttachComplete{ESMMessageContainer: esm}).Marshal()
	if err != nil {
		return nil, fmt.Errorf("build Attach Complete: %w", err)
	}

	out, err := eps.Protect(complete, eps.SHTIntegrityProtectedCiphered,
		nascommon.NASCount(0, ue.ulCount), nascommon.DirectionUplink,
		ue.knasInt, ue.knasEnc, ue.integrityAlg(), ue.cipherAlg())
	if err != nil {
		return nil, fmt.Errorf("protect Attach Complete: %w", err)
	}

	ue.ulCount++

	return out, nil
}

// buildDetachRequest builds a protected UE-originating DETACH REQUEST (EPS
// detach, not switch-off, TS 24.301 §8.2.11) so the network acknowledges it.
func (ue *UE) buildDetachRequest() ([]byte, error) {
	req := &eps.DetachRequestUE{
		SwitchOff:           false,
		TypeOfDetach:        eps.DetachTypeEPS,
		NASKeySetIdentifier: 0,
		EPSMobileIdentity:   eps.EPSMobileIdentity{Type: eps.IdentityIMSI, Digits: ue.IMSI},
	}

	plain, err := req.Marshal()
	if err != nil {
		return nil, fmt.Errorf("build Detach Request: %w", err)
	}

	out, err := eps.Protect(plain, eps.SHTIntegrityProtectedCiphered,
		nascommon.NASCount(0, ue.ulCount), nascommon.DirectionUplink,
		ue.knasInt, ue.knasEnc, ue.integrityAlg(), ue.cipherAlg())
	if err != nil {
		return nil, fmt.Errorf("protect Detach Request: %w", err)
	}

	ue.ulCount++

	return out, nil
}

// buildServiceRequest builds the 4-octet EPS SERVICE REQUEST (TS 24.301 §8.2.25):
// the security-header/PD octet, the KSI plus the 5-bit truncated uplink sequence,
// and the 2-octet short MAC over the first two octets at the current uplink NAS
// COUNT. It advances the uplink COUNT, mirroring the MME.
func (ue *UE) buildServiceRequest() ([]byte, error) {
	octet0 := uint8(eps.SHTServiceRequest)<<4 | 0x07 // security header type | PD (EMM)
	octet1 := ue.ulCount & 0x1f                      // KSI 0 | 5-bit sequence

	mac, err := eps.ServiceRequestShortMAC([]byte{octet0, octet1}, ue.knasInt, uint32(ue.ulCount),
		nascommon.DirectionUplink, ue.integrityAlg())
	if err != nil {
		return nil, fmt.Errorf("compute Service Request short MAC: %w", err)
	}

	ue.ulCount++

	return []byte{octet0, octet1, mac[0], mac[1]}, nil
}

// buildTrackingAreaUpdateRequest builds a protected TRACKING AREA UPDATE REQUEST
// of the given EPS update type (TS 24.301 §8.2.29); activeFlag requests the
// network re-establish the radio bearer.
func (ue *UE) buildTrackingAreaUpdateRequest(updateType uint8, activeFlag bool) ([]byte, error) {
	plain, err := (&eps.TrackingAreaUpdateRequest{EPSUpdateType: updateType, ActiveFlag: activeFlag}).Marshal()
	if err != nil {
		return nil, fmt.Errorf("build Tracking Area Update Request: %w", err)
	}

	out, err := eps.Protect(plain, eps.SHTIntegrityProtectedCiphered,
		nascommon.NASCount(0, ue.ulCount), nascommon.DirectionUplink,
		ue.knasInt, ue.knasEnc, ue.integrityAlg(), ue.cipherAlg())
	if err != nil {
		return nil, fmt.Errorf("protect Tracking Area Update Request: %w", err)
	}

	ue.ulCount++

	return out, nil
}

// buildTrackingAreaUpdateComplete builds the protected TRACKING AREA UPDATE
// COMPLETE that acknowledges a GUTI-reallocating TAU Accept (TS 24.301 §8.2.28).
func (ue *UE) buildTrackingAreaUpdateComplete() ([]byte, error) {
	plain, err := (&eps.TrackingAreaUpdateComplete{}).Marshal()
	if err != nil {
		return nil, fmt.Errorf("build Tracking Area Update Complete: %w", err)
	}

	out, err := eps.Protect(plain, eps.SHTIntegrityProtectedCiphered,
		nascommon.NASCount(0, ue.ulCount), nascommon.DirectionUplink,
		ue.knasInt, ue.knasEnc, ue.integrityAlg(), ue.cipherAlg())
	if err != nil {
		return nil, fmt.Errorf("protect Tracking Area Update Complete: %w", err)
	}

	ue.ulCount++

	return out, nil
}

func (ue *UE) deriveNASKeys() error {
	enc, err := deriveNASKey(ue.kasme, nasEncAlgDistinguisher, ue.eea)
	if err != nil {
		return fmt.Errorf("derive K_NASenc: %w", err)
	}

	intg, err := deriveNASKey(ue.kasme, nasIntAlgDistinguisher, ue.eia)
	if err != nil {
		return fmt.Errorf("derive K_NASint: %w", err)
	}

	ue.knasEnc, ue.knasInt = enc, intg

	return nil
}

func deriveNASKey(kasme []byte, distinguisher, algID byte) ([16]byte, error) {
	var k [16]byte

	out, err := ueauth.GetKDFValue(kasme, fcEPSAlgorithmKey,
		[]byte{distinguisher}, ueauth.KDFLen([]byte{distinguisher}),
		[]byte{algID}, ueauth.KDFLen([]byte{algID}))
	if err != nil {
		return k, err
	}

	copy(k[:], out[16:32])

	return k, nil
}

func (ue *UE) cipherAlg() nascommon.Cipher {
	switch ue.eea {
	case 1:
		return nascommon.SNOW3GCipher{}
	case 2:
		return nascommon.AESCTRCipher{}
	default:
		return nascommon.NullCipher{}
	}
}

func (ue *UE) integrityAlg() nascommon.Integrity {
	switch ue.eia {
	case 1:
		return nascommon.SNOW3GIntegrity{}
	case 2:
		return nascommon.AESCMACIntegrity{}
	default:
		return nascommon.NullIntegrity{}
	}
}
