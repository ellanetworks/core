// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package smf_test

import (
	"bytes"
	"context"
	"net/netip"
	"slices"
	"testing"

	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/smf"
	"github.com/ellanetworks/core/nas/eps"
	"github.com/ellanetworks/core/nas/fgs"
	libngap "github.com/ellanetworks/core/ngap"
)

var floorControlARP = models.Arp{PriorityLevel: 3, PreemptCap: models.PreemptionCapabilityMayPreempt, PreemptVuln: models.PreemptionVulnerabilityPreemptable}

func floorControlRule() smf.PCCRule {
	rate := models.BitRateFromBps(8000)

	return smf.PCCRule{
		ID:  "af;mcptt#2.1",
		QCI: 69,
		ARP: floorControlARP,
		MBR: models.Ambr{Uplink: rate, Downlink: rate},
		Filters: []models.SDFFilter{
			{Direction: models.FilterDownlink, Precedence: 32, Protocol: 17, Remote: netip.MustParsePrefix("192.0.2.10/32"), LocalPort: 50002, RemotePort: 49002},
			{Direction: models.FilterUplink, Precedence: 33, Protocol: 17, Remote: netip.MustParsePrefix("192.0.2.10/32"), LocalPort: 50002, RemotePort: 49002},
		},
	}
}

func TestNonGBRRuleActivatesADedicatedBearer_TS23401_5_4_1(t *testing.T) {
	s, pcf, upf, mmeCb, ref := epsReconcileFixture(t, 0)
	ctx := context.Background()

	pushRules(t, s, ref, floorControlRule())

	acts := mmeCb.dedicatedActivations()
	if len(acts) != 1 {
		t.Fatalf("activations %+v, want one for the QCI 69 rule", acts)
	}

	b := acts[0].Bearer
	if b.QCI != 69 || b.ARP != floorControlARP || !b.MBR.Equal(models.Ambr{}) || !b.GBR.Equal(models.Ambr{}) || len(b.Filters) != 2 {
		t.Fatalf("bearer %+v, want QCI 69 at the rule's ARP with both filters and no bearer bit rates", b)
	}

	if !slices.ContainsFunc(lastModify(t, upf).UpdateQERs, func(q models.QER) bool { return q.QERID >= 256 && q.MBR != nil && q.MBR.ULMBR == 8 }) {
		t.Fatal("the rule's 8 kbps MBR is not enforced in the UPF (TS 29.212 §4.5.5.3)")
	}

	if err := s.DedicatedBearerActivated(ctx, ref, dedicatedUplinkTEID, 6, models.FTEID{TEID: 0x66, Addr: netip.MustParseAddr("10.3.0.3")}); err != nil {
		t.Fatalf("DedicatedBearerActivated: %v", err)
	}

	faster := floorControlRule()
	faster.Version = 1
	faster.MBR = models.Ambr{Uplink: models.BitRateFromBps(16000), Downlink: models.BitRateFromBps(16000)}
	pushRules(t, s, ref, faster)

	if mods := mmeCb.dedicatedModifications(); len(mods) != 0 {
		t.Fatalf("modifications %+v, want none: a non-GBR bearer signals no bit rate", mods)
	}

	emergency := faster
	emergency.Version = 2
	emergency.ARP = models.Arp{PriorityLevel: 1, PreemptCap: models.PreemptionCapabilityMayPreempt, PreemptVuln: models.PreemptionVulnerabilityNotPreemptable}
	pushRules(t, s, ref, emergency)

	mod := waitDedicatedModifications(t, mmeCb, 1)[0]
	if mod.ARP == nil || *mod.ARP != emergency.ARP || mod.Rates != nil || mod.Operation != eps.TFTIgnore {
		t.Fatalf("modification %+v, want only the ARP changed in place (TS 23.379 §5.7.3)", mod)
	}

	if n := len(mmeCb.dedicatedActivations()); n != 1 {
		t.Fatalf("%d activations, want the bearer kept", n)
	}

	if r := pcf.reportedRules(); len(r) != 0 {
		t.Fatalf("reported rules %v, want none", r)
	}
}

