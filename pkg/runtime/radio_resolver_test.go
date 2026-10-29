// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package runtime

import (
	"net/netip"
	"testing"

	"github.com/ellanetworks/core/internal/upf"
)

func TestRadioResolverNamesEachAddressByItsRadio(t *testing.T) {
	addrA, addrB := netip.MustParseAddr("10.3.0.11"), netip.MustParseAddr("10.3.0.12")

	r := &radioResolver{
		addresses: func() map[netip.Addr]string {
			return map[netip.Addr]string{addrA: "gnb:001-01:000001@22", addrB: "enb:001-01:000002"}
		},
		names: func() map[string]string {
			return map[string]string{"gnb:001-01:000001@22": "hall"}
		},
	}

	got := r.RadiosByAddress()

	if want := (upf.Radio{ID: "gnb:001-01:000001@22", Name: "hall"}); got[addrA] != want {
		t.Errorf("%s = %+v, want %+v", addrA, got[addrA], want)
	}

	if want := (upf.Radio{ID: "enb:001-01:000002"}); got[addrB] != want {
		t.Errorf("%s = %+v, want the radio's ID with an empty name while it is not listed, %+v", addrB, got[addrB], want)
	}
}
