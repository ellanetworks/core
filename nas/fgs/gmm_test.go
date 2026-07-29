// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package fgs

import (
	"bytes"
	"encoding/hex"
	"reflect"
	"testing"
	"time"

	"github.com/ellanetworks/core/nas"
)

func wire(t *testing.T, name string, fn func() ([]byte, error), want string) {
	t.Helper()

	if got := hex.EncodeToString(mustMarshal(t, fn)); got != want {
		t.Errorf("%s = %s, want %s", name, got, want)
	}
}

func TestMMBuildersWireBytes(t *testing.T) {
	rand := &[16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}
	autn := &[16]byte{16, 15, 14, 13, 12, 11, 10, 9, 8, 7, 6, 5, 4, 3, 2, 1}
	t3502 := nas.GPRSTimer2{Unit: nas.GPRSTimer2Unit2Seconds, Value: 30} // 0x1e

	wire(t, "AuthenticationRequest", (&AuthenticationRequest{NgKSI: nas.KeySetIdentifier{Value: 1}, ABBA: []byte{0, 0}, RAND: rand, AUTN: autn}).MarshalBinary,
		"7e005601020000210102030405060708090a0b0c0d0e0f102010100f0e0d0c0b0a090807060504030201")
	wire(t, "AuthenticationReject", (&AuthenticationReject{}).MarshalBinary, "7e0058")
	wire(t, "IdentityRequest", (&IdentityRequest{IdentityType: 1}).MarshalBinary, "7e005b01")
	wire(t, "GMMStatus", (&GMMStatus{Cause: 0x6f}).MarshalBinary, "7e00646f")
	wire(t, "ServiceReject", (&ServiceReject{Cause: 0x16}).MarshalBinary, "7e004d16")
	wire(t, "RegistrationReject", (&RegistrationReject{Cause: 0x0b}).MarshalBinary, "7e00440b")
	wire(t, "RegistrationRejectT3502", (&RegistrationReject{Cause: 0x0b, T3502: &t3502}).MarshalBinary, "7e00440b16011e")
	wire(t, "DLNASTransport", (&DLNASTransport{PayloadContainerType: 1, PayloadContainer: []byte{0xAA, 0xBB, 0xCC}, PDUSessionID: ptr(PDUSessionID(5))}).MarshalBinary,
		"7e0068010003aabbcc1205")

	imeisv := IMEISVRequested
	addInfo := AdditionalSecurityInformation{RINMR: true}
	wire(t, "SecurityModeCommand",
		(&SecurityModeCommand{CipheringAlgorithm: 2, IntegrityAlgorithm: 1, NgKSI: nas.KeySetIdentifier{Value: 1}, ReplayedUESecurityCapability: UESecurityCapability{EA: 0xFF, IA: 0xF0}, IMEISVRequested: &imeisv, AdditionalSecurityInformation: &addInfo}).MarshalBinary,
		"7e005d210102fff0e1360102")

	wire(t, "DeregistrationRequestUETerminated", (&DeregistrationRequestUETerminated{AccessType: AccessType3GPP}).MarshalBinary, "7e004701")
	wire(t, "DeregistrationAcceptUEOriginating", (&DeregistrationAcceptUEOriginating{}).MarshalBinary, "7e0046")
	wire(t, "DeregistrationAcceptUETerminated", (&DeregistrationAcceptUETerminated{}).MarshalBinary, "7e0048")

	var psi [16]bool

	psi[1] = true
	wire(t, "ServiceAccept", (&ServiceAccept{PDUSessionStatus: &PSIBitmap{PSI: psi}}).MarshalBinary, "7e004e50020200")

	timer, err := nas.GPRSTimer3FromDuration(3000 * time.Second) // 5 × 10 minutes → 0x05
	if err != nil {
		t.Fatal(err)
	}

	t3512 := timer

	wire(t, "RegistrationAccept",
		(&RegistrationAccept{RegistrationResult: RegistrationResult3GPP, T3512: &t3512}).MarshalBinary, "7e004201015e0105")

	ack := ConfigurationUpdateIndication{ACK: true}
	wire(t, "ConfigurationUpdateCommand", (&ConfigurationUpdateCommand{ConfigurationUpdateIndication: &ack}).MarshalBinary, "7e0054d1")
}

