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
	"go.uber.org/zap"
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

// TestFetchUeContext_PlainRegistrationDoesNotReuseRegisteredVictim is the
// end-to-end regression for the GUTI-spoof DoS: a plain (unauthenticated)
// initial REGISTRATION REQUEST that resolves by GUTI to a registered UE must be
// routed to a fresh context, leaving the victim's committed context untouched
// (TS 24.501).
func TestFetchUeContext_PlainRegistrationDoesNotReuseRegisteredVictim(t *testing.T) {
	// type byte 0x02 = 5G-GUTI; PLMN 001/01; amf.AMF id 0xcafe00; 5G-TMSI 1.
	gutiBytes := []byte{0x02, 0x00, 0xf1, 0x10, 0xca, 0xfe, 0x00, 0x00, 0x00, 0x00, 0x01}

	guti, err := etsi.NewGUTIFromBytes(gutiBytes)
	if err != nil {
		t.Fatalf("NewGUTIFromBytes: %v", err)
	}

	supi, err := etsi.NewSUPIFromPrefixed("imsi-001010000000001")
	if err != nil {
		t.Fatalf("NewSUPIFromPrefixed: %v", err)
	}

	amfInstance := amf.New(nil, nil, nil)

	victim := amf.NewUeContext()
	victim.Log = zap.NewNop()
	victim.SetSupiForTest(supi)
	victim.SetGutiForTest(guti)
	victim.SetSecurityContextAvailableForTest(true)
	victim.ForceState(amf.Registered)

	if err := amfInstance.AddUeContextToPool(victim); err != nil {
		t.Fatalf("AddUeContextToPool: %v", err)
	}

	got, err := fetchUeContextWithMobileIdentity(context.Background(), amfInstance, plainRegistrationWithGuti(t, gutiBytes))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got != nil {
		t.Fatal("a plain registration must be routed to a fresh context, not bound to the registered victim (TS 24.501)")
	}

	if !victim.SecurityContextAvailableForTest() {
		t.Fatal("victim security context must remain intact")
	}

	if victim.GetState() != amf.Registered {
		t.Fatalf("victim must remain amf.Registered, got %v", victim.GetState())
	}
}
