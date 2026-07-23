// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package fgs

import (
	"bytes"
	"errors"
	"testing"

	"github.com/ellanetworks/core/nas/common"
)

func TestSecurityProtectedMessageRoundTrip(t *testing.T) {
	m := &SecurityProtectedMessage{
		SecurityHeaderType: SHTIntegrityProtectedCiphered,
		MAC:                [4]byte{0x11, 0x22, 0x33, 0x44},
		SequenceNumber:     0x2A,
		Payload:            []byte{0xDE, 0xAD, 0xBE, 0xEF},
	}

	b := m.Marshal()

	// Wire layout (TS 24.501 §9.1.1): EPD | SHT+spare | MAC(4) | SN | payload = 7-octet header.
	want := []byte{EPD5GMM, 0x02, 0x11, 0x22, 0x33, 0x44, 0x2A, 0xDE, 0xAD, 0xBE, 0xEF}
	if !bytes.Equal(b, want) {
		t.Fatalf("Marshal = %#x, want %#x", b, want)
	}

	got, err := ParseSecurityProtectedMessage(b)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	if got.SecurityHeaderType != m.SecurityHeaderType || got.SequenceNumber != m.SequenceNumber ||
		got.MAC != m.MAC || !bytes.Equal(got.Payload, m.Payload) {
		t.Fatalf("round-trip mismatch: got %+v, want %+v", got, m)
	}
}

func TestParseSecurityProtectedRejects(t *testing.T) {
	// Plain (SHT 0) is not a security-protected message.
	plain := []byte{EPD5GMM, 0x00, 0, 0, 0, 0, 0}
	if _, err := ParseSecurityProtectedMessage(plain); !errors.Is(err, ErrNotProtected) {
		t.Errorf("plain message: got %v, want ErrNotProtected", err)
	}

	// Wrong EPD (5GSM) in the outer wrapper.
	wrong := []byte{EPD5GSM, 0x01, 0, 0, 0, 0, 0}
	if _, err := ParseSecurityProtectedMessage(wrong); !errors.Is(err, ErrNotGMM) {
		t.Errorf("wrong EPD: got %v, want ErrNotGMM", err)
	}

	// Truncated before the sequence number.
	if _, err := ParseSecurityProtectedMessage([]byte{EPD5GMM, 0x01, 0, 0, 0, 0}); err == nil {
		t.Error("truncated message: expected error, got nil")
	}
}

func TestPlainMMHeader(t *testing.T) {
	var w common.Writer

	writeGMMHeader(&w, MsgIdentityRequest)
	w.U8(0xAB) // one IE octet

	b := w.Bytes()

	want := []byte{EPD5GMM, 0x00, uint8(MsgIdentityRequest), 0xAB}
	if !bytes.Equal(b, want) {
		t.Fatalf("writeGMMHeader = %#x, want %#x", b, want)
	}

	mt, err := PeekMessageType(b)
	if err != nil {
		t.Fatalf("PeekMessageType: %v", err)
	}

	if mt != MsgIdentityRequest {
		t.Fatalf("PeekMessageType = %#x, want %#x", mt, MsgIdentityRequest)
	}

	if err := readGMMHeader(common.NewReader(b), MsgIdentityRequest); err != nil {
		t.Fatalf("readGMMHeader: %v", err)
	}

	if err := readGMMHeader(common.NewReader(b), MsgRegistrationRequest); !errors.Is(err, ErrWrongMessageType) {
		t.Errorf("readGMMHeader wrong type: got %v, want ErrWrongMessageType", err)
	}
}

func TestPeekMessageTypeRejectsProtected(t *testing.T) {
	protected := []byte{EPD5GMM, 0x01, uint8(MsgRegistrationRequest)}
	if _, err := PeekMessageType(protected); !errors.Is(err, ErrNotPlain) {
		t.Errorf("got %v, want ErrNotPlain", err)
	}
}

func TestPlainSMHeader(t *testing.T) {
	var w common.Writer

	writeGSMHeader(&w, 5, 1, MsgPDUSessionEstablishmentRequest)
	w.U8(0xCD)

	b := w.Bytes()

	want := []byte{EPD5GSM, 5, 1, uint8(MsgPDUSessionEstablishmentRequest), 0xCD}
	if !bytes.Equal(b, want) {
		t.Fatalf("writeGSMHeader = %#x, want %#x", b, want)
	}

	mt, err := PeekGSMMessageType(b)
	if err != nil {
		t.Fatalf("PeekGSMMessageType: %v", err)
	}

	if mt != MsgPDUSessionEstablishmentRequest {
		t.Fatalf("PeekGSMMessageType = %#x, want %#x", mt, MsgPDUSessionEstablishmentRequest)
	}

	psi, pti, err := readGSMHeader(common.NewReader(b), MsgPDUSessionEstablishmentRequest)
	if err != nil {
		t.Fatalf("readGSMHeader: %v", err)
	}

	if psi != 5 || pti != 1 {
		t.Fatalf("readGSMHeader psi=%d pti=%d, want 5/1", psi, pti)
	}
}

func TestExtendedProtocolDiscriminator(t *testing.T) {
	if epd, _ := ExtendedProtocolDiscriminator([]byte{EPD5GMM, 0x00, 0x41}); epd != EPD5GMM {
		t.Errorf("5GMM EPD = %#x, want %#x", epd, EPD5GMM)
	}

	if epd, _ := ExtendedProtocolDiscriminator([]byte{EPD5GSM, 0, 0, 0xC1}); epd != EPD5GSM {
		t.Errorf("5GSM EPD = %#x, want %#x", epd, EPD5GSM)
	}
}
