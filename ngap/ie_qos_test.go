// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"encoding/hex"
	"reflect"
	"testing"

	"github.com/ellanetworks/core/per"
	"github.com/ellanetworks/core/per/ap"
)

const (
	// Extension bit, absent iE-Extensions, priorityLevelARP 1 over (1..15) is
	// 0, then two extensible ENUMERATEDs each contributing an extension bit and
	// a one-bit index — all zero.
	goldenAllocationAndRetentionPriority = "0000"

	// Extension bit, four absent-optional bits — notificationControl, the two
	// maximum packet loss rates and iE-Extensions — then four extensible
	// BitRates: each an extension bit, a length determinant and the value.
	goldenGBRQosInformation = "008003e81007d010012c100190"
)

func TestAllocationAndRetentionPriorityGolden(t *testing.T) {
	in := AllocationAndRetentionPriority{
		PriorityLevelARP:        1,
		PreemptionCapability:    PreemptionShallNotTrigger,
		PreemptionVulnerability: PreemptionNotPreemptable,
	}

	w := per.NewWriter()
	if err := in.MarshalPER(w, per.Aligned); err != nil {
		t.Fatal(err)
	}

	if got := hex.EncodeToString(perBytes(w)); got != goldenAllocationAndRetentionPriority {
		t.Fatalf("encoded %s, want %s", got, goldenAllocationAndRetentionPriority)
	}

	out, err := unmarshalPERValue[AllocationAndRetentionPriority](perBytes(w))
	if err != nil {
		t.Fatal(err)
	}

	if !out.Equal(in) {
		t.Fatalf("round trip %+v, want %+v", out, in)
	}
}

func TestGBRQosInformationGolden(t *testing.T) {
	in := GBRQosInformation{
		MaximumFlowBitRateDL:    1000,
		MaximumFlowBitRateUL:    2000,
		GuaranteedFlowBitRateDL: 300,
		GuaranteedFlowBitRateUL: 400,
	}

	w := per.NewWriter()
	if err := in.MarshalPER(w, per.Aligned); err != nil {
		t.Fatal(err)
	}

	if got := hex.EncodeToString(perBytes(w)); got != goldenGBRQosInformation {
		t.Fatalf("encoded %s, want %s", got, goldenGBRQosInformation)
	}

	out, err := unmarshalPERValue[GBRQosInformation](perBytes(w))
	if err != nil {
		t.Fatal(err)
	}

	if out.MaximumFlowBitRateDL != in.MaximumFlowBitRateDL || out.GuaranteedFlowBitRateUL != in.GuaranteedFlowBitRateUL {
		t.Fatalf("round trip %+v, want %+v", out, in)
	}
}

// PriorityLevelARP is INTEGER (1..15) where S1AP's PriorityLevel is (0..15), so
// 0 is not a value this IE can carry.
func TestPriorityLevelARPRejectsZero(t *testing.T) {
	w := per.NewWriter()
	if err := PriorityLevelARP(0).MarshalPER(w, per.Aligned); err == nil {
		t.Fatal("encoded a priority level below the lower bound")
	}
}

const (
	// Extension bit, a four-bit absent-optional bitmap (priorityLevelQos,
	// averagingWindow, maximumDataBurstVolume, iE-Extensions), the fiveQI
	// extension bit, then fiveQI aligned into its own octet (range 256 is the
	// one-octet case, X.691 §11.5.7.2).
	goldenNonDynamic5QIMin  = "0009"
	goldenNonDynamic5QIFull = "7009070007d0000fff"

	// Extension bit, a five-bit absent-optional bitmap, priorityLevelQos as
	// 5-1=4 in a seven-bit field (range 127, §11.5.7.1), then packetDelayBudget
	// as 100-0=100 in two aligned octets (range 1024, §11.5.7.3) — the lower
	// bound is 0, so the value encodes as itself.
	goldenDynamic5QI = "001000640260"

	// Choice index over three alternatives in two bits, then the descriptor.
	goldenQosCharacteristicsNonDynamic = "0009"
	goldenQosCharacteristicsDynamic    = "40040000640260"

	goldenQosFlowLevelQosParametersMin  = "0000090000"
	goldenQosFlowLevelQosParametersFull = "3200200064026000"

	goldenPacketErrorRate = "0260"

	// A root value spends a clear extension bit and two aligned octets; a value
	// above the root sets the bit and switches to the unconstrained encoding,
	// a length octet followed by the minimal big-endian value.
	goldenMaximumDataBurstVolumeRoot = "000fff"
	goldenMaximumDataBurstVolumeExt  = "80030186a0"
)

