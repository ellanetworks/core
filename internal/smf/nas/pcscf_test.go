// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas_test

import (
	"net/netip"
	"slices"
	"testing"

	"github.com/ellanetworks/core/internal/models"
	smfNas "github.com/ellanetworks/core/internal/smf/nas"
	"github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/nas/fgs"
)

func TestBuildGSMPDUSessionEstablishmentAccept_PCSCF(t *testing.T) {
	v4a, v4b := netip.MustParseAddr("10.0.0.5"), netip.MustParseAddr("10.0.0.6")
	v6a := netip.MustParseAddr("2001:db8::5")
	pcscf := []netip.Addr{v6a, v4a, v4b}
	both := nas.PCSCFRequest{IPv4: true, IPv6: true}

	cases := []struct {
		name        string
		sessionType fgs.PDUSessionType
		request     nas.PCSCFRequest
		pcscf       []netip.Addr
		want        []netip.Addr
	}{
		{"dual stack, both requested", fgs.PDUSessionTypeIPv4v6, both, pcscf, []netip.Addr{v6a, v4a, v4b}},
		{"dual stack, IPv4 requested", fgs.PDUSessionTypeIPv4v6, nas.PCSCFRequest{IPv4: true}, pcscf, []netip.Addr{v4a, v4b}},
		{"IPv4 session drops IPv6", fgs.PDUSessionTypeIPv4, both, pcscf, []netip.Addr{v4a, v4b}},
		{"IPv6 session drops IPv4", fgs.PDUSessionTypeIPv6, both, pcscf, []netip.Addr{v6a}},
		{"not requested", fgs.PDUSessionTypeIPv4v6, nas.PCSCFRequest{}, pcscf, nil},
		{"no addresses", fgs.PDUSessionTypeIPv4v6, both, nil, nil},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ambr := &models.Ambr{Uplink: models.MustParseBitRate("1 Gbps"), Downlink: models.MustParseBitRate("1 Gbps")}
			qos := &models.QosData{QFI: 1, Var5qi: 5}
			pco := &smfNas.ProtocolConfigurationOptions{PCSCFRequest: tc.request}
			addrs := &smfNas.PDUSessionAddresses{PDUSessionType: tc.sessionType}

			raw, err := smfNas.BuildGSMPDUSessionEstablishmentAccept(ambr, qos, 5, 1, &models.Snssai{Sst: 1}, "ims", pco, nil, tc.pcscf, 0, nil, addrs, nil, 0)
			if err != nil {
				t.Fatalf("build failed: %v", err)
			}

			acc, err := fgs.ParsePDUSessionEstablishmentAccept(raw)
			if err != nil {
				t.Fatalf("decode failed: %v", err)
			}

			var got []netip.Addr
			if acc.ExtendedPCO != nil {
				got = acc.ExtendedPCO.PCSCFAddresses()
			}

			if !slices.Equal(got, tc.want) {
				t.Fatalf("P-CSCF addresses = %v, want %v", got, tc.want)
			}
		})
	}
}
