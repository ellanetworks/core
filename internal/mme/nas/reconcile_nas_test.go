// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"context"
	"net/netip"
	"slices"
	"testing"

	"github.com/ellanetworks/core/internal/mme"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/nas/eps"
)

func TestDeactivateBearerAcceptReleases(t *testing.T) {
	m := newTestMME(t)
	ue, cc := connectedBearerUE(t, m)
	testPDN(ue).Deactivating = true

	plain, err := (&eps.DeactivateEPSBearerContextAccept{EPSBearerIdentity: eps.EPSBearerIdentity(mme.DefaultERABID)}).MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}

	wire, err := eps.Protect(plain, eps.SHTIntegrityProtectedCiphered, nas.MakeCount(0, uint8(ue.ULCount())), nas.DirectionUplink, mustSecurityContext(t, ue.EIA(), ue.EEA(), ue.KnasIntForTest(), ue.KnasEncForTest()))
	if err != nil {
		t.Fatal(err)
	}

	HandleNAS(context.Background(), m, ue.Conn(), wire)

	if !m.Session.(*fakeSessionManager).released {
		t.Fatal("EPS session not released after Deactivate Accept")
	}

	if ue.EMMState() != mme.EMMDeregistered {
		t.Fatal("UE not EMM-DEREGISTERED after Deactivate Accept")
	}

	if len(cc.sent) != 1 {
		t.Fatalf("expected a UE Context Release Command after Deactivate Accept, got %d", len(cc.sent))
	}

	parseUEContextReleaseCommand(t, cc.sent[0])
}

// TS 23.401 §5.10.3
func TestDeactivateBearerAcceptKeepsAUEWithAnotherPDN(t *testing.T) {
	m := newTestMME(t)
	ue, cc := connectedBearerUE(t, m)
	testPDN(ue).Deactivating = true
	ue.EnsurePDN(6)

	plain, err := (&eps.DeactivateEPSBearerContextAccept{EPSBearerIdentity: eps.EPSBearerIdentity(mme.DefaultERABID)}).MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}

	wire, err := eps.Protect(plain, eps.SHTIntegrityProtectedCiphered, nas.MakeCount(0, uint8(ue.ULCount())), nas.DirectionUplink, mustSecurityContext(t, ue.EIA(), ue.EEA(), ue.KnasIntForTest(), ue.KnasEncForTest()))
	if err != nil {
		t.Fatal(err)
	}

	HandleNAS(context.Background(), m, ue.Conn(), wire)

	if ue.EMMState() == mme.EMMDeregistered {
		t.Error("the UE was detached though it still has a PDN connection on EBI 6")
	}

	if m.LookupPDN(ue, mme.DefaultERABID) != nil {
		t.Error("the deactivated PDN connection was not released")
	}

	if m.LookupPDN(ue, 6) == nil {
		t.Error("the surviving PDN connection was released too")
	}

	if len(cc.sent) != 0 {
		t.Errorf("sent %d S1AP messages, want 0: releasing one of two PDNs must not release the UE context", len(cc.sent))
	}
}

func TestModifyBearerAcceptCommitsConfig(t *testing.T) {
	m := newTestMME(t)
	ue, cc := connectedBearerUE(t, m)
	dns := netip.MustParseAddr("9.9.9.9")
	testPDN(ue).Modifying = &models.EPSBearerModification{
		QoS:     &models.EPSBearerQoS{QCI: 8, ARP: 2},
		APNAMBR: &models.Ambr{Uplink: models.MustParseBitRate("300 Mbps"), Downlink: models.MustParseBitRate("400 Mbps")},
		DNS:     dns,
	}

	plain, err := (&eps.ModifyEPSBearerContextAccept{EPSBearerIdentity: eps.EPSBearerIdentity(mme.DefaultERABID)}).MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}

	wire, err := eps.Protect(plain, eps.SHTIntegrityProtectedCiphered, nas.MakeCount(0, uint8(ue.ULCount())), nas.DirectionUplink, mustSecurityContext(t, ue.EIA(), ue.EEA(), ue.KnasIntForTest(), ue.KnasEncForTest()))
	if err != nil {
		t.Fatal(err)
	}

	HandleNAS(context.Background(), m, ue.Conn(), wire)

	p := testPDN(ue)

	if p.Modifying != nil {
		t.Fatal("UE still marked modifying after Modify Accept")
	}

	if p.Qci != 8 || p.Arp != 2 || p.Dns != dns || p.SessAmbrDLBps != 400_000_000 || p.SessAmbrULBps != 300_000_000 {
		t.Fatalf("committed QCI %d ARP %d DNS %v AMBR %d/%d, want the accepted modification", p.Qci, p.Arp, p.Dns, p.SessAmbrDLBps, p.SessAmbrULBps)
	}

	want := []bearerModificationOutcome{{ref: "ref-internet", accepted: true}}
	if got := m.Session.(*fakeSessionManager).concluded; !slices.Equal(got, want) {
		t.Fatalf("SMF told %+v, want %+v", got, want)
	}

	if m.Session.(*fakeSessionManager).released {
		t.Fatal("EPS session released on a modification (must stay up)")
	}

	if len(cc.sent) != 0 {
		t.Fatalf("modification accept must not trigger downlink S1AP, got %d", len(cc.sent))
	}
}
