// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"testing"

	"github.com/ellanetworks/core/nas/fgs"
)

func TestMovingFromEPC(t *testing.T) {
	for _, tc := range []struct {
		name string
		req  *fgs.RegistrationRequest
		want bool
	}{
		{"no request", nil, false},
		{"no UE status", &fgs.RegistrationRequest{}, false},
		{"EMM-REGISTERED", &fgs.RegistrationRequest{UEStatus: &fgs.UEStatus{S1ModeReg: true}}, true},
		// A UE registered only in 5GMM is not moving from EPC; synchronising its
		// PDU session status is the normal behaviour and must not be skipped.
		{"5GMM-REGISTERED only", &fgs.RegistrationRequest{UEStatus: &fgs.UEStatus{N1ModeReg: true}}, false},
		{"both", &fgs.RegistrationRequest{UEStatus: &fgs.UEStatus{S1ModeReg: true, N1ModeReg: true}}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := movingFromEPC(tc.req); got != tc.want {
				t.Errorf("movingFromEPC = %v, want %v", got, tc.want)
			}
		})
	}
}
