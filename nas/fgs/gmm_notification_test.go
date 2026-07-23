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

	status := PSIToBytes(psi)

	b := append([]byte{EPD5GMM, 0x00, uint8(MsgNotificationResponse), ieiPDUSessionStatus, 0x02}, status...)

	resp, err := ParseNotificationResponse(b)
	if err != nil {
		t.Fatalf("ParseNotificationResponse: %v", err)
	}

	if !bytes.Equal(resp.PDUSessionStatus, status) {
		t.Errorf("PDUSessionStatus = %x, want %x", resp.PDUSessionStatus, status)
	}

	if got := PSIFromBytes(resp.PDUSessionStatus); got != psi {
		t.Errorf("PSIFromBytes = %v, want %v", got, psi)
	}
}

func TestParseNotificationResponseNoStatus(t *testing.T) {
	resp, err := ParseNotificationResponse([]byte{EPD5GMM, 0x00, uint8(MsgNotificationResponse)})
	if err != nil {
		t.Fatalf("ParseNotificationResponse: %v", err)
	}

	if resp.PDUSessionStatus != nil {
		t.Errorf("PDUSessionStatus = %x, want nil", resp.PDUSessionStatus)
	}
}

func TestPSIRoundTrip(t *testing.T) {
	var psi [16]bool

	psi[1], psi[5], psi[15] = true, true, true

	if got := PSIFromBytes(PSIToBytes(psi)); got != psi {
		t.Errorf("PSI round-trip = %v, want %v", got, psi)
	}

	empty := [16]bool{}
	if got := PSIFromBytes([]byte{0x01}); got != empty {
		t.Errorf("PSIFromBytes(short) = %v, want all-false", got)
	}
}
