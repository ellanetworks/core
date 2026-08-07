// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/internal/mme"
	"github.com/ellanetworks/core/nas/eps"
)

// TS 24.301 §6.5.1.6 a) resends the activate only for a request repeating the
// PTI, APN and PDN type of a connection whose ACTIVATE ACCEPT has not arrived.
// A different APN under the same PTI is a new request, not a retransmission.
func TestRetransmissionMatchesAPNAndPDNTypeNotPTIAlone(t *testing.T) {
	m := newTestMME(t)
	ue, _ := securedUE(t, m)
	ue.ForceStateForTest(mme.EMMRegistered)

	first := eps.APN("internet")
	handlePDNConnectivityRequest(context.Background(), m, ue, &eps.PDNConnectivityRequest{
		PTI: 4, RequestType: eps.RequestTypeInitialRequest, PDNType: eps.PDNTypeIPv4,
		AccessPointName: &first,
	})

	if got := len(m.SnapshotPDNs(ue)); got != 1 {
		t.Fatalf("PDN connections after the first request = %d, want 1", got)
	}

	// The same PTI under a different APN or PDN type is a new request.
	if prev, _ := m.FindRetransmittedPDN(ue, 4, "enterprise", eps.PDNTypeIPv4); prev != nil {
		t.Errorf("APN \"enterprise\" under PTI 4 matched EBI %d as a retransmission, want no match", prev.Ebi)
	}

	if prev, _ := m.FindRetransmittedPDN(ue, 4, "internet", eps.PDNTypeIPv6); prev != nil {
		t.Errorf("PDN type IPv6 under PTI 4 matched EBI %d as a retransmission, want no match", prev.Ebi)
	}

	// The genuine repeat still matches.
	if prev, _ := m.FindRetransmittedPDN(ue, 4, "internet", eps.PDNTypeIPv4); prev == nil {
		t.Error("the repeated request did not match as a retransmission")
	}
}

// Once the UE accepts, the retained request stops answering a repeat of its PTI,
// which the UE may legally reuse (TS 24.007 §11.2.3.1a).
func TestRetransmissionRecordClearedOnAccept(t *testing.T) {
	m := newTestMME(t)
	ue, _ := securedUE(t, m)
	ue.ForceStateForTest(mme.EMMRegistered)

	apn := eps.APN("internet")
	handlePDNConnectivityRequest(context.Background(), m, ue, &eps.PDNConnectivityRequest{
		PTI: 4, RequestType: eps.RequestTypeInitialRequest, PDNType: eps.PDNTypeIPv4,
		AccessPointName: &apn,
	})

	pdns := m.SnapshotPDNs(ue)
	if len(pdns) != 1 {
		t.Fatalf("PDN connections = %d, want 1", len(pdns))
	}

	if prev, _ := m.FindRetransmittedPDN(ue, 4, "internet", eps.PDNTypeIPv4); prev == nil {
		t.Fatal("no retransmission record before the accept")
	}

	handleActivateDefaultBearerAccept(m, ue, &eps.ActivateDefaultEPSBearerContextAccept{
		EPSBearerIdentity: eps.EPSBearerIdentity(pdns[0].Ebi),
	})

	if prev, _ := m.FindRetransmittedPDN(ue, 4, "internet", eps.PDNTypeIPv4); prev != nil {
		t.Error("the retransmission record survived the ACTIVATE ACCEPT")
	}
}
