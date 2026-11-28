// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"encoding/hex"
	"testing"

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/ausf"
	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/nas/fgs"
)

type pendingReallocFixture struct {
	amf        *amf.AMF
	ue         *amf.UeContext
	ngapSender *fakeNGAPSender
	key        [16]uint8
	oldTmsi    etsi.TMSI
	newTmsi    etsi.TMSI
}

func newPendingReallocFixture(t *testing.T) pendingReallocFixture {
	t.Helper()

	amfInstance := amf.New(
		&fakeDBInstance{
			Operator: &db.Operator{Mcc: "001", Mnc: "01", SupportedTACs: `["000001"]`},
		},
		&fakeAusf{
			AvKgAka: &ausf.AuthResult{Rand: hex.EncodeToString(make([]byte, 16)), Autn: hex.EncodeToString(make([]byte, 16))},
			Supi:    mustSUPIFromPrefixed("imsi-001019756139935"),
			Kseaf:   []byte("testkey"),
		},
		&fakeSmf{},
	)

	ue, ngapSender, err := buildUeAndRadio()
	if err != nil {
		t.Fatalf("could not build UE and radio: %v", err)
	}

	amfInstance.AttachUeConn(t.Context(), ue, ue.Conn())

	ue.SetPlmnIDForTest(models.PlmnID{Mcc: "001", Mnc: "01"})
	ue.ForceStateForTest(amf.Registered)
	ue.Tai = ue.Conn().Tai
	ue.SetSecuredForTest(true)

	ng := ue.NgKsiForTest()
	ng.Ksi = 1
	ue.SetNgKsiForTest(ng)

	key := [16]uint8{0x0D, 0x0E, 0x0A, 0x0D, 0x0B, 0x0E, 0x0E, 0x0F, 0x0F, 0x0E, 0x0E, 0x0D, 0x0C, 0x0A, 0x0F, 0x0E}

	ue.SetKnasEncForTest(key)
	ue.SetKnasIntForTest(key)
	ue.SetCipheringAlgForTest(nas.CipheringAES)
	ue.SetIntegrityAlgForTest(nas.IntegrityNull)

	if err := amfInstance.ReallocateGUTI(t.Context(), ue); err != nil {
		t.Fatalf("ReallocateGUTI (initial): %v", err)
	}

	amfInstance.CommitGUTIRealloc(ue)

	if err := amfInstance.ReallocateGUTI(t.Context(), ue); err != nil {
		t.Fatalf("ReallocateGUTI (staged): %v", err)
	}

	f := pendingReallocFixture{
		amf:        amfInstance,
		ue:         ue,
		ngapSender: ngapSender,
		key:        key,
		oldTmsi:    ue.OldTmsi(),
		newTmsi:    ue.TmsiForTest(),
	}

	if f.oldTmsi == etsi.InvalidTMSI || f.newTmsi == f.oldTmsi {
		t.Fatalf("fixture must stage a reallocation: old=%v new=%v", f.oldTmsi, f.newTmsi)
	}

	return f
}

func (f pendingReallocFixture) stmsiFor(t *testing.T, tmsi etsi.TMSI) fgs.MobileIdentity {
	t.Helper()

	info, err := f.amf.OperatorInfo(t.Context())
	if err != nil {
		t.Fatalf("OperatorInfo: %v", err)
	}

	guti, err := etsi.NewGUTI5G(info.Guami.PlmnID.Mcc, info.Guami.PlmnID.Mnc, info.Guami.AmfID, tmsi)
	if err != nil {
		t.Fatalf("NewGUTI5G: %v", err)
	}

	id, err := guti.MobileIdentity()
	if err != nil {
		t.Fatalf("MobileIdentity: %v", err)
	}

	return fgs.STMSIIdentity(fgs.STMSI{AMFSetID: id.GUTI.AMFSetID, AMFPointer: id.GUTI.AMFPointer, TMSI: id.GUTI.TMSI})
}

