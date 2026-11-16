// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package amf_test

import (
	"testing"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/ngap"
	"github.com/ellanetworks/core/s1ap"
)

func TestS1APHandoverCause(t *testing.T) {
	for _, tc := range []struct {
		name string
		in   *ngap.Cause
		want s1ap.CauseRadioNetwork
	}{
		{
			name: "handover desirable for radio reasons",
			in:   new(ngap.CauseRadioNetworkHandoverDesirableForRadio.Cause()),
			want: s1ap.CauseRadioNetworkHandoverDesirableForRadio,
		},
		{
			name: "time critical handover",
			in:   new(ngap.CauseRadioNetworkTimeCriticalHandover.Cause()),
			want: s1ap.CauseRadioNetworkTimeCriticalHandover,
		},
		{
			name: "reduce load in serving cell",
			in:   new(ngap.CauseRadioNetworkReduceLoadInServingCell.Cause()),
			want: s1ap.CauseRadioNetworkReduceLoadInServingCell,
		},
		{
			name: "any other radio network value",
			in:   new(ngap.CauseRadioNetworkUnspecified.Cause()),
			want: s1ap.CauseRadioNetworkResourceOptimisationHandover,
		},
		{
			name: "any other group",
			in:   new(ngap.CauseMiscUnspecified.Cause()),
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

			if got.RadioNetwork == nil || *got.RadioNetwork != tc.want {
				t.Errorf("cause = %+v, want radio network %d", got, tc.want)
			}
		})
	}
}

func TestNGAPHandoverCause(t *testing.T) {
	for _, tc := range []struct {
		name string
		in   s1ap.Cause
		want ngap.Cause
	}{
		{
			name: "time critical",
			in:   s1ap.CauseRadioNetworkTimeCriticalHandover.Cause(),
			want: ngap.CauseRadioNetworkTimeCriticalHandover.Cause(),
		},
		{
			name: "resource optimisation",
			in:   s1ap.CauseRadioNetworkResourceOptimisationHandover.Cause(),
			want: ngap.CauseRadioNetworkResourceOptimisationHandover.Cause(),
		},
		{
			name: "reduce load",
			in:   s1ap.CauseRadioNetworkReduceLoadInServingCell.Cause(),
			want: ngap.CauseRadioNetworkReduceLoadInServingCell.Cause(),
		},
		{
			name: "another radio cause",
			in:   s1ap.CauseRadioNetworkUnspecified.Cause(),
			want: ngap.CauseRadioNetworkHandoverDesirableForRadio.Cause(),
		},
		{
			name: "another group",
			in:   s1ap.CauseMiscControlProcessingOverload.Cause(),
			want: ngap.CauseRadioNetworkHandoverDesirableForRadio.Cause(),
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := amf.NGAPHandoverCause(tc.in); !got.Equal(tc.want) {
				t.Errorf("cause = %+v, want %+v", got, tc.want)
			}
		})
	}
}

func TestS1APHandoverFailureCause(t *testing.T) {
	for _, tc := range []struct {
		name string
		in   ngap.Cause
		want s1ap.Cause
	}{
		{
			name: "no radio resources in the target cell",
			in:   ngap.CauseRadioNetworkNoRadioResourcesInTargetCell.Cause(),
			want: s1ap.CauseRadioNetworkNoRadioResourcesInTargetCell.Cause(),
		},
		{
			name: "algorithms not supported",
			in:   ngap.CauseRadioNetworkEncryptionAlgorithmsNotSupported.Cause(),
			want: s1ap.CauseRadioNetworkEncryptionAlgorithmsNotSupported.Cause(),
		},
		{
			name: "insufficient UE capabilities",
			in:   ngap.CauseRadioNetworkInsufficientUECapabilities.Cause(),
			want: s1ap.CauseRadioNetworkInsufficientUECapabilities.Cause(),
		},
		{
			name: "O&M intervention",
			in:   ngap.CauseMiscOMIntervention.Cause(),
			want: s1ap.CauseMiscOMIntervention.Cause(),
		},
		{
			name: "handover target not allowed",
			in:   ngap.CauseRadioNetworkHoTargetNotAllowed.Cause(),
			want: s1ap.CauseRadioNetworkHOTargetNotAllowed.Cause(),
		},
		{
			name: "another radio cause",
			in:   ngap.CauseRadioNetworkUnspecified.Cause(),
			want: s1ap.CauseRadioNetworkHOFailureInTarget.Cause(),
		},
		{
			name: "another misc cause",
			in:   ngap.CauseMiscUnspecified.Cause(),
			want: s1ap.CauseRadioNetworkHOFailureInTarget.Cause(),
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := amf.S1APHandoverFailureCause(tc.in); !got.Equal(tc.want) {
				t.Errorf("cause = %+v, want %+v", got, tc.want)
			}
		})
	}
}

func TestNGAPHandoverFailureCause(t *testing.T) {
	for _, tc := range []struct {
		name string
		in   s1ap.Cause
		want ngap.Cause
	}{
		{
			name: "no radio resources in the target cell",
			in:   s1ap.CauseRadioNetworkNoRadioResourcesInTargetCell.Cause(),
			want: ngap.CauseRadioNetworkNoRadioResourcesInTargetCell.Cause(),
		},
		{
			name: "algorithms not supported",
			in:   s1ap.CauseRadioNetworkEncryptionAlgorithmsNotSupported.Cause(),
			want: ngap.CauseRadioNetworkEncryptionAlgorithmsNotSupported.Cause(),
		},
		{
			name: "insufficient UE capabilities",
			in:   s1ap.CauseRadioNetworkInsufficientUECapabilities.Cause(),
			want: ngap.CauseRadioNetworkInsufficientUECapabilities.Cause(),
		},
		{
			name: "O&M intervention",
			in:   s1ap.CauseMiscOMIntervention.Cause(),
			want: ngap.CauseMiscOMIntervention.Cause(),
		},
		{
			name: "handover target not allowed",
			in:   s1ap.CauseRadioNetworkHOTargetNotAllowed.Cause(),
			want: ngap.CauseRadioNetworkHoTargetNotAllowed.Cause(),
		},
		{
			name: "another radio cause",
			in:   s1ap.CauseRadioNetworkUnspecified.Cause(),
			want: ngap.CauseRadioNetworkHOFailureInTarget.Cause(),
		},
		{
			name: "a transport cause",
			in:   s1ap.CauseTransportResourceUnavailable.Cause(),
			want: ngap.CauseRadioNetworkHOFailureInTarget.Cause(),
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := amf.NGAPHandoverFailureCause(tc.in); !got.Equal(tc.want) {
				t.Errorf("cause = %+v, want %+v", got, tc.want)
			}
		})
	}
}
