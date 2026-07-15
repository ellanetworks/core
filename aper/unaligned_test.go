// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package aper_test

import (
	"testing"

	"github.com/ellanetworks/core/aper"
)

// TestUnalignedConstrainedIntUsesMinimumBits covers X.691 §13.2.6: the
// unaligned variant encodes a constrained whole number in the minimum number of
// bits for the range, with none of the aligned variant's octet forms. The
// range 0..255 is the case that diverges: aligned pads to an octet boundary
// first, unaligned does not.
func TestUnalignedConstrainedIntUsesMinimumBits(t *testing.T) {
	// One bit, then INTEGER(0..255): unaligned packs 1+8 = 9 bits into 2 octets.
	uw := aper.NewUnalignedWriter()
	uw.WriteBit(1)

	if err := uw.WriteConstrainedInt(1, 0, 255); err != nil {
		t.Fatalf("unaligned WriteConstrainedInt: %v", err)
	}

	if got := len(uw.Bytes()); got != 2 {
		t.Errorf("unaligned: got %d octets, want 2 (9 bits, no alignment)", got)
	}

	// Aligned pads to the octet boundary before the value: 1 bit + 7 pad + 8.
	var aw aper.Writer

	aw.WriteBit(1)

	if err := aw.WriteConstrainedInt(1, 0, 255); err != nil {
		t.Fatalf("aligned WriteConstrainedInt: %v", err)
	}

	if got := len(aw.Bytes()); got != 2 {
		t.Errorf("aligned: got %d octets, want 2", got)
	}

	// The encodings must differ: that difference is the alignment padding.
	if string(uw.Bytes()) == string(aw.Bytes()) {
		t.Errorf("unaligned and aligned encodings are identical (%x); alignment was not skipped", uw.Bytes())
	}
}

func TestUnalignedConstrainedIntRoundTrip(t *testing.T) {
	for _, v := range []int64{0, 1, 127, 128, 255, 1000, 65535, 65536} {
		w := aper.NewUnalignedWriter()
		if err := w.WriteConstrainedInt(v, 0, 65536); err != nil {
			t.Fatalf("write %d: %v", v, err)
		}

		got, err := aper.NewUnalignedReader(w.Bytes()).ReadConstrainedInt(0, 65536)
		if err != nil {
			t.Fatalf("read %d: %v", v, err)
		}

		if got != v {
			t.Errorf("round trip: got %d, want %d", got, v)
		}
	}
}

// TestUnalignedDecodesLPPErrorFromUE decodes the LPP-Message header of a reply
// captured from a commercial handset. LPP mandates the unaligned variant
// (TS 37.355 §5), so the aligned reader rejects these octets; this guards that
// the unaligned reader agrees with a real peer.
//
//	LPP-Message ::= SEQUENCE {
//	    transactionID   LPP-TransactionID OPTIONAL,
//	    endTransaction  BOOLEAN,
//	    sequenceNumber  SequenceNumber    OPTIONAL,
//	    acknowledgement Acknowledgement   OPTIONAL,
//	    lpp-MessageBody LPP-MessageBody   OPTIONAL }
func TestUnalignedDecodesLPPErrorFromUE(t *testing.T) {
	r := aper.NewUnalignedReader([]byte{0xf0, 0x01, 0x00, 0x4e, 0x48})

	_, optionals, err := r.ReadSequencePreamble(false, 4)
	if err != nil {
		t.Fatalf("LPP-Message preamble: %v", err)
	}

	for i, want := range []bool{true, true, true, true} {
		if optionals[i] != want {
			t.Fatalf("optional %d: got %v, want %v", i, optionals[i], want)
		}
	}

	if _, err := r.ReadBool(); err != nil { // endTransaction
		t.Fatalf("endTransaction: %v", err)
	}

	if _, _, err := r.ReadSequencePreamble(true, 0); err != nil { // LPP-TransactionID is extensible
		t.Fatalf("LPP-TransactionID preamble: %v", err)
	}

	initiator, _, err := r.ReadEnum(2, true) // Initiator ::= ENUMERATED {locationServer, targetDevice, ...}
	if err != nil {
		t.Fatalf("initiator: %v", err)
	}

	if initiator != 0 {
		t.Errorf("initiator: got %d, want 0 (locationServer)", initiator)
	}

	if _, err := r.ReadConstrainedInt(0, 255); err != nil { // transactionNumber
		t.Fatalf("transactionNumber: %v", err)
	}

	if _, err := r.ReadConstrainedInt(0, 255); err != nil { // sequenceNumber
		t.Fatalf("sequenceNumber: %v", err)
	}

	// Acknowledgement ::= SEQUENCE { ackRequested BOOLEAN, ackIndicator SequenceNumber OPTIONAL }
	if _, _, err := r.ReadSequencePreamble(false, 1); err != nil {
		t.Fatalf("Acknowledgement preamble: %v", err)
	}

	ackRequested, err := r.ReadBool()
	if err != nil {
		t.Fatalf("ackRequested: %v", err)
	}

	if !ackRequested {
		t.Error("ackRequested: got false, want true (the UE awaits an LPP acknowledgement)")
	}

	body, _, err := r.ReadChoiceIndex(2, false) // LPP-MessageBody ::= CHOICE { c1, messageClassExtension }
	if err != nil {
		t.Fatalf("lpp-MessageBody: %v", err)
	}

	if body != 0 {
		t.Errorf("lpp-MessageBody: got choice %d, want 0 (c1)", body)
	}

	c1, _, err := r.ReadChoiceIndex(16, false) // 8 messages + 8 spares
	if err != nil {
		t.Fatalf("c1: %v", err)
	}

	if c1 != 7 {
		t.Errorf("c1: got %d, want 7 (error)", c1)
	}
}
