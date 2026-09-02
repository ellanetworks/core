// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"encoding/hex"
	"net/netip"
	"strings"
	"testing"

	"github.com/ellanetworks/core/internal/mme"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/nas/eps"
)

func ipcpConfigureRequest(t *testing.T) []nas.PCOContainer {
	t.Helper()

	content, err := hex.DecodeString("01000010" + "810600000000" + "830600000000")
	if err != nil {
		t.Fatal(err)
	}

	return []nas.PCOContainer{{ID: nas.PCOProtocolIPCP, Content: content}}
}

func activateWithProtocolOptions(t *testing.T, p *mme.PdnConnection, qos *mme.EpsQoS, useEPCO bool) *eps.ActivateDefaultEPSBearerContextRequest {
	t.Helper()

	wire, err := buildActivateDefaultESM(p, qos, 1, models.PlmnID{Mcc: "001", Mnc: "01"}, useEPCO, ipcpConfigureRequest(t))
	if err != nil {
		t.Fatalf("buildActivateDefaultESM: %v", err)
	}

	act, err := eps.ParseActivateDefaultEPSBearerContextRequest(wire)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	return act
}

func epsPdn() (*mme.PdnConnection, *mme.EpsQoS) {
	p := &mme.PdnConnection{
		Ebi:     mme.DefaultERABID,
		PdnType: eps.PDNTypeIPv4,
		UeIP:    netip.MustParseAddr("10.45.0.1"),
		Dns:     netip.MustParseAddr("8.8.8.8"),
	}
	qos := &mme.EpsQoS{
		APN: "internet", QCI: 9, MTU: 1400,
		SessAmbrDL: models.MustParseBitRate("100 Mbps"),
		SessAmbrUL: models.MustParseBitRate("50 Mbps"),
	}

	return p, qos
}

func TestActivateDefaultAnswersIPCP(t *testing.T) {
	p, qos := epsPdn()

	act := activateWithProtocolOptions(t, p, qos, false)
	if act.ProtocolConfigurationOptions == nil {
		t.Fatal("no protocol configuration options")
	}

	got := act.ProtocolConfigurationOptions.Containers
	if got[0].ID != nas.PCOProtocolIPCP {
		t.Fatalf("IPCP must lead the element (TS 24.008 §10.5.6.3 NOTE 1), got % x", containerIDs(got))
	}

	want := "03000010" + "810608080808" + "830608080808"
	if hex.EncodeToString(got[0].Content) != want {
		t.Fatalf("IPCP answer = %x, want %s", got[0].Content, want)
	}
}

func TestActivateDefaultAnswersIPCPOverEPCO(t *testing.T) {
	p, qos := epsPdn()

	act := activateWithProtocolOptions(t, p, qos, true)
	if act.ExtendedProtocolConfigurationOptions == nil {
		t.Fatal("no extended protocol configuration options")
	}

	if act.ExtendedProtocolConfigurationOptions.Containers[0].ID != nas.PCOProtocolIPCP {
		t.Fatal("IPCP must lead the extended element too")
	}
}

func TestActivateDefaultDropsProtocolAnswersThatDoNotFitTheClassicIE(t *testing.T) {
	p, qos := epsPdn()
	p.Snssai = &models.Snssai{Sst: 1, Sd: "000001"}
	p.PDUSessionID = 5

	oversized := []nas.PCOContainer{{
		ID:      nas.PCOProtocolLCP,
		Content: append([]byte{1, 0, 0x00, 0xfe, 0x01, 0xfa}, make([]byte, 248)...),
	}}

	if answers := nas.AnswerProtocolOptions(oversized, p.Dns, p.UeIP); len(answers) != 1 {
		t.Fatalf("the fixture must produce an answer to be dropped, got %d", len(answers))
	}

	wire, err := buildActivateDefaultESM(p, qos, 1, models.PlmnID{Mcc: "001", Mnc: "01"}, false, oversized)
	if err != nil {
		t.Fatalf("an oversized answer must be dropped, not fail the activation: %v", err)
	}

	act, err := eps.ParseActivateDefaultEPSBearerContextRequest(wire)
	if err != nil {
		t.Fatal(err)
	}

	for _, c := range act.ProtocolConfigurationOptions.Containers {
		if c.ID == nas.PCOProtocolLCP {
			t.Fatal("the oversized answer was sent anyway")
		}
	}
}

func containerIDs(cs []nas.PCOContainer) string {
	var b strings.Builder

	for _, c := range cs {
		b.WriteString(hex.EncodeToString([]byte{byte(c.ID >> 8), byte(c.ID)}) + " ")
	}

	return b.String()
}
