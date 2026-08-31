// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"sync"
	"testing"

	"github.com/ellanetworks/core/internal/epskeys"
	"github.com/ellanetworks/core/internal/mme"
	"github.com/ellanetworks/core/internal/nasreply"
	"github.com/ellanetworks/core/internal/sctp"
	"github.com/ellanetworks/core/internal/udm"
	"github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/nas/eps"
	"github.com/ellanetworks/core/s1ap"
)

type captureConn struct {
	mu   sync.Mutex
	sent [][]byte
}

func (c *captureConn) WriteMsg(b []byte, _ *sctp.SndRcvInfo) (int, error) {
	c.mu.Lock()
	c.sent = append(c.sent, append([]byte(nil), b...))
	c.mu.Unlock()

	return len(b), nil
}

func (c *captureConn) count() int {
	c.mu.Lock()
	defer c.mu.Unlock()

	return len(c.sent)
}

func decodeDownlinkNAS(t *testing.T, pdu []byte) []byte {
	t.Helper()

	msg, err := s1ap.Unmarshal(pdu)
	if err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	im, ok := msg.(*s1ap.InitiatingMessage)
	if !ok || im.ProcedureCode != s1ap.ProcDownlinkNASTransport {
		t.Fatalf("expected Downlink NAS Transport, got %T", msg)
	}

	dl, err := s1ap.ParseDownlinkNASTransport(im.Value)
	if err != nil {
		t.Fatalf("parse downlink: %v", err)
	}

	return []byte(dl.NASPDU)
}

func decodeProtectedDownlink(t *testing.T, ue *mme.UeContext, pdu []byte) []byte {
	t.Helper()

	dl := decodeDownlinkNAS(t, pdu)

	plain, err := unprotected(eps.Unprotect(dl, nas.MakeCount(0, dl[5]), nas.DirectionDownlink,
		mustSecurityContext(t, ue.EIA(), ue.EEA(), ue.KnasIntForTest(), ue.KnasEncForTest())))
	if err != nil {
		t.Fatalf("unprotect downlink: %v", err)
	}

	return plain
}

// TS 24.301 §4.4.4.3
func TestAttachRecoveryAfterMMERestart(t *testing.T) {
	m := newTestMME(t)
	cc := &captureConn{}
	ue := newAttachUe(m, cc, 7)

	esm, err := (&eps.PDNConnectivityRequest{PTI: 1, RequestType: 1, PDNType: 1}).MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}

	attach := &eps.AttachRequest{
		EPSAttachType:       eps.AttachTypeCombined,
		NASKeySetIdentifier: nas.KeySetIdentifier{Value: 7},
		EPSMobileIdentity: eps.GUTIIdentity(eps.GUTI{
			PLMN: nas.PLMN{MCC: "999", MNC: "01"}, MMEGroupID: 1, MMECode: 1, TMSI: [4]byte{0x00, 0x00, 0x00, 0x02},
		}),
		UENetworkCapability: eps.UENetworkCapability{EEA: 0xf0, EIA: 0x70},
		ESMMessageContainer: esm,
	}

	attachBytes, err := attach.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}

	pdu := append([]byte{0x17, 0xde, 0xad, 0xbe, 0xef, 0x04}, attachBytes...)

	HandleNAS(context.Background(), m, ue.Conn(), pdu)

	if len(cc.sent) != 1 {
		t.Fatalf("expected one downlink (Identity Request), got %d", len(cc.sent))
	}

	if _, err := eps.ParseIdentityRequest(decodeDownlinkNAS(t, cc.sent[0])); err != nil {
		t.Fatalf("expected an Identity Request, got: %v", err)
	}
}

// TS 24.301 §4.4.4.3
func TestIdentityResponseRecoveryAfterMMERestart(t *testing.T) {
	m := newTestMME(t)
	cc := &captureConn{}
	ue := newAttachUe(m, cc, 8)
	ue.TransitionTo(mme.EMMRegistrationInitiated)

	idResp, err := (&eps.IdentityResponse{MobileIdentity: eps.MobileIMSI(eps.IMSI(testSubscriber.IMSI))}).MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}

	pdu := append([]byte{0x17, 0xde, 0xad, 0xbe, 0xef, 0x21}, idResp...)

	HandleNAS(context.Background(), m, ue.Conn(), pdu)

	if len(cc.sent) != 1 {
		t.Fatalf("expected one downlink (Authentication Request), got %d", len(cc.sent))
	}

	if mt, err := eps.PeekMessageType(decodeDownlinkNAS(t, cc.sent[0])); err != nil || mt != eps.MsgAuthenticationRequest {
		t.Fatalf("expected Authentication Request, got mt=%#x err=%v", mt, err)
	}

	if ue.IMSI() != testSubscriber.IMSI {
		t.Fatalf("ue.imsi = %q, want %q", ue.IMSI(), testSubscriber.IMSI)
	}
}

