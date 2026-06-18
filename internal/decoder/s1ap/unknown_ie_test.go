// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"testing"

	"github.com/ellanetworks/core/s1ap"
)

// TestAppendUnknownIEs checks an IE the library does not model is surfaced as a
// raw {id, criticality, hex} entry with its id flagged unknown, so a present IE
// is shown rather than dropped (TS 36.413 §9.3).
func TestAppendUnknownIEs(t *testing.T) {
	ies := appendUnknownIEs(nil, []s1ap.RawIE{
		{ID: 300, Criticality: s1ap.CriticalityNotify, Value: []byte{0xde, 0xad}},
	})

	if len(ies) != 1 {
		t.Fatalf("len = %d, want 1", len(ies))
	}

	got := ies[0]
	if got.ID.Value != 300 || !got.ID.Unknown {
		t.Fatalf("id = %+v, want value 300 flagged unknown", got.ID)
	}

	if got.Criticality.Label != "Notify" {
		t.Fatalf("criticality = %+v, want Notify", got.Criticality)
	}

	rv, ok := got.Value.(rawIEValue)
	if !ok || rv.Hex != "dead" {
		t.Fatalf("value = %#v, want rawIEValue{Hex: \"dead\"}", got.Value)
	}
}

// TestDecodeInitialContextSetupFailure checks the Initial Context Setup Failure
// message (UnsuccessfulOutcome, TS 36.413 §9.1.4.3) is now dispatched and its
// Cause rendered, rather than reported as an unsupported procedure.
func TestDecodeInitialContextSetupFailure(t *testing.T) {
	raw, err := (&s1ap.InitialContextSetupFailure{
		MMEUES1APID: 1,
		ENBUES1APID: 2,
		Cause:       s1ap.Cause{Group: s1ap.CauseGroupRadioNetwork, Value: 0},
	}).Marshal()
	if err != nil {
		t.Fatal(err)
	}

	msg := DecodeS1APMessage(raw)
	if msg.PDUType != "UnsuccessfulOutcome" {
		t.Fatalf("PDUType = %q, want UnsuccessfulOutcome", msg.PDUType)
	}

	if msg.Value.Error != "" {
		t.Fatalf("decode error: %s", msg.Value.Error)
	}

	var foundCause bool

	for _, ie := range msg.Value.IEs {
		if ie.ID.Value == idCause {
			foundCause = true
		}
	}

	if !foundCause {
		t.Fatalf("Cause IE not rendered: %+v", msg.Value.IEs)
	}
}