func TestQosGoldens(t *testing.T) {
	prio8 := PriorityLevelQos(8)
	avg2000 := AveragingWindow(2000)
	mdbv4095 := MaximumDataBurstVolume(4095)
	reflective := ReflectiveQosSubjectTo
	additional := AdditionalQosFlowMoreLikely

	arp := AllocationAndRetentionPriority{
		PriorityLevelARP:        1,
		PreemptionCapability:    PreemptionShallNotTrigger,
		PreemptionVulnerability: PreemptionNotPreemptable,
	}

	dyn := Dynamic5QIDescriptor{
		PriorityLevelQos:  5,
		PacketDelayBudget: 100,
		PacketErrorRate:   PacketErrorRate{Scalar: 1, Exponent: 6},
	}

	for _, c := range []struct {
		name   string
		in     per.Marshaler
		golden string
	}{
		{"NonDynamic5QIMin", &NonDynamic5QIDescriptor{FiveQI: 9}, goldenNonDynamic5QIMin},
		{"NonDynamic5QIFull", &NonDynamic5QIDescriptor{
			FiveQI: 9, PriorityLevelQos: &prio8, AveragingWindow: &avg2000, MaximumDataBurstVolume: &mdbv4095,
		}, goldenNonDynamic5QIFull},
		{"Dynamic5QI", &dyn, goldenDynamic5QI},
		{"PacketErrorRate", &PacketErrorRate{Scalar: 1, Exponent: 6}, goldenPacketErrorRate},
		{"MaximumDataBurstVolumeRoot", mdbv4095, goldenMaximumDataBurstVolumeRoot},
		{"MaximumDataBurstVolumeExt", MaximumDataBurstVolume(100000), goldenMaximumDataBurstVolumeExt},
		{"QosCharacteristicsNonDynamic", &QosCharacteristics{
			NonDynamic5QI: &NonDynamic5QIDescriptor{FiveQI: 9},
		}, goldenQosCharacteristicsNonDynamic},
		{"QosCharacteristicsDynamic", &QosCharacteristics{
			Dynamic5QI: &dyn,
		}, goldenQosCharacteristicsDynamic},
		{"QosFlowLevelQosParametersMin", &QosFlowLevelQosParameters{
			QosCharacteristics: QosCharacteristics{
				NonDynamic5QI: &NonDynamic5QIDescriptor{FiveQI: 9},
			},
			AllocationAndRetentionPriority: arp,
		}, goldenQosFlowLevelQosParametersMin},
		{"QosFlowLevelQosParametersFull", &QosFlowLevelQosParameters{
			QosCharacteristics: QosCharacteristics{
				Dynamic5QI: &dyn,
			},
			AllocationAndRetentionPriority: arp,
			ReflectiveQosAttribute:         &reflective,
			AdditionalQosFlowInformation:   &additional,
		}, goldenQosFlowLevelQosParametersFull},
	} {
		t.Run(c.name, func(t *testing.T) {
			w := per.NewWriter()
			if err := c.in.MarshalPER(w, per.Aligned); err != nil {
				t.Fatal(err)
			}

			if got := hex.EncodeToString(perBytes(w)); got != c.golden {
				t.Fatalf("encoded %s, want %s", got, c.golden)
			}
		})
	}
}