func nativeGUTIAttach(t *testing.T, m *mme.MME, ue *mme.UeContext) []byte {
	t.Helper()

	plmn, err := m.OperatorPLMN(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	group, code, err := m.MmeIdentity(t.Context())
	if err != nil {
		t.Fatal(err)
	}

	guti, err := m.ReallocateGUTI(t.Context(), ue, plmn, group, code)
	if err != nil {
		t.Fatal(err)
	}

	esm, err := (&eps.PDNConnectivityRequest{PTI: 1, RequestType: 1, PDNType: 1}).MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}

	attach := &eps.AttachRequest{
		EPSAttachType:       eps.AttachTypeCombined,
		NASKeySetIdentifier: nas.KeySetIdentifier{Value: 0},
		EPSMobileIdentity:   guti,
		UENetworkCapability: eps.UENetworkCapability{EEA: 0xf0, EIA: 0x70},
		ESMMessageContainer: esm,
	}

	attachBytes, err := attach.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}

	wire, err := eps.Protect(attachBytes, eps.SHTIntegrityProtected, nas.MakeCount(0, 0), nas.DirectionUplink, mustSecurityContext(t, ue.EIA(), ue.EEA(), ue.KnasIntForTest(), ue.KnasEncForTest()))
	if err != nil {
		t.Fatal(err)
	}

	return wire
}

// TS 23.401 §5.3.2.1
func TestAttachReusesContextForNativeGUTI(t *testing.T) {
	m := newTestMME(t)
	existing, _ := securedUE(t, m)
	existing.SetDLCountForTest(7)

	wire := nativeGUTIAttach(t, m, existing)

	cc := &captureConn{}
	fresh := newAttachUe(m, cc, 9)
	HandleNAS(context.Background(), m, fresh.Conn(), wire)

	if got, ok := m.LookupUeByIMSI(existing.IMSI()); !ok || got != existing {
		t.Fatal("held context not reused in place")
	}

	if fresh.Conn() != nil {
		t.Fatal("transient context not discarded after context reuse")
	}

	if existing.Conn() == nil || existing.Conn().ConnForTest() != cc {
		t.Fatal("held context not rebound to the returning UE's connection")
	}

	if existing.Conn().AuthVector != nil {
		t.Fatal("authentication was not skipped on a valid native GUTI")
	}

	if existing.ULCount() != 1 {
		t.Fatalf("uplink NAS COUNT = %d, want 1 (continued past the Attach)", existing.ULCount())
	}

	if existing.DLCountForTest() < 7 {
		t.Fatalf("downlink NAS COUNT reset to %d on context reuse (keystream reuse)", existing.DLCountForTest())
	}

	if cc.count() != 1 {
		t.Fatalf("expected one downlink (Initial Context Setup), got %d", cc.count())
	}

	parseInitialContextSetup(t, cc.sent[0])
}

func TestAttachReusesContextForNativeGUTI_ReleasesOldBearers(t *testing.T) {
	m := newTestMME(t)
	existing, _ := securedUE(t, m)

	if existing.Pdns == nil {
		existing.Pdns = map[uint8]*mme.PdnConnection{}
	}

	existing.Pdns[5] = &mme.PdnConnection{Ebi: 5}

	wire := nativeGUTIAttach(t, m, existing)

	cc := &captureConn{}
	fresh := newAttachUe(m, cc, 9)
	HandleNAS(context.Background(), m, fresh.Conn(), wire)

	if !m.Session.(*fakeSessionManager).released {
		t.Fatal("old EPS bearer not released on native-GUTI re-attach (case f: old bearers must be deleted)")
	}
}

