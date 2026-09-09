// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import "testing"

func TestMMEConfigurationUpdateRoundTrips(t *testing.T) {
	in := &MMEConfigurationUpdate{
		MMEName: Ptr("ella"),
		ServedGUMMEIs: ServedGUMMEIs{{
			ServedPLMNs:    []PLMNIdentity{{0x00, 0xf1, 0x10}},
			ServedGroupIDs: []MMEGroupID{{0x80, 0x01}},
			ServedMMECs:    []MMECode{3},
		}},
		RelativeMMECapacity: Ptr(uint8(0)),
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
	if !ok || im.ProcedureCode != ProcMMEConfigurationUpdate {
		t.Fatalf("got %T procedureCode %d", pdu, pdu.procedureCode())
	}

	if im.Criticality != CriticalityReject {
		t.Fatalf("criticality = %v, want reject", im.Criticality)
	}

	out, err := ParseMMEConfigurationUpdate(im.Value)
	if err != nil {
		t.Fatal(err)
	}

	if deref(out.MMEName) != "ella" {
		t.Fatalf("MMEName = %q", deref(out.MMEName))
	}

	if out.RelativeMMECapacity == nil || *out.RelativeMMECapacity != 0 {
		t.Fatalf("RelativeMMECapacity = %v, want 0", out.RelativeMMECapacity)
	}

	if len(out.ServedGUMMEIs) != 1 || len(out.ServedGUMMEIs[0].ServedMMECs) != 1 || out.ServedGUMMEIs[0].ServedMMECs[0] != 3 {
		t.Fatalf("ServedGUMMEIs mismatch: %+v", out.ServedGUMMEIs)
	}
}

func TestMMEConfigurationUpdateCapacityOnly(t *testing.T) {
	b, err := (&MMEConfigurationUpdate{RelativeMMECapacity: Ptr(uint8(0))}).Marshal()
	if err != nil {
		t.Fatal(err)
	}

	pdu, err := Unmarshal(b)
	if err != nil {
		t.Fatal(err)
	}

	im, ok := pdu.(*InitiatingMessage)
	if !ok {
		t.Fatalf("got %T", pdu)
	}

	out, err := ParseMMEConfigurationUpdate(im.Value)
	if err != nil {
		t.Fatal(err)
	}

	if out.MMEName != nil || len(out.ServedGUMMEIs) != 0 {
		t.Fatalf("unexpected IEs: %+v", out)
	}

	if out.RelativeMMECapacity == nil || *out.RelativeMMECapacity != 0 {
		t.Fatalf("RelativeMMECapacity = %v, want 0", out.RelativeMMECapacity)
	}
}

func TestMMEConfigurationUpdateAcknowledgeRoundTrips(t *testing.T) {
	b, err := (&MMEConfigurationUpdateAcknowledge{}).Marshal()
	if err != nil {
		t.Fatal(err)
	}

	pdu, err := Unmarshal(b)
	if err != nil {
		t.Fatal(err)
	}

	so, ok := pdu.(*SuccessfulOutcome)
	if !ok || so.ProcedureCode != ProcMMEConfigurationUpdate {
		t.Fatalf("got %T procedureCode %d", pdu, pdu.procedureCode())
	}

	if _, err := ParseMMEConfigurationUpdateAcknowledge(so.Value); err != nil {
		t.Fatal(err)
	}
}

func TestMMEConfigurationUpdateFailureRoundTrips(t *testing.T) {
	ttw := TimeToWait(1)

	in := &MMEConfigurationUpdateFailure{
		Cause:      Ptr(Cause{Group: CauseGroupProtocol, Value: CauseProtocolSemanticError}),
		TimeToWait: &ttw,
	}

	b, err := in.Marshal()
	if err != nil {
		t.Fatal(err)
	}

	pdu, err := Unmarshal(b)
	if err != nil {
		t.Fatal(err)
	}

	uo, ok := pdu.(*UnsuccessfulOutcome)
	if !ok || uo.ProcedureCode != ProcMMEConfigurationUpdate {
		t.Fatalf("got %T procedureCode %d", pdu, pdu.procedureCode())
	}

	out, err := ParseMMEConfigurationUpdateFailure(uo.Value)
	if err != nil {
		t.Fatal(err)
	}

	if out.Cause == nil || out.Cause.Group != CauseGroupProtocol || out.Cause.Value != CauseProtocolSemanticError {
		t.Fatalf("cause mismatch: %+v", out.Cause)
	}

	if out.TimeToWait == nil || *out.TimeToWait != ttw {
		t.Fatalf("TimeToWait mismatch: %+v", out.TimeToWait)
	}
}
