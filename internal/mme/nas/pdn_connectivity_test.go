// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"context"
	"net/netip"
	"testing"
	"time"

	"github.com/ellanetworks/core/internal/mme"
	mmes1ap "github.com/ellanetworks/core/internal/mme/s1ap"
	"github.com/ellanetworks/core/internal/models"
	nascommon "github.com/ellanetworks/core/nas/common"
	"github.com/ellanetworks/core/nas/eps"
	"github.com/ellanetworks/core/s1ap"
)

// findERABSetupRequest returns the last E-RAB SETUP REQUEST the capture holds.
func findERABSetupRequest(t *testing.T, cc *captureConn) *s1ap.ERABSetupRequest {
	t.Helper()

	cc.mu.Lock()
	defer cc.mu.Unlock()

	for i := len(cc.sent) - 1; i >= 0; i-- {
		pdu, err := s1ap.Unmarshal(cc.sent[i])
		if err != nil {
			continue
		}

		im, ok := pdu.(*s1ap.InitiatingMessage)
		if !ok || im.ProcedureCode != s1ap.ProcERABSetup {
			continue
		}

		req, err := s1ap.ParseERABSetupRequest(im.Value)
		if err != nil {
			t.Fatalf("parse E-RAB Setup Request: %v", err)
		}

		return req
	}

	t.Fatal("no E-RAB Setup Request sent")

	return nil
}

// findERABReleaseCommand returns the last E-RAB RELEASE COMMAND the capture holds.
func findERABReleaseCommand(t *testing.T, cc *captureConn) *s1ap.ERABReleaseCommand {
	t.Helper()

	cc.mu.Lock()
	defer cc.mu.Unlock()

	for i := len(cc.sent) - 1; i >= 0; i-- {
		pdu, err := s1ap.Unmarshal(cc.sent[i])
		if err != nil {
			continue
		}

		im, ok := pdu.(*s1ap.InitiatingMessage)
		if !ok || im.ProcedureCode != s1ap.ProcERABRelease {
			continue
		}

		cmd, err := s1ap.ParseERABReleaseCommand(im.Value)
		if err != nil {
			t.Fatalf("parse E-RAB Release Command: %v", err)
		}

		return cmd
	}

	t.Fatal("no E-RAB Release Command sent")

	return nil
}

// lastDownlinkESM decodes the most recent protected downlink NAS message the MME
// sent and returns its plaintext (e.g. a PDN Connectivity / Disconnect Reject).
func lastDownlinkESM(t *testing.T, ue *mme.UeContext, cc *captureConn) []byte {
	t.Helper()

	wire := decodeDownlinkNAS(t, cc.sent[len(cc.sent)-1])

	plain, err := eps.Unprotect(wire, nascommon.NASCount(0, wire[5]), nascommon.DirectionDownlink,
		ue.KnasIntForTest(), ue.KnasEncForTest(), nascommon.AESCMACIntegrity{}, nascommon.AESCTRCipher{})
	if err != nil {
		t.Fatalf("unprotect downlink: %v", err)
	}

	return plain
}

