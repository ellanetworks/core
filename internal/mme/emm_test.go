// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"context"
	"sync"
	"testing"

	"github.com/ellanetworks/core/internal/amf/sctp"
	"github.com/ellanetworks/core/internal/udm"
	nascommon "github.com/ellanetworks/core/nas/common"
	"github.com/ellanetworks/core/nas/eps"
	"github.com/ellanetworks/core/s1ap"
)

// captureConn records the S1AP messages the MME sends, standing in for an eNB
// SCTP association. WriteMsg is safe for concurrent use so timer-driven
// retransmissions can be exercised under the race detector.
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

// count returns how many messages have been sent. Safe for concurrent use.
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

// TestAttachAuthenticationAndSecurityMode drives the EMM engine through the
// Attach → EPS-AKA → Security Mode exchange, with the test playing the UE.
// TestAttachRecoveryAfterMMERestart reproduces the field case: after the MME
// restarts and loses its in-memory security contexts, the UE returns with an
// integrity-protected combined ATTACH REQUEST carrying the GUTI it still holds.
// The MME cannot verify the MAC (no context), but per TS 24.301 §4.4.4.3 it must
// still process the Attach Request rather than drop it. With a GUTI it cannot
// resolve, the MME recovers by requesting the UE's identity.
func TestAttachRecoveryAfterMMERestart(t *testing.T) {
	m := newTestMME(t)
	cc := &captureConn{}
	ue := m.newUe(cc, 7)

	esm, err := (&eps.PDNConnectivityRequest{ProcedureTransactionIdentity: 1, RequestType: 1, PDNType: 1}).Marshal()
	if err != nil {
		t.Fatal(err)
	}

	attach := &eps.AttachRequest{
		EPSAttachType:       epsAttachTypeCombined,
		NASKeySetIdentifier: 7,
		EPSMobileIdentity: eps.EPSMobileIdentity{
			Type: eps.IdentityGUTI, MCC: "999", MNC: "01", MMEGroupID: 1, MMECode: 1, MTMSI: 2,
		},
		UENetworkCapability: eps.UENetworkCapability{EEA: 0xf0, EIA: 0x70}.Marshal(),
		ESMMessageContainer: esm,
	}

	attachBytes, err := attach.Marshal()
	if err != nil {
		t.Fatal(err)
	}

	// Wrap the plain Attach Request in an integrity-protected envelope (SHT=1) with
	// a MAC the MME cannot reproduce, as the UE does after the MME lost its
	// context: SHT|PD, 4-octet MAC, sequence, then the inner Attach Request.
	nas := append([]byte{0x17, 0xde, 0xad, 0xbe, 0xef, 0x04}, attachBytes...)

	m.handleNAS(ue, nas)

	if len(cc.sent) != 1 {
		t.Fatalf("expected one downlink (Identity Request), got %d", len(cc.sent))
	}

	if _, err := eps.ParseIdentityRequest(decodeDownlinkNAS(t, cc.sent[0])); err != nil {
		t.Fatalf("expected an Identity Request, got: %v", err)
	}
}

// TestIdentityResponseRecoveryAfterMMERestart reproduces the field case where,
// after the MME loses its security contexts on restart, the UE answers the
// MME's IDENTITY REQUEST with an IDENTITY RESPONSE integrity-protected against a
// context the network no longer holds. Per TS 24.301 §4.4.4.3 the MME must still
// process it (the IMSI is in cleartext) and proceed to authentication, instead
// of dropping it and looping on IDENTITY REQUEST.
func TestIdentityResponseRecoveryAfterMMERestart(t *testing.T) {
	m := newTestMME(t)
	cc := &captureConn{}
	ue := m.newUe(cc, 8)

	// Mobile identity for testSubscriber.IMSI (TS 24.008 §10.5.1.4): first digit in
	// the high nibble of octet 1, IMSI type + odd flag in the low nibble, then the
	// remaining digits TBCD-packed.
	mobileID := []byte{0x09, 0x10, 0x10, 0x00, 0x00, 0x00, 0x00, 0x10}
	if got := mobileIdentityDigits(mobileID); got != testSubscriber.IMSI {
		t.Fatalf("test mobile identity decodes to %q, want %q", got, testSubscriber.IMSI)
	}

	idResp, err := (&eps.IdentityResponse{MobileIdentity: mobileID}).Marshal()
	if err != nil {
		t.Fatal(err)
	}

	// Integrity-protected envelope (SHT=1) with a MAC the MME cannot reproduce.
	nas := append([]byte{0x17, 0xde, 0xad, 0xbe, 0xef, 0x21}, idResp...)

	m.handleNAS(ue, nas)

	if len(cc.sent) != 1 {
		t.Fatalf("expected one downlink (Authentication Request), got %d", len(cc.sent))
	}

	if mt, err := eps.PeekMessageType(decodeDownlinkNAS(t, cc.sent[0])); err != nil || mt != eps.MsgAuthenticationRequest {
		t.Fatalf("expected Authentication Request, got mt=%#x err=%v", mt, err)
	}

	if ue.imsi != testSubscriber.IMSI {
		t.Fatalf("ue.imsi = %q, want %q", ue.imsi, testSubscriber.IMSI)
	}
}

