// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"bytes"
	"testing"

	"github.com/ellanetworks/core/internal/mme"
	"github.com/ellanetworks/core/s1ap"
)

func TestUECapabilityInfoIndicationStoresRadioCapability(t *testing.T) {
	m := newTestMME(t)
	cc := &captureConn{}
	ue := m.NewUe(cc, 7)

	radioCap := []byte{0x01, 0x02, 0x03, 0x04}
	pagingCap := []byte{0xaa, 0xbb}
	ind := &s1ap.UECapabilityInfoIndication{
		MMEUES1APID:                ue.Conn().MMEUES1APID,
		ENBUES1APID:                ue.Conn().ENBUES1APID,
		UERadioCapability:          radioCap,
		UERadioCapabilityForPaging: pagingCap,
	}

	b, err := ind.Marshal()
	if err != nil {
		t.Fatal(err)
	}

	handleUECapabilityInfoIndication(m, mme.NewRadioForTest(cc), initiatingValue(t, b))

	if !bytes.Equal(ue.RadioCapability, radioCap) {
		t.Fatalf("radio capability = %x, want %x", ue.RadioCapability, radioCap)
	}

	if !bytes.Equal(ue.RadioCapabilityForPaging, pagingCap) {
		t.Fatalf("radio capability for paging = %x, want %x", ue.RadioCapabilityForPaging, pagingCap)
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
	handleUECapabilityInfoIndication(m, mme.NewRadioForTest(&captureConn{}), initiatingValue(t, b))

	if _, ok := m.LookupUe(999); ok {
		t.Fatal("unexpected UE context for unknown MME-UE-S1AP-ID")
	}
}
