// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package smf_test

import (
	"context"
	"net"
	"testing"

	libngap "github.com/ellanetworks/core/ngap"
)

func buildModifyIndicationTransfer(teid uint32, ip net.IP, qfi int64) ([]byte, error) {
	transfer := libngap.PDUSessionResourceModifyIndicationTransfer{
		DLQosFlowPerTNLInformation: libngap.QosFlowPerTNLInformation{
			UPTransportLayerInformation: testTunnel(teid, ip),
			AssociatedQosFlowList: libngap.AssociatedQosFlowList{
				{QosFlowIdentifier: libngap.QosFlowIdentifier(qfi)},
			},
		},
	}

	b, err := transfer.Marshal()

	return b, err
}

func TestUpdateSmContextN2ModifyIndication_HappyPath(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb)
	ctx := context.Background()

	smCtx, ref := setupSessionWithTunnel(t, s)

	gnbIP := net.ParseIP("10.0.0.201").To4()
	teid := uint32(8000)

	n2Data, err := buildModifyIndicationTransfer(teid, gnbIP, 1)
	if err != nil {
		t.Fatalf("build N2 payload: %v", err)
	}

	n2Rsp, err := s.UpdateSmContextN2ModifyIndication(ctx, ref, n2Data)
	if err != nil {
		t.Fatalf("UpdateSmContextN2ModifyIndication: %v", err)
	}

	if n2Rsp == nil {
		t.Fatal("expected non-nil N2 response")
	}

	if !smCtx.Tunnel.ANInformation.IPv4Address.Equal(gnbIP) {
		t.Fatalf("expected AN IP %s, got %s", gnbIP, smCtx.Tunnel.ANInformation.IPv4Address)
	}

	if smCtx.Tunnel.ANInformation.TEID != teid {
		t.Fatalf("expected AN TEID %d, got %d", teid, smCtx.Tunnel.ANInformation.TEID)
	}

	dlFAR := smCtx.Tunnel.DataPath.DownLinkTunnel.PDR.FAR
	if dlFAR.ForwardingParameters == nil || dlFAR.ForwardingParameters.OuterHeaderCreation == nil {
		t.Fatal("expected DL FAR outer header creation to be set")
	}

	if dlFAR.ForwardingParameters.OuterHeaderCreation.TEID != teid {
		t.Fatalf("expected DL FAR TEID %d, got %d", teid, dlFAR.ForwardingParameters.OuterHeaderCreation.TEID)
	}

	confirm, err := libngap.ParsePDUSessionResourceModifyConfirmTransfer(n2Rsp)
	if err != nil {
		t.Fatalf("decode confirm transfer: %v", err)
	}

	if len(confirm.QosFlowModifyConfirm) != 1 || confirm.QosFlowModifyConfirm[0].QosFlowIdentifier != 1 {
		t.Fatalf("expected confirm list naming QFI 1, got %v", confirm.QosFlowModifyConfirm)
	}

	if got := upf.applyCount(); got != 1 {
		t.Fatalf("UPF session statements = %d, want 1", got)
	}
}
