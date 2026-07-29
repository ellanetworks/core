// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package eps

import (
	"bytes"
	"errors"
	"testing"

	"github.com/ellanetworks/core/nas"
)

var (
	testKNASint = nas.IntegrityKey{0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f}
	testKNASenc = nas.CipherKey{0x10, 0x11, 0x12, 0x13, 0x14, 0x15, 0x16, 0x17, 0x18, 0x19, 0x1a, 0x1b, 0x1c, 0x1d, 0x1e, 0x1f}
	// A representative plain EMM message body (Identity request + one octet).
	testPlain = []byte{uint8(PDEMM), uint8(MsgIdentityRequest), 0x01}
)

// testContext builds a security context over the named algorithm pair.
func testContext(t *testing.T, name string) *nas.SecurityContext {
	t.Helper()

	opts := nas.SecurityContextOptions{IntegrityKey: testKNASint, CipherKey: testKNASenc}

	switch name {
	case "null":
		opts.Integrity, opts.Ciphering, opts.AllowNullIntegrity = nas.IntegrityNull, nas.CipheringNull, true
	case "aes":
		opts.Integrity, opts.Ciphering = nas.IntegrityAES, nas.CipheringAES
	case "snow3g":
		opts.Integrity, opts.Ciphering = nas.IntegritySNOW3G, nas.CipheringSNOW3G
	default:
		t.Fatalf("unknown algorithm pair %q", name)
	}

	sc, err := nas.NewSecurityContext(opts)
	if err != nil {
		t.Fatalf("NewSecurityContext(%s): %v", name, err)
	}

	return sc
}

func TestSecurityProtectedRoundTrip(t *testing.T) {
	in := &SecurityProtectedMessage{
		SecurityHeaderType: SHTIntegrityProtectedCiphered,
		MAC:                [4]byte{0xde, 0xad, 0xbe, 0xef},
		SequenceNumber:     0x2a,
		UnverifiedPayload:  []byte{0x07, 0x41, 0x01},
	}

	out, err := ParseSecurityProtectedMessage(mustBytes(in.MarshalBinary()))
	if err != nil {
		t.Fatal(err)
	}

	if out.SecurityHeaderType != in.SecurityHeaderType || out.MAC != in.MAC ||
		out.SequenceNumber != in.SequenceNumber || !bytes.Equal(out.UnverifiedPayload, in.UnverifiedPayload) {
		t.Fatalf("round-trip mismatch:\n  in  %+v\n  out %+v", in, out)
	}
}

func TestParseSecurityProtectedErrors(t *testing.T) {
	if _, err := ParseSecurityProtectedMessage([]byte{0x02, 0, 0, 0, 0, 0}); !errors.Is(err, ErrNotEMM) {
		t.Fatalf("ESM PD: err = %v, want ErrNotEMM", err)
	}

	if _, err := ParseSecurityProtectedMessage([]byte{0x07, 0x41}); !errors.Is(err, ErrNotProtected) {
		t.Fatalf("SHT 0: err = %v, want ErrNotProtected", err)
	}
}

func TestPeekSecurityHeaderType(t *testing.T) {
	if sht, err := PeekSecurityHeaderType([]byte{uint8(PDEMM), 0x00}); err != nil || sht != SHTPlain {
		t.Errorf("plain: sht=%d err=%v", sht, err)
	}

	if sht, err := PeekSecurityHeaderType([]byte{uint8(SHTIntegrityProtectedCiphered)<<4 | uint8(PDEMM)}); err != nil || sht != SHTIntegrityProtectedCiphered {
		t.Errorf("ciphered: sht=%d err=%v", sht, err)
	}

	if _, err := PeekSecurityHeaderType(nil); err == nil {
		t.Error("empty: want error")
	}
}

func TestPeekProtocolDiscriminator(t *testing.T) {
	if pd, _ := PeekProtocolDiscriminator([]byte{uint8(PDEMM), 0x41}); pd != PDEMM {
		t.Errorf("EMM PD = %#x, want %#x", uint8(pd), uint8(PDEMM))
	}

	if pd, _ := PeekProtocolDiscriminator([]byte{0x50 | uint8(PDESM), 0x01, 0xC1}); pd != PDESM {
		t.Errorf("ESM PD = %#x, want %#x", uint8(pd), uint8(PDESM))
	}

	if _, err := PeekProtocolDiscriminator(nil); err == nil {
		t.Error("empty: want error")
	}
}

func TestProtectUnprotectRoundTrip(t *testing.T) {
	shts := []SecurityHeaderType{
		SHTIntegrityProtected,
		SHTIntegrityProtectedCiphered,
		SHTIntegrityProtectedNewContext,
		SHTIntegrityProtectedCipheredNewContext,
	}
	dirs := []nas.Direction{nas.DirectionUplink, nas.DirectionDownlink}
	count := nas.MakeCount(0, 0x2A)

	for _, name := range []string{"null", "aes", "snow3g"} {
		sc := testContext(t, name)

		for _, sht := range shts {
			for _, dir := range dirs {
				wrapped, err := Protect(testPlain, sht, count, dir, sc)
				if err != nil {
					t.Fatalf("%s sht=%s dir=%s Protect: %v", name, sht, dir, err)
				}

				got, gotSHT, err := Unprotect(wrapped, count, dir, sc)
				if err != nil {
					t.Fatalf("%s sht=%s dir=%s Unprotect: %v", name, sht, dir, err)
				}

				if gotSHT != sht {
					t.Errorf("%s dir=%s: verified under %s, want %s", name, dir, gotSHT, sht)
				}

				if !bytes.Equal(got, testPlain) {
					t.Fatalf("%s sht=%s dir=%s round-trip = %#x, want %#x", name, sht, dir, got, testPlain)
				}
			}
		}
	}
}