// TS 24.301 §5.5.1.2.7
func TestAttachKeepsOldGUTIResolvableUntilComplete(t *testing.T) {
	m := newTestMME(t)
	existing, _ := securedUE(t, m)

	wire := nativeGUTIAttach(t, m, existing)
	presented := existing.TmsiForTest()

	cc := &captureConn{}
	fresh := newAttachUe(m, cc, 9)
	HandleNAS(context.Background(), m, fresh.Conn(), wire)

	newMTMSI := existing.TmsiForTest()
	if newMTMSI == presented {
		t.Fatal("attach did not reallocate the GUTI")
	}

	if got, ok := m.LookupUeByMTMSI(presented); !ok || got != existing {
		t.Fatal("old M-TMSI must stay resolvable until Attach Complete")
	}

	if got, ok := m.LookupUeByMTMSI(newMTMSI); !ok || got != existing {
		t.Fatal("new M-TMSI not resolvable")
	}

	handleAttachComplete(context.Background(), m, existing, existing.Conn(), &eps.AttachComplete{})

	if !existing.OldTmsiUnsetForTest() {
		t.Fatal("GUTI reallocation not committed after Attach Complete")
	}

	if _, ok := m.LookupUeByMTMSI(presented); ok {
		t.Fatal("old M-TMSI still resolvable after Attach Complete")
	}

	if got, ok := m.LookupUeByMTMSI(newMTMSI); !ok || got != existing {
		t.Fatal("new M-TMSI lost after Attach Complete")
	}
}

func TestAttachNativeGUTIBadMACFallsBackToAuth(t *testing.T) {
	m := newTestMME(t)
	existing, _ := securedUE(t, m)
	oldID := existing.Conn().MMEUES1APID

	wire := nativeGUTIAttach(t, m, existing)
	wire[1] ^= 0xff

	cc := &captureConn{}
	fresh := newAttachUe(m, cc, 9)
	HandleNAS(context.Background(), m, fresh.Conn(), wire)

	if _, ok := m.LookupUe(oldID); !ok {
		t.Fatal("context was removed despite a MAC mismatch")
	}

	if _, err := eps.ParseIdentityRequest(decodeDownlinkNAS(t, cc.sent[0])); err != nil {
		t.Fatalf("expected a fallback Identity Request, got: %v", err)
	}
}

func TestAttachNativeGUTIReplayDoesNotRemoveContext(t *testing.T) {
	m := newTestMME(t)
	existing, _ := securedUE(t, m)
	oldID := existing.Conn().MMEUES1APID

	wire := nativeGUTIAttach(t, m, existing)

	existing.SetULCountForTest(50)

	cc := &captureConn{}
	attacker := newAttachUe(m, cc, 9)
	HandleNAS(context.Background(), m, attacker.Conn(), wire)

	if _, ok := m.LookupUe(oldID); !ok {
		t.Fatal("live context removed by a replayed stale-count Attach")
	}
}

