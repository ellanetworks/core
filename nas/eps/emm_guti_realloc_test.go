// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package eps

import "testing"

func TestGUTIReallocationCommandRoundTrip(t *testing.T) {
	guti := EPSMobileIdentity{Type: IdentityGUTI, MCC: "001", MNC: "01", MMEGroupID: 1, MMECode: 1, MTMSI: 0x01020304}

	b, err := (&GUTIReallocationCommand{GUTI: guti}).Marshal()
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	if MessageType(b[1]) != MsgGUTIReallocationCommand {
		t.Fatalf("message type = %#x, want %#x", b[1], MsgGUTIReallocationCommand)
	}

	got, err := ParseGUTIReallocationCommand(b)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	if got.GUTI != guti {
		t.Fatalf("GUTI = %+v, want %+v", got.GUTI, guti)
	}
}

func TestGUTIReallocationCompleteRoundTrip(t *testing.T) {
	b, err := (&GUTIReallocationComplete{}).Marshal()
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	if MessageType(b[1]) != MsgGUTIReallocationComplete {
		t.Fatalf("message type = %#x, want %#x", b[1], MsgGUTIReallocationComplete)
	}

	if _, err := ParseGUTIReallocationComplete(b); err != nil {
		t.Fatalf("Parse: %v", err)
	}
}
