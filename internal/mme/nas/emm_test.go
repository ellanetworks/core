// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"bytes"
	"context"
	"crypto/sha256"
	"sync"
	"testing"

	"github.com/ellanetworks/core/internal/mme"
	"github.com/ellanetworks/core/internal/sctp"
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
// still process the Attach Request, not drop it. With a GUTI it cannot
// resolve, the MME recovers by requesting the UE's identity.
func TestAttachRecoveryAfterMMERestart(t *testing.T) {
	m := newTestMME(t)
	cc := &captureConn{}
	ue := m.NewUe(cc, 7)

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

	HandleNAS(m, context.Background(), ue.Conn(), nas)

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
// context the network has lost. Per TS 24.301 §4.4.4.3 the MME must still
// process it (the IMSI is in cleartext) and proceed to authentication, instead
// of dropping it and looping on IDENTITY REQUEST.
func TestIdentityResponseRecoveryAfterMMERestart(t *testing.T) {
	m := newTestMME(t)
	cc := &captureConn{}
	ue := m.NewUe(cc, 8)
	ue.TransitionTo(mme.EMMRegistrationInitiated) // attach in progress: authentication sub-phase

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

	HandleNAS(m, context.Background(), ue.Conn(), nas)

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

// nativeGUTIAttach builds an integrity-protected combined ATTACH REQUEST that
// carries `ue`'s native GUTI, protected with `ue`'s NAS security context — the
// message a returning UE sends when it still holds a context the MME assigned.
func nativeGUTIAttach(t *testing.T, m *mme.MME, ue *mme.UeContext) []byte {
	t.Helper()

	plmn, err := m.OperatorPLMN(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	group, code := m.MmeIdentity()
	guti := m.ReallocateGUTI(ue, plmn, group, code)

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
		nascommon.DirectionUplink, ue.KnasIntForTest(), ue.KnasEncForTest(), nascommon.AESCMACIntegrity{}, nascommon.AESCTRCipher{})
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
	existing.SetDLCountForTest(7) // a live downlink chain that reuse must continue, not reset

	wire := nativeGUTIAttach(t, m, existing)

	cc := &captureConn{}
	fresh := m.NewUe(cc, 9)
	HandleNAS(m, context.Background(), fresh.Conn(), wire)

	// The held context is reused in place — the connection is rebound onto it and
	// the transient context the Initial UE Message created is discarded.
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

	// NAS COUNTs continue (TS 24.301 §4.4.3, §5.4.3.3): a native context is reused,
	// not re-derived, so the counts are never reset to zero — reusing them with the
	// same keys would be a keystream reuse.
	if existing.ULCount() != 1 {
		t.Fatalf("uplink NAS COUNT = %d, want 1 (continued past the Attach)", existing.ULCount())
	}

	if existing.DLCountForTest() < 7 {
		t.Fatalf("downlink NAS COUNT reset to %d on context reuse (keystream reuse)", existing.DLCountForTest())
	}

	// The security mode procedure is skipped: the only downlink is the Initial
	// Context Setup carrying the Attach Accept, not a Security Mode Command.
	if cc.count() != 1 {
		t.Fatalf("expected one downlink (Initial Context Setup), got %d", cc.count())
	}

	parseInitialContextSetup(t, cc.sent[0])
}

// TestAttachReusesContextForNativeGUTI_ReleasesOldBearers asserts TS 24.301
// §5.5.1.2.4 case f: a genuine re-attach reuses the security context (§4.4.3, keys
// kept) but the UE's OLD EPS bearer contexts are deleted — their anchor sessions
// released, not preserved — before the new attach is progressed.
func TestAttachReusesContextForNativeGUTI_ReleasesOldBearers(t *testing.T) {
	m := newTestMME(t)
	existing, _ := securedUE(t, m)

	// The returning UE held a live PDN before the re-attach.
	if existing.Pdns == nil {
		existing.Pdns = map[uint8]*mme.PdnConnection{}
	}

	existing.Pdns[5] = &mme.PdnConnection{Ebi: 5}

	wire := nativeGUTIAttach(t, m, existing)

	cc := &captureConn{}
	fresh := m.NewUe(cc, 9)
	HandleNAS(m, context.Background(), fresh.Conn(), wire)

	if !m.Session.(*fakeSessionManager).released {
		t.Fatal("old EPS bearer not released on native-GUTI re-attach (case f: old bearers must be deleted)")
	}
}

// TestAttachKeepsOldGUTIResolvableUntilComplete guards the two-phase GUTI
// reallocation on attach: the M-TMSI the UE was addressed by stays resolvable
// through the T3450 window (TS 24.301 §5.5.1.2.7 — the old GUTI is valid until
// the UE acknowledges), and ATTACH COMPLETE commits the new GUTI and frees the
// old.
func TestAttachKeepsOldGUTIResolvableUntilComplete(t *testing.T) {
	m := newTestMME(t)
	existing, _ := securedUE(t, m)

	wire := nativeGUTIAttach(t, m, existing)
	presented := existing.TmsiForTest()

	cc := &captureConn{}
	fresh := m.NewUe(cc, 9)
	HandleNAS(m, context.Background(), fresh.Conn(), wire)

	// The Attach Accept has been sent but not yet acknowledged. The M-TMSI the UE
	// presented must stay resolvable so a retransmitted Attach still finds the
	// context, and the newly allocated one must resolve too.
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

	complete, err := (&eps.AttachComplete{}).Marshal()
	if err != nil {
		t.Fatal(err)
	}

	handleAttachComplete(m, context.Background(), existing, complete)

	if existing.OldTmsiForTest() != 0 {
		t.Fatal("GUTI reallocation not committed after Attach Complete")
	}

	if _, ok := m.LookupUeByMTMSI(presented); ok {
		t.Fatal("old M-TMSI still resolvable after Attach Complete")
	}

	if got, ok := m.LookupUeByMTMSI(newMTMSI); !ok || got != existing {
		t.Fatal("new M-TMSI lost after Attach Complete")
	}
}

// TestAttachNativeGUTIBadMACFallsBackToAuth checks that when the Attach does not
// verify against the resolved context (stale/spoofed GUTI), the MME does not
// reuse it — it keeps the existing context and runs a normal attach.
func TestAttachNativeGUTIBadMACFallsBackToAuth(t *testing.T) {
	m := newTestMME(t)
	existing, _ := securedUE(t, m)
	oldID := existing.Conn().MMEUES1APID

	wire := nativeGUTIAttach(t, m, existing)
	wire[1] ^= 0xff // corrupt the MAC

	cc := &captureConn{}
	fresh := m.NewUe(cc, 9)
	HandleNAS(m, context.Background(), fresh.Conn(), wire)

	if _, ok := m.LookupUe(oldID); !ok {
		t.Fatal("context was removed despite a MAC mismatch")
	}

	if _, err := eps.ParseIdentityRequest(decodeDownlinkNAS(t, cc.sent[0])); err != nil {
		t.Fatalf("expected a fallback Identity Request, got: %v", err)
	}
}

// A replayed native-GUTI Attach with a stale uplink NAS COUNT must not remove the
// live context (TS 24.301).
func TestAttachNativeGUTIReplayDoesNotRemoveContext(t *testing.T) {
	m := newTestMME(t)
	existing, _ := securedUE(t, m)
	oldID := existing.Conn().MMEUES1APID

	wire := nativeGUTIAttach(t, m, existing) // protected at NASCount(0, 0)

	existing.SetULCountForTest(50)

	cc := &captureConn{}
	attacker := m.NewUe(cc, 9)
	HandleNAS(m, context.Background(), attacker.Conn(), wire)

	if _, ok := m.LookupUe(oldID); !ok {
		t.Fatal("live context removed by a replayed stale-count Attach")
	}
}

func TestAttachAuthenticationAndSecurityMode(t *testing.T) {
	m := newTestMME(t)
	cc := &captureConn{}
	ue := m.NewUe(cc, 7)

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

	HandleNAS(m, context.Background(), ue.Conn(), attachBytes)

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

	kasme := ue.Conn().AuthVector.KASME

	authResp, err := (&eps.AuthenticationResponse{RES: res}).Marshal()
	if err != nil {
		t.Fatal(err)
	}

	HandleNAS(m, context.Background(), ue.Conn(), authResp)

	// Authentication succeeded: the vector is dropped (its K_ASME is held in the security
	// context) so no key material lingers and AuthVector==nil means "no challenge in
	// flight"; the per-exchange resync budget is reset.
	if ue.Conn().AuthVector != nil {
		t.Fatal("AuthVector must be cleared on authentication success")
	}

	if ue.Conn().ResyncTried() {
		t.Fatal("resyncTried must be reset on authentication success")
	}

	// MME → Security Mode Command (integrity protected with the new context).
	if len(cc.sent) != 2 {
		t.Fatalf("expected Security Mode Command, got %d downlinks", len(cc.sent))
	}

	smcWire := decodeDownlinkNAS(t, cc.sent[1])

	knasEnc, err := mme.DeriveKNASEnc(kasme, 2)
	if err != nil {
		t.Fatal(err)
	}

	knasInt, err := mme.DeriveKNASInt(kasme, 2)
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

	// The unprotected Attach is hashed into the SMC HashMME (TS 24.301 §5.4.3.2).
	wantHash := sha256.Sum256(attachBytes)
	if !bytes.Equal(smc.HASHMME, wantHash[:8]) {
		t.Fatalf("SMC mme.HashMME = %x, want %x", smc.HASHMME, wantHash[:8])
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

	HandleNAS(m, context.Background(), ue.Conn(), smCompleteWire)

	if !ue.Secured() {
		t.Fatal("NAS security context not established after Security Mode Complete")
	}

	// The IMEISV is converted to a 15-digit IMEI for the status API.
	if ue.Imei.IMEI() != "035063821436588" {
		t.Fatalf("IMEI from IMEISV = %q, want 035063821436588", ue.Imei.IMEI())
	}

	// Initial Context Setup seeds the X2-handover key chain: NH(NCC=1) is ready
	// for a later Path Switch (TS 33.401 §7.2.8.4).
	if ue.NCCForTest() != 1 || ue.NHForTest() == ([32]byte{}) {
		t.Fatalf("NH chain not seeded: ncc=%d nh-zero=%v", ue.NCCForTest(), ue.NHForTest() == ([32]byte{}))
	}

	// MME → Initial Context Setup Request (default bearer + Attach Accept).
	if len(cc.sent) != 3 {
		t.Fatalf("expected Initial Context Setup Request, got %d downlinks", len(cc.sent))
	}

	ics := parseInitialContextSetup(t, cc.sent[2])

	if ics.MMEUES1APID != ue.Conn().MMEUES1APID || ics.ENBUES1APID != 7 || len(ics.ERABToBeSetup) != 1 {
		t.Fatalf("unexpected Initial Context Setup Request: %+v", ics)
	}

	// K_eNB uses the uplink NAS COUNT of the Security Mode Complete (one less
	// than the next-expected count).
	wantKeNB, err := mme.DeriveKeNB(kasme, ue.ULCount()-1)
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

	if _, ok := m.LookupUeByMTMSI(accept.GUTI.MTMSI); !ok {
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

	completeWire, err := eps.Protect(complete, eps.SHTIntegrityProtectedCiphered, nascommon.NASCount(0, uint8(ue.ULCount())),
		nascommon.DirectionUplink, knasInt, knasEnc, nascommon.AESCMACIntegrity{}, nascommon.AESCTRCipher{})
	if err != nil {
		t.Fatal(err)
	}

	HandleNAS(m, context.Background(), ue.Conn(), completeWire)

	if ue.EMMState() != mme.EMMRegistered {
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
	ue := m.NewUe(cc, 7)

	// A security mode exchange is in flight (the command was sent).
	if !m.TryClaimKeyChain(ue) {
		t.Fatal("could not claim the security mode exchange")
	}

	ue.TransitionTo(mme.EMMRegistrationInitiated)
	ue.AdvanceRegStep(mme.RegStepSecurityMode)

	plain, err := (&eps.SecurityModeReject{Cause: 23}).Marshal()
	if err != nil {
		t.Fatal(err)
	}

	HandleNAS(m, context.Background(), ue.Conn(), plain)

	if !ue.Conn().ReleasingForTest() {
		t.Fatal("UE not released after Security Mode Reject")
	}

	if cc.count() != 1 {
		t.Fatalf("expected one S1AP message (UE Context Release Command), got %d", cc.count())
	}

	parseUEContextReleaseCommand(t, cc.sent[0])
}

// TestIdentityResponseIgnoredAfterAuthStarted verifies an out-of-order IDENTITY
// RESPONSE (authentication already in progress) does not re-set the IMSI or
// restart authentication — the message is admissible without integrity
// (TS 24.301 §4.4.4.3). Mirrors the AMF's RegStep gating.
func TestIdentityResponseIgnoredAfterAuthStarted(t *testing.T) {
	m := newTestMME(t)
	cc := &captureConn{}
	ue := m.NewUe(cc, 7)
	ue.SetIMSIForTest(testSubscriber.IMSI)
	ue.Conn().AuthVector = &udm.EPSAV{} // authentication already in progress

	// An IDENTITY RESPONSE carrying a different identity (type-of-identity = IMSI).
	plain, err := (&eps.IdentityResponse{MobileIdentity: []byte{0x19, 0x32, 0x54, 0x76, 0x98}}).Marshal()
	if err != nil {
		t.Fatal(err)
	}

	handleIdentityResponse(m, context.Background(), ue, plain)

	if ue.IMSI() != testSubscriber.IMSI {
		t.Fatalf("out-of-order Identity Response overwrote the IMSI: got %q, want %q", ue.IMSI(), testSubscriber.IMSI)
	}
}

// TestSecurityModeRejectIgnoredOutsideExchange verifies an out-of-order SECURITY
// MODE REJECT (no security mode exchange in flight) does not release the UE — the
// message is admissible without integrity (TS 24.301 §4.4.4.3), so a spurious one
// must not tear down a UE. Mirrors the AMF's RegStep gating.
func TestSecurityModeRejectIgnoredOutsideExchange(t *testing.T) {
	m := newTestMME(t)
	cc := &captureConn{}
	ue := m.NewUe(cc, 7)

	// No security mode exchange is claimed.
	plain, err := (&eps.SecurityModeReject{Cause: 23}).Marshal()
	if err != nil {
		t.Fatal(err)
	}

	HandleNAS(m, context.Background(), ue.Conn(), plain)

	if ue.Conn() == nil || ue.Conn().ReleasingForTest() {
		t.Fatal("an out-of-order Security Mode Reject must not release the UE")
	}

	if cc.count() != 0 {
		t.Fatalf("expected no S1AP message for an ignored reject, got %d", cc.count())
	}
}

// TestSecurityModeCompleteRecoversReplayedAttach verifies the anti-tamper
// recovery: when SECURITY MODE COMPLETE carries a Replayed NAS message container,
// the MME re-ingests the genuine ATTACH REQUEST it holds, not the
// (possibly tampered) parameters from the initial plain Attach (TS 24.301
// §5.4.3.4).
func TestSecurityModeCompleteRecoversReplayedAttach(t *testing.T) {
	m := newTestMME(t)
	ue, _ := securedUE(t, m)

	// A security mode exchange is in flight (the command was sent).
	if !m.TryClaimKeyChain(ue) {
		t.Fatal("could not claim the security mode exchange")
	}

	ue.ForceRegStepForTest(mme.RegStepSecurityMode)

	// A tampered initial Attach was ingested before the security context existed.
	ue.CombinedAttach = true
	ue.RequestedAPN = "tampered-apn"

	// The UE's HASHMME check failed, so it replays the genuine plain Attach it sent:
	// a plain EPS attach with no APN.
	esm, err := (&eps.PDNConnectivityRequest{ProcedureTransactionIdentity: 1, RequestType: 1, PDNType: 1}).Marshal()
	if err != nil {
		t.Fatal(err)
	}

	genuine, err := (&eps.AttachRequest{
		EPSAttachType:       eps.AttachTypeEPS,
		NASKeySetIdentifier: 7,
		EPSMobileIdentity:   eps.EPSMobileIdentity{Type: eps.IdentityIMSI, Digits: testSubscriber.IMSI},
		UENetworkCapability: eps.UENetworkCapability{EEA: 0xf0, EIA: 0x70}.Marshal(),
		ESMMessageContainer: esm,
	}).Marshal()
	if err != nil {
		t.Fatal(err)
	}

	smc, err := (&eps.SecurityModeComplete{ReplayedNASMessage: genuine}).Marshal()
	if err != nil {
		t.Fatal(err)
	}

	handleSecurityModeComplete(m, context.Background(), ue, smc)

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

// TestDispatchEMM_UnhandledMessageSendsEMMStatus verifies that an unhandled EMM
// message type is answered with an EMM STATUS so the UE is not left waiting
// (TS 24.301 §5.7; mirrors the AMF's 5GMM STATUS on NAS protocol error).
func TestDispatchEMM_UnhandledMessageSendsEMMStatus(t *testing.T) {
	m := newTestMME(t)
	cc := &captureConn{}
	ue := m.NewUe(cc, 7)

	// A plain EMM message (SHT=plain, PD=EMM) carrying a type the MME does not handle.
	plain := []byte{0x07, 0x55}

	HandleEmmMessage(m, context.Background(), ue, plain, true)

	if len(cc.sent) != 1 {
		t.Fatalf("expected 1 downlink (EMM STATUS), got %d", len(cc.sent))
	}

	status, err := eps.ParseEMMStatus(decodeDownlinkNAS(t, cc.sent[0]))
	if err != nil {
		t.Fatalf("parse EMM STATUS: %v", err)
	}

	if status.EMMCause != mme.EmmCauseMessageTypeNonExistent {
		t.Fatalf("EMM STATUS cause = %d, want %d", status.EMMCause, mme.EmmCauseMessageTypeNonExistent)
	}
}

// TestDispatchEMM_EMMStatusHandledNoReply verifies that an inbound EMM STATUS is
// handled locally with no state change and no reply (TS 24.301 §5.7) — in
// particular it must not fall through to the unhandled-message path that would send
// an EMM STATUS back.
func TestDispatchEMM_EMMStatusHandledNoReply(t *testing.T) {
	m := newTestMME(t)
	cc := &captureConn{}
	ue := m.NewUe(cc, 7)

	plain, err := (&eps.EMMStatus{EMMCause: mme.EmmCauseProtocolErrorUnspec}).Marshal()
	if err != nil {
		t.Fatal(err)
	}

	HandleEmmMessage(m, context.Background(), ue, plain, true)

	if cc.count() != 0 {
		t.Fatalf("EMM STATUS must be handled with no reply, got %d downlink(s)", cc.count())
	}
}

// TestAttachDuplicateIdenticalIEsResendsAccept verifies a duplicate ATTACH REQUEST
// with identical IEs while awaiting ATTACH COMPLETE resends the ATTACH ACCEPT without
// re-authenticating or releasing the UE (TS 24.301 §5.5.1.2.7 case d).
func TestAttachDuplicateIdenticalIEsResendsAccept(t *testing.T) {
	m := newTestMME(t)
	ue, cc := securedUE(t, m)
	ue.ForceRegStepForTest(mme.RegStepContextSetup)

	attach := &eps.AttachRequest{
		EPSAttachType:       eps.AttachTypeEPS,
		NASKeySetIdentifier: 7,
		EPSMobileIdentity:   eps.EPSMobileIdentity{Type: eps.IdentityIMSI, Digits: "001010000000001"},
		UENetworkCapability: eps.UENetworkCapability{EEA: 0xf0, EIA: 0x70}.Marshal(),
	}

	plain, err := attach.Marshal()
	if err != nil {
		t.Fatal(err)
	}

	// The attach being served, and the accept last sent.
	ue.Conn().AttachRequestPlain = plain
	ue.Conn().AttachAcceptPdu = []byte{0x07, 0x42, 0x01}

	handleAttachRequest(m, context.Background(), ue, plain, false)

	if cc.count() != 1 {
		t.Fatalf("expected the Attach Accept resent (one downlink), got %d", cc.count())
	}

	if ue.Conn() == nil || ue.Conn().ReleasingForTest() {
		t.Fatal("an identical duplicate Attach Request must not release the UE")
	}

	if ue.RegStep() != mme.RegStepContextSetup {
		t.Fatalf("an identical duplicate must not re-authenticate; RegStep = %s", ue.RegStep())
	}
}

// TestAttachDuplicateDifferingIEsProgresses verifies an ATTACH REQUEST with differing
// IEs while awaiting ATTACH COMPLETE aborts the previous attach and progresses the new
// one — here re-identifying via authentication (TS 24.301 §5.5.1.2.7 case d).
func TestAttachDuplicateDifferingIEsProgresses(t *testing.T) {
	m := newTestMME(t)
	ue, cc := securedUE(t, m)
	ue.ForceRegStepForTest(mme.RegStepContextSetup)

	attach := &eps.AttachRequest{
		EPSAttachType:       eps.AttachTypeEPS,
		NASKeySetIdentifier: 7,
		EPSMobileIdentity:   eps.EPSMobileIdentity{Type: eps.IdentityIMSI, Digits: "001010000000001"},
		UENetworkCapability: eps.UENetworkCapability{EEA: 0xf0, EIA: 0x70}.Marshal(),
	}

	plain, err := attach.Marshal()
	if err != nil {
		t.Fatal(err)
	}

	// A different prior request, so the incoming one differs and the attach progresses.
	ue.Conn().AttachRequestPlain = []byte{0x07, 0x41, 0x99}
	ue.Conn().AttachAcceptPdu = []byte{0x07, 0x42, 0x01}

	handleAttachRequest(m, context.Background(), ue, plain, false)

	// Progressing an IMSI attach re-authenticates: it enters the authentication
	// sub-phase and sends an AUTHENTICATION REQUEST, not a resent accept.
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

// TestAttachIgnoredDuringNetworkInitiatedDetach verifies that an ATTACH REQUEST
// colliding with a network-initiated ("re-attach not required") detach is ignored,
// not superseding the detach (TS 24.301 §5.5.2.3.4 case d).
func TestAttachIgnoredDuringNetworkInitiatedDetach(t *testing.T) {
	m := newTestMME(t)
	cc := &captureConn{}
	ue := m.NewUe(cc, 7)
	ue.ForceStateForTest(mme.EMMDeregistrationInitiated)

	attach := &eps.AttachRequest{
		EPSAttachType:       eps.AttachTypeEPS,
		NASKeySetIdentifier: 7,
		EPSMobileIdentity:   eps.EPSMobileIdentity{Type: eps.IdentityIMSI, Digits: "001010000000001"},
		UENetworkCapability: eps.UENetworkCapability{EEA: 0xf0, EIA: 0x70}.Marshal(),
	}

	plain, err := attach.Marshal()
	if err != nil {
		t.Fatal(err)
	}

	handleAttachRequest(m, context.Background(), ue, plain, false)

	if ue.EMMState() != mme.EMMDeregistrationInitiated {
		t.Fatalf("attach during network-initiated detach must be ignored; state = %s, want EMM-DEREGISTERED-INITIATED", ue.EMMState())
	}

	if cc.count() != 0 {
		t.Fatalf("expected no downlink for an ignored attach, got %d", cc.count())
	}
}
