// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/internal/mme"
	"github.com/ellanetworks/core/s1ap"
)

func TestMMEConfigurationUpdateOutcomesAreRouted(t *testing.T) {
	cases := []struct {
		name string
		pdu  func(t *testing.T) any
	}{
		{"acknowledge", func(t *testing.T) any {
			t.Helper()

			return &s1ap.SuccessfulOutcome{
				ProcedureCode: s1ap.ProcMMEConfigurationUpdate,
				Value:         successfulValue(t, mustMarshal(t, (&s1ap.MMEConfigurationUpdateAcknowledge{}).Marshal)),
			}
		}},
		{"failure", func(t *testing.T) any {
			t.Helper()

			b := mustMarshal(t, (&s1ap.MMEConfigurationUpdateFailure{
				Cause:      s1ap.Ptr(s1ap.Cause{Group: s1ap.CauseGroupProtocol, Value: s1ap.CauseProtocolSemanticError}),
				TimeToWait: s1ap.Ptr(s1ap.TimeToWait(1)),
			}).Marshal)

			return &s1ap.UnsuccessfulOutcome{
				ProcedureCode: s1ap.ProcMMEConfigurationUpdate,
				Value:         unsuccessfulValue(t, b),
			}
		}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := newTestMME(t)

			conn := &captureConn{}
			radio := mme.NewRadioForTest(conn)

			Route(m, context.Background(), radio, tc.pdu(t))

			if got := conn.count(); got != 0 {
				t.Fatalf("MME answered a configuration-update outcome with %d messages, want 0", got)
			}
		})
	}
}
