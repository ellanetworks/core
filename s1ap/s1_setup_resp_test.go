// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"testing"
)

func sampleResponse() *S1SetupResponse {
	return &S1SetupResponse{
		MMEName: "MME-1",
		ServedGUMMEIs: ServedGUMMEIs{{
			ServedPLMNs:    []PLMNIdentity{{0x00, 0xf1, 0x10}},
			ServedGroupIDs: []MMEGroupID{{0x80, 0x01}},
			ServedMMECs:    []MMECode{0x01},
		}},
		RelativeMMECapacity: 255,
		CriticalityDiagnostics: &CriticalityDiagnostics{
			ProcedureCode: func() *ProcedureCode { p := ProcS1Setup; return &p }(),
		},
	}
}

func TestS1SetupResponseRoundTrip(t *testing.T) {
	in := sampleResponse()

	b, err := in.Marshal()
	if err != nil {
		t.Fatal(err)
	}

	// A successfulOutcome PDU begins 20 11 (choice index 1, procedureCode 17).
	if b[0] != 0x20 || b[1] != 0x11 {
		t.Fatalf("envelope prefix = % x, want 20 11", b[:2])
	}

	pdu, err := Unmarshal(b)
	if err != nil {
		t.Fatal(err)
	}

	if _, ok := pdu.(*SuccessfulOutcome); !ok {
		t.Fatalf("got %T, want *SuccessfulOutcome", pdu)
	}

	out, err := ParseS1SetupResponse(pdu.value())
	if err != nil {
		t.Fatal(err)
	}

	if out.MMEName != in.MMEName || out.RelativeMMECapacity != in.RelativeMMECapacity {
		t.Fatalf("scalar mismatch: %+v", out)
	}

	if len(out.ServedGUMMEIs) != 1 ||
		len(out.ServedGUMMEIs[0].ServedPLMNs) != 1 ||
		out.ServedGUMMEIs[0].ServedPLMNs[0] != (PLMNIdentity{0x00, 0xf1, 0x10}) ||
		out.ServedGUMMEIs[0].ServedGroupIDs[0] != (MMEGroupID{0x80, 0x01}) ||
		out.ServedGUMMEIs[0].ServedMMECs[0] != MMECode(0x01) {
		t.Fatalf("ServedGUMMEIs mismatch: %+v", out.ServedGUMMEIs)
	}

	if out.CriticalityDiagnostics == nil || out.CriticalityDiagnostics.ProcedureCode == nil ||
		*out.CriticalityDiagnostics.ProcedureCode != ProcS1Setup {
		t.Fatalf("CriticalityDiagnostics mismatch: %+v", out.CriticalityDiagnostics)
	}
}

func TestS1SetupResponseReencodeStable(t *testing.T) {
	b1, err := sampleResponse().Marshal()
	if err != nil {
		t.Fatal(err)
	}

	pdu, err := Unmarshal(b1)
	if err != nil {
		t.Fatal(err)
	}

	out, err := ParseS1SetupResponse(pdu.value())
	if err != nil {
		t.Fatal(err)
	}

	b2, err := out.Marshal()
	if err != nil {
		t.Fatal(err)
	}

	if string(b1) != string(b2) {
		t.Fatalf("re-encode unstable:\n  b1 % x\n  b2 % x", b1, b2)
	}
}