// TestAdditionalPDNConnectionLifecycle drives the full multiple-PDN flow: a
// registered UE opens a second PDN connection, the eNB sets up its radio leg, the
// UE accepts the default bearer, then the UE disconnects the second PDN — leaving
// the UE registered with only its first PDN (TS 24.301 §6.5).
func TestAdditionalPDNConnectionLifecycle(t *testing.T) {
	m := newTestMME(t)
	ue, cc := securedUE(t, m)

	p0 := testPDN(ue)
	p0.Apn = "internet"

	apnIE, err := eps.MarshalAPN("ims")
	if err != nil {
		t.Fatal(err)
	}

	connReq, err := (&eps.PDNConnectivityRequest{
		ProcedureTransactionIdentity: 2, RequestType: 1, PDNType: eps.PDNTypeIPv4, AccessPointName: apnIE,
	}).Marshal()
	if err != nil {
		t.Fatal(err)
	}

	handlePDNConnectivityRequest(context.Background(), m, ue, connReq)

	p := ue.PdnForAPN("ims")
	if p == nil {
		t.Fatal("second PDN connection not created")
	}

	if p.Ebi != 6 {
		t.Fatalf("second PDN EBI = %d, want 6", p.Ebi)
	}

	req := findERABSetupRequest(t, cc)
	if len(req.ERABToBeSetup) != 1 || req.ERABToBeSetup[0].ERABID != 6 || len(req.ERABToBeSetup[0].NASPDU) == 0 {
		t.Fatalf("E-RAB Setup Request malformed: %+v", req)
	}

	tla, err := models.EncodeTransportLayerAddress(netip.MustParseAddr("10.31.0.9"), netip.Addr{})
	if err != nil {
		t.Fatal(err)
	}

	resp := &s1ap.ERABSetupResponse{
		MMEUES1APID: ue.Conn().MMEUES1APID, ENBUES1APID: ue.Conn().ENBUES1APID,
		ERABSetup: []s1ap.ERABSetupItemBearerSURes{{
			ERABID: 6, TransportLayerAddress: s1ap.TransportLayerAddress(tla), GTPTEID: 0x1234,
		}},
	}

	rb, err := resp.Marshal()
	if err != nil {
		t.Fatal(err)
	}

	rpdu, err := s1ap.Unmarshal(rb)
	if err != nil {
		t.Fatal(err)
	}

	mmes1ap.HandleERABSetupResponse(m, context.Background(), mme.NewRadioForTest(cc), rpdu.(*s1ap.SuccessfulOutcome).Value)

	if ue.Pdns[6].EnbFTEID.TEID != 0x1234 {
		t.Fatalf("eNB F-TEID not recorded on the second PDN: %+v", ue.Pdns[6].EnbFTEID)
	}

	if !m.Session.(*fakeSessionManager).modifiedENB.Addr.IsValid() {
		t.Fatal("ModifyEPSSession not called for the second PDN")
	}

	acc, err := (&eps.ActivateDefaultEPSBearerContextAccept{EPSBearerIdentity: 6, ProcedureTransactionIdentity: 2}).Marshal()
	if err != nil {
		t.Fatal(err)
	}

	handleActivateDefaultBearerAccept(m, ue, acc)

	dis, err := (&eps.PDNDisconnectRequest{ProcedureTransactionIdentity: 3, LinkedEPSBearerIdentity: 6}).Marshal()
	if err != nil {
		t.Fatal(err)
	}

	handlePDNDisconnectRequest(context.Background(), m, ue, dis)

	if !ue.Pdns[6].Deactivating || !ue.Pdns[6].Disconnecting {
		t.Fatalf("deactivation not in flight for the disconnected PDN: %+v", ue.Pdns[6])
	}

	// TS 23.401 §5.4.4 (symmetric with 5G TS 23.502 §4.3.4.2 step 2): the UPF user plane
	// is released at the start of the deactivation — before the UE's DEACTIVATE ACCEPT —
	// so it stops forwarding immediately rather than until the handshake completes.
	if !m.Session.(*fakeSessionManager).released {
		t.Fatal("EPS session not released up front on the PDN disconnect request; the UPF would keep forwarding until the accept")
	}

	// The deactivation of an additional PDN releases the radio bearer via an
	// E-RAB Release Command carrying the Deactivate NAS, so a real eNB tears the
	// E-RAB down (TS 23.401 §5.10.3) — not a plain Downlink NAS Transport.
	relCmd := findERABReleaseCommand(t, cc)
	if len(relCmd.ERABToBeReleased) != 1 || relCmd.ERABToBeReleased[0].ERABID != 6 || len(relCmd.NASPDU) == 0 {
		t.Fatalf("E-RAB Release Command malformed: %+v", relCmd)
	}

	relResp := &s1ap.ERABReleaseResponse{
		MMEUES1APID: ue.Conn().MMEUES1APID, ENBUES1APID: ue.Conn().ENBUES1APID,
		ERABReleased: []s1ap.ERABReleaseItemBearerRelComp{{ERABID: 6}},
	}

	rrb, err := relResp.Marshal()
	if err != nil {
		t.Fatal(err)
	}

	rrpdu, err := s1ap.Unmarshal(rrb)
	if err != nil {
		t.Fatal(err)
	}

	mmes1ap.HandleERABReleaseResponse(m, mme.NewRadioForTest(cc), rrpdu.(*s1ap.SuccessfulOutcome).Value)

	da, err := (&eps.DeactivateEPSBearerContextAccept{EPSBearerIdentity: 6, ProcedureTransactionIdentity: 3}).Marshal()
	if err != nil {
		t.Fatal(err)
	}

	handleDeactivateBearerAccept(context.Background(), m, ue, da)

	if _, ok := ue.Pdns[6]; ok {
		t.Fatal("second PDN not released after disconnect accept")
	}

	if _, ok := m.LookupUe(ue.Conn().MMEUES1APID); !ok {
		t.Fatal("UE removed by a single-PDN disconnect; expected it to stay registered")
	}

	if ue.EMMState() != mme.EMMRegistered {
		t.Fatalf("UE emmState = %v after PDN disconnect, want mme.EMMRegistered", ue.EMMState())
	}

	if p := m.DefaultPDN(ue); p == nil || p.Apn != "internet" {
		t.Fatal("default PDN disturbed by the second PDN's disconnect")
	}
}

