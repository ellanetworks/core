// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas_test

import (
	"net"
	"testing"

	"github.com/ellanetworks/core/internal/models"
	smfNas "github.com/ellanetworks/core/internal/smf/nas"
	"github.com/ellanetworks/core/nas/eps"
	"github.com/ellanetworks/core/nas/fgs"
)

func acceptWithEBI(t *testing.T, ebi uint8) *fgs.PDUSessionEstablishmentAccept {
	t.Helper()

	ambr := &models.Ambr{Uplink: models.MustParseBitRate("50 Mbps"), Downlink: models.MustParseBitRate("100 Mbps")}
	qos := &models.QosData{QFI: 1, Var5qi: 9}
	snssai := &models.Snssai{Sst: 1}

	msg, err := smfNas.BuildGSMPDUSessionEstablishmentAccept(ambr, qos, 5, 1, snssai, "internet",
		&smfNas.ProtocolConfigurationOptions{}, net.IP{}, 0, nil, nil, nil, ebi)
	if err != nil {
		t.Fatalf("build failed: %v", err)
	}

	acc, err := fgs.ParsePDUSessionEstablishmentAccept(msg)
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}

	return acc
}

// TS 24.501 §6.1.4.1, §9.11.4.8
func TestEstablishmentAcceptCarriesTheMappedEPSBearerContext(t *testing.T) {
	acc := acceptWithEBI(t, 5)

	if len(acc.MappedEPSBearerContexts) != 1 {
		t.Fatalf("got %d mapped EPS bearer contexts, want 1", len(acc.MappedEPSBearerContexts))
	}

	mapped := acc.MappedEPSBearerContexts[0]
	if mapped.EPSBearerIdentity != 5 {
		t.Fatalf("EPS bearer identity = %d, want 5", mapped.EPSBearerIdentity)
	}

	if mapped.Operation != fgs.MappedEPSBearerOpCreate {
		t.Fatalf("operation = %s, want create", mapped.Operation)
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

			if qos.QCI != 9 {
				t.Fatalf("QCI = %d, want the 5QI 9 the session runs", qos.QCI)
			}
		case fgs.EPSParameterAPNAMBR:
			sawAMBR = true

			ambr, err := eps.ParseAPNAMBR(p.Contents)
			if err != nil {
				t.Fatalf("APN-AMBR: %v", err)
			}

			dl, ul, ok := ambr.Kbps()
			if !ok || dl != 100_000 || ul != 50_000 {
				t.Fatalf("APN-AMBR = (%d, %d, %v), want the session AMBR", dl, ul, ok)
			}
		}
	}

	if !sawQoS || !sawAMBR {
		t.Fatalf("mapped context is missing parameters: qos=%v ambr=%v", sawQoS, sawAMBR)
	}
}

// TS 24.501 §9.11.4.12
func TestEstablishmentAcceptCarriesTheEPSBearerIdentityParameter(t *testing.T) {
	acc := acceptWithEBI(t, 7)

	if len(acc.QoSFlowDescriptions) != 1 {
		t.Fatalf("got %d QoS flow descriptions, want 1", len(acc.QoSFlowDescriptions))
	}

	var found bool

	for _, p := range acc.QoSFlowDescriptions[0].Parameters {
		if p.ID != fgs.QoSFlowParamEPSBearerID {
			continue
		}

		found = true

		if len(p.Value) != 1 || p.Value[0]>>4 != 7 {
			t.Fatalf("EPS bearer identity parameter = % x, want 7 in bits 5 to 8", p.Value)
		}
	}

	if !found {
		t.Fatal("the QoS flow description does not name its EPS bearer identity")
	}
}

func TestEstablishmentAcceptWithoutAnEPSBearerIdentity(t *testing.T) {
	acc := acceptWithEBI(t, 0)

	if acc.MappedEPSBearerContexts != nil {
		t.Fatal("a session with no EPS bearer identity must map to no EPS bearer")
	}

	for _, p := range acc.QoSFlowDescriptions[0].Parameters {
		if p.ID == fgs.QoSFlowParamEPSBearerID {
			t.Fatal("the QoS flow description must not name an EPS bearer identity")
		}
	}
}
