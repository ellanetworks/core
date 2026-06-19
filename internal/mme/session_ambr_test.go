// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"net/netip"
	"testing"

	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/nas/eps"
)

// TestQosForPolicyDNSeparatesUEAndSessionAMBR verifies the 4G QoS resolution
// keeps the two AMBRs distinct: the UE-AMBR (S1AP) from the profile, and the
// per-APN Session-AMBR (UPF QER + APN-AMBR) from the policy.
func TestQosForPolicyDNSeparatesUEAndSessionAMBR(t *testing.T) {
	profile := &db.Profile{UeAmbrUplink: "500 Mbps", UeAmbrDownlink: "500 Mbps", Allow4G: true}
	pol := &db.Policy{ID: "p1", Var5qi: 7, Arp: 15, SessionAmbrUplink: "30 Mbps", SessionAmbrDownlink: "60 Mbps"}
	dn := &db.DataNetwork{Name: "enterprise", IPv4Pool: "10.46.0.0/16"}

	qos := (&MME{}).qosForPolicyDN(profile, pol, dn)

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

// TestBuildActivateDefaultESMSignalsAPNAMBR verifies the Activate Default EPS
// Bearer Context Request now carries the APN-AMBR IE encoding the policy
// Session-AMBR (TS 24.301 §8.3.6.7).
func TestBuildActivateDefaultESMSignalsAPNAMBR(t *testing.T) {
	p := &pdnConnection{ebi: defaultERABID, pdnType: eps.PDNTypeIPv4, ueIP: netip.MustParseAddr("10.45.0.1")}
	qos := &epsQoS{APN: "internet", QCI: 9, SessAmbrDLStr: "100 Mbps", SessAmbrULStr: "50 Mbps"}

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
