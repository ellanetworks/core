// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package fgs

import (
	"testing"

	naslib "github.com/ellanetworks/core/nas"
)

func TestGPRSTimer3Rendering(t *testing.T) {
	got := gprsTimer3(&naslib.GPRSTimer3{Unit: naslib.GPRSTimer3Unit10Minutes, Value: 6})

	if got.Unit.Label != "10 minutes" || got.Unit.Unknown {
		t.Errorf("unit = %+v, want a known \"10 minutes\"", got.Unit)
	}

	if got.Seconds == nil || *got.Seconds != 3600 {
		t.Errorf("seconds = %v, want 3600", got.Seconds)
	}

	if got.Deactivated {
		t.Error("timer reported as deactivated")
	}
}

func TestGPRSTimer3Deactivated(t *testing.T) {
	got := gprsTimer3(&naslib.GPRSTimer3{Unit: naslib.GPRSTimerUnitDeactivated})

	if !got.Deactivated {
		t.Error("deactivated timer not reported as such")
	}

	if got.Seconds != nil {
		t.Errorf("deactivated timer carries %d seconds", *got.Seconds)
	}
}

func TestGPRSTimer3AbsentStaysNil(t *testing.T) {
	if got := gprsTimer3(nil); got != nil {
		t.Fatalf("expected nil for an absent timer, got %+v", got)
	}
}

func TestGPRSTimer2Rendering(t *testing.T) {
	got := gprsTimer2(&naslib.GPRSTimer2{Unit: naslib.GPRSTimer2UnitDecihours, Value: 5})

	if got.Unit.Label != "decihours" || got.Unit.Unknown {
		t.Errorf("unit = %+v, want a known \"decihours\"", got.Unit)
	}

	if got.Seconds == nil || *got.Seconds != 1800 {
		t.Errorf("seconds = %v, want 1800", got.Seconds)
	}
}

func TestGPRSTimer2Deactivated(t *testing.T) {
	got := gprsTimer2(&naslib.GPRSTimer2{Unit: naslib.GPRSTimerUnitDeactivated})

	if !got.Deactivated || got.Seconds != nil {
		t.Errorf("deactivated timer rendered as %+v", got)
	}
}

func TestGPRSTimer2AbsentStaysNil(t *testing.T) {
	if got := gprsTimer2(nil); got != nil {
		t.Fatalf("expected nil for an absent timer, got %+v", got)
	}
}