func TestMMParsers(t *testing.T) {
	res := bytes.Repeat([]byte{0xAB}, 16)

	resp, err := ParseAuthenticationResponse(append([]byte{uint8(EPD5GMM), 0x00, uint8(MsgAuthenticationResponse), ieiAuthResponseParam, 0x10}, res...))
	if err != nil || !bytes.Equal(resp.RES, res) {
		t.Errorf("AuthenticationResponse RES = %x (err %v)", resp.RES, err)
	}

	auts := bytes.Repeat([]byte{0xCD}, 14)

	fail, err := ParseAuthenticationFailure(append([]byte{uint8(EPD5GMM), 0x00, uint8(MsgAuthenticationFailure), 0x15, ieiAuthFailureParam, 0x0e}, auts...))
	if err != nil || fail.Cause != 0x15 || !bytes.Equal(fail.AUTS, auts) {
		t.Errorf("AuthenticationFailure cause=%#x AUTS=%x (err %v)", fail.Cause, fail.AUTS, err)
	}

	// A SUCI in the NAI SUPI format: type 001, SUPI format 001, then the NAI.
	mi := []byte{0x11, 0xAB, 0xCD}

	id, err := ParseIdentityResponse(append([]byte{uint8(EPD5GMM), 0x00, uint8(MsgIdentityResponse), 0x00, 0x03}, mi...))
	if err != nil || id.MobileIdentity.SUCI == nil || !bytes.Equal(id.MobileIdentity.SUCI.NAI, mi[1:]) {
		t.Errorf("IdentityResponse MobileIdentity = %+v (err %v)", id.MobileIdentity, err)
	}

	st, err := ParseGMMStatus([]byte{uint8(EPD5GMM), 0x00, uint8(MsgGMMStatus), 0x6f})
	if err != nil || st.Cause != 0x6f {
		t.Errorf("GMMStatus cause = %#x (err %v)", st.Cause, err)
	}

	if _, err := ParseSecurityModeReject([]byte{uint8(EPD5GMM), 0x00, uint8(MsgSecurityModeReject), 0x18}); err != nil {
		t.Errorf("SecurityModeReject: %v", err)
	}
}

