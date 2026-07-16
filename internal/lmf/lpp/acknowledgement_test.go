// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package lpp

import (
	"encoding/hex"
	"testing"
)

// TestParseAckInfoFromUE reads the acknowledgement header of the capabilities
// reply a handset sent. It sets ackRequested and sequence number 0, and a UE
// that does not see an acknowledgement retransmits every 2 s (TS 37.355 §4.3.4).
func TestParseAckInfoFromUE(t *testing.T) {
	raw, err := hex.DecodeString("f001014200")
	if err != nil {
		t.Fatalf("hex: %v", err)
	}

	info, err := ParseAckInfo(raw)
	if err != nil {
		t.Fatalf("ParseAckInfo: %v", err)
	}

	if !info.AckRequested {
		t.Error("ackRequested: got false, want true")
	}

	if !info.HasSequence || info.SequenceNumber != 1 {
		t.Errorf("sequenceNumber: got (%d, present=%v), want (1, true)", info.SequenceNumber, info.HasSequence)
	}
}

// TestBuildAcknowledgementRoundTrip pins the acknowledgement the LMF returns:
// a body-less message whose ackIndicator is the acknowledged sequence number
// and whose own sequence number is this sender's counter (TS 37.355 §4.3.3).
func TestBuildAcknowledgementRoundTrip(t *testing.T) {
	b, err := BuildAcknowledgement(4, 0)
	if err != nil {
		t.Fatalf("BuildAcknowledgement: %v", err)
	}

	msg, err := DecodeMessage(b)
	if err != nil {
		t.Fatalf("DecodeMessage: %v", err)
	}

	if msg.LppMessageBody != nil {
		t.Errorf("lpp-MessageBody: got %+v, want nil", msg.LppMessageBody)
	}

	if msg.SequenceNumber == nil || *msg.SequenceNumber != 4 {
		t.Errorf("sequenceNumber: got %v, want 4", msg.SequenceNumber)
	}

	if msg.Acknowledgement == nil || msg.Acknowledgement.AckRequested {
		t.Fatalf("acknowledgement: got %+v, want ackRequested=false", msg.Acknowledgement)
	}

	if msg.Acknowledgement.AckIndicator == nil || *msg.Acknowledgement.AckIndicator != 0 {
		t.Errorf("ackIndicator: got %v, want 0", msg.Acknowledgement.AckIndicator)
	}
}
