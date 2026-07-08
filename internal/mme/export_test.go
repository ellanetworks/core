// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"context"
	"net/netip"
	"testing"
	"time"

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/sctp"
)

func TestExportUEs(t *testing.T) {
	m := newTestMME(t)

	conn := new(sctp.SCTPConn)
	m.trackRadio(conn, RadioInfo{Name: "enb-a", ID: "00f110-1"})

	ue := m.NewUe(conn, 7)
	registerTestUE(m, ue, "001010000000001")
	ue.ForceStateForTest(EMMRegistered)
	ue.cipheringAlg = 2
	ue.integrityAlg = 2
	ue.Imei, _ = etsi.NewIMEIFromPEI("353456789012347")
	ue.Ambr = &models.Ambr{Uplink: "1 Gbps", Downlink: "2 Gbps"}

	pdn := testPDN(ue)
	pdn.Apn = "internet"
	pdn.PdnType = 1
	pdn.Qci = 9
	pdn.UeIP = netip.MustParseAddr("10.45.0.2")

	ue.TouchLastSeen()

	exports, err := m.ExportUEs(context.Background())
	if err != nil {
		t.Fatalf("ExportUEs: %v", err)
	}

	if len(exports) != 1 {
		t.Fatalf("ExportUEs returned %d entries, want 1: %+v", len(exports), exports)
	}

	e := exports[0]

	if e.Identity.Supi != "imsi-001010000000001" {
		t.Errorf("Supi = %q, want imsi-001010000000001", e.Identity.Supi)
	}

	if e.Identity.PlmnID.Mcc != "001" || e.Identity.PlmnID.Mnc != "01" {
		t.Errorf("PlmnID = %+v, want {001 01}", e.Identity.PlmnID)
	}

	if e.Identity.Pei == "" {
		t.Error("Pei is empty, want the UE IMEI")
	}

	if e.State.EMMState != "EMM-REGISTERED" {
		t.Errorf("EMMState = %q, want EMM-REGISTERED", e.State.EMMState)
	}

	if e.Security.CipheringAlgorithm != "EEA2" || e.Security.IntegrityAlgorithm != "EIA2" {
		t.Errorf("Security = %+v, want EEA2/EIA2", e.Security)
	}

	if e.Subscription.Ambr == nil || e.Subscription.Ambr.Uplink != "1 Gbps" {
		t.Errorf("Subscription.Ambr = %+v, want uplink 1 Gbps", e.Subscription.Ambr)
	}

	if len(e.PDNConnections) != 1 {
		t.Fatalf("PDNConnections = %+v, want 1", e.PDNConnections)
	}

	for _, p := range e.PDNConnections {
		if p.Apn != "internet" {
			t.Errorf("PDN Apn = %q, want internet", p.Apn)
		}

		if p.UeIPv4Address != "10.45.0.2" {
			t.Errorf("PDN UeIPv4Address = %q, want 10.45.0.2", p.UeIPv4Address)
		}

		if p.Qci != 9 {
			t.Errorf("PDN Qci = %d, want 9", p.Qci)
		}
	}

	if e.RANConnection == nil || e.RANConnection.ENBName != "enb-a" {
		t.Errorf("RANConnection = %+v, want ENBName enb-a", e.RANConnection)
	}

	if want := int64(T3412PeriodicTAU / time.Second); e.Timers.T3412ValueSeconds != want {
		t.Errorf("T3412ValueSeconds = %d, want %d", e.Timers.T3412ValueSeconds, want)
	}
}

func TestExportUEsEmpty(t *testing.T) {
	m := newTestMME(t)

	exports, err := m.ExportUEs(context.Background())
	if err != nil {
		t.Fatalf("ExportUEs returned unexpected error: %v", err)
	}

	if len(exports) != 0 {
		t.Fatalf("ExportUEs returned %d entries, want 0", len(exports))
	}
}
