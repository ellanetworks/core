// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"bytes"
	"testing"
)

// Golden RAN CONFIGURATION UPDATE PDUs
const (
	goldenRANConfigUpdate         = "0023003e000004005240120780456c6c612d436f72652d5465737465720066001000000000010000f110000010081020300015400140001b40080000f11010000102"
	goldenRANConfigUpdateNameOnly = "0023000d000001005240060180676e6231"
	goldenRANConfigUpdateAck      = "20230003000000"
	goldenRANConfigUpdateFailure  = "4023000d000002000f400188006b400130"
)

func goldRANConfigUpdate() *RANConfigurationUpdate {
	return &RANConfigurationUpdate{
		RANNodeName: Ptr("Ella-Core-Tester"),
		SupportedTAList: SupportedTAList{{
			TAC: 1,
			BroadcastPLMNList: BroadcastPLMNList{{
				PLMNIdentity:        PLMNIdentity{0x00, 0xf1, 0x10},
				TAISliceSupportList: SliceSupportList{{SNSSAI: SNSSAI{SST: 1, SD: Ptr(SD{0x10, 0x20, 0x30})}}},
			}},
		}},
		DefaultPagingDRX: Ptr(PagingDRXv128),
		GlobalRANNodeID: Ptr(GlobalRANNodeID{
			Kind:         RANNodeIDGNB,
			PLMNIdentity: PLMNIdentity{0x00, 0xf1, 0x10},
			Value:        0x000102,
			Bits:         24,
		}),
	}
}

func TestRANConfigurationUpdateGoldenEncode(t *testing.T) {
	tests := []struct {
		name    string
		marshal func() ([]byte, error)
		want    string
	}{
		{"full", goldRANConfigUpdate().Marshal, goldenRANConfigUpdate},
		{"name only", (&RANConfigurationUpdate{RANNodeName: Ptr("gnb1")}).Marshal, goldenRANConfigUpdateNameOnly},
		{"acknowledge", (&RANConfigurationUpdateAcknowledge{}).Marshal, goldenRANConfigUpdateAck},
		{
			"failure",
			(&RANConfigurationUpdateFailure{
				Cause:      Ptr(Cause{Group: CauseGroupMisc, Value: CauseMiscUnknownPLMNOrSNPN}),
				TimeToWait: Ptr(TimeToWaitV10s),
			}).Marshal,
			goldenRANConfigUpdateFailure,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.marshal()
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}

			if want := mustHex(t, tt.want); !bytes.Equal(got, want) {
				t.Fatalf("encode mismatch:\n  got  %x\n  want %x", got, want)
			}
		})
	}
}

func TestRANConfigurationUpdateGoldenDecode(t *testing.T) {
	pdu, err := Unmarshal(mustHex(t, goldenRANConfigUpdate))
	if err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	im, ok := pdu.(*InitiatingMessage)
	if !ok || im.ProcedureCode != ProcRANConfigurationUpdate {
		t.Fatalf("got %T procedureCode %d", pdu, pdu.procedureCode())
	}

	msg, err := ParseRANConfigurationUpdate(im.Value)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	want := goldRANConfigUpdate()

	if deref(msg.RANNodeName) != deref(want.RANNodeName) {
		t.Errorf("RANNodeName = %q, want %q", deref(msg.RANNodeName), deref(want.RANNodeName))
	}

	if len(msg.SupportedTAList) != 1 || msg.SupportedTAList[0].TAC != 1 {
		t.Fatalf("SupportedTAList = %+v", msg.SupportedTAList)
	}

	if deref(msg.DefaultPagingDRX) != PagingDRXv128 {
		t.Errorf("DefaultPagingDRX = %d", deref(msg.DefaultPagingDRX))
	}

	if msg.GlobalRANNodeID == nil {
		t.Fatal("GlobalRANNodeID = nil")
	}

	if got := *msg.GlobalRANNodeID; got != deref(want.GlobalRANNodeID) {
		t.Errorf("GlobalRANNodeID = %+v, want %+v", got, deref(want.GlobalRANNodeID))
	}
}

// TS 38.413 §8.7.2.2: an absent IE leaves that configuration unchanged, so a
// message carrying only one IE is well formed and must not be read as a request
// to clear the others.
func TestRANConfigurationUpdateNameOnlyDecode(t *testing.T) {
	pdu, err := Unmarshal(mustHex(t, goldenRANConfigUpdateNameOnly))
	if err != nil {
		t.Fatal(err)
	}

	msg, err := ParseRANConfigurationUpdate(pdu.value())
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	if deref(msg.RANNodeName) != "gnb1" {
		t.Errorf("RANNodeName = %q", deref(msg.RANNodeName))
	}

	if msg.SupportedTAList != nil || msg.DefaultPagingDRX != nil ||
		msg.GlobalRANNodeID != nil || msg.NGRANTNLAssociationToRemoveList != nil {
		t.Errorf("absent IEs decoded to non-nil: %+v", msg)
	}
}

// The acknowledge has no mandatory IE, so an empty one is valid.
func TestRANConfigurationUpdateAcknowledgeEmpty(t *testing.T) {
	pdu, err := Unmarshal(mustHex(t, goldenRANConfigUpdateAck))
	if err != nil {
		t.Fatal(err)
	}

	so, ok := pdu.(*SuccessfulOutcome)
	if !ok || so.ProcedureCode != ProcRANConfigurationUpdate {
		t.Fatalf("got %T procedureCode %d", pdu, pdu.procedureCode())
	}

	msg, err := ParseRANConfigurationUpdateAcknowledge(so.Value)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	if msg.CriticalityDiagnostics != nil {
		t.Errorf("CriticalityDiagnostics = %+v, want nil", msg.CriticalityDiagnostics)
	}
}

func TestRANConfigurationUpdateFailureGoldenDecode(t *testing.T) {
	pdu, err := Unmarshal(mustHex(t, goldenRANConfigUpdateFailure))
	if err != nil {
		t.Fatal(err)
	}

	uo, ok := pdu.(*UnsuccessfulOutcome)
	if !ok || uo.ProcedureCode != ProcRANConfigurationUpdate {
		t.Fatalf("got %T procedureCode %d", pdu, pdu.procedureCode())
	}

	msg, err := ParseRANConfigurationUpdateFailure(uo.Value)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	want := Cause{Group: CauseGroupMisc, Value: CauseMiscUnknownPLMNOrSNPN}
	if deref(msg.Cause) != want {
		t.Errorf("cause = %v, want %v", deref(msg.Cause), want)
	}

	if deref(msg.TimeToWait) != TimeToWaitV10s {
		t.Errorf("timeToWait = %d", deref(msg.TimeToWait))
	}
}

// The Cause is mandatory but carries ignore criticality, which binds the two
// ends differently. TS 38.413 §10.3.3 binds the sender, so encoding an unset
// Cause fails. §10.3.5 keys the receiver's reaction to the IE's criticality
// rather than to its being mandatory, so an arriving message that omits it is
// still delivered, with the omission reported in Criticality Diagnostics — the
// reason Cause is nil-able here rather than a value.
func TestRANConfigurationUpdateFailureMissingCause(t *testing.T) {
	if _, err := (&RANConfigurationUpdateFailure{TimeToWait: Ptr(TimeToWaitV10s)}).Marshal(); err == nil {
		t.Error("Marshal() = nil error, want a required-IE error for the absent Cause")
	}

	value := container(t, ieField{id: idTimeToWait, crit: CriticalityIgnore, val: TimeToWaitV10s})

	msg, err := ParseRANConfigurationUpdateFailure(value)
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
