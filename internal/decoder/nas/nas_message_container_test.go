// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"encoding/hex"
	"testing"
)

// A SECURITY MODE COMPLETE is itself ciphered, so the complete initial NAS
// message it replays (TS 24.501 §9.11.3.33) is plaintext. Bytes observed on a
// live 001/01 network.
func TestNASMessageContainerDecodesReplayedInitialMessage(t *testing.T) {
	raw, err := hex.DecodeString("7e00417900350100f11000ff0101758cae48ebd0cf8275b6d0842b6e202f0cf3721c3eee3cd9900e30f1fe3fff2a1846bdcb633d11dfbccd55e7b01001042e02f0702f02010118010074000090530101")
	if err != nil {
		t.Fatal(err)
	}

	got := nasMessageContainer(raw)

	if got == nil || got.Decoded == nil {
		t.Fatal("container did not decode")
	}

	if got.Protocol != "NAS" {
		t.Errorf("protocol = %q, want NAS", got.Protocol)
	}

	rr := got.Decoded.GmmMessage
	if rr == nil || rr.RegistrationRequest == nil {
		t.Fatal("container does not carry a registration request")
	}

	if rr.GmmHeader.MessageType.Label != "REGISTRATION REQUEST" {
		t.Errorf("message type = %q", rr.GmmHeader.MessageType.Label)
	}
}

// Ciphered contents must report as such rather than decode into a fabricated
// message.
func TestNASMessageContainerRejectsCiphertext(t *testing.T) {
	raw, err := hex.DecodeString("7e0299887766554433221100aabbccddeeff")
	if err != nil {
		t.Fatal(err)
	}

	got := nasMessageContainer(raw)

	if got.Decoded == nil || !got.Decoded.Encrypted {
		t.Fatalf("ciphered container not reported as encrypted: %+v", got.Decoded)
	}

	if got.Decoded.GmmMessage != nil {
		t.Error("ciphered container produced a decoded message")
	}
}

func TestNASMessageContainerAbsentStaysNil(t *testing.T) {
	if got := nasMessageContainer(nil); got != nil {
		t.Fatalf("expected nil for an absent container, got %+v", got)
	}
}
