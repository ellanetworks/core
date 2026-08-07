// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/internal/mme"
	"github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/nas/eps"
)

// openTestPDN runs a standalone PDN CONNECTIVITY REQUEST for APN "internet" to
// completion of the network half, leaving it awaiting the ACTIVATE ACCEPT.
func openTestPDN(t *testing.T, m *mme.MME, ue *mme.UeContext, pti uint8, requestType eps.RequestType) {
	t.Helper()

	name := eps.APN("internet")

	handlePDNConnectivityRequest(context.Background(), m, ue, &eps.PDNConnectivityRequest{
		PTI: nas.ProcedureTransactionIdentity(pti), RequestType: requestType, PDNType: eps.PDNTypeIPv4,
		AccessPointName: &name,
	})
}

// TS 24.301 §6.5.1.6 a) matches a retransmission on the request's own transaction:
// a different PTI naming the same APN and PDN type is a new request.
func TestRetransmissionRequiresTheSamePTI(t *testing.T) {
	m := newTestMME(t)
	ue, _ := securedUE(t, m)
	ue.ForceStateForTest(mme.EMMRegistered)

	openTestPDN(t, m, ue, 4, eps.RequestTypeInitialRequest)

	if got := len(m.SnapshotPDNs(ue)); got != 1 {
		t.Fatalf("PDN connections after the request = %d, want 1", got)
	}

	if prev, _ := m.FindRetransmittedPDN(ue, 5, "internet", eps.PDNTypeIPv4); prev != nil {
		t.Errorf("PTI 5 matched EBI %d, opened under PTI 4, as a retransmission; want no match", prev.Ebi)
	}

	if prev, _ := m.FindRetransmittedPDN(ue, 4, "internet", eps.PDNTypeIPv4); prev == nil {
		t.Error("PTI 4 matched nothing, want the connection it opened")
	}
}

// TS 24.301 §6.5.1.6 a): the retransmission is answered by resending the ACTIVATE
// DEFAULT EPS BEARER CONTEXT REQUEST and continuing the previous procedure, not by
// opening a second connection or refusing the APN.
func TestRetransmittedPDNConnectivityResendsTheActivate(t *testing.T) {
	m := newTestMME(t)
	ue, cc := securedUE(t, m)
	ue.ForceStateForTest(mme.EMMRegistered)

	openTestPDN(t, m, ue, 4, eps.RequestTypeInitialRequest)

	pdns := m.SnapshotPDNs(ue)
	if len(pdns) != 1 {
		t.Fatalf("PDN connections after the request = %d, want 1", len(pdns))
	}

	sent := cc.count()

	openTestPDN(t, m, ue, 4, eps.RequestTypeInitialRequest)

	if got := len(m.SnapshotPDNs(ue)); got != 1 {
		t.Errorf("PDN connections after the retransmission = %d, want 1", got)
	}

	if cc.count() != sent+1 {
		t.Fatalf("messages sent for the retransmission = %d, want 1", cc.count()-sent)
	}

	activate, err := eps.ParseActivateDefaultEPSBearerContextRequest(
		unprotectDownlink(t, ue, decodeDownlinkNAS(t, cc.snapshot()[sent])))
	if err != nil {
		t.Fatalf("the resent message is not an ACTIVATE DEFAULT EPS BEARER CONTEXT REQUEST: %v", err)
	}

	if uint8(activate.EPSBearerIdentity) != pdns[0].Ebi {
		t.Errorf("resent activate names EBI %d, want the previous procedure's %d", uint8(activate.EPSBearerIdentity), pdns[0].Ebi)
	}
}

// A "handover" request type names a PDU session the UE already holds in 5GS
// (TS 23.502 §4.11.2.2 step 13), not a second connection to the APN, so ESM cause
// #55 does not apply to it (TS 24.301 §6.5.1.6 a).
func TestHandoverRequestTypeBypassesAPNAlreadyConnected(t *testing.T) {
	m := newTestMME(t)
	ue, _ := securedUE(t, m)
	ue.ForceStateForTest(mme.EMMRegistered)

	openTestPDN(t, m, ue, 4, eps.RequestTypeInitialRequest)

	pdns := m.SnapshotPDNs(ue)
	if len(pdns) != 1 {
		t.Fatalf("PDN connections after the request = %d, want 1", len(pdns))
	}

	// The accept drops the retained request, so the next one cannot match as a
	// retransmission (TS 24.007 §11.2.3.1a).
	handleActivateDefaultBearerAccept(m, ue, &eps.ActivateDefaultEPSBearerContextAccept{
		EPSBearerIdentity: eps.EPSBearerIdentity(pdns[0].Ebi),
	})

	openTestPDN(t, m, ue, 6, eps.RequestTypeHandover)

	if got := len(m.SnapshotPDNs(ue)); got != 2 {
		t.Errorf("PDN connections after the handover request = %d, want 2", got)
	}
}

// A concurrent 5GS establishment supersedes the anchor session between it
// answering and the connection being committed (TS 23.502 §4.11.2.3). Activating
// the bearer then advertises an address whose lease is back in the pool.
func TestPDNConnectivityRefusedWhenTheAnchorMovedTheSessionOffEPS(t *testing.T) {
	m := newTestMME(t)
	m.Session = &fakeSessionManager{movedOffEPS: true}

	ue, cc := securedUE(t, m)
	ue.ForceStateForTest(mme.EMMRegistered)

	sent := cc.count()

	openTestPDN(t, m, ue, 4, eps.RequestTypeInitialRequest)

	if got := len(m.SnapshotPDNs(ue)); got != 0 {
		t.Errorf("PDN connections committed over a session the anchor does not hold on EPS = %d, want 0", got)
	}

	if cc.count() != sent+1 {
		t.Fatalf("messages sent = %d, want 1 PDN CONNECTIVITY REJECT", cc.count()-sent)
	}

	if _, err := eps.ParsePDNConnectivityReject(unprotectDownlink(t, ue, decodeDownlinkNAS(t, cc.snapshot()[sent]))); err != nil {
		t.Fatalf("reply is not a PDN CONNECTIVITY REJECT: %v", err)
	}
}

// The attach's commit half checks the same thing: no default bearer is installed
// over a session the anchor has stopped holding on EPS.
func TestAttachRefusedWhenTheAnchorMovedTheSessionOffEPS(t *testing.T) {
	m := newTestMME(t)
	m.Session = &fakeSessionManager{movedOffEPS: true}

	ue, cc := securedUE(t, m)

	sent := cc.count()

	activateDefaultBearer(context.Background(), m, ue)

	if m.DefaultPDN(ue) != nil {
		t.Error("a default bearer was installed over a session the anchor does not hold on EPS, want none")
	}

	if cc.count() == sent {
		t.Fatal("no message sent, want an ATTACH REJECT")
	}

	if _, err := eps.ParseAttachReject(unprotectDownlink(t, ue, decodeDownlinkNAS(t, cc.snapshot()[sent]))); err != nil {
		t.Fatalf("reply is not an ATTACH REJECT: %v", err)
	}
}
