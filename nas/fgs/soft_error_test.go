// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package fgs

import (
	"bytes"
	"errors"
	"testing"

	"github.com/ellanetworks/core/nas"
)

// TestSyntacticallyIncorrectOptionalIEIsAbsent pins TS 24.501 §7.7.1: an optional
// element the network cannot decode is treated as not present, the rest of the
// message decodes, and the element still re-encodes so nothing is lost.
func TestSyntacticallyIncorrectOptionalIEIsAbsent(t *testing.T) {
	// REGISTRATION ACCEPT: registration result, then a T3512 value (IEI 0x5E,
	// GPRS timer 3) whose value is two octets where the element defines one, and
	// then a well-formed negotiated DRX (IEI 0x51).
	b := []byte{
		uint8(EPD5GMM), 0x00, uint8(MsgRegistrationAccept),
		0x01, 0x01, // 5GS registration result (LV)
		ieiT3512Value, 0x02, 0x0a, 0x0b, // malformed: GPRS timer 3 is one octet
		ieiNegotiatedDRX, 0x01, 0x03,
	}

	msg, err := ParseRegistrationAccept(b)
	if err == nil {
		t.Fatal("a malformed optional element must be reported")
	}

	if !nas.SoftOnly(err) {
		t.Fatalf("a malformed optional element must be a soft error, got %v", err)
	}

	if msg == nil {
		t.Fatal("a soft error must still yield a usable message")
	}

	if msg.T3512 != nil {
		t.Errorf("the malformed element must be absent, got %+v", msg.T3512)
	}

	// The elements around it still decoded.
	if msg.NegotiatedDRX == nil || msg.NegotiatedDRX.Value != DRXCycleParameterT128 {
		t.Errorf("NegotiatedDRX = %v, want 3", msg.NegotiatedDRX)
	}

	var ie *nas.IEError
	if !errors.As(err, &ie) || ie.IEI != ieiT3512Value {
		t.Fatalf("error does not name the failed element: %v", err)
	}

	if !bytes.Equal(ie.Raw, []byte{0x0a, 0x0b}) {
		t.Errorf("IEError.Raw = % x, want the element value", ie.Raw)
	}

	// Preserved, so the message re-encodes with everything that arrived.
	again, err := msg.MarshalBinary()
	if err != nil {
		t.Fatalf("MarshalBinary: %v", err)
	}

	if !bytes.Contains(again, []byte{ieiT3512Value, 0x02, 0x0a, 0x0b}) {
		t.Errorf("re-encoding dropped the malformed element: % x", again)
	}
}

// TestSecurityCriticalIEFailureIsHard pins the exception: a UE security
// capability that will not decode fails the message outright, so no caller can
// negotiate algorithms from a silently absent capability (TS 33.501 §6.7.2).
func TestSecurityCriticalIEFailureIsHard(t *testing.T) {
	b := []byte{
		uint8(EPD5GMM), 0x00, uint8(MsgRegistrationRequest),
		0x01,                               // registration type / ngKSI
		0x00, 0x07, 0xf4, 0, 1, 2, 3, 4, 5, // 5G-S-TMSI mobile identity (LV-E)
		ieiUESecurityCapability, 0x03, 1, 2, 3, // malformed: 3 octets is not a valid length
	}

	msg, err := ParseRegistrationRequest(b)
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

// TestRepeatedIEUsesTheFirst pins TS 24.501 §7.6.3: only the first occurrence of
// a repeated element is handled, and the rest are ignored without being lost.
func TestRepeatedIEUsesTheFirst(t *testing.T) {
	b := []byte{
		uint8(EPD5GMM), 0x00, uint8(MsgRegistrationAccept),
		0x01, 0x01,
		ieiNegotiatedDRX, 0x01, 0x03,
		ieiNegotiatedDRX, 0x01, 0x04, // repetition: must be ignored
	}

	msg, err := ParseRegistrationAccept(b)
	if err != nil {
		t.Fatalf("ParseRegistrationAccept: %v", err)
	}

	if msg.NegotiatedDRX == nil || msg.NegotiatedDRX.Value != DRXCycleParameterT128 {
		t.Fatalf("NegotiatedDRX = %v, want the first occurrence (3)", msg.NegotiatedDRX)
	}

	again, err := msg.MarshalBinary()
	if err != nil {
		t.Fatalf("MarshalBinary: %v", err)
	}

	if got := bytes.Count(again, []byte{ieiNegotiatedDRX, 0x01}); got != 2 {
		t.Errorf("re-encoding kept %d occurrences, want both: % x", got, again)
	}
}
