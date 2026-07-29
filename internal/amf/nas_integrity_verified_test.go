// SPDX-FileCopyrightText: Ella Networks Inc.
//
// SPDX-License-Identifier: BUSL-1.1

package amf

import (
	"testing"

	"github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/nas/fgs"
)

// wrapIntegrityProtected wraps a plain inner NAS message in a security-protected
// header (EPD, security header type, 4-byte MAC, sequence number) with a MAC
// computed over [sqn || inner] against the UE's current security context, with
// the sequence number folded into the count exactly as decodeProtectedNAS does
// (TS 33.501).
func wrapIntegrityProtected(t *testing.T, ue *UeContext, inner []byte, sqn uint8) []byte {
	t.Helper()

	cnt, _ := ue.ulCount.Estimate(sqn)

	pdu, err := fgs.Protect(inner, fgs.SHTIntegrityProtected, cnt, nas.DirectionUplink, ue.sc)
	if err != nil {
		t.Fatalf("protect NAS: %v", err)
	}

	return pdu
}

func newSecuredUE(t *testing.T) *UeContext {
	t.Helper()

	ue := newDecoderTestUE(t)
	ue.integrityAlg = nas.IntegrityAES
	ue.knasInt = [16]uint8{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}

	if err := ue.installSecurityContextLocked(); err != nil {
		t.Fatalf("install security context: %v", err)
	}

	return ue
}

func encodePlainEmergencyRegistration(t *testing.T) []byte {
	t.Helper()

	m := &fgs.RegistrationRequest{
		RegistrationType:     fgs.RegistrationTypeEmergency,
		FOR:                  true,
		MobileIdentity:       testMobileIdentity(),
		UESecurityCapability: &fgs.UESecurityCapability{EA: 0xe0, IA: 0xe0},
	}

	payload, err := m.MarshalBinary()
	if err != nil {
		t.Fatalf("encode plain emergency RegistrationRequest: %v", err)
	}

	return payload
}

func TestNasIntegrityVerified_GenuineMessageVerifies(t *testing.T) {
	ue := newSecuredUE(t)
	pdu := wrapIntegrityProtected(t, ue, encodePlainRegistrationRequest(t), 0)

	if !ue.NasIntegrityVerified(pdu) {
		t.Fatal("a correctly MAC'd message against the UE context must verify")
	}
}

func TestNasIntegrityVerified_TamperedMacRejected(t *testing.T) {
	ue := newSecuredUE(t)
	pdu := wrapIntegrityProtected(t, ue, encodePlainRegistrationRequest(t), 0)
	pdu[3] ^= 0xff // flip a MAC byte

	if ue.NasIntegrityVerified(pdu) {
		t.Fatal("a tampered MAC must not verify")
	}
}

func TestNasIntegrityVerified_PlainMessageRejected(t *testing.T) {
	ue := newSecuredUE(t)

	if ue.NasIntegrityVerified(encodePlainRegistrationRequest(t)) {
		t.Fatal("a plain NAS message proves nothing about the context and must not verify")
	}
}

func TestNasIntegrityVerified_NoSecurityContextRejected(t *testing.T) {
	ue := newSecuredUE(t)
	pdu := wrapIntegrityProtected(t, ue, encodePlainRegistrationRequest(t), 0)
	ue.secured = false

	if ue.NasIntegrityVerified(pdu) {
		t.Fatal("without an available security context nothing can verify")
	}
}

// TestNasIntegrityVerified_DoesNotMutateCount asserts the check is read-only:
// the uplink count must be untouched so a hostile message cannot advance it.
func TestNasIntegrityVerified_DoesNotMutateCount(t *testing.T) {
	ue := newSecuredUE(t)
	pdu := wrapIntegrityProtected(t, ue, encodePlainRegistrationRequest(t), 0)

	before := ue.ulCount
	_ = ue.NasIntegrityVerified(pdu)

	if ue.ulCount != before {
		t.Fatalf("ULCount must not change: before=%d after=%d", before.NextExpected(), ue.ulCount.NextExpected())
	}
}

