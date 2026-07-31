// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"bytes"
	"context"
	"encoding/hex"
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

	handleUECapabilityInfoIndication(m, context.Background(), mme.NewRadioForTest(cc), initiatingValue(t, b))

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
	handleUECapabilityInfoIndication(m, context.Background(), mme.NewRadioForTest(&captureConn{}), initiatingValue(t, b))

	if _, ok := m.LookupUe(999); ok {
		t.Fatal("unexpected UE context for unknown MME-UE-S1AP-ID")
	}
}

// TS 36.413 §10.3.5: UE Radio Capability is mandatory/ignore, so a message
// omitting it is delivered, and must leave the stored capability standing.
func TestUECapabilityInfoIndicationAbsentCapabilityKeepsStored(t *testing.T) {
	m := newTestMME(t)
	cc := &captureConn{}
	ue := m.NewUe(cc, 7)

	stored := []byte{0x01, 0x02, 0x03, 0x04}
	ue.RadioCapability = stored

	// Only the two UE S1AP IDs; the encoder refuses to omit a required IE, so
	// the container is built as a peer would send it.
	body, err := hex.DecodeString("000002" + "00004002" + "0001" + "00084002" + "0007")
	if err != nil {
		t.Fatal(err)
	}

	msg, err := s1ap.ParseUECapabilityInfoIndication(body)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	if msg.UERadioCapability != nil {
		t.Fatalf("UERadioCapability = %x, want absent", msg.UERadioCapability)
	}

	handleUECapabilityInfoIndication(m, context.Background(), mme.NewRadioForTest(cc), body)

	if !bytes.Equal(ue.RadioCapability, stored) {
		t.Fatalf("radio capability = %x, want the stored %x", ue.RadioCapability, stored)
	}
}
