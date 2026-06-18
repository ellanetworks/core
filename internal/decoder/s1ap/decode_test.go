// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"encoding/hex"
	"testing"
)

// The example inputs in the per-message tests are raw S1AP PDUs captured from a
// running Ella Core deployment on the 999/01 test PLMN.

func decodeHex(t *testing.T, h string) S1APMessage {
	t.Helper()

	raw, err := hex.DecodeString(h)
	if err != nil {
		t.Fatal(err)
	}

	msg := DecodeS1APMessage(raw)
	if msg.Value.Error != "" {
		t.Fatalf("decode error: %s", msg.Value.Error)
	}

	return msg
}

func findIE(ies []IE, id int64) (IE, bool) {
	for _, ie := range ies {
		if ie.ID.Value == id {
			return ie, true
		}
	}

	return IE{}, false
}

func mustIE(t *testing.T, msg S1APMessage, id int64) IE {
	t.Helper()

	ie, ok := findIE(msg.Value.IEs, id)
	if !ok {
		t.Fatalf("IE %d (%s) missing", id, ieNames[id])
	}

	return ie
}

func TestDecodeS1APInvalid(t *testing.T) {
	msg := DecodeS1APMessage([]byte{0xff, 0x00, 0x01})
	if msg.Value.Error == "" {
		t.Fatal("expected a decode error for malformed input")
	}
}
