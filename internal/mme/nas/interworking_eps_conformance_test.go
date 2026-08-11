// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"bytes"
	"net/netip"
	"testing"

	"github.com/ellanetworks/core/internal/mme"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/nas/eps"
)

// Conformance tests for interworking without N26
func TestEPSNetworkFeatureSupportNeverEncodesIWKN26(t *testing.T) {
	const (
		epcoOctet4 = 0x08 // octet 4, bit 4
	)

	for _, tc := range []struct {
		name string
		// UE network capability octets 7.. — octet 8 bit 8 is ePCO, octet 9 bit 6 is
		// N1 mode. The IWK N26 bit (octet 4, bit 7) is never set: it means
		// interworking *without* N26, and this MME supports N26.
		rest      []byte
		wantOctet byte
	}{
		{"neither indicated", []byte{0x00, 0x00, 0x00}, 0x00},
		{"N1 mode only", []byte{0x00, 0x00, 0x20}, 0x00},
		{"ePCO only", []byte{0x00, 0x80, 0x00}, epcoOctet4},
		{"both", []byte{0x00, 0x80, 0x20}, epcoOctet4},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := &mme.MME{}

			nfs := m.NetworkFeatureSupport(eps.UENetworkCapability{Rest: tc.rest})

			raw, err := nfs.MarshalBinary()
			if err != nil {
				t.Fatalf("encode EPS network feature support: %v", err)
			}

			if tc.wantOctet == 0 {
				if len(raw) > 1 && raw[1] != 0 {
					t.Fatalf("octet 4 = %#02x, want no interworking bits set", raw[1])
				}

				return
			}

			if len(raw) < 2 {
				t.Fatalf("encoded %d octets, want octet 4 present to carry %#02x", len(raw), tc.wantOctet)
			}

			if raw[1] != tc.wantOctet {
				t.Errorf("octet 4 = %#02x, want %#02x", raw[1], tc.wantOctet)
			}
		})
	}
}

// TS 24.301 §9.9.4.14
func TestEPSRequestTypeHandoverCodePoint(t *testing.T) {
	for _, tc := range []struct {
		name string
		in   eps.RequestType
		want byte
	}{
		{"handover", eps.RequestTypeHandover, 0x02},
		{"initial request", eps.RequestTypeInitialRequest, 0x01},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := &eps.PDNConnectivityRequest{
				EPSBearerIdentity: 0,
				PTI:               1,
				RequestType:       tc.in,
				PDNType:           eps.PDNTypeIPv4v6,
			}

			raw, err := req.MarshalBinary()
			if err != nil {
				t.Fatalf("encode PDN CONNECTIVITY REQUEST: %v", err)
			}

			if got := raw[3] & 0x07; got != tc.want {
				t.Errorf("octet 4 bits 1-3 = %#03b, want %#03b", got, tc.want)
			}

			if got := raw[3] >> 4 & 0x07; got != byte(eps.PDNTypeIPv4v6) {
				t.Errorf("octet 4 bits 5-7 = %#03b, want the PDN type %#03b: request type must not overrun it", got, byte(eps.PDNTypeIPv4v6))
			}

			back, err := eps.ParsePDNConnectivityRequest(raw)
			if err != nil {
				t.Fatalf("parse the encoded request: %v", err)
			}

			if back.RequestType != tc.in {
				t.Errorf("decoded request type = %d, want %d", back.RequestType, tc.in)
			}
		})
	}
}

