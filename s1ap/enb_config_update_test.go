// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import "testing"

func TestENBConfigurationUpdateRoundTrips(t *testing.T) {
	drx := PagingDRX(2)

	in := &ENBConfigurationUpdate{
		ENBName: Ptr("enb-updated"),
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

	if deref(out.ENBName) != "enb-updated" || len(out.SupportedTAs) != 1 || out.SupportedTAs[0].TAC != 7 {
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
	in := &ENBConfigurationUpdateFailure{Cause: Ptr(Cause{Group: CauseGroupMisc, Value: 5})}

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

	if deref(out.Cause) != deref(in.Cause) {
		t.Fatalf("cause mismatch: %+v", out.Cause)
	}
}

// The Cause is mandatory but carries ignore criticality, which binds the two
// ends differently. TS 36.413 §10.3.3 binds the sender, so encoding an unset
// Cause fails. §10.3.5 keys the receiver's reaction to the IE's criticality
// rather than to its being mandatory, so an arriving message that omits it is
// still delivered, with the omission reported in Criticality Diagnostics — the
// reason Cause is nil-able here rather than a value.
func TestENBConfigurationUpdateFailureMissingCause(t *testing.T) {
	if _, err := (&ENBConfigurationUpdateFailure{TimeToWait: Ptr(TimeToWait(0))}).Marshal(); err == nil {
		t.Error("Marshal() = nil error, want a required-IE error for the absent Cause")
	}

	value := container(t, ieField{id: idTimeToWait, crit: CriticalityIgnore, val: TimeToWait(0)})

	msg, err := ParseENBConfigurationUpdateFailure(value)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	if msg.Cause != nil {
		t.Errorf("Cause = %v, want nil", msg.Cause)
	}

	diag := msg.Diagnostics()

	var reported bool

	for _, ie := range diag.IEs {
		if ie.ID == idCause && ie.TypeOfError == TypeOfErrorMissing {
			reported = true
		}
	}

	if !reported {
		t.Errorf("missing Cause not reported in diagnostics: %+v", diag.IEs)
	}
}
