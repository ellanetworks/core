// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package eps

import (
	"bytes"
	"errors"
	"testing"

	"github.com/ellanetworks/core/nas"
)

// protectWithBearer frames plain as an EPS security-protected message whose
// NAS-MAC is computed over the named bearer, standing in for a UE that MACs its
// TAU REQUEST as a 5G NAS message (bearer 3GPP) or as an EPS one (bearer EPS).
func protectWithBearer(t *testing.T, sc *nas.SecurityContext, plain []byte, count nas.Count, bearer nas.Bearer) []byte {
	t.Helper()

	seq := count.SQN()

	mac, err := sc.MAC(macInput(seq, plain), count, bearer, nas.DirectionUplink)
	if err != nil {
		t.Fatalf("MAC: %v", err)
	}

	raw, err := (&SecurityProtectedMessage{
		SecurityHeaderType: SHTIntegrityProtected,
		MAC:                mac,
		SequenceNumber:     seq,
		UnverifiedPayload:  plain,
	}).MarshalBinary()
	if err != nil {
		t.Fatalf("MarshalBinary: %v", err)
	}

	return raw
}

// TS 33.501 §8.5.2: the UE computes the NAS-MAC for the TAU REQUEST as it is
// done for a 5G NAS message over a 3GPP access, so the bearer is the 3GPP access
// connection identifier and not the EPS constant Unprotect uses.
func TestVerifyWith5GContextTakesTheThreeGPPBearer(t *testing.T) {
	sc := testContext(t, "aes")
	count := nas.Count(7)

	raw := protectWithBearer(t, sc, testPlain, count, nas.Bearer3GPP)

	plain, err := VerifyWith5GContext(raw, count, nas.DirectionUplink, sc)
	if err != nil {
		t.Fatalf("VerifyWith5GContext: %v", err)
	}

	if !bytes.Equal(plain, testPlain) {
		t.Errorf("plain = %x, want %x", plain, testPlain)
	}

	epsFramed := protectWithBearer(t, sc, testPlain, count, nas.BearerEPS)
	if _, err := VerifyWith5GContext(epsFramed, count, nas.DirectionUplink, sc); !errors.Is(err, nas.ErrMACMismatch) {
		t.Errorf("a message MAC'd over the EPS bearer verified with error %v, want a MAC mismatch", err)
	}
}

// The count is the caller's replay gate, so a message whose sequence number does
// not belong to it is refused before the MAC is even checked.
func TestVerifyWith5GContextChecksTheSequenceNumber(t *testing.T) {
	sc := testContext(t, "aes")

	raw := protectWithBearer(t, sc, testPlain, nas.Count(7), nas.Bearer3GPP)

	if _, err := VerifyWith5GContext(raw, nas.Count(8), nas.DirectionUplink, sc); !errors.Is(err, ErrSequenceNumberMismatch) {
		t.Errorf("error = %v, want a sequence number mismatch", err)
	}
}

// TS 24.301 §4.4.2.3: the message arrives unciphered so the MME can read its Old
// GUTI and UE status. A ciphered header type would hide them.
func TestVerifyWith5GContextRefusesCipheredAndPlainHeaders(t *testing.T) {
	sc := testContext(t, "aes")
	count := nas.Count(7)

	for _, sht := range []SecurityHeaderType{
		SHTIntegrityProtectedCiphered,
		SHTIntegrityProtectedCipheredNewContext,
	} {
		mac, err := sc.MAC(macInput(count.SQN(), testPlain), count, nas.Bearer3GPP, nas.DirectionUplink)
		if err != nil {
			t.Fatalf("MAC: %v", err)
		}

		raw, err := (&SecurityProtectedMessage{
			SecurityHeaderType: sht,
			MAC:                mac,
			SequenceNumber:     count.SQN(),
			UnverifiedPayload:  testPlain,
		}).MarshalBinary()
		if err != nil {
			t.Fatalf("MarshalBinary: %v", err)
		}

		if _, err := VerifyWith5GContext(raw, count, nas.DirectionUplink, sc); !errors.Is(err, ErrSecurityHeaderTypeNotPermitted) {
			t.Errorf("%s: error = %v, want the header type refused", sht, err)
		}
	}

	if _, err := VerifyWith5GContext(testPlain, count, nas.DirectionUplink, sc); err == nil {
		t.Error("a plain message verified, want a refusal")
	}
}

// TS 24.301 §4.4.2.3 case 1 has the MME run a NAS SMC when it changes algorithm,
// so the UE's next message carries the new-context header type.
func TestVerifyWith5GContextAdmitsTheNewContextHeaderType(t *testing.T) {
	sc := testContext(t, "aes")
	count := nas.Count(3)

	mac, err := sc.MAC(macInput(count.SQN(), testPlain), count, nas.Bearer3GPP, nas.DirectionUplink)
	if err != nil {
		t.Fatalf("MAC: %v", err)
	}

	raw, err := (&SecurityProtectedMessage{
		SecurityHeaderType: SHTIntegrityProtectedNewContext,
		MAC:                mac,
		SequenceNumber:     count.SQN(),
		UnverifiedPayload:  testPlain,
	}).MarshalBinary()
	if err != nil {
		t.Fatalf("MarshalBinary: %v", err)
	}

	if _, err := VerifyWith5GContext(raw, count, nas.DirectionUplink, sc); err != nil {
		t.Errorf("VerifyWith5GContext: %v", err)
	}
}

func TestVerifyWith5GContextWithoutAContext(t *testing.T) {
	if _, err := VerifyWith5GContext(testPlain, 0, nas.DirectionUplink, nil); !errors.Is(err, nas.ErrNoSecurityContext) {
		t.Errorf("error = %v, want no security context", err)
	}
}