func (f pendingReallocFixture) pagingResponse(t *testing.T, tmsi etsi.TMSI) []byte {
	t.Helper()

	m, err := buildServiceRequestCipheredFor(nas.CipheringAES, f.key, f.ue.ULCount(), fgs.ServiceTypeMobileTerminatedServices, f.stmsiFor(t, tmsi))
	if err != nil {
		t.Fatalf("could not build service request: %v", err)
	}

	return encSR(t, m)
}

func (f pendingReallocFixture) lastConfigurationUpdateGUTI(t *testing.T) etsi.TMSI {
	t.Helper()

	sent := f.ngapSender.SentDownlinkNASTransport
	if len(sent) == 0 {
		t.Fatal("no downlink NAS transport sent")
	}

	plain := decipherGmmCount(t, f.ue, sent[len(sent)-1].NASPDU, uint32(f.ue.DLCountForTest())-1, uint8(fgs.MsgConfigurationUpdateCommand))

	cuc, err := fgs.ParseConfigurationUpdateCommand(plain)
	if err != nil {
		t.Fatalf("decode Configuration Update Command: %v", err)
	}

	if cuc.GUTI == nil {
		t.Fatal("Configuration Update Command carries no 5G-GUTI")
	}

	guti, err := etsi.NewGUTI5GFromNAS(*cuc.GUTI)
	if err != nil {
		t.Fatalf("NewGUTI5GFromNAS: %v", err)
	}

	return guti.Tmsi
}

func TestPagingResponseWithTheNewGUTIInvalidatesTheOldAndReallocatesAFreshOne(t *testing.T) {
	f := newPendingReallocFixture(t)

	HandleGmmMessage(t.Context(), f.amf, f.ue, uint8(fgs.MsgServiceRequest), f.pagingResponse(t, f.newTmsi), true, false)

	if f.amf.TmsiInUseForTest(f.oldTmsi) {
		t.Fatal("the old 5G-TMSI is still allocated after the UE used the new 5G-GUTI")
	}

	if _, ok := f.amf.LookupUeByGuti(mustGuami(t, f.amf), mustGutiFor(t, f.amf, f.oldTmsi)); ok {
		t.Fatal("the old 5G-GUTI still resolves after the UE used the new 5G-GUTI")
	}

	fresh := f.ue.TmsiForTest()
	if fresh == f.newTmsi || fresh == f.oldTmsi {
		t.Fatalf("the paging response must draw a fresh 5G-TMSI, got %v (old %v, previously new %v)", fresh, f.oldTmsi, f.newTmsi)
	}

	if f.ue.OldTmsi() != f.newTmsi {
		t.Fatalf("the staged old 5G-TMSI = %v, want the GUTI the UE presented %v", f.ue.OldTmsi(), f.newTmsi)
	}

	if sent := f.lastConfigurationUpdateGUTI(t); sent != fresh {
		t.Fatalf("Configuration Update Command carries 5G-TMSI %v, want the fresh %v", sent, fresh)
	}
}

func TestPagingResponseWithTheOldGUTIKeepsTheReallocationPending(t *testing.T) {
	f := newPendingReallocFixture(t)

	HandleGmmMessage(t.Context(), f.amf, f.ue, uint8(fgs.MsgServiceRequest), f.pagingResponse(t, f.oldTmsi), true, false)

	if f.ue.OldTmsi() != f.oldTmsi || f.ue.TmsiForTest() != f.newTmsi {
		t.Fatalf("a message citing the old 5G-GUTI changed the reallocation: old=%v new=%v, want old=%v new=%v",
			f.ue.OldTmsi(), f.ue.TmsiForTest(), f.oldTmsi, f.newTmsi)
	}

	if !f.amf.TmsiInUseForTest(f.oldTmsi) || !f.amf.TmsiInUseForTest(f.newTmsi) {
		t.Fatal("both 5G-TMSIs must stay allocated while the reallocation is pending")
	}

	if sent := f.lastConfigurationUpdateGUTI(t); sent != f.newTmsi {
		t.Fatalf("Configuration Update Command carries 5G-TMSI %v, want the staged %v", sent, f.newTmsi)
	}
}

