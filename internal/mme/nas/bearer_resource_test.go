// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/internal/mme"
	"github.com/ellanetworks/core/internal/nasreply"
	"github.com/ellanetworks/core/nas/eps"
)

func TestBearerResourceAllocationRejected(t *testing.T) {
	m := newTestMME(t)
	ue, cc := securedUE(t, m)

	req, err := (&eps.BearerResourceAllocationRequest{PTI: 3}).MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}

	d := HandleEmmMessage(context.Background(), m, ue, ue.Conn(), req, true)
	if d.Action != nasreply.ActionHandled {
		t.Fatalf("disposition = %+v, want ActionHandled (a reject was already sent)", d)
	}

	reject, err := eps.ParseBearerResourceAllocationReject(lastDownlinkESM(t, ue, cc))
	if err != nil {
		t.Fatalf("expected a Bearer Resource Allocation Reject: %v", err)
	}

	if reject.PTI != 3 {
		t.Fatalf("PTI = %d, want 3 (echoed from the request)", reject.PTI)
	}

	if reject.Cause != eps.ESMCauseRequestRejectedUnspecified {
		t.Fatalf("ESM cause = %d, want %d (request rejected, unspecified)", reject.Cause, eps.ESMCauseRequestRejectedUnspecified)
	}
}

func TestBearerResourceModificationForResourcesRejected(t *testing.T) {
	m := newTestMME(t)
	ue, cc := securedUE(t, m)

	testPDN(ue)

	req, err := (&eps.BearerResourceModificationRequest{
		PTI:                              7,
		EPSBearerIdentityForPacketFilter: eps.EPSBearerIdentity(mme.DefaultERABID),
		TrafficFlowAggregate:             []byte{0x41, 0x01},
	}).MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}

	d := HandleEmmMessage(context.Background(), m, ue, ue.Conn(), req, true)
	if d.Action != nasreply.ActionHandled {
		t.Fatalf("disposition = %+v, want ActionHandled (a reject was already sent)", d)
	}

	reject, err := eps.ParseBearerResourceModificationReject(lastDownlinkESM(t, ue, cc))
	if err != nil {
		t.Fatalf("expected a Bearer Resource Modification Reject: %v", err)
	}

	if reject.PTI != 7 {
		t.Fatalf("PTI = %d, want 7 (echoed from the request)", reject.PTI)
	}

	if reject.Cause != eps.ESMCauseEPSQoSNotAccepted {
		t.Fatalf("ESM cause = %d, want %d (EPS QoS not accepted)", reject.Cause, eps.ESMCauseEPSQoSNotAccepted)
	}
}

func TestBearerResourceModificationWithoutResourcesAccepted(t *testing.T) {
	m := newTestMME(t)
	ue, cc := securedUE(t, m)

	testPDN(ue)

	req, err := (&eps.BearerResourceModificationRequest{
		PTI:                              7,
		EPSBearerIdentityForPacketFilter: eps.EPSBearerIdentity(mme.DefaultERABID),
		TrafficFlowAggregate:             []byte{0xC0},
	}).MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}

	d := HandleEmmMessage(context.Background(), m, ue, ue.Conn(), req, true)
	if d.Action != nasreply.ActionHandled {
		t.Fatalf("disposition = %+v, want ActionHandled", d)
	}

	modify, err := eps.ParseModifyEPSBearerContextRequest(lastDownlinkESM(t, ue, cc))
	if err != nil {
		t.Fatalf("expected a Modify EPS Bearer Context Request: %v", err)
	}

	if modify.PTI != 7 {
		t.Fatalf("PTI = %d, want 7 (echoed from the request, TS 24.301 §6.5.4.3)", modify.PTI)
	}

	if uint8(modify.EPSBearerIdentity) != mme.DefaultERABID {
		t.Fatalf("EPS bearer identity = %d, want %d", modify.EPSBearerIdentity, mme.DefaultERABID)
	}

	if modify.NewEPSQoS != nil || modify.APNAMBR != nil {
		t.Error("the acceptance changed a bearer parameter the UE did not ask about")
	}
}

func TestBearerResourceModificationUnknownBearerRejected(t *testing.T) {
	m := newTestMME(t)
	ue, cc := securedUE(t, m)

	req, err := (&eps.BearerResourceModificationRequest{
		PTI:                              7,
		EPSBearerIdentityForPacketFilter: 9,
		TrafficFlowAggregate:             []byte{0xC0},
	}).MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}

	HandleEmmMessage(context.Background(), m, ue, ue.Conn(), req, true)

	reject, err := eps.ParseBearerResourceModificationReject(lastDownlinkESM(t, ue, cc))
	if err != nil {
		t.Fatalf("expected a Bearer Resource Modification Reject: %v", err)
	}

	if reject.Cause != eps.ESMCauseInvalidEPSBearerIdentity {
		t.Fatalf("ESM cause = %d, want %d (invalid EPS bearer identity)", reject.Cause, eps.ESMCauseInvalidEPSBearerIdentity)
	}
}

// TS 24.301 §7.3
func TestBearerResourceModificationInvalidPTI(t *testing.T) {
	m := newTestMME(t)
	ue, cc := securedUE(t, m)

	req, err := (&eps.BearerResourceModificationRequest{PTI: 0}).MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}

	HandleEmmMessage(context.Background(), m, ue, ue.Conn(), req, true)

	reject, err := eps.ParseBearerResourceModificationReject(lastDownlinkESM(t, ue, cc))
	if err != nil {
		t.Fatalf("expected a Bearer Resource Modification Reject: %v", err)
	}

	if reject.Cause != eps.ESMCauseInvalidPTIValue {
		t.Fatalf("ESM cause = %d, want %d (invalid PTI value)", reject.Cause, eps.ESMCauseInvalidPTIValue)
	}
}
