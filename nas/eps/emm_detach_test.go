// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package eps

import (
	"bytes"
	"reflect"
	"testing"

	"github.com/ellanetworks/core/nas"
)

func TestDetachRoundTrips(t *testing.T) {
	t.Run("RequestUE switch-off", func(t *testing.T) {
		in := &DetachRequestUE{
			SwitchOff:           true,
			TypeOfDetach:        DetachTypeEPS,
			NASKeySetIdentifier: nas.KeySetIdentifier{Value: 0},
			EPSMobileIdentity: GUTIIdentity(GUTI{
				PLMN:       nas.PLMN{MCC: "001", MNC: "01"},
				MMEGroupID: 0x0002, MMECode: 0x01, TMSI: [4]byte{0x03, 0x00, 0x03, 0xe6},
			}),
		}

		b, err := in.MarshalBinary()
		if err != nil {
			t.Fatal(err)
		}

		out, err := ParseDetachRequestUE(b)
		if err != nil {
			t.Fatal(err)
		}

		if !out.SwitchOff || out.TypeOfDetach != DetachTypeEPS || out.NASKeySetIdentifier.Value != 0 ||
			!reflect.DeepEqual(out.EPSMobileIdentity, in.EPSMobileIdentity) {
			t.Fatalf("mismatch:\n in  %+v\n out %+v", in, out)
		}
	})

	t.Run("RequestUE normal (not switch-off)", func(t *testing.T) {
		in := &DetachRequestUE{
			SwitchOff:    false,
			TypeOfDetach: DetachTypeCombined,
			EPSMobileIdentity: GUTIIdentity(GUTI{
				PLMN: nas.PLMN{MCC: "001", MNC: "01"}, MMEGroupID: 1, MMECode: 1, TMSI: [4]byte{0x00, 0x00, 0x00, 0x01},
			}),
		}

		b, _ := in.MarshalBinary()

		out, err := ParseDetachRequestUE(b)
		if err != nil || out.SwitchOff || out.TypeOfDetach != DetachTypeCombined {
			t.Fatalf("got %+v err %v", out, err)
		}
	})

	t.Run("RequestNetwork with EMM cause", func(t *testing.T) {
		cause := uint8(2)
		in := &DetachRequestNetwork{TypeOfDetach: DetachTypeReattachRequired, Cause: ptr(EMMCause(cause))}

		b, _ := in.MarshalBinary()

		out, err := ParseDetachRequestNetwork(b)
		if err != nil || out.TypeOfDetach != DetachTypeReattachRequired || out.Cause == nil || *out.Cause != 2 {
			t.Fatalf("got %+v err %v", out, err)
		}
	})

	t.Run("RequestNetwork no cause", func(t *testing.T) {
		in := &DetachRequestNetwork{TypeOfDetach: DetachTypeReattachRequired}

		b, _ := in.MarshalBinary()

		out, err := ParseDetachRequestNetwork(b)
		if err != nil || out.Cause != nil {
			t.Fatalf("got %+v err %v", out, err)
		}
	})

	t.Run("Accept", func(t *testing.T) {
		b, err := (&DetachAccept{}).MarshalBinary()
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
