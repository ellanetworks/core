// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package eps

import (
	"bytes"
	"errors"
	"testing"

	"github.com/ellanetworks/core/nas/common"
)

func TestSecurityProtectedRoundTrip(t *testing.T) {
	in := &SecurityProtectedMessage{
		SecurityHeaderType: SHTIntegrityProtectedCiphered,
		MAC:                [4]byte{0xde, 0xad, 0xbe, 0xef},
		SequenceNumber:     0x2a,
		Payload:            []byte{0x07, 0x41, 0x01},
	}

	out, err := ParseSecurityProtectedMessage(in.Marshal())
	if err != nil {
		t.Fatal(err)
	}

	if out.SecurityHeaderType != in.SecurityHeaderType || out.MAC != in.MAC ||
		out.SequenceNumber != in.SequenceNumber || !bytes.Equal(out.Payload, in.Payload) {
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

func TestProtectUnprotect(t *testing.T) {
	var kInt, kEnc [16]byte
	copy(kInt[:], bytes.Repeat([]byte{0x11}, 16))
	copy(kEnc[:], bytes.Repeat([]byte{0x22}, 16))

	plain := []byte{0x07, 0x42, 0x01, 0x02, 0x03, 0x04} // a stand-in plain EMM message
	count := common.NASCount(0, 0x2a)

	integ := common.AESCMACIntegrity{}
	ciph := common.AESCTRCipher{}

	t.Run("ciphered", func(t *testing.T) {
		wire, err := Protect(plain, SHTIntegrityProtectedCiphered, count, common.DirectionDownlink, kInt, kEnc, integ, ciph)
		if err != nil {
			t.Fatal(err)
		}

		m, _ := ParseSecurityProtectedMessage(wire)
		if bytes.Equal(m.Payload, plain) {
			t.Fatal("payload not ciphered on the wire")
		}

		got, err := Unprotect(wire, count, common.DirectionDownlink, kInt, kEnc, integ, ciph)
		if err != nil {
			t.Fatal(err)
		}

		if !bytes.Equal(got, plain) {
			t.Fatalf("recovered %x, want %x", got, plain)
		}
	})

	t.Run("integrity only", func(t *testing.T) {
		wire, err := Protect(plain, SHTIntegrityProtected, count, common.DirectionUplink, kInt, kEnc, integ, common.NullCipher{})
		if err != nil {
			t.Fatal(err)
		}

		m, _ := ParseSecurityProtectedMessage(wire)
		if !bytes.Equal(m.Payload, plain) {
			t.Fatal("payload should be in clear for integrity-only")
		}

		got, err := Unprotect(wire, count, common.DirectionUplink, kInt, kEnc, integ, common.NullCipher{})
		if err != nil {
			t.Fatal(err)
		}

		if !bytes.Equal(got, plain) {
			t.Fatalf("recovered %x, want %x", got, plain)
		}
	})

	t.Run("tampered MAC", func(t *testing.T) {
		wire, _ := Protect(plain, SHTIntegrityProtectedCiphered, count, common.DirectionDownlink, kInt, kEnc, integ, ciph)
		wire[1] ^= 0xff // corrupt the MAC

		if _, err := Unprotect(wire, count, common.DirectionDownlink, kInt, kEnc, integ, ciph); !errors.Is(err, ErrMACMismatch) {
			t.Fatalf("err = %v, want ErrMACMismatch", err)
		}
	})

	t.Run("tampered ciphertext", func(t *testing.T) {
		wire, _ := Protect(plain, SHTIntegrityProtectedCiphered, count, common.DirectionDownlink, kInt, kEnc, integ, ciph)
		wire[len(wire)-1] ^= 0x01 // corrupt a ciphertext octet; the MAC must catch it

		if _, err := Unprotect(wire, count, common.DirectionDownlink, kInt, kEnc, integ, ciph); !errors.Is(err, ErrMACMismatch) {
			t.Fatalf("err = %v, want ErrMACMismatch", err)
		}
	})

	t.Run("snow3g ciphered uplink", func(t *testing.T) {
		si := common.SNOW3GIntegrity{}
		sc := common.SNOW3GCipher{}

		wire, err := Protect(plain, SHTIntegrityProtectedCiphered, count, common.DirectionUplink, kInt, kEnc, si, sc)
		if err != nil {
			t.Fatal(err)
		}

		m, _ := ParseSecurityProtectedMessage(wire)
		if bytes.Equal(m.Payload, plain) {
			t.Fatal("payload not ciphered on the wire")
		}

		got, err := Unprotect(wire, count, common.DirectionUplink, kInt, kEnc, si, sc)
		if err != nil {
			t.Fatal(err)
		}

		if !bytes.Equal(got, plain) {
			t.Fatalf("recovered %x, want %x", got, plain)
		}

		wire[1] ^= 0xff // corrupt the MAC
		if _, err := Unprotect(wire, count, common.DirectionUplink, kInt, kEnc, si, sc); !errors.Is(err, ErrMACMismatch) {
			t.Fatalf("err = %v, want ErrMACMismatch", err)
		}
	})
}
