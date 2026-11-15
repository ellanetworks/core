// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ap

import (
	"bytes"
	"errors"
	"math/rand/v2"
	"net/netip"
	"testing"

	"github.com/ellanetworks/core/per"
)

func encodeTLAThenBit(t *testing.T, enc per.Encoding, a TransportLayerAddress) []byte {
	t.Helper()

	w := per.NewWriter()

	if err := a.MarshalPER(w, enc); err != nil {
		t.Fatal(err)
	}

	w.WriteBit(true)

	return w.Bytes()
}

func decodeTLAThenBit(t *testing.T, enc per.Encoding, b []byte) TransportLayerAddress {
	t.Helper()

	r := per.NewReader(b)

	var a TransportLayerAddress
	if err := a.UnmarshalPER(r, enc); err != nil {
		t.Fatal(err)
	}

	bit, err := r.ReadBit()
	if err != nil || !bit {
		t.Fatalf("trailing bit = %v, %v: the address consumed the wrong number of bits", bit, err)
	}

	return a
}

func TestTransportLayerAddressRoundTripsEveryLength(t *testing.T) {
	rng := rand.New(rand.NewPCG(1, 2))

	for nbits := 1; nbits <= 200; nbits++ {
		for range 8 {
			b := make([]byte, (nbits+7)/8)
			for i := range b {
				b[i] = byte(rng.Uint32())
			}

			a, ok := TransportLayerAddressFromBits(b, nbits)
			if !ok {
				t.Fatalf("%d bits: rejected", nbits)
			}

			if a.Bits() != nbits {
				t.Fatalf("Bits() = %d, want %d", a.Bits(), nbits)
			}

			for _, enc := range []per.Encoding{per.Aligned, per.Unaligned} {
				w := per.NewWriter()
				if err := per.EncodeBitString(w, enc, 1, 160, true, true, true, b, nbits); err != nil {
					t.Fatal(err)
				}

				w.WriteBit(true)
				want := w.Bytes()

				got := encodeTLAThenBit(t, enc, a)
				if !bytes.Equal(got, want) {
					t.Fatalf("%d bits %v: encoded %x, want %x", nbits, enc, got, want)
				}

				back := decodeTLAThenBit(t, enc, want)
				if back != a {
					t.Fatalf("%d bits %v: decoded %v, want %v", nbits, enc, back, a)
				}

				if again := encodeTLAThenBit(t, enc, back); !bytes.Equal(again, want) {
					t.Fatalf("%d bits %v: re-encoded %x, want %x", nbits, enc, again, want)
				}
			}
		}
	}
}

func TestTransportLayerAddressIgnoresBitsPastTheLength(t *testing.T) {
	a, _ := TransportLayerAddressFromBits([]byte{0xff, 0xff}, 9)
	b, _ := TransportLayerAddressFromBits([]byte{0xff, 0x80}, 9)

	if a != b {
		t.Fatalf("%v != %v", a, b)
	}

	if !bytes.Equal(a.Bytes(), []byte{0xff, 0x80}) {
		t.Fatalf("Bytes() = %x", a.Bytes())
	}
}

func TestTransportLayerAddressFromOctets(t *testing.T) {
	b := []byte{10, 0, 0, 1, 7}

	a := TransportLayerAddressFromOctets(b)
	b[0] = 11

	if a.Bits() != 40 || !bytes.Equal(a.Bytes(), []byte{10, 0, 0, 1, 7}) {
		t.Fatalf("got %d bits %x", a.Bits(), a.Bytes())
	}

	if v4, _ := TransportLayerAddressFrom(netip.MustParseAddr("10.0.0.1")); TransportLayerAddressFromOctets([]byte{10, 0, 0, 1}) != v4 {
		t.Fatal("4 octets are not the IPv4 address")
	}
}

func TestTransportLayerAddressFromBitsRejectsShortBuffers(t *testing.T) {
	for _, tc := range []struct {
		b     []byte
		nbits int
	}{{nil, 1}, {[]byte{1}, 9}, {[]byte{1}, -1}} {
		if _, ok := TransportLayerAddressFromBits(tc.b, tc.nbits); ok {
			t.Errorf("TransportLayerAddressFromBits(%x, %d) accepted", tc.b, tc.nbits)
		}
	}
}

