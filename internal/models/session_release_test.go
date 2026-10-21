// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package models_test

import (
	"bytes"
	"testing"

	"github.com/ellanetworks/core/internal/models"
)

func TestReleaseReasonsMapToAChargingTrigger(t *testing.T) {
	want := map[models.SessionReleaseReason]models.ChargingTerminationTrigger{
		models.ReleaseUERequested:            models.ChargingTriggerFinal,
		models.ReleaseDeregistration:         models.ChargingTriggerFinal,
		models.ReleaseImplicitDeregistration: models.ChargingTriggerFinal,
		models.ReleaseStatusMismatch:         models.ChargingTriggerFinal,
		models.ReleaseDuplicateSessionID:     models.ChargingTriggerFinal,
		models.ReleaseSubscriptionWithdrawn:  models.ChargingTriggerManagementIntervention,
		models.ReleaseSubscriptionChanged:    models.ChargingTriggerManagementIntervention,
		models.ReleasePolicyChanged:          models.ChargingTriggerManagementIntervention,
		models.ReleaseRegistrationFailure:    models.ChargingTriggerAbnormalRelease,
		models.ReleaseHandoverFailure:        models.ChargingTriggerAbnormalRelease,
		models.ReleaseNotTransferred:         models.ChargingTriggerAbnormalRelease,
		models.ReleaseUENotResponding:        models.ChargingTriggerAbnormalRelease,
		models.ReleaseProtocolError:          models.ChargingTriggerAbnormalRelease,
		models.ReleaseEstablishmentFailure:   models.ChargingTriggerAbnormalRelease,
		models.ReleaseAccessReleased:         models.ChargingTriggerAbnormalRelease,
		models.ReleaseNetworkFailure:         models.ChargingTriggerAbnormalRelease,
	}

	if len(want) != len(models.SessionReleaseReasons) {
		t.Fatalf("%d reasons listed, %d classified", len(models.SessionReleaseReasons), len(want))
	}

	for _, reason := range models.SessionReleaseReasons {
		trigger, ok := want[reason]
		if !ok {
			t.Fatalf("reason %s is not classified", reason)
		}

		if got := reason.ChargingTrigger(); got != trigger {
			t.Errorf("%s terminates charging with %s, want %s", reason, got, trigger)
		}
	}
}

func TestChargingTriggersCloseTheRecordWithTheirCause(t *testing.T) {
	want := map[models.ChargingTerminationTrigger]models.RecordClosingCause{
		models.ChargingTriggerFinal:                  0,
		models.ChargingTriggerAbnormalRelease:        4,
		models.ChargingTriggerManagementIntervention: 20,
	}

	for trigger, closing := range want {
		if got := trigger.RecordClosing(); got != closing {
			t.Errorf("%s closes the record with %d, want %d", trigger, got, closing)
		}
	}
}

func TestEPSRanNasCauseEncoding(t *testing.T) {
	cases := []struct {
		name  string
		cause models.EPSRanNasCause
		want  []byte
	}{
		{"s1ap radio network", models.EPSRanNasCause{Protocol: models.EPSCauseS1AP, CauseType: 0, Value: 21}, []byte{0x10, 21}},
		{"s1ap nas", models.EPSRanNasCause{Protocol: models.EPSCauseS1AP, CauseType: 2, Value: 2}, []byte{0x12, 2}},
		{"esm", models.EPSRanNasCause{Protocol: models.EPSCauseESM, Value: 36}, []byte{0x30, 36}},
		{"diameter", models.EPSRanNasCause{Protocol: models.EPSCauseDiameter, Value: 5030}, []byte{0x40, 0x13, 0xa6}},
	}

	for _, tc := range cases {
		if got := tc.cause.Encode(); !bytes.Equal(got, tc.want) {
			t.Errorf("%s: encoded %x, want %x", tc.name, got, tc.want)
		}
	}
}

func TestReleaseRequestBuildersDoNotShareDiagnostics(t *testing.T) {
	base := models.ReleaseFor(models.ReleaseAccessReleased).WithRanNas(models.ReceivedNGAPCause(0, 21))

	a := base.WithRanNas(models.Received5GMMCause(9))
	b := base.WithRanNas(models.Received5GSMCause(36))

	if len(base.RanNas) != 1 || len(a.RanNas) != 2 || len(b.RanNas) != 2 {
		t.Fatalf("diagnostics lengths base=%d a=%d b=%d, want 1/2/2", len(base.RanNas), len(a.RanNas), len(b.RanNas))
	}

	if a.RanNas[1].FiveGMm == nil || b.RanNas[1].FiveGSm == nil {
		t.Fatal("a derived request overwrote another's diagnostics")
	}
}

func TestReleaseRequestIgnoresAnEmptyLocation(t *testing.T) {
	if got := models.ReleaseFor(models.ReleaseDeregistration).WithLocation(models.UserLocation{}); got.UeLocation != nil {
		t.Fatalf("location = %+v, want none", got.UeLocation)
	}
}
