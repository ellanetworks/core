// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"net/netip"
	"slices"
	"testing"

	"github.com/ellanetworks/core/internal/mme"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/nas/eps"
)

func TestBuildActivateDefaultESMDeliversPCSCFAddresses(t *testing.T) {
	v4a, v4b := netip.MustParseAddr("10.0.0.5"), netip.MustParseAddr("10.0.0.6")
	v6a := netip.MustParseAddr("2001:db8::5")
	both := nas.PCSCFRequest{IPv4: true, IPv6: true}

	cases := []struct {
		name    string
		pdnType eps.PDNType
		useEPCO bool
		request nas.PCSCFRequest
		want    []netip.Addr
	}{
		{"dual stack, PCO", eps.PDNTypeIPv4v6, false, both, []netip.Addr{v6a, v4a, v4b}},
		{"dual stack, ePCO", eps.PDNTypeIPv4v6, true, both, []netip.Addr{v6a, v4a, v4b}},
		{"IPv6 requested only", eps.PDNTypeIPv4v6, false, nas.PCSCFRequest{IPv6: true}, []netip.Addr{v6a}},
		{"IPv4 PDN drops IPv6", eps.PDNTypeIPv4, false, both, []netip.Addr{v4a, v4b}},
		{"IPv6 PDN drops IPv4", eps.PDNTypeIPv6, false, both, []netip.Addr{v6a}},
		{"not requested", eps.PDNTypeIPv4v6, false, nas.PCSCFRequest{}, nil},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := &mme.PdnConnection{Ebi: mme.DefaultERABID, Apn: "ims", PdnType: tc.pdnType, UeIP: netip.MustParseAddr("10.46.0.2")}
			bearer := models.EPSBearer{
				QoS:   models.EPSBearerQoS{QCI: 5, APNAMBR: models.Ambr{Downlink: models.MustParseBitRate("1 Mbps"), Uplink: models.MustParseBitRate("1 Mbps")}},
				PCSCF: []netip.Addr{v6a, v4a, v4b},
			}

			wire, err := buildActivateDefaultESM(p, bearer, 1, models.PlmnID{Mcc: "001", Mnc: "01"}, tc.useEPCO, nil, tc.request)
			if err != nil {
				t.Fatalf("buildActivateDefaultESM: %v", err)
			}

			act, err := eps.ParseActivateDefaultEPSBearerContextRequest(wire)
			if err != nil {
				t.Fatalf("parse: %v", err)
			}

			pco := act.ProtocolConfigurationOptions
			if tc.useEPCO {
				pco = act.ExtendedProtocolConfigurationOptions
			}

			var got []netip.Addr
			if pco != nil {
				got = pco.PCSCFAddresses()
			}

			if !slices.Equal(got, tc.want) {
				t.Fatalf("P-CSCF addresses = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestPCSCFRequestFromPCOsPrefersTheExtendedElement(t *testing.T) {
	pco := nas.NewRequestedProtocolConfigurationOptions(nas.PCOContainerPCSCFIPv4Address)
	epco := nas.NewRequestedProtocolConfigurationOptions(nas.PCOContainerPCSCFIPv6Address)

	if got, ok := pcscfRequestFromPCOs(&pco, &epco); !ok || got != (nas.PCSCFRequest{IPv6: true}) {
		t.Fatalf("pcscfRequestFromPCOs = %+v, %v", got, ok)
	}

	if got, ok := pcscfRequestFromPCOs(&pco, nil); !ok || got != (nas.PCSCFRequest{IPv4: true}) {
		t.Fatalf("pcscfRequestFromPCOs = %+v, %v", got, ok)
	}

	if _, ok := pcscfRequestFromPCOs(nil, nil); ok {
		t.Fatal("pcscfRequestFromPCOs reported a request without any PCO")
	}
}
