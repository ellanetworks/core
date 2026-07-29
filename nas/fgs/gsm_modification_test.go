// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package fgs

import (
	"bytes"
	"testing"

	"github.com/ellanetworks/core/nas"
)

// TestPDUSessionModificationRequestRoundTrip checks the message the UE sends to
// modify a session decodes to its fields and re-encodes unchanged, including the
// elements this package does not model (TS 24.501 §8.3.7).
func TestPDUSessionModificationRequestRoundTrip(t *testing.T) {
	cause := GSMCauseRegularDeactivation

	in := &PDUSessionModificationRequest{
		PDUSessionID:      5,
		PTI:               3,
		GSMCapability:     &GSMCapability{RqoS: true},
		Cause:             &cause,
		AlwaysOnRequested: true,
		RequestedQoSFlows: QoSFlowDescriptions{FiveQIQoSFlow(1, 9, QoSFlowOpCreate)},
		Unrecognized: []nas.RawIE{
			// Maximum number of supported packet filters, a TV the table frames.
			{IEI: ieiMaxPacketFilters, Format: nas.IETV3, Value: []byte{0x10, 0x00}},
		},
	}

	b, err := in.MarshalBinary()
	if err != nil {
		t.Fatalf("MarshalBinary: %v", err)
	}

	msg, err := ParseMessage(b)
	if err != nil {
		t.Fatalf("ParseMessage: %v", err)
	}

	out, ok := msg.(*PDUSessionModificationRequest)
	if !ok {
		t.Fatalf("ParseMessage returned %T", msg)
	}

	if out.SessionIdentity() != 5 || out.TransactionIdentity() != 3 {
		t.Errorf("header = %d/%d, want 5/3", out.SessionIdentity(), out.TransactionIdentity())
	}

	if out.Cause == nil || *out.Cause != cause {
		t.Errorf("cause = %v, want %s", out.Cause, cause)
	}

	if !out.AlwaysOnRequested {
		t.Error("always-on requested was dropped")
	}

	if out.GSMCapability == nil || !out.GSMCapability.RqoS {
		t.Errorf("5GSM capability = %+v", out.GSMCapability)
	}

	if len(out.RequestedQoSFlows) != 1 {
		t.Errorf("requested QoS flow descriptions = %+v", out.RequestedQoSFlows)
	}

	if len(out.Unrecognized) != 1 {
		t.Errorf("unmodelled elements = %+v, want the packet-filter maximum preserved", out.Unrecognized)
	}

	again, err := out.MarshalBinary()
	if err != nil {
		t.Fatalf("re-encode: %v", err)
	}

	if !bytes.Equal(again, b) {
		t.Errorf("re-encode = % x, want % x", again, b)
	}
}