func TestProtectCipherThenMAC(t *testing.T) {
	count := nas.MakeCount(0x0100, 0x63)
	sc := testContext(t, "aes")

	wrapped, err := Protect(testPlain, SHTIntegrityProtectedCiphered, count, nas.DirectionDownlink, sc)
	if err != nil {
		t.Fatalf("Protect: %v", err)
	}

	// Sequence number octet is sent in clear (TS 24.301 §9.3.1).
	if wrapped[5] != count.SQN() {
		t.Errorf("SN octet = %#x, want %#x", wrapped[5], count.SQN())
	}

	// The payload is ciphered, so it must differ from the plaintext body.
	if bytes.Equal(wrapped[6:], testPlain) {
		t.Error("payload was not ciphered")
	}

	// The MAC is computed over SN ‖ ciphertext, so tampering the ciphertext fails.
	tampered := bytes.Clone(wrapped)
	tampered[6] ^= 0xFF

	if _, _, err := Unprotect(tampered, count, nas.DirectionDownlink, sc); !errors.Is(err, nas.ErrMACMismatch) {
		t.Errorf("tampered ciphertext: got %v, want ErrMACMismatch", err)
	}
}

func TestUnprotectRejectsWrongInputs(t *testing.T) {
	count := nas.MakeCount(0, 0x2A)
	sc := testContext(t, "aes")

	wrapped, err := Protect(testPlain, SHTIntegrityProtected, count, nas.DirectionUplink, sc)
	if err != nil {
		t.Fatalf("Protect: %v", err)
	}

	// Wrong direction breaks the MAC (direction is an algorithm input).
	if _, _, err := Unprotect(wrapped, count, nas.DirectionDownlink, sc); !errors.Is(err, nas.ErrMACMismatch) {
		t.Errorf("wrong direction: got %v, want ErrMACMismatch", err)
	}

	// A count whose sequence number is not the one on the wire is refused before
	// the MAC is computed, which is what catches it under null integrity.
	if _, _, err := Unprotect(wrapped, count.Next(), nas.DirectionUplink, sc); !errors.Is(err, nas.ErrSequenceNumberMismatch) {
		t.Errorf("wrong sequence number: got %v, want ErrSequenceNumberMismatch", err)
	}

	// A count that differs only above the sequence number breaks the MAC.
	if _, _, err := Unprotect(wrapped, nas.MakeCount(1, count.SQN()), nas.DirectionUplink, sc); !errors.Is(err, nas.ErrMACMismatch) {
		t.Errorf("wrong overflow: got %v, want ErrMACMismatch", err)
	}

	if _, _, err := Unprotect(wrapped, count, nas.DirectionUplink, nil); !errors.Is(err, nas.ErrNoSecurityContext) {
		t.Errorf("no context: got %v, want ErrNoSecurityContext", err)
	}
}

// TestUnprotectPermittedTypes checks that a caller can pin the security header
// types it accepts, which is what keeps a new-context type out of a procedure
// that is not the security mode one (TS 33.401 §7.2.4.4).
func TestUnprotectPermittedTypes(t *testing.T) {
	count := nas.MakeCount(0, 0x2A)
	sc := testContext(t, "aes")

	wrapped, err := Protect(testPlain, SHTIntegrityProtectedCipheredNewContext, count, nas.DirectionUplink, sc)
	if err != nil {
		t.Fatal(err)
	}

	_, sht, err := Unprotect(wrapped, count, nas.DirectionUplink, sc, SHTIntegrityProtected, SHTIntegrityProtectedCiphered)
	if !errors.Is(err, nas.ErrSecurityHeaderTypeNotPermitted) {
		t.Errorf("err = %v, want ErrSecurityHeaderTypeNotPermitted", err)
	}

	if sht != SHTIntegrityProtectedCipheredNewContext {
		t.Errorf("refused under %s, want the type it carried reported", sht)
	}

	if _, _, err := Unprotect(wrapped, count, nas.DirectionUplink, sc, SHTIntegrityProtectedCipheredNewContext); err != nil {
		t.Errorf("permitted type: %v", err)
	}
}

// TestProtectRejectsUnprotectedTypes checks that Protect refuses to build a
// wrapper that names no protection.
func TestProtectRejectsUnprotectedTypes(t *testing.T) {
	count := nas.MakeCount(0, 1)
	sc := testContext(t, "aes")

	if _, err := Protect(testPlain, SHTPlain, count, nas.DirectionDownlink, sc); !errors.Is(err, nas.ErrNotProtected) {
		t.Errorf("plain: err = %v, want ErrNotProtected", err)
	}

	if _, err := Protect(testPlain, SecurityHeaderType(9), count, nas.DirectionDownlink, sc); err == nil {
		t.Error("a reserved security header type encoded")
	}

	if _, err := Protect(testPlain, SHTIntegrityProtected, count, nas.DirectionDownlink, nil); !errors.Is(err, nas.ErrNoSecurityContext) {
		t.Error("a nil security context protected a message")
	}
}

// TestParseSecurityProtectedRejectsReserved checks that a wrapper naming a
// reserved security header type is refused rather than verified under a guess
// (TS 24.301 table 9.3.1).
func TestParseSecurityProtectedRejectsReserved(t *testing.T) {
	// 5 is defined (partially ciphered) and 12 is the SERVICE REQUEST, which is
	// framed differently; both are covered elsewhere.
	for _, sht := range []uint8{6, 9, 15} {
		b := []byte{sht<<4 | uint8(PDEMM), 0, 0, 0, 0, 0x2A, 0x07}
		if _, err := ParseSecurityProtectedMessage(b); err == nil {
			t.Errorf("security header type %d parsed as a wrapper", sht)
		}
	}
}
