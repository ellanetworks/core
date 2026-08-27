// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package amf_test

import (
	"testing"
	"time"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/nas/fgs"
)

// TS 24.501 §5.5.1.3.5
func TestBuildRegistrationRejectCarriesT3346(t *testing.T) {
	wire, err := amf.BuildRegistrationReject(720, fgs.GMMCauseCongestion, 10*time.Second)
	if err != nil {
		t.Fatalf("BuildRegistrationReject: %v", err)
	}

	got, err := fgs.ParseRegistrationReject(wire)
	if err != nil {
		t.Fatalf("ParseRegistrationReject: %v", err)
	}

	if got.Cause != fgs.GMMCauseCongestion {
		t.Errorf("cause = %v, want %v", got.Cause, fgs.GMMCauseCongestion)
	}

	if got.T3346 == nil {
		t.Fatal("T3346 not encoded")
	}

	if got.T3346.Unit != nas.GPRSTimer2Unit2Seconds || got.T3346.Value != 5 {
		t.Errorf("T3346 = %v/%v, want %v/5", got.T3346.Unit, got.T3346.Value, nas.GPRSTimer2Unit2Seconds)
	}

	if got.T3502 == nil {
		t.Error("T3502 not encoded")
	}
}

func TestBuildRegistrationRejectOmitsT3346WhenZero(t *testing.T) {
	wire, err := amf.BuildRegistrationReject(720, fgs.GMMCauseIllegalUE, 0)
	if err != nil {
		t.Fatalf("BuildRegistrationReject: %v", err)
	}

	got, err := fgs.ParseRegistrationReject(wire)
	if err != nil {
		t.Fatalf("ParseRegistrationReject: %v", err)
	}

	if got.T3346 != nil {
		t.Errorf("T3346 = %v, want nil", got.T3346)
	}
}
