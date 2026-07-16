// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"encoding/hex"
	"slices"
	"testing"

	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/s1ap"
)

// goldenS1SetupRequest is a real S1 SETUP REQUEST (eNB "JLT-621", PLMN 001/01,
// TAC 0x3039), used to exercise the MME's request->response handling without a
// live SCTP association.
const goldenS1SetupRequest = "0011002d000004003b00090000f1104054f64010003c400903004a4c542d36323100400007000c0e4000f1100089400100"

func goldenS1SetupValue(t *testing.T) []byte {
	t.Helper()

	raw, err := hex.DecodeString(goldenS1SetupRequest)
	if err != nil {
		t.Fatal(err)
	}

	pdu, err := s1ap.Unmarshal(raw)
	if err != nil {
		t.Fatalf("unmarshal request: %v", err)
	}

	im, ok := pdu.(*s1ap.InitiatingMessage)
	if !ok || im.ProcedureCode != s1ap.ProcS1Setup {
		t.Fatalf("got %T", pdu)
	}

	return im.Value
}

// TestS1SetupOutcomeAccepts checks that an eNB broadcasting a PLMN this MME
// serves (001/01) is answered with an S1 Setup Response carrying our identity.
func TestS1SetupOutcomeAccepts(t *testing.T) {
	req, respBytes, accepted, _, err := s1SetupOutcomeFor(goldenS1SetupValue(t), models.PlmnID{Mcc: "001", Mnc: "01"}, []uint16{0x3039}, 1, 1, "ella", 0xff)
	if err != nil {
		t.Fatalf("handle: %v", err)
	}

	if !accepted {
		t.Fatal("S1 Setup with a served TAI was rejected")
	}

	if req.ENBName != "JLT-621" {
		t.Fatalf("eNB name = %q, want JLT-621", req.ENBName)
	}

	respPDU, err := s1ap.Unmarshal(respBytes)
	if err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	so, ok := respPDU.(*s1ap.SuccessfulOutcome)
	if !ok || so.ProcedureCode != s1ap.ProcS1Setup {
		t.Fatalf("response is %T", respPDU)
	}

	resp, err := s1ap.ParseS1SetupResponse(so.Value)
	if err != nil {
		t.Fatalf("parse response: %v", err)
	}

	if resp.MMEName != "ella" ||
		len(resp.ServedGUMMEIs) != 1 ||
		resp.ServedGUMMEIs[0].ServedPLMNs[0] != (s1ap.PLMNIdentity{0x00, 0xf1, 0x10}) {
		t.Fatalf("response identity mismatch: %+v", resp)
	}
}

// TestS1SetupOutcomeRejectsUnknownPLMN checks that an eNB broadcasting only PLMNs
// this MME does not serve is rejected with an S1 Setup Failure carrying cause
// Misc "unknown-PLMN" (TS 36.413 §8.7.3.4). The golden eNB broadcasts 001/01; the
// MME here serves 999/99.
func TestS1SetupOutcomeRejectsUnknownPLMN(t *testing.T) {
	_, outBytes, accepted, _, err := s1SetupOutcomeFor(goldenS1SetupValue(t), models.PlmnID{Mcc: "999", Mnc: "99"}, []uint16{0x3039}, 1, 1, "ella", 0xff)
	if err != nil {
		t.Fatalf("handle: %v", err)
	}

	if accepted {
		t.Fatal("S1 Setup from an eNB with no served PLMN was accepted")
	}

	pdu, err := s1ap.Unmarshal(outBytes)
	if err != nil {
		t.Fatalf("unmarshal failure: %v", err)
	}

	uo, ok := pdu.(*s1ap.UnsuccessfulOutcome)
	if !ok || uo.ProcedureCode != s1ap.ProcS1Setup {
		t.Fatalf("outcome is %T, want S1 Setup UnsuccessfulOutcome", pdu)
	}

	fail, err := s1ap.ParseS1SetupFailure(uo.Value)
	if err != nil {
		t.Fatalf("parse failure: %v", err)
	}

	if fail.Cause != causeUnknownPLMN {
		t.Fatalf("cause = %+v, want %+v (Misc/unknown-PLMN)", fail.Cause, causeUnknownPLMN)
	}
}

