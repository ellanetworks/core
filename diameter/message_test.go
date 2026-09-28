// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package diameter

import (
	"bytes"
	"encoding/hex"
	"errors"
	"net/netip"
	"testing"
	"time"
)

func TestMessageMarshalLayout(t *testing.T) {
	m := &Message{
		Flags:         FlagRequest | FlagProxiable,
		CommandCode:   8388645,
		ApplicationID: 16777313,
		HopByHopID:    0x01020304,
		EndToEndID:    0x05060708,
		AVPs: []AVP{
			UTF8String(AVPOriginHost, AVPFlagMandatory, 0, "smsc"),
			Unsigned32(3113, AVPFlagMandatory, 10415, 7),
			OctetString(AVPSessionID, AVPFlagMandatory, 0, []byte{0xaa}),
		},
	}

	b, err := m.Marshal()
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	want, _ := hex.DecodeString("" +
		"0100003c" + "c0800025" + "01000061" + "01020304" + "05060708" +
		"00000108" + "4000000c" + "736d7363" +
		"00000c29" + "c0000010" + "000028af" + "00000007" +
		"00000107" + "40000009" + "aa000000")
	if !bytes.Equal(b, want) {
		t.Fatalf("Marshal = %x, want %x", b, want)
	}

	got, err := Unmarshal(b)
	if err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if got.Flags != m.Flags || got.CommandCode != m.CommandCode || got.ApplicationID != m.ApplicationID ||
		got.HopByHopID != m.HopByHopID || got.EndToEndID != m.EndToEndID || len(got.AVPs) != 3 {
		t.Fatalf("Unmarshal = %+v", got)
	}

	if v, err := got.AVPs[1].Unsigned32(); err != nil || v != 7 || got.AVPs[1].VendorID != 10415 {
		t.Fatalf("vendor AVP = %+v (%v)", got.AVPs[1], err)
	}

	if !bytes.Equal(got.AVPs[2].Data, []byte{0xaa}) {
		t.Fatalf("octet string = %x", got.AVPs[2].Data)
	}
}

func TestUnmarshalErrors(t *testing.T) {
	valid, err := (&Message{CommandCode: CommandDeviceWatchdog, AVPs: []AVP{
		UTF8String(AVPOriginHost, AVPFlagMandatory, 0, "a"),
	}}).Marshal()
	if err != nil {
		t.Fatal(err)
	}

	badVersion := append([]byte(nil), valid...)
	badVersion[0] = 2

	badLength := append([]byte(nil), valid...)
	badLength[3]++

	badAVPLength := append([]byte(nil), valid...)
	badAVPLength[27] = 0xff

	tests := map[string]struct {
		in   []byte
		want error
	}{
		"short header":   {valid[:10], ErrInvalidMessageLength},
		"version":        {badVersion, ErrUnsupportedVersion},
		"message length": {badLength, ErrInvalidMessageLength},
		"avp length":     {badAVPLength, ErrInvalidAVPLength},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			if _, err := Unmarshal(tt.in); !errors.Is(err, tt.want) {
				t.Fatalf("err = %v, want %v", err, tt.want)
			}
		})
	}
}

func TestMarshalRejectsVendorIDWithoutFlag(t *testing.T) {
	m := &Message{AVPs: []AVP{{Code: 1, VendorID: 10415, Data: []byte{0}}}}
	if _, err := m.Marshal(); err == nil {
		t.Fatal("expected an error")
	}
}

func TestAddressAVP(t *testing.T) {
	for _, s := range []string{"192.0.2.1", "2001:db8::1"} {
		addr := netip.MustParseAddr(s)

		got, err := Address(AVPHostIPAddress, AVPFlagMandatory, 0, addr).Address()
		if err != nil || got != addr {
			t.Fatalf("Address(%s) = %s, %v", s, got, err)
		}
	}

	if _, err := (AVP{Code: AVPHostIPAddress, Data: []byte{0, 1, 1}}).Address(); err == nil {
		t.Fatal("expected an error for a truncated IPv4 address")
	}
}

func TestGroupedAVP(t *testing.T) {
	g := Grouped(AVPVendorSpecificApplicationID, AVPFlagMandatory, 0,
		Unsigned32(AVPVendorID, AVPFlagMandatory, 0, 10415),
		Unsigned32(AVPAuthApplicationID, AVPFlagMandatory, 0, 16777313),
	)

	inner, err := g.Grouped()
	if err != nil {
		t.Fatalf("Grouped: %v", err)
	}

	a, ok := Find(inner, AVPAuthApplicationID, 0)
	if !ok {
		t.Fatal("Auth-Application-Id missing")
	}

	if v, _ := a.Unsigned32(); v != 16777313 {
		t.Fatalf("Auth-Application-Id = %d", v)
	}
}

func TestUnsigned32RejectsWrongLength(t *testing.T) {
	if _, err := (AVP{Data: []byte{1, 2}}).Unsigned32(); err == nil {
		t.Fatal("expected an error")
	}
}

func TestTimeAVP(t *testing.T) {
	at := time.Date(2026, 9, 27, 12, 0, 0, 0, time.UTC)

	a := Time(3307, AVPFlagMandatory, 10415, at)
	if !bytes.Equal(a.Data, []byte{0xee, 0x63, 0x83, 0xc0}) {
		t.Fatalf("Time data = %x", a.Data)
	}

	got, err := a.Time()
	if err != nil || !got.Equal(at) {
		t.Fatalf("Time() = %v, %v", got, err)
	}

	if _, err := (AVP{Data: []byte{1}}).Time(); err == nil {
		t.Fatal("expected an error for a short Time")
	}
}
