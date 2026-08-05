// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"encoding/hex"
	"errors"
	"testing"

	"github.com/ellanetworks/core/per"
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

	if out != in {
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
			Kind: QosCharacteristicsNonDynamic5QI, NonDynamic5QI: NonDynamic5QIDescriptor{FiveQI: 9},
		}, goldenQosCharacteristicsNonDynamic},
		{"QosCharacteristicsDynamic", &QosCharacteristics{
			Kind: QosCharacteristicsDynamic5QI, Dynamic5QI: dyn,
		}, goldenQosCharacteristicsDynamic},
		{"QosFlowLevelQosParametersMin", &QosFlowLevelQosParameters{
			QosCharacteristics: QosCharacteristics{
				Kind: QosCharacteristicsNonDynamic5QI, NonDynamic5QI: NonDynamic5QIDescriptor{FiveQI: 9},
			},
			AllocationAndRetentionPriority: arp,
		}, goldenQosFlowLevelQosParametersMin},
		{"QosFlowLevelQosParametersFull", &QosFlowLevelQosParameters{
			QosCharacteristics: QosCharacteristics{
				Kind: QosCharacteristicsDynamic5QI, Dynamic5QI: dyn,
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
		out.PacketErrorRate != in.PacketErrorRate || *out.FiveQI != fiveQI ||
		*out.DelayCritical != delayCritical || *out.AveragingWindow != avg ||
		*out.MaximumDataBurstVolume != mdbv {
		t.Fatalf("round trip %+v, want %+v", out, in)
	}
}

// PriorityLevelQos is (1..127): zero is below the lower bound and must not
// encode as if it were the first root value.
func TestPriorityLevelQosRejectsZero(t *testing.T) {
	w := per.NewWriter()
	if err := PriorityLevelQos(0).MarshalPER(w, per.Aligned); err == nil {
		t.Fatal("encoded a PriorityLevelQos of 0")
	}
}

func TestQosCharacteristicsRejectsUnknownKind(t *testing.T) {
	// Kind 2 is the choice-Extensions index: encoding it would emit that index
	// with a Dynamic5QI body. A far-out-of-range kind would pass a weaker guard.
	for _, kind := range []QosCharacteristicsKind{qosCharacteristicsChoiceExtensions, 7} {
		w := per.NewWriter()
		if err := (QosCharacteristics{Kind: kind}).MarshalPER(w, per.Aligned); err == nil {
			t.Errorf("encoded QosCharacteristics kind %d", kind)
		}
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
			Kind: QosCharacteristicsNonDynamic5QI, NonDynamic5QI: NonDynamic5QIDescriptor{FiveQI: 9},
		},
		AllocationAndRetentionPriority: AllocationAndRetentionPriority{
			PriorityLevelARP:        1,
			PreemptionCapability:    PreemptionShallNotTrigger,
			PreemptionVulnerability: PreemptionNotPreemptable,
		},
	}

	tnl := QosFlowPerTNLInformation{
		UPTransportLayerInformation: UPTransportLayerInformation{
			GTPTunnel: GTPTunnel{
				TransportLayerAddress: TransportLayerAddress{192, 168, 1, 1},
				GTPTEID:               1,
			},
		},
		AssociatedQosFlowList: AssociatedQosFlowList{{QosFlowIdentifier: 3}},
	}

	cause := Cause{Group: CauseGroupRadioNetwork, Value: CauseRadioNetworkUnspecified}

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
				Kind: QosCharacteristicsNonDynamic5QI, NonDynamic5QI: NonDynamic5QIDescriptor{FiveQI: 9},
			},
			AllocationAndRetentionPriority: AllocationAndRetentionPriority{PriorityLevelARP: 1},
		},
	}

	tnl := QosFlowPerTNLInformationItem{QosFlowPerTNLInformation: QosFlowPerTNLInformation{
		UPTransportLayerInformation: UPTransportLayerInformation{GTPTunnel: GTPTunnel{
			TransportLayerAddress: TransportLayerAddress{192, 168, 1, 1}, GTPTEID: 1,
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
		if err := per.EncodeEnumerated(w, per.Aligned, pduSessionTypeRootCount, true, int64(c.v)); err != nil {
			t.Fatal(err)
		}

		if got := hex.EncodeToString(perBytes(w)); got != c.golden {
			t.Fatalf("PDUSessionType %d encoded %s, want %s", c.v, got, c.golden)
		}
	}
}

func TestNetworkInstanceRejectsZero(t *testing.T) {
	w := per.NewWriter()
	if err := NetworkInstance(0).MarshalPER(w, per.Aligned); err == nil {
		t.Fatal("encoded a NetworkInstance of 0, lower bound is 1")
	}
}

// The ENUMERATED root counts are inlined into per_gen.go from the struct tags,
// so these constants document rather than drive. They cannot be pinned by a
// golden either: counts 3 and 4 share an X.691 §11.5.7.1 row, so every root
// value encodes identically under both, and a decoder built one wider simply
// reads fewer bits and misparses downstream instead of erroring. Pinning the
// values against the ASN.1 module is what is left.
func TestQosEnumRootCountsMatchSpec(t *testing.T) {
	for _, c := range []struct {
		name string
		got  int
		want int
	}{
		{"Pre-emptionCapability", preemptionCapabilityRootCount, 2},
		{"Pre-emptionVulnerability", preemptionVulnerabilityRootCount, 2},
		{"NotificationControl", notificationControlRootCount, 1},
		{"DelayCritical", delayCriticalRootCount, 2},
		{"ReflectiveQosAttribute", reflectiveQosAttributeRootCount, 1},
		{"AdditionalQosFlowInformation", additionalQosFlowInformationRootCount, 1},
		{"QosFlowMappingIndication", qosFlowMappingIndicationRootCount, 2},
		{"PDUSessionType", pduSessionTypeRootCount, 5},
		{"IntegrityProtectionIndication", integrityProtectionIndicationRootCount, 3},
		{"ConfidentialityProtectionIndication", confidentialityProtectionIndicationRootCount, 3},
		{"IntegrityProtectionResult", integrityProtectionResultRootCount, 2},
		{"ConfidentialityProtectionResult", confidentialityProtectionResultRootCount, 2},
		{"MaximumIntegrityProtectedDataRate", maximumIntegrityProtectedDataRateRootCount, 2},
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

	err := pdb.UnmarshalPER(r, per.Aligned)
	if err == nil {
		t.Fatal("extension value decoded as a root value")
	}

	if !errors.Is(err, errNotComprehended) {
		t.Fatalf("err = %v, want errNotComprehended so criticality decides", err)
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

// 3GPP defines no extension addition for these constraints, so there is no
// legal out-of-root value to send.
func TestExtensibleIntRefusesOutOfRootOnEncode(t *testing.T) {
	w := per.NewWriter()
	if err := PacketDelayBudget(1024).MarshalPER(w, per.Aligned); err == nil {
		t.Error("encoded a PacketDelayBudget above the root with no addition defined")
	}

	if n := len(perBytes(w)); n != 0 {
		t.Errorf("wrote %d bytes on a refused encode, want none", n)
	}
}

// MaximumDataBurstVolume ::= INTEGER (0..4095, ..., 4096..2000000): the
// extension branch is unconstrained on the wire, so both ends need bounding.
func TestMaximumDataBurstVolumeIsBounded(t *testing.T) {
	w := per.NewWriter()
	if err := MaximumDataBurstVolume(2000001).MarshalPER(w, per.Aligned); err == nil {
		t.Error("encoded a value above the extension range ceiling")
	}

	// Extension bit, one length octet, then 0xff — two's complement -1.
	neg := per.NewWriter()
	neg.WriteBit(true)
	neg.AlignToByte()
	neg.WriteBits(0x01ff, 16)

	var m MaximumDataBurstVolume
	if err := m.UnmarshalPER(per.NewReader(neg.Bytes()), per.Aligned); err == nil {
		t.Errorf("decoded a negative extension value as %d", uint32(m))
	}
}
