// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import "testing"

func TestAMFConfigurationUpdateRoundTrips(t *testing.T) {
	in := &AMFConfigurationUpdate{
		AMFName:             Ptr("ella"),
		RelativeAMFCapacity: Ptr(uint8(0)),
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
	if !ok || im.ProcedureCode != ProcAMFConfigurationUpdate {
		t.Fatalf("got %T procedureCode %d", pdu, pdu.procedureCode())
	}

	out, err := ParseAMFConfigurationUpdate(im.Value)
	if err != nil {
		t.Fatal(err)
	}

	if deref(out.AMFName) != "ella" {
		t.Fatalf("AMFName = %q", deref(out.AMFName))
	}

	if out.RelativeAMFCapacity == nil || *out.RelativeAMFCapacity != 0 {
		t.Fatalf("RelativeAMFCapacity = %v, want 0", out.RelativeAMFCapacity)
	}
}

func TestAMFConfigurationUpdateCapacityOnly(t *testing.T) {
	b, err := (&AMFConfigurationUpdate{RelativeAMFCapacity: Ptr(uint8(0))}).Marshal()
	if err != nil {
		t.Fatal(err)
	}

	pdu, err := Unmarshal(b)
	if err != nil {
		t.Fatal(err)
	}

	out, err := ParseAMFConfigurationUpdate(pdu.(*InitiatingMessage).Value)
	if err != nil {
		t.Fatal(err)
	}

	if out.AMFName != nil || len(out.ServedGUAMIList) != 0 {
		t.Fatalf("unexpected IEs: %+v", out)
	}

	if out.RelativeAMFCapacity == nil || *out.RelativeAMFCapacity != 0 {
		t.Fatalf("RelativeAMFCapacity = %v, want 0", out.RelativeAMFCapacity)
	}
}

func TestAMFConfigurationUpdateAcknowledgeRoundTrips(t *testing.T) {
	b, err := (&AMFConfigurationUpdateAcknowledge{}).Marshal()
	if err != nil {
		t.Fatal(err)
	}

	pdu, err := Unmarshal(b)
	if err != nil {
		t.Fatal(err)
	}

	so, ok := pdu.(*SuccessfulOutcome)
	if !ok || so.ProcedureCode != ProcAMFConfigurationUpdate {
		t.Fatalf("got %T procedureCode %d", pdu, pdu.procedureCode())
	}

	if _, err := ParseAMFConfigurationUpdateAcknowledge(so.Value); err != nil {
		t.Fatal(err)
	}
}

func TestAMFConfigurationUpdateFailureRoundTrips(t *testing.T) {
	ttw := TimeToWaitV2s

	in := &AMFConfigurationUpdateFailure{
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
	if !ok || uo.ProcedureCode != ProcAMFConfigurationUpdate {
		t.Fatalf("got %T procedureCode %d", pdu, pdu.procedureCode())
	}

	out, err := ParseAMFConfigurationUpdateFailure(uo.Value)
	if err != nil {
		t.Fatal(err)
	}

	if out.Cause == nil || out.Cause.Group != CauseGroupProtocol {
		t.Fatalf("cause mismatch: %+v", out.Cause)
	}

	if out.TimeToWait == nil || *out.TimeToWait != ttw {
		t.Fatalf("TimeToWait mismatch: %+v", out.TimeToWait)
	}
}
