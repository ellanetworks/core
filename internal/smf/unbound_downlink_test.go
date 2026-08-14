// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package smf_test

import (
	"context"
	"net"
	"testing"

	"github.com/ellanetworks/core/internal/smf"
	libngap "github.com/ellanetworks/core/ngap"
)

func assertNoModify(t *testing.T, upf *fakeUPF) {
	t.Helper()

	upf.mu.Lock()
	defer upf.mu.Unlock()

	if len(upf.modifyCalls) != 0 {
		t.Fatalf("the UPF was sent %d modification(s) for a downlink with no endpoint", len(upf.modifyCalls))
	}
}

// TS 23.502 §4.9.1.3.3 step 10a
func TestHandoverCompleteWithoutAPreparedTargetIsRefused(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb)

	smCtx, ref := setupSessionWithTunnel(t, s)

	sourceTEID := smCtx.Tunnel.AN.TEID

	if err := s.UpdateSmContextN2HandoverComplete(context.Background(), ref); err == nil {
		t.Fatal("a Handover Notify for a session with no prepared handover was accepted")
	}

	assertNoModify(t, upf)

	if smCtx.Tunnel.AN.TEID != sourceTEID {
		t.Errorf("AN TEID = %#x, want the source's %#x: a refused completion must not move the downlink",
			smCtx.Tunnel.AN.TEID, sourceTEID)
	}
}

func TestHandoverCompleteAfterAnIdleMoveIsRefused(t *testing.T) {
	pcf, store, upf, amfCb, mmeCb := interworkingFakes()
	s := newTestSMF(pcf, store, upf, amfCb)
	s.SetMME(mmeCb)

	ctx := context.Background()

	sc := establishEPSOnENB(t, s)

	ref, err := s.TransferIdle(ctx, testSUPI(), movedPDUSessionID, epsTestEBI, testDNN, testSnssai, smf.Access5G)
	if err != nil {
		t.Fatalf("TransferIdle: %v", err)
	}

	upf.mu.Lock()
	upf.modifyCalls = nil
	upf.mu.Unlock()

	if err := s.UpdateSmContextN2HandoverComplete(ctx, ref); err == nil {
		t.Fatal("a Handover Notify for a session with no access-network endpoint was accepted")
	}

	assertNoModify(t, upf)

	sc.Mutex.Lock()
	defer sc.Mutex.Unlock()

	if sc.Tunnel.Downlink != smf.DownlinkBuffering {
		t.Errorf("downlink state = %d, want buffering: it has nowhere to forward", sc.Tunnel.Downlink)
	}
}

// TS 38.413 §9.3.2.4
func TestNGRANBindWithAnUnusableTransportAddressIsRefused(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb)

	_, ref := setupSessionWithTunnel(t, s)

	transfer := libngap.PDUSessionResourceSetupResponseTransfer{
		DLQosFlowPerTNLInformation: libngap.QosFlowPerTNLInformation{
			UPTransportLayerInformation: libngap.UPTransportLayerInformation{GTPTunnel: libngap.GTPTunnel{
				TransportLayerAddress: libngap.TransportLayerAddress{10, 0, 0},
				GTPTEID:               libngap.GTPTEID(0x9001),
			}},
			AssociatedQosFlowList: libngap.AssociatedQosFlowList{{QosFlowIdentifier: 1}},
		},
	}

	n2, err := transfer.Marshal()
	if err != nil {
		t.Fatalf("marshal setup response: %v", err)
	}

	upf.mu.Lock()
	upf.modifyCalls = nil
	upf.mu.Unlock()

	if err := s.UpdateSmContextN2InfoPduResSetupRsp(context.Background(), ref, n2); err == nil {
		t.Fatal("a setup response naming no usable transport address was accepted")
	}

	assertNoModify(t, upf)
}

func TestNGRANBindWithAUsableTransportAddressStillBinds(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb)

	smCtx, ref := setupSessionWithTunnel(t, s)

	n2, err := buildPDUSessionResourceSetupResponseTransfer(0x9002, net.ParseIP("10.0.0.9").To4())
	if err != nil {
		t.Fatalf("build setup response: %v", err)
	}

	if err := s.UpdateSmContextN2InfoPduResSetupRsp(context.Background(), ref, n2); err != nil {
		t.Fatalf("UpdateSmContextN2InfoPduResSetupRsp: %v", err)
	}

	if smCtx.Tunnel.AN.TEID != 0x9002 {
		t.Errorf("AN TEID = %#x, want 0x9002", smCtx.Tunnel.AN.TEID)
	}

	if smCtx.Tunnel.Downlink != smf.DownlinkForwarding {
		t.Errorf("downlink state = %d, want forwarding", smCtx.Tunnel.Downlink)
	}

	upf.mu.Lock()
	defer upf.mu.Unlock()

	if len(upf.modifyCalls) != 1 {
		t.Fatalf("PFCP modify calls = %d, want 1", len(upf.modifyCalls))
	}

	if ohc := upf.modifyCalls[0].UpdateFARs[1].ForwardingParameters.OuterHeaderCreation; ohc == nil || ohc.TEID != 0x9002 {
		t.Errorf("downlink FAR outer header creation = %+v, want the gNB's TEID 0x9002", ohc)
	}
}
