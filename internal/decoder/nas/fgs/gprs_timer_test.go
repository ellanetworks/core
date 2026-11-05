// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package fgs

import (
	"testing"

	naslib "github.com/ellanetworks/core/nas"
)

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