func TestAttachAuthenticationAndSecurityMode(t *testing.T) {
	m := newTestMME(t)
	cc := &captureConn{}
	ue := newAttachUe(m, cc, 7)

	esm, err := (&eps.PDNConnectivityRequest{PTI: 1, RequestType: 1, PDNType: 1}).MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}

	attach := &eps.AttachRequest{
		EPSAttachType:       eps.AttachTypeEPS,
		NASKeySetIdentifier: nas.KeySetIdentifier{Value: 7},
		EPSMobileIdentity:   eps.IMSIIdentity(eps.IMSI(testSubscriber.IMSI)),
		UENetworkCapability: eps.UENetworkCapability{EEA: 0xf0, EIA: 0x70},
		ESMMessageContainer: esm,
	}

	attachBytes, err := attach.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}

	HandleNAS(context.Background(), m, ue.Conn(), attachBytes)

	if len(cc.sent) != 1 {
		t.Fatalf("expected 1 downlink (Authentication Request), got %d", len(cc.sent))
	}

	authReq, err := eps.ParseAuthenticationRequest(decodeDownlinkNAS(t, cc.sent[0]))
	if err != nil {
		t.Fatal(err)
	}

	if authReq.NASKeySetIdentifier.Value != 1 {
		t.Fatalf("Authentication Request eKSI = %d, want 1 (cycled from stored 0)", authReq.NASKeySetIdentifier.Value)
	}

	res := make([]byte, 8)
	if err := udm.F2345(testSubscriber.OPc[:], testSubscriber.K[:], authReq.RAND[:],
		res, make([]byte, 16), make([]byte, 16), make([]byte, 6), nil); err != nil {
		t.Fatal(err)
	}

	kasme := ue.Conn().AuthVector.KASME

	authResp, err := (&eps.AuthenticationResponse{RES: res}).MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}

	HandleNAS(context.Background(), m, ue.Conn(), authResp)

	if ue.Conn().AuthVector != nil {
		t.Fatal("AuthVector must be cleared on authentication success")
	}

	if ue.Conn().ResyncTried() {
		t.Fatal("resyncTried must be reset on authentication success")
	}

	if len(cc.sent) != 2 {
		t.Fatalf("expected Security Mode Command, got %d downlinks", len(cc.sent))
	}

	smcWire := decodeDownlinkNAS(t, cc.sent[1])

	knasEnc, err := epskeys.DeriveKNASEnc(kasme, 2)
	if err != nil {
		t.Fatal(err)
	}

	knasInt, err := epskeys.DeriveKNASInt(kasme, 2)
	if err != nil {
		t.Fatal(err)
	}

	smcPlain, err := unprotected(eps.Unprotect(smcWire, nas.MakeCount(0, smcWire[5]), nas.DirectionDownlink, mustSecurityContext(t, nas.IntegrityAES, nas.CipheringAES, knasInt, knasEnc)))
	if err != nil {
		t.Fatalf("Security Mode Command failed integrity check: %v", err)
	}

	smc, err := eps.ParseSecurityModeCommand(smcPlain)
	if err != nil {
		t.Fatal(err)
	}

	if smc.CipheringAlgorithm != 2 || smc.IntegrityAlgorithm != 2 {
		t.Fatalf("SMC algorithms eea=%d eia=%d, want 2/2", smc.CipheringAlgorithm, smc.IntegrityAlgorithm)
	}

	if smc.NASKeySetIdentifier != authReq.NASKeySetIdentifier {
		t.Fatalf("SMC eKSI = %v, want %v (same as Authentication Request)", smc.NASKeySetIdentifier, authReq.NASKeySetIdentifier)
	}

	wantHash := sha256.Sum256(attachBytes)
	if !bytes.Equal(smc.HASHMME, wantHash[:8]) {
		t.Fatalf("SMC mme.HashMME = %x, want %x", smc.HASHMME, wantHash[:8])
	}

	imeisv := eps.MobileIMEISV("0350638214365870")

	smCompletePlain, err := (&eps.SecurityModeComplete{IMEISV: &imeisv}).MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}

	smCompleteWire, err := eps.Protect(smCompletePlain, eps.SHTIntegrityProtectedCipheredNewContext, nas.MakeCount(0, 0), nas.DirectionUplink, mustSecurityContext(t, nas.IntegrityAES, nas.CipheringAES, knasInt, knasEnc))
	if err != nil {
		t.Fatal(err)
	}

	HandleNAS(context.Background(), m, ue.Conn(), smCompleteWire)

	if !ue.Secured() {
		t.Fatal("NAS security context not established after Security Mode Complete")
	}

	if ue.Imei.IMEI() != "035063821436588" {
		t.Fatalf("IMEI from IMEISV = %q, want 035063821436588", ue.Imei.IMEI())
	}

	if ue.NCCForTest() != 1 || ue.NHForTest() == ([32]byte{}) {
		t.Fatalf("NH chain not seeded: ncc=%d nh-zero=%v", ue.NCCForTest(), ue.NHForTest() == ([32]byte{}))
	}

	if len(cc.sent) != 3 {
		t.Fatalf("expected Initial Context Setup Request, got %d downlinks", len(cc.sent))
	}

	ics := parseInitialContextSetup(t, cc.sent[2])

	if ics.MMEUES1APID != ue.Conn().MMEUES1APID || ics.ENBUES1APID != 7 || len(ics.ERABToBeSetup) != 1 {
		t.Fatalf("unexpected Initial Context Setup Request: %+v", ics)
	}

	wantKeNB, err := epskeys.DeriveKeNB(kasme, ue.ULCount()-1)
	if err != nil {
		t.Fatal(err)
	}

	if [32]byte(ics.SecurityKey) != wantKeNB {
		t.Fatalf("K_eNB mismatch in Initial Context Setup Request")
	}

	erab := ics.ERABToBeSetup[0]
	if erab.ERABID != s1ap.ERABID(mme.DefaultERABID) || erab.QoS.QCI != s1ap.QCI(9) ||
		erab.GTPTEID != s1ap.GTPTEID(testSGWFTEID.TEID) {
		t.Fatalf("unexpected E-RAB: %+v", erab)
	}

	acceptWire := []byte(erab.NASPDU)

	acceptPlain, err := unprotected(eps.Unprotect(acceptWire, nas.MakeCount(0, acceptWire[5]), nas.DirectionDownlink, mustSecurityContext(t, nas.IntegrityAES, nas.CipheringAES, knasInt, knasEnc)))
	if err != nil {
		t.Fatalf("Attach Accept failed integrity check: %v", err)
	}

	accept, err := eps.ParseAttachAccept(acceptPlain)
	if err != nil {
		t.Fatal(err)
	}

	if accept.GUTI == nil || accept.GUTI.GUTI == nil {
		t.Fatal("Attach Accept did not assign a GUTI")
	}

	gutiID := *accept.GUTI.GUTI

	group, code, err := m.MmeIdentity(t.Context())
	if err != nil {
		t.Fatal(err)
	}

	if gutiID.PLMN.MCC != "001" || gutiID.PLMN.MNC != "01" ||
		gutiID.MMEGroupID != group || gutiID.MMECode != code {
		t.Fatalf("GUTI = %+v, want the node's GUMMEI %d/%d on PLMN 001-01", gutiID, group, code)
	}

	if _, ok := m.LookupUeByMTMSI(binary.BigEndian.Uint32(gutiID.TMSI[:])); !ok {
		t.Fatal("UE not indexed by its assigned M-TMSI")
	}

	activate, err := eps.ParseActivateDefaultEPSBearerContextRequest(accept.ESMMessageContainer)
	if err != nil {
		t.Fatal(err)
	}

	pdn := activate.PDNAddress
	if pdn.IPv4 != testUEIP.As4() {
		t.Fatalf("assigned UE IP = %v, want %v", pdn.IPv4, testUEIP.As4())
	}

	completePlain, err := (&eps.AttachComplete{ESMMessageContainer: []byte{uint8(activate.EPSBearerIdentity)<<4 | 0x02, uint8(activate.PTI), 0xc2}}).MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}

	completeWire, err := eps.Protect(completePlain, eps.SHTIntegrityProtectedCiphered, nas.MakeCount(0, uint8(ue.ULCount())), nas.DirectionUplink, mustSecurityContext(t, nas.IntegrityAES, nas.CipheringAES, knasInt, knasEnc))
	if err != nil {
		t.Fatal(err)
	}

	HandleNAS(context.Background(), m, ue.Conn(), completeWire)

	if ue.EMMState() != mme.EMMRegistered {
		t.Fatal("UE not EMM-REGISTERED after Attach Complete")
	}
}