func TestDynamic5QIRoundTrip(t *testing.T) {
	fiveQI := FiveQI(9)
	delayCritical := DelayCriticalYes
	avg := AveragingWindow(2000)
	mdbv := MaximumDataBurstVolume(100000)

	in := Dynamic5QIDescriptor{
		PriorityLevelQos:       127,
		PacketDelayBudget:      1023,
		PacketErrorRate:        PacketErrorRate{Scalar: 9, Exponent: 9},
		FiveQI:                 &fiveQI,
		DelayCritical:          &delayCritical,
		AveragingWindow:        &avg,
		MaximumDataBurstVolume: &mdbv,
	}

	w := per.NewWriter()
	if err := in.MarshalPER(w, per.Aligned); err != nil {
		t.Fatal(err)
	}

	out, err := unmarshalPERValue[Dynamic5QIDescriptor](perBytes(w))
	if err != nil {
		t.Fatal(err)
	}

	if out.PriorityLevelQos != in.PriorityLevelQos || out.PacketDelayBudget != in.PacketDelayBudget ||
		!out.PacketErrorRate.Equal(in.PacketErrorRate) || *out.FiveQI != fiveQI ||
		*out.DelayCritical != delayCritical || *out.AveragingWindow != avg ||
		*out.MaximumDataBurstVolume != mdbv {
		t.Fatalf("round trip %+v, want %+v", out, in)
	}
}

func TestQosCharacteristicsRejectsNoAlternative(t *testing.T) {
	if err := (&QosCharacteristics{}).MarshalPER(per.NewWriter(), per.Aligned); err == nil {
		t.Error("encoded a QosCharacteristics with no alternative")
	}
}

func TestQosCharacteristicsKeepsChoiceExtension(t *testing.T) {
	in := QosCharacteristics{ChoiceExtension: &SingleContainer{ID: 999, Criticality: CriticalityIgnore, Value: "\x01"}}

	b, err := per.Marshal(&in, per.Aligned)
	if err != nil {
		t.Fatal(err)
	}

	var out QosCharacteristics
	if err := per.Unmarshal(b, &out); err != nil {
		t.Fatalf("decode is lenient, got %v", err)
	}

	if out.ChoiceExtension == nil || *out.ChoiceExtension != *in.ChoiceExtension {
		t.Fatalf("choice-Extensions = %+v, want %+v", out.ChoiceExtension, in.ChoiceExtension)
	}

	d := ap.Diagnose[ProtocolIEID](&out)
	if len(d.IEs) != 1 || d.IEs[0].ID != 999 || d.IEs[0].Kind != ap.KindNotComprehended {
		t.Fatalf("diagnostics = %+v, want the unknown alternative", d.IEs)
	}
}

const (
	// Extension bit, a two-bit absent-optional bitmap (e-RAB-ID,
	// iE-Extensions), qosFlowIdentifier 1 in six bits (range 64), then the
	// QosFlowLevelQosParameters.
	goldenQosFlowSetupRequestItem     = "004000090000"
	goldenQosFlowSetupRequestItemERAB = "4fc00009000a"
	goldenQosFlowSetupRequestList1    = "00010000090000"

	goldenQosFlowWithCauseItem  = "010000"
	goldenQosFlowListWithCause1 = "00040000"

	// Extension bit, a two-bit absent-optional bitmap, then qosFlowIdentifier 3
	// in six bits: 0 00 0 000011, padded to 0x00 0xc0.
	goldenAssociatedQosFlowItem   = "00c0"
	goldenAssociatedQosFlowItemDL = "40d0"
	goldenAssociatedQosFlowList1  = "0003"

	goldenQosFlowIdentifier = "0a"

	goldenQosFlowPerTNLInformation      = "007cc0a80101000000010003"
	goldenQosFlowPerTNLInformationItem  = "001fc0a80101000000010003"
	goldenQosFlowPerTNLInformationList1 = "0007c0c0a80101000000010003"
)

