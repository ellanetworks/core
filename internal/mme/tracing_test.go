// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/s1ap"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

// TestDispatchRecordsReceiveSpan verifies the MME emits an s1ap/receive span for
// every inbound S1AP message, so 4G control-plane traffic is traceable.
func TestDispatchRecordsReceiveSpan(t *testing.T) {
	m := newTestMME(t)

	raw, err := (&s1ap.InitialUEMessage{
		ENBUES1APID:           1,
		NASPDU:                s1ap.NASPDU{0x07, 0x41},
		TAI:                   s1ap.TAI{PLMNIdentity: s1ap.PLMNIdentity{0x00, 0xf1, 0x10}, TAC: 1},
		EUTRANCGI:             s1ap.EUTRANCGI{PLMNIdentity: s1ap.PLMNIdentity{0x00, 0xf1, 0x10}, CellID: 1},
		RRCEstablishmentCause: s1ap.RRCCauseMOSignalling,
	}).Marshal()
	if err != nil {
		t.Fatal(err)
	}

	before := len(testSpanRecorder.Ended())

	// A nil conn gates the message, but the receive span is created and ended
	// before the gate, so it is recorded regardless.
	m.dispatch(context.Background(), nil, raw)

	emitted := testSpanRecorder.Ended()[before:]

	var receive sdktrace.ReadOnlySpan

	for _, s := range emitted {
		if s.Name() == "s1ap/receive" {
			receive = s
			break
		}
	}

	if receive == nil {
		t.Fatalf("no s1ap/receive span recorded; got %d spans from dispatch", len(emitted))
	}

	var sawMessageType bool

	for _, attr := range receive.Attributes() {
		if attr.Key == "s1ap.message_type" {
			sawMessageType = true
		}
	}

	if !sawMessageType {
		t.Fatal("s1ap/receive span missing s1ap.message_type attribute")
	}
}
