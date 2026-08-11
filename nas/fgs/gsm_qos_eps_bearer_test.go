// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package fgs

import (
	"bytes"
	"testing"
)

// TestEPSBearerIDQoSFlowParameter pins the nibble TS 24.501 §9.11.4.12 puts the
// EPS bearer identity in: bits 5 to 8 of a one-octet parameter, with bits 1 to 4
// spare. Written into the low nibble instead it would name a different bearer,
// and EBI 5 would read as 0.
func TestEPSBearerIDQoSFlowParameter(t *testing.T) {
	p, err := EPSBearerIDQoSFlowParameter(5)
	if err != nil {
		t.Fatal(err)
	}

	if p.ID != QoSFlowParamEPSBearerID {
		t.Errorf("identifier = %#02x, want 0x07", uint8(p.ID))
	}

	if want := []byte{0x50}; !bytes.Equal(p.Value, want) {
		t.Fatalf("value = % x, want % x", p.Value, want)
	}

	if _, err := EPSBearerIDQoSFlowParameter(16); err == nil {
		t.Error("EBI 16: want an error, got none")
	}
}

// TestQoSFlowDescriptionEPSBearerIDRoundTrip carries the parameter through the
// element it belongs to, which is how the EBI-to-QFI mapping reaches the UE.
func TestQoSFlowDescriptionEPSBearerIDRoundTrip(t *testing.T) {
	param, err := EPSBearerIDQoSFlowParameter(9)
	if err != nil {
		t.Fatal(err)
	}

	flow := FiveQIQoSFlow(1, 9, QoSFlowOpCreate)
	flow.Parameters = append(flow.Parameters, param)

	raw, err := QoSFlowDescriptions{flow}.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}

	back, err := ParseQoSFlowDescriptions(raw)
	if err != nil {
		t.Fatal(err)
	}

	if len(back) != 1 {
		t.Fatalf("decoded %d descriptions, want 1", len(back))
	}

	ebi, ok := back[0].EPSBearerID()
	if !ok {
		t.Fatalf("no EPS bearer identity in %+v", back[0])
	}

	if ebi != 9 {
		t.Fatalf("EPS bearer identity = %d, want 9", ebi)
	}

	// A flow with no EBI is the ordinary case: TS 23.502 §4.11.1.4.1 allocates
	// them only to the flows that can transfer, and TS 24.501 §6.1.4.1 has the UE
	// locally delete the rules and descriptions of the flows without one.
	if _, ok := FiveQIQoSFlow(2, 9, QoSFlowOpCreate).EPSBearerID(); ok {
		t.Error("a flow with no EPS bearer identity reported one")
	}
}
