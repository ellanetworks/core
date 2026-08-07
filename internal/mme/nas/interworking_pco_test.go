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

func TestActivateDefaultCarriesTheSNSSAI(t *testing.T) {
	p := &mme.PdnConnection{
		Ebi:     mme.DefaultERABID,
		PdnType: eps.PDNTypeIPv4,
		UeIP:    netip.MustParseAddr("10.45.0.1"),
		Snssai:  &models.Snssai{Sst: 1, Sd: "000001"},
	}
	qos := &mme.EpsQoS{
		APN: "internet", QCI: 9,
		SessAmbrDL: models.MustParseBitRate("100 Mbps"),
		SessAmbrUL: models.MustParseBitRate("50 Mbps"),
		Snssai:     &models.Snssai{Sst: 2},
	}

	act := buildActivate(t, p, qos)

	if act.ProtocolConfigurationOptions == nil {
		t.Fatal("no protocol configuration options in the Activate Default EPS Bearer Context Request")
	}

	var content []byte

	for _, c := range act.ProtocolConfigurationOptions.Containers {
		if c.ID == nas.PCOContainerSNSSAI {
			if content != nil {
				t.Fatal("more than one S-NSSAI container")
			}

			content = c.Content
		}
	}

	if content == nil {
		t.Fatal("no S-NSSAI container in the protocol configuration options")
	}

	want := []byte{0x01, 0x00, 0x00, 0x01, 0x00, 0xf1, 0x10}
	if !bytes.Equal(content, want) {
		t.Errorf("S-NSSAI container = % x, want % x (the anchor's slice, not the policy's)", content, want)
	}
}

func TestActivateDefaultOmitsTheSNSSAIWhenTheAnchorHasNone(t *testing.T) {
	p := &mme.PdnConnection{Ebi: mme.DefaultERABID, PdnType: eps.PDNTypeIPv4, UeIP: netip.MustParseAddr("10.45.0.1")}
	qos := &mme.EpsQoS{APN: "internet", QCI: 9, SessAmbrDL: models.MustParseBitRate("100 Mbps"), SessAmbrUL: models.MustParseBitRate("50 Mbps")}

	act := buildActivate(t, p, qos)

	if act.ProtocolConfigurationOptions == nil {
		return
	}

	for _, c := range act.ProtocolConfigurationOptions.Containers {
		if c.ID == nas.PCOContainerSNSSAI {
			t.Fatal("an S-NSSAI container was sent for a connection the anchor holds on no slice")
		}
	}
}

func buildActivate(t *testing.T, p *mme.PdnConnection, qos *mme.EpsQoS) *eps.ActivateDefaultEPSBearerContextRequest {
	t.Helper()

	wire, err := buildActivateDefaultESM(p, qos, 1, models.PlmnID{Mcc: "001", Mnc: "01"})
	if err != nil {
		t.Fatalf("buildActivateDefaultESM: %v", err)
	}

	act, err := eps.ParseActivateDefaultEPSBearerContextRequest(wire)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	return act
}

func TestPDUSessionIDFromPCO(t *testing.T) {
	uplink := func(id uint8) *nas.ProtocolConfigurationOptions {
		pco := nas.ProtocolConfigurationOptions{
			ConfigProtocol: nas.PCOConfigProtocolPPP,
			Direction:      nas.PCOMSToNetwork,
			Containers:     []nas.PCOContainer{{ID: nas.PCOContainerPDUSessionID, Content: []byte{id}}},
		}

		return &pco
	}

	if got := pduSessionIDFromPCO(uplink(7)); got != 7 {
		t.Errorf("pduSessionIDFromPCO = %d, want 7", got)
	}

	if got := pduSessionIDFromPCO(nil); got != 0 {
		t.Errorf("pduSessionIDFromPCO(nil) = %d, want 0", got)
	}

	empty := nas.NewRequestedProtocolConfigurationOptions(nas.PCOContainerDNSServerIPv4Address)
	if got := pduSessionIDFromPCO(&empty); got != 0 {
		t.Errorf("pduSessionIDFromPCO with no identity container = %d, want 0", got)
	}
}
