// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"testing"

	"github.com/ellanetworks/core/internal/db"
)

// TestQosForPolicyDNSeparatesUEAndSessionAMBR verifies the 4G QoS resolution
// keeps the two AMBRs distinct: the UE-AMBR (S1AP) from the profile, and the
// per-APN Session-AMBR (UPF QER + APN-AMBR) from the policy.
func TestQosForPolicyDNSeparatesUEAndSessionAMBR(t *testing.T) {
	profile := &db.Profile{UeAmbrUplink: "500 Mbps", UeAmbrDownlink: "500 Mbps", Allow4G: true}
	pol := &db.Policy{ID: "p1", Var5qi: 7, Arp: 15, SessionAmbrUplink: "30 Mbps", SessionAmbrDownlink: "60 Mbps"}
	dn := &db.DataNetwork{Name: "enterprise", IPv4Pool: "10.46.0.0/16"}

	qos := qosForPolicyDN(profile, pol, dn)

	if qos.AMBRUL != 500_000_000 || qos.AMBRDL != 500_000_000 {
		t.Errorf("UE-AMBR (S1AP) = %d/%d bps, want 500/500 Mbps from the profile", qos.AMBRUL, qos.AMBRDL)
	}

	if qos.SessAmbrULStr != "30 Mbps" || qos.SessAmbrDLStr != "60 Mbps" {
		t.Errorf("Session-AMBR = %q/%q, want 30/60 Mbps from the policy", qos.SessAmbrULStr, qos.SessAmbrDLStr)
	}

	if qos.QCI != 7 || qos.APN != "enterprise" {
		t.Errorf("QCI=%d APN=%q, want 7/enterprise", qos.QCI, qos.APN)
	}
}
