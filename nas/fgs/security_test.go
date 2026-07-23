// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package fgs

import (
	"bytes"
	"errors"
	"testing"

	"github.com/ellanetworks/core/nas/common"
)

var (
	testKNASint = [16]byte{0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f}
	testKNASenc = [16]byte{0x10, 0x11, 0x12, 0x13, 0x14, 0x15, 0x16, 0x17, 0x18, 0x19, 0x1a, 0x1b, 0x1c, 0x1d, 0x1e, 0x1f}
	// A representative plain 5GMM message body (Identity request + one octet).
	testPlain = []byte{EPD5GMM, 0x00, uint8(MsgIdentityRequest), 0x01}
)

func algs(name string) (common.Integrity, common.Cipher) {
	switch name {
	case "null":
		return common.NullIntegrity{}, common.NullCipher{}
	case "aes":
		return common.AESCMACIntegrity{}, common.AESCTRCipher{}
	case "snow3g":
		return common.SNOW3GIntegrity{}, common.SNOW3GCipher{}
	}

	return nil, nil
}

func TestProtectUnprotectRoundTrip(t *testing.T) {
	shts := []SecurityHeaderType{
		SHTIntegrityProtected,
		SHTIntegrityProtectedCiphered,
		SHTIntegrityProtectedNewContext,
		SHTIntegrityProtectedCipheredNewContext,
	}
	dirs := []uint8{common.DirectionUplink, common.DirectionDownlink}

	for _, name := range []string{"null", "aes", "snow3g"} {
		integ, ciph := algs(name)

		for _, sht := range shts {
			for _, dir := range dirs {
				const count uint32 = 0x0000002A

				wrapped, err := Protect(testPlain, sht, count, dir, testKNASint, testKNASenc, integ, ciph)
				if err != nil {
					t.Fatalf("%s sht=%d dir=%d Protect: %v", name, sht, dir, err)
				}

				got, err := Unprotect(wrapped, count, dir, testKNASint, testKNASenc, integ, ciph)
				if err != nil {
					t.Fatalf("%s sht=%d dir=%d Unprotect: %v", name, sht, dir, err)
				}

				if !bytes.Equal(got, testPlain) {
					t.Fatalf("%s sht=%d dir=%d round-trip = %#x, want %#x", name, sht, dir, got, testPlain)
				}
			}
		}
	}
}

func TestProtectCipherThenMAC(t *testing.T) {
	count := uint32(0x00010063) // overflow 0x0100, sqn 0x63

	integ, ciph := algs("aes")

	wrapped, err := Protect(testPlain, SHTIntegrityProtectedCiphered, count, common.DirectionDownlink,
		testKNASint, testKNASenc, integ, ciph)
	if err != nil {
		t.Fatalf("Protect: %v", err)
	}

	// Sequence number octet is sent in clear (TS 24.501 §9.1.1).
	if wrapped[6] != uint8(count) {
		t.Errorf("SN octet = %#x, want %#x", wrapped[6], uint8(count))
	}

	// The payload is ciphered, so it must differ from the plaintext body.
	if bytes.Equal(wrapped[7:], testPlain) {
		t.Error("payload was not ciphered")
	}

	// The MAC is computed over SN ‖ ciphertext, so tampering the ciphertext fails.
	tampered := bytes.Clone(wrapped)
	tampered[7] ^= 0xFF

	if _, err := Unprotect(tampered, count, common.DirectionDownlink, testKNASint, testKNASenc, integ, ciph); !errors.Is(err, ErrMACMismatch) {
		t.Errorf("tampered ciphertext: got %v, want ErrMACMismatch", err)
	}
}

func TestUnprotectRejectsWrongInputs(t *testing.T) {
	const count uint32 = 0x0000002A

	integ, ciph := algs("aes")

	wrapped, err := Protect(testPlain, SHTIntegrityProtected, count, common.DirectionUplink,
		testKNASint, testKNASenc, integ, ciph)
	if err != nil {
		t.Fatalf("Protect: %v", err)
	}

	// Wrong direction breaks the MAC (direction is an algorithm input).
	if _, err := Unprotect(wrapped, count, common.DirectionDownlink, testKNASint, testKNASenc, integ, ciph); !errors.Is(err, ErrMACMismatch) {
		t.Errorf("wrong direction: got %v, want ErrMACMismatch", err)
	}

	// Wrong count breaks the MAC.
	if _, err := Unprotect(wrapped, count+1, common.DirectionUplink, testKNASint, testKNASenc, integ, ciph); !errors.Is(err, ErrMACMismatch) {
		t.Errorf("wrong count: got %v, want ErrMACMismatch", err)
	}
}
