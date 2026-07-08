// SPDX-FileCopyrightText: Ella Networks Inc.
//
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/amf"
	"github.com/free5gc/nas"
	"github.com/free5gc/nas/nasMessage"
	"github.com/free5gc/nas/nasType"
	"github.com/free5gc/nas/security"
)

func plainRegistrationWithGuti(t *testing.T, guti []byte) []byte {
	t.Helper()

	m := nas.NewMessage()
	m.GmmMessage = nas.NewGmmMessage()
	m.GmmHeader.SetMessageType(nas.MsgTypeRegistrationRequest)

	rr := nasMessage.NewRegistrationRequest(0)
	rr.SetExtendedProtocolDiscriminator(nasMessage.Epd5GSMobilityManagementMessage)
	rr.SetSecurityHeaderType(nas.SecurityHeaderTypePlainNas)
	rr.SetSpareHalfOctet(0)
	rr.SetMessageType(nas.MsgTypeRegistrationRequest)
	rr.NgksiAndRegistrationType5GS.SetNasKeySetIdentifiler(0)
	rr.SetRegistrationType5GS(nasMessage.RegistrationType5GSInitialRegistration)
	rr.SetFOR(1)
	rr.MobileIdentity5GS = nasType.MobileIdentity5GS{
		Iei:    nasMessage.MobileIdentity5GSType5gGuti,
		Len:    uint16(len(guti)),
		Buffer: guti,
	}
	rr.UESecurityCapability = &nasType.UESecurityCapability{}

	m.RegistrationRequest = rr

	payload, err := m.PlainNasEncode()
	if err != nil {
		t.Fatalf("encode plain RegistrationRequest: %v", err)
	}

	return payload
}

func plainDeregistrationWithGuti(t *testing.T, guti []byte) []byte {
	t.Helper()

	m := nas.NewMessage()
	m.GmmMessage = nas.NewGmmMessage()
	m.GmmHeader.SetMessageType(nas.MsgTypeDeregistrationRequestUEOriginatingDeregistration)

	dr := nasMessage.NewDeregistrationRequestUEOriginatingDeregistration(0)
	dr.SetExtendedProtocolDiscriminator(nasMessage.Epd5GSMobilityManagementMessage)
	dr.SetSecurityHeaderType(nas.SecurityHeaderTypePlainNas)
	dr.SetSpareHalfOctet(0)
	dr.SetMessageType(nas.MsgTypeDeregistrationRequestUEOriginatingDeregistration)
	dr.SetSwitchOff(0)
	dr.SetReRegistrationRequired(0)
	dr.SetAccessType(nasMessage.AccessType3GPP)
	dr.SetNasKeySetIdentifiler(0)
	dr.MobileIdentity5GS = nasType.MobileIdentity5GS{
		Iei:    0,
		Len:    uint16(len(guti)),
		Buffer: guti,
	}

	m.DeregistrationRequestUEOriginatingDeregistration = dr

	payload, err := m.PlainNasEncode()
	if err != nil {
		t.Fatalf("encode plain DeregistrationRequest: %v", err)
	}

	return payload
}

// wrapIntegrityProtected wraps a plain inner NAS message in an
// integrity-protected header with a MAC computed against the UE's current
// security context, exactly as decodeProtectedNAS expects (TS 33.501).
func wrapIntegrityProtected(t *testing.T, ue *amf.UeContext, inner []byte, sqn uint8) []byte {
	t.Helper()

	cnt := ue.ULCountForTest().ReconcileUplink(sqn)

	seqAndMsg := append([]byte{sqn}, inner...)

	mac, err := security.NASMacCalculate(ue.IntegrityAlgForTest(), ue.KnasIntForTest(), cnt.Value(), security.Bearer3GPP, security.DirectionUplink, seqAndMsg)
	if err != nil {
		t.Fatalf("NASMacCalculate: %v", err)
	}

	pdu := []byte{0x7e, nas.SecurityHeaderTypeIntegrityProtected}
	pdu = append(pdu, mac...)
	pdu = append(pdu, seqAndMsg...)

	return pdu
}

