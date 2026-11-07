// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package lmf

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/mme"
	"github.com/ellanetworks/core/internal/roaming"
)

func roamerSUPI(t *testing.T) etsi.SUPI {
	t.Helper()

	supi, err := etsi.NewSUPIFromIMSI("001020000000001")
	if err != nil {
		t.Fatal(err)
	}

	return supi
}

func TestDetermineLocationRefusesA5GRoamer(t *testing.T) {
	l, _ := registeredLMF(t)
	roamer := roamerSUPI(t)

	l.amf.HoldVisitorForTest(roamer, roaming.Visitor5G{PartnerID: "p", Registered: true})

	if _, err := l.DetermineLocation(context.Background(), roamer, LocationRequest{Method: RequestedCellID}); !errors.Is(err, ErrRoamer) {
		t.Fatalf("err = %v, want ErrRoamer", err)
	}

	if _, err := l.DetermineLocation(context.Background(), roamer, LocationRequest{Method: "nope"}); !errors.Is(err, ErrUnsupportedMethod) {
		t.Fatalf("an invalid request must fail validation first: %v", err)
	}
}

func TestDetermineLocationRefusesA4GRoamer(t *testing.T) {
	m := mme.New(nil, nil, nil)
	roamer := roamerSUPI(t)

	ue := mme.NewUeContext()
	ue.SetVisitorForTest(roaming.Visitor{PartnerID: "p", Registered: true})
	m.RegisterUEForTest(ue, roamer.IMSI())

	l := New(nil, m, nil)

	if _, err := l.DetermineLocation(context.Background(), roamer, LocationRequest{Method: RequestedCellID}); !errors.Is(err, ErrRoamer) {
		t.Fatalf("err = %v, want ErrRoamer", err)
	}
}

func TestDetermineLocationOfAnOwnSubscriberIsNotRefused(t *testing.T) {
	l, supi := registeredLMF(t)

	if _, err := l.DetermineLocation(context.Background(), supi, LocationRequest{Method: RequestedCellID}); errors.Is(err, ErrRoamer) {
		t.Fatalf("home subscriber refused: %v", err)
	}
}

func TestCreateDeferredRequestRefusesARoamer(t *testing.T) {
	l, _ := registeredLMF(t)
	roamer := roamerSUPI(t)

	ue := l.amf.HoldVisitorForTest(roamer, roaming.Visitor5G{PartnerID: "p", Registered: true})
	ue.ForceStateForTest(amf.Registered)

	future := time.Now().Add(time.Hour)

	_, err := l.CreateDeferredRequest(context.Background(), DeferredRequestParams{
		SUPI:              roamer,
		Location:          LocationRequest{Method: RequestedCellID},
		PeriodicEventInfo: PeriodicEventInfo{ReportingAmount: 2, ReportingInterval: 60},
		ScheduledLocTime:  &future,
	})
	if !errors.Is(err, ErrRoamer) {
		t.Fatalf("err = %v, want ErrRoamer", err)
	}

	rows, total, err := l.ListDeferredRequests(context.Background(), 1, 10, nil, nil)
	if err != nil || total != 0 || len(rows) != 0 {
		t.Fatalf("a refused request was stored: %+v, %d, %v", rows, total, err)
	}
}
