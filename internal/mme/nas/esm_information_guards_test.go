// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"context"
	"testing"
	"time"

	"github.com/ellanetworks/core/internal/mme"
	"github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/nas/eps"
)

// standalonePTI is the transaction deferStandaloneESMInformation runs.
const standalonePTI nas.ProcedureTransactionIdentity = 4

// deferAttachESMInformation puts a UE in the attach's ESM information exchange for
// transaction pti, with the request sent and T3489 armed.
func deferAttachESMInformation(t *testing.T, m *mme.MME, ue *mme.UeContext, pti uint8) {
	t.Helper()

	eit := true

	esm, err := (&eps.PDNConnectivityRequest{
		PTI: nas.ProcedureTransactionIdentity(pti), RequestType: eps.RequestTypeInitialRequest, PDNType: eps.PDNTypeIPv4,
		ESMInformationTransferFlag: &eit,
	}).MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}

	ingestAttachRequest(context.Background(), ue, &eps.AttachRequest{ESMMessageContainer: esm})
	activateDefaultBearer(context.Background(), m, ue)

	if !ue.AwaitingESMInformation() {
		t.Fatal("AwaitingESMInformation = false after the deferred attach, want true")
	}
}

// deferStandaloneESMInformation puts an EMM-REGISTERED UE in a standalone PDN
// CONNECTIVITY REQUEST's ESM information exchange for transaction 4.
func deferStandaloneESMInformation(t *testing.T, m *mme.MME, ue *mme.UeContext) {
	t.Helper()

	eit := true

	handlePDNConnectivityRequest(context.Background(), m, ue, &eps.PDNConnectivityRequest{
		PTI: standalonePTI, RequestType: eps.RequestTypeInitialRequest, PDNType: eps.PDNTypeIPv4,
		ESMInformationTransferFlag: &eit,
	})

	if !ue.AwaitingESMInformation() {
		t.Fatal("AwaitingESMInformation = false after the deferred request, want true")
	}
}

// TS 24.301 §7.3.1 e): a response whose assigned PTI matches no ongoing
// transaction draws ESM STATUS #81 — the network responds, it does not abort. The
// transaction the UE is actually running has to survive it.
func TestESMInformationResponseForAnotherTransactionKeepsTheProcedure(t *testing.T) {
	m := newTestMME(t)
	ue, cc := securedUE(t, m)
	ue.ForceStateForTest(mme.EMMRegistered)

	deferStandaloneESMInformation(t, m, ue)

	sent := cc.count()

	handleESMInformationResponse(context.Background(), m, ue, &eps.ESMInformationResponse{PTI: 9})

	if !ue.AwaitingESMInformation() {
		t.Error("AwaitingESMInformation = false after a response naming PTI 9, want true: the ongoing PTI 4 transaction must survive")
	}

	if cc.count() != sent+1 {
		t.Fatalf("messages sent = %d, want 1 ESM STATUS", cc.count()-sent)
	}

	status, err := eps.ParseESMStatus(unprotectDownlink(t, ue, decodeDownlinkNAS(t, cc.snapshot()[sent])))
	if err != nil {
		t.Fatalf("reply is not an ESM STATUS: %v", err)
	}

	if status.Cause != eps.ESMCauseInvalidPTIValue {
		t.Errorf("ESM cause = %s, want #81 invalid PTI value", status.Cause)
	}

	// The transaction still runs, so its own response still resumes it.
	apn := eps.APN("internet")
	handleESMInformationResponse(context.Background(), m, ue, &eps.ESMInformationResponse{PTI: 4, AccessPointName: &apn})

	if got := len(m.SnapshotPDNs(ue)); got != 1 {
		t.Errorf("PDN connections after the matching response = %d, want 1", got)
	}
}

// A second response for a transaction already concluded names no ongoing one, so
// it draws #81 (TS 24.301 §7.3.1 e) and resumes nothing.
func TestDuplicateESMInformationResponseDrawsInvalidPTI(t *testing.T) {
	m := newTestMME(t)
	ue, cc := securedUE(t, m)
	ue.ForceStateForTest(mme.EMMRegistered)

	deferStandaloneESMInformation(t, m, ue)

	apn := eps.APN("internet")
	handleESMInformationResponse(context.Background(), m, ue, &eps.ESMInformationResponse{PTI: 4, AccessPointName: &apn})

	if got := len(m.SnapshotPDNs(ue)); got != 1 {
		t.Fatalf("PDN connections after the response = %d, want 1", got)
	}

	sent := cc.count()

	handleESMInformationResponse(context.Background(), m, ue, &eps.ESMInformationResponse{PTI: 4, AccessPointName: &apn})

	if got := len(m.SnapshotPDNs(ue)); got != 1 {
		t.Errorf("PDN connections after the duplicate response = %d, want 1", got)
	}

	if cc.count() != sent+1 {
		t.Fatalf("messages sent for the duplicate = %d, want 1 ESM STATUS", cc.count()-sent)
	}

	status, err := eps.ParseESMStatus(unprotectDownlink(t, ue, decodeDownlinkNAS(t, cc.snapshot()[sent])))
	if err != nil {
		t.Fatalf("reply is not an ESM STATUS: %v", err)
	}

	if status.Cause != eps.ESMCauseInvalidPTIValue {
		t.Errorf("ESM cause = %s, want #81 invalid PTI value", status.Cause)
	}
}

