// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas_test

import (
	"testing"

	"github.com/ellanetworks/core/internal/models"
	smfNas "github.com/ellanetworks/core/internal/smf/nas"
	"github.com/ellanetworks/core/nas/eps"
	"github.com/ellanetworks/core/nas/fgs"
)

func refreshedCommand(t *testing.T, ebi uint8, mapped *smfNas.MappedEPSQoS) *fgs.PDUSessionModificationCommand {
	t.Helper()

	ambr := &models.Ambr{Uplink: models.MustParseBitRate("50 Mbps"), Downlink: models.MustParseBitRate("100 Mbps")}
	qos := &models.QosData{QFI: 1, Var5qi: 6, Arp: &models.Arp{PriorityLevel: 8}}

	encoded, err := smfNas.BuildPDUSessionModificationCommand(1, ambr, qos, nil, ebi, mapped)
	if err != nil {
		t.Fatalf("build failed: %v", err)
	}

	return decodeModCmd(t, encoded)
}

func mappedEPSQoS(fiveQI int32, uplink, downlink string) *smfNas.MappedEPSQoS {
	return &smfNas.MappedEPSQoS{
		QosData: models.QosData{QFI: 1, Var5qi: fiveQI},
		Ambr: models.Ambr{
			Uplink:   models.MustParseBitRate(uplink),
			Downlink: models.MustParseBitRate(downlink),
		},
	}
}

func TestTS24501_6_3_2_2_ModificationCommandRefreshesTheMappedEPSBearerContext(t *testing.T) {
	cmd := refreshedCommand(t, 5, mappedEPSQoS(6, "50 Mbps", "100 Mbps"))

	if len(cmd.MappedEPSBearerContexts) != 1 {
		t.Fatalf("got %d mapped EPS bearer contexts, want 1", len(cmd.MappedEPSBearerContexts))
	}

	mapped := cmd.MappedEPSBearerContexts[0]
	if mapped.EPSBearerIdentity != 5 {
		t.Fatalf("EPS bearer identity = %d, want 5", mapped.EPSBearerIdentity)
	}

	if mapped.Operation != fgs.MappedEPSBearerOpModify {
		t.Fatalf("operation = %s, want modify: the UE already holds a context for this bearer", mapped.Operation)
	}

	if !mapped.EBit {
		t.Fatal("the E bit must request replacement of all previously provided parameters")
	}

	var sawQoS, sawAMBR bool

	for _, p := range mapped.Parameters {
		switch p.Identifier {
		case fgs.EPSParameterMappedEPSQoS:
			sawQoS = true

			qos, err := eps.ParseEPSQoS(p.Contents)
			if err != nil {
				t.Fatalf("mapped EPS QoS: %v", err)
			}

			if qos.QCI != 6 {
				t.Fatalf("QCI = %d, want the new 5QI 6", qos.QCI)
			}
		case fgs.EPSParameterAPNAMBR:
			sawAMBR = true

			ambr, err := eps.ParseAPNAMBR(p.Contents)
			if err != nil {
				t.Fatalf("APN-AMBR: %v", err)
			}

			dl, ul, ok := ambr.Kbps()
			if !ok || dl != 100_000 || ul != 50_000 {
				t.Fatalf("APN-AMBR = (%d, %d, %v), want the new session AMBR", dl, ul, ok)
			}
		}
	}

	if !sawQoS || !sawAMBR {
		t.Fatalf("the refreshed context is missing parameters: qos=%v ambr=%v", sawQoS, sawAMBR)
	}
}

func TestTS24501_9_11_4_12_ModificationCommandKeepsTheEPSBearerIdentityParameter(t *testing.T) {
	cmd := refreshedCommand(t, 7, mappedEPSQoS(6, "50 Mbps", "100 Mbps"))

	if len(cmd.QoSFlowDescriptions) != 1 {
		t.Fatalf("got %d QoS flow descriptions, want 1", len(cmd.QoSFlowDescriptions))
	}

	flow := cmd.QoSFlowDescriptions[0]
	if flow.OperationCode != fgs.QoSFlowOpModify || !flow.EBit {
		t.Fatalf("operation = %s, E bit = %v; the modify operation replaces the whole parameters list",
			flow.OperationCode, flow.EBit)
	}

	var found bool

	for _, p := range flow.Parameters {
		if p.ID != fgs.QoSFlowParamEPSBearerID {
			continue
		}

		found = true

		if len(p.Value) != 1 || p.Value[0]>>4 != 7 {
			t.Fatalf("EPS bearer identity parameter = % x, want 7 in bits 5 to 8", p.Value)
		}
	}

	if !found {
		t.Fatal("the replacement parameters list dropped the flow's EPS bearer identity")
	}
}

func TestTS24501_6_1_4_1_ModificationCommandWithoutAnEPSBearerIdentityMapsToNoEPSBearer(t *testing.T) {
	cmd := refreshedCommand(t, 0, mappedEPSQoS(6, "50 Mbps", "100 Mbps"))

	if cmd.MappedEPSBearerContexts != nil {
		t.Fatal("a session with no EPS bearer identity must map to no EPS bearer")
	}

	for _, p := range cmd.QoSFlowDescriptions[0].Parameters {
		if p.ID == fgs.QoSFlowParamEPSBearerID {
			t.Fatal("the QoS flow description must not name an EPS bearer identity")
		}
	}
}

func TestTS24501_6_3_2_2_ModificationCommandOmitsAnUnchangedMappedEPSBearerContext(t *testing.T) {
	cmd := refreshedCommand(t, 5, nil)

	if cmd.MappedEPSBearerContexts != nil {
		t.Fatal("the Mapped EPS bearer contexts IE belongs in the command only when the contexts changed")
	}

	var found bool

	for _, p := range cmd.QoSFlowDescriptions[0].Parameters {
		if p.ID == fgs.QoSFlowParamEPSBearerID {
			found = true
		}
	}

	if !found {
		t.Fatal("the flow's EPS bearer identity survives a command that refreshes no mapped context")
	}
}
