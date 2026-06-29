// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"testing"

	"github.com/ellanetworks/core/s1ap"
)

// resetValue marshals a Reset and returns the initiatingMessage open-type
// payload handleReset consumes.
func resetValue(t *testing.T, r *s1ap.Reset) []byte {
	t.Helper()

	b, err := r.Marshal()
	if err != nil {
		t.Fatalf("marshal Reset: %v", err)
	}

	pdu, err := s1ap.Unmarshal(b)
	if err != nil {
		t.Fatalf("unmarshal Reset: %v", err)
	}

	return pdu.(*s1ap.InitiatingMessage).Value
}

// parseResetAcknowledge decodes the single Reset Acknowledge the capture holds.
func parseResetAcknowledge(t *testing.T, pdu []byte) *s1ap.ResetAcknowledge {
	t.Helper()

	msg, err := s1ap.Unmarshal(pdu)
	if err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	so, ok := msg.(*s1ap.SuccessfulOutcome)
	if !ok || so.ProcedureCode != s1ap.ProcReset {
		t.Fatalf("expected Reset Acknowledge, got %T", msg)
	}

	ack, err := s1ap.ParseResetAcknowledge(so.Value)
	if err != nil {
		t.Fatalf("parse Reset Acknowledge: %v", err)
	}

	return ack
}

// TestS1ResetWholeInterface confirms a whole-interface reset moves every
// registered UE on the association to ECM-IDLE, acknowledges with no connection
// list, and leaves a UE on another association untouched.
func TestS1ResetWholeInterface(t *testing.T) {
	m := newTestMME(t)

	ue1, cc := securedUE(t, m)
	registerTestUE(m, ue1, "001010000000010")
	testPDN(ue1).Apn = "internet"

	ue2 := m.NewUe(cc, 8)
	ue2.emmState.store(EMMRegistered)
	registerTestUE(m, ue2, "001010000000011")
	testPDN(ue2).Apn = "internet"

	other, _ := securedUE(t, m)
	registerTestUE(m, other, "001010000000012")
	testPDN(other).Apn = "internet"

	cause := s1ap.Cause{Group: s1ap.CauseGroupMisc, Value: 0}
	m.handleReset(cc, resetValue(t, &s1ap.Reset{Cause: cause, ResetType: s1ap.ResetType{All: true}}))

	for _, ue := range []*UeContext{ue1, ue2} {
		got, ok := m.LookupUeByIMSI(ue.imsi)
		if !ok || got != ue {
			t.Fatalf("registered UE %q deleted by S1 reset; expected ECM-IDLE retention", ue.imsi)
		}

		if got.Connected() {
			t.Fatalf("UE %q not in ECM-IDLE after S1 reset", ue.imsi)
		}

		m.RemoveUe(ue) // stop the default-duration timer
	}

	if got, ok := m.LookupUeByIMSI(other.imsi); !ok || !got.Connected() {
		t.Fatal("UE on another association disturbed by S1 reset")
	}

	if cc.count() != 1 {
		t.Fatalf("sent %d messages, want 1 Reset Acknowledge", cc.count())
	}

	if ack := parseResetAcknowledge(t, cc.sent[0]); len(ack.ConnectionList) != 0 {
		t.Fatalf("whole-interface acknowledge carried a connection list: %+v", ack.ConnectionList)
	}
}

// TestS1ResetPartOfInterface confirms a part-of-interface reset releases only
// the listed UE and echoes the connection list in the acknowledge.
func TestS1ResetPartOfInterface(t *testing.T) {
	m := newTestMME(t)

	ue1, cc := securedUE(t, m)
	registerTestUE(m, ue1, "001010000000010")
	testPDN(ue1).Apn = "internet"

	ue2 := m.NewUe(cc, 8)
	ue2.emmState.store(EMMRegistered)
	registerTestUE(m, ue2, "001010000000011")
	testPDN(ue2).Apn = "internet"

	mmeID := ue1.S1.MMEUES1APID
	enbID := ue1.S1.ENBUES1APID
	cause := s1ap.Cause{Group: s1ap.CauseGroupRadioNetwork, Value: 0}

	m.handleReset(cc, resetValue(t, &s1ap.Reset{
		Cause: cause,
		ResetType: s1ap.ResetType{Part: []s1ap.UEAssociatedLogicalS1ConnectionItem{
			{MMEUES1APID: &mmeID, ENBUES1APID: &enbID},
		}},
	}))

	got1, ok := m.LookupUeByIMSI(ue1.imsi)
	if !ok || got1 != ue1 || got1.Connected() {
		t.Fatalf("listed UE not released to ECM-IDLE: ok=%v connected=%v", ok, got1.Connected())
	}

	m.RemoveUe(ue1)

	if got2, ok := m.LookupUeByIMSI(ue2.imsi); !ok || !got2.Connected() {
		t.Fatal("unlisted UE disturbed by part-of-interface reset")
	}

	ack := parseResetAcknowledge(t, cc.sent[0])
	if len(ack.ConnectionList) != 1 {
		t.Fatalf("acknowledge connection list length = %d, want 1", len(ack.ConnectionList))
	}

	it := ack.ConnectionList[0]
	if it.MMEUES1APID == nil || *it.MMEUES1APID != mmeID {
		t.Fatalf("acknowledge did not echo MME-UE-S1AP-ID %d: %+v", mmeID, it)
	}
}

// TestS1ResetDropsMidAttachUE confirms a reset of a UE that never completed
// registration drops the context and releases its session.
func TestS1ResetDropsMidAttachUE(t *testing.T) {
	m := newTestMME(t)

	ue, cc := securedUE(t, m)
	ue.emmState.store(EMMDeregistered) // attach not yet completed
	testPDN(ue).Apn = "internet"

	cause := s1ap.Cause{Group: s1ap.CauseGroupMisc, Value: 0}
	m.handleReset(cc, resetValue(t, &s1ap.Reset{Cause: cause, ResetType: s1ap.ResetType{All: true}}))

	if _, ok := m.LookupUeByIMSI(ue.imsi); ok {
		t.Fatal("incomplete-registration UE retained after S1 reset; expected drop")
	}

	if !m.Session.(*fakeSessionManager).released {
		t.Fatal("EPS session not released when dropping an incomplete UE on S1 reset")
	}
}
