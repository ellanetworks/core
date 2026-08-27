// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package amf_test

import (
	"testing"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/nas/fgs"
)

// TS 24.501 §5.5.1.2.5, §5.5.1.3.5
func TestBuildRegistrationRejectNeverCarriesT3346(t *testing.T) {
	for _, cause := range []fgs.GMMCause{fgs.GMMCauseIllegalUE, fgs.GMMCauseUEIdentityCannotBeDerived} {
		wire, err := amf.BuildRegistrationReject(720, cause)
		if err != nil {
			t.Fatalf("BuildRegistrationReject: %v", err)
		}

		got, err := fgs.ParseRegistrationReject(wire)
		if err != nil {
			t.Fatalf("ParseRegistrationReject: %v", err)
		}

		if got.Cause != cause {
			t.Errorf("cause = %v, want %v", got.Cause, cause)
		}

		if got.T3346 != nil {
			t.Errorf("T3346 = %v, want nil; a back-off timer belongs to congestion control, which the AMF does not run", got.T3346)
		}

		if got.T3502 == nil {
			t.Error("T3502 not encoded")
		}
	}
}
