// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"bytes"
	"context"
	"testing"

	"github.com/ellanetworks/core/s1ap"
)

func TestDispatchGatesUEMessageBeforeS1Setup(t *testing.T) {
	m := newTestMME(t)

	raw, err := (&s1ap.InitialUEMessage{
		ENBUES1APID:           1,
		NASPDU:                s1ap.NASPDU{0x07, 0x41},
		TAI:                   s1ap.TAI{PLMNIdentity: s1ap.PLMNIdentity{0x00, 0xf1, 0x10}, TAC: 1},
		EUTRANCGI:             s1ap.Ptr(s1ap.EUTRANCGI{PLMNIdentity: s1ap.PLMNIdentity{0x00, 0xf1, 0x10}, CellID: 1}),
		RRCEstablishmentCause: s1ap.Ptr(s1ap.RRCCauseMOSignalling),
	}).Marshal()
	if err != nil {
		t.Fatal(err)
	}

	Dispatch(context.Background(), m, nil, raw)

	if got := m.ConnCountForTest(); got != 0 {
		t.Fatalf("UE context created from an Initial UE Message before S1 Setup: %d", got)
	}
}

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
		Dispatch(context.Background(), m, nil, g)
	}

	if got := m.ConnCountForTest(); got != 0 {
		t.Fatalf("malformed PDUs created %d UE contexts", got)
	}
}
