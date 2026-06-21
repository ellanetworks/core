// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"bytes"
	"testing"
)

func TestERABModifyRequestRoundTrips(t *testing.T) {
	qos := ERABLevelQoSParameters{
		QCI: 7,
		ARP: AllocationAndRetentionPriority{
			PriorityLevel:           14,
			PreemptionCapability:    PreemptionShallNotTrigger,
			PreemptionVulnerability: PreemptionNotPreemptable,
		},
	}

	in := &ERABModifyRequest{
		MMEUES1APID: 42,
		ENBUES1APID: 1,
		ERABToBeModified: []ERABToBeModifiedItemBearerModReq{{
			ERABID: 5, QoS: qos, NASPDU: NASPDU{0x02, 0x01, 0xc9},
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
	if !ok || im.ProcedureCode != ProcERABModify {
		t.Fatalf("got %T procedureCode %d", pdu, pdu.procedureCode())
	}

	out, err := ParseERABModifyRequest(im.Value)
	if err != nil {
		t.Fatal(err)
	}

	if out.MMEUES1APID != 42 || out.ENBUES1APID != 1 || len(out.ERABToBeModified) != 1 {
		t.Fatalf("header mismatch: %+v", out)
	}

	gi, go_ := in.ERABToBeModified[0], out.ERABToBeModified[0]
	if gi.ERABID != go_.ERABID || gi.QoS.QCI != go_.QoS.QCI ||
		gi.QoS.ARP.PriorityLevel != go_.QoS.ARP.PriorityLevel || !bytes.Equal(gi.NASPDU, go_.NASPDU) {
		t.Fatalf("E-RAB item mismatch:\n  in  %+v\n  out %+v", gi, go_)
	}
}

func TestERABModifyResponseRoundTrips(t *testing.T) {
	in := &ERABModifyResponse{
		MMEUES1APID: 42,
		ENBUES1APID: 1,
		ERABModify:  []ERABModifyItemBearerModRes{{ERABID: 5}},
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
	if !ok || so.ProcedureCode != ProcERABModify {
		t.Fatalf("got %T procedureCode %d", pdu, pdu.procedureCode())
	}

	out, err := ParseERABModifyResponse(so.Value)
	if err != nil {
		t.Fatal(err)
	}

	if out.MMEUES1APID != 42 || out.ENBUES1APID != 1 ||
		len(out.ERABModify) != 1 || out.ERABModify[0].ERABID != 5 {
		t.Fatalf("mismatch:\n  in  %+v\n  out %+v", in, out)
	}
}
