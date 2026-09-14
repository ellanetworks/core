// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package smf_test

import (
	"context"
	"net"
	"net/netip"
	"testing"
	"time"

	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/smf"
	libngap "github.com/ellanetworks/core/ngap"
)

const (
	upfForwardingTEID = uint32(0xABCD)
	forwardingFARID   = uint32(3)
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

	request, command, _ := n2HandoverForwardingExchangeUPF(t, directPath)

	return request, command
}

func n2HandoverForwardingExchangeUPF(t *testing.T, directPath bool) (*libngap.PDUSessionResourceSetupRequestTransfer, *libngap.HandoverCommandTransfer, *fakeUPF) {
	t.Helper()

	pcf, store, upf, amfCb := defaultFakes()
	upf.forwardingTEID = upfForwardingTEID
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

	return request, command, upf
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

func TestN2HandoverWithoutDirectPathForwardsIndirectly(t *testing.T) {
	request, command, upf := n2HandoverForwardingExchangeUPF(t, false)

	if request.DataForwardingNotPossible != nil {
		t.Error("the target was told data forwarding is impossible, so it allocates no forwarding endpoint")
	}

	if request.DirectForwardingPathAvailability != nil {
		t.Error("Direct Forwarding Path Availability was claimed though the source reported none")
	}

	if command.DLForwardingUPTNLInformation == nil {
		t.Fatal("the Handover Command carried no forwarding endpoint")
	}

	if teid := uint32(command.DLForwardingUPTNLInformation.GTPTunnel.GTPTEID); teid != upfForwardingTEID {
		t.Errorf("relayed forwarding TEID = %#x, want the UPF's %#x", teid, upfForwardingTEID)
	}

	if len(command.QosFlowToBeForwarded) != 1 {
		t.Errorf("QoS flows to be forwarded = %d, want 1", len(command.QosFlowToBeForwarded))
	}

	far, ok := lastForwardingFAR(upf)
	if !ok {
		t.Fatal("the UPF was never asked for a forwarding tunnel")
	}

	if far.ForwardingParameters == nil || far.ForwardingParameters.OuterHeaderCreation == nil {
		t.Fatal("the forwarding FAR does not encapsulate")
	}

	if teid := far.ForwardingParameters.OuterHeaderCreation.TEID; teid != 0xfeed {
		t.Errorf("the forwarding tunnel encapsulates towards TEID %#x, want the target's 0xfeed", teid)
	}
}

func TestN2HandoverWithDirectPathOpensNoForwardingTunnel(t *testing.T) {
	_, _, upf := n2HandoverForwardingExchangeUPF(t, true)

	if _, ok := lastForwardingFAR(upf); ok {
		t.Error("a direct forwarding path still opened an indirect tunnel in the UPF")
	}
}

func lastForwardingFAR(upf *fakeUPF) (models.FAR, bool) {
	upf.mu.Lock()
	defer upf.mu.Unlock()

	var (
		found models.FAR
		ok    bool
	)

	for _, req := range upf.modifyCalls {
		for _, far := range req.UpdateFARs {
			if far.FARID == forwardingFARID {
				found, ok = far, true
			}
		}
	}

	return found, ok
}

func n2HandoverToForwarding(t *testing.T) (*smf.SMF, *fakeUPF, string, context.Context) {
	t.Helper()

	pcf, store, upf, amfCb := defaultFakes()
	upf.forwardingTEID = upfForwardingTEID
	s := newTestSMF(pcf, store, upf, amfCb)
	ctx := context.Background()

	_, ref := setupSessionWithTunnel(t, s)

	if _, err := s.UpdateSmContextN2HandoverPreparing(ctx, ref, n2HandoverRequired(t, false)); err != nil {
		t.Fatalf("UpdateSmContextN2HandoverPreparing: %v", err)
	}

	ack := n2HandoverAcknowledge(t, 8000, net.ParseIP("10.0.0.201"), 0xfeed)

	if _, err := s.UpdateSmContextN2HandoverPrepared(ctx, ref, ack); err != nil {
		t.Fatalf("UpdateSmContextN2HandoverPrepared: %v", err)
	}

	if _, ok := lastForwardingFAR(upf); !ok {
		t.Fatal("no forwarding tunnel was opened")
	}

	return s, upf, ref, ctx
}

func forwardingTunnelRemoved(upf *fakeUPF) bool {
	upf.mu.Lock()
	defer upf.mu.Unlock()

	for _, req := range upf.modifyCalls {
		for _, id := range req.RemovePDRs {
			if id == 4 {
				return true
			}
		}
	}

	return false
}

func TestN2HandoverCompletionReleasesForwardingTunnelOnTheTimer(t *testing.T) {
	restore := smf.SetIndirectForwardingDurationForTest(20 * time.Millisecond)
	defer restore()

	s, upf, ref, ctx := n2HandoverToForwarding(t)

	if err := s.UpdateSmContextN2HandoverComplete(ctx, ref); err != nil {
		t.Fatalf("UpdateSmContextN2HandoverComplete: %v", err)
	}

	if forwardingTunnelRemoved(upf) {
		t.Fatal("the forwarding tunnel was released before the timer expired")
	}

	deadline := time.Now().Add(2 * time.Second)
	for !forwardingTunnelRemoved(upf) {
		if time.Now().After(deadline) {
			t.Fatal("the forwarding tunnel was never released after the handover completed")
		}

		time.Sleep(5 * time.Millisecond)
	}
}

func TestN2HandoverCancelReleasesForwardingTunnel(t *testing.T) {
	s, upf, ref, ctx := n2HandoverToForwarding(t)

	if err := s.UpdateSmContextN2HandoverCanceled(ctx, ref); err != nil {
		t.Fatalf("UpdateSmContextN2HandoverCanceled: %v", err)
	}

	if !forwardingTunnelRemoved(upf) {
		t.Error("a cancelled handover left its forwarding tunnel behind")
	}
}

func TestN2HandoverFailureReleasesForwardingTunnel(t *testing.T) {
	s, upf, ref, ctx := n2HandoverToForwarding(t)

	unsuccessful := libngap.HandoverResourceAllocationUnsuccessfulTransfer{
		Cause: libngap.Cause{Group: libngap.CauseGroupRadioNetwork, Value: libngap.CauseRadioNetworkRadioResourcesNotAvailable},
	}

	b, err := unsuccessful.Marshal()
	if err != nil {
		t.Fatalf("marshal HandoverResourceAllocationUnsuccessfulTransfer: %v", err)
	}

	if err := s.UpdateSmContextN2HandoverFailed(ctx, ref, b); err != nil {
		t.Fatalf("UpdateSmContextN2HandoverFailed: %v", err)
	}

	if !forwardingTunnelRemoved(upf) {
		t.Error("a refused handover left its forwarding tunnel behind")
	}
}

func TestN2HandoverCancelAfterCompletionKeepsForwardingTunnel(t *testing.T) {
	s, upf, ref, ctx := n2HandoverToForwarding(t)

	if err := s.UpdateSmContextN2HandoverComplete(ctx, ref); err != nil {
		t.Fatalf("UpdateSmContextN2HandoverComplete: %v", err)
	}

	if err := s.UpdateSmContextN2HandoverCanceled(ctx, ref); err != nil {
		t.Fatalf("UpdateSmContextN2HandoverCanceled: %v", err)
	}

	if forwardingTunnelRemoved(upf) {
		t.Error("a cancel arriving after the handover completed tore down the forwarding tunnel the timer still owns")
	}
}

func TestN2HandoverForwardingEndpointIsDualStack(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	upf.forwardingTEID = upfForwardingTEID
	s := newTestSMF(pcf, store, upf, amfCb)
	ctx := context.Background()

	sc, ref := setupSessionWithTunnel(t, s)
	sc.Tunnel.N3IPv6 = netip.MustParseAddr("2001:db8::1")

	if _, err := s.UpdateSmContextN2HandoverPreparing(ctx, ref, n2HandoverRequired(t, false)); err != nil {
		t.Fatalf("UpdateSmContextN2HandoverPreparing: %v", err)
	}

	toSource, err := s.UpdateSmContextN2HandoverPrepared(ctx, ref,
		n2HandoverAcknowledge(t, 8000, net.ParseIP("10.0.0.201"), 0xfeed))
	if err != nil {
		t.Fatalf("UpdateSmContextN2HandoverPrepared: %v", err)
	}

	command, err := libngap.ParseHandoverCommandTransfer(toSource)
	if err != nil {
		t.Fatalf("parse the Handover Command transfer: %v", err)
	}

	if command.DLForwardingUPTNLInformation == nil {
		t.Fatal("the Handover Command carried no forwarding endpoint")
	}

	if got := len(command.DLForwardingUPTNLInformation.GTPTunnel.TransportLayerAddress); got != 20 {
		t.Fatalf("forwarding transport layer address is %d octets, want the 20-octet dual-stack form", got)
	}
}
