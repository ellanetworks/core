// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package eps

import (
	"bytes"
	"testing"
)

func TestDetachRoundTrips(t *testing.T) {
	t.Run("RequestUE switch-off", func(t *testing.T) {
		in := &DetachRequestUE{
			SwitchOff:           true,
			TypeOfDetach:        DetachTypeEPS,
			NASKeySetIdentifier: 0,
			EPSMobileIdentity: EPSMobileIdentity{
				Type: IdentityGUTI, MCC: "001", MNC: "01",
				MMEGroupID: 0x0002, MMECode: 0x01, MTMSI: 0x030003e6,
			},
		}

		b, err := in.Marshal()
		if err != nil {
			t.Fatal(err)
		}

		out, err := ParseDetachRequestUE(b)
		if err != nil {
			t.Fatal(err)
		}

		if !out.SwitchOff || out.TypeOfDetach != DetachTypeEPS || out.NASKeySetIdentifier != 0 ||
			out.EPSMobileIdentity != in.EPSMobileIdentity {
			t.Fatalf("mismatch:\n in  %+v\n out %+v", in, out)
		}
	})

	t.Run("RequestUE normal (not switch-off)", func(t *testing.T) {
		in := &DetachRequestUE{
			SwitchOff:    false,
			TypeOfDetach: DetachTypeCombined,
			EPSMobileIdentity: EPSMobileIdentity{
				Type: IdentityGUTI, MCC: "001", MNC: "01", MMEGroupID: 1, MMECode: 1, MTMSI: 1,
			},
		}

		b, _ := in.Marshal()

		out, err := ParseDetachRequestUE(b)
		if err != nil || out.SwitchOff || out.TypeOfDetach != DetachTypeCombined {
			t.Fatalf("got %+v err %v", out, err)
		}
	})

	t.Run("RequestNetwork with EMM cause", func(t *testing.T) {
		cause := uint8(2)
		in := &DetachRequestNetwork{TypeOfDetach: DetachTypeEPS, EMMCause: &cause}

		b, _ := in.Marshal()

		out, err := ParseDetachRequestNetwork(b)
		if err != nil || out.TypeOfDetach != DetachTypeEPS || out.EMMCause == nil || *out.EMMCause != 2 {
			t.Fatalf("got %+v err %v", out, err)
		}
	})

	t.Run("RequestNetwork no cause", func(t *testing.T) {
		in := &DetachRequestNetwork{TypeOfDetach: DetachTypeEPS}

		b, _ := in.Marshal()

		out, err := ParseDetachRequestNetwork(b)
		if err != nil || out.EMMCause != nil {
			t.Fatalf("got %+v err %v", out, err)
		}
	})

	t.Run("Accept", func(t *testing.T) {
		b, err := (&DetachAccept{}).Marshal()
		if err != nil {
			t.Fatal(err)
		}

		// Detach Accept is header-only: SHT/PD + message type.
		if !bytes.Equal(b, []byte{0x07, byte(MsgDetachAccept)}) {
			t.Fatalf("Detach Accept = % x, want 07 46", b)
		}

		if _, err := ParseDetachAccept(b); err != nil {
			t.Fatal(err)
		}
	})
}
