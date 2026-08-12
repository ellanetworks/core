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
	"github.com/ellanetworks/core/nas/fgs"
)

func TestActivateDefaultCarriesTheSNSSAI(t *testing.T) {
	p := &mme.PdnConnection{
		Ebi:          mme.DefaultERABID,
		PdnType:      eps.PDNTypeIPv4,
		UeIP:         netip.MustParseAddr("10.45.0.1"),
		Snssai:       &models.Snssai{Sst: 1, Sd: "000001"},
		PDUSessionID: 5,
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

	return buildActivateWithEPCO(t, p, qos, false)
}

func buildActivateWithEPCO(t *testing.T, p *mme.PdnConnection, qos *mme.EpsQoS, useEPCO bool) *eps.ActivateDefaultEPSBearerContextRequest {
	t.Helper()

	wire, err := buildActivateDefaultESM(p, qos, 1, models.PlmnID{Mcc: "001", Mnc: "01"}, useEPCO)
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

	if got := pduSessionIDFromPCOs(uplink(7), nil); got != 7 {
		t.Errorf("from the classic PCO = %d, want 7", got)
	}

	if got := pduSessionIDFromPCOs(nil, uplink(9)); got != 9 {
		t.Errorf("from the extended PCO = %d, want 9", got)
	}

	if got := pduSessionIDFromPCOs(nil, nil); got != 0 {
		t.Errorf("with no options at all = %d, want 0", got)
	}

	empty := nas.NewRequestedProtocolConfigurationOptions(nas.PCOContainerDNSServerIPv4Address)
	if got := pduSessionIDFromPCOs(&empty, nil); got != 0 {
		t.Errorf("with no identity container = %d, want 0", got)
	}
}

func TestActivateDefaultWithholdsTheSNSSAIFromA5GCUnawareUE(t *testing.T) {
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
	}

	act := buildActivate(t, p, qos)

	if act.ProtocolConfigurationOptions == nil {
		t.Fatal("no protocol configuration options in the Activate Default EPS Bearer Context Request")
	}

	for _, c := range act.ProtocolConfigurationOptions.Containers {
		if c.ID == nas.PCOContainerSNSSAI {
			t.Error("an S-NSSAI container went to a UE that sent no PDU session identity")
		}
	}
}

// TS 24.301 §6.6.1.1
func TestActivateDefaultCarriesTheSNSSAIInEPCOOnATransferredPDN(t *testing.T) {
	p := &mme.PdnConnection{
		Ebi:          mme.DefaultERABID,
		PdnType:      eps.PDNTypeIPv4,
		UeIP:         netip.MustParseAddr("10.45.0.1"),
		Snssai:       &models.Snssai{Sst: 1, Sd: "000001"},
		PDUSessionID: 5,
	}
	qos := &mme.EpsQoS{
		APN: "internet", QCI: 9,
		SessAmbrDL: models.MustParseBitRate("100 Mbps"),
		SessAmbrUL: models.MustParseBitRate("50 Mbps"),
	}

	act := buildActivateWithEPCO(t, p, qos, true)

	if act.ProtocolConfigurationOptions != nil {
		t.Error("both protocol configuration options elements were sent; TS 24.301 §8.3.6.4 makes them exclusive")
	}

	if act.ExtendedProtocolConfigurationOptions == nil {
		t.Fatal("no extended protocol configuration options on a transferred PDN connection")
	}

	found := false

	for _, c := range act.ExtendedProtocolConfigurationOptions.Containers {
		if c.ID == nas.PCOContainerSNSSAI {
			found = true
		}
	}

	if !found {
		t.Error("the S-NSSAI container did not move into the extended element")
	}
}

func containerContent(t *testing.T, pco *nas.ProtocolConfigurationOptions, id uint16) []byte {
	t.Helper()

	var content []byte

	for _, c := range pco.Containers {
		if c.ID != id {
			continue
		}

		if content != nil {
			t.Fatalf("more than one container %#04x", id)
		}

		content = c.Content
	}

	return content
}

