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

func TestESMInformationCarriesTheDeferredIdentity(t *testing.T) {
	m := newTestMME(t)
	ue, cc := securedUE(t, m)

	eit := true

	esm, err := (&eps.PDNConnectivityRequest{
		PTI: 3, RequestType: eps.RequestTypeHandover, PDNType: eps.PDNTypeIPv4,
		ESMInformationTransferFlag: &eit,
	}).MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}

	ingestAttachRequest(context.Background(), ue, &eps.AttachRequest{ESMMessageContainer: esm})

	if !ue.AwaitingESMInformation() {
		t.Fatal("the ESM information transfer flag was not recorded")
	}

	if ue.RequestedPDUSessionID != 0 {
		t.Fatalf("RequestedPDUSessionID = %d, want 0", ue.RequestedPDUSessionID)
	}

	sent := cc.count()

	activateDefaultBearer(context.Background(), m, ue)

	if cc.count() != sent+1 {
		t.Fatalf("sent %d messages, want 1", cc.count()-sent)
	}

	pco := nas.ProtocolConfigurationOptions{
		ConfigProtocol: nas.PCOConfigProtocolPPP,
		Direction:      nas.PCOMSToNetwork,
		Containers:     []nas.PCOContainer{{ID: nas.PCOContainerPDUSessionID, Content: []byte{9}}},
	}
	apn := eps.APN("internet")

	handleESMInformationResponse(context.Background(), m, ue, &eps.ESMInformationResponse{
		PTI:                          3,
		AccessPointName:              &apn,
		ProtocolConfigurationOptions: &pco,
	})

	if ue.AwaitingESMInformation() {
		t.Error("still awaiting ESM information after the response")
	}

	if ue.RequestedPDUSessionID != 9 {
		t.Errorf("PDU session identity = %d, want 9", ue.RequestedPDUSessionID)
	}

	if ue.RequestedAPN != "internet" {
		t.Errorf("APN = %q, want %q", ue.RequestedAPN, "internet")
	}
}

func TestESMInformationResponseWhenNoneWasRequested(t *testing.T) {
	m := newTestMME(t)
	ue, _ := securedUE(t, m)

	ue.TakeESMInfoWait()
	ue.RequestedPDUSessionID = 4

	handleESMInformationResponse(context.Background(), m, ue, &eps.ESMInformationResponse{PTI: 1})

	if ue.RequestedPDUSessionID != 4 {
		t.Errorf("PDU session identity = %d, want 4", ue.RequestedPDUSessionID)
	}
}

// TS 24.301 §6.5.1.2: a standalone PDN CONNECTIVITY REQUEST may defer its APN to
// the ESM information exchange, and §6.5.1.6 c) rejects #53 only after T3489.
func TestStandalonePDNConnectivityDefersToESMInformation(t *testing.T) {
	m := newTestMME(t)
	ue, cc := securedUE(t, m)
	ue.ForceStateForTest(mme.EMMRegistered)

	eit := true

	sent := cc.count()

	handlePDNConnectivityRequest(context.Background(), m, ue, &eps.PDNConnectivityRequest{
		PTI: 4, RequestType: eps.RequestTypeInitialRequest, PDNType: eps.PDNTypeIPv4,
		ESMInformationTransferFlag: &eit,
	})

	if !ue.AwaitingESMInformation() {
		t.Fatal("the standalone request did not start the ESM information procedure")
	}

	if !ue.AwaitingESMInformation() {
		t.Fatal("the standalone request was not recorded for resume")
	}

	if cc.count() == sent {
		t.Fatal("no ESM Information Request was sent")
	}
}