func TestMMUplinkMarshalRoundTrip(t *testing.T) {
	stmsi := STMSIIdentity(STMSI{AMFSetID: 0x004, AMFPointer: 0x02, TMSI: [4]byte{0x02, 0x03, 0x04, 0x05}})
	guti := GUTIIdentity(GUTI{PLMN: nas.PLMN{MCC: "001", MNC: "01"}, AMFRegionID: 1, AMFSetID: 0x008, AMFPointer: 0x03, TMSI: [4]byte{0x04, 0x05, 0x06, 0x07}})
	eap := []byte{0x02, 0x00, 0x00, 0x05, 0x01}
	res16 := bytes.Repeat([]byte{0xef}, 16)
	drx := DRXParameter{Value: DRXValueNotSpecified}
	mico := MICOIndication{RAAI: true}

	// IDENTITY RESPONSE
	ir := &IdentityResponse{MobileIdentity: stmsi}
	if got, err := ParseIdentityResponse(mustMarshal(t, ir.MarshalBinary)); err != nil || !reflect.DeepEqual(got.MobileIdentity, stmsi) {
		t.Errorf("IdentityResponse MobileIdentity = %+v (err %v)", got.MobileIdentity, err)
	}

	// AUTHENTICATION RESPONSE (RES + EAP)
	ar := &AuthenticationResponse{RES: res16, EAP: eap}
	if got, err := ParseAuthenticationResponse(mustMarshal(t, ar.MarshalBinary)); err != nil || !bytes.Equal(got.RES, res16) || !bytes.Equal(got.EAP, eap) {
		t.Errorf("AuthenticationResponse = %+v (err %v)", got, err)
	}

	// REGISTRATION COMPLETE (SOR)
	sor := bytes.Repeat([]byte{0x11}, 17)
	if got, err := ParseRegistrationComplete(mustMarshal(t, (&RegistrationComplete{SORTransparentContainer: sor}).MarshalBinary)); err != nil || !bytes.Equal(got.SORTransparentContainer, sor) {
		t.Errorf("RegistrationComplete SOR = %x (err %v)", got.SORTransparentContainer, err)
	}

	// SECURITY MODE COMPLETE (IMEISV + NAS message container)
	imeisv := PEIIdentity(PEI{Type: IdentityIMEISV, Digits: "1234567890123456"})
	container := []byte{0x7e, 0x00, 0x41}

	if got, err := ParseSecurityModeComplete(mustMarshal(t, (&SecurityModeComplete{IMEISV: &imeisv, NASMessageContainer: container}).MarshalBinary)); err != nil ||
		got.IMEISV == nil || !reflect.DeepEqual(*got.IMEISV, imeisv) || !bytes.Equal(got.NASMessageContainer, container) {
		t.Errorf("SecurityModeComplete = %+v (err %v)", got, err)
	}

	// SERVICE REQUEST
	var srPSI [16]bool

	srPSI[1] = true

	sr := &ServiceRequest{ServiceType: 1, NgKSI: nas.KeySetIdentifier{Value: 2}, MobileIdentity: stmsi, PDUSessionStatus: &PSIBitmap{PSI: srPSI}}
	if got, err := ParseServiceRequest(mustMarshal(t, sr.MarshalBinary)); err != nil || got.ServiceType != 1 || got.NgKSI.Value != 2 ||
		!reflect.DeepEqual(got.MobileIdentity, stmsi) || got.PDUSessionStatus == nil || got.PDUSessionStatus.PSI != srPSI {
		t.Errorf("ServiceRequest = %+v (err %v)", got, err)
	}

	// DEREGISTRATION REQUEST (UE-originating)
	dr := &DeregistrationRequestUEOriginating{AccessType: 1, SwitchOff: true, NgKSI: nas.KeySetIdentifier{Value: 3}, MobileIdentity: guti}
	if got, err := ParseDeregistrationRequestUEOriginating(mustMarshal(t, dr.MarshalBinary)); err != nil || got.AccessType != 1 || !got.SwitchOff || got.NgKSI.Value != 3 || !reflect.DeepEqual(got.MobileIdentity, guti) {
		t.Errorf("DeregistrationRequest = %+v (err %v)", got, err)
	}

	// UL NAS TRANSPORT
	pduID := PDUSessionID(uint8(5))
	reqType := uint8(1)

	ult := &ULNASTransport{PayloadContainerType: 1, PayloadContainer: []byte{0x2e, 0x01, 0x00, 0xc1}, PDUSessionID: &pduID, RequestType: ptr(RequestType(reqType)), DNN: ptr(DNN("internet"))}

	if got, err := ParseULNASTransport(mustMarshal(t, ult.MarshalBinary)); err != nil || got.PayloadContainerType != 1 || got.PDUSessionID == nil || *got.PDUSessionID != 5 || got.RequestType == nil || *got.RequestType != 1 || !bytes.Equal(got.PayloadContainer, ult.PayloadContainer) {
		t.Errorf("ULNASTransport = %+v (err %v)", got, err)
	}

	// REGISTRATION REQUEST
	rr := &RegistrationRequest{
		RegistrationType: RegistrationTypeInitial, FOR: true, NgKSI: nas.NoKeySet, MobileIdentity: guti,
		GMMCapability: &GMMCapability{S1Mode: true}, UESecurityCapability: &UESecurityCapability{EA: 0xe0, IA: 0xe0}, RequestedNSSAI: NSSAI{{SST: 1}},
		RequestedDRXParameters: &drx, MICOIndication: &mico,
	}
	if got, err := ParseRegistrationRequest(mustMarshal(t, rr.MarshalBinary)); err != nil || got.RegistrationType != RegistrationTypeInitial || !got.FOR || got.NgKSI != nas.NoKeySet || !reflect.DeepEqual(got.MobileIdentity, guti) || (got.UESecurityCapability == nil || !got.UESecurityCapability.Equal(UESecurityCapability{EA: 0xe0, IA: 0xe0})) || !reflect.DeepEqual(got.RequestedNSSAI, NSSAI{{SST: 1}}) || got.MICOIndication == nil {
		t.Errorf("RegistrationRequest = %+v (err %v)", got, err)
	}

	// CONFIGURATION UPDATE COMPLETE (header only)
	if _, err := ParseConfigurationUpdateComplete(mustMarshal(t, (&ConfigurationUpdateComplete{}).MarshalBinary)); err != nil {
		t.Errorf("ConfigurationUpdateComplete: %v", err)
	}
}