// TestReuseForInboundNAS_PlainInitialRegistrationDiverted is the core
// regression for the GUTI-spoof DoS: a plain (unauthenticated) initial
// registration that resolved to an existing context must NOT reuse it
// (TS 24.501).
func TestReuseForInboundNAS_PlainInitialRegistrationDiverted(t *testing.T) {
	ue := newSecuredUE(t)

	if ue.ReuseForInboundNAS(encodePlainRegistrationRequest(t)) {
		t.Fatal("a plain initial registration must not reuse the committed context")
	}
}

// TestReuseForInboundNAS_IntegrityVerifiedRegistrationReuses confirms a genuine
// integrity-protected registration still reuses the context (no forced re-auth).
func TestReuseForInboundNAS_IntegrityVerifiedRegistrationReuses(t *testing.T) {
	ue := newSecuredUE(t)
	pdu := wrapIntegrityProtected(t, ue, encodePlainRegistrationRequest(t), 0)

	if !ue.ReuseForInboundNAS(pdu) {
		t.Fatal("an integrity-verified registration must reuse the committed context")
	}
}

// TestReuseForInboundNAS_UnverifiedNonEmergencyDiverted confirms the gate
// diverts every unverified message that resolved to a committed context —
// service request and deregistration included — so none can act on it. Each is
// handled correctly on a fresh context (TS 24.501).
func TestReuseForInboundNAS_UnverifiedNonEmergencyDiverted(t *testing.T) {
	ue := newSecuredUE(t)

	if ue.ReuseForInboundNAS(encodePlainServiceRequest(t)) {
		t.Fatal("an unverified service request must not reuse the committed context")
	}

	if ue.ReuseForInboundNAS(encodePlainDeregistrationRequest(t)) {
		t.Fatal("an unverified deregistration must not reuse the committed context")
	}
}

// TestReuseForInboundNAS_PlainEmergencyDiverted confirms context resolution is
// uniform: even a plain emergency registration does not reuse a committed
// context — it is processed on a fresh one, so the committed context is never
// mutated by an unverified message (TS 24.501).
func TestReuseForInboundNAS_PlainEmergencyDiverted(t *testing.T) {
	ue := newSecuredUE(t)

	if ue.ReuseForInboundNAS(encodePlainEmergencyRegistration(t)) {
		t.Fatal("a plain emergency registration must not reuse the committed context")
	}
}

// TestDecodeNASMessage_MacFailedDoesNotAdvanceULCount asserts that a message
// failing the integrity check does not advance the committed uplink count (so an
// attacker cannot desync a genuine UE), while a verified message does.
func TestDecodeNASMessage_MacFailedDoesNotAdvanceULCount(t *testing.T) {
	ue := newSecuredUE(t)

	pdu := wrapIntegrityProtected(t, ue, encodePlainRegistrationRequest(t), 7)

	bad := append([]byte(nil), pdu...)
	bad[3] ^= 0xff // corrupt the MAC

	before := ue.ulCount

	if _, err := DecodeNASMessage(ue, bad); err != nil {
		t.Fatalf("a mac-failed registration request must be admitted (on the pre-secure-exchange whitelist): %v", err)
	}

	if ue.ulCount != before {
		t.Fatalf("a mac-failed message must not advance ULCount: before=%d after=%d", before.NextExpected(), ue.ulCount.NextExpected())
	}

	if _, err := DecodeNASMessage(ue, pdu); err != nil {
		t.Fatalf("a verified registration request must decode: %v", err)
	}

	if ue.ulCount.LastAccepted().SQN() != 7 {
		t.Fatalf("a verified message must accept sqn=7, got %d", ue.ulCount.LastAccepted().SQN())
	}
}

