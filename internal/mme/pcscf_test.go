// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"context"
	"net/netip"
	"slices"
	"testing"

	"github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/nas/eps"
)

func tenPCSCFs() []netip.Addr {
	var addrs []netip.Addr

	for i := range 5 {
		addrs = append(addrs, netip.AddrFrom4([4]byte{10, 0, 0, byte(i + 1)}), netip.AddrFrom16([16]byte{0x20, 0x01, 0x0d, 0xb8, 15: byte(i + 1)}))
	}

	return addrs
}

func TestWithPCSCFKeepsTheUnextendedOptionsWithin253Octets(t *testing.T) {
	addrs := tenPCSCFs()
	both := nas.PCSCFRequest{IPv4: true, IPv6: true}

	base := nas.NewProtocolConfigurationOptions(nas.DNSServers(netip.MustParseAddr("2001:db8::53")), 1500)
	base.Containers = append(base.Containers, nas.PCOContainer{ID: 0x0005, Content: make([]byte, 120)})

	got := WithPCSCF(context.Background(), base, addrs, both, eps.PDNTypeIPv4v6, false)
	if !got.FitsUnextended() {
		t.Fatal("the protocol configuration options do not fit 253 octets")
	}

	kept := got.PCSCFAddresses()
	if len(kept) == 0 || len(kept) == len(addrs) || !slices.Equal(kept, addrs[:len(kept)]) {
		t.Fatalf("kept %v, want a proper prefix of %v", kept, addrs)
	}

	if extended := WithPCSCF(context.Background(), base, addrs, both, eps.PDNTypeIPv4v6, true); !slices.Equal(extended.PCSCFAddresses(), addrs) {
		t.Fatalf("extended options carry %v, want all of %v", extended.PCSCFAddresses(), addrs)
	}

	if all := WithPCSCF(context.Background(), nas.NewProtocolConfigurationOptions(nil, 1500), addrs, both, eps.PDNTypeIPv4v6, false); !slices.Equal(all.PCSCFAddresses(), addrs) {
		t.Fatalf("ten P-CSCF addresses on their own carry %v, want all of them", all.PCSCFAddresses())
	}
}
