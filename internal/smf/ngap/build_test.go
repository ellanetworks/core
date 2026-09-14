// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap_test

import (
	"net/netip"
	"strings"
	"testing"

	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/smf/ngap"
	libngap "github.com/ellanetworks/core/ngap"
)

func decodeSetupRequestGTPTunnel(t *testing.T, buf []byte) (uint32, libngap.TransportLayerAddress) {
	t.Helper()

	transfer, err := libngap.ParsePDUSessionResourceSetupRequestTransfer(buf)
	if err != nil {
		t.Fatalf("unmarshal PDUSessionResourceSetupRequestTransfer: %v", err)
	}

	tunnel := transfer.ULNGUUPTNLInformation.GTPTunnel

	return uint32(tunnel.GTPTEID), tunnel.TransportLayerAddress
}

func TestBuildPDUSessionResourceSetupRequestTransfer(t *testing.T) {
	ambr := &models.Ambr{Uplink: models.MustParseBitRate("100 Mbps"), Downlink: models.MustParseBitRate("200 Mbps")}
	qos := &models.QosData{Var5qi: 9, Arp: &models.Arp{PriorityLevel: 1}, QFI: 1}
	addr := netip.MustParseAddr("10.3.0.2")

	buf, err := ngap.BuildPDUSessionResourceSetupRequestTransfer(ambr, qos, 42, addr, netip.Addr{}, libngap.PDUSessionTypeIPv4)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	teid, bs := decodeSetupRequestGTPTunnel(t, buf)

	if teid != 42 {
		t.Errorf("TEID: got %d, want 42", teid)
	}

	if len(bs)*8 != 32 {
		t.Fatalf("BitLength: got %d, want 32", len(bs)*8)
	}

	var ip [4]byte
	copy(ip[:], []byte(bs))

	if ip != [4]byte{10, 3, 0, 2} {
		t.Errorf("IP: got %v, want [10 3 0 2]", ip)
	}
}

func TestBuildHandoverRequestTransfer(t *testing.T) {
	ambr := &models.Ambr{Uplink: models.MustParseBitRate("100 Mbps"), Downlink: models.MustParseBitRate("200 Mbps")}
	qos := &models.QosData{Var5qi: 9, Arp: &models.Arp{PriorityLevel: 1}, QFI: 1}
	addr := netip.MustParseAddr("10.3.0.2")

	buf, err := ngap.BuildHandoverRequestTransfer(ambr, qos, 42, addr, netip.Addr{}, libngap.PDUSessionTypeIPv4, nil, ngap.DataForwardingNone)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	transfer, err := libngap.ParsePDUSessionResourceSetupRequestTransfer(buf)
	if err != nil {
		t.Fatalf("unmarshal PDUSessionResourceSetupRequestTransfer: %v", err)
	}

	if transfer.DataForwardingNotPossible == nil {
		t.Fatal("Data Forwarding Not Possible: got absent, want present")
	}

	if *transfer.DataForwardingNotPossible != libngap.DataForwardingNotPossibleTrue {
		t.Errorf("Data Forwarding Not Possible: got %d, want %d", *transfer.DataForwardingNotPossible, libngap.DataForwardingNotPossibleTrue)
	}

	if teid := uint32(transfer.ULNGUUPTNLInformation.GTPTunnel.GTPTEID); teid != 42 {
		t.Errorf("TEID: got %d, want 42", teid)
	}
}

func TestBuildHandoverRequestTransfer_NilAmbr(t *testing.T) {
	_, err := ngap.BuildHandoverRequestTransfer(nil, nil, 1, netip.MustParseAddr("1.2.3.4"), netip.Addr{}, libngap.PDUSessionTypeIPv4, nil, ngap.DataForwardingNone)
	if err == nil {
		t.Fatal("expected error for nil ambr")
	}
}

