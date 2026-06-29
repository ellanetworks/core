// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"net/netip"
	"testing"

	"github.com/ellanetworks/core/internal/mme"
	"github.com/ellanetworks/core/nas/eps"
)

// TestBuildActivateDefaultESMSignalsAPNAMBR verifies the Activate Default EPS
// Bearer Context Request now carries the APN-AMBR IE encoding the policy
// Session-AMBR (TS 24.301 §8.3.6.7).
func TestBuildActivateDefaultESMSignalsAPNAMBR(t *testing.T) {
	p := &mme.PdnConnection{Ebi: mme.DefaultERABID, PdnType: eps.PDNTypeIPv4, UeIP: netip.MustParseAddr("10.45.0.1")}
	qos := &mme.EpsQoS{APN: "internet", QCI: 9, SessAmbrDLStr: "100 Mbps", SessAmbrULStr: "50 Mbps"}

	wire, err := buildActivateDefaultESM(p, qos, 1)
	if err != nil {
		t.Fatalf("buildActivateDefaultESM: %v", err)
	}

	act, err := eps.ParseActivateDefaultEPSBearerContextRequest(wire)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	if len(act.APNAMBR) == 0 {
		t.Fatal("APN-AMBR IE not signaled in the Activate Default EPS Bearer Context Request")
	}

	ambr, err := eps.ParseAPNAMBR(act.APNAMBR)
	if err != nil {
		t.Fatalf("ParseAPNAMBR: %v", err)
	}

	if dl, ul := ambr.BitsPerSecond(); dl != 100_000_000 || ul != 50_000_000 {
		t.Errorf("signaled APN-AMBR = %d/%d bps, want 100/50 Mbps", dl, ul)
	}
}