// TestAdditionalPDNActivationIsGuarded verifies T3485 guards a standalone additional-PDN
// activation: armed on send, stopped on accept (TS 24.301 §6.4.1.6).
func TestAdditionalPDNActivationIsGuarded(t *testing.T) {
	m := newTestMME(t)
	ue, _ := securedUE(t, m)

	testPDN(ue).Apn = "internet"

	apnIE, err := eps.MarshalAPN("ims")
	if err != nil {
		t.Fatal(err)
	}

	connReq, err := (&eps.PDNConnectivityRequest{
		ProcedureTransactionIdentity: 2, RequestType: 1, PDNType: eps.PDNTypeIPv4, AccessPointName: apnIE,
	}).Marshal()
	if err != nil {
		t.Fatal(err)
	}

	handlePDNConnectivityRequest(context.Background(), m, ue, connReq)

	p := ue.PdnForAPN("ims")
	if p == nil {
		t.Fatal("second PDN connection not created")
	}

	if !m.ESMGuardActiveForTest(p) {
		t.Fatal("T3485 guard not armed on additional-PDN activation; a lost ACTIVATE DEFAULT would leak the PDN")
	}

	acc, err := (&eps.ActivateDefaultEPSBearerContextAccept{EPSBearerIdentity: p.Ebi, ProcedureTransactionIdentity: 2}).Marshal()
	if err != nil {
		t.Fatal(err)
	}

	handleActivateDefaultBearerAccept(m, ue, acc)

	if m.ESMGuardActiveForTest(p) {
		t.Fatal("T3485 guard still armed after the UE accepted the default bearer; it would retransmit indefinitely")
	}
}

// TestAdditionalPDNActivationTimeoutReleasesPDN verifies that when the UE never answers,
// T3485 exhaustion releases the half-open PDN. The send chokepoint swallows write
// failures, so an unanswered activation would otherwise leak the PDN (TS 24.301 §6.4.1.6).
func TestAdditionalPDNActivationTimeoutReleasesPDN(t *testing.T) {
	m := newTestMME(t)
	m.SetESMGuardConfigForTest(5*time.Millisecond, 2)

	ue, _ := securedUE(t, m)

	testPDN(ue).Apn = "internet"

	apnIE, err := eps.MarshalAPN("ims")
	if err != nil {
		t.Fatal(err)
	}

	connReq, err := (&eps.PDNConnectivityRequest{
		ProcedureTransactionIdentity: 2, RequestType: 1, PDNType: eps.PDNTypeIPv4, AccessPointName: apnIE,
	}).Marshal()
	if err != nil {
		t.Fatal(err)
	}

	handlePDNConnectivityRequest(context.Background(), m, ue, connReq)

	if ue.PdnForAPN("ims") == nil {
		t.Fatal("second PDN connection not created")
	}

	// PDNCount takes the UE lock, so polling it is safe against the guard timer's release.
	deadline := time.Now().Add(2 * time.Second)
	for ue.PDNCount() > 1 && time.Now().Before(deadline) {
		time.Sleep(2 * time.Millisecond)
	}

	if got := ue.PDNCount(); got != 1 {
		t.Fatalf("additional PDN not released after activation timed out: PDNCount = %d, want 1 (it leaked)", got)
	}
}

