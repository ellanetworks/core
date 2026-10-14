// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package enginetest_test

import (
	"context"
	"errors"
	"net/netip"
	"testing"

	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/upf/engine"
	"github.com/ellanetworks/core/internal/upf/engine/enginetest"
)

var errInjected = errors.New("injected map failure")

func newEngine(t *testing.T, maps engine.Maps) *engine.SessionEngine {
	t.Helper()

	rm, err := engine.NewFteIDResourceManager(1024)
	if err != nil {
		t.Fatalf("fteid manager: %v", err)
	}

	n3 := netip.MustParseAddr("2.3.4.5")

	eng, err := engine.NewSessionEngine("1.2.3.4", "nodeId", n3, netip.Addr{}, n3, netip.Addr{}, nil, rm)
	if err != nil {
		t.Fatalf("new session engine: %v", err)
	}

	eng.SetMaps(maps)

	return eng
}

func sessionRequest(seid uint64) *models.EstablishRequest {
	return &models.EstablishRequest{
		SEID: seid,
		IMSI: "001010000000001",
		URRs: []models.URR{{URRID: 1}},
		PDRs: []models.PDR{
			{PDRID: 1, FARID: 1, QERID: 1, URRID: 1, PDI: models.PDI{LocalFTEID: &models.FTEID{}}},
			{PDRID: 2, FARID: 1, QERID: 1, URRID: 1, PDI: models.PDI{UEIPAddress: netip.MustParseAddr("10.45.0.1")}},
		},
		FARs: []models.FAR{{FARID: 1, ApplyAction: models.ApplyAction{Forw: true}}},
		QERs: []models.QER{{QERID: 1, QFI: 9}},
	}
}

func TestEstablishSessionInstallsUplinkAndDownlinkPDRs(t *testing.T) {
	maps := enginetest.NewMaps()
	eng := newEngine(t, maps)

	resp, err := eng.EstablishSession(context.Background(), sessionRequest(1))
	if err != nil {
		t.Fatalf("establish: %v", err)
	}

	if _, ok := maps.UplinkPDR(resp.N3TEID); !ok {
		t.Fatalf("no uplink PDR for allocated TEID %d", resp.N3TEID)
	}

	if _, ok := maps.DownlinkPDR(netip.MustParseAddr("10.45.0.1")); !ok {
		t.Fatal("no downlink PDR for the UE address")
	}
}

func TestEstablishSessionRollsBackWhenAMapWriteFails(t *testing.T) {
	for _, op := range []enginetest.Op{enginetest.OpNewUrr, enginetest.OpPutPdrUplink, enginetest.OpPutPdrDownlink} {
		t.Run(string(op), func(t *testing.T) {
			maps := enginetest.NewMaps()
			faulty := enginetest.NewFaultyMaps(maps)
			eng := newEngine(t, faulty)

			faulty.Fail(op, errInjected)

			if _, err := eng.EstablishSession(context.Background(), sessionRequest(7)); err == nil {
				t.Fatalf("establish succeeded although %s failed", op)
			}

			if eng.GetSession(7) != nil {
				t.Fatal("session registered after a failed establish")
			}

			if up, down, framed, urrs := maps.Counts(); up+down+framed+urrs != 0 {
				t.Fatalf("leaked map entries after rollback: uplink=%d downlink=%d framed=%d urrs=%d", up, down, framed, urrs)
			}

			faulty.Heal(op)

			if _, err := eng.EstablishSession(context.Background(), sessionRequest(7)); err != nil {
				t.Fatalf("establish after the fault cleared: %v", err)
			}
		})
	}
}