func TestUnverifiedMessageWithTheNewGUTIKeepsTheReallocationPending(t *testing.T) {
	f := newPendingReallocFixture(t)

	HandleGmmMessage(t.Context(), f.amf, f.ue, uint8(fgs.MsgServiceRequest), f.pagingResponse(t, f.newTmsi), false, false)

	if f.ue.OldTmsi() != f.oldTmsi || f.ue.TmsiForTest() != f.newTmsi {
		t.Fatalf("an unverified message changed the reallocation: old=%v new=%v, want old=%v new=%v",
			f.ue.OldTmsi(), f.ue.TmsiForTest(), f.oldTmsi, f.newTmsi)
	}

	if !f.amf.TmsiInUseForTest(f.oldTmsi) {
		t.Fatal("an unverified message freed the old 5G-TMSI")
	}
}

func TestHandleNAS_OnlyAVerifiedMACCommitsTheReallocation(t *testing.T) {
	for _, tc := range []struct {
		name       string
		tamper     bool
		wantCommit bool
	}{
		{"verified", false, true},
		{"mac failure", true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newPendingReallocFixture(t)
			f.ue.SetIntegrityAlgForTest(nas.IntegrityAES)
			f.ue.Conn().MarkSecureExchangeEstablished()

			inner := encSR(t, &fgs.ServiceRequest{
				ServiceType:    fgs.ServiceTypeSignalling,
				NgKSI:          nas.KeySetIdentifier{Value: 1},
				MobileIdentity: f.stmsiFor(t, f.newTmsi),
			})

			pdu := wrapIntegrityProtected(t, f.ue, inner, uint8(f.ue.ULCount()))
			if tc.tamper {
				pdu[2] ^= 0xff
			}

			HandleNAS(t.Context(), f.amf, f.ue.Conn(), pdu)

			committed := f.ue.OldTmsi() == etsi.InvalidTMSI && !f.amf.TmsiInUseForTest(f.oldTmsi)
			if committed != tc.wantCommit {
				t.Fatalf("committed = %v, want %v (old=%v new=%v)", committed, tc.wantCommit, f.ue.OldTmsi(), f.ue.TmsiForTest())
			}

			if f.ue.TmsiForTest() != f.newTmsi {
				t.Fatalf("current 5G-TMSI = %v, want %v", f.ue.TmsiForTest(), f.newTmsi)
			}
		})
	}
}

func mustGuami(t *testing.T, a *amf.AMF) *models.Guami {
	t.Helper()

	info, err := a.OperatorInfo(t.Context())
	if err != nil {
		t.Fatalf("OperatorInfo: %v", err)
	}

	return info.Guami
}

func mustGutiFor(t *testing.T, a *amf.AMF, tmsi etsi.TMSI) etsi.GUTI5G {
	t.Helper()

	guami := mustGuami(t, a)

	guti, err := etsi.NewGUTI5G(guami.PlmnID.Mcc, guami.PlmnID.Mnc, guami.AmfID, tmsi)
	if err != nil {
		t.Fatalf("NewGUTI5G: %v", err)
	}

	return guti
}

func (f pendingReallocFixture) gutiIdentity(t *testing.T, tmsi etsi.TMSI) fgs.MobileIdentity {
	t.Helper()

	id, err := mustGutiFor(t, f.amf, tmsi).MobileIdentity()
	if err != nil {
		t.Fatalf("MobileIdentity: %v", err)
	}

	return id
}

func (f pendingReallocFixture) committed() bool {
	return f.ue.OldTmsi() != f.oldTmsi && !f.amf.TmsiInUseForTest(f.oldTmsi)
}

func (f pendingReallocFixture) pending() bool {
	return f.ue.OldTmsi() == f.oldTmsi && f.ue.TmsiForTest() == f.newTmsi && f.amf.TmsiInUseForTest(f.oldTmsi)
}

