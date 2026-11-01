// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package usim

import (
	"encoding/hex"
	"errors"
	"testing"
)

func mustHex(t *testing.T, s string) []byte {
	t.Helper()

	b, err := hex.DecodeString(s)
	if err != nil {
		t.Fatal(err)
	}

	return b
}

// sja5 is key set 1 of a sysmoISIM-SJA5: the packets of the tests that use it were exchanged with the card.
func sja5(t *testing.T) Keyset {
	return Keyset{
		Version: 1, Algorithm: TripleDES,
		KIc: mustHex(t, "2184A42A93F563DB248095A5F539D489"), KID: mustHex(t, "E45C662B528BB14A67F99FE70AB193C9"),
	}
}

// pysim3DES and pysimAES are key sets of the tests of pySim (tests/unittests/test_ota.py).
func pysim3DES(t *testing.T) Keyset {
	return Keyset{
		Version: 3, Algorithm: TripleDES,
		KIc: mustHex(t, "C21DD66ACAC13CB3BC8B331B24AFB57B"), KID: mustHex(t, "12110C78E678C25408233076AA033615"),
	}
}

func pysimAES(t *testing.T) Keyset {
	return Keyset{
		Version: 2, Algorithm: AES,
		KIc: mustHex(t, "200102030405060708090a0b0c0d0e0f"), KID: mustHex(t, "201102030405060708090a0b0c0d0e0f"),
	}
}

// The SPIs of the vectors: a ciphered command with a cryptographic checksum and no counter that asks for a PoR
// ciphered with a cryptographic checksum, signed asks for a PoR with a checksum, not ciphered, and plain for an
// unsecured PoR.
const (
	secured uint16 = 0x0619
	signed  uint16 = 0x0609
	plain   uint16 = 0x0601
)

func keysets(ks ...Keyset) map[uint8]Keyset {
	m := map[uint8]Keyset{}
	for _, k := range ks {
		m[k.Version] = k
	}

	return m
}

// TestUnpack checks that the card unpacks Command Packets of a real card's exchanges and of pySim.
func TestUnpack(t *testing.T) {
	cases := []struct {
		name    string
		keyset  Keyset
		packet  string
		tar     string
		counter uint64
		data    string
	}{
		{
			name: "SJA5, select DF.GSM", keyset: sja5(t), tar: "b00010", data: "a0a40000027f20a0c0000016",
			packet: "00281506191515b00010cf71695f686b9516cfee5d0194c605ac9e8baa3b77f0144614068eb3ab557f93",
		},
		{
			name: "SJA5, counter", keyset: sja5(t), tar: "b00010", counter: 100, data: "a0a40000027f20a0c0000016",
			packet: "00281516191515b000108fb60fad7125e0ee4811b11427719bef3eb8b1ef6ecb1b4f326272f85256243e",
		},
		{
			name: "SJA5, read EF.OPLMNwAcT", keyset: sja5(t), tar: "b00010", data: "a0a40000027f20a0a40000026f61a0b000000f",
			packet: "00301506191515b000109a46af5dbcd005a46055f55440c44512bd230ba12854b987522f687b50bc495d8638822e5edaef0e",
		},
		{
			name: "pySim AES", keyset: pysimAES(t), tar: "b00011", data: "00a40004023f00",
			packet: "00281506192222b00011e87cceebb2d93083011ce294f93fc4d8de80da1abae8c37ca3e72ec4432e5058",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c, err := unpack(append(append([]byte{}, commandUDH...), mustHex(t, tc.packet)...), keysets(tc.keyset))
			if err != nil {
				t.Fatal(err)
			}

			if hex.EncodeToString(c.tar[:]) != tc.tar || c.counter != tc.counter || hex.EncodeToString(c.data) != tc.data ||
				c.keyVersion() != tc.keyset.Version {
				t.Fatalf("command TAR %x counter %d data %x key set %d", c.tar, c.counter, c.data, c.keyVersion())
			}
		})
	}
}

