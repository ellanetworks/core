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

func TestHandleHandoverRequestAcknowledgeTransfer_ProposesTarget(t *testing.T) {
	source := AnchorBinding{TEID: 0x99}
	smContext := &SMContext{
		PolicyData: &Policy{QosData: models.QosData{QFI: models.DefaultQFI}},
		Tunnel:     &UPTunnel{dataPlane: dataPlane{AN: source, Downlink: DownlinkForwarding}},
	}

	if err := handleHandoverRequestAcknowledgeTransfer(validHandoverRequestAcknowledgeTransfer(t), smContext); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if smContext.handoverTargetAN == nil {
		t.Fatal("the target endpoint was not recorded")
	}

	if smContext.handoverTargetAN.TEID != testHandoverTEID {
		t.Errorf("handoverTargetAN.TEID = %#x, want %#x", smContext.handoverTargetAN.TEID, testHandoverTEID)
	}

	if smContext.Tunnel.AN.TEID != source.TEID {
		t.Errorf("AN.TEID = %#x, want the source endpoint's %#x", smContext.Tunnel.AN.TEID, source.TEID)
	}
}

// Undecodable input is rejected with an error rather than a panic.
func TestHandleHandoverRequestAcknowledgeTransfer_BadInput(t *testing.T) {
	smContext := &SMContext{PolicyData: &Policy{QosData: models.QosData{QFI: models.DefaultQFI}}, Tunnel: &UPTunnel{}}

	if err := handleHandoverRequestAcknowledgeTransfer([]byte{0xff, 0xff}, smContext); err == nil {
		t.Fatal("expected an error for undecodable transfer, got nil")
	}
}
