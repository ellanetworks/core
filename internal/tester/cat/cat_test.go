// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package cat

import (
	"bytes"
	"testing"
)

func TestTLVRoundTrip(t *testing.T) {
	for _, n := range []int{0, 0x7f, 0x80, 0xff} {
		value := bytes.Repeat([]byte{0xaa}, n)

		tag, got, rest, err := ReadTLV(append(AppendTLV(nil, TagSMSTPDU, value), 0x01))
		if err != nil || tag != TagSMSTPDU || !bytes.Equal(got, value) || !bytes.Equal(rest, []byte{0x01}) {
			t.Fatalf("%d octets: tag %#02x value %d octets rest %x: %v", n, tag, len(got), rest, err)
		}
	}

	for _, b := range [][]byte{{0x0b}, {0x0b, 0x81, 0x7f}, {0x0b, 0x82, 0x00, 0x01}, {0x0b, 0x02, 0x00}} {
		if _, _, _, err := ReadTLV(b); err == nil {
			t.Errorf("%x accepted", b)
		}
	}
}

func TestReadObjects(t *testing.T) {
	b := AppendTLV(nil, TagDeviceIdentities|ComprehensionRequired, []byte{DeviceNetwork, DeviceUICC})
	b = AppendTLV(b, TagDeviceIdentities, []byte{DeviceTerminal, DeviceUICC})
	b = AppendTLV(b, TagAddress, []byte{0x91})

	objects, err := ReadObjects(b)
	if err != nil {
		t.Fatal(err)
	}

	if v, _ := objects.Get(TagDeviceIdentities); !bytes.Equal(v, []byte{DeviceNetwork, DeviceUICC}) {
		t.Fatalf("device identities %x, want the first instance", v)
	}

	if v, ok := objects.Get(TagAddress | ComprehensionRequired); !ok || !bytes.Equal(v, []byte{0x91}) {
		t.Fatalf("address %x", v)
	}
}