// TS 24.301 §5.4.3.5
func TestSecurityModeRejectReleasesUE(t *testing.T) {
	m := newTestMME(t)
	cc := &captureConn{}
	ue := newAttachUe(m, cc, 7)

	if !m.TryClaimKeyChain(ue) {
		t.Fatal("could not claim the security mode exchange")
	}

	ue.TransitionTo(mme.EMMRegistrationInitiated)
	ue.AdvanceRegStep(mme.RegStepSecurityMode)

	plain, err := (&eps.SecurityModeReject{Cause: 23}).MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}

	HandleNAS(context.Background(), m, ue.Conn(), plain)

	if !ue.ReleasingForTest() {
		t.Fatal("UE not released after Security Mode Reject")
	}

	if cc.count() != 1 {
		t.Fatalf("expected one S1AP message (UE Context Release Command), got %d", cc.count())
	}

	parseUEContextReleaseCommand(t, cc.sent[0])
}

// TS 24.301 §4.4.4.3
func TestIdentityResponseIgnoredAfterAuthStarted(t *testing.T) {
	m := newTestMME(t)
	cc := &captureConn{}
	ue := newAttachUe(m, cc, 7)
	ue.SetIMSIForTest(testSubscriber.IMSI)
	ue.Conn().AuthVector = &udm.EPSAV{}

	resp := &eps.IdentityResponse{MobileIdentity: eps.MobileIMSI("123456789")}

	handleIdentityResponse(context.Background(), m, ue, ue.Conn(), resp)

	if ue.IMSI() != testSubscriber.IMSI {
		t.Fatalf("out-of-order Identity Response overwrote the IMSI: got %q, want %q", ue.IMSI(), testSubscriber.IMSI)
	}
}

// TS 24.301 §4.4.4.3
func TestSecurityModeRejectIgnoredOutsideExchange(t *testing.T) {
	m := newTestMME(t)
	cc := &captureConn{}
	ue := newAttachUe(m, cc, 7)

	plain, err := (&eps.SecurityModeReject{Cause: 23}).MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}

	HandleNAS(context.Background(), m, ue.Conn(), plain)

	if ue.Conn() == nil || ue.ReleasingForTest() {
		t.Fatal("an out-of-order Security Mode Reject must not release the UE")
	}

	if cc.count() != 0 {
		t.Fatalf("expected no S1AP message for an ignored reject, got %d", cc.count())
	}
}

