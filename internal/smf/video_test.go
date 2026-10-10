// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package smf_test

import (
	"context"
	"net/netip"
	"slices"
	"testing"

	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/smf"
)

const videoQFI = 3

func videoRule() smf.PCCRule {
	r := voiceRule()
	r.ID = "af;1#2.1"
	r.QCI = 2
	r.ARP = models.Arp{PriorityLevel: 4, PreemptCap: models.PreemptionCapabilityNotPreempt, PreemptVuln: models.PreemptionVulnerabilityPreemptable}
	r.MBR = models.Ambr{Uplink: models.BitRateFromBps(512000), Downlink: models.BitRateFromBps(512000)}
	r.GBR = r.MBR
	r.Filters = []models.SDFFilter{
		{Direction: models.FilterDownlink, Precedence: 40, Protocol: 17, Remote: netip.MustParsePrefix("192.0.2.10/32"), LocalPort: 50010, RemotePort: 49010},
		{Direction: models.FilterUplink, Precedence: 41, Protocol: 17, Remote: netip.MustParsePrefix("192.0.2.10/32"), LocalPort: 50010, RemotePort: 49010},
	}

	return r
}

func TestActivatingBothBearersKeepsThem(t *testing.T) {
	for _, order := range [][]uint8{{1, 2}, {2, 1}} {
		s, _, _, mmeCb, ref := epsReconcileFixture(t, 0)

		pushRules(t, s, ref, voiceRule(), videoRule())

		acts := mmeCb.dedicatedActivations()

		for i, qci := range order {
			j := slices.IndexFunc(acts, func(a models.DedicatedBearerRequest) bool { return a.QCI == qci })
			if err := s.DedicatedBearerActivated(context.Background(), ref, acts[j].SGW.TEID, uint8(6+i), models.FTEID{TEID: uint32(0x66 + i), Addr: netip.MustParseAddr("10.3.0.3")}); err != nil {
				t.Fatalf("DedicatedBearerActivated: %v", err)
			}
		}

		if d := mmeCb.dedicatedDeactivations(); len(d) != 0 {
			t.Fatalf("activated QCIs %v: deactivations %v, want the voice and video bearers kept", order, d)
		}
	}
}

func TestVideoGetsADedicatedBearerOfItsOwn(t *testing.T) {
	s, _, _, mmeCb, ref := epsReconcileFixture(t, 0)

	pushRules(t, s, ref, voiceRule(), videoRule())

	acts := mmeCb.dedicatedActivations()
	if len(acts) != 2 {
		t.Fatalf("activations %+v, want one bearer for voice and one for video with the same ARP (TS 29.213 §5.4)", acts)
	}

	qcis := []uint8{acts[0].QCI, acts[1].QCI}
	slices.Sort(qcis)

	if !slices.Equal(qcis, []uint8{1, 2}) || acts[0].SGW.TEID == acts[1].SGW.TEID {
		t.Fatalf("activations %+v, want QCI 1 and QCI 2 on their own S-GW endpoints (IR.94 §4.2.1)", acts)
	}
}

func TestLostVideoBearerReportsOnlyTheVideoRule(t *testing.T) {
	s, pcf, _, mmeCb, ref := epsReconcileFixture(t, 0)
	ctx := context.Background()

	pushRules(t, s, ref, voiceRule(), videoRule())

	var voiceTEID, videoTEID uint32

	for i, a := range mmeCb.dedicatedActivations() {
		if a.QCI == 2 {
			videoTEID = a.SGW.TEID
		} else {
			voiceTEID = a.SGW.TEID
		}

		if err := s.DedicatedBearerActivated(ctx, ref, a.SGW.TEID, uint8(6+i), models.FTEID{TEID: uint32(0x66 + i), Addr: netip.MustParseAddr("10.3.0.3")}); err != nil {
			t.Fatalf("DedicatedBearerActivated: %v", err)
		}
	}

	s.DedicatedBearerReleased(ctx, ref, videoTEID)

	if r := waitReportedRules(t, pcf); !slices.Equal(r, []string{"af;1#2.1"}) {
		t.Fatalf("reported rules %v, want only the video rule so the call can continue as voice (IR.94 §2.4.1; TS 29.214 §4.4.6.2)", r)
	}

	if reports := pcf.reportedFailures(); reports[0].cause != smf.BearerReleased {
		t.Fatalf("report %+v, want a bearer release", reports[0])
	}

	if d := mmeCb.dedicatedDeactivations(); len(d) != 0 {
		t.Fatalf("deactivations %v after the video bearer was lost, want the voice bearer on TEID %#x kept", d, voiceTEID)
	}
}