func TestQosFlowListGoldens(t *testing.T) {
	erab := ERABID(5)
	dl := QosFlowMappingDL

	qp := QosFlowLevelQosParameters{
		QosCharacteristics: QosCharacteristics{
			NonDynamic5QI: &NonDynamic5QIDescriptor{FiveQI: 9},
		},
		AllocationAndRetentionPriority: AllocationAndRetentionPriority{
			PriorityLevelARP:        1,
			PreemptionCapability:    PreemptionShallNotTrigger,
			PreemptionVulnerability: PreemptionNotPreemptable,
		},
	}

	tnl := QosFlowPerTNLInformation{
		UPTransportLayerInformation: UPTransportLayerInformation{
			GTPTunnel: &GTPTunnel{
				TransportLayerAddress: tlaOf(192, 168, 1, 1),
				GTPTEID:               1,
			},
		},
		AssociatedQosFlowList: AssociatedQosFlowList{{QosFlowIdentifier: 3}},
	}

	cause := CauseRadioNetworkUnspecified.Cause()

	for _, c := range []struct {
		name   string
		in     per.Marshaler
		golden string
	}{
		{"QosFlowIdentifier", QosFlowIdentifier(5), goldenQosFlowIdentifier},
		{"SetupRequestItem", &QosFlowSetupRequestItem{
			QosFlowIdentifier: 1, QosFlowLevelQosParameters: qp,
		}, goldenQosFlowSetupRequestItem},
		{"SetupRequestItemERAB", &QosFlowSetupRequestItem{
			QosFlowIdentifier: 63, QosFlowLevelQosParameters: qp, ERABID: &erab,
		}, goldenQosFlowSetupRequestItemERAB},
		{"SetupRequestList", QosFlowSetupRequestList{{
			QosFlowIdentifier: 1, QosFlowLevelQosParameters: qp,
		}}, goldenQosFlowSetupRequestList1},
		{"WithCauseItem", &QosFlowWithCauseItem{
			QosFlowIdentifier: 2, Cause: cause,
		}, goldenQosFlowWithCauseItem},
		{"ListWithCause", QosFlowListWithCause{{
			QosFlowIdentifier: 2, Cause: cause,
		}}, goldenQosFlowListWithCause1},
		{"AssociatedItem", &AssociatedQosFlowItem{QosFlowIdentifier: 3}, goldenAssociatedQosFlowItem},
		{"AssociatedItemDL", &AssociatedQosFlowItem{
			QosFlowIdentifier: 3, QosFlowMappingIndication: &dl,
		}, goldenAssociatedQosFlowItemDL},
		{"AssociatedList", AssociatedQosFlowList{{QosFlowIdentifier: 3}}, goldenAssociatedQosFlowList1},
		{"PerTNLInformation", &tnl, goldenQosFlowPerTNLInformation},
		{"PerTNLInformationItem", &QosFlowPerTNLInformationItem{
			QosFlowPerTNLInformation: tnl,
		}, goldenQosFlowPerTNLInformationItem},
		{"PerTNLInformationList", QosFlowPerTNLInformationList{{
			QosFlowPerTNLInformation: tnl,
		}}, goldenQosFlowPerTNLInformationList1},
	} {
		t.Run(c.name, func(t *testing.T) {
			w := per.NewWriter()
			if err := c.in.MarshalPER(w, per.Aligned); err != nil {
				t.Fatal(err)
			}

			if got := hex.EncodeToString(perBytes(w)); got != c.golden {
				t.Fatalf("encoded %s, want %s", got, c.golden)
			}
		})
	}
}

