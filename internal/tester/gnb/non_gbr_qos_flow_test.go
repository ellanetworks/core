// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package gnb

import (
	"errors"
	"net/netip"
	"slices"
	"sync"
	"testing"

	"github.com/ellanetworks/core/internal/models"
	smfngap "github.com/ellanetworks/core/internal/smf/ngap"
	"github.com/ellanetworks/core/internal/tester/air"
	"github.com/ellanetworks/core/ngap"
)

const (
	testRANUEID     = 7
	testPDUSession  = 1
	defaultTestQFI  = 1
	mcpttTestQFI    = 2
	voiceTestQFI    = 3
	mcpttTestFiveQI = 69
)

type silentUE struct{}

func (silentUE) SendDownlinkNAS([]byte, int64, int64) error { return nil }

func (silentUE) RRCRelease() {}

func newQoSTestGnodeB() *GnodeB {
	g := &GnodeB{
		N3Address: netip.MustParseAddr("10.3.0.10"),
		UEPool:    map[int64]air.DownlinkSender{testRANUEID: silentUE{}},
		n2Active:  -1,
	}
	g.cond = sync.NewCond(&g.mu)

	return g
}

func mcpttFlow() smfngap.QoSFlow {
	return smfngap.QoSFlow{
		QFI:    mcpttTestQFI,
		FiveQI: mcpttTestFiveQI,
		ARP:    models.Arp{PriorityLevel: 2, PreemptCap: models.PreemptionCapabilityMayPreempt, PreemptVuln: models.PreemptionVulnerabilityNotPreemptable},
		ERABID: 6,
	}
}

func voiceFlow() smfngap.QoSFlow {
	return smfngap.QoSFlow{
		QFI:    voiceTestQFI,
		FiveQI: 1,
		ARP:    models.Arp{PriorityLevel: 3},
		MFBR:   models.Ambr{Uplink: models.MustParseBitRate("128 Kbps"), Downlink: models.MustParseBitRate("128 Kbps")},
		GFBR:   models.Ambr{Uplink: models.MustParseBitRate("64 Kbps"), Downlink: models.MustParseBitRate("64 Kbps")},
	}
}

func TestSetupRequestTransferKeepsTheDefaultFlowApartFromNonGBRDedicatedFlows(t *testing.T) {
	g := newQoSTestGnodeB()

	transfer, err := smfngap.BuildPDUSessionResourceSetupRequestTransfer(
		&models.Ambr{Uplink: models.MustParseBitRate("100 Mbps"), Downlink: models.MustParseBitRate("200 Mbps")},
		&models.QosData{QFI: defaultTestQFI, Var5qi: 9, Arp: &models.Arp{PriorityLevel: 1}},
		42, netip.MustParseAddr("10.3.0.2"), netip.Addr{}, ngap.PDUSessionTypeIPv4, 5,
		[]smfngap.QoSFlow{mcpttFlow(), voiceFlow()},
	)
	if err != nil {
		t.Fatalf("build PDU Session Resource Setup Request Transfer: %v", err)
	}

	info, err := getPDUSessionInfoFromSetupRequestTransfer(g, transfer)
	if err != nil {
		t.Fatalf("getPDUSessionInfoFromSetupRequestTransfer: %v", err)
	}

	if info.QFI != defaultTestQFI || info.FiveQi != 9 || info.PriArp != 1 {
		t.Errorf("default flow = QFI %d 5QI %d ARP %d, want QFI %d 5QI 9 ARP 1", info.QFI, info.FiveQi, info.PriArp, defaultTestQFI)
	}

	if !slices.Equal(info.Flows, []uint8{mcpttTestQFI, voiceTestQFI}) {
		t.Errorf("dedicated flows = %v, want [%d %d]", info.Flows, mcpttTestQFI, voiceTestQFI)
	}

	info.PDUSessionID = testPDUSession
	g.storePDUSession(testRANUEID, info)

	mcptt, ok := g.DedicatedQoSFlow(testRANUEID, testPDUSession, mcpttTestFiveQI)
	if !ok {
		t.Fatalf("DedicatedQoSFlow found no 5QI %d flow among %v", mcpttTestFiveQI, g.QoSFlows(testRANUEID, testPDUSession))
	}

	if mcptt.QFI != mcpttTestQFI || mcptt.PriArp != 2 || mcptt.PreemptCap != ngap.PreemptionMayTrigger || mcptt.GBR {
		t.Errorf("MCPTT flow = %+v, want QFI %d, ARP 2, may preempt, no GBR QoS Information", mcptt, mcpttTestQFI)
	}

	voice, ok := g.DedicatedQoSFlow(testRANUEID, testPDUSession, 1)
	if !ok || voice.QFI != voiceTestQFI || !voice.GBR {
		t.Errorf("voice flow = %+v (found %t), want QFI %d with GBR QoS Information", voice, ok, voiceTestQFI)
	}

	if _, ok := g.DedicatedQoSFlow(testRANUEID, testPDUSession, 9); ok {
		t.Error("DedicatedQoSFlow reported the default flow as a dedicated one")
	}
}