func (f pendingReallocFixture) protect(t *testing.T, msg interface{ MarshalBinary() ([]byte, error) }) []byte {
	t.Helper()

	f.ue.SetIntegrityAlgForTest(nas.IntegrityAES)
	f.ue.Conn().MarkSecureExchangeEstablished()

	inner, err := msg.MarshalBinary()
	if err != nil {
		t.Fatalf("encode uplink: %v", err)
	}

	return wrapIntegrityProtected(t, f.ue, inner, uint8(f.ue.ULCount()))
}

func (f pendingReallocFixture) lastRegistrationAcceptGUTI(t *testing.T) etsi.TMSI {
	t.Helper()

	sent := f.ngapSender.SentDownlinkNASTransport
	if len(sent) == 0 {
		t.Fatal("no downlink NAS transport sent")
	}

	plain := decipherGmmCount(t, f.ue, sent[len(sent)-1].NASPDU, uint32(f.ue.DLCountForTest())-1, uint8(fgs.MsgRegistrationAccept))

	accept, err := fgs.ParseRegistrationAccept(plain)
	if err != nil {
		t.Fatalf("decode Registration Accept: %v", err)
	}

	if accept.GUTI == nil {
		t.Fatal("Registration Accept carries no 5G-GUTI")
	}

	guti, err := etsi.NewGUTI5GFromNAS(*accept.GUTI)
	if err != nil {
		t.Fatalf("NewGUTI5GFromNAS: %v", err)
	}

	return guti.Tmsi
}

func TestRegistrationRequestWithTheNewGUTIInvalidatesTheOld(t *testing.T) {
	for _, tc := range []struct {
		name       string
		tamper     bool
		wantCommit bool
	}{
		{"verified", false, true},
		{"mac failure", true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newPendingReallocFixture(t)

			pdu := f.protect(t, &fgs.RegistrationRequest{
				RegistrationType:     fgs.RegistrationTypePeriodicUpdating,
				FOR:                  true,
				NgKSI:                nas.KeySetIdentifier{Value: 1},
				MobileIdentity:       f.gutiIdentity(t, f.newTmsi),
				UESecurityCapability: &fgs.UESecurityCapability{EA: 0xc0, IA: 0xc0},
			})
			if tc.tamper {
				pdu[2] ^= 0xff
			}

			HandleNAS(t.Context(), f.amf, f.ue.Conn(), pdu)

			if tc.wantCommit {
				if !f.committed() {
					t.Fatalf("the reallocation is still pending: old=%v new=%v", f.ue.OldTmsi(), f.ue.TmsiForTest())
				}

				return
			}

			if !f.pending() {
				t.Fatalf("an unverified Registration Request changed the reallocation: old=%v new=%v", f.ue.OldTmsi(), f.ue.TmsiForTest())
			}
		})
	}
}

func TestRegistrationRequestWithTheNewAdditionalGUTIInvalidatesTheOld(t *testing.T) {
	f := newPendingReallocFixture(t)

	additional := f.gutiIdentity(t, f.newTmsi)

	mapped := f.gutiIdentity(t, f.oldTmsi)
	mapped.GUTI.AMFRegionID ^= 0xff

	HandleNAS(t.Context(), f.amf, f.ue.Conn(), f.protect(t, &fgs.RegistrationRequest{
		RegistrationType:     fgs.RegistrationTypePeriodicUpdating,
		FOR:                  true,
		NgKSI:                nas.KeySetIdentifier{Value: 1},
		MobileIdentity:       mapped,
		UESecurityCapability: &fgs.UESecurityCapability{EA: 0xc0, IA: 0xc0},
		AdditionalGUTI:       &additional,
	}))

	if !f.committed() {
		t.Fatalf("the reallocation is still pending: old=%v new=%v", f.ue.OldTmsi(), f.ue.TmsiForTest())
	}
}

