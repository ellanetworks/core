// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas_test

import (
	"testing"

	"github.com/ellanetworks/core/internal/models"
	smfNas "github.com/ellanetworks/core/internal/smf/nas"
	"github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/nas/fgs"
)

func containerContent(t *testing.T, containers []nas.PCOContainer, id uint16) []byte {
	t.Helper()

	var content []byte

	for _, c := range containers {
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

func TestMappedFiveGSQoS(t *testing.T) {
	const ebi uint8 = 5

	ambr := &models.Ambr{Uplink: models.MustParseBitRate("50 Mbps"), Downlink: models.MustParseBitRate("100 Mbps")}

	containers, err := smfNas.MappedFiveGSQoS(ebi, &models.QosData{QFI: models.DefaultQFI, Var5qi: 9}, ambr)
	if err != nil {
		t.Fatal(err)
	}

	rules, err := fgs.ParseQoSRules(containerContent(t, containers, nas.PCOContainerQoSRules))
	if err != nil {
		t.Fatalf("parse the mapped QoS rules: %v", err)
	}

	if len(rules) != 1 || rules[0].DQR != 1 || rules[0].OperationCode != fgs.QoSRuleOpCreate {
		t.Fatalf("mapped QoS rules = %+v, want one created default rule", rules)
	}

	if rules[0].Parameters == nil || rules[0].Parameters.QFI != models.DefaultQFI {
		t.Errorf("mapped QoS rule QFI = %+v, want %d", rules[0].Parameters, models.DefaultQFI)
	}

	flows, err := fgs.ParseQoSFlowDescriptions(containerContent(t, containers, nas.PCOContainerQoSFlowDescriptions))
	if err != nil {
		t.Fatalf("parse the mapped QoS flow descriptions: %v", err)
	}

	if len(flows) != 1 || flows[0].QFI != models.DefaultQFI || flows[0].OperationCode != fgs.QoSFlowOpCreate {
		t.Fatalf("mapped QoS flows = %+v, want one created flow on QFI %d", flows, models.DefaultQFI)
	}

	if got, ok := flows[0].EPSBearerID(); !ok || got != ebi {
		t.Errorf("mapped QoS flow EPS bearer identity = %d/%t, want %d: the UE drops a flow naming another bearer", got, ok, ebi)
	}

	sessionAMBR, err := fgs.ParseSessionAMBR(containerContent(t, containers, nas.PCOContainerSessionAMBR))
	if err != nil {
		t.Fatalf("parse the mapped Session-AMBR: %v", err)
	}

	if dl, ul, ok := sessionAMBR.Kbps(); !ok || dl != ambr.Downlink.Kbps() || ul != ambr.Uplink.Kbps() {
		t.Errorf("mapped Session-AMBR = %d/%d kbps, want %d/%d", dl, ul, ambr.Downlink.Kbps(), ambr.Uplink.Kbps())
	}

	for _, id := range []uint16{nas.PCOContainerQoSRulesTwoOctet, nas.PCOContainerQoSFlowDescriptionsTwoOctet} {
		if containerContent(t, containers, id) != nil {
			t.Errorf("container %#04x was sent alongside its one-octet form; the UE accepts only one", id)
		}
	}
}

func TestMappedFiveGSQoSRefreshOmitsTheDefaultRule(t *testing.T) {
	containers, err := smfNas.MappedFiveGSQoSRefresh(5, &models.QosData{QFI: models.DefaultQFI, Var5qi: 9}, &models.Ambr{Uplink: models.MustParseBitRate("50 Mbps"), Downlink: models.MustParseBitRate("100 Mbps")})
	if err != nil {
		t.Fatal(err)
	}

	if containerContent(t, containers, nas.PCOContainerQoSRules) != nil {
		t.Error("the refresh re-sends the default QoS rule, which the UE rejects with 5GSM cause #83")
	}
}
