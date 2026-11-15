// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"bytes"
	"testing"
)

func TestDownlinkNASTransportWithUERadioCapabilityIDParses(t *testing.T) {
	in := &DownlinkNASTransport{
		MMEUES1APID:         1,
		ENBUES1APID:         2,
		NASPDU:              NASPDU{0x07, 0x41},
		UERadioCapabilityID: &UERadioCapabilityID{0x0a, 0x0b},
		MaskedIMEISV:        Ptr(MaskedIMEISV(0x0123456789abcdef)),
	}

	b, err := in.Marshal()
	if err != nil {
		t.Fatal(err)
	}

	pdu, err := Unmarshal(b)
	if err != nil {
		t.Fatal(err)
	}

	out, err := ParseDownlinkNASTransport(pdu.value())
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	if !out.Equal(*in) {
		t.Fatalf("decoded %+v, want %+v", out, in)
	}

	if !out.Diagnostics().Empty() {
		t.Fatalf("diagnostics = %v, want none", out.Diagnostics().IEs)
	}

	re, err := out.Marshal()
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(re, b) {
		t.Fatalf("re-encode = % x, want % x", re, b)
	}
}