func TestTransportLayerAddressConstructors(t *testing.T) {
	v4 := netip.MustParseAddr("10.0.0.1")
	v6 := netip.MustParseAddr("2001:db8::1")
	mapped := netip.MustParseAddr("::ffff:10.0.0.1")

	tests := []struct {
		name       string
		build      func() (TransportLayerAddress, error)
		bits       int
		ipv4, ipv6 netip.Addr
		octets     []byte
	}{
		{"ipv4", func() (TransportLayerAddress, error) { return TransportLayerAddressFrom(v4) }, 32, v4, netip.Addr{}, []byte{10, 0, 0, 1}},
		{"ipv4 mapped in ipv6 is unmapped", func() (TransportLayerAddress, error) { return TransportLayerAddressFrom(mapped) }, 32, v4, netip.Addr{}, []byte{10, 0, 0, 1}},
		{"ipv4 from a 16-octet slice", func() (TransportLayerAddress, error) {
			return TransportLayerAddressFrom(netip.AddrFrom16(mapped.As16()))
		}, 32, v4, netip.Addr{}, []byte{10, 0, 0, 1}},
		{"ipv6", func() (TransportLayerAddress, error) { return TransportLayerAddressFrom(v6) }, 128, netip.Addr{}, v6, v6.AsSlice()},
		{"pair ipv4 only", func() (TransportLayerAddress, error) { return TransportLayerAddressFromAddrs(v4, netip.Addr{}) }, 32, v4, netip.Addr{}, []byte{10, 0, 0, 1}},
		{"pair mapped ipv4 only", func() (TransportLayerAddress, error) { return TransportLayerAddressFromAddrs(mapped, netip.Addr{}) }, 32, v4, netip.Addr{}, []byte{10, 0, 0, 1}},
		{"pair ipv6 only", func() (TransportLayerAddress, error) { return TransportLayerAddressFromAddrs(netip.Addr{}, v6) }, 128, netip.Addr{}, v6, v6.AsSlice()},
		{"dual puts ipv4 first", func() (TransportLayerAddress, error) { return TransportLayerAddressFromAddrs(v4, v6) }, 160, v4, v6, append([]byte{10, 0, 0, 1}, v6.AsSlice()...)},
		{"dual unmaps ipv4", func() (TransportLayerAddress, error) { return TransportLayerAddressFromAddrs(mapped, v6) }, 160, v4, v6, append([]byte{10, 0, 0, 1}, v6.AsSlice()...)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a, err := tt.build()
			if err != nil {
				t.Fatal(err)
			}

			if a.Bits() != tt.bits || a.IPv4() != tt.ipv4 || a.IPv6() != tt.ipv6 || !bytes.Equal(a.Bytes(), tt.octets) || !a.Valid() {
				t.Fatalf("got bits %d ipv4 %v ipv6 %v octets %x valid %v", a.Bits(), a.IPv4(), a.IPv6(), a.Bytes(), a.Valid())
			}

			b, ok := TransportLayerAddressFromBits(tt.octets, tt.bits)
			if !ok || a != b {
				t.Fatalf("constructor %v != bit form %v", a, b)
			}
		})
	}
}

func TestTransportLayerAddressConstructorsRejectMisplacedFamilies(t *testing.T) {
	v4 := netip.MustParseAddr("10.0.0.1")
	v6 := netip.MustParseAddr("2001:db8::1")
	mapped := netip.MustParseAddr("::ffff:10.0.0.1")

	for _, tc := range []struct {
		name       string
		ipv4, ipv6 netip.Addr
	}{
		{"neither", netip.Addr{}, netip.Addr{}},
		{"ipv6 in the ipv4 slot", v6, netip.Addr{}},
		{"ipv4 in the ipv6 slot", netip.Addr{}, v4},
		{"mapped ipv4 in the ipv6 slot", v4, mapped},
	} {
		if a, err := TransportLayerAddressFromAddrs(tc.ipv4, tc.ipv6); !errors.Is(err, ErrTransportLayerAddress) {
			t.Errorf("%s: got %v, %v", tc.name, a, err)
		}
	}

	if a, err := TransportLayerAddressFrom(netip.Addr{}); !errors.Is(err, ErrTransportLayerAddress) {
		t.Errorf("zero Addr: got %v, %v", a, err)
	}
}

