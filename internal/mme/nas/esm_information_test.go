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

	if !ue.AwaitingESMInformation {
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

	if ue.AwaitingESMInformation {
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

	ue.AwaitingESMInformation = false
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

	if !ue.AwaitingESMInformation {
		t.Fatal("the standalone request did not start the ESM information procedure")
	}

	if ue.PendingPDN == nil {
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
