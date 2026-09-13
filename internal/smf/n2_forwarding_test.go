// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package smf_test

import (
	"context"
	"net"
	"testing"

	"github.com/ellanetworks/core/internal/models"
	libngap "github.com/ellanetworks/core/ngap"
)

func n2HandoverRequired(t *testing.T, directPath bool) []byte {
	t.Helper()

	transfer := libngap.HandoverRequiredTransfer{}
	if directPath {
		transfer.DirectForwardingPathAvailability = libngap.Ptr(libngap.DirectForwardingPathAvailable)
	}

	b, err := transfer.Marshal()
	if err != nil {
		t.Fatalf("marshal HandoverRequiredTransfer: %v", err)
	}

	return b
}

func n2HandoverAcknowledge(t *testing.T, teid uint32, ip net.IP, forwardingTEID uint32) []byte {
	t.Helper()

	transfer := libngap.HandoverRequestAcknowledgeTransfer{
		DLNGUUPTNLInformation: libngap.UPTransportLayerInformation{GTPTunnel: libngap.GTPTunnel{
			TransportLayerAddress: libngap.TransportLayerAddress(ip.To4()), GTPTEID: libngap.GTPTEID(teid),
		}},
		QosFlowSetupResponse: libngap.QosFlowListWithDataForwarding{{
			QosFlowIdentifier:      libngap.QosFlowIdentifier(models.DefaultQFI),
			DataForwardingAccepted: libngap.Ptr(libngap.DataForwardingAcceptedTrue),
		}},
		DLForwardingUPTNLInformation: &libngap.UPTransportLayerInformation{GTPTunnel: libngap.GTPTunnel{
			TransportLayerAddress: libngap.TransportLayerAddress(ip.To4()), GTPTEID: libngap.GTPTEID(forwardingTEID),
		}},
	}

	b, err := transfer.Marshal()
	if err != nil {
		t.Fatalf("marshal HandoverRequestAcknowledgeTransfer: %v", err)
	}

	return b
}

func n2HandoverForwardingExchange(t *testing.T, directPath bool) (*libngap.PDUSessionResourceSetupRequestTransfer, *libngap.HandoverCommandTransfer) {
	t.Helper()

	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb)
	ctx := context.Background()

	_, ref := setupSessionWithTunnel(t, s)

	toTarget, err := s.UpdateSmContextN2HandoverPreparing(ctx, ref, n2HandoverRequired(t, directPath))
	if err != nil {
		t.Fatalf("UpdateSmContextN2HandoverPreparing: %v", err)
	}

	request, err := libngap.ParsePDUSessionResourceSetupRequestTransfer(toTarget)
	if err != nil {
		t.Fatalf("parse the Handover Request transfer: %v", err)
	}

	ack := n2HandoverAcknowledge(t, 8000, net.ParseIP("10.0.0.201"), 0xfeed)

	toSource, err := s.UpdateSmContextN2HandoverPrepared(ctx, ref, ack)
	if err != nil {
		t.Fatalf("UpdateSmContextN2HandoverPrepared: %v", err)
	}

	command, err := libngap.ParseHandoverCommandTransfer(toSource)
	if err != nil {
		t.Fatalf("parse the Handover Command transfer: %v", err)
	}

	return request, command
}

func TestN2HandoverRelaysForwardingEndpointOnDirectPath(t *testing.T) {
	request, command := n2HandoverForwardingExchange(t, true)

	if request.DataForwardingNotPossible != nil {
		t.Error("the target was told data forwarding is impossible despite a direct forwarding path")
	}

	if request.DirectForwardingPathAvailability == nil {
		t.Error("the source's Direct Forwarding Path Availability was not relayed to the target")
	}

	if command.DLForwardingUPTNLInformation == nil {
		t.Fatal("the target's forwarding endpoint was not relayed to the source")
	}

	if teid := uint32(command.DLForwardingUPTNLInformation.GTPTunnel.GTPTEID); teid != 0xfeed {
		t.Errorf("relayed forwarding TEID = %#x, want 0xfeed", teid)
	}

	if len(command.QosFlowToBeForwarded) != 1 {
		t.Errorf("QoS flows to be forwarded = %d, want 1", len(command.QosFlowToBeForwarded))
	}
}

func TestN2HandoverWithoutDirectPathOffersNoForwarding(t *testing.T) {
	request, command := n2HandoverForwardingExchange(t, false)

	if request.DataForwardingNotPossible == nil {
		t.Error("the target was not told that data forwarding is impossible")
	}

	if request.DirectForwardingPathAvailability != nil {
		t.Error("Direct Forwarding Path Availability was claimed though the source reported none")
	}

	if command.DLForwardingUPTNLInformation != nil || len(command.QosFlowToBeForwarded) != 0 {
		t.Errorf("the Handover Command offered forwarding: %+v / %+v",
			command.DLForwardingUPTNLInformation, command.QosFlowToBeForwarded)
	}
}
