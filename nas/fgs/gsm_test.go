// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package fgs

import (
	"bytes"
	"testing"
)

func TestSMBuildersWireBytes(t *testing.T) {
	cases := []struct {
		name string
		got  []byte
		want []byte
	}{
		{
			"GSMStatus",
			mustMarshal(t, (&GSMStatus{PDUSessionID: 5, PTI: 1, Cause: GSMCausePTIMismatch}).Marshal),
			[]byte{EPD5GSM, 5, 1, uint8(MsgGSMStatus), GSMCausePTIMismatch},
		},
		{
			"EstablishmentReject",
			mustMarshal(t, (&PDUSessionEstablishmentReject{PDUSessionID: 5, PTI: 1, Cause: GSMCauseRequestRejectedUnspecified}).Marshal),
			[]byte{EPD5GSM, 5, 1, uint8(MsgPDUSessionEstablishmentReject), GSMCauseRequestRejectedUnspecified},
		},
		{
			"ModificationReject",
			mustMarshal(t, (&PDUSessionModificationReject{PDUSessionID: 5, PTI: 1, Cause: GSMCauseInsufficientResources}).Marshal),
			[]byte{EPD5GSM, 5, 1, uint8(MsgPDUSessionModificationReject), GSMCauseInsufficientResources},
		},
		{
			"ReleaseCommand",
			mustMarshal(t, (&PDUSessionReleaseCommand{PDUSessionID: 5, PTI: 1, Cause: GSMCauseRegularDeactivation}).Marshal),
			[]byte{EPD5GSM, 5, 1, uint8(MsgPDUSessionReleaseCommand), GSMCauseRegularDeactivation},
		},
	}

	for _, c := range cases {
		if !bytes.Equal(c.got, c.want) {
			t.Errorf("%s = %#x, want %#x", c.name, c.got, c.want)
		}
	}
}

func TestStatus5GSMRoundTrip(t *testing.T) {
	orig := &GSMStatus{PDUSessionID: 3, PTI: 7, Cause: GSMCauseProtocolErrorUnspecified}

	b := mustMarshal(t, orig.Marshal)

	got, err := ParseGSMStatus(b)
	if err != nil {
		t.Fatalf("ParseGSMStatus: %v", err)
	}

	if *got != *orig {
		t.Fatalf("round-trip = %+v, want %+v", got, orig)
	}
}

func TestParseEstablishmentRequest(t *testing.T) {
	// header | integrity-max-data-rate(2) | PDU session type (0x9, IPv4) |
	// 5GSM capability (0x28 TLV, 1 octet) | always-on requested (0xB, present)
	b := []byte{
		EPD5GSM, 5, 1, uint8(MsgPDUSessionEstablishmentRequest),
		0xFF, 0xFF,
		0x90 | PDUSessionTypeIPv4,
		iei5GSMCapability, 0x01, 0x00,
		0xB0 | 0x01,
	}

	req, err := ParsePDUSessionEstablishmentRequest(b)
	if err != nil {
		t.Fatalf("ParsePDUSessionEstablishmentRequest: %v", err)
	}

	if req.PDUSessionID != 5 || req.PTI != 1 {
		t.Errorf("header psi=%d pti=%d, want 5/1", req.PDUSessionID, req.PTI)
	}

	if req.IntegrityProtMaxDataRate != [2]byte{0xFF, 0xFF} {
		t.Errorf("integrity max data rate = %#x, want ffff", req.IntegrityProtMaxDataRate)
	}

	// always-on must be reached even though a full-octet IE (0x28) precedes it.
	if !req.AlwaysOnRequested {
		t.Error("AlwaysOnRequested = false, want true")
	}

	if req.PDUSessionType == nil || *req.PDUSessionType != PDUSessionTypeIPv4 {
		t.Errorf("PDUSessionType = %v, want IPv4", req.PDUSessionType)
	}
}

func TestParseReleaseWithOptionalCause(t *testing.T) {
	reqBytes := []byte{EPD5GSM, 5, 1, uint8(MsgPDUSessionReleaseRequest), iei5GSMCause, GSMCauseRegularDeactivation}

	req, err := ParsePDUSessionReleaseRequest(reqBytes)
	if err != nil {
		t.Fatalf("ParsePDUSessionReleaseRequest: %v", err)
	}

	if req.Cause == nil || *req.Cause != GSMCauseRegularDeactivation {
		t.Errorf("release request cause = %v, want RegularDeactivation", req.Cause)
	}

	// Release complete with no optional cause.
	compBytes := []byte{EPD5GSM, 5, 1, uint8(MsgPDUSessionReleaseComplete)}

	comp, err := ParsePDUSessionReleaseComplete(compBytes)
	if err != nil {
		t.Fatalf("ParsePDUSessionReleaseComplete: %v", err)
	}

	if comp.Cause != nil {
		t.Errorf("release complete cause = %v, want nil", comp.Cause)
	}
}

func TestParseModification(t *testing.T) {
	comp, err := ParsePDUSessionModificationComplete([]byte{EPD5GSM, 5, 1, uint8(MsgPDUSessionModificationComplete)})
	if err != nil {
		t.Fatalf("ParsePDUSessionModificationComplete: %v", err)
	}

	if comp.PDUSessionID != 5 || comp.PTI != 1 {
		t.Errorf("modification complete psi=%d pti=%d, want 5/1", comp.PDUSessionID, comp.PTI)
	}

	rej, err := ParsePDUSessionModificationCommandReject(
		[]byte{EPD5GSM, 5, 1, uint8(MsgPDUSessionModificationCmdReject), GSMCauseInvalidPTIValue})
	if err != nil {
		t.Fatalf("ParsePDUSessionModificationCommandReject: %v", err)
	}

	if rej.Cause != GSMCauseInvalidPTIValue {
		t.Errorf("command reject cause = %#x, want %#x", rej.Cause, GSMCauseInvalidPTIValue)
	}
}

func TestParseRejectsWrongMessageType(t *testing.T) {
	// A 5GSM STATUS body parsed as a release request must fail on the type.
	if _, err := ParsePDUSessionReleaseRequest([]byte{EPD5GSM, 5, 1, uint8(MsgGSMStatus)}); err == nil {
		t.Error("expected wrong-message-type error, got nil")
	}
}

func mustMarshal(t *testing.T, fn func() ([]byte, error)) []byte {
	t.Helper()

	b, err := fn()
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	return b
}