// A list over its bound must be refused rather than encoded with a truncated
// count. The items are valid ones: a zero-valued QosFlowSetupRequestItem fails
// on its own PriorityLevelARP of 0, which would make the assertion pass without
// the length bound ever being consulted.
func TestQosFlowListsRejectOversizedAndEmpty(t *testing.T) {
	item := QosFlowSetupRequestItem{
		QosFlowIdentifier: 1,
		QosFlowLevelQosParameters: QosFlowLevelQosParameters{
			QosCharacteristics: QosCharacteristics{
				NonDynamic5QI: &NonDynamic5QIDescriptor{FiveQI: 9},
			},
			AllocationAndRetentionPriority: AllocationAndRetentionPriority{PriorityLevelARP: 1},
		},
	}

	tnl := QosFlowPerTNLInformationItem{QosFlowPerTNLInformation: QosFlowPerTNLInformation{
		UPTransportLayerInformation: UPTransportLayerInformation{GTPTunnel: &GTPTunnel{
			TransportLayerAddress: tlaOf(192, 168, 1, 1), GTPTEID: 1,
		}},
		AssociatedQosFlowList: AssociatedQosFlowList{{QosFlowIdentifier: 3}},
	}}

	// The item must encode on its own, or the oversized assertions below prove
	// nothing about the bound.
	for name, m := range map[string]per.Marshaler{
		"QosFlowSetupRequestList":      QosFlowSetupRequestList{item},
		"QosFlowPerTNLInformationList": QosFlowPerTNLInformationList{tnl},
	} {
		w := per.NewWriter()
		if err := m.MarshalPER(w, per.Aligned); err != nil {
			t.Fatalf("%s: single valid item did not encode: %v", name, err)
		}
	}

	oversized := make(QosFlowSetupRequestList, maxnoofQosFlows+1)
	for i := range oversized {
		oversized[i] = item
	}

	w := per.NewWriter()
	if err := oversized.MarshalPER(w, per.Aligned); err == nil {
		t.Errorf("encoded %d QosFlowSetupRequestItems, bound is %d", len(oversized), maxnoofQosFlows)
	}

	w = per.NewWriter()
	if err := (QosFlowSetupRequestList{}).MarshalPER(w, per.Aligned); err == nil {
		t.Error("encoded an empty QosFlowSetupRequestList, lower bound is 1")
	}

	tnls := make(QosFlowPerTNLInformationList, maxnoofMultiConnectivityMinusOne+1)
	for i := range tnls {
		tnls[i] = tnl
	}

	w = per.NewWriter()
	if err := tnls.MarshalPER(w, per.Aligned); err == nil {
		t.Errorf("encoded %d QosFlowPerTNLInformationItems, bound is %d", len(tnls), maxnoofMultiConnectivityMinusOne)
	}
}

// The list bounds come from the NGAP-Constants module. A wrong bound is not
// always visible in a golden — the length determinant for 1..3 and 1..4 is two
// bits either way — so the values are pinned literally here.
func TestNGAPConstantsMatchSpec(t *testing.T) {
	for _, c := range []struct {
		name string
		got  int
		want int
	}{
		{"maxnoofQosFlows", maxnoofQosFlows, 64},
		{"maxnoofMultiConnectivity", maxnoofMultiConnectivity, 4},
		{"maxnoofMultiConnectivityMinusOne", maxnoofMultiConnectivityMinusOne, 3},
		{"maxnoofPDUSessions", maxnoofPDUSessions, 256},
		{"maxnoofAllowedSNSSAIs", maxnoofAllowedSNSSAIs, 8},
		{"maxnoofSliceItems", maxnoofSliceItems, 1024},
		{"maxnoofEPLMNs", maxnoofEPLMNs, 15},
		{"maxnoofEPLMNsPlusOne", maxnoofEPLMNsPlusOne, 16},
		{"maxnoofForbTACs", maxnoofForbTACs, 4096},
		{"maxnoofAllowedAreas", maxnoofAllowedAreas, 16},
	} {
		if c.got != c.want {
			t.Errorf("%s is %d, TS 38.413 NGAP-Constants says %d", c.name, c.got, c.want)
		}
	}
}

const (
	goldenNetworkInstance1   = "0000"
	goldenNetworkInstance256 = "00ff"

	goldenPDUSessionAggregateMaximumBitRate = "0403e81007d0"

	// Extension bit, a two-bit absent-optional bitmap, then two extensible
	// three-alternative ENUMERATEDs each spending an extension bit and a
	// two-bit index.
	goldenSecurityIndicationMin = "0000"
	// maximumIntegrityProtectedDataRate-UL present: 0 1 0 | 0 01 | 0 10 | 0 1.
	goldenSecurityIndicationFull = "4520"

	goldenSecurityResult = "04"
)