func TestActivateDefaultCarriesTheMappedFiveGSQoS(t *testing.T) {
	p := &mme.PdnConnection{
		Ebi:          mme.DefaultERABID,
		PdnType:      eps.PDNTypeIPv4,
		UeIP:         netip.MustParseAddr("10.45.0.1"),
		Snssai:       &models.Snssai{Sst: 1},
		PDUSessionID: 5,
	}
	qos := &mme.EpsQoS{
		APN: "internet", QCI: 9,
		SessAmbrDL: models.MustParseBitRate("100 Mbps"),
		SessAmbrUL: models.MustParseBitRate("50 Mbps"),
	}

	act := buildActivate(t, p, qos)

	if act.ProtocolConfigurationOptions == nil {
		t.Fatal("no protocol configuration options in the Activate Default EPS Bearer Context Request")
	}

	pco := act.ProtocolConfigurationOptions

	rules, err := fgs.ParseQoSRules(containerContent(t, pco, nas.PCOContainerQoSRules))
	if err != nil {
		t.Fatalf("parse the mapped QoS rules: %v", err)
	}

	if len(rules) != 1 {
		t.Fatalf("mapped QoS rules = %d, want 1", len(rules))
	}

	if rules[0].DQR != 1 || rules[0].OperationCode != fgs.QoSRuleOpCreate {
		t.Errorf("mapped QoS rule = %+v, want a created default rule", rules[0])
	}

	if rules[0].Parameters == nil || rules[0].Parameters.QFI != models.DefaultQFI {
		t.Errorf("mapped QoS rule QFI = %+v, want %d", rules[0].Parameters, models.DefaultQFI)
	}

	flows, err := fgs.ParseQoSFlowDescriptions(containerContent(t, pco, nas.PCOContainerQoSFlowDescriptions))
	if err != nil {
		t.Fatalf("parse the mapped QoS flow descriptions: %v", err)
	}

	if len(flows) != 1 {
		t.Fatalf("mapped QoS flow descriptions = %d, want 1", len(flows))
	}

	if flows[0].QFI != models.DefaultQFI || flows[0].OperationCode != fgs.QoSFlowOpCreate {
		t.Errorf("mapped QoS flow = %+v, want a created flow on QFI %d", flows[0], models.DefaultQFI)
	}

	if ebi, ok := flows[0].EPSBearerID(); !ok || ebi != mme.DefaultERABID {
		t.Errorf("mapped QoS flow EPS bearer identity = %d/%t, want %d: the UE drops a flow naming another bearer",
			ebi, ok, mme.DefaultERABID)
	}

	ambr, err := fgs.ParseSessionAMBR(containerContent(t, pco, nas.PCOContainerSessionAMBR))
	if err != nil {
		t.Fatalf("parse the mapped Session-AMBR: %v", err)
	}

	dl, ul, ok := ambr.Kbps()
	if !ok || dl != qos.SessAmbrDL.Kbps() || ul != qos.SessAmbrUL.Kbps() {
		t.Errorf("mapped Session-AMBR = %d/%d kbps, want %d/%d", dl, ul, qos.SessAmbrDL.Kbps(), qos.SessAmbrUL.Kbps())
	}

	for _, id := range []uint16{nas.PCOContainerQoSRulesTwoOctet, nas.PCOContainerQoSFlowDescriptionsTwoOctet} {
		if containerContent(t, pco, id) != nil {
			t.Errorf("container %#04x was sent alongside its one-octet form; the UE accepts only one", id)
		}
	}
}

func TestActivateDefaultOmitsTheMappedFiveGSQoSWithoutAPDUSessionIdentity(t *testing.T) {
	p := &mme.PdnConnection{
		Ebi:     mme.DefaultERABID,
		PdnType: eps.PDNTypeIPv4,
		UeIP:    netip.MustParseAddr("10.45.0.1"),
		Snssai:  &models.Snssai{Sst: 1},
	}
	qos := &mme.EpsQoS{
		APN: "internet", QCI: 9,
		SessAmbrDL: models.MustParseBitRate("100 Mbps"),
		SessAmbrUL: models.MustParseBitRate("50 Mbps"),
	}

	act := buildActivate(t, p, qos)
	if act.ProtocolConfigurationOptions == nil {
		return
	}

	for _, id := range []uint16{nas.PCOContainerQoSRules, nas.PCOContainerSessionAMBR, nas.PCOContainerQoSFlowDescriptions} {
		if containerContent(t, act.ProtocolConfigurationOptions, id) != nil {
			t.Errorf("container %#04x was sent for a PDN connection the UE gave no PDU session identity", id)
		}
	}
}