func TestBuildPDUSessionResourceSetupRequestTransfer_NoDataForwardingNotPossible(t *testing.T) {
	ambr := &models.Ambr{Uplink: models.MustParseBitRate("100 Mbps"), Downlink: models.MustParseBitRate("200 Mbps")}
	qos := &models.QosData{Var5qi: 9, Arp: &models.Arp{PriorityLevel: 1}, QFI: 1}

	buf, err := ngap.BuildPDUSessionResourceSetupRequestTransfer(ambr, qos, 42, netip.MustParseAddr("10.3.0.2"), netip.Addr{}, libngap.PDUSessionTypeIPv4)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	transfer, err := libngap.ParsePDUSessionResourceSetupRequestTransfer(buf)
	if err != nil {
		t.Fatalf("unmarshal PDUSessionResourceSetupRequestTransfer: %v", err)
	}

	if transfer.DataForwardingNotPossible != nil {
		t.Errorf("Data Forwarding Not Possible: got %d, want absent", *transfer.DataForwardingNotPossible)
	}
}

func TestBuildPDUSessionResourceSetupRequestTransfer_NilAmbr(t *testing.T) {
	_, err := ngap.BuildPDUSessionResourceSetupRequestTransfer(nil, nil, 1, netip.MustParseAddr("1.2.3.4"), netip.Addr{}, libngap.PDUSessionTypeIPv4)
	if err == nil {
		t.Fatal("expected error for nil ambr")
	}
}

func TestBuildPDUSessionResourceSetupRequestTransfer_IPv6Only(t *testing.T) {
	ambr := &models.Ambr{Uplink: models.MustParseBitRate("100 Mbps"), Downlink: models.MustParseBitRate("200 Mbps")}
	qos := &models.QosData{Var5qi: 9, Arp: &models.Arp{PriorityLevel: 1}, QFI: 1}
	ipv6 := netip.MustParseAddr("2001:db8::1")

	buf, err := ngap.BuildPDUSessionResourceSetupRequestTransfer(ambr, qos, 7, netip.Addr{}, ipv6, libngap.PDUSessionTypeIPv6)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, bs := decodeSetupRequestGTPTunnel(t, buf)

	if len(bs)*8 != 128 {
		t.Fatalf("BitLength: got %d, want 128", len(bs)*8)
	}

	if len([]byte(bs)) != 16 {
		t.Fatalf("Bytes length: got %d, want 16", len([]byte(bs)))
	}

	v6 := ipv6.As16()
	for i, b := range []byte(bs) {
		if b != v6[i] {
			t.Errorf("IPv6 byte[%d]: got %02x, want %02x", i, b, v6[i])
		}
	}
}

func TestBuildPDUSessionResourceSetupRequestTransfer_DualStack(t *testing.T) {
	ambr := &models.Ambr{Uplink: models.MustParseBitRate("100 Mbps"), Downlink: models.MustParseBitRate("200 Mbps")}
	qos := &models.QosData{Var5qi: 9, Arp: &models.Arp{PriorityLevel: 1}, QFI: 1}
	ipv4 := netip.MustParseAddr("10.3.0.2")
	ipv6 := netip.MustParseAddr("2001:db8::1")

	buf, err := ngap.BuildPDUSessionResourceSetupRequestTransfer(ambr, qos, 99, ipv4, ipv6, libngap.PDUSessionTypeIPv4v6)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, bs := decodeSetupRequestGTPTunnel(t, buf)

	if len(bs)*8 != 160 {
		t.Fatalf("BitLength: got %d, want 160", len(bs)*8)
	}

	if len([]byte(bs)) != 20 {
		t.Fatalf("Bytes length: got %d, want 20", len([]byte(bs)))
	}

	wantV4 := ipv4.As4()
	if [4]byte([]byte(bs)[0:4]) != wantV4 {
		t.Errorf("IPv4 part: got %v, want %v", []byte(bs)[0:4], wantV4)
	}

	wantV6 := ipv6.As16()
	for i, b := range []byte(bs)[4:20] {
		if b != wantV6[i] {
			t.Errorf("IPv6 byte[%d]: got %02x, want %02x", i, b, wantV6[i])
		}
	}
}