func TestVoiceAndVideoFlowsSetUpTogetherOn5G(t *testing.T) {
	s, pcf, _, amfCb, ref := fiveGVoiceFixture(t)

	pushRules(t, s, ref, voiceRule(), videoRule())

	c := waitFlowModifications(t, amfCb, 1)[0]
	cmd := flowCommand(t, c)

	if len(cmd.QoSFlowDescriptions) != 2 || len(cmd.QoSRules) != 2 {
		t.Fatalf("command %+v, want a QoS rule and flow description for voice and for video", cmd)
	}

	tr := modifyTransfer(t, c)
	fiveQIs := map[uint8]int64{}

	for _, item := range tr.QosFlowAddOrModifyRequest {
		if item.QosFlowLevelQosParameters == nil || item.QosFlowLevelQosParameters.GBRQosInformation == nil {
			t.Fatalf("N2 QoS flow %+v, want GBR QoS Flow Information", item)
		}

		fiveQIs[uint8(item.QosFlowIdentifier)] = int64(item.QosFlowLevelQosParameters.QosCharacteristics.NonDynamic5QI.FiveQI)
	}

	qfiOf := map[int64]uint8{}
	for qfi, fiveQI := range fiveQIs {
		qfiOf[fiveQI] = qfi
	}

	if len(fiveQIs) != 2 || qfiOf[1] == 0 || qfiOf[2] == 0 {
		t.Fatalf("N2 flows %v, want a 5QI 1 and a 5QI 2 flow (NG.114 §4.5.3)", fiveQIs)
	}

	ueAnswers(t, s, ref, true)
	ranAnswers(t, s, ref, []uint8{qfiOf[1]}, []uint8{qfiOf[2]})

	if r := waitReportedRules(t, pcf); !slices.Equal(r, []string{"af;1#2.1"}) {
		t.Fatalf("reported rules %v, want only the refused video rule", r)
	}

	if reports := pcf.reportedFailures(); reports[0].cause != smf.ResourcesNotAllocated {
		t.Fatalf("report %+v, want a resource allocation failure", reports[0])
	}
}

func TestVideoHoldChangesOnlyTheUPF(t *testing.T) {
	s, _, upf, amfCb, ref := fiveGVoiceFixture(t)

	pushRules(t, s, ref, voiceRule(), videoRule())
	waitFlowModifications(t, amfCb, 1)

	ueAnswers(t, s, ref, true)
	ranAnswers(t, s, ref, []uint8{voiceQFI, videoQFI}, nil)

	pushRules(t, s, ref, heldRule(voiceRule()), heldRule(videoRule()))
	requireOneFlowModification(t, amfCb)

	gates := ruleGates(t, upf)
	if len(gates) != 2 {
		t.Fatalf("rule gates %+v, want the voice and video rules'", gates)
	}

	for _, g := range gates {
		if g != (models.GateStatus{DLGate: models.GateClose}) {
			t.Fatalf("gate %+v, want the downlink closed on both streams (IR.94 §2.3.2)", g)
		}
	}
}

func TestHandoverToEPSCarriesTheVoiceAndVideoBearers(t *testing.T) {
	pcf, store, upf, amfCb, mmeCb := interworkingFakes()
	s := newTestSMF(pcf, store, upf, amfCb)
	s.SetMME(mmeCb)

	ref := establish5GSWithEBI(t, s, sessionEBI).Ref

	pushRules(t, s, ref, voiceRule(), videoRule())
	waitFlowModifications(t, amfCb, 1)
	ueAnswers(t, s, ref, true)
	ranAnswers(t, s, ref, []uint8{voiceQFI, videoQFI}, nil)

	bearer, err := s.CreateEPSSession(context.Background(), epsMove(movedPDUSessionID))
	if err != nil {
		t.Fatalf("move to EPS: %v", err)
	}

	if len(bearer.Dedicated) != 2 {
		t.Fatalf("dedicated bearer contexts %+v, want the voice and the video flow (TS 23.502 §4.11.1.2.1 step 2)", bearer.Dedicated)
	}

	byQCI := map[uint8]models.DedicatedBearerContext{}
	for _, d := range bearer.Dedicated {
		byQCI[d.QCI] = d
	}

	voice, video := byQCI[1], byQCI[2]
	if voice.EBI == 0 || video.EBI == 0 || voice.EBI == video.EBI || voice.SGW.TEID == video.SGW.TEID {
		t.Fatalf("dedicated bearer contexts %+v, want QCI 1 and QCI 2 each on its own EBI and S-GW TEID", bearer.Dedicated)
	}

	if r := pcf.reportedRules(); len(r) != 0 {
		t.Fatalf("reported rules %v, want none for flows that survived the move", r)
	}
}
