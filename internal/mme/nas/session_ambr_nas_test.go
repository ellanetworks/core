// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"net/netip"
	"testing"

	"github.com/ellanetworks/core/internal/mme"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/nas/eps"
)

// TS 24.301 §8.3.6.7
func TestBuildActivateDefaultESMSignalsAPNAMBR(t *testing.T) {
	p := &mme.PdnConnection{Ebi: mme.DefaultERABID, PdnType: eps.PDNTypeIPv4, UeIP: netip.MustParseAddr("10.45.0.1")}
	qos := &mme.EpsQoS{APN: "internet", QCI: 9, SessAmbrDL: models.MustParseBitRate("100 Mbps"), SessAmbrUL: models.MustParseBitRate("50 Mbps")}

	wire, err := buildActivateDefaultESM(p, qos, 1, models.PlmnID{Mcc: "001", Mnc: "01"}, false)
	if err != nil {
		t.Fatalf("buildActivateDefaultESM: %v", err)
	}

	act, err := eps.ParseActivateDefaultEPSBearerContextRequest(wire)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	if act.APNAMBR == nil {
		t.Fatal("APN-AMBR IE not signaled in the Activate Default EPS Bearer Context Request")
	}

	ambr := *act.APNAMBR

	if dl, ul, ok := ambr.Kbps(); !ok || dl != 100_000 || ul != 50_000 {
		t.Errorf("signaled APN-AMBR = %d/%d kbit/s, want 100/50 Mbps", dl, ul)
	}
}
