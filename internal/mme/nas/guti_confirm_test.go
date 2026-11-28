// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"context"
	"encoding/binary"
	"testing"

	"github.com/ellanetworks/core/internal/mme"
	"github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/nas/eps"
	"github.com/ellanetworks/core/s1ap"
)

func stageGUTIRealloc(t *testing.T, m *mme.MME, ue *mme.UeContext) (eps.EPSMobileIdentity, eps.EPSMobileIdentity) {
	t.Helper()

	plmn, err := m.OperatorPLMN(t.Context())
	if err != nil {
		t.Fatal(err)
	}

	group, code, err := m.MmeIdentity(t.Context())
	if err != nil {
		t.Fatal(err)
	}

	old, err := m.ReallocateGUTI(t.Context(), ue, plmn, group, code)
	if err != nil {
		t.Fatal(err)
	}

	m.CommitGUTIRealloc(ue)

	staged, err := m.ReallocateGUTI(t.Context(), ue, plmn, group, code)
	if err != nil {
		t.Fatal(err)
	}

	if mtmsiOf(old) == mtmsiOf(staged) || ue.OldTmsiForTest() != mtmsiOf(old) {
		t.Fatalf("fixture must stage a reallocation: old=%d staged=%d", mtmsiOf(old), mtmsiOf(staged))
	}

	return old, staged
}

func mtmsiOf(id eps.EPSMobileIdentity) uint32 {
	return binary.BigEndian.Uint32(id.GUTI.TMSI[:])
}

func protectedTAU(t *testing.T, ue *mme.UeContext, oldGUTI eps.EPSMobileIdentity) []byte {
	t.Helper()

	plain, err := (&eps.TrackingAreaUpdateRequest{EPSUpdateType: eps.EPSUpdateTypePeriodic, OldGUTI: oldGUTI}).MarshalBinary()
	if err != nil {
		t.Fatalf("encode TAU Request: %v", err)
	}

	wire, err := eps.Protect(plain, eps.SHTIntegrityProtectedCiphered, nas.MakeCount(0, uint8(ue.ULCount())), nas.DirectionUplink,
		mustSecurityContext(t, ue.EIA(), ue.EEA(), ue.KnasIntForTest(), ue.KnasEncForTest()))
	if err != nil {
		t.Fatal(err)
	}

	return wire
}

func downlinkGUTI(t *testing.T, ue *mme.UeContext, pdu []byte) uint32 {
	t.Helper()

	dl := decodeDownlinkNAS(t, pdu)

	plain, err := unprotected(eps.Unprotect(dl, nas.MakeCount(0, dl[5]), nas.DirectionDownlink,
		mustSecurityContext(t, ue.EIA(), ue.EEA(), ue.KnasIntForTest(), ue.KnasEncForTest())))
	if err != nil {
		t.Fatalf("unprotect downlink: %v", err)
	}

	msg, err := eps.ParseMessage(plain, nas.DirectionDownlink)
	if err != nil {
		t.Fatalf("parse downlink: %v", err)
	}

	switch msg := msg.(type) {
	case *eps.TrackingAreaUpdateAccept:
		if msg.GUTI == nil || msg.GUTI.GUTI == nil {
			t.Fatal("TAU Accept carries no GUTI")
		}

		return mtmsiOf(*msg.GUTI)
	case *eps.GUTIReallocationCommand:
		if msg.GUTI.GUTI == nil {
			t.Fatal("GUTI Reallocation Command carries no GUTI")
		}

		return mtmsiOf(msg.GUTI)
	default:
		t.Fatalf("downlink %T carries no GUTI", msg)
		return 0
	}
}

