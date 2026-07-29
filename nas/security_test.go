// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

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

	mac, _ := nullIntegrity{}.MAC(key, 7, 0, DirectionUplink, []byte("x"))
	if mac != [4]byte{} {
		t.Fatalf("nullIntegrity MAC = %x, want zero", mac)
	}

	data := []byte("y")

	out, _ := nullCipher{}.Apply(key, 7, 0, DirectionUplink, data)
	if !bytes.Equal(out, data) {
		t.Fatalf("nullCipher changed data: %x", out)
	}
}