// TS 24.301 §6.6.1.2.6 a): T3489 retransmits on the first and second expiry and
// aborts the procedure on the third, rejecting the attach with ESM cause #53.
func TestT3489ThirdExpiryRejectsTheAttach(t *testing.T) {
	m := newTestMME(t)
	m.SetT3489ForTest(2*time.Millisecond, 2)

	ue, cc := securedUE(t, m)

	eit := true

	esm, err := (&eps.PDNConnectivityRequest{
		PTI: 3, RequestType: eps.RequestTypeInitialRequest, PDNType: eps.PDNTypeIPv4,
		ESMInformationTransferFlag: &eit,
	}).MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}

	ingestAttachRequest(context.Background(), ue, &eps.AttachRequest{ESMMessageContainer: esm})
	activateDefaultBearer(context.Background(), m, ue)

	var (
		reject        *eps.AttachReject
		beforeMessage int
	)

	for range 200 {
		time.Sleep(5 * time.Millisecond)

		for i, sent := range cc.snapshot() {
			plain, err := eps.ParseAttachReject(unprotectDownlink(t, ue, decodeDownlinkNAS(t, sent)))
			if err == nil {
				reject, beforeMessage = plain, i

				break
			}
		}

		if reject != nil {
			break
		}
	}

	if reject == nil {
		t.Fatal("T3489 never aborted the procedure with an Attach Reject")
	}

	// The request itself plus a retransmission on the first and second expiry.
	if beforeMessage != 3 {
		t.Errorf("%d messages preceded the reject, want 3 ESM Information Requests", beforeMessage)
	}

	if reject.Cause != eps.EMMCauseESMFailure {
		t.Errorf("EMM cause = %d, want #19 ESM failure", reject.Cause)
	}

	esmReject, err := eps.ParsePDNConnectivityReject(reject.ESMMessageContainer)
	if err != nil {
		t.Fatalf("Attach Reject carries no PDN Connectivity Reject: %v", err)
	}

	if esmReject.Cause != eps.ESMCauseESMInformationNotReceived {
		t.Errorf("ESM cause = %s, want #53 ESM information not received", esmReject.Cause)
	}
}

// TS 24.301 §6.5.1.2: the identity may arrive in either configuration-options
// container, and §6.6.1.2.4 has the response replace what the request carried.
func TestESMInformationIdentityFromExtendedPCO(t *testing.T) {
	m := newTestMME(t)
	ue, _ := securedUE(t, m)

	ue.AwaitESMInformation(3, nil)
	ue.RequestedPTI = 3

	epco := nas.ProtocolConfigurationOptions{
		Direction:  nas.PCOMSToNetwork,
		Containers: []nas.PCOContainer{{ID: nas.PCOContainerPDUSessionID, Content: []byte{9}}},
	}

	apn := eps.APN("internet")

	handleESMInformationResponse(context.Background(), m, ue, &eps.ESMInformationResponse{
		PTI:                                  3,
		AccessPointName:                      &apn,
		ExtendedProtocolConfigurationOptions: &epco,
	})

	if ue.RequestedPDUSessionID != 9 {
		t.Errorf("RequestedPDUSessionID = %d, want 9 from the extended container", ue.RequestedPDUSessionID)
	}
}

// TS 24.301 §7.3.1 e): an assigned PTI naming no ongoing transaction draws ESM
// STATUS #81, and §8.3.15 has it name the transaction it refuses. §7.3.2 e)
// ignores a response carrying an assigned EPS bearer identity.
func TestESMInformationResponsePTIAndEBIHandling(t *testing.T) {
	m := newTestMME(t)
	ue, cc := securedUE(t, m)

	ue.AwaitESMInformation(3, nil)
	ue.RequestedPTI = 3

	// An assigned EBI is ignored: no status, and the wait survives.
	sent := cc.count()

	handleESMInformationResponse(context.Background(), m, ue, &eps.ESMInformationResponse{PTI: 3, EPSBearerIdentity: 5})

	if cc.count() != sent {
		t.Error("a response with an assigned EPS bearer identity drew a reply, want it ignored")
	}

	if !ue.AwaitingESMInformation() {
		t.Error("a response with an assigned EPS bearer identity ended the wait")
	}

	// An unassigned PTI is ignored outright.
	handleESMInformationResponse(context.Background(), m, ue, &eps.ESMInformationResponse{PTI: 0})

	if cc.count() != sent {
		t.Error("a response with an unassigned PTI drew a reply, want it ignored")
	}

	// An assigned PTI naming no transaction draws #81, carrying that PTI.
	handleESMInformationResponse(context.Background(), m, ue, &eps.ESMInformationResponse{PTI: 9})

	if cc.count() != sent+1 {
		t.Fatalf("messages sent = %d, want one ESM STATUS", cc.count()-sent)
	}

	status, err := eps.ParseESMStatus(unprotectDownlink(t, ue, decodeDownlinkNAS(t, cc.snapshot()[sent])))
	if err != nil {
		t.Fatalf("not an ESM STATUS: %v", err)
	}

	if status.Cause != eps.ESMCauseInvalidPTIValue {
		t.Errorf("ESM cause = %s, want #81 invalid PTI value", status.Cause)
	}

	if uint8(status.PTI) != 9 {
		t.Errorf("ESM STATUS PTI = %d, want the offending 9", uint8(status.PTI))
	}
}

