// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"bytes"
	"testing"

	"github.com/ellanetworks/core/s1ap"
)

func TestUECapabilityInfoIndicationStoresRadioCapability(t *testing.T) {
	m := newTestMME(t)
	cc := &captureConn{}
	ue := m.NewUe(cc, 7)

	radioCap := []byte{0x01, 0x02, 0x03, 0x04}
	ind := &s1ap.UECapabilityInfoIndication{
		MMEUES1APID:       ue.S1.MMEUES1APID,
		ENBUES1APID:       ue.S1.ENBUES1APID,
		UERadioCapability: radioCap,
	}

	b, err := ind.Marshal()
	if err != nil {
		t.Fatal(err)
	}

	handleUECapabilityInfoIndication(m, cc, initiatingValue(t, b))

	if !bytes.Equal(ue.RadioCapability, radioCap) {
		t.Fatalf("radio capability = %x, want %x", ue.RadioCapability, radioCap)
	}
}

func TestUECapabilityInfoIndicationUnknownUE(t *testing.T) {
	m := newTestMME(t)

	ind := &s1ap.UECapabilityInfoIndication{
		MMEUES1APID:       999,
		ENBUES1APID:       7,
		UERadioCapability: []byte{0xaa},
	}

	b, err := ind.Marshal()
	if err != nil {
		t.Fatal(err)
	}

	// Must not panic or create a context for an unknown MME-UE-S1AP-ID.
	handleUECapabilityInfoIndication(m, &captureConn{}, initiatingValue(t, b))

	if _, ok := m.LookupUe(999); ok {
		t.Fatal("unexpected UE context for unknown MME-UE-S1AP-ID")
	}
}