// TestUnpackUnauthenticated checks the Command Packets whose Sending Entity the card cannot authenticate.
func TestUnpackUnauthenticated(t *testing.T) {
	good := "00281516191515b000108fb60fad7125e0ee4811b11427719bef3eb8b1ef6ecb1b4f326272f85256243e"

	tampered := mustHex(t, good)
	tampered[len(tampered)-1] ^= 0x01

	cases := map[string]struct {
		keysets map[uint8]Keyset
		packet  []byte
	}{
		"checksum mismatch": {keysets(sja5(t)), tampered},
		"unknown key set":   {keysets(pysim3DES(t)), mustHex(t, good)},
		// pySim's ciphered command without a checksum, of SPI '04 19', asks for a PoR with a checksum, against rule 1
		// of TS 102 225 clause 5.1.1.
		"PoR integrity not the command's": {keysets(pysim3DES(t)), mustHex(t, "00180d04193500b00000e3ec80a849b554421276af3883927c20")},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := unpack(append(append([]byte{}, commandUDH...), tc.packet...), tc.keysets); !errors.Is(err, errUnauthenticated) {
				t.Fatalf("unpack = %v, want %v", err, errUnauthenticated)
			}
		})
	}
}

// TestPack checks that the card's Response Packets are those of a real card and of pySim.
func TestPack(t *testing.T) {
	cases := []struct {
		name   string
		keyset Keyset
		spi    uint16
		tar    [3]byte
		data   string
		packet string
	}{
		{
			name: "SJA5, select DF.GSM", keyset: sja5(t), spi: secured, tar: [3]byte{0xb0, 0x00, 0x10},
			data:   "0290000000ffff7f2002000000000009b106350400838a838a",
			packet: "027100002c12b0001023dd41b55fb14f5e7726736aeb0bd69e4743f58b520e4f3bc42ca3f66b789b404fb60c953eff1625",
		},
		{
			name: "3DES ciphered", keyset: pysim3DES(t), spi: secured, tar: [3]byte{0xb0, 0x00, 0x11}, data: "01612f",
			packet: "027100001c12b000119660ebdb81be189b5e4389e9e7ab2bc0954f963ad869ed7c",
		},
		{
			name: "3DES signed", keyset: pysim3DES(t), spi: signed, tar: [3]byte{0xb0, 0x00, 0x11}, data: "01612f",
			packet: "027100001612b000110000000000000055f47118381175fb01612f",
		},
		{
			name: "3DES clear", keyset: pysim3DES(t), spi: plain, tar: [3]byte{0xb0, 0x00, 0x11}, data: "01612f",
			packet: "027100000e0ab000110000000000000001612f",
		},
		{
			name: "AES ciphered", keyset: pysimAES(t), spi: secured, tar: [3]byte{0xb0, 0x00, 0x11}, data: "016132",
			packet: "027100002412b00011ebc6b497e2cad7aedf36ace0e3a29b38853f0fe9ccde81913be5702b73abce1f",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := command{spi: tc.spi, tar: tc.tar, cipher: &tc.keyset, verified: &tc.keyset}

			got, err := response{tar: tc.tar, status: StatusPoROK, data: mustHex(t, tc.data)}.pack(c)
			if err != nil {
				t.Fatal(err)
			}

			if hex.EncodeToString(got) != tc.packet {
				t.Fatalf("packet %x, want %s", got, tc.packet)
			}
		})
	}
}

func TestCheckCounter(t *testing.T) {
	cases := []struct {
		name    string
		spi     uint16
		counter uint64
		last    uint64
		next    uint64
		status  Status
	}{
		{"no counter", 0x0619, 3, 5, 5, StatusPoROK},
		{"for information", 0x0e19, 3, 5, 5, StatusPoROK},
		{"higher", 0x1619, 9, 5, 9, StatusPoROK},
		{"not higher", 0x1619, 5, 5, 5, StatusCNTRLow},
		{"one more", 0x1e19, 6, 5, 6, StatusPoROK},
		{"more than one more", 0x1e19, 7, 5, 5, StatusCNTRHigh},
		{"blocked", 0x1619, maxCounter, maxCounter, maxCounter, StatusCNTRBlocked},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			next, status, _ := checkCounter(command{spi: tc.spi, counter: tc.counter}, tc.last)
			if next != tc.next || status != tc.status {
				t.Fatalf("counter %d status %#02x, want %d %#02x", next, byte(status), tc.next, byte(tc.status))
			}
		})
	}
}
