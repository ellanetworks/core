// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package fgs

import (
	"bytes"
	"errors"
	"testing"

	"github.com/ellanetworks/core/nas"
)

func TestSecurityProtectedMessageRoundTrip(t *testing.T) {
	m := &SecurityProtectedMessage{
		SecurityHeaderType: SHTIntegrityProtectedCiphered,
		MAC:                [4]byte{0x11, 0x22, 0x33, 0x44},
		SequenceNumber:     0x2A,
		UnverifiedPayload:  []byte{0xDE, 0xAD, 0xBE, 0xEF},
	}

	b, err := m.MarshalBinary()
	if err != nil {
		t.Fatalf("MarshalBinary: %v", err)
	}

	// Wire layout (TS 24.501 §9.1.1): EPD | SHT+spare | MAC(4) | SN | payload = 7-octet header.
	want := []byte{uint8(EPD5GMM), 0x02, 0x11, 0x22, 0x33, 0x44, 0x2A, 0xDE, 0xAD, 0xBE, 0xEF}
	if !bytes.Equal(b, want) {
		t.Fatalf("MarshalBinary = %#x, want %#x", b, want)
	}

	got, err := ParseSecurityProtectedMessage(b)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	if got.SecurityHeaderType != m.SecurityHeaderType || got.SequenceNumber != m.SequenceNumber ||
		got.MAC != m.MAC || !bytes.Equal(got.UnverifiedPayload, m.UnverifiedPayload) {
		t.Fatalf("round-trip mismatch: got %+v, want %+v", got, m)
	}
}

func TestParseSecurityProtectedRejects(t *testing.T) {
	// Plain (SHT 0) is not a security-protected message.
	plain := []byte{uint8(EPD5GMM), 0x00, 0, 0, 0, 0, 0}
	if _, err := ParseSecurityProtectedMessage(plain); !errors.Is(err, ErrNotProtected) {
		t.Errorf("plain message: got %v, want ErrNotProtected", err)
	}

	// Wrong EPD (5GSM) in the outer wrapper.
	wrong := []byte{uint8(EPD5GSM), 0x01, 0, 0, 0, 0, 0}
	if _, err := ParseSecurityProtectedMessage(wrong); !errors.Is(err, ErrNotGMM) {
		t.Errorf("wrong EPD: got %v, want ErrNotGMM", err)
	}

	// Truncated before the sequence number.
	if _, err := ParseSecurityProtectedMessage([]byte{uint8(EPD5GMM), 0x01, 0, 0, 0, 0}); err == nil {
		t.Error("truncated message: expected error, got nil")
	}
}

func TestPlainMMHeader(t *testing.T) {
	w := nas.NewWriter(nil)

	writeGMMHeader(w, MsgIdentityRequest)
	w.U8(0xAB) // one IE octet

	b, err := w.Bytes()
	if err != nil {
		t.Fatalf("Writer: %v", err)
	}

	want := []byte{uint8(EPD5GMM), 0x00, uint8(MsgIdentityRequest), 0xAB}
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

	if err := readGMMHeader(nas.NewReader(b), MsgIdentityRequest); err != nil {
		t.Fatalf("readGMMHeader: %v", err)
	}

	if err := readGMMHeader(nas.NewReader(b), MsgRegistrationRequest); !errors.Is(err, ErrWrongMessageType) {
		t.Errorf("readGMMHeader wrong type: got %v, want ErrWrongMessageType", err)
	}
}

func TestPeekMessageTypeRejectsProtected(t *testing.T) {
	protected := []byte{uint8(EPD5GMM), 0x01, uint8(MsgRegistrationRequest)}
	if _, err := PeekMessageType(protected); !errors.Is(err, ErrNotPlain) {
		t.Errorf("got %v, want ErrNotPlain", err)
	}
}

func TestPlainSMHeader(t *testing.T) {
	w := nas.NewWriter(nil)

	writeGSMHeader(w, 5, 1, MsgPDUSessionEstablishmentRequest)
	w.U8(0xCD)

	b, err := w.Bytes()
	if err != nil {
		t.Fatalf("Writer: %v", err)
	}

	want := []byte{uint8(EPD5GSM), 5, 1, uint8(MsgPDUSessionEstablishmentRequest), 0xCD}
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

	psi, pti, err := readGSMHeader(nas.NewReader(b), MsgPDUSessionEstablishmentRequest)
	if err != nil {
		t.Fatalf("readGSMHeader: %v", err)
	}

	if psi != 5 || pti != 1 {
		t.Fatalf("readGSMHeader psi=%d pti=%d, want 5/1", psi, pti)
	}
}

func TestPeekSecurityHeaderType(t *testing.T) {
	if sht, err := PeekSecurityHeaderType([]byte{uint8(EPD5GMM), 0x00, uint8(MsgRegistrationRequest)}); err != nil || sht != SHTPlain {
		t.Errorf("plain: sht=%d err=%v", sht, err)
	}

	if sht, err := PeekSecurityHeaderType([]byte{uint8(EPD5GMM), uint8(SHTIntegrityProtectedCiphered), 0, 0, 0, 0, 0}); err != nil || sht != SHTIntegrityProtectedCiphered {
		t.Errorf("ciphered: sht=%d err=%v", sht, err)
	}

	if _, err := PeekSecurityHeaderType([]byte{uint8(EPD5GMM)}); err == nil {
		t.Error("too short: want error")
	}
}

func TestPeekProtocolDiscriminator(t *testing.T) {
	if epd, _ := PeekProtocolDiscriminator([]byte{uint8(EPD5GMM), 0x00, 0x41}); epd != EPD5GMM {
		t.Errorf("5GMM EPD = %#x, want %#x", uint8(epd), uint8(EPD5GMM))
	}

	if epd, _ := PeekProtocolDiscriminator([]byte{uint8(EPD5GSM), 0, 0, 0xC1}); epd != EPD5GSM {
		t.Errorf("5GSM EPD = %#x, want %#x", uint8(epd), uint8(EPD5GSM))
	}

	if _, err := PeekProtocolDiscriminator(nil); err == nil {
		t.Error("empty: want error")
	}
}
