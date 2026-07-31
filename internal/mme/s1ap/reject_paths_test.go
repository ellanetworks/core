// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"context"
	"encoding/hex"
	"testing"

	"github.com/ellanetworks/core/internal/mme"
	"github.com/ellanetworks/core/s1ap"
)

// outcomeOf decodes the single PDU a handler sent, failing if it sent none.
func outcomeOf(t *testing.T, cc *captureConn) any {
	t.Helper()

	if len(cc.sent) != 1 {
		t.Fatalf("sent %d S1AP messages, want exactly 1", len(cc.sent))
	}

	pdu, err := s1ap.Unmarshal(cc.sent[0])
	if err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	return pdu
}

// TS 36.413 §10.3.5: a rejected PATH SWITCH REQUEST draws the procedure's own
// unsuccessful outcome, named with the source MME-UE-S1AP-ID it carries
// (§9.1.5.8).
func TestPathSwitchRequestRejectionAnswers(t *testing.T) {
	m := newTestMME(t)
	cc := &captureConn{}

	// Source MME-UE-S1AP-ID (88) and eNB-UE-S1AP-ID (8) only: the mandatory
	// reject-criticality E-RAB list is absent.
	body, err := hex.DecodeString("000002" + "00084002" + "0009" + "00584002" + "0007")
	if err != nil {
		t.Fatal(err)
	}

	handlePathSwitchRequest(m, context.Background(), mme.NewRadioForTest(cc), body)

	pdu := outcomeOf(t, cc)

	out, ok := pdu.(*s1ap.UnsuccessfulOutcome)
	if !ok {
		t.Fatalf("sent %T, want *s1ap.UnsuccessfulOutcome", pdu)
	}

	fail, err := s1ap.ParsePathSwitchRequestFailure(out.Value)
	if err != nil {
		t.Fatalf("parse failure: %v", err)
	}

	if fail.MMEUES1APID == nil || *fail.MMEUES1APID != 7 {
		t.Errorf("MME-UE-S1AP-ID = %v, want 7", fail.MMEUES1APID)
	}

	if fail.ENBUES1APID == nil || *fail.ENBUES1APID != 9 {
		t.Errorf("eNB-UE-S1AP-ID = %v, want 9", fail.ENBUES1APID)
	}

	if fail.CriticalityDiagnostics == nil {
		t.Error("failure carries no Criticality Diagnostics")
	}
}

// TS 36.413 §10.3.5: where the received information cannot build the
// unsuccessful outcome, the receiver reports by Error Indication instead of
// staying silent.
func TestRejectionFallsBackToErrorIndication(t *testing.T) {
	m := newTestMME(t)
	cc := &captureConn{}

	// Undecodable octets: nothing is recovered, so no failure message can name
	// the association.
	handlePathSwitchRequest(m, context.Background(), mme.NewRadioForTest(cc), []byte{0x00, 0xff})

	pdu := outcomeOf(t, cc)

	im, ok := pdu.(*s1ap.InitiatingMessage)
	if !ok {
		t.Fatalf("sent %T, want *s1ap.InitiatingMessage", pdu)
	}

	if im.ProcedureCode != s1ap.ProcErrorIndication {
		t.Fatalf("procedure = %s, want ErrorIndication", im.ProcedureCode)
	}
}

// A rejected ENB CONFIGURATION UPDATE draws the FAILURE its procedure defines.
func TestENBConfigurationUpdateRejectionAnswers(t *testing.T) {
	m := newTestMME(t)
	cc := &captureConn{}

	// Default Paging DRX carried twice is a falsely constructed message
	// (TS 36.413 §10.3.6).
	body, err := hex.DecodeString("000002" + "00894001" + "00" + "00894001" + "00")
	if err != nil {
		t.Fatal(err)
	}

	handleENBConfigurationUpdate(m, context.Background(), mme.NewRadioForTest(cc), body)

	pdu := outcomeOf(t, cc)

	out, ok := pdu.(*s1ap.UnsuccessfulOutcome)
	if !ok {
		t.Fatalf("sent %T, want *s1ap.UnsuccessfulOutcome", pdu)
	}

	fail, err := s1ap.ParseENBConfigurationUpdateFailure(out.Value)
	if err != nil {
		t.Fatalf("parse failure: %v", err)
	}

	want := s1ap.Cause{
		Group: s1ap.CauseGroupProtocol,
		Value: s1ap.CauseProtocolAbstractSyntaxErrorFalselyConstructedMessage,
	}
	if fail.Cause == nil || *fail.Cause != want {
		t.Errorf("cause = %v, want falsely-constructed-message", fail.Cause)
	}
}