func TestSecurityModeCompleteRecoversReplayedAttach(t *testing.T) {
	m := newTestMME(t)
	ue, _ := securedUE(t, m)

	if !m.TryClaimKeyChain(ue) {
		t.Fatal("could not claim the security mode exchange")
	}

	ue.ForceRegStepForTest(mme.RegStepSecurityMode)

	ue.CombinedAttach = true
	ue.RequestedAPN = "tampered-apn"

	esm, err := (&eps.PDNConnectivityRequest{PTI: 1, RequestType: 1, PDNType: 1}).MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}

	genuine, err := (&eps.AttachRequest{
		EPSAttachType:       eps.AttachTypeEPS,
		NASKeySetIdentifier: nas.KeySetIdentifier{Value: 7},
		EPSMobileIdentity:   eps.IMSIIdentity(eps.IMSI(testSubscriber.IMSI)),
		UENetworkCapability: eps.UENetworkCapability{EEA: 0xf0, EIA: 0x70},
		ESMMessageContainer: esm,
	}).MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}

	smc := &eps.SecurityModeComplete{ReplayedNASMessageContainer: genuine}

	handleSecurityModeComplete(context.Background(), m, ue, ue.Conn(), smc)

	if ue.CombinedAttach {
		t.Fatal("MME must re-ingest the genuine (non-combined) Attach from the replayed NAS message container")
	}

	if ue.RequestedAPN != "" {
		t.Fatalf("genuine Attach carried no APN; re-ingest must reset RequestedAPN, got %q", ue.RequestedAPN)
	}
}

func parseInitialContextSetup(t *testing.T, pdu []byte) *s1ap.InitialContextSetupRequest {
	t.Helper()

	msg, err := s1ap.Unmarshal(pdu)
	if err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	im, ok := msg.(*s1ap.InitiatingMessage)
	if !ok || im.ProcedureCode != s1ap.ProcInitialContextSetup {
		t.Fatalf("expected Initial Context Setup Request, got %T", msg)
	}

	ics, err := s1ap.ParseInitialContextSetupRequest(im.Value)
	if err != nil {
		t.Fatalf("parse ICS: %v", err)
	}

	return ics
}

// TS 24.301 §5.7
func TestDispatchEMM_UnhandledMessageReturnsEMMStatus(t *testing.T) {
	m := newTestMME(t)
	cc := &captureConn{}
	ue := newAttachUe(m, cc, 7)

	plain := []byte{0x07, 0x70}

	d := HandleEmmMessage(context.Background(), m, ue, ue.Conn(), plain, true)

	if d.Action != nasreply.ActionStatus || d.Domain != nasreply.DomainMM || d.Cause != nasreply.CauseMessageTypeNotImplemented {
		t.Fatalf("disposition = %+v, want an EMM STATUS #97 (message type non-existent or not implemented)", d)
	}
}

// TS 24.301 §7.4
func TestDispatchESM_UnhandledMessageReturnsESMStatus(t *testing.T) {
	m := newTestMME(t)
	cc := &captureConn{}
	ue := newAttachUe(m, cc, 7)

	plain := []byte{0x02, 0x00, 0x55}

	d := HandleEmmMessage(context.Background(), m, ue, ue.Conn(), plain, true)

	if d.Action != nasreply.ActionStatus || d.Domain != nasreply.DomainSM || d.Cause != nasreply.CauseMessageTypeNotImplemented {
		t.Fatalf("disposition = %+v, want an ESM STATUS #97 (message type non-existent or not implemented)", d)
	}
}

// TS 24.301 §5.7
func TestDispatchEMM_EMMStatusHandledNoReply(t *testing.T) {
	m := newTestMME(t)
	cc := &captureConn{}
	ue := newAttachUe(m, cc, 7)

	plain, err := (&eps.EMMStatus{Cause: eps.EMMCauseProtocolErrorUnspecified}).MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}

	HandleEmmMessage(context.Background(), m, ue, ue.Conn(), plain, true)

	if cc.count() != 0 {
		t.Fatalf("EMM STATUS must be handled with no reply, got %d downlink(s)", cc.count())
	}
}

// TS 24.301 §5.5.1.2.7
func TestAttachDuplicateIdenticalIEsResendsAccept(t *testing.T) {
	m := newTestMME(t)
	ue, cc := securedUE(t, m)
	ue.ForceRegStepForTest(mme.RegStepContextSetup)

	attach := &eps.AttachRequest{
		EPSAttachType:       eps.AttachTypeEPS,
		NASKeySetIdentifier: nas.KeySetIdentifier{Value: 7},
		EPSMobileIdentity:   eps.IMSIIdentity(eps.IMSI("001010000000001")),
		UENetworkCapability: eps.UENetworkCapability{EEA: 0xf0, EIA: 0x70},
	}

	plain, err := attach.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}

	ue.Conn().AttachRequestPlain = plain
	ue.Conn().AttachAcceptPlain = []byte{0x07, 0x42, 0x01}

	handleAttachRequest(context.Background(), m, ue, ue.Conn(), attach, plain, false)

	if cc.count() != 1 {
		t.Fatalf("expected the Attach Accept resent (one downlink), got %d", cc.count())
	}

	if ue.Conn() == nil || ue.ReleasingForTest() {
		t.Fatal("an identical duplicate Attach Request must not release the UE")
	}

	if ue.RegStep() != mme.RegStepContextSetup {
		t.Fatalf("an identical duplicate must not re-authenticate; RegStep = %s", ue.RegStep())
	}
}