func TestTrackingAreaUpdateWithTheNewGUTIInvalidatesTheOldAndReallocatesAFreshOne(t *testing.T) {
	m := newTestMME(t)
	ue, cc := securedUE(t, m)
	old, staged := stageGUTIRealloc(t, m, ue)

	HandleNAS(context.Background(), m, ue.Conn(), protectedTAU(t, ue, staged))

	if _, ok := m.LookupUeByMTMSI(mtmsiOf(old)); ok {
		t.Fatal("the old M-TMSI still resolves after the UE used the new GUTI")
	}

	fresh := ue.TmsiForTest()
	if fresh == mtmsiOf(old) || fresh == mtmsiOf(staged) {
		t.Fatalf("the TAU must draw a fresh M-TMSI, got %d (old %d, previously new %d)", fresh, mtmsiOf(old), mtmsiOf(staged))
	}

	if ue.OldTmsiForTest() != mtmsiOf(staged) {
		t.Fatalf("staged old M-TMSI = %d, want the GUTI the UE presented %d", ue.OldTmsiForTest(), mtmsiOf(staged))
	}

	if len(cc.sent) != 1 {
		t.Fatalf("expected one downlink (TAU Accept), got %d", len(cc.sent))
	}

	if sent := downlinkGUTI(t, ue, cc.sent[0]); sent != fresh {
		t.Fatalf("TAU Accept carries M-TMSI %d, want the fresh %d", sent, fresh)
	}
}

func TestTrackingAreaUpdateWithTheOldGUTIKeepsTheReallocationPending(t *testing.T) {
	m := newTestMME(t)
	ue, cc := securedUE(t, m)
	old, staged := stageGUTIRealloc(t, m, ue)

	HandleNAS(context.Background(), m, ue.Conn(), protectedTAU(t, ue, old))

	if ue.OldTmsiForTest() != mtmsiOf(old) || ue.TmsiForTest() != mtmsiOf(staged) {
		t.Fatalf("a TAU citing the old GUTI changed the reallocation: old=%d new=%d", ue.OldTmsiForTest(), ue.TmsiForTest())
	}

	if _, ok := m.LookupUeByMTMSI(mtmsiOf(old)); !ok {
		t.Fatal("the old M-TMSI must stay resolvable while the reallocation is pending")
	}

	if len(cc.sent) != 1 {
		t.Fatalf("expected one downlink (TAU Accept), got %d", len(cc.sent))
	}

	if sent := downlinkGUTI(t, ue, cc.sent[0]); sent != mtmsiOf(staged) {
		t.Fatalf("TAU Accept carries M-TMSI %d, want the staged %d", sent, mtmsiOf(staged))
	}
}

func TestTrackingAreaUpdateFailingIntegrityWithTheNewGUTIKeepsTheReallocationPending(t *testing.T) {
	m := newTestMME(t)
	ue, _ := securedUE(t, m)
	old, staged := stageGUTIRealloc(t, m, ue)

	pdu := protectedTAU(t, ue, staged)
	pdu[2] ^= 0xff

	HandleNAS(context.Background(), m, ue.Conn(), pdu)

	if ue.OldTmsiForTest() != mtmsiOf(old) || ue.TmsiForTest() != mtmsiOf(staged) {
		t.Fatalf("an unverified TAU changed the reallocation: old=%d new=%d", ue.OldTmsiForTest(), ue.TmsiForTest())
	}

	if _, ok := m.LookupUeByMTMSI(mtmsiOf(old)); !ok {
		t.Fatal("an unverified TAU freed the old M-TMSI")
	}
}

func TestUnverifiedTrackingAreaUpdateWithTheNewGUTIDoesNotCommit(t *testing.T) {
	m := newTestMME(t)
	ue, _ := securedUE(t, m)
	old, staged := stageGUTIRealloc(t, m, ue)

	req := &eps.TrackingAreaUpdateRequest{EPSUpdateType: eps.EPSUpdateTypePeriodic, OldGUTI: staged}

	plain, err := req.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}

	HandleEmmMessage(context.Background(), m, ue, ue.Conn(), plain, false)

	if _, ok := m.LookupUeByMTMSI(mtmsiOf(old)); !ok {
		t.Fatal("a message not integrity-verified freed the old M-TMSI")
	}

	if ue.OldTmsiForTest() == mtmsiOf(staged) {
		t.Fatal("a message not integrity-verified committed the reallocation")
	}
}

