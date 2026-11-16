// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"testing"

	"github.com/ellanetworks/core/per"
)

func TestResetRoundTripsResetAll(t *testing.T) {
	cause := CauseMiscControlProcessingOverload.Cause()

	in := &Reset{Cause: &cause, ResetType: ResetType{S1Interface: Ptr(ResetAllResetAll)}}

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

	if out.ResetType.S1Interface == nil || *out.ResetType.S1Interface != ResetAllResetAll {
		t.Fatalf("ResetType.S1Interface = %v, want reset-all", out.ResetType.S1Interface)
	}

	if out.ResetType.PartOfS1Interface != nil {
		t.Fatalf("ResetType.PartOfS1Interface = %v, want nil", out.ResetType.PartOfS1Interface)
	}

	if !deref(out.Cause).Equal(cause) {
		t.Fatalf("Cause = %+v, want %+v", out.Cause, cause)
	}
}

func TestResetRoundTripsPartOfInterface(t *testing.T) {
	cause := CauseRadioNetworkUnspecified.Cause()
	mme := MMEUES1APID(42)
	enb := ENBUES1APID(7)

	in := &Reset{
		Cause: &cause,
		ResetType: ResetType{PartOfS1Interface: &UEAssociatedLogicalS1ConnectionListRes{
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

	if out.ResetType.S1Interface != nil {
		t.Fatalf("ResetType.S1Interface = %v, want nil", out.ResetType.S1Interface)
	}

	if out.ResetType.PartOfS1Interface == nil || len(*out.ResetType.PartOfS1Interface) != 2 {
		t.Fatalf("ResetType.PartOfS1Interface = %v, want 2 items", out.ResetType.PartOfS1Interface)
	}

	part := *out.ResetType.PartOfS1Interface

	first := part[0]
	if first.MMEUES1APID == nil || *first.MMEUES1APID != mme ||
		first.ENBUES1APID == nil || *first.ENBUES1APID != enb {
		t.Fatalf("first item = %+v, want mme=%d enb=%d", first, mme, enb)
	}

	second := part[1]
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

// A reset naming no UE association at all is refused: the list is
// SIZE(1..maxnoofIndividualS1ConnectionsToReset), so an empty one cannot be
// encoded.
func TestResetTypeRejectsEmptyPart(t *testing.T) {
	w := per.NewWriter()
	if err := (&ResetType{PartOfS1Interface: &UEAssociatedLogicalS1ConnectionListRes{}}).MarshalPER(w, per.Aligned); err == nil {
		t.Fatal("encode accepted an empty partOfS1-Interface list")
	}
}
