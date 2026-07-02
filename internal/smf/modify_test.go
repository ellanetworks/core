// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package smf_test

import (
	"context"
	"encoding/binary"
	"net"
	"testing"

	"github.com/free5gc/aper"
	"github.com/free5gc/ngap/ngapType"
)

func buildModifyIndicationTransfer(teid uint32, ip net.IP, qfi int64) ([]byte, error) {
	transfer := ngapType.PDUSessionResourceModifyIndicationTransfer{}

	tnl := &transfer.DLQosFlowPerTNLInformation.UPTransportLayerInformation
	tnl.Present = ngapType.UPTransportLayerInformationPresentGTPTunnel
	tnl.GTPTunnel = new(ngapType.GTPTunnel)

	teidBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(teidBytes, teid)
	tnl.GTPTunnel.GTPTEID.Value = teidBytes
	tnl.GTPTunnel.TransportLayerAddress.Value = aper.BitString{
		Bytes:     ip.To4(),
		BitLength: 32,
	}

	transfer.DLQosFlowPerTNLInformation.AssociatedQosFlowList.List = append(
		transfer.DLQosFlowPerTNLInformation.AssociatedQosFlowList.List,
		ngapType.AssociatedQosFlowItem{QosFlowIdentifier: ngapType.QosFlowIdentifier{Value: qfi}},
	)

	return aper.MarshalWithParams(transfer, "valueExt")
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

	// The downlink tunnel is rebound to the address the NG-RAN provided.
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

	// The confirm transfer decodes and reports the confirmed QoS flow.
	confirm := ngapType.PDUSessionResourceModifyConfirmTransfer{}
	if err := aper.UnmarshalWithParams(n2Rsp, &confirm, "valueExt"); err != nil {
		t.Fatalf("decode confirm transfer: %v", err)
	}

	if len(confirm.QosFlowModifyConfirmList.List) != 1 || confirm.QosFlowModifyConfirmList.List[0].QosFlowIdentifier.Value != 1 {
		t.Fatalf("expected confirm list naming QFI 1, got %v", confirm.QosFlowModifyConfirmList.List)
	}

	upf.mu.Lock()
	defer upf.mu.Unlock()

	if len(upf.modifyCalls) != 1 {
		t.Fatalf("expected 1 PFCP modify call, got %d", len(upf.modifyCalls))
	}
}
