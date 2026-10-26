// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package diameter

import (
	"bytes"
	"encoding/binary"
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

	tests := map[string]struct {
		in   []byte
		want error
	}{
		"short header":   {valid[:10], ErrInvalidMessageLength},
		"version":        {badVersion, ErrUnsupportedVersion},
		"message length": {badLength, ErrInvalidMessageLength},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			if _, err := Unmarshal(tt.in); !errors.Is(err, tt.want) {
				t.Fatalf("err = %v, want %v", err, tt.want)
			}
		})
	}
}

func TestUnmarshalInvalidAVPLength(t *testing.T) {
	header := func(code uint32, flags uint8, length int, vendorID uint32) []byte {
		b := binary.BigEndian.AppendUint32(nil, code)
		b = append(b, flags, byte(length>>16), byte(length>>8), byte(length))

		if flags&AVPFlagVendor != 0 {
			b = binary.BigEndian.AppendUint32(b, vendorID)
		}

		return b
	}

	tests := map[string]struct {
		avps []byte
		want AVP
	}{
		"length below header": {
			append(header(AVPUserName, AVPFlagMandatory, 4, 0), 0, 0, 0, 0),
			AVP{Code: AVPUserName, Flags: AVPFlagMandatory},
		},
		"length beyond message": {
			append(header(AVPUserName, AVPFlagMandatory, 64, 0), 0, 0, 0, 0),
			AVP{Code: AVPUserName, Flags: AVPFlagMandatory},
		},
		"vendor length below header": {
			header(3113, AVPFlagVendor|AVPFlagMandatory, 8, 10415),
			AVP{Code: 3113, Flags: AVPFlagVendor | AVPFlagMandatory, VendorID: 10415},
		},
		"truncated header": {
			[]byte{0, 0, 0x0c, 0x29},
			AVP{Code: 3113},
		},
		"truncated vendor header": {
			header(3113, AVPFlagVendor, 64, 10415)[:8],
			AVP{Code: 3113, Flags: AVPFlagVendor},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			b, err := (&Message{CommandCode: CommandDeviceWatchdog}).Marshal()
			if err != nil {
				t.Fatal(err)
			}

			b = append(b, tt.avps...)
			b = append(b, make([]byte, (4-len(b)%4)%4)...)
			b[1], b[2], b[3] = byte(len(b)>>16), byte(len(b)>>8), byte(len(b))

			_, err = Unmarshal(b)

			var avpErr *AVPError
			if !errors.As(err, &avpErr) || avpErr.ResultCode != ResultInvalidAVPLength {
				t.Fatalf("err = %v, want an AVPError with result %d", err, ResultInvalidAVPLength)
			}

			if avpErr.AVPs[0].Code != tt.want.Code || avpErr.AVPs[0].Flags != tt.want.Flags ||
				avpErr.AVPs[0].VendorID != tt.want.VendorID || len(avpErr.AVPs[0].Data) != 0 {
				t.Fatalf("offending AVP = %+v, want %+v", avpErr.AVPs[0], tt.want)
			}
		})
	}
}

func TestGroupedInvalidAVPLengthWrapsOffendingAVP(t *testing.T) {
	proxyInfo := OctetString(AVPProxyInfo, AVPFlagMandatory, 0, []byte{0, 0, 0x01, 0x01, 0x40, 0, 0, 0xff})

	_, err := proxyInfo.Grouped()

	var avpErr *AVPError
	if !errors.As(err, &avpErr) || avpErr.ResultCode != ResultInvalidAVPLength {
		t.Fatalf("err = %v", err)
	}

	if avpErr.AVPs[0].Code != AVPProxyInfo || avpErr.AVPs[0].Flags != AVPFlagMandatory {
		t.Fatalf("parent = %+v", avpErr.AVPs[0])
	}

	got, err := avpErr.AVPs[0].Grouped()
	if err != nil || len(got) != 1 || got[0].Code != AVPHostIPAddress || got[0].Flags != AVPFlagMandatory || len(got[0].Data) != 0 {
		t.Fatalf("hierarchy = %+v, %v", got, err)
	}
}

func TestFailedAVPForInvalidLengthEncodesConsistently(t *testing.T) {
	failed := FailedAVP(AVP{Code: 3113, Flags: AVPFlagVendor | AVPFlagMandatory, VendorID: 10415})

	b := failed.appendTo(nil)
	if len(b) != 8+12 {
		t.Fatalf("Failed-AVP = %x", b)
	}

	if length := int(b[13])<<16 | int(b[14])<<8 | int(b[15]); length != 12 {
		t.Fatalf("inner AVP length = %d, want 12", length)
	}

	inner, err := failed.Grouped()
	if err != nil || len(inner) != 1 || inner[0].Code != 3113 || inner[0].VendorID != 10415 {
		t.Fatalf("Failed-AVP contents = %+v, %v", inner, err)
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

func TestUnmarshalledAVPDataDoesNotAlias(t *testing.T) {
	b, err := (&Message{AVPs: []AVP{
		OctetString(AVPUserName, AVPFlagMandatory, 0, []byte{1, 2, 3, 4}),
		UTF8String(AVPOriginHost, AVPFlagMandatory, 0, "host"),
		Grouped(AVPFailedAVP, AVPFlagMandatory, 0, OctetString(AVPUserName, 0, 0, []byte{5, 6, 7, 8}), UTF8String(AVPOriginRealm, 0, 0, "realm")),
	}}).Marshal()
	if err != nil {
		t.Fatal(err)
	}

	m, err := Unmarshal(b)
	if err != nil {
		t.Fatal(err)
	}

	_ = append(m.AVPs[0].Data, bytes.Repeat([]byte{0xff}, 32)...)

	if host, _ := m.Find(AVPOriginHost, 0); host.UTF8String() != "host" {
		t.Fatalf("append to one AVP overwrote the next: %+v", m.AVPs)
	}

	inner, err := m.AVPs[2].Grouped()
	if err != nil {
		t.Fatal(err)
	}

	_ = append(inner[0].Data, bytes.Repeat([]byte{0xff}, 32)...)

	if realm, _ := Find(inner, AVPOriginRealm, 0); realm.UTF8String() != "realm" {
		t.Fatalf("append to a grouped member overwrote the next: %+v", inner)
	}
}

func TestUnsigned64(t *testing.T) {
	a := Unsigned64(552, 0, 10415, 1<<40+7)

	if v, err := a.Unsigned64(); err != nil || v != 1<<40+7 || len(a.Data) != 8 {
		t.Fatalf("Unsigned64 = %d, %v", v, err)
	}

	if _, err := Unsigned32(552, 0, 10415, 1).Unsigned64(); err == nil {
		t.Fatal("4-octet AVP accepted as Unsigned64")
	}
}