// TestS1SetupOutcomeRejectsUnknownTAC checks that an eNB broadcasting the served
// PLMN but no served TAC is rejected with an S1 Setup Failure carrying cause Misc
// "unspecified", matching the AMF's NG Setup handling. The golden eNB broadcasts
// 001/01 with TAC 0x3039; the MME here serves 001/01 but only TAC 0x0007.
func TestS1SetupOutcomeRejectsUnknownTAC(t *testing.T) {
	_, outBytes, accepted, reason, err := s1SetupOutcomeFor(goldenS1SetupValue(t), models.PlmnID{Mcc: "001", Mnc: "01"}, []uint16{0x0007}, 1, 1, "ella", 0xff)
	if err != nil {
		t.Fatalf("handle: %v", err)
	}

	if accepted {
		t.Fatal("S1 Setup from an eNB with no served TAC was accepted")
	}

	if reason == "" {
		t.Fatal("rejection reason is empty")
	}

	pdu, err := s1ap.Unmarshal(outBytes)
	if err != nil {
		t.Fatalf("unmarshal failure: %v", err)
	}

	uo, ok := pdu.(*s1ap.UnsuccessfulOutcome)
	if !ok || uo.ProcedureCode != s1ap.ProcS1Setup {
		t.Fatalf("outcome is %T, want S1 Setup UnsuccessfulOutcome", pdu)
	}

	fail, err := s1ap.ParseS1SetupFailure(uo.Value)
	if err != nil {
		t.Fatalf("parse failure: %v", err)
	}

	if fail.Cause != causeNoServedTAC {
		t.Fatalf("cause = %+v, want %+v (Misc/unspecified)", fail.Cause, causeNoServedTAC)
	}
}

// TestS1SetupFailureNamesMissingIEs checks the S1 Setup Failure sent for a request
// omitting mandatory reject-criticality IEs carries cause "abstract-syntax-error-
// reject" and names each missing IE in Criticality Diagnostics (TS 36.413 §10.3.5),
// mirroring the AMF's NG Setup Failure.
func TestS1SetupFailureNamesMissingIEs(t *testing.T) {
	const (
		ieGlobalENBID  s1ap.ProtocolIEID = 59
		ieSupportedTAs s1ap.ProtocolIEID = 64
	)

	out, err := buildS1SetupFailureMissingIEs([]s1ap.ProtocolIEID{ieGlobalENBID, ieSupportedTAs})
	if err != nil {
		t.Fatalf("build: %v", err)
	}

	pdu, err := s1ap.Unmarshal(out)
	if err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	uo, ok := pdu.(*s1ap.UnsuccessfulOutcome)
	if !ok || uo.ProcedureCode != s1ap.ProcS1Setup {
		t.Fatalf("outcome is %T, want S1 Setup UnsuccessfulOutcome", pdu)
	}

	fail, err := s1ap.ParseS1SetupFailure(uo.Value)
	if err != nil {
		t.Fatalf("parse failure: %v", err)
	}

	wantCause := s1ap.Cause{Group: s1ap.CauseGroupProtocol, Value: s1ap.CauseProtocolAbstractSyntaxErrorReject}
	if fail.Cause != wantCause {
		t.Fatalf("cause = %+v, want abstract-syntax-error-reject", fail.Cause)
	}

	cd := fail.CriticalityDiagnostics
	if cd == nil {
		t.Fatal("failure carries no Criticality Diagnostics")
	}

	if cd.ProcedureCode == nil || *cd.ProcedureCode != s1ap.ProcS1Setup ||
		cd.TriggeringMessage == nil || *cd.TriggeringMessage != s1ap.TriggeringInitiatingMessage ||
		cd.ProcedureCriticality == nil || *cd.ProcedureCriticality != s1ap.CriticalityReject {
		t.Fatalf("Criticality Diagnostics header mismatch: %+v", cd)
	}

	want := []s1ap.CriticalityDiagnosticsIEItem{
		{IECriticality: s1ap.CriticalityReject, IEID: ieGlobalENBID, TypeOfError: s1ap.TypeOfErrorMissing},
		{IECriticality: s1ap.CriticalityReject, IEID: ieSupportedTAs, TypeOfError: s1ap.TypeOfErrorMissing},
	}
	if !slices.Equal(cd.IEsCriticalityDiagnostics, want) {
		t.Fatalf("reported IEs = %+v, want %+v", cd.IEsCriticalityDiagnostics, want)
	}
}
