// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import "testing"

func TestENBConfigurationUpdateRoundTrips(t *testing.T) {
	drx := PagingDRX(2)

	in := &ENBConfigurationUpdate{
		ENBName: "enb-updated",
		SupportedTAs: SupportedTAs{{
			TAC:            7,
			BroadcastPLMNs: []PLMNIdentity{{0x00, 0xf1, 0x10}},
		}},
		DefaultPagingDRX: &drx,
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
	if !ok || im.ProcedureCode != ProcENBConfigurationUpdate {
		t.Fatalf("got %T procedureCode %d", pdu, pdu.procedureCode())
	}

	out, err := ParseENBConfigurationUpdate(im.Value)
	if err != nil {
		t.Fatal(err)
	}

	if out.ENBName != "enb-updated" || len(out.SupportedTAs) != 1 || out.SupportedTAs[0].TAC != 7 {
		t.Fatalf("mismatch: %+v", out)
	}

	if out.DefaultPagingDRX == nil || *out.DefaultPagingDRX != drx {
		t.Fatalf("DRX mismatch: %+v", out.DefaultPagingDRX)
	}
}

func TestENBConfigurationUpdateAcknowledgeRoundTrips(t *testing.T) {
	b, err := (&ENBConfigurationUpdateAcknowledge{}).Marshal()
	if err != nil {
		t.Fatal(err)
	}

	pdu, err := Unmarshal(b)
	if err != nil {
		t.Fatal(err)
	}

	so, ok := pdu.(*SuccessfulOutcome)
	if !ok || so.ProcedureCode != ProcENBConfigurationUpdate {
		t.Fatalf("got %T procedureCode %d", pdu, pdu.procedureCode())
	}

	if _, err := ParseENBConfigurationUpdateAcknowledge(so.Value); err != nil {
		t.Fatal(err)
	}
}

func TestENBConfigurationUpdateFailureRoundTrips(t *testing.T) {
	in := &ENBConfigurationUpdateFailure{Cause: Cause{Group: CauseGroupMisc, Value: 5}}

	b, err := in.Marshal()
	if err != nil {
		t.Fatal(err)
	}

	pdu, err := Unmarshal(b)
	if err != nil {
		t.Fatal(err)
	}

	uo, ok := pdu.(*UnsuccessfulOutcome)
	if !ok || uo.ProcedureCode != ProcENBConfigurationUpdate {
		t.Fatalf("got %T procedureCode %d", pdu, pdu.procedureCode())
	}

	out, err := ParseENBConfigurationUpdateFailure(uo.Value)
	if err != nil {
		t.Fatal(err)
	}

	if out.Cause != in.Cause {
		t.Fatalf("cause mismatch: %+v", out.Cause)
	}
}
