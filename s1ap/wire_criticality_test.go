// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"testing"

	"github.com/ellanetworks/core/per"
)

// wireIEs encodes a message body and reports the {id, criticality} pairs it
// put on the wire. decodeIEContainer is the only place criticality survives
// decoding — the message parsers discard it — so this is the only way to
// assert what a peer actually receives.
func wireIEs(t *testing.T, body func(*per.Writer, per.Encoding) error) []rawIE {
	t.Helper()

	w := per.NewWriter()
	if err := body(w, per.Aligned); err != nil {
		t.Fatal(err)
	}

	w.AlignToByte()
	r := per.NewReader(w.Bytes())

	if _, err := r.ReadBit(); err != nil {
		t.Fatal(err)
	}

	fields, err := decodeIEContainer(r, per.Aligned)
	if err != nil {
		t.Fatal(err)
	}

	return fields
}

// TestWireCriticality pins each stamped criticality against TS 36.413 §9.1.
// A round-trip test cannot: encode and decode agree with each other while both
// disagree with the spec.
func TestWireCriticality(t *testing.T) {
	tests := []struct {
		name string
		body func(*per.Writer, per.Encoding) error
		want []MissingIE // {id, criticality} in wire order
	}{
		{
			"HandoverCancel §9.1.5.11",
			(&HandoverCancel{}).encodeBody,
			[]MissingIE{
				{idMMEUES1APID, CriticalityReject},
				{idENBUES1APID, CriticalityReject},
				{idCause, CriticalityIgnore},
			},
		},
		{
			"HandoverPreparationFailure §9.1.5.3",
			(&HandoverPreparationFailure{}).encodeBody,
			[]MissingIE{
				{idMMEUES1APID, CriticalityIgnore},
				{idENBUES1APID, CriticalityIgnore},
				{idCause, CriticalityIgnore},
			},
		},
		{
			"PathSwitchRequestFailure §9.1.5.10",
			(&PathSwitchRequestFailure{}).encodeBody,
			[]MissingIE{
				{idMMEUES1APID, CriticalityIgnore},
				{idENBUES1APID, CriticalityIgnore},
				{idCause, CriticalityIgnore},
			},
		},
		{
			"UplinkNASTransport §9.1.7.3",
			(&UplinkNASTransport{}).encodeBody,
			[]MissingIE{
				{idMMEUES1APID, CriticalityReject},
				{idENBUES1APID, CriticalityReject},
				{idNASPDU, CriticalityReject},
				{idEUTRANCGI, CriticalityIgnore},
				{idTAI, CriticalityIgnore},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := wireIEs(t, tt.body)
			if len(got) != len(tt.want) {
				t.Fatalf("encoded %d IEs, want %d", len(got), len(tt.want))
			}

			for i, w := range tt.want {
				if got[i].id != w.ID || got[i].crit != w.Criticality {
					t.Errorf("IE %d: got %v/%v, want %v/%v", i, got[i].id, got[i].crit, w.ID, w.Criticality)
				}
			}
		})
	}
}
