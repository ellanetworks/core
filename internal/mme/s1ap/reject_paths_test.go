// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"context"
	"encoding/hex"
	"fmt"
	"testing"

	"github.com/ellanetworks/core/internal/mme"
	"github.com/ellanetworks/core/s1ap"
)

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

// TS 36.413 §10.3.5
func TestPathSwitchRequestRejectionAnswers(t *testing.T) {
	m := newTestMME(t)
	cc := &captureConn{}

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

// TS 36.413 §10.3.5
func TestRejectionFallsBackToErrorIndication(t *testing.T) {
	m := newTestMME(t)
	cc := &captureConn{}

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

func TestENBConfigurationUpdateRejectionAnswers(t *testing.T) {
	m := newTestMME(t)
	cc := &captureConn{}

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

// TS 36.413 §9.2.1.21
func TestReportOnResponseNamesTheOutcome(t *testing.T) {
	m := newTestMME(t)
	ue, cc := securedUE(t, m)

	mmeID := uint16(ue.Conn().MMEUES1APID)
	enbID := uint16(ue.Conn().ENBUES1APID)

	body, err := hex.DecodeString(fmt.Sprintf("000003"+"00004002%04x"+"00084002%04x"+"ea608001"+"00", mmeID, enbID))
	if err != nil {
		t.Fatal(err)
	}

	handleInitialContextSetupResponse(m, context.Background(), mme.NewRadioForTest(cc), body)

	var ind *s1ap.ErrorIndication

	for _, sent := range cc.sent {
		pdu, err := s1ap.Unmarshal(sent)
		if err != nil {
			t.Fatalf("unmarshal: %v", err)
		}

		im, ok := pdu.(*s1ap.InitiatingMessage)
		if !ok || im.ProcedureCode != s1ap.ProcErrorIndication {
			continue
		}

		if ind, err = s1ap.ParseErrorIndication(im.Value); err != nil {
			t.Fatalf("parse Error Indication: %v", err)
		}
	}

	if ind == nil {
		t.Fatal("no ERROR INDICATION sent for the notify IE")
	}

	cd := ind.CriticalityDiagnostics
	if cd == nil || cd.TriggeringMessage == nil {
		t.Fatalf("Criticality Diagnostics = %+v, want a Triggering Message", cd)
	}

	if *cd.TriggeringMessage != s1ap.TriggeringSuccessfulOutcome {
		t.Errorf("triggering message = %v, want successful-outcome", *cd.TriggeringMessage)
	}
}
