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

	if !smCtx.Tunnel.AN.IPv4.Equal(gnbIP) {
		t.Fatalf("expected AN IP %s, got %s", gnbIP, smCtx.Tunnel.AN.IPv4)
	}

	if smCtx.Tunnel.AN.TEID != teid {
		t.Fatalf("expected AN TEID %d, got %d", teid, smCtx.Tunnel.AN.TEID)
	}

	ohc := upf.modifyCalls[0].UpdateFARs[1].ForwardingParameters.OuterHeaderCreation
	if ohc == nil {
		t.Fatal("expected DL FAR outer header creation to be set")
	}

	if ohc.TEID != teid {
		t.Fatalf("expected DL FAR TEID %d, got %d", teid, ohc.TEID)
	}

	confirm, err := libngap.ParsePDUSessionResourceModifyConfirmTransfer(n2Rsp)
	if err != nil {
		t.Fatalf("decode confirm transfer: %v", err)
	}

	if len(confirm.QosFlowModifyConfirm) != 1 || confirm.QosFlowModifyConfirm[0].QosFlowIdentifier != 1 {
		t.Fatalf("expected confirm list naming QFI 1, got %v", confirm.QosFlowModifyConfirm)
	}

	upf.mu.Lock()
	defer upf.mu.Unlock()

	if len(upf.modifyCalls) != 1 {
		t.Fatalf("expected 1 PFCP modify call, got %d", len(upf.modifyCalls))
	}
}

// The uplink PDR's OuterHeaderRemoval follows the endpoint's family as much as
// the downlink FAR does, so sending only the FAR would leave the UPF
// decapsulating uplink GTP-U as the previous family when the RAN moves between
// IPv4 and IPv6.
func TestUpdateSmContextN2ModifyIndication_SendsUplinkPDR(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb)
	ctx := context.Background()

	smCtx, ref := setupSessionWithTunnel(t, s)

	// An IPv6 transport address is what changes the OuterHeaderRemoval.
	gnbIP := net.ParseIP("2001:db8::c0de")

	n2Data, err := buildModifyIndicationTransfer(9000, gnbIP, 1)
	if err != nil {
		t.Fatalf("build N2 payload: %v", err)
	}

	if _, err := s.UpdateSmContextN2ModifyIndication(ctx, ref, n2Data); err != nil {
		t.Fatalf("UpdateSmContextN2ModifyIndication: %v", err)
	}

	if smCtx.Tunnel.AN.IPv6 == nil {
		t.Fatalf("AN = %+v, want the IPv6 endpoint the indication named", smCtx.Tunnel.AN)
	}

	upf.mu.Lock()
	defer upf.mu.Unlock()

	if len(upf.modifyCalls) != 1 {
		t.Fatalf("expected 1 PFCP modify call, got %d", len(upf.modifyCalls))
	}

	req := upf.modifyCalls[0]

	var sawUplink bool

	for _, p := range req.UpdatePDRs {
		if p.PDI.LocalFTEID != nil {
			sawUplink = true

			if p.OuterHeaderRemoval == nil || *p.OuterHeaderRemoval != models.OuterHeaderRemovalGtpUUdpIpv6 {
				t.Errorf("uplink PDR sent with OuterHeaderRemoval %v, want the IPv6 descriptor", p.OuterHeaderRemoval)
			}
		}
	}

	if !sawUplink {
		t.Errorf("modify request omits the uplink PDR; it carries %d PDR(s) and %d FAR(s)",
			len(req.UpdatePDRs), len(req.UpdateFARs))
	}
}

// A session torn down but still in the pool — startRelease leaves it there for
// the whole T3592 window — has a nil Tunnel. An NGAP handler reaching it must
// report an error, not panic the dispatch goroutine and abort the association.
func TestUpdateSmContextN2ModifyIndication_HollowedSession(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb)
	ctx := context.Background()

	smCtx, ref := setupSessionWithTunnel(t, s)

	smCtx.Mutex.Lock()
	smCtx.Tunnel = nil
	smCtx.Mutex.Unlock()

	n2Data, err := buildModifyIndicationTransfer(8000, net.ParseIP("10.0.0.201").To4(), 1)
	if err != nil {
		t.Fatalf("build N2 payload: %v", err)
	}

	if _, err := s.UpdateSmContextN2ModifyIndication(ctx, ref, n2Data); err == nil {
		t.Fatal("modify indication on a hollowed session returned no error")
	}
}

func TestUpdateSmContextXnHandoverPathSwitchReq_HollowedSession(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb)
	ctx := context.Background()

	smCtx, ref := setupSessionWithTunnel(t, s)

	smCtx.Mutex.Lock()
	smCtx.Tunnel = nil
	smCtx.Mutex.Unlock()

	transfer := libngap.PathSwitchRequestTransfer{
		DLNGUUPTNLInformation: testTunnel(9000, net.ParseIP("10.0.0.202").To4()),
		QosFlowAccepted:       libngap.QosFlowAcceptedList{{QosFlowIdentifier: 1}},
	}

	n2Data, err := transfer.Marshal()
	if err != nil {
		t.Fatalf("marshal path switch transfer: %v", err)
	}

	if _, err := s.UpdateSmContextXnHandoverPathSwitchReq(ctx, ref, n2Data); err == nil {
		t.Fatal("path switch on a hollowed session returned no error")
	}
}