func TestMMDownlinkParsersRoundTrip(t *testing.T) {
	rand := &[16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}
	autn := &[16]byte{16, 15, 14, 13, 12, 11, 10, 9, 8, 7, 6, 5, 4, 3, 2, 1}

	areq := &AuthenticationRequest{NgKSI: nas.KeySetIdentifier{Value: 1}, ABBA: []byte{0x00, 0x00}, RAND: rand, AUTN: autn}

	wire, err := areq.MarshalBinary()
	if err != nil {
		t.Fatalf("marshal AuthenticationRequest: %v", err)
	}

	got, err := ParseAuthenticationRequest(wire)
	if err != nil {
		t.Fatalf("parse AuthenticationRequest: %v", err)
	}

	if got.NgKSI.Value != 1 || !bytes.Equal(got.ABBA, areq.ABBA) || *got.RAND != *rand || *got.AUTN != *autn {
		t.Errorf("AuthenticationRequest round-trip mismatch: %+v", got)
	}

	// EAP-based variants: the parser accepts an EAP message Ella never sends.
	eap := []byte{0x02, 0x00, 0x00, 0x05, 0x01}

	areqEAP := append([]byte{uint8(EPD5GMM), 0x00, uint8(MsgAuthenticationRequest), 0x01, 0x02, 0x00, 0x00, ieiEAPMessage, 0x00, byte(len(eap))}, eap...)
	if got, err := ParseAuthenticationRequest(areqEAP); err != nil || !bytes.Equal(got.EAP, eap) {
		t.Errorf("AuthenticationRequest EAP = %x (err %v)", got.EAP, err)
	}

	arejEAP := append([]byte{uint8(EPD5GMM), 0x00, uint8(MsgAuthenticationReject), ieiEAPMessage, 0x00, byte(len(eap))}, eap...)
	if got, err := ParseAuthenticationReject(arejEAP); err != nil || !bytes.Equal(got.EAP, eap) {
		t.Errorf("AuthenticationReject EAP = %x (err %v)", got.EAP, err)
	}

	if _, err := ParseAuthenticationReject([]byte{uint8(EPD5GMM), 0x00, uint8(MsgAuthenticationReject)}); err != nil {
		t.Errorf("AuthenticationReject header-only: %v", err)
	}

	idReq, err := ParseIdentityRequest([]byte{uint8(EPD5GMM), 0x00, uint8(MsgIdentityRequest), 0x01})
	if err != nil || idReq.IdentityType != 1 {
		t.Errorf("IdentityRequest type = %d (err %v)", idReq.IdentityType, err)
	}

	// REGISTRATION REJECT with T3346, T3502 and EAP (eap declared above).
	rrej := append([]byte{uint8(EPD5GMM), 0x00, uint8(MsgRegistrationReject), 0x0b, ieiT3346Value, 0x01, 0x0a, ieiT3502Value, 0x01, 0x0b, ieiEAPMessage, 0x00, byte(len(eap))}, eap...)
	if got, err := ParseRegistrationReject(rrej); err != nil || got.Cause != 0x0b || got.T3346 == nil || got.T3346.Unit != nas.GPRSTimer2Unit2Seconds || got.T3346.Value != 0x0a || got.T3502 == nil || got.T3502.Unit != nas.GPRSTimer2Unit2Seconds || got.T3502.Value != 0x0b || !bytes.Equal(got.EAP, eap) {
		t.Errorf("RegistrationReject = %+v (err %v)", got, err)
	}

	// REGISTRATION COMPLETE with SOR transparent container.
	sor := bytes.Repeat([]byte{0x11}, 17)

	rcomp := append([]byte{uint8(EPD5GMM), 0x00, uint8(MsgRegistrationComplete), ieiSORContainer, 0x00, byte(len(sor))}, sor...)
	if got, err := ParseRegistrationComplete(rcomp); err != nil || !bytes.Equal(got.SORTransparentContainer, sor) {
		t.Errorf("RegistrationComplete SOR = %x (err %v)", got.SORTransparentContainer, err)
	}

	// SECURITY MODE COMMAND round-trip plus the decode-only optional IEs.
	imeisv := IMEISVRequested
	addInfo := AdditionalSecurityInformation{RINMR: true, HDP: true}
	smc := &SecurityModeCommand{CipheringAlgorithm: 2, IntegrityAlgorithm: 1, NgKSI: nas.KeySetIdentifier{Value: 1}, ReplayedUESecurityCapability: UESecurityCapability{EA: 0xFF, IA: 0xF0}, IMEISVRequested: &imeisv, AdditionalSecurityInformation: &addInfo}

	wire, err = smc.MarshalBinary()
	if err != nil {
		t.Fatalf("marshal SecurityModeCommand: %v", err)
	}

	if got, err := ParseSecurityModeCommand(wire); err != nil || got.CipheringAlgorithm != 2 || got.IntegrityAlgorithm != 1 || got.NgKSI.Value != 1 || !got.ReplayedUESecurityCapability.Equal(UESecurityCapability{EA: 0xFF, IA: 0xF0}) || got.IMEISVRequested == nil || !got.IMEISVRequested.Requested() || got.AdditionalSecurityInformation == nil || *got.AdditionalSecurityInformation != (AdditionalSecurityInformation{RINMR: true, HDP: true}) {
		t.Errorf("SecurityModeCommand round-trip = %+v (err %v)", got, err)
	}

	smcFull := append([]byte{uint8(EPD5GMM), 0x00, uint8(MsgSecurityModeCommand), 0x21, 0x01, 0x02, 0xff, 0xf0, ieiReplayedS1UESecCap, 0x04, 0, 0, 0, 0, ieiABBA, 0x02, 0xaa, 0xbb, ieiSelectedEPSNASSecAlg, 0x01, ieiEAPMessage, 0x00, byte(len(eap))}, eap...)
	if got, err := ParseSecurityModeCommand(smcFull); err != nil || got.ReplayedS1UESecurityCapability == nil || !bytes.Equal(got.ABBA, []byte{0xaa, 0xbb}) || got.SelectedEPSNASSecurityAlgorithms == nil || *got.SelectedEPSNASSecurityAlgorithms != (SelectedEPSNASSecurityAlgorithms{Integrity: 1}) || !bytes.Equal(got.EAP, eap) {
		t.Errorf("SecurityModeCommand full = %+v (err %v)", got, err)
	}

	// SERVICE REJECT with PDU session status, T3346 and EAP.
	srej := append([]byte{uint8(EPD5GMM), 0x00, uint8(MsgServiceReject), 0x16, ieiPDUSessionStatus, 0x02, 0x02, 0x00, ieiT3346Value, 0x01, 0x0a, ieiEAPMessage, 0x00, byte(len(eap))}, eap...)
	if got, err := ParseServiceReject(srej); err != nil || got.Cause != 0x16 || got.PDUSessionStatus == nil || !got.PDUSessionStatus.PSI[1] || got.T3346 == nil || got.T3346.Unit != nas.GPRSTimer2Unit2Seconds || got.T3346.Value != 0x0a || !bytes.Equal(got.EAP, eap) {
		t.Errorf("ServiceReject = %+v (err %v)", got, err)
	}

	// SERVICE ACCEPT round-trip plus EAP.
	var psi [16]bool

	psi[1] = true
	sacc := &ServiceAccept{PDUSessionStatus: &PSIBitmap{PSI: psi}}

	wire, err = sacc.MarshalBinary()
	if err != nil {
		t.Fatalf("marshal ServiceAccept: %v", err)
	}

	if got, err := ParseServiceAccept(wire); err != nil || got.PDUSessionStatus == nil || got.PDUSessionStatus.PSI != psi {
		t.Errorf("ServiceAccept round-trip = %+v (err %v)", got, err)
	}

	// REGISTRATION ACCEPT round-trip plus decode-only optional IEs.
	guti := GUTIIdentity(GUTI{PLMN: nas.PLMN{MCC: "001", MNC: "01"}, AMFRegionID: 1, AMFSetID: 0x008, AMFPointer: 0x03, TMSI: [4]byte{0x04, 0x05, 0x06, 0x07}})
	t3512 := nas.GPRSTimer3{Unit: nas.GPRSTimer3Unit10Minutes, Value: 5}
	racc := &RegistrationAccept{RegistrationResult: RegistrationResult3GPP, GUTI: &guti, T3512: &t3512}

	wire, err = racc.MarshalBinary()
	if err != nil {
		t.Fatalf("marshal RegistrationAccept: %v", err)
	}

	if got, err := ParseRegistrationAccept(wire); err != nil || got.RegistrationResult != RegistrationResult3GPP || (got.GUTI == nil || !reflect.DeepEqual(*got.GUTI, guti)) || got.T3512 == nil || *got.T3512 != t3512 {
		t.Errorf("RegistrationAccept round-trip = %+v (err %v)", got, err)
	}

	// Decode-only IEs: MICO type-1, PDU session status, EAP.
	raccFull := append([]byte{uint8(EPD5GMM), 0x00, uint8(MsgRegistrationAccept), 0x01, 0x01, ieiPDUSessionStatus, 0x02, 0x02, 0x00, 0xb1, ieiEAPMessage, 0x00, byte(len(eap))}, eap...)
	if got, err := ParseRegistrationAccept(raccFull); err != nil || got.PDUSessionStatus == nil || got.MICOIndication == nil || !bytes.Equal(got.EAP, eap) {
		t.Errorf("RegistrationAccept full = %+v (err %v)", got, err)
	}

	// DL NAS TRANSPORT round-trip (payload container + routing/diagnostic IEs).
	cause := uint8(0x16)
	dl := &DLNASTransport{PayloadContainerType: PayloadContainerTypeN1SMInfo, PayloadContainer: []byte{0x2e, 0x01, 0x00, 0xc1}, PDUSessionID: ptr(PDUSessionID(5)), Cause: ptr(GMMCause(cause))}

	wire, err = dl.MarshalBinary()
	if err != nil {
		t.Fatalf("marshal DLNASTransport: %v", err)
	}

	if got, err := ParseDLNASTransport(wire); err != nil || got.PayloadContainerType != PayloadContainerTypeN1SMInfo || !bytes.Equal(got.PayloadContainer, dl.PayloadContainer) || (got.PDUSessionID == nil || *got.PDUSessionID != 5) || got.Cause == nil || *got.Cause != 0x16 {
		t.Errorf("DLNASTransport round-trip = %+v (err %v)", got, err)
	}

	dlBackoff := []byte{uint8(EPD5GMM), 0x00, uint8(MsgDLNASTransport), 0x01, 0x00, 0x02, 0x2e, 0x01, ieiBackoffTimer, 0x01, 0x21}
	if got, err := ParseDLNASTransport(dlBackoff); err != nil || got.BackoffTimer == nil || got.BackoffTimer.Unit != nas.GPRSTimer3Unit1Hour || got.BackoffTimer.Value != 1 {
		t.Errorf("DLNASTransport backoff = %+v (err %v)", got, err)
	}
}