// TS 24.301 §5.5.1.2.7
func TestAttachDuplicatePreAcceptIdenticalIEsIgnored(t *testing.T) {
	m := newTestMME(t)
	ue, cc := securedUE(t, m)
	ue.ForceRegStepForTest(mme.RegStepAuthenticating)

	attach := &eps.AttachRequest{
		EPSAttachType:       eps.AttachTypeEPS,
		NASKeySetIdentifier: nas.KeySetIdentifier{Value: 7},
		EPSMobileIdentity:   eps.IMSIIdentity(eps.IMSI("001010000000001")),
		UENetworkCapability: eps.UENetworkCapability{EEA: 0xf0, EIA: 0x70},
	}

	plain, err := attach.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}

	ue.Conn().AttachRequestPlain = plain

	handleAttachRequest(context.Background(), m, ue, ue.Conn(), attach, plain, false)

	if cc.count() != 0 {
		t.Fatalf("an identical pre-accept duplicate must be ignored (no downlink), got %d", cc.count())
	}

	if ue.RegStep() != mme.RegStepAuthenticating {
		t.Fatalf("an identical pre-accept duplicate must not restart the procedure; RegStep = %s", ue.RegStep())
	}
}

// TS 24.301 §5.5.1.2.7
func TestAttachDuplicatePreAcceptSecurityModeIdenticalIEsIgnored(t *testing.T) {
	m := newTestMME(t)
	ue, cc := securedUE(t, m)
	ue.ForceRegStepForTest(mme.RegStepSecurityMode)

	attach := &eps.AttachRequest{
		EPSAttachType:       eps.AttachTypeEPS,
		NASKeySetIdentifier: nas.KeySetIdentifier{Value: 7},
		EPSMobileIdentity:   eps.IMSIIdentity(eps.IMSI("001010000000001")),
		UENetworkCapability: eps.UENetworkCapability{EEA: 0xf0, EIA: 0x70},
	}

	plain, err := attach.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}

	ue.Conn().AttachRequestPlain = plain

	handleAttachRequest(context.Background(), m, ue, ue.Conn(), attach, plain, false)

	if cc.count() != 0 {
		t.Fatalf("an identical pre-accept duplicate must be ignored (no downlink), got %d", cc.count())
	}

	if ue.RegStep() != mme.RegStepSecurityMode {
		t.Fatalf("an identical pre-accept duplicate must not restart the procedure; RegStep = %s", ue.RegStep())
	}
}

// TS 24.301 §5.5.1.2.7
func TestAttachDuplicateDifferingIEsProgresses(t *testing.T) {
	m := newTestMME(t)
	ue, cc := securedUE(t, m)
	ue.ForceRegStepForTest(mme.RegStepContextSetup)

	attach := &eps.AttachRequest{
		EPSAttachType:       eps.AttachTypeEPS,
		NASKeySetIdentifier: nas.KeySetIdentifier{Value: 7},
		EPSMobileIdentity:   eps.IMSIIdentity(eps.IMSI("001010000000001")),
		UENetworkCapability: eps.UENetworkCapability{EEA: 0xf0, EIA: 0x70},
	}

	plain, err := attach.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}

	ue.Conn().AttachRequestPlain = []byte{0x07, 0x41, 0x99}
	ue.Conn().AttachAcceptPlain = []byte{0x07, 0x42, 0x01}

	handleAttachRequest(context.Background(), m, ue, ue.Conn(), attach, plain, false)

	if ue.RegStep() != mme.RegStepAuthenticating {
		t.Fatalf("a differing duplicate must abort and progress the attach; RegStep = %s", ue.RegStep())
	}

	if cc.count() != 1 {
		t.Fatalf("expected the progressed attach to send an Authentication Request, got %d downlinks", cc.count())
	}

	mt, err := eps.PeekMessageType(decodeDownlinkNAS(t, cc.sent[0]))
	if err != nil {
		t.Fatal(err)
	}

	if mt != eps.MsgAuthenticationRequest {
		t.Fatalf("expected Authentication Request, got message type %#x", mt)
	}
}

