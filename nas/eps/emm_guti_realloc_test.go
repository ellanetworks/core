// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package eps

import (
	"reflect"
	"testing"

	"github.com/ellanetworks/core/nas"
)

func TestGUTIReallocationCommandRoundTrip(t *testing.T) {
	guti := GUTIIdentity(GUTI{PLMN: nas.PLMN{MCC: "001", MNC: "01"}, MMEGroupID: 1, MMECode: 1, TMSI: [4]byte{0x01, 0x02, 0x03, 0x04}})

	b, err := (&GUTIReallocationCommand{GUTI: guti}).MarshalBinary()
	if err != nil {
		t.Fatalf("MarshalBinary: %v", err)
	}

	if MessageType(b[1]) != MsgGUTIReallocationCommand {
		t.Fatalf("message type = %#x, want %#x", b[1], MsgGUTIReallocationCommand)
	}

	got, err := ParseGUTIReallocationCommand(b)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	if !reflect.DeepEqual(got.GUTI, guti) {
		t.Fatalf("GUTI = %+v, want %+v", got.GUTI, guti)
	}
}

func TestGUTIReallocationCompleteRoundTrip(t *testing.T) {
	b, err := (&GUTIReallocationComplete{}).MarshalBinary()
	if err != nil {
		t.Fatalf("MarshalBinary: %v", err)
	}

	if MessageType(b[1]) != MsgGUTIReallocationComplete {
		t.Fatalf("message type = %#x, want %#x", b[1], MsgGUTIReallocationComplete)
	}

	if _, err := ParseGUTIReallocationComplete(b); err != nil {
		t.Fatalf("Parse: %v", err)
	}
}
