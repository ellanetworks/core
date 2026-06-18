// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package common

import (
	"bytes"
	"encoding/hex"
	"testing"
)

func mustHex(t *testing.T, s string) []byte {
	t.Helper()

	b, err := hex.DecodeString(s)
	if err != nil {
		t.Fatalf("bad hex %q: %v", s, err)
	}

	return b
}

func TestNullAlgorithms(t *testing.T) {
	var key [16]byte

	mac, _ := NullIntegrity{}.MAC(key, 7, 0, DirectionUplink, []byte("x"))
	if mac != [4]byte{} {
		t.Fatalf("NullIntegrity MAC = %x, want zero", mac)
	}

	data := []byte("y")

	out, _ := NullCipher{}.Apply(key, 7, 0, DirectionUplink, data)
	if !bytes.Equal(out, data) {
		t.Fatalf("NullCipher changed data: %x", out)
	}
}

func TestNASCount(t *testing.T) {
	if got := NASCount(0x0102, 0x03); got != 0x00010203 {
		t.Fatalf("NASCount = %#x, want 0x00010203", got)
	}
}