func TestNonGBRRuleCreatesAQoSFlow_TS24501_9_11_4_12(t *testing.T) {
	s, pcf, _, amfCb, ref := fiveGVoiceFixture(t)

	pushRules(t, s, ref, floorControlRule())

	c := waitFlowModifications(t, amfCb, 1)[0]
	cmd := flowCommand(t, c)

	if len(cmd.QoSRules) != 1 || cmd.QoSRules[0].Parameters == nil || cmd.QoSRules[0].Parameters.QFI != voiceQFI {
		t.Fatalf("QoS rules %+v, want one on QFI %d", cmd.QoSRules, voiceQFI)
	}

	if len(cmd.QoSFlowDescriptions) != 1 {
		t.Fatalf("QoS flow descriptions %+v, want one", cmd.QoSFlowDescriptions)
	}

	for _, p := range cmd.QoSFlowDescriptions[0].Parameters {
		if p.ID != fgs.QoSFlowParam5QI || !bytes.Equal(p.Value, []byte{69}) {
			t.Fatalf("QoS flow description %+v, want only 5QI 69 (TS 24.501 §9.11.4.12)", cmd.QoSFlowDescriptions[0])
		}
	}

	item := modifyTransfer(t, c).QosFlowAddOrModifyRequest[0]
	wantARP := libngap.AllocationAndRetentionPriority{
		PriorityLevelARP:        3,
		PreemptionCapability:    libngap.PreemptionMayTrigger,
		PreemptionVulnerability: libngap.PreemptionPreemptable,
	}

	if q := item.QosFlowLevelQosParameters; q == nil || q.GBRQosInformation != nil || q.QosCharacteristics.NonDynamic5QI.FiveQI != 69 || q.AllocationAndRetentionPriority != wantARP {
		t.Fatalf("N2 QoS flow %+v, want 5QI 69 at ARP %+v without GBR QoS Flow Information (TS 38.413 §9.3.1.12)", item, wantARP)
	}

	ueAnswers(t, s, ref, true)
	ranAnswers(t, s, ref, []uint8{voiceQFI}, nil)

	if err := s.DeactivateSmContext(context.Background(), ref, false); err != nil {
		t.Fatalf("DeactivateSmContext: %v", err)
	}

	if req := reactivate(t, s, ref, voiceQFI); !slices.ContainsFunc(req.QosFlowSetupRequest, func(i libngap.QosFlowSetupRequestItem) bool {
		return i.QosFlowIdentifier == voiceQFI
	}) {
		t.Fatalf("Setup Request Transfer flows %+v, want the non-GBR flow kept across the AN release (TS 23.502 §4.2.6)", req.QosFlowSetupRequest)
	}

	if r := pcf.reportedRules(); len(r) != 0 {
		t.Fatalf("reported rules %v, want the non-GBR rule kept across the AN release", r)
	}
}

func TestInterworkingNonGBRFlowCarriesItsEPSBearer_TS23502_4_11_1_1(t *testing.T) {
	s, amfCb, ref := interworkingVoiceFixture(t)

	pushRules(t, s, ref, floorControlRule())

	c := waitFlowModifications(t, amfCb, 1)[0]
	cmd := flowCommand(t, c)

	if ebi, ok := cmd.QoSFlowDescriptions[0].EPSBearerID(); !ok || ebi != flowEBI {
		t.Fatalf("QoS flow description EBI %d (present %v), want its own EPS bearer %d (TS 23.502 §4.11.1.1 NOTE 1)", ebi, ok, flowEBI)
	}

	if len(cmd.MappedEPSBearerContexts) != 1 {
		t.Fatalf("mapped EPS bearer contexts %+v, want one", cmd.MappedEPSBearerContexts)
	}

	qos, err := eps.ParseEPSQoS(mappedParameter(t, cmd.MappedEPSBearerContexts[0], fgs.EPSParameterMappedEPSQoS))
	if err != nil {
		t.Fatalf("mapped EPS QoS: %v", err)
	}

	if _, ok := qos.GBRBitRates(); qos.QCI != 69 || ok {
		t.Fatalf("mapped EPS QoS %+v, want QCI 69 without bit rates (TS 24.301 §9.9.4.3)", qos)
	}

	if item := modifyTransfer(t, c).QosFlowAddOrModifyRequest[0]; item.ERABID == nil || *item.ERABID != flowEBI {
		t.Fatalf("N2 QoS flow %+v, want E-RAB ID %d", item, flowEBI)
	}
}

func TestNonGBRDedicatedBearerCarriesItsQoSFlowForN1Mode_TS24301_6_4_2_2(t *testing.T) {
	s, mmeCb, ref := interworkingEPSVoice(t)

	pushRules(t, s, ref, floorControlRule())

	activations := mmeCb.dedicatedActivations()
	if len(activations) != 1 {
		t.Fatalf("%d activations, want one", len(activations))
	}

	rules, flows := mappedFiveGSQoS(t, activations[0].MappedFiveGSQoS)
	if len(rules) != 1 || len(flows) != 1 || flows[0].QFI != rules[0].Parameters.QFI {
		t.Fatalf("QoS rules %+v and flows %+v, want the rule on its own QoS flow", rules, flows)
	}

	for _, p := range flows[0].Parameters {
		if p.ID == fgs.QoSFlowParamGFBRUplink || p.ID == fgs.QoSFlowParamMFBRUplink {
			t.Fatalf("QoS flow description %+v, want no flow bit rates for 5QI 69 (TS 24.501 §9.11.4.12)", flows[0])
		}
	}
}