func TestBuildHandoverCommandTransfer(t *testing.T) {
	buf, err := ngap.BuildHandoverCommandTransfer(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	transfer, err := libngap.ParseHandoverCommandTransfer(buf)
	if err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if transfer.DLForwardingUPTNLInformation != nil {
		t.Errorf("DL Forwarding UP TNL Information: got %v, want absent", transfer.DLForwardingUPTNLInformation)
	}

	if len(transfer.QosFlowToBeForwarded) != 0 {
		t.Errorf("QoS Flow to be Forwarded List: got %d items, want 0", len(transfer.QosFlowToBeForwarded))
	}

	if len(transfer.DataForwardingResponseDRB) != 0 {
		t.Errorf("Data Forwarding Response DRB List: got %d items, want 0", len(transfer.DataForwardingResponseDRB))
	}
}

func TestBuildPathSwitchRequestAcknowledgeTransfer(t *testing.T) {
	addr := netip.MustParseAddr("172.16.0.1")

	buf, err := ngap.BuildPathSwitchRequestAcknowledgeTransfer(7, addr, netip.Addr{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	transfer, err := libngap.ParsePathSwitchRequestAcknowledgeTransfer(buf)
	if err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	tunnel := transfer.ULNGUUPTNLInformation.GTPTunnel
	teid := uint32(tunnel.GTPTEID)

	if teid != 7 {
		t.Errorf("TEID: got %d, want 7", teid)
	}

	bs := tunnel.TransportLayerAddress
	if len(bs)*8 != 32 {
		t.Fatalf("BitLength: got %d, want 32", len(bs)*8)
	}

	var ip [4]byte
	copy(ip[:], []byte(bs))

	if ip != [4]byte{172, 16, 0, 1} {
		t.Errorf("IP: got %v, want [172 16 0 1]", ip)
	}
}

func TestBuildPathSwitchRequestAcknowledgeTransfer_IPv6Only(t *testing.T) {
	ipv6 := netip.MustParseAddr("2001:db8::3")

	buf, err := ngap.BuildPathSwitchRequestAcknowledgeTransfer(3, netip.Addr{}, ipv6)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	transfer, err := libngap.ParsePathSwitchRequestAcknowledgeTransfer(buf)
	if err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	bs := transfer.ULNGUUPTNLInformation.GTPTunnel.TransportLayerAddress

	if len(bs)*8 != 128 {
		t.Fatalf("BitLength: got %d, want 128", len(bs)*8)
	}
}

// A malformed AMBR can no longer reach a builder: models.BitRate is only
// obtainable from ParseBitRate, so the text is rejected at the boundary that
// reads it. models.TestParseBitRateRejectsMalformed covers that; this records
// why there is no builder-level test for it.

// TS 38.413 bounds priorityLevelARP at 1..15. The library encoder would refuse
// a 0 on its own, but only as "value out of range"; the builder checks first so
// an operator with a bad policy is told which field is wrong.
func TestBuildPDUSessionResourceSetupRequestTransferRejectsARPZero(t *testing.T) {
	ambr := &models.Ambr{Downlink: models.MustParseBitRate("1 Gbps"), Uplink: models.MustParseBitRate("1 Gbps")}
	qos := &models.QosData{QFI: 1, Var5qi: 9, Arp: &models.Arp{PriorityLevel: 0}}

	_, err := ngap.BuildPDUSessionResourceSetupRequestTransfer(ambr, qos, 1,
		netip.MustParseAddr("1.2.3.4"), netip.Addr{}, libngap.PDUSessionTypeIPv4)
	if err == nil {
		t.Fatal("ARP priority 0 encoded, want an error")
	}

	if !strings.Contains(err.Error(), "ARP priority level") {
		t.Errorf("err = %v, want it to name the offending field", err)
	}
}

// A policy has an ARP priority column and no pre-emption columns, so the
// unprovisioned pair is the only one an operator's configuration can produce,
// and it must match what the 4G paths encode (mme.BearerARP).
func TestQosFlowARPPreemptionDefaults(t *testing.T) {
	ambr := &models.Ambr{Uplink: models.MustParseBitRate("100 Mbps"), Downlink: models.MustParseBitRate("200 Mbps")}

	for _, tc := range []struct {
		name     string
		arp      *models.Arp
		wantCap  libngap.PreemptionCapability
		wantVuln libngap.PreemptionVulnerability
	}{
		{
			"unprovisioned",
			&models.Arp{PriorityLevel: 5},
			libngap.PreemptionShallNotTrigger, libngap.PreemptionNotPreemptable,
		},
		{
			"explicitly may-preempt",
			&models.Arp{PriorityLevel: 5, PreemptCap: models.PreemptionCapabilityMayPreempt},
			libngap.PreemptionMayTrigger, libngap.PreemptionNotPreemptable,
		},
		{
			"explicitly preemptable",
			&models.Arp{PriorityLevel: 5, PreemptVuln: models.PreemptionVulnerabilityPreemptable},
			libngap.PreemptionShallNotTrigger, libngap.PreemptionPreemptable,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			qos := &models.QosData{Var5qi: 9, QFI: 1, Arp: tc.arp}

			buf, err := ngap.BuildPDUSessionResourceSetupRequestTransfer(
				ambr, qos, 42, netip.MustParseAddr("10.3.0.2"), netip.Addr{}, libngap.PDUSessionTypeIPv4)
			if err != nil {
				t.Fatalf("build: %v", err)
			}

			transfer, err := libngap.ParsePDUSessionResourceSetupRequestTransfer(buf)
			if err != nil {
				t.Fatalf("decode: %v", err)
			}

			flows := transfer.QosFlowSetupRequest
			if len(flows) != 1 {
				t.Fatalf("got %d QoS flows, want 1", len(flows))
			}

			arp := flows[0].QosFlowLevelQosParameters.AllocationAndRetentionPriority

			if arp.PriorityLevelARP != 5 {
				t.Errorf("priority = %d, want 5", arp.PriorityLevelARP)
			}

			if arp.PreemptionCapability != tc.wantCap {
				t.Errorf("pre-emption capability = %v, want %v", arp.PreemptionCapability, tc.wantCap)
			}

			if arp.PreemptionVulnerability != tc.wantVuln {
				t.Errorf("pre-emption vulnerability = %v, want %v", arp.PreemptionVulnerability, tc.wantVuln)
			}
		})
	}
}

// TS 23.502 §4.11.1.2.2.2 step 7
func TestBuildHandoverRequestTransferCarriesTheERABID(t *testing.T) {
	ambr := &models.Ambr{Uplink: models.MustParseBitRate("1 Mbps"), Downlink: models.MustParseBitRate("2 Mbps")}
	qos := &models.QosData{Var5qi: 9, QFI: 1, Arp: &models.Arp{PriorityLevel: 1}}
	ebi := uint8(5)

	buf, err := ngap.BuildHandoverRequestTransfer(ambr, qos, 42, netip.MustParseAddr("1.2.3.4"), netip.Addr{}, libngap.PDUSessionTypeIPv4, &ebi, ngap.DataForwardingNone)
	if err != nil {
		t.Fatalf("BuildHandoverRequestTransfer: %v", err)
	}

	transfer, err := libngap.ParsePDUSessionResourceSetupRequestTransfer(buf)
	if err != nil {
		t.Fatalf("parse the transfer: %v", err)
	}

	if len(transfer.QosFlowSetupRequest) != 1 {
		t.Fatalf("QoS flows = %d, want 1", len(transfer.QosFlowSetupRequest))
	}

	got := transfer.QosFlowSetupRequest[0].ERABID
	if got == nil {
		t.Fatal("no E-RAB ID, so the target cannot map the QoS flow back to its EPS bearer")
	}

	if uint8(*got) != ebi {
		t.Errorf("E-RAB ID = %d, want the EPS bearer identity %d", *got, ebi)
	}
}

// An intra-5GS handover has no EPS bearer, so the optional IE stays absent.
func TestBuildHandoverRequestTransferOmitsTheERABIDWithoutABearer(t *testing.T) {
	ambr := &models.Ambr{Uplink: models.MustParseBitRate("1 Mbps"), Downlink: models.MustParseBitRate("2 Mbps")}
	qos := &models.QosData{Var5qi: 9, QFI: 1, Arp: &models.Arp{PriorityLevel: 1}}

	buf, err := ngap.BuildHandoverRequestTransfer(ambr, qos, 42, netip.MustParseAddr("1.2.3.4"), netip.Addr{}, libngap.PDUSessionTypeIPv4, nil, ngap.DataForwardingNone)
	if err != nil {
		t.Fatalf("BuildHandoverRequestTransfer: %v", err)
	}

	transfer, err := libngap.ParsePDUSessionResourceSetupRequestTransfer(buf)
	if err != nil {
		t.Fatalf("parse the transfer: %v", err)
	}

	if transfer.QosFlowSetupRequest[0].ERABID != nil {
		t.Error("an intra-5GS handover carried an E-RAB ID")
	}
}

func TestBuildHandoverRequestTransferWithDirectForwarding(t *testing.T) {
	ambr := &models.Ambr{Uplink: models.MustParseBitRate("1 Mbps"), Downlink: models.MustParseBitRate("2 Mbps")}
	qos := &models.QosData{Var5qi: 9, QFI: 1, Arp: &models.Arp{PriorityLevel: 1}}

	buf, err := ngap.BuildHandoverRequestTransfer(ambr, qos, 42, netip.MustParseAddr("1.2.3.4"), netip.Addr{}, libngap.PDUSessionTypeIPv4, nil, ngap.DataForwardingDirect)
	if err != nil {
		t.Fatalf("BuildHandoverRequestTransfer: %v", err)
	}

	transfer, err := libngap.ParsePDUSessionResourceSetupRequestTransfer(buf)
	if err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if transfer.DataForwardingNotPossible != nil {
		t.Error("Data Forwarding Not Possible was set despite a direct forwarding path")
	}

	if transfer.DirectForwardingPathAvailability == nil {
		t.Fatal("Direct Forwarding Path Availability: got absent, want present")
	}

	if *transfer.DirectForwardingPathAvailability != libngap.DirectForwardingPathAvailable {
		t.Errorf("Direct Forwarding Path Availability = %d, want %d",
			*transfer.DirectForwardingPathAvailability, libngap.DirectForwardingPathAvailable)
	}
}

func TestForwardingPlanFromRelaysTheTargetsEndpoints(t *testing.T) {
	fwd := libngap.UPTransportLayerInformation{GTPTunnel: libngap.GTPTunnel{
		TransportLayerAddress: libngap.TransportLayerAddress{10, 4, 0, 9}, GTPTEID: 0xfeed,
	}}

	ack := &libngap.HandoverRequestAcknowledgeTransfer{
		DLNGUUPTNLInformation:        libngap.UPTransportLayerInformation{GTPTunnel: libngap.GTPTunnel{TransportLayerAddress: libngap.TransportLayerAddress{10, 4, 0, 2}, GTPTEID: 1}},
		DLForwardingUPTNLInformation: &fwd,
		QosFlowSetupResponse: libngap.QosFlowListWithDataForwarding{
			{QosFlowIdentifier: 1, DataForwardingAccepted: libngap.Ptr(libngap.DataForwardingAcceptedTrue)},
			{QosFlowIdentifier: 2},
		},
		DataForwardingResponseDRB: libngap.DataForwardingResponseDRBList{
			{DRBID: 1, DLForwardingUPTNLInformation: &fwd},
		},
	}

	plan := ngap.ForwardingPlanFrom(ack)

	if !plan.Forwards() {
		t.Fatal("plan reports nothing offered")
	}

	if plan.DLForwardingUPTNLInformation != &fwd {
		t.Error("the session's DL forwarding endpoint was not relayed")
	}

	if len(plan.QosFlowToBeForwarded) != 1 || plan.QosFlowToBeForwarded[0].QosFlowIdentifier != 1 {
		t.Errorf("QoS flows to be forwarded = %+v, want only the accepted flow 1", plan.QosFlowToBeForwarded)
	}

	if len(plan.DataForwardingResponseDRB) != 1 {
		t.Errorf("DRB forwarding list has %d items, want 1", len(plan.DataForwardingResponseDRB))
	}

	buf, err := ngap.BuildHandoverCommandTransfer(plan)
	if err != nil {
		t.Fatalf("BuildHandoverCommandTransfer: %v", err)
	}

	cmd, err := libngap.ParseHandoverCommandTransfer(buf)
	if err != nil {
		t.Fatalf("parse handover command transfer: %v", err)
	}

	if cmd.DLForwardingUPTNLInformation == nil || cmd.DLForwardingUPTNLInformation.GTPTunnel.GTPTEID != 0xfeed {
		t.Errorf("HandoverCommandTransfer DL forwarding endpoint = %+v, want TEID 0xfeed", cmd.DLForwardingUPTNLInformation)
	}

	if len(cmd.QosFlowToBeForwarded) != 1 || len(cmd.DataForwardingResponseDRB) != 1 {
		t.Errorf("HandoverCommandTransfer lists = %+v / %+v, want one each", cmd.QosFlowToBeForwarded, cmd.DataForwardingResponseDRB)
	}
}

func TestForwardingPlanFromWithoutAcceptedForwarding(t *testing.T) {
	ack := &libngap.HandoverRequestAcknowledgeTransfer{
		DLNGUUPTNLInformation: libngap.UPTransportLayerInformation{GTPTunnel: libngap.GTPTunnel{TransportLayerAddress: libngap.TransportLayerAddress{10, 4, 0, 2}, GTPTEID: 1}},
		QosFlowSetupResponse:  libngap.QosFlowListWithDataForwarding{{QosFlowIdentifier: 1}},
	}

	if ngap.ForwardingPlanFrom(ack).Forwards() {
		t.Error("a target that accepted no forwarding should offer nothing")
	}

	if ngap.ForwardingPlanFrom(nil).Forwards() {
		t.Error("a nil acknowledge should offer nothing")
	}
}

func TestForwardingPlanFromDropsUnusableForwardingEndpoints(t *testing.T) {
	bad := libngap.UPTransportLayerInformation{GTPTunnel: libngap.GTPTunnel{
		TransportLayerAddress: libngap.TransportLayerAddress{10, 4, 0, 9, 7}, GTPTEID: 0xfeed,
	}}
	good := libngap.UPTransportLayerInformation{GTPTunnel: libngap.GTPTunnel{
		TransportLayerAddress: libngap.TransportLayerAddress{10, 4, 0, 4}, GTPTEID: 0xbeef,
	}}

	ack := &libngap.HandoverRequestAcknowledgeTransfer{
		DLNGUUPTNLInformation:        libngap.UPTransportLayerInformation{GTPTunnel: libngap.GTPTunnel{TransportLayerAddress: libngap.TransportLayerAddress{10, 4, 0, 2}, GTPTEID: 1}},
		DLForwardingUPTNLInformation: &bad,
		QosFlowSetupResponse: libngap.QosFlowListWithDataForwarding{
			{QosFlowIdentifier: 1, DataForwardingAccepted: libngap.Ptr(libngap.DataForwardingAcceptedTrue)},
		},
		DataForwardingResponseDRB: libngap.DataForwardingResponseDRBList{
			{DRBID: 1, DLForwardingUPTNLInformation: &bad, ULForwardingUPTNLInformation: &good},
			{DRBID: 2, DLForwardingUPTNLInformation: &bad},
		},
	}

	plan := ngap.ForwardingPlanFrom(ack)

	if plan.DLForwardingUPTNLInformation != nil {
		t.Error("relayed a session DL forwarding endpoint the source gNB cannot reach: TS 38.413 §8.4.1.2 has the source NG-RAN node read its presence as the target accepting downlink forwarding")
	}

	if len(plan.DataForwardingResponseDRB) != 1 {
		t.Fatalf("DRB forwarding list has %d items, want 1: the DRB with no usable direction left should be dropped", len(plan.DataForwardingResponseDRB))
	}

	drb := plan.DataForwardingResponseDRB[0]
	if drb.DRBID != 1 {
		t.Errorf("surviving DRB = %d, want 1", drb.DRBID)
	}

	if drb.DLForwardingUPTNLInformation != nil {
		t.Error("relayed a DRB DL forwarding endpoint the source gNB cannot reach")
	}

	if drb.ULForwardingUPTNLInformation == nil || drb.ULForwardingUPTNLInformation.GTPTunnel.GTPTEID != 0xbeef {
		t.Errorf("DRB UL forwarding endpoint = %+v, want the usable one kept", drb.ULForwardingUPTNLInformation)
	}
}
