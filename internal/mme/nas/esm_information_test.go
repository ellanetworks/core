// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"context"
	"testing"

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
