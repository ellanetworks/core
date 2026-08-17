// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package lmf

import (
	"context"
	"testing"
	"time"

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/db"
	coremodels "github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/sctp"
)

// countingNGAPSender stands in for a gNB association, counting PDUs written to it.
type countingNGAPSender struct {
	writes int
}

func (c *countingNGAPSender) WriteMsg(b []byte, _ *sctp.SndRcvInfo) (int, error) {
	c.writes++

	return len(b), nil
}

// provisionOperator sets the PLMN, AMF identity and served TAC the AMF needs to build a
// Paging message.
func provisionOperator(t *testing.T, database *db.Database) {
	t.Helper()

	ctx := context.Background()

	if err := database.UpdateOperatorID(ctx, "262", "01"); err != nil {
		t.Fatalf("set operator PLMN: %v", err)
	}

	if err := database.UpdateOperatorAMFIdentity(ctx, 1, 1); err != nil {
		t.Fatalf("set operator AMF identity: %v", err)
	}

	if err := database.UpdateOperatorTracking(ctx, []string{"00001a"}); err != nil {
		t.Fatalf("set operator served TACs: %v", err)
	}
}

// idleStaleUE registers a CM-IDLE UE whose NR location is ageSeconds old. The returned
// timestamp is what a last-known location answer must carry.
func idleStaleUE(t *testing.T, amfInstance *amf.AMF, imsi string, ageSeconds int32) (*amf.UeContext, etsi.SUPI, time.Time) {
	t.Helper()

	supi, err := etsi.NewSUPIFromIMSI(imsi)
	if err != nil {
		t.Fatalf("build SUPI: %v", err)
	}

	tmsi, err := etsi.NewTMSI(0x01020304)
	if err != nil {
		t.Fatalf("build TMSI: %v", err)
	}

	staleTime := time.Now().Add(-time.Duration(ageSeconds) * time.Second)

	ue := amf.NewUeContext()
	ue.SetSupiForTest(supi)
	ue.SetTmsiForTest(tmsi)
	ue.Location = coremodels.UserLocation{
		NrLocation: &coremodels.NrLocation{
			Tai: &coremodels.Tai{
				PlmnID: &coremodels.PlmnID{Mcc: "262", Mnc: "01"},
				Tac:    "0x00001a",
			},
			Ncgi: &coremodels.Ncgi{
				PlmnID:   &coremodels.PlmnID{Mcc: "262", Mnc: "01"},
				NrCellID: "0x00000001",
			},
			AgeOfLocationInformation: ageSeconds,
			UeLocationTimestamp:      &staleTime,
		},
	}
	ue.ForceStateForTest(amf.Registered)
	ue.RegistrationArea = []coremodels.Tai{{
		PlmnID: &coremodels.PlmnID{Mcc: "262", Mnc: "01"}, Tac: "00001a",
	}}

	if err := amfInstance.AddUeContextToPoolForTest(ue); err != nil {
		t.Fatalf("add UE to AMF: %v", err)
	}

	return ue, supi, staleTime
}

// When the UE never answers the page, the request is answered with its last known
// location and the age of that location (TS 23.273 §6.1.2 step 6) rather than failing.
func TestDetermineLocation_IdleUE_RefreshUnanswered_ReturnsLastKnownWithAge(t *testing.T) {
	const ageSeconds = 20

	database := testDBWithCell(t, db.RATNR, "262", "01", "0x00000001")
	provisionOperator(t, database)

	amfInstance := amf.New(database, nil, nil)

	lmfInstance := New(amfInstance, nil, database)
	lmfInstance.maxLocationAge = 10 // the 20s-old location below is stale
	lmfInstance.refreshTimeout = 100 * time.Millisecond

	ue, supi, staleTime := idleStaleUE(t, amfInstance, "123456789012345", ageSeconds)

	// A connected radio, so the page reaches a gNB; the UE never answers.
	sender := &countingNGAPSender{}
	radio := &amf.Radio{Conn: sender}
	radio.BindAMFForTest(amfInstance)
	amfInstance.UpdateRadioSupportedTAIs(radio, []amf.SupportedTAI{{
		Tai: coremodels.Tai{PlmnID: &coremodels.PlmnID{Mcc: "262", Mnc: "01"}, Tac: "00001a"},
	}})

	start := time.Now()

	result, _, err := lmfInstance.DetermineLocation(context.Background(), supi, MethodCellID)
	if err != nil {
		t.Fatalf("expected the last known location to be returned, got error: %v", err)
	}

	if result == nil {
		t.Fatal("expected a non-nil result")
	}

	// The page must have reached the RAN, otherwise the timeout path is not what is tested.
	if sender.writes == 0 {
		t.Error("expected the idle UE to be paged as part of the refresh")
	}

	ue.StopPaging()

	// The answer must be the stale estimate, carrying its age.
	if result.AgeOfLocationInfo != ageSeconds {
		t.Errorf("AgeOfLocationInfo = %d, want %d: the age must accompany a last known location",
			result.AgeOfLocationInfo, ageSeconds)
	}

	if result.UeLocationTimestamp == nil {
		t.Fatal("expected the last known location's timestamp to be reported")
	}

	if !result.UeLocationTimestamp.Equal(staleTime) {
		t.Errorf("UeLocationTimestamp = %v, want the stale timestamp %v",
			result.UeLocationTimestamp, staleTime)
	}

	// Still usable: the stale cell resolves to its provisioned coordinate.
	if result.Latitude == 0 || result.Longitude == 0 {
		t.Errorf("expected the stale cell's provisioned coordinate, got lat=%d lon=%d",
			result.Latitude, result.Longitude)
	}

	// The configured refresh timeout must be honoured, not the 5s default.
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Errorf("refresh waited %v; the configured refresh timeout was not honoured", elapsed)
	}
}
