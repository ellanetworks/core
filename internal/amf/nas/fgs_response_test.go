// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"testing"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/nas/fgs"
)

// assertPlainGmm asserts pdu is a plain 5GMM message of the given type. The plain
// wire bytes double as the plaintext, so callers parse pdu directly with fgs.ParseX.
func assertPlainGmm(t *testing.T, pdu []byte, wantType uint8) {
	t.Helper()

	if len(pdu) < 3 {
		t.Fatalf("NAS PDU too short: %d bytes", len(pdu))
	}

	if fgs.SecurityHeaderType(pdu[1]&0x0f) != fgs.SHTPlain {
		t.Fatalf("expected plain NAS, got security header type %d", pdu[1]&0x0f)
	}

	if pdu[2] != wantType {
		t.Fatalf("expected GMM message type %#x, got %#x", wantType, pdu[2])
	}
}

// assertPlainDLTransport asserts pdu is a plain DL NAS TRANSPORT message and
// returns the parsed message for further field inspection.
func assertPlainDLTransport(t *testing.T, pdu []byte) *fgs.DLNASTransport {
	t.Helper()

	assertPlainGmm(t, pdu, uint8(fgs.MsgDLNASTransport))

	dl, err := fgs.ParseDLNASTransport(pdu)
	if err != nil {
		t.Fatalf("could not parse DL NAS transport: %v", err)
	}

	return dl
}

// psiSet reports whether PSI n (0..15) is set in a 2-octet PSI bitmap value
// (PDU session status / reactivation result; TS 24.501 §9.11.3.44).
func psiSet(b *fgs.PSIBitmap, n int) bool {
	if b == nil || n < 0 || n >= len(b.PSI) {
		return false
	}

	return b.PSI[n]
}

// decipherGmm asserts pdu is an integrity-protected-and-ciphered downlink NAS
// message of wantType (deciphered against the UE's security context) and returns
// the plaintext.
func decipherGmm(t *testing.T, ue *amf.UeContext, pdu []byte, wantType uint8) []byte {
	t.Helper()

	return decipherGmmCount(t, ue, pdu, ue.ULCount(), wantType)
}

// decipherGmmCount is decipherGmm with an explicit NAS count, for downlinks whose
// sequence number has advanced past the UE's uplink count.
func decipherGmmCount(t *testing.T, ue *amf.UeContext, pdu []byte, count uint32, wantType uint8) []byte {
	t.Helper()

	if len(pdu) < 7 || fgs.SecurityHeaderType(pdu[1]&0x0f) != fgs.SHTIntegrityProtectedCiphered {
		t.Fatalf("expected a protected and ciphered NAS message, got sht %d", pdu[1]&0x0f)
	}

	sc := mustSecurityContext(t, ue.IntegrityAlgForTest(),
		ue.CipheringAlgForTest(), ue.KnasIntForTest(), ue.KnasEncForTest())

	plain, err := sc.Cipher(append([]byte(nil), pdu[7:]...), nas.Count(count), nas.Bearer3GPP, nas.DirectionDownlink)
	if err != nil {
		t.Fatalf("could not decrypt NAS message: %v", err)
	}

	if len(plain) < 3 || plain[2] != wantType {
		t.Fatalf("expected GMM message type %#x, got % x", wantType, plain)
	}

	return plain
}
