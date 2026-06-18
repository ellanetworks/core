// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package models_test

import (
	"bytes"
	"net/netip"
	"testing"

	"github.com/ellanetworks/core/internal/models"
)

func TestEncodeTransportLayerAddress(t *testing.T) {
	v4 := netip.AddrFrom4([4]byte{10, 3, 0, 2})
	v6 := netip.MustParseAddr("2001:db8:3::10")
	v6Octets := v6.As16()

	tests := []struct {
		name    string
		v4, v6  netip.Addr
		want    []byte
		wantErr bool
	}{
		{
			name: "ipv4 only is 4 octets",
			v4:   v4,
			want: []byte{10, 3, 0, 2},
		},
		{
			name: "ipv6 only is 16 octets",
			v6:   v6,
			want: v6Octets[:],
		},
		{
			name: "dual stack is ipv4 followed by ipv6",
			v4:   v4,
			v6:   v6,
			want: append([]byte{10, 3, 0, 2}, v6Octets[:]...),
		},
		{
			name:    "neither is an error",
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := models.EncodeTransportLayerAddress(tc.v4, tc.v6)
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected an error for an empty address pair")
				}

				return
			}

			if err != nil {
				t.Fatal(err)
			}

			if !bytes.Equal(got, tc.want) {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
		})
	}
}

func TestDecodeTransportLayerAddress(t *testing.T) {
	v4 := netip.AddrFrom4([4]byte{10, 3, 0, 2})
	v6 := netip.MustParseAddr("2001:db8:3::10")
	v6Octets := v6.As16()

	tests := []struct {
		name    string
		in      []byte
		wantV4  netip.Addr
		wantV6  netip.Addr
		wantErr bool
	}{
		{name: "ipv4", in: []byte{10, 3, 0, 2}, wantV4: v4},
		{name: "ipv6", in: v6Octets[:], wantV6: v6},
		{name: "dual stack", in: append([]byte{10, 3, 0, 2}, v6Octets[:]...), wantV4: v4, wantV6: v6},
		{name: "bad length", in: []byte{1, 2, 3}, wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gotV4, gotV6, err := models.DecodeTransportLayerAddress(tc.in)
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected an error for a bad length")
				}

				return
			}

			if err != nil {
				t.Fatal(err)
			}

			if gotV4 != tc.wantV4 || gotV6 != tc.wantV6 {
				t.Fatalf("got v4=%v v6=%v, want v4=%v v6=%v", gotV4, gotV6, tc.wantV4, tc.wantV6)
			}
		})
	}
}