func TestTransportLayerAddressValid(t *testing.T) {
	for nbits := 0; nbits <= 200; nbits++ {
		a, _ := TransportLayerAddressFromBits(make([]byte, 25), nbits)

		want := nbits == 32 || nbits == 128 || nbits == 160
		if a.Valid() != want {
			t.Errorf("%d bits: Valid() = %v, want %v", nbits, a.Valid(), want)
		}

		if a.IPv4().IsValid() != (nbits == 32 || nbits == 160) {
			t.Errorf("%d bits: IPv4() = %v", nbits, a.IPv4())
		}

		if a.IPv6().IsValid() != (nbits == 128 || nbits == 160) {
			t.Errorf("%d bits: IPv6() = %v", nbits, a.IPv6())
		}
	}
}

func TestTransportLayerAddressAccessorsKeepWireFamily(t *testing.T) {
	mapped := netip.MustParseAddr("::ffff:10.0.0.1")

	a, _ := TransportLayerAddressFromBits(mapped.AsSlice(), 128)
	if a.IPv4().IsValid() || a.IPv6() != mapped {
		t.Fatalf("a 128-bit address is an IPv6 address: IPv4() = %v, IPv6() = %v", a.IPv4(), a.IPv6())
	}
}

func TestTransportLayerAddressKeepsOddLengths(t *testing.T) {
	for _, enc := range []per.Encoding{per.Aligned, per.Unaligned} {
		w := per.NewWriter()
		if err := per.EncodeBitString(w, enc, 1, 160, true, true, true, []byte{10, 0, 0, 1, 0x80}, 33); err != nil {
			t.Fatal(err)
		}

		w.WriteBit(true)
		wire := w.Bytes()

		a := decodeTLAThenBit(t, enc, wire)
		if a.Bits() != 33 {
			t.Fatalf("%v: 33-bit address decoded as %d bits", enc, a.Bits())
		}

		if got := encodeTLAThenBit(t, enc, a); !bytes.Equal(got, wire) {
			t.Fatalf("%v: 33-bit address re-encoded as %x, want %x", enc, got, wire)
		}

		if a.Valid() || a.IPv4().IsValid() {
			t.Fatalf("%v: 33 bits is not a TS 38.414 address", enc)
		}

		w = per.NewWriter()
		if err := per.EncodeBitString(w, enc, 1, 160, true, true, true, []byte{10, 0, 0, 0}, 31); err != nil {
			t.Fatal(err)
		}

		w.WriteBit(true)

		short := decodeTLAThenBit(t, enc, w.Bytes())
		if short.Bits() != 31 || short.Valid() || short.IPv4().IsValid() {
			t.Fatalf("%v: 31-bit address read as IPv4 %v", enc, short.IPv4())
		}
	}
}

func TestTransportLayerAddressKeepsExtensionLengths(t *testing.T) {
	b := bytes.Repeat([]byte{0xa5}, 32)

	for _, nbits := range []int{0, 161, 200, 255} {
		for _, enc := range []per.Encoding{per.Aligned, per.Unaligned} {
			a, _ := TransportLayerAddressFromBits(b, nbits)

			got := encodeTLAThenBit(t, enc, a)
			if back := decodeTLAThenBit(t, enc, got); back != a {
				t.Fatalf("%d bits %v: decoded %v, want %v", nbits, enc, back, a)
			}

			if got[0]&0x80 == 0 {
				t.Fatalf("%d bits %v: the extension bit is not set", nbits, enc)
			}
		}
	}
}

func TestTransportLayerAddressString(t *testing.T) {
	v4, _ := TransportLayerAddressFrom(netip.MustParseAddr("10.0.0.1"))
	dual, _ := TransportLayerAddressFromAddrs(netip.MustParseAddr("10.0.0.1"), netip.MustParseAddr("2001:db8::1"))
	odd, _ := TransportLayerAddressFromBits([]byte{0xab, 0xc0}, 10)

	for _, tc := range []struct {
		a    TransportLayerAddress
		want string
	}{{v4, "10.0.0.1"}, {dual, "10.0.0.1,2001:db8::1"}, {odd, "abc0/10"}, {TransportLayerAddress{}, "/0"}} {
		if got := tc.a.String(); got != tc.want {
			t.Errorf("String() = %q, want %q", got, tc.want)
		}
	}
}