// TestDecodeNASMessage_SecureExchangeEstablished_DiscardsPlain asserts TS 24.501:
// plain NAS is admitted while bootstrapping, a verified message
// establishes secure exchange for the connection, and after that a plain
// message is discarded.
func TestDecodeNASMessage_SecureExchangeEstablished_DiscardsPlain(t *testing.T) {
	ue := newSecuredUE(t)

	if _, err := DecodeNASMessage(ue, encodePlainRegistrationRequest(t)); err != nil {
		t.Fatalf("plain registration must be admitted before secure exchange: %v", err)
	}

	if ue.Conn().SecureExchangeEstablished() {
		t.Fatal("a plain message must not establish secure exchange")
	}

	if _, err := DecodeNASMessage(ue, wrapIntegrityProtected(t, ue, encodePlainRegistrationRequest(t), 0)); err != nil {
		t.Fatalf("a verified message must be admitted: %v", err)
	}

	if !ue.Conn().SecureExchangeEstablished() {
		t.Fatal("a verified message must establish secure exchange (TS 24.501)")
	}

	if _, err := DecodeNASMessage(ue, encodePlainRegistrationRequest(t)); err == nil {
		t.Fatal("a plain message must be discarded once secure exchange is established (TS 24.501)")
	}
}

// TestDecodeNASMessage_SecureExchangeEstablished_DiscardsMacFailed asserts that
// once secure exchange is established, a message failing the integrity check is
// discarded rather than admitted as mac-failed (TS 24.501).
func TestDecodeNASMessage_SecureExchangeEstablished_DiscardsMacFailed(t *testing.T) {
	ue := newSecuredUE(t)

	if _, err := DecodeNASMessage(ue, wrapIntegrityProtected(t, ue, encodePlainServiceRequest(t), 3)); err != nil {
		t.Fatalf("a verified service request must be admitted: %v", err)
	}

	if !ue.Conn().SecureExchangeEstablished() {
		t.Fatal("a verified message must establish secure exchange")
	}

	bad := wrapIntegrityProtected(t, ue, encodePlainServiceRequest(t), 4)
	bad[3] ^= 0xff // corrupt the MAC

	if _, err := DecodeNASMessage(ue, bad); err == nil {
		t.Fatal("a mac-failed message must be discarded once secure exchange is established (TS 24.501)")
	}
}

// TestDecodeProtectedNAS_NewContextOutsideSecurityMode pins the window in which
// the new-context security header type is accepted. TS 24.501 §4.4.4.3 reserves
// it for the SECURITY MODE COMPLETE answering a command in flight; accepting it
// at any other time let an attacker replay that captured message under unchanged
// keys, roll the uplink NAS COUNT back to zero, and replay everything captured
// after it.
//
// It decodes through DecodeNASMessage rather than the unexported handler, so the
// guard runs under the lock DecodeNASMessage holds, as it does in production.
func TestDecodeProtectedNAS_NewContextOutsideSecurityMode(t *testing.T) {
	ue := newSecuredUE(t)

	inner, err := (&fgs.SecurityModeComplete{}).MarshalBinary()
	if err != nil {
		t.Fatalf("encode SECURITY MODE COMPLETE: %v", err)
	}

	cnt, err := ue.ulCount.Estimate(0)
	if err != nil {
		t.Fatal(err)
	}

	wire, err := fgs.Protect(inner, fgs.SHTIntegrityProtectedCipheredNewContext, cnt, nas.DirectionUplink, ue.sc)
	if err != nil {
		t.Fatalf("protect SECURITY MODE COMPLETE: %v", err)
	}

	// Registered: the security mode procedure has long finished.
	ue.ForceRegStepForTest(RegStepContextSetup)

	if _, err := DecodeNASMessage(ue, wire); err == nil {
		t.Fatal("a new-context message outside the security mode procedure was accepted")
	}

	// In the security mode procedure it is the expected answer.
	ue.ForceRegStepForTest(RegStepSecurityMode)

	if _, err := DecodeNASMessage(ue, wire); err != nil {
		t.Fatalf("the SECURITY MODE COMPLETE answering a command in flight was refused: %v", err)
	}
}
