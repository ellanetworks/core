// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"encoding/hex"
	"testing"
)

const goldenTraceStart = "001b4023000003000000020001000800020002001940100002f8390a0b0c1234f800f80a000001"

var goldenTraceID = EUTRANTraceID{0x02, 0xf8, 0x39, 0x0a, 0x0b, 0x0c, 0x12, 0x34}

func goldenTraceActivation() TraceActivation {
	return TraceActivation{
		EUTRANTraceID:                  goldenTraceID,
		InterfacesToTrace:              InterfaceToTraceS1MME | InterfaceToTraceX2 | InterfaceToTraceUu | InterfaceToTraceF1C | InterfaceToTraceE1,
		TraceDepth:                     TraceDepthMinimum,
		TraceCollectionEntityIPAddress: TransportLayerAddress{10, 0, 0, 1},
	}
}

func TestTraceStartGolden(t *testing.T) {
	ta := goldenTraceActivation()

	in := TraceStart{MMEUES1APID: 1, ENBUES1APID: 2, TraceActivation: &ta}

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

	if out.MMEUES1APID != 1 || out.ENBUES1APID != 2 || out.TraceActivation == nil {
		t.Fatalf("round trip %+v", out)
	}

	got := *out.TraceActivation
	if got.EUTRANTraceID != ta.EUTRANTraceID || got.InterfacesToTrace != ta.InterfacesToTrace ||
		got.TraceDepth != ta.TraceDepth || hex.EncodeToString(got.TraceCollectionEntityIPAddress) != "0a000001" {
		t.Fatalf("round trip TraceActivation %+v, want %+v", got, ta)
	}
}

func TestTraceStartRequiresTraceActivation(t *testing.T) {
	in := TraceStart{MMEUES1APID: 1, ENBUES1APID: 2}

	if _, err := in.Marshal(); err == nil {
		t.Fatal("encoded a TraceStart without Trace Activation")
	}
}

func TestTraceFailureIndicationRoundTrip(t *testing.T) {
	cause := Cause{Group: CauseGroupRadioNetwork, Value: CauseRadioNetworkHandoverCancelled}
	id := goldenTraceID

	in := TraceFailureIndication{MMEUES1APID: 1, ENBUES1APID: 2, EUTRANTraceID: &id, Cause: &cause}

	b, err := in.Marshal()
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

	if out.MMEUES1APID != 1 || out.ENBUES1APID != 2 || out.EUTRANTraceID == nil || *out.EUTRANTraceID != goldenTraceID ||
		out.Cause == nil || *out.Cause != cause {
		t.Fatalf("decoded %+v", out)
	}
}

func TestPeekMMEUES1APID(t *testing.T) {
	b, _ := hex.DecodeString(goldenTraceStart)

	pdu, err := Unmarshal(b)
	if err != nil {
		t.Fatal(err)
	}

	id, ok := PeekMMEUES1APID(pdu)
	if !ok || id != 1 {
		t.Fatalf("peeked %d %v, want 1", id, ok)
	}
}

func TestPeekMMEUES1APIDRejectsNilAndReadsSourceID(t *testing.T) {
	if _, ok := PeekMMEUES1APID(nil); ok {
		t.Fatal("peeked an ID from a nil PDU")
	}

	b, err := (&PathSwitchRequest{
		ENBUES1APID:       9,
		SourceMMEUES1APID: 77,
		ERABToBeSwitchedDL: []ERABToBeSwitchedDLItem{{
			ERABID:                5,
			TransportLayerAddress: TransportLayerAddress{10, 1, 2, 3},
			GTPTEID:               1,
		}},
		EUTRANCGI:              &EUTRANCGI{PLMNIdentity: PLMNIdentity{0x00, 0xf1, 0x10}},
		TAI:                    &TAI{PLMNIdentity: PLMNIdentity{0x00, 0xf1, 0x10}, TAC: 1},
		UESecurityCapabilities: &UESecurityCapabilities{EncryptionAlgorithms: 0xc000, IntegrityProtectionAlgorithms: 0xc000},
	}).Marshal()
	if err != nil {
		t.Fatal(err)
	}

	pdu, err := Unmarshal(b)
	if err != nil {
		t.Fatal(err)
	}

	if id, ok := PeekMMEUES1APID(pdu); !ok || id != 77 {
		t.Fatalf("peeked %d %v, want the Source MME-UE-S1AP-ID 77", id, ok)
	}

	if _, ok := PeekMMEUES1APID(&InitiatingMessage{ProcedureCode: ProcTraceStart, Value: []byte{0x00, 0x00}}); ok {
		t.Fatal("peeked an ID from a truncated message")
	}
}

func TestHandoverRequestTraceActivationRoundTrip(t *testing.T) {
	ta := goldenTraceActivation()

	in := &HandoverRequest{
		MMEUES1APID:  7,
		HandoverType: HandoverTypeIntraLTE,
		Cause:        Ptr(Cause{Group: CauseGroupRadioNetwork, Value: CauseRadioNetworkHandoverCancelled}),
		UEAMBR:       UEAggregateMaximumBitRate{DL: 1_000_000, UL: 500_000},
		ERABToBeSetup: []ERABToBeSetupItemHOReq{{
			ERABID:                5,
			TransportLayerAddress: TransportLayerAddress{10, 1, 2, 3},
			GTPTEID:               0x01020304,
			QoS:                   ERABLevelQoSParameters{QCI: 9, ARP: AllocationAndRetentionPriority{PriorityLevel: 15}},
		}},
		SourceToTarget:         TransparentContainer{0xaa, 0xbb},
		UESecurityCapabilities: UESecurityCapabilities{EncryptionAlgorithms: 0xc000, IntegrityProtectionAlgorithms: 0xc000},
		TraceActivation:        &ta,
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

	if out.TraceActivation == nil || out.TraceActivation.EUTRANTraceID != goldenTraceID {
		t.Fatalf("round trip TraceActivation %+v", out.TraceActivation)
	}
}
