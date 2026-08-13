// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/nas/fgs"
)

func reuseTestAMF() *amf.AMF {
	return amf.New(&fakeDBInstance{Operator: &db.Operator{
		Mcc: "001", Mnc: "01", AmfRegionID: 0xca, AmfSetID: 0x3f,
	}}, nil, nil)
}

func plainRegistrationWithGuti(t *testing.T, guti fgs.MobileIdentity) []byte {
	t.Helper()

	rr := &fgs.RegistrationRequest{
		RegistrationType:     fgs.RegistrationTypeInitial,
		FOR:                  true,
		MobileIdentity:       guti,
		UESecurityCapability: &fgs.UESecurityCapability{},
	}

	payload, err := rr.MarshalBinary()
	if err != nil {
		t.Fatalf("encode plain RegistrationRequest: %v", err)
	}

	return payload
}

func plainDeregistrationWithGuti(t *testing.T, guti fgs.MobileIdentity) []byte {
	t.Helper()

	dr := &fgs.DeregistrationRequestUEOriginating{
		AccessType:     fgs.AccessType3GPP,
		MobileIdentity: guti,
	}

	payload, err := dr.MarshalBinary()
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

	cnt, _ := ue.ULCountForTest().Estimate(sqn)

	seqAndMsg := append([]byte{sqn}, inner...)

	sc := mustSecurityContext(t, ue.IntegrityAlgForTest(),
		ue.CipheringAlgForTest(), ue.KnasIntForTest(), ue.KnasEncForTest())

	mac, err := sc.MAC(seqAndMsg, cnt, nas.Bearer3GPP, nas.DirectionUplink)
	if err != nil {
		t.Fatalf("could not compute NAS MAC: %v", err)
	}

	pdu := []byte{0x7e, byte(fgs.SHTIntegrityProtected)}
	pdu = append(pdu, mac[:]...)
	pdu = append(pdu, seqAndMsg...)

	return pdu
}

// TestFetchUeContext_DeregistrationResolvesExistingContextByGuti guards the
// GUTI-shadowing defect: an integrity-verified UE-originating DEREGISTRATION
// citing a known GUTI must resolve to the existing context. A local guti that
// shadows the outer guti leaves it invalid and forces a fresh context.
func TestFetchUeContext_DeregistrationResolvesExistingContextByGuti(t *testing.T) {
	gutiID := fgs.GUTIIdentity(fgs.GUTI{
		PLMN: nas.PLMN{MCC: "001", MNC: "01"}, AMFRegionID: 0xca, AMFSetID: 0x3f, AMFPointer: 0x00,
		TMSI: [4]byte{0x00, 0x00, 0x00, 0x01},
	})

	guti, err := etsi.NewGUTI5GFromNAS(gutiID)
	if err != nil {
		t.Fatalf("NewGUTI5GFromNAS: %v", err)
	}

	supi, err := etsi.NewSUPIFromPrefixed("imsi-001010000000001")
	if err != nil {
		t.Fatalf("NewSUPIFromPrefixed: %v", err)
	}

	amfInstance := reuseTestAMF()

	ue := amf.NewUeContext()
	ue.SetSupiForTest(supi)
	ue.SetSecuredForTest(true)
	ue.SetIntegrityAlgForTest(nas.IntegrityAES)
	ue.SetKnasIntForTest([16]uint8{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16})
	ue.ForceStateForTest(amf.Registered)

	if err := amfInstance.CommitUEIdentity(context.Background(), ue, amf.MintAuthProofForRegistrationCommit()); err != nil {
		t.Fatalf("CommitUEIdentity: %v", err)
	}

	amfInstance.AssignGutiForTest(ue, guti)

	pdu := wrapIntegrityProtected(t, ue, plainDeregistrationWithGuti(t, gutiID), 0)

	got, err := fetchUeContextWithMobileIdentity(context.Background(), amfInstance, pdu)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got != ue {
		t.Fatal("an integrity-verified deregistration citing a known GUTI must resolve to the existing context")
	}
}

func gutiWithTMSI(t *testing.T, tmsi [4]byte) (fgs.MobileIdentity, etsi.GUTI5G) {
	t.Helper()

	id := fgs.GUTIIdentity(fgs.GUTI{
		PLMN: nas.PLMN{MCC: "001", MNC: "01"}, AMFRegionID: 0xca, AMFSetID: 0x3f, AMFPointer: 0x00,
		TMSI: tmsi,
	})

	guti, err := etsi.NewGUTI5GFromNAS(id)
	if err != nil {
		t.Fatalf("NewGUTI5GFromNAS: %v", err)
	}

	return id, guti
}