// TestConnectedSubscriberReportsAllPDNs confirms the status view lists every PDN
// connection (ordered by EPS bearer identity), not just the default bearer.
func TestConnectedSubscriberReportsAllPDNs(t *testing.T) {
	m := newTestMME(t)
	ue, _ := securedUE(t, m)

	def := testPDN(ue)
	def.Apn = "internet"
	def.PdnType = eps.PDNTypeIPv4
	def.UeIP = netip.MustParseAddr("10.45.0.2")

	second := ue.EnsurePDN(6)
	second.Apn = "ims"
	second.PdnType = eps.PDNTypeIPv4
	second.UeIP = netip.MustParseAddr("10.46.0.2")

	cs, ok := m.LookupSubscriber(ue.IMSI())
	if !ok {
		t.Fatal("subscriber not found")
	}

	if cs.NumSessions != 2 || len(cs.Sessions) != 2 {
		t.Fatalf("NumSessions = %d / len(Sessions) = %d, want 2", cs.NumSessions, len(cs.Sessions))
	}

	if cs.Sessions[0].BearerID != 5 || cs.Sessions[0].APN != "internet" ||
		cs.Sessions[1].BearerID != 6 || cs.Sessions[1].APN != "ims" {
		t.Fatalf("sessions not reported in EBI order: %+v", cs.Sessions)
	}
}

// TestAdditionalPDNRejectedUnknownAPN confirms a PDN connectivity request for an
// APN outside the subscriber's profile is rejected (TS 24.301 ESM cause #27).
func TestAdditionalPDNRejectedUnknownAPN(t *testing.T) {
	m := newTestMME(t)
	ue, cc := securedUE(t, m)

	p0 := testPDN(ue)
	p0.Apn = "internet"

	apnIE, err := eps.MarshalAPN("enterprise")
	if err != nil {
		t.Fatal(err)
	}

	connReq, err := (&eps.PDNConnectivityRequest{
		ProcedureTransactionIdentity: 4, RequestType: 1, PDNType: eps.PDNTypeIPv4, AccessPointName: apnIE,
	}).Marshal()
	if err != nil {
		t.Fatal(err)
	}

	handlePDNConnectivityRequest(context.Background(), m, ue, connReq)

	if ue.PdnForAPN("enterprise") != nil {
		t.Fatal("PDN created for an APN not in the profile")
	}

	reject, err := eps.ParsePDNConnectivityReject(lastDownlinkESM(t, ue, cc))
	if err != nil {
		t.Fatalf("expected a PDN Connectivity Reject: %v", err)
	}

	if reject.ESMCause != esmCauseUnknownAPN {
		t.Fatalf("ESM cause = %d, want %d (unknown APN)", reject.ESMCause, esmCauseUnknownAPN)
	}
}