// nativeGUTIAttach builds an integrity-protected combined ATTACH REQUEST that
// carries `ue`'s native GUTI, protected with `ue`'s NAS security context — the
// message a returning UE sends when it still holds a context the MME assigned.
func nativeGUTIAttach(t *testing.T, m *MME, ue *UeContext) []byte {
	t.Helper()

	plmn, err := m.operatorPLMN(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	group, code := m.mmeIdentity()
	guti := m.assignGUTI(ue, plmn, group, code)

	esm, err := (&eps.PDNConnectivityRequest{ProcedureTransactionIdentity: 1, RequestType: 1, PDNType: 1}).Marshal()
	if err != nil {
		t.Fatal(err)
	}

	attach := &eps.AttachRequest{
		EPSAttachType:       epsAttachTypeCombined,
		NASKeySetIdentifier: 0,
		EPSMobileIdentity:   guti,
		UENetworkCapability: eps.UENetworkCapability{EEA: 0xf0, EIA: 0x70}.Marshal(),
		ESMMessageContainer: esm,
	}

	attachBytes, err := attach.Marshal()
	if err != nil {
		t.Fatal(err)
	}

	wire, err := eps.Protect(attachBytes, eps.SHTIntegrityProtected, nascommon.NASCount(0, 0),
		nascommon.DirectionUplink, ue.knasInt, ue.knasEnc, nascommon.AESCMACIntegrity{}, nascommon.AESCTRCipher{})
	if err != nil {
		t.Fatal(err)
	}

	return wire
}

// TestAttachReusesContextForNativeGUTI checks that an Attach carrying a native
// GUTI the MME still holds — with an integrity-protected message that verifies
// against the stored context — reuses that context and skips authentication
// (TS 23.401 §5.3.2.1).
func TestAttachReusesContextForNativeGUTI(t *testing.T) {
	m := newTestMME(t)
	existing, _ := securedUE(t, m)
	oldID := existing.MMEUES1APID

	wire := nativeGUTIAttach(t, m, existing)

	cc := &captureConn{}
	fresh := m.newUe(cc, 9)
	m.handleNAS(fresh, wire)

	if fresh.authVector != nil {
		t.Fatal("authentication was not skipped on a valid native GUTI")
	}

	if fresh.imsi != existing.imsi || len(fresh.kasme) == 0 {
		t.Fatalf("security context not reused: imsi=%q kasme=%d bytes", fresh.imsi, len(fresh.kasme))
	}

	if _, ok := m.lookupUe(oldID); ok {
		t.Fatal("superseded registration was not removed")
	}

	if cc.count() != 1 {
		t.Fatalf("expected one downlink (Security Mode Command), got %d", cc.count())
	}
}

// TestAttachNativeGUTIBadMACFallsBackToAuth checks that when the Attach does not
// verify against the resolved context (stale/spoofed GUTI), the MME does not
// reuse it — it keeps the existing context and runs a normal attach.
func TestAttachNativeGUTIBadMACFallsBackToAuth(t *testing.T) {
	m := newTestMME(t)
	existing, _ := securedUE(t, m)
	oldID := existing.MMEUES1APID

	wire := nativeGUTIAttach(t, m, existing)
	wire[1] ^= 0xff // corrupt the MAC

	cc := &captureConn{}
	fresh := m.newUe(cc, 9)
	m.handleNAS(fresh, wire)

	if _, ok := m.lookupUe(oldID); !ok {
		t.Fatal("context was removed despite a MAC mismatch")
	}

	if _, err := eps.ParseIdentityRequest(decodeDownlinkNAS(t, cc.sent[0])); err != nil {
		t.Fatalf("expected a fallback Identity Request, got: %v", err)
	}
}

func TestAttachAuthenticationAndSecurityMode(t *testing.T) {
	m := newTestMME(t)
	cc := &captureConn{}
	ue := m.newUe(cc, 7)

	// 1. UE → Attach Request (IMSI), EEA2/EIA2 capable, with a PDN Connectivity
	// Request in the ESM container.
	esm, err := (&eps.PDNConnectivityRequest{ProcedureTransactionIdentity: 1, RequestType: 1, PDNType: 1}).Marshal()
	if err != nil {
		t.Fatal(err)
	}

	attach := &eps.AttachRequest{
		EPSAttachType:       eps.AttachTypeEPS,
		NASKeySetIdentifier: 7,
		EPSMobileIdentity:   eps.EPSMobileIdentity{Type: eps.IdentityIMSI, Digits: testSubscriber.IMSI},
		UENetworkCapability: eps.UENetworkCapability{EEA: 0xf0, EIA: 0x70}.Marshal(),
		ESMMessageContainer: esm,
	}

	attachBytes, err := attach.Marshal()
	if err != nil {
		t.Fatal(err)
	}

	m.handleNAS(ue, attachBytes)

	// MME → Authentication Request.
	if len(cc.sent) != 1 {
		t.Fatalf("expected 1 downlink (Authentication Request), got %d", len(cc.sent))
	}

	authReq, err := eps.ParseAuthenticationRequest(decodeDownlinkNAS(t, cc.sent[0]))
	if err != nil {
		t.Fatal(err)
	}

	// 2. UE side: compute RES from the MME's RAND (RES is independent of SQN) and
	// reply. K_ASME is what the credential authority derived; read it from the UE
	// context to mirror the UE for the rest of the exchange.
	res := make([]byte, 8)
	if err := udm.F2345(testSubscriber.OPc[:], testSubscriber.K[:], authReq.RAND[:],
		res, make([]byte, 16), make([]byte, 16), make([]byte, 6), nil); err != nil {
		t.Fatal(err)
	}

	kasme := ue.authVector.KASME

	authResp, err := (&eps.AuthenticationResponse{RES: res}).Marshal()
	if err != nil {
		t.Fatal(err)
	}

	m.handleNAS(ue, authResp)

	// MME → Security Mode Command (integrity protected with the new context).
	if len(cc.sent) != 2 {
		t.Fatalf("expected Security Mode Command, got %d downlinks", len(cc.sent))
	}

	smcWire := decodeDownlinkNAS(t, cc.sent[1])

	knasEnc, err := deriveKNASEnc(kasme, 2)
	if err != nil {
		t.Fatal(err)
	}

	knasInt, err := deriveKNASInt(kasme, 2)
	if err != nil {
		t.Fatal(err)
	}

	smcPlain, err := eps.Unprotect(smcWire, nascommon.NASCount(0, smcWire[5]), nascommon.DirectionDownlink,
		knasInt, knasEnc, nascommon.AESCMACIntegrity{}, nascommon.AESCTRCipher{})
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

	// 3. UE → Security Mode Complete (integrity protected + ciphered), returning
	// the IMEISV the Security Mode Command requested (TS 24.301 §5.4.3.2).
	smCompletePlain, err := (&eps.SecurityModeComplete{
		IMEISV: []byte{0x03, 0x53, 0x60, 0x83, 0x12, 0x34, 0x56, 0x78, 0xf0},
	}).Marshal()
	if err != nil {
		t.Fatal(err)
	}

	smCompleteWire, err := eps.Protect(smCompletePlain, eps.SHTIntegrityProtectedCiphered, nascommon.NASCount(0, 0),
		nascommon.DirectionUplink, knasInt, knasEnc, nascommon.AESCMACIntegrity{}, nascommon.AESCTRCipher{})
	if err != nil {
		t.Fatal(err)
	}

	m.handleNAS(ue, smCompleteWire)

	if !ue.secured {
		t.Fatal("NAS security context not established after Security Mode Complete")
	}

	// The IMEISV is converted to a 15-digit IMEI for the status API.
	if ue.imei != "035063821436588" {
		t.Fatalf("IMEI from IMEISV = %q, want 035063821436588", ue.imei)
	}

	// Initial Context Setup seeds the X2-handover key chain: NH(NCC=1) is ready
	// for a later Path Switch (TS 33.401 §7.2.8.4).
	if ue.ncc != 1 || ue.nh == ([32]byte{}) {
		t.Fatalf("NH chain not seeded: ncc=%d nh-zero=%v", ue.ncc, ue.nh == ([32]byte{}))
	}

	// MME → Initial Context Setup Request (default bearer + Attach Accept).
	if len(cc.sent) != 3 {
		t.Fatalf("expected Initial Context Setup Request, got %d downlinks", len(cc.sent))
	}

	ics := parseInitialContextSetup(t, cc.sent[2])

	if ics.MMEUES1APID != ue.MMEUES1APID || ics.ENBUES1APID != 7 || len(ics.ERABToBeSetup) != 1 {
		t.Fatalf("unexpected Initial Context Setup Request: %+v", ics)
	}

	// K_eNB uses the uplink NAS COUNT of the Security Mode Complete (one less
	// than the next-expected count).
	wantKeNB, err := deriveKeNB(kasme, ue.ulCount-1)
	if err != nil {
		t.Fatal(err)
	}

	if [32]byte(ics.SecurityKey) != wantKeNB {
		t.Fatalf("K_eNB mismatch in Initial Context Setup Request")
	}

	erab := ics.ERABToBeSetup[0]
	if erab.ERABID != s1ap.ERABID(defaultERABID) || erab.QoS.QCI != s1ap.QCI(9) ||
		erab.GTPTEID != s1ap.GTPTEID(testSGWFTEID.TEID) {
		t.Fatalf("unexpected E-RAB: %+v", erab)
	}

	// The Attach Accept (the E-RAB's NAS-PDU) carries the UE's assigned IP.
	acceptWire := []byte(erab.NASPDU)

	acceptPlain, err := eps.Unprotect(acceptWire, nascommon.NASCount(0, acceptWire[5]), nascommon.DirectionDownlink,
		knasInt, knasEnc, nascommon.AESCMACIntegrity{}, nascommon.AESCTRCipher{})
	if err != nil {
		t.Fatalf("Attach Accept failed integrity check: %v", err)
	}

	accept, err := eps.ParseAttachAccept(acceptPlain)
	if err != nil {
		t.Fatal(err)
	}

	// The Attach Accept assigns a GUTI (TS 24.301 §5.5.1.2.4) and the UE is
	// indexed by its M-TMSI for later S-TMSI-addressed procedures.
	if accept.GUTI == nil || accept.GUTI.Type != eps.IdentityGUTI {
		t.Fatal("Attach Accept did not assign a GUTI")
	}

	// GUMMEI sourced from the operator config (fakeBearerStore returns group/code 1).
	if accept.GUTI.MCC != "001" || accept.GUTI.MNC != "01" ||
		accept.GUTI.MMEGroupID != 1 || accept.GUTI.MMECode != 1 {
		t.Fatalf("unexpected GUTI: %+v", accept.GUTI)
	}

	if _, ok := m.lookupUeByMTMSI(accept.GUTI.MTMSI); !ok {
		t.Fatal("UE not indexed by its assigned M-TMSI")
	}

	activate, err := eps.ParseActivateDefaultEPSBearerContextRequest(accept.ESMMessageContainer)
	if err != nil {
		t.Fatal(err)
	}

	pdn, err := eps.ParsePDNAddress(activate.PDNAddress)
	if err != nil {
		t.Fatal(err)
	}

	if pdn.IPv4 != testUEIP.As4() {
		t.Fatalf("assigned UE IP = %v, want %v", pdn.IPv4, testUEIP.As4())
	}

	// 4. UE → Attach Complete reaches EMM-REGISTERED.
	complete, err := (&eps.AttachComplete{ESMMessageContainer: []byte{0x02, activate.ProcedureTransactionIdentity, 0xc2}}).Marshal()
	if err != nil {
		t.Fatal(err)
	}

	completeWire, err := eps.Protect(complete, eps.SHTIntegrityProtectedCiphered, nascommon.NASCount(0, uint8(ue.ulCount)),
		nascommon.DirectionUplink, knasInt, knasEnc, nascommon.AESCMACIntegrity{}, nascommon.AESCTRCipher{})
	if err != nil {
		t.Fatal(err)
	}

	m.handleNAS(ue, completeWire)

	if ue.emmState != EMMRegistered {
		t.Fatal("UE not EMM-REGISTERED after Attach Complete")
	}
}

// TestSecurityModeRejectReleasesUE checks that a SECURITY MODE REJECT — sent
// unprotected by the UE (TS 24.301 §5.4.3.5) — aborts the security mode control
// procedure and releases the UE's S1 context. Cause #23 (UE security
// capabilities mismatch) is the reject a misbuilt replayed UE security
// capability provokes.
func TestSecurityModeRejectReleasesUE(t *testing.T) {
	m := newTestMME(t)
	cc := &captureConn{}
	ue := m.newUe(cc, 7)

	plain, err := (&eps.SecurityModeReject{Cause: 23}).Marshal()
	if err != nil {
		t.Fatal(err)
	}

	m.handleNAS(ue, plain)

	if !ue.releasing {
		t.Fatal("UE not released after Security Mode Reject")
	}

	if cc.count() != 1 {
		t.Fatalf("expected one S1AP message (UE Context Release Command), got %d", cc.count())
	}

	parseUEContextReleaseCommand(t, cc.sent[0])
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
