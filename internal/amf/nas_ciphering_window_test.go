// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package amf

import (
	"testing"

	"github.com/ellanetworks/core/nas/fgs"
)

func encodePlainSecurityModeReject(t *testing.T) []byte {
	t.Helper()

	m := &fgs.SecurityModeReject{Cause: fgs.GMMCauseSecurityModeRejectedUnspecified}

	payload, err := m.MarshalBinary()
	if err != nil {
		t.Fatalf("encode plain SecurityModeReject: %v", err)
	}

	return payload
}

// TS 24.501 §4.4.5 starts the AMF's ciphering as described in §4.4.2.5, which
// re-establishes the secure exchange on the AMF's ciphered reply — not on its
// receipt of the UE's initial NAS message. Until then the UE has not started
// ciphering either, so its answer to a security mode command it could not
// accept arrives unciphered and the AMF has to act on it (§5.4.2.5), not
// silently drop it and leave the UE to time out.
func TestDecodeNASMessageAcceptsAnUncipheredSecurityModeRejectBeforeCipheringStarts(t *testing.T) {
	ue := newSecuredUE(t)
	ue.SetULCountForTest(0)

	// The initial NAS message of the connection: integrity protected, unciphered.
	if _, err := DecodeNASMessage(ue, wrapIntegrityProtected(t, ue, encodePlainRegistrationRequest(t), 0)); err != nil {
		t.Fatalf("initial NAS message not accepted: %v", err)
	}

	if ue.Conn().CipheringStarted() {
		t.Fatal("the AMF counts itself as ciphering before it has replied to the UE")
	}

	if _, err := DecodeNASMessage(ue, wrapIntegrityProtected(t, ue, encodePlainSecurityModeReject(t), 1)); err != nil {
		t.Fatalf("DecodeNASMessage(unciphered SECURITY MODE REJECT) = %v, want it accepted so the AMF can abort the procedure", err)
	}
}

// The window admits the common-procedure answers and nothing else: ordinary
// signalling that should have been ciphered is still discarded.
func TestDecodeNASMessageDiscardsUncipheredOrdinarySignallingBeforeCipheringStarts(t *testing.T) {
	ue := newSecuredUE(t)
	ue.SetULCountForTest(0)

	if _, err := DecodeNASMessage(ue, wrapIntegrityProtected(t, ue, encodePlainRegistrationRequest(t), 0)); err != nil {
		t.Fatalf("initial NAS message not accepted: %v", err)
	}

	res, err := DecodeNASMessage(ue, wrapIntegrityProtected(t, ue, encodePlainULNasTransport(t), 1))
	if err == nil {
		t.Fatalf("DecodeNASMessage(unciphered UL NAS TRANSPORT) = %+v, nil, want it discarded", res)
	}
}

// Once the AMF has replied ciphered the window shuts, and even a SECURITY MODE
// REJECT has to arrive ciphered (§5.4.2.5 protects it under the rules of §4.4.5).
func TestDecodeNASMessageDiscardsAnUncipheredSecurityModeRejectAfterCipheringStarts(t *testing.T) {
	ue := newSecuredUE(t)
	ue.SetULCountForTest(0)

	if _, err := DecodeNASMessage(ue, wrapIntegrityProtected(t, ue, encodePlainRegistrationRequest(t), 0)); err != nil {
		t.Fatalf("initial NAS message not accepted: %v", err)
	}

	ue.Conn().MarkCipheringStarted()

	res, err := DecodeNASMessage(ue, wrapIntegrityProtected(t, ue, encodePlainSecurityModeReject(t), 1))
	if err == nil {
		t.Fatalf("DecodeNASMessage(unciphered SECURITY MODE REJECT) = %+v, nil, want it discarded once ciphering started", res)
	}
}
