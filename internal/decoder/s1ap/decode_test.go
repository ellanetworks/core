// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"encoding/hex"
	"testing"

	"github.com/ellanetworks/core/s1ap"
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

func findIE(ies []IE, id s1ap.ProtocolIEID) (IE, bool) {
	for _, ie := range ies {
		if ie.ID.Value == int64(id) {
			return ie, true
		}
	}

	return IE{}, false
}

func mustIE(t *testing.T, msg S1APMessage, id s1ap.ProtocolIEID) IE {
	t.Helper()

	ie, ok := findIE(msg.Value.IEs, id)
	if !ok {
		name, _ := s1ap.ProtocolIEIDName(id)
		t.Fatalf("IE %d (%s) missing", id, name)
	}

	return ie
}

func TestDecodeS1APInvalid(t *testing.T) {
	msg := DecodeS1APMessage([]byte{0xff, 0x00, 0x01})
	if msg.Value.Error == "" {
		t.Fatal("expected a decode error for malformed input")
	}
}

// The rendered label is the codec's, for every id and procedure code, so a
// local table reintroduced anywhere in the render path fails here.
func TestLabelsComeFromTheCodec(t *testing.T) {
	for id := 0; id <= 0xFFFF; id++ {
		name, known := s1ap.ProtocolIEIDName(s1ap.ProtocolIEID(id))

		got := ieEnum(s1ap.ProtocolIEID(id))
		if got.Label != name || got.Unknown == known {
			t.Fatalf("IE id %d renders %+v, want label %q known %v", id, got, name, known)
		}
	}

	for code := 0; code <= 0xFF; code++ {
		name, known := s1ap.ProcedureCodeName(s1ap.ProcedureCode(code))

		got := procedureCodeToEnum(s1ap.ProcedureCode(code))
		if got.Label != name || got.Unknown == known {
			t.Fatalf("procedure code %d renders %+v, want label %q known %v", code, got, name, known)
		}
	}
}
