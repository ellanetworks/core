// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package eps

import (
	"testing"

	"github.com/ellanetworks/core/nas/common"
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

	header := []byte{0xc7, 0x05}
	integ := common.AESCMACIntegrity{}

	got, err := ServiceRequestShortMAC(header, key, 7, common.DirectionUplink, integ)
	if err != nil {
		t.Fatal(err)
	}

	// The short MAC is the two least significant octets of the full NAS-MAC over
	// the header at the same count/bearer/direction.
	full, err := integ.MAC(key, 7, nasBearer, common.DirectionUplink, header)
	if err != nil {
		t.Fatal(err)
	}

	if got != [2]byte{full[2], full[3]} {
		t.Fatalf("short MAC = %x, want %x", got, full[2:4])
	}
}

func TestServiceRejectMarshal(t *testing.T) {
	b, err := (&ServiceReject{Cause: 9}).Marshal()
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