func TestSessionAndSecurityGoldens(t *testing.T) {
	maxUERate := MaximumIntegrityProtectedDataRateMaximumUERate

	for _, c := range []struct {
		name   string
		in     per.Marshaler
		golden string
	}{
		{"NetworkInstance1", NetworkInstance(1), goldenNetworkInstance1},
		{"NetworkInstance256", NetworkInstance(256), goldenNetworkInstance256},
		{"PDUSessionAMBR", &PDUSessionAggregateMaximumBitRate{DL: 1000, UL: 2000}, goldenPDUSessionAggregateMaximumBitRate},
		{"SecurityIndicationMin", &SecurityIndication{
			IntegrityProtectionIndication:       IntegrityProtectionRequired,
			ConfidentialityProtectionIndication: ConfidentialityProtectionRequired,
		}, goldenSecurityIndicationMin},
		{"SecurityIndicationFull", &SecurityIndication{
			IntegrityProtectionIndication:       IntegrityProtectionPreferred,
			ConfidentialityProtectionIndication: ConfidentialityProtectionNotNeeded,
			MaximumIntegrityProtectedDataRateUL: &maxUERate,
		}, goldenSecurityIndicationFull},
		{"SecurityResult", &SecurityResult{
			IntegrityProtectionResult:       IntegrityProtectionPerformed,
			ConfidentialityProtectionResult: ConfidentialityProtectionNotPerformed,
		}, goldenSecurityResult},
	} {
		t.Run(c.name, func(t *testing.T) {
			w := per.NewWriter()
			if err := c.in.MarshalPER(w, per.Aligned); err != nil {
				t.Fatal(err)
			}

			if got := hex.EncodeToString(perBytes(w)); got != c.golden {
				t.Fatalf("encoded %s, want %s", got, c.golden)
			}
		})
	}
}

// PDUSessionType has no container in the library yet, so its root count is
// pinned directly: a wrong count silently shifts every index on the wire.
func TestPDUSessionTypeRootCount(t *testing.T) {
	for _, c := range []struct {
		v      PDUSessionType
		golden string
	}{
		{PDUSessionTypeIPv4, "00"},
		{PDUSessionTypeUnstructured, "40"},
	} {
		w := per.NewWriter()
		if err := per.EncodeEnumerated(w, per.Aligned, int64(len(enumPDUSessionType.Root)), true, int64(c.v)); err != nil {
			t.Fatal(err)
		}

		if got := hex.EncodeToString(perBytes(w)); got != c.golden {
			t.Fatalf("PDUSessionType %d encoded %s, want %s", c.v, got, c.golden)
		}
	}
}

func TestQosEnumRootCountsMatchSpec(t *testing.T) {
	for _, c := range []struct {
		name string
		got  int64
		want int64
	}{
		{"Pre-emptionCapability", int64(len(enumPreemptionCapability.Root)), 2},
		{"Pre-emptionVulnerability", int64(len(enumPreemptionVulnerability.Root)), 2},
		{"NotificationControl", int64(len(enumNotificationControl.Root)), 1},
		{"DelayCritical", int64(len(enumDelayCritical.Root)), 2},
		{"ReflectiveQosAttribute", int64(len(enumReflectiveQosAttribute.Root)), 1},
		{"AdditionalQosFlowInformation", int64(len(enumAdditionalQosFlowInformation.Root)), 1},
		{"QosFlowMappingIndication", int64(len(enumQosFlowMappingIndication.Root)), 2},
		{"PDUSessionType", int64(len(enumPDUSessionType.Root)), 5},
		{"IntegrityProtectionIndication", int64(len(enumIntegrityProtectionIndication.Root)), 3},
		{"ConfidentialityProtectionIndication", int64(len(enumConfidentialityProtectionIndication.Root)), 3},
		{"IntegrityProtectionResult", int64(len(enumIntegrityProtectionResult.Root)), 2},
		{"ConfidentialityProtectionResult", int64(len(enumConfidentialityProtectionResult.Root)), 2},
		{"MaximumIntegrityProtectedDataRate", int64(len(enumMaximumIntegrityProtectedDataRate.Root)), 2},
	} {
		if c.got != c.want {
			t.Errorf("%s root count is %d, TS 38.413 ASN.1 says %d", c.name, c.got, c.want)
		}
	}
}