func TestRegistrationRequestMovingFromEPCIgnoresTheMappedGUTI(t *testing.T) {
	for _, tc := range []struct {
		name       string
		additional bool
	}{
		{"mapped only", false},
		{"mapped and additional", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newPendingReallocFixture(t)

			req := &fgs.RegistrationRequest{
				RegistrationType: fgs.RegistrationTypeMobilityUpdating,
				MobileIdentity:   f.gutiIdentity(t, f.newTmsi),
				UEStatus:         &fgs.UEStatus{S1ModeReg: true},
			}

			if tc.additional {
				other := f.gutiIdentity(t, f.oldTmsi)
				req.AdditionalGUTI = &other
			}

			confirmGUTIRealloc(t.Context(), f.amf, f.ue, req)

			if !f.pending() {
				t.Fatalf("a 5G-GUTI mapped from a 4G-GUTI changed the reallocation: old=%v new=%v", f.ue.OldTmsi(), f.ue.TmsiForTest())
			}
		})
	}
}

func TestDeregistrationRequestWithTheNewGUTIInvalidatesTheOld(t *testing.T) {
	f := newPendingReallocFixture(t)

	HandleNAS(t.Context(), f.amf, f.ue.Conn(), f.protect(t, &fgs.DeregistrationRequestUEOriginating{
		AccessType:     fgs.AccessType3GPP,
		NgKSI:          nas.KeySetIdentifier{Value: 1},
		MobileIdentity: f.gutiIdentity(t, f.newTmsi),
	}))

	if !f.committed() {
		t.Fatalf("the reallocation is still pending: old=%v new=%v", f.ue.OldTmsi(), f.ue.TmsiForTest())
	}
}

func TestSecurityModeCompleteContainerWithTheNewGUTIReallocatesAFreshOne(t *testing.T) {
	for _, tc := range []struct {
		name     string
		verified bool
	}{
		{"verified", true},
		{"unverified", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newPendingReallocFixture(t)

			f.ue.SetSupiForTest(mustSUPIFromPrefixed("imsi-001019756139935"))

			if err := f.amf.CommitUEIdentity(t.Context(), f.ue, amf.MintAuthProofForRegistrationCommit()); err != nil {
				t.Fatalf("CommitUEIdentity: %v", err)
			}

			f.ue.ForceRegStepForTest(amf.RegStepSecurityMode)
			f.ue.Conn().RegistrationType5GS = fgs.RegistrationTypeInitial

			container, err := buildRegReqBytes(uint8(fgs.RegistrationTypeInitial), f.gutiIdentity(t, f.newTmsi), &fgs.UESecurityCapability{EA: 0xc0, IA: 0xc0}, 0, nil, 0, 1)
			if err != nil {
				t.Fatalf("encode Registration Request: %v", err)
			}

			handleSecurityModeComplete(t.Context(), f.amf, f.ue, &fgs.SecurityModeComplete{NASMessageContainer: container}, tc.verified)

			if !tc.verified {
				if !f.pending() {
					t.Fatalf("an unverified container changed the reallocation: old=%v new=%v", f.ue.OldTmsi(), f.ue.TmsiForTest())
				}

				return
			}

			if f.amf.TmsiInUseForTest(f.oldTmsi) {
				t.Fatal("the old 5G-TMSI is still allocated after the UE used the new 5G-GUTI")
			}

			fresh := f.ue.TmsiForTest()
			if fresh == f.newTmsi || fresh == f.oldTmsi {
				t.Fatalf("the registration must draw a fresh 5G-TMSI, got %v (old %v, previously new %v)", fresh, f.oldTmsi, f.newTmsi)
			}

			if f.ue.OldTmsi() != f.newTmsi {
				t.Fatalf("the staged old 5G-TMSI = %v, want the GUTI the UE presented %v", f.ue.OldTmsi(), f.newTmsi)
			}

			if sent := f.lastRegistrationAcceptGUTI(t); sent != fresh {
				t.Fatalf("Registration Accept carries 5G-TMSI %v, want the fresh %v", sent, fresh)
			}
		})
	}
}
