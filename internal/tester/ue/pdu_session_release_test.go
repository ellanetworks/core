// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ue

import (
	"sync"
	"testing"

	"github.com/ellanetworks/core/nas/fgs"
)

func TestBuildPDUSessionReleaseRequestRoundTrips(t *testing.T) {
	pdu, err := BuildPDUSessionReleaseRequest(&PDUSessionReleaseRequestOpts{
		PDUSessionID: 5,
		PTI:          7,
		Cause:        fgs.GSMCauseRegularDeactivation,
	})
	if err != nil {
		t.Fatalf("build: %v", err)
	}

	msg, err := fgs.ParsePDUSessionReleaseRequest(pdu)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	if msg.PDUSessionID != 5 {
		t.Errorf("PDU session ID = %d, want 5", msg.PDUSessionID)
	}

	if msg.PTI != 7 {
		t.Errorf("PTI = %d, want 7", msg.PTI)
	}

	if msg.Cause == nil || *msg.Cause != fgs.GSMCauseRegularDeactivation {
		t.Errorf("cause = %v, want regular deactivation", msg.Cause)
	}
}

func TestBuildPDUSessionReleaseRequestRejectsUnusablePTI(t *testing.T) {
	for _, pti := range []uint8{0, 0xff} {
		if _, err := BuildPDUSessionReleaseRequest(&PDUSessionReleaseRequestOpts{
			PDUSessionID: 5,
			PTI:          pti,
		}); err == nil {
			t.Errorf("expected an error for PTI %d", pti)
		}
	}
}

func TestBuildPDUSessionReleaseCompleteRoundTrips(t *testing.T) {
	pdu, err := BuildPDUSessionReleaseComplete(&PDUSessionReleaseCompleteOpts{
		PDUSessionID: 5,
		PTI:          7,
	})
	if err != nil {
		t.Fatalf("build: %v", err)
	}

	msg, err := fgs.ParsePDUSessionReleaseComplete(pdu)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	if msg.PDUSessionID != 5 {
		t.Errorf("PDU session ID = %d, want 5", msg.PDUSessionID)
	}

	if msg.PTI != 7 {
		t.Errorf("PTI = %d, want 7", msg.PTI)
	}
}

func TestBuildUplinkNasTransportSMOmitsEstablishmentIEs(t *testing.T) {
	release, err := BuildPDUSessionReleaseComplete(&PDUSessionReleaseCompleteOpts{PDUSessionID: 5, PTI: 7})
	if err != nil {
		t.Fatalf("build release complete: %v", err)
	}

	pdu, err := BuildUplinkNasTransportSM(5, release)
	if err != nil {
		t.Fatalf("build: %v", err)
	}

	msg, err := fgs.ParseULNASTransport(pdu)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	if msg.PDUSessionID == nil || *msg.PDUSessionID != 5 {
		t.Errorf("PDU session ID = %v, want 5", msg.PDUSessionID)
	}

	if msg.RequestType != nil {
		t.Errorf("request type = %v, want none", *msg.RequestType)
	}

	if msg.DNN != nil {
		t.Errorf("DNN = %v, want none", msg.DNN)
	}

	if msg.SNSSAI != nil {
		t.Errorf("S-NSSAI = %v, want none", msg.SNSSAI)
	}
}

func TestBuildUplinkNasTransportSMRequiresSessionAndPayload(t *testing.T) {
	if _, err := BuildUplinkNasTransportSM(0, []byte{0x01}); err == nil {
		t.Error("expected an error for PDU session ID 0")
	}

	if _, err := BuildUplinkNasTransportSM(5, nil); err == nil {
		t.Error("expected an error for an empty payload container")
	}
}

func TestDropPDUSessionForgetsTheSession(t *testing.T) {
	u := &UE{pduSessions: map[uint8]PDUSessionInfo{}}
	u.cond = sync.NewCond(&u.mu)

	u.pduSessions[1] = PDUSessionInfo{PDUSessionID: 1}
	u.pduSessions[5] = PDUSessionInfo{PDUSessionID: 5}

	u.DropPDUSession(1)

	got := u.ActivePDUSessionIDs()
	if len(got) != 1 || got[0] != 5 {
		t.Fatalf("ActivePDUSessionIDs = %v, want [5]", got)
	}
}

func TestNextPTISkipsTheUnassignedAndReservedValues(t *testing.T) {
	u := &UE{lastPTI: 0xfd}

	if got := u.nextPTI(); got != 0xfe {
		t.Fatalf("nextPTI = %d, want 254", got)
	}

	if got := u.nextPTI(); got != 1 {
		t.Fatalf("nextPTI = %d, want it to wrap to 1", got)
	}
}