func TestConfigurationUpdateCommandRoundTrip(t *testing.T) {
	ind := ConfigurationUpdateIndication{ACK: true}
	guti := GUTIIdentity(GUTI{PLMN: nas.PLMN{MCC: "001", MNC: "01"}, AMFRegionID: 1, AMFSetID: 0x008, AMFPointer: 0x03, TMSI: [4]byte{0x04, 0x05, 0x06, 0x07}})

	in := &ConfigurationUpdateCommand{
		ConfigurationUpdateIndication: &ind,
		GUTI:                          &guti,
		FullNameForNetwork:            ptr(nas.NewNetworkName("Ella")),
	}

	b, err := in.MarshalBinary()
	if err != nil {
		t.Fatalf("MarshalBinary: %v", err)
	}

	out, err := ParseConfigurationUpdateCommand(b)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	if out.ConfigurationUpdateIndication == nil || *out.ConfigurationUpdateIndication != ind ||
		(out.GUTI == nil || !reflect.DeepEqual(*out.GUTI, guti)) || (out.FullNameForNetwork == nil || *out.FullNameForNetwork != *in.FullNameForNetwork) ||
		out.ShortNameForNetwork != nil {
		t.Fatalf("round-trip mismatch:\n in  %+v\n out %+v", in, out)
	}
}

func TestGUTIRoundTrip(t *testing.T) {
	in := GUTI{PLMN: nas.PLMN{MCC: "001", MNC: "01"}, AMFRegionID: 205, AMFSetID: 1018, AMFPointer: 1, TMSI: [4]byte{0x21, 0x43, 0x65, 0x84}}

	b, err := in.MarshalBinary()
	if err != nil {
		t.Fatalf("MarshalBinary: %v", err)
	}

	out, err := ParseGUTI(b)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	if out != in {
		t.Fatalf("round-trip mismatch:\n in  %+v\n out %+v", in, out)
	}
}

// mustBytes returns the octets of a MarshalBinary call that must succeed, so encode
// calls stay usable as expressions in test fixtures.
func mustBytes(b []byte, err error) []byte {
	if err != nil {
		panic(err)
	}

	return b
}