func securedUeForTest(t *testing.T, amfInstance *amf.AMF, imsi string, guti etsi.GUTI5G, knasInt [16]uint8) *amf.UeContext {
	t.Helper()

	supi, err := etsi.NewSUPIFromPrefixed(imsi)
	if err != nil {
		t.Fatalf("NewSUPIFromPrefixed: %v", err)
	}

	ue := amf.NewUeContext()
	ue.SetSupiForTest(supi)
	ue.SetSecuredForTest(true)
	ue.SetIntegrityAlgForTest(nas.IntegrityAES)
	ue.SetKnasIntForTest(knasInt)
	ue.ForceStateForTest(amf.Registered)

	amfInstance.AssignGutiForTest(ue, guti)

	return ue
}

// TestFetchUeContext_InterSystemChangeResolvesTheAdditionalGUTI covers the
// 5G-GUTI mapped from a 4G-GUTI carrying this deployment's own GUAMI, so it
// resolves in the AMF's own 5G-TMSI index even though it names the MME. The
// native context lives behind the Additional GUTI IE and must still be found
// (TS 24.501 §5.5.1.3.2 a) NOTE 6, §5.5.1.3.4 a) and c)).
func TestFetchUeContext_InterSystemChangeResolvesTheAdditionalGUTI(t *testing.T) {
	amfInstance := reuseTestAMF()

	collidingID, collidingGuti := gutiWithTMSI(t, [4]byte{0x00, 0x00, 0x00, 0x01})
	nativeID, nativeGuti := gutiWithTMSI(t, [4]byte{0x00, 0x00, 0x00, 0x02})

	stranger := securedUeForTest(t, amfInstance, "imsi-001010000000001", collidingGuti,
		[16]uint8{9, 9, 9, 9, 9, 9, 9, 9, 9, 9, 9, 9, 9, 9, 9, 9})
	mover := securedUeForTest(t, amfInstance, "imsi-001010000000002", nativeGuti,
		[16]uint8{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16})

	inner, err := (&fgs.RegistrationRequest{
		RegistrationType:       fgs.RegistrationTypeMobilityUpdating,
		FOR:                    true,
		MobileIdentity:         collidingID,
		AdditionalGUTI:         &nativeID,
		UEStatus:               &fgs.UEStatus{S1ModeReg: true},
		UESecurityCapability:   &fgs.UESecurityCapability{},
		EPSNASMessageContainer: []byte{0x17, 0xde, 0xad, 0xbe, 0xef, 0x00, 0x07, 0x48},
	}).MarshalBinary()
	if err != nil {
		t.Fatalf("encode RegistrationRequest: %v", err)
	}

	got, err := fetchUeContextWithMobileIdentity(context.Background(), amfInstance,
		wrapIntegrityProtected(t, mover, inner, 0))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got == stranger {
		t.Fatal("the mapped 5G-GUTI resolved another subscriber's context")
	}

	if got != mover {
		t.Fatal("the UE's native 5G context was not found behind the Additional GUTI, so the AMF re-authenticates a UE it can already verify")
	}
}

// TestFetchUeContext_PlainRegistrationDoesNotReuseRegisteredVictim is the
// end-to-end regression for the GUTI-spoof DoS: a plain (unauthenticated)
// initial REGISTRATION REQUEST that resolves by GUTI to a registered UE must be
// routed to a fresh context, leaving the victim's committed context untouched
// (TS 24.501).
func TestFetchUeContext_PlainRegistrationDoesNotReuseRegisteredVictim(t *testing.T) {
	gutiID := fgs.GUTIIdentity(fgs.GUTI{
		PLMN: nas.PLMN{MCC: "001", MNC: "01"}, AMFRegionID: 0xca, AMFSetID: 0x3f, AMFPointer: 0x00,
		TMSI: [4]byte{0x00, 0x00, 0x00, 0x01},
	})

	guti, err := etsi.NewGUTI5GFromNAS(gutiID)
	if err != nil {
		t.Fatalf("NewGUTI5GFromNAS: %v", err)
	}

	supi, err := etsi.NewSUPIFromPrefixed("imsi-001010000000001")
	if err != nil {
		t.Fatalf("NewSUPIFromPrefixed: %v", err)
	}

	amfInstance := reuseTestAMF()

	victim := amf.NewUeContext()
	victim.SetSupiForTest(supi)
	victim.SetGutiForTest(guti)
	victim.SetSecuredForTest(true)
	victim.ForceStateForTest(amf.Registered)

	if err := amfInstance.CommitUEIdentity(context.Background(), victim, amf.MintAuthProofForRegistrationCommit()); err != nil {
		t.Fatalf("CommitUEIdentity: %v", err)
	}

	got, err := fetchUeContextWithMobileIdentity(context.Background(), amfInstance, plainRegistrationWithGuti(t, gutiID))
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
