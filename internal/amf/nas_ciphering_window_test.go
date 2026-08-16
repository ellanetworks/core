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

// TS 24.501 §4.4.5, §4.4.2.5, §5.4.2.5
func TestDecodeNASMessageAcceptsAnUncipheredSecurityModeRejectBeforeCipheringStarts(t *testing.T) {
	ue := newSecuredUE(t)
	ue.SetULCountForTest(0)

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

// TS 24.501 §4.4.5
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

// TS 24.501 §4.4.5, §5.4.2.5
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