func idleUEWithStagedRealloc(t *testing.T, m *mme.MME) (*mme.UeContext, eps.EPSMobileIdentity, eps.EPSMobileIdentity) {
	t.Helper()

	ue, _ := idleRegisteredUE(t, m)
	old, staged := stageGUTIRealloc(t, m, ue)

	return ue, old, staged
}

func serviceRequestFor(t *testing.T, ue *mme.UeContext, id eps.EPSMobileIdentity, tamper bool) *s1ap.InitialUEMessage {
	t.Helper()

	pdu := serviceRequestNAS(t, ue)
	if tamper {
		pdu[3] ^= 0xff
	}

	return &s1ap.InitialUEMessage{
		ENBUES1APID: 9,
		NASPDU:      s1ap.NASPDU(pdu),
		STMSI:       &s1ap.STMSI{MMEC: s1ap.MMECode(id.GUTI.MMECode), MTMSI: s1ap.MTMSI(mtmsiOf(id))},
	}
}

func TestServiceRequestWithTheNewSTMSIKeepsTheReallocationPending(t *testing.T) {
	m := newTestMME(t)
	ue, old, staged := idleUEWithStagedRealloc(t, m)

	cc := &captureConn{}
	HandleServiceRequest(context.Background(), m, cc, serviceRequestFor(t, ue, staged, false))

	if !ue.Connected() {
		t.Fatal("a verified Service Request must re-establish the UE")
	}

	if ue.OldTmsiForTest() != mtmsiOf(old) || ue.TmsiForTest() != mtmsiOf(staged) {
		t.Fatalf("an S-TMSI outside the short MAC changed the reallocation: old=%d new=%d, want old=%d new=%d",
			ue.OldTmsiForTest(), ue.TmsiForTest(), mtmsiOf(old), mtmsiOf(staged))
	}

	if _, ok := m.LookupUeByMTMSI(mtmsiOf(old)); !ok {
		t.Fatal("an S-TMSI outside the short MAC freed the old M-TMSI")
	}

	if len(cc.sent) != 2 {
		t.Fatalf("expected Initial Context Setup Request + GUTI Reallocation Command, got %d", len(cc.sent))
	}

	if sent := downlinkGUTI(t, ue, cc.sent[1]); sent != mtmsiOf(staged) {
		t.Fatalf("GUTI Reallocation Command carries M-TMSI %d, want the staged %d", sent, mtmsiOf(staged))
	}
}

func TestServiceRequestWithTheOldGUTIKeepsTheReallocationPending(t *testing.T) {
	m := newTestMME(t)
	ue, old, staged := idleUEWithStagedRealloc(t, m)

	cc := &captureConn{}
	HandleServiceRequest(context.Background(), m, cc, serviceRequestFor(t, ue, old, false))

	if ue.OldTmsiForTest() != mtmsiOf(old) || ue.TmsiForTest() != mtmsiOf(staged) {
		t.Fatalf("a Service Request citing the old GUTI changed the reallocation: old=%d new=%d", ue.OldTmsiForTest(), ue.TmsiForTest())
	}

	if len(cc.sent) != 2 {
		t.Fatalf("expected Initial Context Setup Request + GUTI Reallocation Command, got %d", len(cc.sent))
	}

	if sent := downlinkGUTI(t, ue, cc.sent[1]); sent != mtmsiOf(staged) {
		t.Fatalf("GUTI Reallocation Command carries M-TMSI %d, want the staged %d", sent, mtmsiOf(staged))
	}
}

func TestServiceRequestBadMACWithTheNewGUTIKeepsTheReallocationPending(t *testing.T) {
	m := newTestMME(t)
	ue, old, staged := idleUEWithStagedRealloc(t, m)

	cc := &captureConn{}
	HandleServiceRequest(context.Background(), m, cc, serviceRequestFor(t, ue, staged, true))

	if ue.Connected() {
		t.Fatal("UE reconnected despite a bad short MAC")
	}

	if ue.OldTmsiForTest() != mtmsiOf(old) || ue.TmsiForTest() != mtmsiOf(staged) {
		t.Fatalf("an unverified Service Request changed the reallocation: old=%d new=%d", ue.OldTmsiForTest(), ue.TmsiForTest())
	}

	if _, ok := m.LookupUeByMTMSI(mtmsiOf(old)); !ok {
		t.Fatal("an unverified Service Request freed the old M-TMSI")
	}
}