// TS 24.301 §6.6.1.2.5: the response stops T3489, so no retransmission follows a
// procedure the UE has already answered.
func TestSuccessfulESMInformationResponseStopsT3489(t *testing.T) {
	m := newTestMME(t)
	m.SetT3489ForTest(2*time.Millisecond, 3)

	ue, cc := securedUE(t, m)

	deferAttachESMInformation(t, m, ue, 3)

	apn := eps.APN("internet")
	handleESMInformationResponse(context.Background(), m, ue, &eps.ESMInformationResponse{PTI: 3, AccessPointName: &apn})

	settled := cc.count()

	time.Sleep(60 * time.Millisecond)

	if cc.count() != settled {
		t.Errorf("T3489 sent %d more messages after the response, want 0", cc.count()-settled)
	}
}

// TS 24.301 §6.6.1.2.4: the response's configuration options replace what the
// request carried. A response carrying neither container leaves what the request
// carried in place.
func TestESMInformationResponseWithoutConfigurationOptionsKeepsTheIdentity(t *testing.T) {
	m := newTestMME(t)
	ue, _ := securedUE(t, m)

	ue.AwaitESMInformation(3, nil)
	ue.RequestedPTI = 3
	ue.RequestedPDUSessionID = 7

	apn := eps.APN("internet")
	handleESMInformationResponse(context.Background(), m, ue, &eps.ESMInformationResponse{PTI: 3, AccessPointName: &apn})

	if ue.RequestedPDUSessionID != 7 {
		t.Errorf("RequestedPDUSessionID = %d, want the request's 7 kept by a response carrying no configuration options", ue.RequestedPDUSessionID)
	}
}

// An exhausted downlink NAS COUNT has already released the connection (TS 33.401
// §6.5), so the procedure ends with no request sent and no supervision armed.
func TestESMInformationRequestOnAnExhaustedCountEndsTheWait(t *testing.T) {
	m := newTestMME(t)
	ue, _ := securedUE(t, m)

	ue.AwaitESMInformation(3, nil)
	ue.RequestedPTI = 3

	ue.SetDLCountForTest(0xffffff)

	if _, err := ue.ProtectDownlink([]byte{0x07, 0x42}, eps.SHTIntegrityProtectedCiphered); err != nil {
		t.Fatalf("ProtectDownlink at the last COUNT: %v", err)
	}

	aborted := false

	if !requestESMInformation(context.Background(), ue, func(uint8) { aborted = true }) {
		t.Fatal("requestESMInformation reported no wait, want true")
	}

	if aborted {
		t.Error("the abort ran on an exhausted COUNT, want it skipped: its reject cannot be protected either")
	}

	if ue.AwaitingESMInformation() {
		t.Error("AwaitingESMInformation = true after an exhausted COUNT, want false")
	}
}

// A fresh ATTACH REQUEST supersedes any earlier deferral: its wait is dropped and
// its T3489 stopped, so the abandoned procedure neither retransmits nor rejects
// the new attach (TS 24.301 §5.5.1.2.4).
func TestAttachResetsAnAbandonedDeferral(t *testing.T) {
	m := newTestMME(t)
	m.SetT3489ForTest(2*time.Millisecond, 3)

	ue, cc := securedUE(t, m)

	deferAttachESMInformation(t, m, ue, 4)

	apn := eps.APN("internet")

	second, err := (&eps.PDNConnectivityRequest{
		PTI: 7, RequestType: eps.RequestTypeInitialRequest, PDNType: eps.PDNTypeIPv4,
		AccessPointName: &apn,
	}).MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}

	ingestAttachRequest(context.Background(), ue, &eps.AttachRequest{ESMMessageContainer: second})

	if ue.AwaitingESMInformation() {
		t.Error("AwaitingESMInformation = true after an attach carrying no deferral, want false")
	}

	settled := cc.count()

	time.Sleep(60 * time.Millisecond)

	if cc.count() != settled {
		t.Errorf("the abandoned T3489 sent %d more messages, want 0", cc.count()-settled)
	}
}

// TS 24.301 §6.6.1.2.6 a): the third T3489 expiry aborts the standalone request.
// The wait goes with it, so a response arriving afterwards opens no connection.
func TestStandaloneESMInformationTimeoutEndsTheWait(t *testing.T) {
	m := newTestMME(t)
	m.SetT3489ForTest(2*time.Millisecond, 1)

	ue, _ := securedUE(t, m)
	ue.ForceStateForTest(mme.EMMRegistered)

	deferStandaloneESMInformation(t, m, ue)

	waitForNoESMInformation(t, ue)

	apn := eps.APN("internet")
	handleESMInformationResponse(context.Background(), m, ue, &eps.ESMInformationResponse{PTI: 4, AccessPointName: &apn})

	if got := len(m.SnapshotPDNs(ue)); got != 0 {
		t.Errorf("PDN connections after a response to an aborted procedure = %d, want 0", got)
	}
}