// TS 24.008 §10.5.6.3
func TestEPSProtocolConfigurationContainerDirections(t *testing.T) {
	idContainer := []nas.PCOContainer{{ID: nas.PCOContainerPDUSessionID, Content: []byte{7}}}

	for _, tc := range []struct {
		name string
		in   []nas.PCOContainer
		want []byte
	}{
		{"PDU session ID", idContainer, []byte{0x00, 0x1A, 0x01, 0x07}},
		{"S-NSSAI", []nas.PCOContainer{{ID: nas.PCOContainerSNSSAI, Content: []byte{0x01}}}, []byte{0x00, 0x1B, 0x01, 0x01}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			pco := nas.ProtocolConfigurationOptions{Direction: nas.PCOMSToNetwork, Containers: tc.in}

			raw, err := pco.MarshalBinary()
			if err != nil {
				t.Fatalf("encode the protocol configuration options: %v", err)
			}

			if !bytes.Contains(raw, tc.want) {
				t.Errorf("encoded % x carries no container % x", raw, tc.want)
			}
		})
	}

	uplink := nas.ProtocolConfigurationOptions{Direction: nas.PCOMSToNetwork, Containers: idContainer}
	if got := pduSessionIDFromPCOs(&uplink, nil); got != 7 {
		t.Errorf("PDU session identity from the MS-to-network PCO = %d, want 7", got)
	}

	downlink := nas.ProtocolConfigurationOptions{Direction: nas.PCONetworkToMS, Containers: idContainer}
	if got := pduSessionIDFromPCOs(&downlink, nil); got != 0 {
		t.Errorf("PDU session identity read from a network-to-MS PCO = %d, want 0: 001AH is reserved in that direction", got)
	}

	epco := nas.ProtocolConfigurationOptions{Direction: nas.PCOMSToNetwork, Containers: idContainer}
	if got := pduSessionIDFromPCOs(nil, &epco); got != 7 {
		t.Errorf("PDU session identity from the extended element = %d, want 7", got)
	}
}

// TS 23.501 §5.15.7.1
func TestEPSSNSSAIContainerFormat(t *testing.T) {
	container, err := snssaiPCOContainer(models.Snssai{Sst: 1, Sd: "000001"}, models.PlmnID{Mcc: "001", Mnc: "01"})
	if err != nil {
		t.Fatalf("build the S-NSSAI container: %v", err)
	}

	if container.ID != nas.PCOContainerSNSSAI {
		t.Errorf("container identifier = %#06x, want the network-to-MS S-NSSAI identifier 001BH", container.ID)
	}

	want := []byte{0x01, 0x00, 0x00, 0x01, 0x00, 0xf1, 0x10}
	if !bytes.Equal(container.Content, want) {
		t.Errorf("container content = % x, want % x (SST, SD, then MCC/MNC with the 1111 filler)", container.Content, want)
	}
}

// TS 24.301 §6.6.1.1
func TestEPSExtendedPCOIsAPropertyOfThePDNConnection(t *testing.T) {
	epcoCapable := eps.UENetworkCapability{HasUMTS: true, Rest: []byte{0x00, 0x80, 0x20}}

	for _, tc := range []struct {
		name        string
		transferred bool
		cap         eps.UENetworkCapability
		want        bool
	}{
		{"transferred, UE supports ePCO", true, epcoCapable, true},
		{"transferred, UE does not", true, eps.UENetworkCapability{HasUMTS: true, Rest: []byte{0x00, 0x00, 0x20}}, false},
		{"ordinary attach, UE supports ePCO", false, epcoCapable, false},
		{"ordinary attach, UE does not", false, eps.UENetworkCapability{}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ue := &mme.UeContext{}
			ue.SetUESecurityCapability(tc.cap, nil, mme.MintAuthProofForTrackingAreaUpdate())

			p := &mme.PdnConnection{
				Ebi:         mme.DefaultERABID,
				PdnType:     eps.PDNTypeIPv4,
				UeIP:        netip.MustParseAddr("10.45.0.2"),
				Dns:         netip.MustParseAddr("8.8.8.8"),
				Transferred: tc.transferred,
			}

			qos := &mme.EpsQoS{
				APN:        "internet",
				QCI:        9,
				MTU:        1400,
				SessAmbrUL: models.MustParseBitRate("1 Gbps"),
				SessAmbrDL: models.MustParseBitRate("1 Gbps"),
			}

			raw, err := buildActivateDefaultESM(p, qos, 1, models.PlmnID{Mcc: "001", Mnc: "01"}, ue.UsesEPCO(p))
			if err != nil {
				t.Fatalf("build ACTIVATE DEFAULT EPS BEARER CONTEXT REQUEST: %v", err)
			}

			activate, err := eps.ParseActivateDefaultEPSBearerContextRequest(raw)
			if err != nil {
				t.Fatalf("parse the encoded request: %v", err)
			}

			carriesEPCO := activate.ExtendedProtocolConfigurationOptions != nil
			carriesPCO := activate.ProtocolConfigurationOptions != nil

			if carriesEPCO != tc.want || carriesPCO == tc.want {
				t.Errorf("extended element = %v, classic element = %v, want extended %v: the two are mutually exclusive (TS 24.301 §8.3.6.9/.15)",
					carriesEPCO, carriesPCO, tc.want)
			}
		})
	}
}