func protectedEMM(t *testing.T, ue *mme.UeContext, msg interface{ MarshalBinary() ([]byte, error) }) []byte {
	t.Helper()

	plain, err := msg.MarshalBinary()
	if err != nil {
		t.Fatalf("encode uplink: %v", err)
	}

	wire, err := eps.Protect(plain, eps.SHTIntegrityProtectedCiphered, nas.MakeCount(0, uint8(ue.ULCount())), nas.DirectionUplink,
		mustSecurityContext(t, ue.EIA(), ue.EEA(), ue.KnasIntForTest(), ue.KnasEncForTest()))
	if err != nil {
		t.Fatal(err)
	}

	return wire
}

func committed(m *mme.MME, ue *mme.UeContext, old eps.EPSMobileIdentity) bool {
	_, oldResolves := m.LookupUeByMTMSI(mtmsiOf(old))

	return ue.OldTmsiForTest() != mtmsiOf(old) && !oldResolves
}

func TestAttachWithTheNewGUTIInvalidatesTheOld(t *testing.T) {
	m := newTestMME(t)
	ue, _ := securedUE(t, m)
	old, staged := stageGUTIRealloc(t, m, ue)

	esm, err := (&eps.PDNConnectivityRequest{PTI: 1, RequestType: 1, PDNType: 1}).MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}

	native := eps.GUTITypeNative

	HandleNAS(context.Background(), m, ue.Conn(), protectedEMM(t, ue, &eps.AttachRequest{
		EPSAttachType:       eps.AttachTypeEPS,
		NASKeySetIdentifier: nas.KeySetIdentifier{Value: 2},
		EPSMobileIdentity:   staged,
		UENetworkCapability: eps.UENetworkCapability{EEA: 0xf0, EIA: 0x70},
		ESMMessageContainer: esm,
		OldGUTIType:         &native,
	}))

	if !committed(m, ue, old) {
		t.Fatalf("an integrity-verified Attach citing the new GUTI left the reallocation pending: old=%d new=%d", ue.OldTmsiForTest(), ue.TmsiForTest())
	}
}

func TestDetachWithTheNewGUTIInvalidatesTheOld(t *testing.T) {
	m := newTestMME(t)
	ue, _ := securedUE(t, m)
	old, staged := stageGUTIRealloc(t, m, ue)

	HandleNAS(context.Background(), m, ue.Conn(), protectedEMM(t, ue, &eps.DetachRequestUE{
		SwitchOff:         true,
		TypeOfDetach:      eps.DetachTypeEPS,
		EPSMobileIdentity: staged,
	}))

	if !committed(m, ue, old) {
		t.Fatalf("an integrity-verified Detach citing the new GUTI left the reallocation pending: old=%d new=%d", ue.OldTmsiForTest(), ue.TmsiForTest())
	}
}