// TestPDNConnectivityRejectedInvalidHeader confirms the MME validates the ESM
// header of a PDN Connectivity Request (TS 24.301 §7.3): an unassigned/reserved
// PTI is rejected with ESM cause #81, a non-zero header EBI with #43.
func TestPDNConnectivityRejectedInvalidHeader(t *testing.T) {
	apnIE, err := eps.MarshalAPN("ims")
	if err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name      string
		req       *eps.PDNConnectivityRequest
		wantCause uint8
	}{
		{"unassigned PTI", &eps.PDNConnectivityRequest{ProcedureTransactionIdentity: 0, RequestType: 1, PDNType: eps.PDNTypeIPv4, AccessPointName: apnIE}, esmCauseInvalidPTIValue},
		{"reserved PTI", &eps.PDNConnectivityRequest{ProcedureTransactionIdentity: 255, RequestType: 1, PDNType: eps.PDNTypeIPv4, AccessPointName: apnIE}, esmCauseInvalidPTIValue},
		{"assigned header EBI", &eps.PDNConnectivityRequest{EPSBearerIdentity: 5, ProcedureTransactionIdentity: 2, RequestType: 1, PDNType: eps.PDNTypeIPv4, AccessPointName: apnIE}, esmCauseInvalidEPSBearerIdentity},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := newTestMME(t)
			ue, cc := securedUE(t, m)
			testPDN(ue).Apn = "internet"

			plain, err := tc.req.Marshal()
			if err != nil {
				t.Fatal(err)
			}

			handlePDNConnectivityRequest(context.Background(), m, ue, plain)

			if ue.PdnForAPN("ims") != nil {
				t.Fatal("PDN created despite an invalid ESM header")
			}

			reject, err := eps.ParsePDNConnectivityReject(lastDownlinkESM(t, ue, cc))
			if err != nil {
				t.Fatalf("expected a PDN Connectivity Reject: %v", err)
			}

			if reject.ESMCause != tc.wantCause {
				t.Fatalf("ESM cause = %d, want %d", reject.ESMCause, tc.wantCause)
			}
		})
	}
}

// TestPDNDisconnectRejectedInvalidHeader confirms the same §7.3 header validation
// for a PDN Disconnect Request, before the linked-bearer and last-PDN checks.
func TestPDNDisconnectRejectedInvalidHeader(t *testing.T) {
	cases := []struct {
		name      string
		req       *eps.PDNDisconnectRequest
		wantCause uint8
	}{
		{"unassigned PTI", &eps.PDNDisconnectRequest{ProcedureTransactionIdentity: 0, LinkedEPSBearerIdentity: 5}, esmCauseInvalidPTIValue},
		{"assigned header EBI", &eps.PDNDisconnectRequest{EPSBearerIdentity: 5, ProcedureTransactionIdentity: 3, LinkedEPSBearerIdentity: 5}, esmCauseInvalidEPSBearerIdentity},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := newTestMME(t)
			ue, cc := securedUE(t, m)
			testPDN(ue).Apn = "internet"

			plain, err := tc.req.Marshal()
			if err != nil {
				t.Fatal(err)
			}

			handlePDNDisconnectRequest(context.Background(), m, ue, plain)

			reject, err := eps.ParsePDNDisconnectReject(lastDownlinkESM(t, ue, cc))
			if err != nil {
				t.Fatalf("expected a PDN Disconnect Reject: %v", err)
			}

			if reject.ESMCause != tc.wantCause {
				t.Fatalf("ESM cause = %d, want %d", reject.ESMCause, tc.wantCause)
			}
		})
	}
}

// TestLastPDNDisconnectRejected confirms the UE cannot disconnect its only PDN
// connection (TS 24.301 ESM cause #49); it must detach instead.
func TestLastPDNDisconnectRejected(t *testing.T) {
	m := newTestMME(t)
	ue, cc := securedUE(t, m)

	p0 := testPDN(ue)
	p0.Apn = "internet"

	dis, err := (&eps.PDNDisconnectRequest{ProcedureTransactionIdentity: 5, LinkedEPSBearerIdentity: mme.DefaultERABID}).Marshal()
	if err != nil {
		t.Fatal(err)
	}

	handlePDNDisconnectRequest(context.Background(), m, ue, dis)

	if m.DefaultPDN(ue) == nil {
		t.Fatal("the only PDN was disconnected; expected it to be retained")
	}

	reject, err := eps.ParsePDNDisconnectReject(lastDownlinkESM(t, ue, cc))
	if err != nil {
		t.Fatalf("expected a PDN Disconnect Reject: %v", err)
	}

	if reject.ESMCause != esmCauseLastPDNDisconnectNotAllowed {
		t.Fatalf("ESM cause = %d, want %d (last PDN disconnect not allowed)", reject.ESMCause, esmCauseLastPDNDisconnectNotAllowed)
	}
}