// C7's shape: a standalone request carrying the identity only in the extended
// element must not inherit the previous request's value (TS 24.301 §6.5.1.2).
func TestStandaloneRequestReadsTheIdentityFromTheExtendedPCO(t *testing.T) {
	m := newTestMME(t)
	ue, _ := securedUE(t, m)
	ue.ForceStateForTest(mme.EMMRegistered)

	// A previous request left an identity behind.
	ue.RequestedPDUSessionID = 7

	epco := nas.ProtocolConfigurationOptions{
		Direction:  nas.PCOMSToNetwork,
		Containers: []nas.PCOContainer{{ID: nas.PCOContainerPDUSessionID, Content: []byte{9}}},
	}

	apn := eps.APN("internet")

	handlePDNConnectivityRequest(context.Background(), m, ue, &eps.PDNConnectivityRequest{
		PTI: 4, RequestType: eps.RequestTypeInitialRequest, PDNType: eps.PDNTypeIPv4,
		AccessPointName:                      &apn,
		ExtendedProtocolConfigurationOptions: &epco,
	})

	if ue.RequestedPDUSessionID != 9 {
		t.Errorf("RequestedPDUSessionID = %d, want 9 from the extended element", ue.RequestedPDUSessionID)
	}
}

// The deferred standalone request resumes on the response, using the APN the UE
// held back (TS 24.301 §6.6.1.2.4), not the attach path.
func TestStandaloneDeferralResumesWithTheDeferredAPN(t *testing.T) {
	m := newTestMME(t)
	ue, _ := securedUE(t, m)
	ue.ForceStateForTest(mme.EMMRegistered)

	eit := true

	handlePDNConnectivityRequest(context.Background(), m, ue, &eps.PDNConnectivityRequest{
		PTI: 4, RequestType: eps.RequestTypeInitialRequest, PDNType: eps.PDNTypeIPv4,
		ESMInformationTransferFlag: &eit,
	})

	if !ue.AwaitingESMInformation() {
		t.Fatal("the standalone request did not record a deferral")
	}

	apn := eps.APN("internet")

	epco := nas.ProtocolConfigurationOptions{
		Direction:  nas.PCOMSToNetwork,
		Containers: []nas.PCOContainer{{ID: nas.PCOContainerPDUSessionID, Content: []byte{3}}},
	}

	handleESMInformationResponse(context.Background(), m, ue, &eps.ESMInformationResponse{
		PTI: 4, AccessPointName: &apn, ExtendedProtocolConfigurationOptions: &epco,
	})

	if ue.AwaitingESMInformation() {
		t.Error("AwaitingESMInformation = true after the response, want false")
	}

	if ue.RequestedAPN != "internet" {
		t.Errorf("RequestedAPN = %q, want the deferred \"internet\"", ue.RequestedAPN)
	}

	// The identity the UE deferred has to reach the anchor: without it the
	// connection cannot be correlated with a PDU session.
	if ue.RequestedPDUSessionID != 3 {
		t.Errorf("RequestedPDUSessionID = %d, want the deferred 3", ue.RequestedPDUSessionID)
	}

	if len(m.SnapshotPDNs(ue)) == 0 {
		t.Error("the resumed request opened no PDN connection")
	}
}