func modifyRequest(t *testing.T, flows ...smfngap.QoSFlow) []byte {
	t.Helper()

	transfer, err := smfngap.BuildQoSFlowsModifyRequestTransfer(flows, nil)
	if err != nil {
		t.Fatalf("build PDU Session Resource Modify Request Transfer: %v", err)
	}

	wire, err := (&ngap.PDUSessionResourceModifyRequest{
		AMFUENGAPID: 11,
		RANUENGAPID: testRANUEID,
		PDUSessionResourceModify: ngap.PDUSessionResourceModifyListModReq{{
			PDUSessionID: testPDUSession,
			Transfer:     transfer,
		}},
	}).Marshal()
	if err != nil {
		t.Fatalf("marshal PDU Session Resource Modify Request: %v", err)
	}

	pdu, err := ngap.Unmarshal(wire)
	if err != nil {
		t.Fatalf("unmarshal PDU Session Resource Modify Request: %v", err)
	}

	im, ok := pdu.(*ngap.InitiatingMessage)
	if !ok {
		t.Fatalf("PDU Session Resource Modify Request decoded as %T", pdu)
	}

	return im.Value
}

func handleModify(t *testing.T, g *GnodeB, value []byte) {
	t.Helper()

	if err := handlePDUSessionResourceModifyRequest(g, value); !errors.Is(err, ErrNoActivePeer) {
		t.Fatalf("handlePDUSessionResourceModifyRequest = %v, want it to reach the response, which has no N2 peer here", err)
	}
}

func TestModifyRequestAdmitsANonGBRQoSFlow(t *testing.T) {
	g := newQoSTestGnodeB()
	g.storePDUSession(testRANUEID, &PDUSessionInformation{PDUSessionID: testPDUSession, QFI: defaultTestQFI, QosId: defaultTestQFI, FiveQi: 9})

	handleModify(t, g, modifyRequest(t, mcpttFlow()))

	if flows := g.QoSFlows(testRANUEID, testPDUSession); !slices.Equal(flows, []uint8{mcpttTestQFI}) {
		t.Fatalf("admitted flows = %v, want [%d]", flows, mcpttTestQFI)
	}

	if g.sessionQFI(testRANUEID, testPDUSession) != defaultTestQFI {
		t.Errorf("default QFI = %d, want %d: a dedicated flow must not replace the default one", g.sessionQFI(testRANUEID, testPDUSession), defaultTestQFI)
	}

	flow, ok := g.DedicatedQoSFlow(testRANUEID, testPDUSession, mcpttTestFiveQI)
	if !ok || flow.QFI != mcpttTestQFI || flow.PriArp != 2 || flow.GBR {
		t.Errorf("MCPTT flow = %+v (found %t), want QFI %d, ARP 2 and no GBR QoS Information", flow, ok, mcpttTestQFI)
	}
}

func TestGBRRefusalSparesNonGBRQoSFlows(t *testing.T) {
	g := newQoSTestGnodeB()
	g.storePDUSession(testRANUEID, &PDUSessionInformation{PDUSessionID: testPDUSession, QFI: defaultTestQFI, QosId: defaultTestQFI, FiveQi: 9})

	cause := ngap.Cause{Group: ngap.CauseGroupRadioNetwork, Value: ngap.CauseRadioNetworkUnspecified}
	g.RefuseNextGBRQoSFlows(testRANUEID, cause)

	handleModify(t, g, modifyRequest(t, mcpttFlow()))

	if flows := g.QoSFlows(testRANUEID, testPDUSession); !slices.Equal(flows, []uint8{mcpttTestQFI}) {
		t.Fatalf("admitted flows = %v, want the non-GBR flow %d", flows, mcpttTestQFI)
	}

	handleModify(t, g, modifyRequest(t, voiceFlow()))

	if flows := g.QoSFlows(testRANUEID, testPDUSession); slices.Contains(flows, voiceTestQFI) {
		t.Fatalf("admitted flows = %v, want the GBR flow %d refused", flows, voiceTestQFI)
	}

	if _, armed := g.takeGBRFlowRefusal(testRANUEID); armed {
		t.Error("the refusal must be spent on the GBR flow it refused")
	}
}
