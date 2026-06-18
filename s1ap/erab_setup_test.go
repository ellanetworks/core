// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"bytes"
	"testing"
)

func TestERABSetupRequestRoundTrips(t *testing.T) {
	qos := ERABLevelQoSParameters{
		QCI: 9,
		ARP: AllocationAndRetentionPriority{
			PriorityLevel:           15,
			PreemptionCapability:    PreemptionShallNotTrigger,
			PreemptionVulnerability: PreemptionNotPreemptable,
		},
	}

	tla := TransportLayerAddress{10, 45, 0, 1}
	ambr := UEAggregateMaximumBitRate{DL: 1000000000, UL: 500000000}

	in := &ERABSetupRequest{
		MMEUES1APID:               42,
		ENBUES1APID:               1,
		UEAggregateMaximumBitRate: &ambr,
		ERABToBeSetup: []ERABToBeSetupItemBearerSUReq{{
			ERABID: 6, QoS: qos, TransportLayerAddress: tla, GTPTEID: 0x12345678,
			NASPDU: NASPDU{0x02, 0x01, 0xc1},
		}},
	}

	b, err := in.Marshal()
	if err != nil {
		t.Fatal(err)
	}

	pdu, err := Unmarshal(b)
	if err != nil {
		t.Fatal(err)
	}

	im, ok := pdu.(*InitiatingMessage)
	if !ok || im.ProcedureCode != ProcERABSetup {
		t.Fatalf("got %T procedureCode %d", pdu, pdu.procedureCode())
	}

	out, err := ParseERABSetupRequest(im.Value)
	if err != nil {
		t.Fatal(err)
	}

	if out.MMEUES1APID != 42 || out.ENBUES1APID != 1 || len(out.ERABToBeSetup) != 1 {
		t.Fatalf("header mismatch: %+v", out)
	}

	if out.UEAggregateMaximumBitRate == nil || *out.UEAggregateMaximumBitRate != ambr {
		t.Fatalf("UE-AMBR mismatch: %+v", out.UEAggregateMaximumBitRate)
	}

	gi, go_ := in.ERABToBeSetup[0], out.ERABToBeSetup[0]
	if gi.ERABID != go_.ERABID || gi.GTPTEID != go_.GTPTEID || gi.QoS.QCI != go_.QoS.QCI ||
		!bytes.Equal(gi.TransportLayerAddress, go_.TransportLayerAddress) || !bytes.Equal(gi.NASPDU, go_.NASPDU) {
		t.Fatalf("E-RAB item mismatch:\n  in  %+v\n  out %+v", gi, go_)
	}
}

func TestERABSetupResponseRoundTrips(t *testing.T) {
	tla := TransportLayerAddress{10, 45, 0, 9}

	in := &ERABSetupResponse{
		MMEUES1APID: 42,
		ENBUES1APID: 1,
		ERABSetup:   []ERABSetupItemBearerSURes{{ERABID: 6, TransportLayerAddress: tla, GTPTEID: 0xdeadbeef}},
	}

	b, err := in.Marshal()
	if err != nil {
		t.Fatal(err)
	}

	pdu, err := Unmarshal(b)
	if err != nil {
		t.Fatal(err)
	}

	so, ok := pdu.(*SuccessfulOutcome)
	if !ok || so.ProcedureCode != ProcERABSetup {
		t.Fatalf("got %T procedureCode %d", pdu, pdu.procedureCode())
	}

	out, err := ParseERABSetupResponse(so.Value)
	if err != nil {
		t.Fatal(err)
	}

	if out.MMEUES1APID != 42 || out.ENBUES1APID != 1 || len(out.ERABSetup) != 1 ||
		out.ERABSetup[0].ERABID != 6 || out.ERABSetup[0].GTPTEID != 0xdeadbeef ||
		!bytes.Equal(out.ERABSetup[0].TransportLayerAddress, tla) {
		t.Fatalf("mismatch:\n  in  %+v\n  out %+v", in, out)
	}
}
