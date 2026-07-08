// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/internal/mme"
	"github.com/ellanetworks/core/s1ap"
)

func TestUplinkNASTransportUnknownUE(t *testing.T) {
	m := newTestMME(t)

	uplink := &s1ap.UplinkNASTransport{
		MMEUES1APID: 999,
		ENBUES1APID: 7,
		NASPDU:      s1ap.NASPDU{0x07, 0x56},
		EUTRANCGI:   s1ap.EUTRANCGI{PLMNIdentity: s1ap.PLMNIdentity{0x00, 0xf1, 0x10}, CellID: 1},
		TAI:         s1ap.TAI{PLMNIdentity: s1ap.PLMNIdentity{0x00, 0xf1, 0x10}, TAC: 1},
	}

	b, err := uplink.Marshal()
	if err != nil {
		t.Fatal(err)
	}

	// An unknown MME-UE-S1AP-ID is answered with an Error Indication, not
	// silently dropped, and no context is created (TS 36.413).
	conn := &captureConn{}
	handleUplinkNASTransport(m, context.Background(), mme.NewRadioForTest(conn), initiatingValue(t, b))

	if _, ok := m.LookupUe(999); ok {
		t.Fatal("unexpected UE context for unknown MME-UE-S1AP-ID")
	}

	if len(conn.sent) != 1 {
		t.Fatalf("expected one Error Indication, got %d", len(conn.sent))
	}

	ind := parseOutboundErrorIndication(t, conn.sent[0])
	if ind.MMEUES1APID == nil || *ind.MMEUES1APID != 999 || ind.ENBUES1APID == nil || *ind.ENBUES1APID != 7 {
		t.Fatalf("expected the received AP-ID pair (999, 7), got (%v, %v)", ind.MMEUES1APID, ind.ENBUES1APID)
	}

	if ind.Cause == nil || *ind.Cause != causeUnknownMMEUES1APID {
		t.Fatalf("expected cause unknown-mme-ue-s1ap-id, got %v", ind.Cause)
	}
}
