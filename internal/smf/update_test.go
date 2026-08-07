// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package smf

import (
	"testing"

	"github.com/ellanetworks/core/internal/models"
	libngap "github.com/ellanetworks/core/ngap"
)

const testHandoverTEID = 0x1234

// validHandoverRequestAcknowledgeTransfer marshals a transfer carrying an IPv4
// downlink GTP tunnel with TEID testHandoverTEID.
func validHandoverRequestAcknowledgeTransfer(t *testing.T) []byte {
	t.Helper()

	transfer := libngap.HandoverRequestAcknowledgeTransfer{
		DLNGUUPTNLInformation: libngap.UPTransportLayerInformation{GTPTunnel: libngap.GTPTunnel{
			TransportLayerAddress: libngap.TransportLayerAddress{10, 0, 0, 1},
			GTPTEID:               libngap.GTPTEID(testHandoverTEID),
		}},
		QosFlowSetupResponse: libngap.QosFlowListWithDataForwarding{{QosFlowIdentifier: 1}},
	}

	b, err := transfer.Marshal()
	if err != nil {
		t.Fatalf("marshal transfer: %v", err)
	}

	return b
}

// An activated data path whose downlink FAR has no forwarding parameters must
// gain them from the handover target's tunnel, without panicking.
func TestHandleHandoverRequestAcknowledgeTransfer_ActivatedNilForwarding(t *testing.T) {
	dlFAR := &FAR{}
	smContext := &SMContext{
		Tunnel: &UPTunnel{
			Activated:   true,
			DownlinkPDR: &PDR{FAR: dlFAR},
			UplinkPDR:   &PDR{},
		},
	}

	if err := handleHandoverRequestAcknowledgeTransfer(validHandoverRequestAcknowledgeTransfer(t), smContext); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if smContext.Tunnel.AN.TEID != testHandoverTEID {
		t.Errorf("AN.TEID = %#x, want %#x", smContext.Tunnel.AN.TEID, testHandoverTEID)
	}

	if dlFAR.ForwardingParameters == nil || dlFAR.ForwardingParameters.OuterHeaderCreation == nil {
		t.Fatal("downlink FAR forwarding parameters were not populated")
	}

	ohc := dlFAR.ForwardingParameters.OuterHeaderCreation
	if ohc.TEID != testHandoverTEID {
		t.Errorf("OuterHeaderCreation.TEID = %#x, want %#x", ohc.TEID, testHandoverTEID)
	}

	if ohc.Description != models.OuterHeaderCreationGtpUUdpIpv4 {
		t.Errorf("OuterHeaderCreation.Description = %v, want IPv4 GTP-U", ohc.Description)
	}
}

// With no active data path the tunnel endpoint is recorded but no FAR is
// touched, so a nil downlink tunnel must not panic.
func TestHandleHandoverRequestAcknowledgeTransfer_NotActivated(t *testing.T) {
	smContext := &SMContext{Tunnel: &UPTunnel{}}

	if err := handleHandoverRequestAcknowledgeTransfer(validHandoverRequestAcknowledgeTransfer(t), smContext); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if smContext.Tunnel.AN.TEID != testHandoverTEID {
		t.Errorf("AN.TEID = %#x, want %#x", smContext.Tunnel.AN.TEID, testHandoverTEID)
	}
}

// Undecodable input is rejected with an error rather than a panic.
func TestHandleHandoverRequestAcknowledgeTransfer_BadInput(t *testing.T) {
	smContext := &SMContext{Tunnel: &UPTunnel{}}

	if err := handleHandoverRequestAcknowledgeTransfer([]byte{0xff, 0xff}, smContext); err == nil {
		t.Fatal("expected an error for undecodable transfer, got nil")
	}
}