// X.691 §13.1 sends an out-of-root value as an unconstrained integer, which
// §13.2.6 prefixes with its own length determinant. The field is therefore
// skippable, and the decoder must consume it: reporting not-comprehended while
// leaving the reader mid-field would desynchronise everything after it.
func TestExtensibleIntConsumesExtensionValue(t *testing.T) {
	w := per.NewWriter()

	// PacketDelayBudget ::= INTEGER (0..1023, ...) carrying 1024, i.e. the
	// first value a later release could add beyond the root.
	if err := per.EncodeInteger(w, per.Aligned, per.Bounds{LB: 0, HasLB: true, UB: 1023, HasUB: true, Extensible: true}, 1024); err != nil {
		t.Fatal(err)
	}

	if err := QosFlowIdentifier(5).MarshalPER(w, per.Aligned); err != nil {
		t.Fatal(err)
	}

	w.AlignToByte()

	r := per.NewReader(w.Bytes())

	var pdb PacketDelayBudget

	if err := pdb.UnmarshalPER(r, per.Aligned); err != nil {
		t.Fatal(err)
	}

	if pdb != 1024 || !pdb.Diagnose(&ap.Diagnosis[ProtocolIEID]{}) {
		t.Fatalf("PacketDelayBudget = %d, want 1024 kept and reported not comprehended", pdb)
	}

	// The payload for the next field is only reachable if the extension value
	// was consumed in full.
	var qfi QosFlowIdentifier
	if err := qfi.UnmarshalPER(r, per.Aligned); err != nil {
		t.Fatalf("reader left mid-field: %v", err)
	}

	if qfi != 5 {
		t.Fatalf("next field decoded as %d, want 5 — reader was misaligned", qfi)
	}
}

func TestExtensibleIntOutOfRootValuesRoundTripAndAreNotComprehended(t *testing.T) {
	for _, v := range []interface {
		per.Marshaler
		ap.Diagnosable[ProtocolIEID]
	}{
		PriorityLevelQos(0),
		NetworkInstance(0),
		PacketDelayBudget(1024),
		DRBID(0),
		FiveQI(255),
		MaximumDataBurstVolume(2000001),
	} {
		w := per.NewWriter()
		if err := v.MarshalPER(w, per.Aligned); err != nil {
			t.Fatalf("%T(%v): %v", v, v, err)
		}

		out := reflect.New(reflect.TypeOf(v))

		if err := out.Interface().(per.Unmarshaler).UnmarshalPER(per.NewReader(perBytes(w)), per.Aligned); err != nil {
			t.Fatalf("%T: decode: %v", v, err)
		}

		if got := out.Elem().Interface(); got != v {
			t.Errorf("%T round trip = %v, want %v", v, got, v)
		}

		_, isFiveQI := v.(FiveQI)
		if got := v.Diagnose(&ap.Diagnosis[ProtocolIEID]{}); got == isFiveQI {
			t.Errorf("%T(%v).Diagnose() = %v", v, v, got)
		}
	}
}

func TestMaximumDataBurstVolumeExtensionRange(t *testing.T) {
	if MaximumDataBurstVolume(2000000).Diagnose(&ap.Diagnosis[ProtocolIEID]{}) {
		t.Error("the top of the extension range is a known value")
	}

	neg := per.NewWriter()
	neg.WriteBit(true)
	neg.AlignToByte()
	neg.WriteBits(0x01ff, 16)

	var m MaximumDataBurstVolume
	if err := m.UnmarshalPER(per.NewReader(neg.Bytes()), per.Aligned); err == nil {
		t.Errorf("decoded a negative extension value as %d", uint32(m))
	}
}
