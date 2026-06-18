// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"testing"

	"github.com/ellanetworks/core/s1ap"
)

func TestS1APMessageType(t *testing.T) {
	cases := []struct {
		name string
		pdu  any
		want S1APProcedure
	}{
		{
			name: "initiating known procedure",
			pdu:  &s1ap.InitiatingMessage{ProcedureCode: s1ap.ProcInitialUEMessage},
			want: S1APProcedureInitialUEMessage,
		},
		{
			name: "s1 setup request",
			pdu:  &s1ap.InitiatingMessage{ProcedureCode: s1ap.ProcS1Setup},
			want: S1APProcedureS1SetupRequest,
		},
		{
			name: "s1 setup response shares the procedure code but differs by category",
			pdu:  &s1ap.SuccessfulOutcome{ProcedureCode: s1ap.ProcS1Setup},
			want: S1APProcedureS1SetupResponse,
		},
		{
			name: "ue context release complete",
			pdu:  &s1ap.SuccessfulOutcome{ProcedureCode: s1ap.ProcUEContextRelease},
			want: S1APProcedureUEContextReleaseComplete,
		},
		{
			name: "initial context setup failure",
			pdu:  &s1ap.UnsuccessfulOutcome{ProcedureCode: s1ap.ProcInitialContextSetup},
			want: S1APProcedureInitialContextSetupFailure,
		},
		{
			name: "unhandled procedure",
			pdu:  &s1ap.InitiatingMessage{ProcedureCode: s1ap.ProcPaging},
			want: S1APProcedureUnknown,
		},
		{
			name: "unknown category",
			pdu:  struct{}{},
			want: S1APProcedureUnknown,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := s1apMessageType(tc.pdu); got != tc.want {
				t.Fatalf("s1apMessageType = %q, want %q", got, tc.want)
			}
		})
	}
}
