// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package lmf

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/lmf/models"
	"github.com/ellanetworks/core/internal/roaming"
)

var errHomeRefuses = errors.New("home refuses")

func TestDetermineLocationRefusesARoamerTheHomeDoesNotAllow(t *testing.T) {
	l, supi := registeredLMF(t)

	var asked atomic.Int32

	l.privacyOverride = func(_ context.Context, s etsi.SUPI) error {
		asked.Add(1)

		if s != supi {
			t.Errorf("privacy asked about %s, want %s", s, supi)
		}

		return errHomeRefuses
	}

	_, err := l.DetermineLocation(context.Background(), supi, LocationRequest{Method: RequestedCellID})
	if !errors.Is(err, ErrPrivacyRefused) || !errors.Is(err, errHomeRefuses) || asked.Load() != 1 {
		t.Fatalf("err = %v after %d checks, want ErrPrivacyRefused", err, asked.Load())
	}

	if _, err := l.DetermineLocation(context.Background(), supi, LocationRequest{Method: "nope"}); !errors.Is(err, ErrUnsupportedMethod) {
		t.Fatalf("an invalid request must fail validation first: %v", err)
	}
}

func TestDetermineLocationOfAHomeSubscriberIsNotRefused(t *testing.T) {
	l, supi := registeredLMF(t)

	if err := l.checkPrivacy(context.Background(), supi); err != nil {
		t.Fatalf("home subscriber: %v", err)
	}

	if _, err := l.DetermineLocation(context.Background(), supi, LocationRequest{Method: RequestedCellID}); errors.Is(err, ErrPrivacyRefused) {
		t.Fatalf("home subscriber refused: %v", err)
	}
}

func TestDetermineLocationRefusesARoamerWithoutAReachableHome(t *testing.T) {
	l, _ := registeredLMF(t)

	roamer, err := etsi.NewSUPIFromIMSI("001020000000001")
	if err != nil {
		t.Fatal(err)
	}

	l.amf.HoldVisitorForTest(roamer, roaming.Visitor5G{PartnerID: "p", Registered: true})

	if _, err := l.DetermineLocation(context.Background(), roamer, LocationRequest{Method: RequestedCellID}); !errors.Is(err, ErrPrivacyRefused) || !errors.Is(err, roaming.ErrLocationPrivacy) {
		t.Fatalf("err = %v, want ErrPrivacyRefused", err)
	}
}

func TestCreateDeferredRequestRefusesARoamerTheHomeDoesNotAllow(t *testing.T) {
	l, supi := registeredLMF(t)

	l.privacyOverride = func(context.Context, etsi.SUPI) error { return errHomeRefuses }

	future := time.Now().Add(time.Hour)

	_, err := l.CreateDeferredRequest(context.Background(), DeferredRequestParams{
		SUPI:              supi,
		Location:          LocationRequest{Method: RequestedCellID},
		PeriodicEventInfo: PeriodicEventInfo{ReportingAmount: 2, ReportingInterval: 60},
		ScheduledLocTime:  &future,
	})
	if !errors.Is(err, ErrPrivacyRefused) {
		t.Fatalf("err = %v, want ErrPrivacyRefused", err)
	}

	rows, total, err := l.ListDeferredRequests(context.Background(), 1, 10, nil, nil)
	if err != nil || total != 0 || len(rows) != 0 {
		t.Fatalf("a refused request was stored: %+v, %d, %v", rows, total, err)
	}
}

func TestDeferredRunEndsWhenTheHomeNoLongerAllowsLocation(t *testing.T) {
	l, supi := registeredLMF(t)

	var refuse atomic.Bool

	l.privacyOverride = func(context.Context, etsi.SUPI) error {
		if refuse.Load() {
			return errHomeRefuses
		}

		return nil
	}

	l.fixOverride = func(context.Context, etsi.SUPI, LocationRequest) (*models.LocationResult, error) {
		refuse.Store(true)
		return &models.LocationResult{SUPI: supi.String()}, nil
	}

	id := startTestRun(t, l, supi, 30*time.Millisecond, 5)

	got := waitForEnd(t, l, id)

	if got.TerminationCause != TerminationByNetwork || got.TerminationReason != TerminationReasonPrivacy || got.EventReportCounter != 1 {
		t.Fatalf("ended request = %+v, want TERMINATION_BY_NETWORK / privacy after 1 report", got)
	}
}
