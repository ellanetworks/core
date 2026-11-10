// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"encoding/hex"
	"testing"
)

const (
	goldenTraceStart             = "00274023000003000a00020001005500020002006c40100002f8390a0b0c1234f800f80a000001"
	goldenTraceFailureIndication = "00264021000004000a00020001005500020002002c400802f8390a0b0c1234000f40020140"
)

var goldenTraceID = NGRANTraceID{0x02, 0xf8, 0x39, 0x0a, 0x0b, 0x0c, 0x12, 0x34}

func goldenTraceActivation() TraceActivation {
	return TraceActivation{
		NGRANTraceID:                   goldenTraceID,
		InterfacesToTrace:              InterfaceToTraceNGC | InterfaceToTraceXnC | InterfaceToTraceUu | InterfaceToTraceF1C | InterfaceToTraceE1,
		TraceDepth:                     TraceDepthMinimum,
		TraceCollectionEntityIPAddress: TransportLayerAddress{10, 0, 0, 1},
	}
}

func TestTraceStartGolden(t *testing.T) {
	ta := goldenTraceActivation()

	in := TraceStart{AMFUENGAPID: 1, RANUENGAPID: 2, TraceActivation: &ta}

	b, err := in.Marshal()
	if err != nil {
		t.Fatal(err)
	}

	if got := hex.EncodeToString(b); got != goldenTraceStart {
		t.Fatalf("encoded %s, want %s", got, goldenTraceStart)
	}

	pdu, err := Unmarshal(b)
	if err != nil {
		t.Fatal(err)
	}

	im, ok := pdu.(*InitiatingMessage)
	if !ok || im.ProcedureCode != ProcTraceStart || im.Criticality != CriticalityIgnore {
		t.Fatalf("decoded %+v, want an initiating TraceStart with ignore criticality", pdu)
	}

	out, err := ParseTraceStart(im.Value)
	if err != nil {
		t.Fatal(err)
	}

	if out.AMFUENGAPID != 1 || out.RANUENGAPID != 2 || out.TraceActivation == nil {
		t.Fatalf("round trip %+v", out)
	}

	got := *out.TraceActivation
	if got.NGRANTraceID != ta.NGRANTraceID || got.InterfacesToTrace != ta.InterfacesToTrace ||
		got.TraceDepth != ta.TraceDepth || hex.EncodeToString(got.TraceCollectionEntityIPAddress) != "0a000001" {
		t.Fatalf("round trip TraceActivation %+v, want %+v", got, ta)
	}
}

func TestTraceStartRequiresTraceActivation(t *testing.T) {
	in := TraceStart{AMFUENGAPID: 1, RANUENGAPID: 2}

	if _, err := in.Marshal(); err == nil {
		t.Fatal("encoded a TraceStart without Trace Activation")
	}
}

func TestTraceActivationRequiresTCEAddress(t *testing.T) {
	ta := goldenTraceActivation()
	ta.TraceCollectionEntityIPAddress = nil

	in := TraceStart{AMFUENGAPID: 1, RANUENGAPID: 2, TraceActivation: &ta}

	if _, err := in.Marshal(); err == nil {
		t.Fatal("encoded a Trace Activation without a TCE address")
	}
}

func TestTraceFailureIndicationGolden(t *testing.T) {
	b, err := hex.DecodeString(goldenTraceFailureIndication)
	if err != nil {
		t.Fatal(err)
	}

	pdu, err := Unmarshal(b)
	if err != nil {
		t.Fatal(err)
	}

	im, ok := pdu.(*InitiatingMessage)
	if !ok || im.ProcedureCode != ProcTraceFailureIndication {
		t.Fatalf("decoded %T, want an initiating TraceFailureIndication", pdu)
	}

	out, err := ParseTraceFailureIndication(im.Value)
	if err != nil {
		t.Fatal(err)
	}

	want := Cause{Group: CauseGroupRadioNetwork, Value: CauseRadioNetworkHandoverCancelled}

	if out.AMFUENGAPID != 1 || out.RANUENGAPID != 2 || out.NGRANTraceID == nil || *out.NGRANTraceID != goldenTraceID ||
		out.Cause == nil || *out.Cause != want {
		t.Fatalf("decoded %+v", out)
	}

	re, err := out.Marshal()
	if err != nil {
		t.Fatal(err)
	}

	if got := hex.EncodeToString(re); got != goldenTraceFailureIndication {
		t.Fatalf("re-encoded %s, want %s", got, goldenTraceFailureIndication)
	}
}

func TestHandoverRequestTraceActivationRoundTrip(t *testing.T) {
	ta := goldenTraceActivation()

	in := HandoverRequest{
		AMFUENGAPID: 1, HandoverType: HandoverTypeIntra5GS,
		Cause:                     &Cause{Group: CauseGroupRadioNetwork, Value: CauseRadioNetworkHandoverDesirableForRadio},
		UEAggregateMaximumBitRate: UEAggregateMaximumBitRate{DL: 1000000000, UL: 1000000000},
		UESecurityCapabilities:    UESecurityCapabilities{NREncryptionAlgorithms: 0x8000, NRIntegrityProtectionAlgorithms: 0x8000},
		SecurityContext:           SecurityContext{NextHopChainingCount: 1},
		PDUSessionResourceSetupListHOReq: PDUSessionResourceSetupListHOReq{{
			PDUSessionID: 5, SNSSAI: SNSSAI{SST: 1}, Transfer: handoverSetupTransfer(t),
		}},
		AllowedNSSAI:                       AllowedNSSAI{{SNSSAI: SNSSAI{SST: 1}}},
		TraceActivation:                    &ta,
		SourceToTargetTransparentContainer: SourceToTargetTransparentContainer{0xaa, 0xbb},
		GUAMI:                              GUAMI{PLMNIdentity: PLMNIdentity{0x02, 0xf8, 0x39}, AMFRegionID: 1, AMFSetID: 1},
	}

	b, err := in.Marshal()
	if err != nil {
		t.Fatal(err)
	}

	pdu, err := Unmarshal(b)
	if err != nil {
		t.Fatal(err)
	}

	out, err := ParseHandoverRequest(pdu.(*InitiatingMessage).Value)
	if err != nil {
		t.Fatal(err)
	}

	if out.TraceActivation == nil || out.TraceActivation.NGRANTraceID != goldenTraceID {
		t.Fatalf("round trip TraceActivation %+v", out.TraceActivation)
	}
}

func TestPeekAMFUENGAPID(t *testing.T) {
	b, _ := hex.DecodeString(goldenTraceFailureIndication)

	pdu, err := Unmarshal(b)
	if err != nil {
		t.Fatal(err)
	}

	id, ok := PeekAMFUENGAPID(pdu)
	if !ok || id != 1 {
		t.Fatalf("peeked %d %v, want 1", id, ok)
	}

	if _, ok := PeekAMFUENGAPID(nil); ok {
		t.Fatal("peeked an ID from a nil PDU")
	}
}
