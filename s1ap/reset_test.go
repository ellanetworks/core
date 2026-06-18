// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import "testing"

func TestResetRoundTripsResetAll(t *testing.T) {
	cause := Cause{Group: CauseGroupMisc, Value: 0}

	in := &Reset{Cause: cause, ResetType: ResetType{All: true}}

	b, err := in.Marshal()
	if err != nil {
		t.Fatal(err)
	}

	pdu, err := Unmarshal(b)
	if err != nil {
		t.Fatal(err)
	}

	im, ok := pdu.(*InitiatingMessage)
	if !ok || im.ProcedureCode != ProcReset {
		t.Fatalf("got %T procedureCode %d", pdu, pdu.procedureCode())
	}

	out, err := ParseReset(im.Value)
	if err != nil {
		t.Fatal(err)
	}

	if !out.ResetType.All {
		t.Fatalf("ResetType.All = false, want true")
	}

	if len(out.ResetType.Part) != 0 {
		t.Fatalf("ResetType.Part = %v, want empty", out.ResetType.Part)
	}

	if out.Cause != cause {
		t.Fatalf("Cause = %+v, want %+v", out.Cause, cause)
	}
}

func TestResetRoundTripsPartOfInterface(t *testing.T) {
	cause := Cause{Group: CauseGroupRadioNetwork, Value: 0}
	mme := MMEUES1APID(42)
	enb := ENBUES1APID(7)

	in := &Reset{
		Cause: cause,
		ResetType: ResetType{Part: []UEAssociatedLogicalS1ConnectionItem{
			{MMEUES1APID: &mme, ENBUES1APID: &enb},
			{ENBUES1APID: &enb},
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

	out, err := ParseReset(pdu.(*InitiatingMessage).Value)
	if err != nil {
		t.Fatal(err)
	}

	if out.ResetType.All {
		t.Fatalf("ResetType.All = true, want false")
	}

	if len(out.ResetType.Part) != 2 {
		t.Fatalf("ResetType.Part length = %d, want 2", len(out.ResetType.Part))
	}

	first := out.ResetType.Part[0]
	if first.MMEUES1APID == nil || *first.MMEUES1APID != mme ||
		first.ENBUES1APID == nil || *first.ENBUES1APID != enb {
		t.Fatalf("first item = %+v, want mme=%d enb=%d", first, mme, enb)
	}

	second := out.ResetType.Part[1]
	if second.MMEUES1APID != nil || second.ENBUES1APID == nil || *second.ENBUES1APID != enb {
		t.Fatalf("second item = %+v, want only enb=%d", second, enb)
	}
}

func TestResetAcknowledgeRoundTripsNoList(t *testing.T) {
	b, err := (&ResetAcknowledge{}).Marshal()
	if err != nil {
		t.Fatal(err)
	}

	pdu, err := Unmarshal(b)
	if err != nil {
		t.Fatal(err)
	}

	so, ok := pdu.(*SuccessfulOutcome)
	if !ok || so.ProcedureCode != ProcReset {
		t.Fatalf("got %T procedureCode %d", pdu, pdu.procedureCode())
	}

	out, err := ParseResetAcknowledge(so.Value)
	if err != nil {
		t.Fatal(err)
	}

	if len(out.ConnectionList) != 0 {
		t.Fatalf("ConnectionList = %v, want empty", out.ConnectionList)
	}
}

func TestResetAcknowledgeRoundTripsWithList(t *testing.T) {
	mme := MMEUES1APID(3)
	enb := ENBUES1APID(9)

	in := &ResetAcknowledge{ConnectionList: []UEAssociatedLogicalS1ConnectionItem{
		{MMEUES1APID: &mme, ENBUES1APID: &enb},
	}}

	b, err := in.Marshal()
	if err != nil {
		t.Fatal(err)
	}

	pdu, err := Unmarshal(b)
	if err != nil {
		t.Fatal(err)
	}

	out, err := ParseResetAcknowledge(pdu.(*SuccessfulOutcome).Value)
	if err != nil {
		t.Fatal(err)
	}

	if len(out.ConnectionList) != 1 {
		t.Fatalf("ConnectionList length = %d, want 1", len(out.ConnectionList))
	}

	it := out.ConnectionList[0]
	if it.MMEUES1APID == nil || *it.MMEUES1APID != mme ||
		it.ENBUES1APID == nil || *it.ENBUES1APID != enb {
		t.Fatalf("item = %+v, want mme=%d enb=%d", it, mme, enb)
	}
}
