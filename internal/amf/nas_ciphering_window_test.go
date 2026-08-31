// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package amf

import (
	"testing"

	"github.com/ellanetworks/core/nas"
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

func encodePlainSecurityModeComplete(t *testing.T) []byte {
	t.Helper()

	payload, err := (&fgs.SecurityModeComplete{}).MarshalBinary()
	if err != nil {
		t.Fatalf("encode plain SecurityModeComplete: %v", err)
	}

	return payload
}

// TS 24.501 §4.4.2.5, §4.4.5
func TestDecodeNASMessageAcceptsUncipheredOrdinarySignallingBeforeCipheringStarts(t *testing.T) {
	ue := newSecuredUE(t)
	ue.SetULCountForTest(0)

	if _, err := DecodeNASMessage(ue, wrapIntegrityProtected(t, ue, encodePlainRegistrationRequest(t), 0)); err != nil {
		t.Fatalf("initial NAS message not accepted: %v", err)
	}

	if ue.Conn().CipheringStarted() {
		t.Fatal("the AMF counts itself as ciphering before it has replied to the UE")
	}

	if _, err := DecodeNASMessage(ue, wrapIntegrityProtected(t, ue, encodePlainULNasTransport(t), 1)); err != nil {
		t.Fatalf("DecodeNASMessage(unciphered UL NAS TRANSPORT) = %v, want it accepted before the AMF has replied ciphered", err)
	}
}

// TS 24.501 §4.4.5
func TestDecodeNASMessageDiscardsUncipheredOrdinarySignallingAfterCipheringStarts(t *testing.T) {
	ue := newSecuredUE(t)
	ue.SetULCountForTest(0)

	if _, err := DecodeNASMessage(ue, wrapIntegrityProtected(t, ue, encodePlainRegistrationRequest(t), 0)); err != nil {
		t.Fatalf("initial NAS message not accepted: %v", err)
	}

	ue.Conn().MarkCipheringStarted()

	res, err := DecodeNASMessage(ue, wrapIntegrityProtected(t, ue, encodePlainULNasTransport(t), 1))
	if err == nil {
		t.Fatalf("DecodeNASMessage(unciphered UL NAS TRANSPORT) = %+v, nil, want it discarded once ciphering started", res)
	}
}

// TS 24.501 §4.4.2.5, §4.4.5, §5.4.2.5
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

// TS 24.501 §5.4.2.3
func TestDecodeNASMessageAcceptsASecurityModeCompleteWithTheNewContextHeaderType(t *testing.T) {
	ue := newSecuredUE(t)
	ue.SetULCountForTest(0)
	ue.ForceRegStepForTest(RegStepSecurityMode)

	cnt, err := ue.ulCount.Estimate(0)
	if err != nil {
		t.Fatal(err)
	}

	wire, err := fgs.Protect(encodePlainSecurityModeComplete(t), fgs.SHTIntegrityProtectedCipheredNewContext, cnt, nas.DirectionUplink, ue.sc)
	if err != nil {
		t.Fatalf("protect SECURITY MODE COMPLETE: %v", err)
	}

	if _, err := DecodeNASMessage(ue, wire); err != nil {
		t.Fatalf("DecodeNASMessage(SECURITY MODE COMPLETE, new-context header type) = %v, want it accepted", err)
	}

	if !ue.Conn().CipheringStarted() {
		t.Error("a ciphered SECURITY MODE COMPLETE did not start ciphering on the connection")
	}
}

// TS 24.501 §5.4.2.3
func TestDecodeNASMessageDiscardsASecurityModeCompleteWithoutTheNewContextHeaderType(t *testing.T) {
	ue := newSecuredUE(t)
	ue.SetULCountForTest(0)

	if _, err := DecodeNASMessage(ue, wrapIntegrityProtected(t, ue, encodePlainRegistrationRequest(t), 0)); err != nil {
		t.Fatalf("initial NAS message not accepted: %v", err)
	}

	if ue.Conn().CipheringStarted() {
		t.Fatal("the AMF counts itself as ciphering before it has replied to the UE")
	}

	res, err := DecodeNASMessage(ue, wrapIntegrityProtected(t, ue, encodePlainSecurityModeComplete(t), 1))
	if err == nil {
		t.Fatalf("DecodeNASMessage(unciphered SECURITY MODE COMPLETE) = %+v, nil, want it discarded: it would carry the replayed initial NAS message in the clear", res)
	}
}
