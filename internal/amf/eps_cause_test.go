// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package amf_test

import (
	"testing"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/ngap"
	"github.com/ellanetworks/core/s1ap"
)

// TS 29.010 Table 7.2.3
func TestS1APHandoverCause(t *testing.T) {
	for _, tc := range []struct {
		name string
		in   *ngap.Cause
		want uint64
	}{
		{
			name: "handover desirable for radio reasons",
			in:   &ngap.Cause{Group: ngap.CauseGroupRadioNetwork, Value: ngap.CauseRadioNetworkHandoverDesirableForRadio},
			want: s1ap.CauseRadioNetworkHandoverDesirableForRadio,
		},
		{
			name: "time critical handover",
			in:   &ngap.Cause{Group: ngap.CauseGroupRadioNetwork, Value: ngap.CauseRadioNetworkTimeCriticalHandover},
			want: s1ap.CauseRadioNetworkTimeCriticalHandover,
		},
		{
			name: "reduce load in serving cell",
			in:   &ngap.Cause{Group: ngap.CauseGroupRadioNetwork, Value: ngap.CauseRadioNetworkReduceLoadInServingCell},
			want: s1ap.CauseRadioNetworkReduceLoadInServingCell,
		},
		{
			name: "any other radio network value",
			in:   &ngap.Cause{Group: ngap.CauseGroupRadioNetwork, Value: ngap.CauseRadioNetworkUnspecified},
			want: s1ap.CauseRadioNetworkResourceOptimisationHandover,
		},
		{
			name: "any other group",
			in:   &ngap.Cause{Group: ngap.CauseGroupMisc},
			want: s1ap.CauseRadioNetworkResourceOptimisationHandover,
		},
		{
			name: "absent",
			in:   nil,
			want: s1ap.CauseRadioNetworkResourceOptimisationHandover,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := amf.S1APHandoverCause(tc.in)

			if got.Group != s1ap.CauseGroupRadioNetwork || uint64(got.Value) != tc.want {
				t.Errorf("cause = %+v, want radio network %d", got, tc.want)
			}
		})
	}
}