// TestFetchUeContext_DeregistrationResolvesExistingContextByGuti guards the
// GUTI-shadowing defect: an integrity-verified UE-originating DEREGISTRATION
// citing a known GUTI must resolve to the existing context. The shadowed local
// guti previously left the outer guti invalid, forcing a fresh context.
func TestFetchUeContext_DeregistrationResolvesExistingContextByGuti(t *testing.T) {
	// type byte 0x02 = 5G-GUTI; PLMN 001/01; amf.AMF id 0xcafe00; 5G-TMSI 1.
	gutiBytes := []byte{0x02, 0x00, 0xf1, 0x10, 0xca, 0xfe, 0x00, 0x00, 0x00, 0x00, 0x01}

	guti, err := etsi.NewGUTI5GFromBytes(gutiBytes)
	if err != nil {
		t.Fatalf("NewGUTIFromBytes: %v", err)
	}

	supi, err := etsi.NewSUPIFromPrefixed("imsi-001010000000001")
	if err != nil {
		t.Fatalf("NewSUPIFromPrefixed: %v", err)
	}

	amfInstance := amf.New(nil, nil, nil)

	ue := amf.NewUeContext()
	ue.SetSupiForTest(supi)
	ue.SetSecuredForTest(true)
	ue.SetIntegrityAlgForTest(security.AlgIntegrity128NIA2)
	ue.SetKnasIntForTest([16]uint8{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16})
	ue.ForceStateForTest(amf.Registered)

	if err := amfInstance.CommitUEIdentity(context.Background(), ue, amf.MintAuthProofForRegistrationCommit()); err != nil {
		t.Fatalf("CommitUEIdentity: %v", err)
	}

	amfInstance.AssignGutiForTest(ue, guti)

	pdu := wrapIntegrityProtected(t, ue, plainDeregistrationWithGuti(t, gutiBytes), 0)

	got, err := fetchUeContextWithMobileIdentity(context.Background(), amfInstance, pdu)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got != ue {
		t.Fatal("an integrity-verified deregistration citing a known GUTI must resolve to the existing context")
	}
}

// TestFetchUeContext_PlainRegistrationDoesNotReuseRegisteredVictim is the
// end-to-end regression for the GUTI-spoof DoS: a plain (unauthenticated)
// initial REGISTRATION REQUEST that resolves by GUTI to a registered UE must be
// routed to a fresh context, leaving the victim's committed context untouched
// (TS 24.501).
func TestFetchUeContext_PlainRegistrationDoesNotReuseRegisteredVictim(t *testing.T) {
	// type byte 0x02 = 5G-GUTI; PLMN 001/01; amf.AMF id 0xcafe00; 5G-TMSI 1.
	gutiBytes := []byte{0x02, 0x00, 0xf1, 0x10, 0xca, 0xfe, 0x00, 0x00, 0x00, 0x00, 0x01}

	guti, err := etsi.NewGUTI5GFromBytes(gutiBytes)
	if err != nil {
		t.Fatalf("NewGUTIFromBytes: %v", err)
	}

	supi, err := etsi.NewSUPIFromPrefixed("imsi-001010000000001")
	if err != nil {
		t.Fatalf("NewSUPIFromPrefixed: %v", err)
	}

	amfInstance := amf.New(nil, nil, nil)

	victim := amf.NewUeContext()
	victim.SetSupiForTest(supi)
	victim.SetGutiForTest(guti)
	victim.SetSecuredForTest(true)
	victim.ForceStateForTest(amf.Registered)

	if err := amfInstance.CommitUEIdentity(context.Background(), victim, amf.MintAuthProofForRegistrationCommit()); err != nil {
		t.Fatalf("CommitUEIdentity: %v", err)
	}

	got, err := fetchUeContextWithMobileIdentity(context.Background(), amfInstance, plainRegistrationWithGuti(t, gutiBytes))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got != nil {
		t.Fatal("a plain registration must be routed to a fresh context, not bound to the registered victim (TS 24.501)")
	}

	if !victim.SecuredForTest() {
		t.Fatal("victim security context must remain intact")
	}

	if victim.State() != amf.Registered {
		t.Fatalf("victim must remain amf.Registered, got %v", victim.State())
	}
}