// TS 24.301 §5.5.2.3.4
func TestAttachIgnoredDuringNetworkInitiatedDetach(t *testing.T) {
	m := newTestMME(t)
	cc := &captureConn{}
	ue := newAttachUe(m, cc, 7)
	ue.ForceStateForTest(mme.EMMDeregistrationInitiated)

	attach := &eps.AttachRequest{
		EPSAttachType:       eps.AttachTypeEPS,
		NASKeySetIdentifier: nas.KeySetIdentifier{Value: 7},
		EPSMobileIdentity:   eps.IMSIIdentity(eps.IMSI("001010000000001")),
		UENetworkCapability: eps.UENetworkCapability{EEA: 0xf0, EIA: 0x70},
	}

	plain, err := attach.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}

	handleAttachRequest(context.Background(), m, ue, ue.Conn(), attach, plain, false)

	if ue.EMMState() != mme.EMMDeregistrationInitiated {
		t.Fatalf("attach during network-initiated detach must be ignored; state = %s, want EMM-DEREGISTERED-INITIATED", ue.EMMState())
	}

	if cc.count() != 0 {
		t.Fatalf("expected no downlink for an ignored attach, got %d", cc.count())
	}
}

// TS 24.301 §5.5.1.2.4
func TestAuthenticationSuccessMakesTheMMEServeTheUE(t *testing.T) {
	m := newTestMME(t)
	cc := &captureConn{}
	ue := newAttachUe(m, cc, 7)

	esm, err := (&eps.PDNConnectivityRequest{PTI: 1, RequestType: 1, PDNType: 1}).MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}

	attach := &eps.AttachRequest{
		EPSAttachType:       eps.AttachTypeEPS,
		NASKeySetIdentifier: nas.KeySetIdentifier{Value: 7},
		EPSMobileIdentity:   eps.IMSIIdentity(eps.IMSI(testSubscriber.IMSI)),
		UENetworkCapability: eps.UENetworkCapability{EEA: 0xf0, EIA: 0x70},
		ESMMessageContainer: esm,
	}

	attachBytes, err := attach.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}

	HandleNAS(context.Background(), m, ue.Conn(), attachBytes)

	if _, ok := m.LookupUeByIMSI(testSubscriber.IMSI); ok {
		t.Fatal("an unauthenticated attach must not make the MME serve the subscriber")
	}

	authReq, err := eps.ParseAuthenticationRequest(decodeDownlinkNAS(t, cc.sent[0]))
	if err != nil {
		t.Fatal(err)
	}

	res := make([]byte, 8)
	if err := udm.F2345(testSubscriber.OPc[:], testSubscriber.K[:], authReq.RAND[:],
		res, make([]byte, 16), make([]byte, 16), make([]byte, 6), nil); err != nil {
		t.Fatal(err)
	}

	authResp, err := (&eps.AuthenticationResponse{RES: res}).MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}

	HandleNAS(context.Background(), m, ue.Conn(), authResp)

	held, ok := m.LookupUeByIMSI(testSubscriber.IMSI)
	if !ok || held != ue {
		t.Fatalf("LookupUeByIMSI = (%p, %v), want the authenticated context %p: the MME can never reach this UE", held, ok, ue)
	}

	if !m.ServesUeContext(ue) {
		t.Error("the MME does not serve a UE it has just authenticated, so no attach accept can be built for it")
	}
}

func TestAttachReadsOperatorOnce(t *testing.T) {
	m, store := newCountingMME(t)
	cc := &captureConn{}
	ue := newAttachUe(m, cc, 7)

	esm, err := (&eps.PDNConnectivityRequest{PTI: 1, RequestType: 1, PDNType: 1}).MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}

	attach := &eps.AttachRequest{
		EPSAttachType:       eps.AttachTypeEPS,
		NASKeySetIdentifier: nas.KeySetIdentifier{Value: 7},
		EPSMobileIdentity:   eps.IMSIIdentity(eps.IMSI(testSubscriber.IMSI)),
		UENetworkCapability: eps.UENetworkCapability{EEA: 0xf0, EIA: 0x70},
		ESMMessageContainer: esm,
	}

	b, err := attach.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}

	HandleNAS(context.Background(), m, ue.Conn(), b)

	if len(cc.sent) != 1 {
		t.Fatalf("expected one downlink (Authentication Request), got %d", len(cc.sent))
	}

	if mt, err := eps.PeekMessageType(decodeDownlinkNAS(t, cc.sent[0])); err != nil || mt != eps.MsgAuthenticationRequest {
		t.Fatalf("expected Authentication Request, got mt=%#x err=%v", mt, err)
	}

	if got := store.reads.Load(); got != 1 {
		t.Fatalf("attach read the operator row %d times, want 1", got)
	}
}
