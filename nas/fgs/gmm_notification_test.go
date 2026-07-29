// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package fgs

import (
	"bytes"
	"testing"
)

func TestParseNotificationResponse(t *testing.T) {
	// Header (EPD, SHT+spare, message type) + PDU session status TLV (IEI 0x50,
	// len 2, PSI 1 and 3 active).
	var psi [16]bool

	psi[1] = true
	psi[3] = true

	status := mustBytes(PSIBitmap{PSI: psi}.MarshalBinary())

	b := append([]byte{uint8(EPD5GMM), 0x00, uint8(MsgNotificationResponse), ieiPDUSessionStatus, 0x02}, status...)

	resp, err := ParseNotificationResponse(b)
	if err != nil {
		t.Fatalf("ParseNotificationResponse: %v", err)
	}

	if resp.PDUSessionStatus == nil || resp.PDUSessionStatus.PSI != psi {
		t.Errorf("PDUSessionStatus = %v, want %v", resp.PDUSessionStatus, psi)
	}
}

func TestParseNotificationResponseNoStatus(t *testing.T) {
	resp, err := ParseNotificationResponse([]byte{uint8(EPD5GMM), 0x00, uint8(MsgNotificationResponse)})
	if err != nil {
		t.Fatalf("ParseNotificationResponse: %v", err)
	}

	if resp.PDUSessionStatus != nil {
		t.Errorf("PDUSessionStatus = %v, want nil", resp.PDUSessionStatus)
	}
}

func TestPSIRoundTrip(t *testing.T) {
	var psi [16]bool

	psi[1], psi[5], psi[15] = true, true, true

	got, err := ParsePSIBitmap(mustBytes(PSIBitmap{PSI: psi}.MarshalBinary()))
	if err != nil {
		t.Fatalf("ParsePSIBitmap: %v", err)
	}

	if got.PSI != psi {
		t.Errorf("PSI round-trip = %v, want %v", got.PSI, psi)
	}

	// A bitmap shorter than the mandatory two octets is malformed, not an empty
	// bitmap: decoding it as empty would report every session inactive.
	if _, err := ParsePSIBitmap([]byte{0x01}); err == nil {
		t.Error("ParsePSIBitmap(short): want error")
	}

	// Octets a later release adds survive a round-trip.
	extended := []byte{0x22, 0x80, 0xAA}

	ext, err := ParsePSIBitmap(extended)
	if err != nil {
		t.Fatalf("ParsePSIBitmap(extended): %v", err)
	}

	if out, err := ext.MarshalBinary(); err != nil || !bytes.Equal(out, extended) {
		t.Errorf("extended round-trip = %#x err %v, want %#x", out, err, extended)
	}
}