func TestTrackingAreaUpdateWithAMappedOldGUTIAndTheNewAdditionalGUTIInvalidatesTheOld(t *testing.T) {
	m := newTestMME(t)
	ue, cc := securedUE(t, m)
	old, staged := stageGUTIRealloc(t, m, ue)

	mapped := eps.GUTITypeMapped
	mappedGUTI := *staged.GUTI
	mappedGUTI.MMEGroupID |= 0x8000
	mappedGUTI.TMSI = old.GUTI.TMSI

	HandleNAS(context.Background(), m, ue.Conn(), protectedEMM(t, ue, &eps.TrackingAreaUpdateRequest{
		EPSUpdateType:  eps.EPSUpdateTypePeriodic,
		OldGUTI:        eps.GUTIIdentity(mappedGUTI),
		OldGUTIType:    &mapped,
		AdditionalGUTI: &staged,
	}))

	if _, ok := m.LookupUeByMTMSI(mtmsiOf(old)); ok {
		t.Fatal("the old M-TMSI still resolves after the UE used the new GUTI in the Additional GUTI")
	}

	fresh := ue.TmsiForTest()
	if fresh == mtmsiOf(old) || fresh == mtmsiOf(staged) {
		t.Fatalf("the TAU must draw a fresh M-TMSI, got %d", fresh)
	}

	if len(cc.sent) != 1 {
		t.Fatalf("expected one downlink (TAU Accept), got %d", len(cc.sent))
	}

	if sent := downlinkGUTI(t, ue, cc.sent[0]); sent != fresh {
		t.Fatalf("TAU Accept carries M-TMSI %d, want the fresh %d", sent, fresh)
	}
}

func TestPresentedGUTIsSkipTheOldGUTIMappedFrom5GS(t *testing.T) {
	m := newTestMME(t)
	ue, _ := securedUE(t, m)
	old, staged := stageGUTIRealloc(t, m, ue)

	native := eps.GUTITypeNative

	confirmGUTIRealloc(context.Background(), m, ue, &eps.TrackingAreaUpdateRequest{
		EPSUpdateType: eps.EPSUpdateTypeTA,
		OldGUTI:       staged,
		OldGUTIType:   &native,
		UEStatus:      &eps.UEStatus{N1ModeReg: true},
	})

	if committed(m, ue, old) {
		t.Fatal("a GUTI mapped from a 5G-GUTI committed the EPS reallocation")
	}
}

func TestTrackingAreaUpdateWithAForeignGUMMEIKeepsTheReallocationPending(t *testing.T) {
	m := newTestMME(t)
	ue, cc := securedUE(t, m)
	old, staged := stageGUTIRealloc(t, m, ue)

	foreign := *staged.GUTI
	foreign.MMECode++

	HandleNAS(context.Background(), m, ue.Conn(), protectedTAU(t, ue, eps.GUTIIdentity(foreign)))

	if ue.OldTmsiForTest() != mtmsiOf(old) || ue.TmsiForTest() != mtmsiOf(staged) {
		t.Fatalf("a GUTI of another MME changed the reallocation: old=%d new=%d", ue.OldTmsiForTest(), ue.TmsiForTest())
	}

	if _, ok := m.LookupUeByMTMSI(mtmsiOf(old)); !ok {
		t.Fatal("a GUTI of another MME freed the old M-TMSI")
	}

	if len(cc.sent) != 1 {
		t.Fatalf("expected one downlink (TAU Accept), got %d", len(cc.sent))
	}

	if sent := downlinkGUTI(t, ue, cc.sent[0]); sent != mtmsiOf(staged) {
		t.Fatalf("TAU Accept carries M-TMSI %d, want the staged %d", sent, mtmsiOf(staged))
	}
}

func TestPresentedGUTIsSkipAMappedAttachIdentity(t *testing.T) {
	m := newTestMME(t)
	ue, _ := securedUE(t, m)
	old, staged := stageGUTIRealloc(t, m, ue)

	mapped := eps.GUTITypeMapped

	confirmGUTIRealloc(context.Background(), m, ue, &eps.AttachRequest{
		EPSAttachType:     eps.AttachTypeEPS,
		EPSMobileIdentity: staged,
		OldGUTIType:       &mapped,
	})

	if committed(m, ue, old) {
		t.Fatal("a GUTI mapped from a P-TMSI committed the EPS reallocation")
	}

	confirmGUTIRealloc(context.Background(), m, ue, &eps.AttachRequest{
		EPSAttachType:     eps.AttachTypeEPS,
		EPSMobileIdentity: old,
		OldGUTIType:       &mapped,
		AdditionalGUTI:    &staged,
	})

	if !committed(m, ue, old) {
		t.Fatal("the native GUTI in the Additional GUTI did not commit the EPS reallocation")
	}
}
