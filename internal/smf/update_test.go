// Copyright 2026 Ella Networks
// SPDX-License-Identifier: Apache-2.0

package smf

import (
	"encoding/binary"
	"testing"

	"github.com/ellanetworks/core/internal/models"
	"github.com/free5gc/aper"
	"github.com/free5gc/ngap/ngapType"
)

const testHandoverTEID = 0x1234

// validHandoverRequestAcknowledgeTransfer marshals a transfer carrying an IPv4
// downlink GTP tunnel with TEID testHandoverTEID.
func validHandoverRequestAcknowledgeTransfer(t *testing.T) []byte {
	t.Helper()

	teid := make([]byte, 4)
	binary.BigEndian.PutUint32(teid, testHandoverTEID)

	transfer := ngapType.HandoverRequestAcknowledgeTransfer{}
	transfer.DLNGUUPTNLInformation.Present = ngapType.UPTransportLayerInformationPresentGTPTunnel
	transfer.DLNGUUPTNLInformation.GTPTunnel = &ngapType.GTPTunnel{
		TransportLayerAddress: ngapType.TransportLayerAddress{
			Value: aper.BitString{Bytes: []byte{10, 0, 0, 1}, BitLength: 32},
		},
		GTPTEID: ngapType.GTPTEID{Value: teid},
	}
	transfer.QosFlowSetupResponseList.List = append(transfer.QosFlowSetupResponseList.List,
		ngapType.QosFlowItemWithDataForwarding{QosFlowIdentifier: ngapType.QosFlowIdentifier{Value: 1}})

	b, err := aper.MarshalWithParams(transfer, "valueExt")
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
			DataPath: &DataPath{
				Activated:      true,
				DownLinkTunnel: &GTPTunnel{PDR: &PDR{FAR: dlFAR}},
				UpLinkTunnel:   &GTPTunnel{PDR: &PDR{}},
			},
		},
	}

	if err := handleHandoverRequestAcknowledgeTransfer(validHandoverRequestAcknowledgeTransfer(t), smContext); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if smContext.Tunnel.ANInformation.TEID != testHandoverTEID {
		t.Errorf("ANInformation.TEID = %#x, want %#x", smContext.Tunnel.ANInformation.TEID, testHandoverTEID)
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

	if dlFAR.State != RuleUpdate {
		t.Errorf("downlink FAR State = %v, want RuleUpdate", dlFAR.State)
	}
}

// With no active data path the tunnel endpoint is recorded but no FAR is
// touched, so a nil downlink tunnel must not panic.
func TestHandleHandoverRequestAcknowledgeTransfer_NotActivated(t *testing.T) {
	smContext := &SMContext{Tunnel: &UPTunnel{DataPath: &DataPath{Activated: false}}}

	if err := handleHandoverRequestAcknowledgeTransfer(validHandoverRequestAcknowledgeTransfer(t), smContext); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if smContext.Tunnel.ANInformation.TEID != testHandoverTEID {
		t.Errorf("ANInformation.TEID = %#x, want %#x", smContext.Tunnel.ANInformation.TEID, testHandoverTEID)
	}
}

// Undecodable input is rejected with an error rather than a panic.
func TestHandleHandoverRequestAcknowledgeTransfer_BadInput(t *testing.T) {
	smContext := &SMContext{Tunnel: &UPTunnel{DataPath: &DataPath{Activated: true}}}

	if err := handleHandoverRequestAcknowledgeTransfer([]byte{0xff, 0xff}, smContext); err == nil {
		t.Fatal("expected an error for undecodable transfer, got nil")
	}
}
