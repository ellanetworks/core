// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package eps

import (
	"errors"
	"testing"

	"github.com/ellanetworks/core/nas"
)

func TestParseServiceRequest(t *testing.T) {
	// SHT=12|PD=7, KSI=1|seq=10, short MAC 0x1234.
	sr, err := ParseServiceRequest([]byte{0xc7, 0x2a, 0x12, 0x34})
	if err != nil {
		t.Fatal(err)
	}

	if sr.KSI != 1 {
		t.Fatalf("KSI = %d, want 1", sr.KSI)
	}

	if sr.SeqShort != 10 {
		t.Fatalf("SeqShort = %d, want 10", sr.SeqShort)
	}

	if sr.ShortMAC != [2]byte{0x12, 0x34} {
		t.Fatalf("ShortMAC = %x, want 1234", sr.ShortMAC)
	}
}

func TestParseServiceRequestWrongLength(t *testing.T) {
	for _, b := range [][]byte{nil, {0xc7}, {0xc7, 0x00, 0x00}, {0xc7, 0x00, 0x00, 0x00, 0x00}} {
		if _, err := ParseServiceRequest(b); err == nil {
			t.Fatalf("ParseServiceRequest(%x) = nil error, want failure", b)
		}
	}
}

func TestServiceRequestShortMAC(t *testing.T) {
	var key [16]byte
	for i := range key {
		key[i] = byte(i)
	}

	sc, err := nas.NewSecurityContext(nas.SecurityContextOptions{
		Integrity:    nas.IntegrityAES,
		Ciphering:    nas.CipheringAES,
		IntegrityKey: key,
		CipherKey:    key,
	})
	if err != nil {
		t.Fatal(err)
	}

	count := nas.MakeCount(0, 0x25)

	sr, err := NewServiceRequest(3, count, sc)
	if err != nil {
		t.Fatal(err)
	}

	if sr.KSI != 3 || sr.SeqShort != 0x05 {
		t.Fatalf("SERVICE REQUEST = %+v, want KSI 3 and the count's low 5 bits", sr)
	}

	if err := VerifyServiceRequestShortMAC(sr, count, sc); err != nil {
		t.Fatalf("the short MAC it built does not verify: %v", err)
	}

	// A different count is a different MAC, so the check must fail.
	if err := VerifyServiceRequestShortMAC(sr, nas.MakeCount(1, 0x25), sc); !errors.Is(err, nas.ErrMACMismatch) {
		t.Errorf("wrong count: err = %v, want ErrMACMismatch", err)
	}

	// So is a flipped MAC octet.
	forged := *sr
	forged.ShortMAC[0] ^= 0xFF

	if err := VerifyServiceRequestShortMAC(&forged, count, sc); !errors.Is(err, nas.ErrMACMismatch) {
		t.Errorf("forged MAC: err = %v, want ErrMACMismatch", err)
	}

	if err := VerifyServiceRequestShortMAC(sr, count, nil); !errors.Is(err, nas.ErrNoSecurityContext) {
		t.Errorf("no context: err = %v, want ErrNoSecurityContext", err)
	}
}

func TestServiceRejectMarshal(t *testing.T) {
	b, err := (&ServiceReject{Cause: 9}).MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}

	mt, err := PeekMessageType(b)
	if err != nil {
		t.Fatal(err)
	}

	if mt != MsgServiceReject {
		t.Fatalf("message type = %#x, want %#x", mt, MsgServiceReject)
	}

	if b[len(b)-1] != 9 {
		t.Fatalf("cause octet = %d, want 9", b[len(b)-1])
	}
}

// TestServiceAcceptModelsItsElements guards the rule that made both of this
// message's optional elements unreadable: the walker offers an element to the
// message only if the message's table frames it, whatever the element's own
// framing, so an element a message models must have a table entry
// (TS 24.301 table 8.2.34.1).
func TestServiceAcceptModelsItsElements(t *testing.T) {
	var status nas.EPSBearerContextStatus

	status.Active[5] = true

	timer := nas.GPRSTimer2{Unit: nas.GPRSTimer2Unit1Minute, Value: 1}

	raw, err := (&ServiceAccept{EPSBearerContextStatus: &status, T3448: &timer}).MarshalBinary()
	if err != nil {
		t.Fatalf("MarshalBinary: %v", err)
	}

	got, err := ParseServiceAccept(raw)
	if err != nil {
		t.Fatalf("ParseServiceAccept: %v", err)
	}

	if len(got.Unrecognized) != 0 {
		t.Errorf("%d elements fell through to Unrecognized: %+v", len(got.Unrecognized), got.Unrecognized)
	}

	if got.EPSBearerContextStatus == nil || !got.EPSBearerContextStatus.Active[5] {
		t.Errorf("EPS bearer context status = %v, want bearer 5 active", got.EPSBearerContextStatus)
	}

	if got.T3448 == nil || *got.T3448 != timer {
		t.Errorf("T3448 = %v, want %v", got.T3448, timer)
	}
}
