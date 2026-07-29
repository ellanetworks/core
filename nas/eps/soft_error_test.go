// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package eps

import (
	"bytes"
	"errors"
	"testing"

	"github.com/ellanetworks/core/nas"
)

// TestSyntacticallyIncorrectOptionalIEIsAbsent pins TS 24.301 §7.7.1: an optional
// element the network cannot decode is treated as not present, the rest of the
// message decodes, and the element still re-encodes so nothing is lost.
func TestSyntacticallyIncorrectOptionalIEIsAbsent(t *testing.T) {
	// ACTIVATE DEFAULT EPS BEARER CONTEXT REQUEST with a malformed APN-AMBR
	// (IEI 0x5E, which needs at least two octets) ahead of a well-formed ESM cause.
	in := &ActivateDefaultEPSBearerContextRequest{
		EPSBearerIdentity: 5,
		PTI:               1,
		EPSQoS:            EPSQoS{QCI: 9},
		AccessPointName:   APN("internet"),
		PDNAddress:        PDNAddress{PDNType: PDNTypeIPv4, IPv4: [4]byte{10, 45, 0, 1}},
	}

	head, err := in.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}

	b := append(head, ieiAPNAMBR, 0x01, 0xfe, ieiESMCause, 0x32)

	msg, err := ParseActivateDefaultEPSBearerContextRequest(b)
	if err == nil {
		t.Fatal("a malformed optional element must be reported")
	}

	if !nas.SoftOnly(err) {
		t.Fatalf("a malformed optional element must be a soft error, got %v", err)
	}

	if msg == nil {
		t.Fatal("a soft error must still yield a usable message")
	}

	if msg.APNAMBR != nil {
		t.Errorf("the malformed element must be absent, got %+v", msg.APNAMBR)
	}

	// The mandatory part and the element after it still decoded.
	if msg.AccessPointName != "internet" || msg.Cause == nil || *msg.Cause != 0x32 {
		t.Errorf("surrounding elements lost: apn=%q cause=%v", msg.AccessPointName, msg.Cause)
	}

	var ie *nas.IEError
	if !errors.As(err, &ie) || ie.IEI != ieiAPNAMBR {
		t.Fatalf("error does not name the failed element: %v", err)
	}

	again, err := msg.MarshalBinary()
	if err != nil {
		t.Fatalf("MarshalBinary: %v", err)
	}

	if !bytes.Contains(again, []byte{ieiAPNAMBR, 0x01, 0xfe}) {
		t.Errorf("re-encoding dropped the malformed element: % x", again)
	}
}

// TestSecurityCriticalIEFailureIsHard pins the exception: the replayed NAS
// message proves a SECURITY MODE COMPLETE echoes what the UE actually sent, so a
// value that will not decode fails the message rather than vanishing.
func TestSecurityCriticalIEFailureIsHard(t *testing.T) {
	// A replayed NAS message container (IEI 0x4F, TLV-E) whose length runs past
	// the buffer is a framing failure the walk cannot step over.
	b := []byte{
		uint8(PDEMM), uint8(MsgSecurityModeComplete),
		ieiReplayedNASMessage, 0x00, 0x40, 0x01,
	}

	msg, err := ParseSecurityModeComplete(b)
	if err == nil {
		t.Fatal("a malformed security-critical element must be reported")
	}

	if nas.SoftOnly(err) {
		t.Fatalf("a malformed security-critical element must be a hard error, got %v", err)
	}

	if msg != nil {
		t.Fatal("a hard error must not yield a message")
	}
}

// TestRepeatedIEUsesTheFirst pins TS 24.301 §7.6.3: only the first occurrence of
// a repeated element is handled, and the rest are ignored without being lost.
func TestRepeatedIEUsesTheFirst(t *testing.T) {
	b := []byte{
		uint8(PDEMM), uint8(MsgAttachReject), 0x11,
		ieiT3402Value, 0x01, 0x21,
		ieiT3402Value, 0x01, 0x22, // repetition: must be ignored
	}

	msg, err := ParseAttachReject(b)
	if err != nil {
		t.Fatalf("ParseAttachReject: %v", err)
	}

	if msg.T3402 == nil || msg.T3402.Value != 1 {
		t.Fatalf("T3402 = %+v, want the first occurrence", msg.T3402)
	}

	again, err := msg.MarshalBinary()
	if err != nil {
		t.Fatalf("MarshalBinary: %v", err)
	}

	if got := bytes.Count(again, []byte{ieiT3402Value, 0x01}); got != 2 {
		t.Errorf("re-encoding kept %d occurrences, want both: % x", got, again)
	}
}
