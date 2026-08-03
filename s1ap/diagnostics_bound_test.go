// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"errors"
	"testing"
)

// CriticalityDiagnostics-IE-List is SIZE(1..maxnoofErrors), so a report naming
// more than that cannot be encoded. A peer choosing how many unknown reject IEs
// to send therefore chooses whether the answer is sendable — the report is
// truncated instead, so the procedure it answers is always answered.
func TestNotUnderstoodReportStaysSendable(t *testing.T) {
	fields := make([]ieField, 0, maxnoofErrors+64)
	for i := range cap(fields) {
		fields = append(fields, ieField{
			id:   ProtocolIEID(40000 + i), // outside every id this version models
			crit: CriticalityReject,
			raw:  []byte{0x00},
		})
	}

	_, err := ParseS1SetupRequest(container(t, fields...))

	var ase *AbstractSyntaxError
	if !errors.As(err, &ase) {
		t.Fatalf("err = %v, want an AbstractSyntaxError", err)
	}

	if len(ase.IEs) != maxnoofErrors {
		t.Errorf("reported %d IEs, want the list truncated to maxnoofErrors (%d)", len(ase.IEs), maxnoofErrors)
	}

	diag := ase.OutcomeDiagnostics()
	failure := &S1SetupFailure{Cause: &ase.Cause, CriticalityDiagnostics: &diag}

	if _, err := failure.Marshal(); err != nil {
		t.Fatalf("the failure answering %d unknown reject IEs does not encode: %v", len(fields), err)
	}
}
