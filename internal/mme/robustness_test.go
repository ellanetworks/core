// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"bytes"
	"context"
	"testing"

	"github.com/ellanetworks/core/s1ap"
)

// TestDispatchSurvivesGarbage feeds malformed PDUs to the dispatcher and checks
// it neither panics nor disrupts the association — the codecs reject malformed
// input rather than relying on any panic recovery.
func TestDispatchSurvivesGarbage(t *testing.T) {
	m := newTestMME(t)

	for _, g := range [][]byte{
		nil,
		{},
		{0x00},
		{0xff, 0xff, 0xff},
		{0x00, 0x11, 0x22, 0x33, 0x44, 0x55},
		bytes.Repeat([]byte{0xab}, 128),
	} {
		m.dispatch(context.Background(), nil, g) // must not panic
	}
}

func initialUEMessagePDU(t *testing.T, enbID s1ap.ENBUES1APID) []byte {
	t.Helper()

	plmn := s1ap.PLMNIdentity{0x00, 0xf1, 0x10}

	b, err := (&s1ap.InitialUEMessage{
		ENBUES1APID:           enbID,
		NASPDU:                s1ap.NASPDU{0x00}, // routed-but-ignored (non-EMM PD)
		TAI:                   s1ap.TAI{PLMNIdentity: plmn, TAC: 1},
		EUTRANCGI:             s1ap.EUTRANCGI{PLMNIdentity: plmn, CellID: 1},
		RRCEstablishmentCause: s1ap.RRCCauseEmergency,
	}).Marshal()
	if err != nil {
		t.Fatal(err)
	}

	return b
}

// TestDuplicateInitialUEMessage checks a second Initial UE Message for the same
// eNB UE id on the same association replaces the prior context rather than
// leaking it.
func TestDuplicateInitialUEMessage(t *testing.T) {
	m := newTestMME(t)

	m.handleInitialUEMessage(nil, initiatingValue(t, initialUEMessagePDU(t, 7)))
	m.handleInitialUEMessage(nil, initiatingValue(t, initialUEMessagePDU(t, 7)))

	if got := len(m.ues); got != 1 {
		t.Fatalf("duplicate Initial UE Message left %d contexts, want 1", got)
	}
}
