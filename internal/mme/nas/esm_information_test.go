// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/nas/eps"
)

// A UE that needs to send a ciphered PCO or an APN during attach sets the ESM
// information transfer flag and waits to be asked (TS 24.301 §6.5.1.2 NOTE 1).
// Its PDU session identity arrives in the response, so without the procedure the
// PDN connection has none and cannot be transferred to 5GS.
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
		t.Fatal("the deferred request carried no identity, yet one was recorded")
	}

	sent := cc.count()

	// The default bearer cannot be activated yet: the APN and the identity are
	// still with the UE.
	activateDefaultBearer(context.Background(), m, ue)

	if cc.count() != sent+1 {
		t.Fatalf("sent %d messages, want the ESM Information Request", cc.count()-sent)
	}

	// The response's PCO replaces anything the request carried (§6.6.1.2.4).
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
		t.Errorf("PDU session identity = %d, want 9 from the ESM Information Response", ue.RequestedPDUSessionID)
	}

	if ue.RequestedAPN != "internet" {
		t.Errorf("APN = %q, want the one the response carried", ue.RequestedAPN)
	}
}

// An unsolicited response is ignored rather than resuming an attach that never
// deferred anything.
func TestESMInformationResponseWhenNoneWasRequested(t *testing.T) {
	m := newTestMME(t)
	ue, _ := securedUE(t, m)

	ue.AwaitingESMInformation = false
	ue.RequestedPDUSessionID = 4

	handleESMInformationResponse(context.Background(), m, ue, &eps.ESMInformationResponse{PTI: 1})

	if ue.RequestedPDUSessionID != 4 {
		t.Errorf("PDU session identity = %d, want it untouched by an unsolicited response", ue.RequestedPDUSessionID)
	}
}