// The attach's abort ends the wait too, so a response arriving after the ATTACH
// REJECT establishes nothing (TS 24.301 §6.6.1.2.6 a).
func TestAttachESMInformationTimeoutEndsTheWait(t *testing.T) {
	m := newTestMME(t)
	m.SetT3489ForTest(2*time.Millisecond, 1)

	sm := &fakeSessionManager{}
	m.Session = sm

	ue, _ := securedUE(t, m)

	deferAttachESMInformation(t, m, ue, 3)

	waitForNoESMInformation(t, ue)

	apn := eps.APN("internet")
	handleESMInformationResponse(context.Background(), m, ue, &eps.ESMInformationResponse{PTI: 3, AccessPointName: &apn})

	if sm.lastRequest.IMSI != "" {
		t.Errorf("an EPS session was created for the rejected attach (IMSI %q), want none", sm.lastRequest.IMSI)
	}
}

// A DETACH REQUEST ends any outstanding ESM information exchange, so a response
// arriving after DETACH ACCEPT establishes nothing (TS 23.401 §5.3.8.2.1).
func TestDetachEndsTheESMInformationProcedure(t *testing.T) {
	m := newTestMME(t)
	sm := &fakeSessionManager{}
	m.Session = sm

	ue, _ := securedUE(t, m)
	ue.ForceStateForTest(mme.EMMRegistered)

	deferStandaloneESMInformation(t, m, ue)

	handleDetachRequest(context.Background(), m, ue, &eps.DetachRequestUE{}, true)

	if ue.AwaitingESMInformation() {
		t.Error("AwaitingESMInformation = true after the DETACH REQUEST, want false")
	}

	apn := eps.APN("internet")
	handleESMInformationResponse(context.Background(), m, ue, &eps.ESMInformationResponse{PTI: 4, AccessPointName: &apn})

	if sm.lastRequest.IMSI != "" {
		t.Errorf("an EPS session was created for the detached UE (IMSI %q), want none", sm.lastRequest.IMSI)
	}

	if got := len(m.SnapshotPDNs(ue)); got != 0 {
		t.Errorf("PDN connections after the detach = %d, want 0", got)
	}
}

// The commit half re-checks the liveness the parse half checked: a standalone
// request requires an EMM-REGISTERED, connected UE (TS 24.301 §6.5.1.1), and the
// response can arrive after the UE has left that state.
func TestDeferredPDNConnectivityAbandonedAfterDeregistration(t *testing.T) {
	m := newTestMME(t)
	sm := &fakeSessionManager{}
	m.Session = sm

	ue, _ := securedUE(t, m)
	ue.ForceStateForTest(mme.EMMRegistered)

	deferStandaloneESMInformation(t, m, ue)

	ue.ForceStateForTest(mme.EMMDeregistered)

	apn := eps.APN("internet")
	handleESMInformationResponse(context.Background(), m, ue, &eps.ESMInformationResponse{PTI: 4, AccessPointName: &apn})

	if sm.lastRequest.IMSI != "" {
		t.Errorf("an EPS session was created for a deregistered UE (IMSI %q), want none", sm.lastRequest.IMSI)
	}

	if got := len(m.SnapshotPDNs(ue)); got != 0 {
		t.Errorf("PDN connections opened for a deregistered UE = %d, want 0", got)
	}
}

// The attach's commit half checks the same liveness: a response resuming it after
// the UE has deregistered must activate no default bearer.
func TestDeferredAttachAbandonedAfterDeregistration(t *testing.T) {
	m := newTestMME(t)
	sm := &fakeSessionManager{}
	m.Session = sm

	ue, _ := securedUE(t, m)

	deferAttachESMInformation(t, m, ue, 3)

	ue.ForceStateForTest(mme.EMMDeregistered)

	apn := eps.APN("internet")
	handleESMInformationResponse(context.Background(), m, ue, &eps.ESMInformationResponse{PTI: 3, AccessPointName: &apn})

	if sm.lastRequest.IMSI != "" {
		t.Errorf("an EPS session was created for a deregistered UE (IMSI %q), want none", sm.lastRequest.IMSI)
	}

	if m.DefaultPDN(ue) != nil {
		t.Error("a default bearer was installed for a deregistered UE, want none")
	}
}

// waitForNoESMInformation blocks until the UE's ESM information exchange has ended.
func waitForNoESMInformation(t *testing.T, ue *mme.UeContext) {
	t.Helper()

	for range 200 {
		if !ue.AwaitingESMInformation() {
			return
		}

		time.Sleep(5 * time.Millisecond)
	}

	t.Fatal("AwaitingESMInformation = true after T3489 expired, want false")
}
