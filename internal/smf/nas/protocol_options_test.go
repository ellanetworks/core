// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas_test

import (
	"encoding/hex"
	"net"
	"testing"

	"github.com/ellanetworks/core/internal/models"
	smfNas "github.com/ellanetworks/core/internal/smf/nas"
	"github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/nas/fgs"
)

func ipcpRequest(t *testing.T, body string) []nas.PCOContainer {
	t.Helper()

	content, err := hex.DecodeString(body)
	if err != nil {
		t.Fatal(err)
	}

	return []nas.PCOContainer{{ID: nas.PCOProtocolIPCP, Content: content}}
}

func TestEstablishmentAcceptAnswersIPCP(t *testing.T) {
	pco := &smfNas.ProtocolConfigurationOptions{
		DNSIPv4Request:   true,
		ProtocolRequests: ipcpRequest(t, "01000010"+"810600000000"+"830600000000"),
	}
	addrs := &smfNas.PDUSessionAddresses{
		PDUSessionType: fgs.PDUSessionTypeIPv4,
		IPv4Address:    net.IP{10, 45, 0, 1},
	}

	acc := buildAccept(t, &models.Snssai{Sst: 1}, pco, net.IP{8, 8, 8, 8}, nil, addrs, nil)

	if acc.ExtendedPCO == nil {
		t.Fatal("no extended protocol configuration options")
	}

	got := acc.ExtendedPCO.Containers

	if got[0].ID != nas.PCOProtocolIPCP {
		t.Fatalf("IPCP must lead the element (TS 24.008 §10.5.6.3 NOTE 1), got %+v", got)
	}

	want := "03000010" + "810608080808" + "830608080808"
	if hex.EncodeToString(got[0].Content) != want {
		t.Fatalf("IPCP answer = %x, want %s", got[0].Content, want)
	}

	var sawDNS bool

	for _, c := range got {
		if c.ID == nas.PCOContainerDNSServerIPv4Address {
			sawDNS = true
		}
	}

	if !sawDNS {
		t.Fatal("the 000D answer must still be sent alongside the IPCP one")
	}
}

func TestEstablishmentAcceptNaksIPCPAddressWithTheAllocatedOne(t *testing.T) {
	pco := &smfNas.ProtocolConfigurationOptions{
		ProtocolRequests: ipcpRequest(t, "0100000a"+"030600000000"),
	}
	addrs := &smfNas.PDUSessionAddresses{
		PDUSessionType: fgs.PDUSessionTypeIPv4,
		IPv4Address:    net.IP{10, 45, 0, 7},
	}

	acc := buildAccept(t, &models.Snssai{Sst: 1}, pco, net.IP{8, 8, 8, 8}, nil, addrs, nil)

	if acc.ExtendedPCO == nil {
		t.Fatal("an IPCP answer alone still warrants the element")
	}

	want := "03" + "00" + "000a" + "0306" + "0a2d0007"
	if hex.EncodeToString(acc.ExtendedPCO.Containers[0].Content) != want {
		t.Fatalf("got %x, want %s", acc.ExtendedPCO.Containers[0].Content, want)
	}
}

func TestEstablishmentAcceptWithoutProtocolRequestsIsUnchanged(t *testing.T) {
	pco := &smfNas.ProtocolConfigurationOptions{DNSIPv4Request: true}

	acc := buildAccept(t, &models.Snssai{Sst: 1}, pco, net.IP{8, 8, 8, 8}, nil, nil, nil)

	if acc.ExtendedPCO == nil {
		t.Fatal("no extended protocol configuration options")
	}

	for _, c := range acc.ExtendedPCO.Containers {
		switch c.ID {
		case nas.PCOProtocolIPCP, nas.PCOProtocolLCP, nas.PCOProtocolPAP, nas.PCOProtocolCHAP:
			t.Fatalf("unsolicited protocol option %04x", c.ID)
		}
	}
}
