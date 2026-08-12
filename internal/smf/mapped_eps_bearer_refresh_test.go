// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package smf_test

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/smf"
	"github.com/ellanetworks/core/nas/eps"
	"github.com/ellanetworks/core/nas/fgs"
)

const refreshTestEBI uint8 = 5

func setupInterworkingSession(t *testing.T, s *smf.SMF) (*smf.SMContext, string) {
	t.Helper()

	return setupSessionWithTunnelIdentity(t, s, smf.SessionIdentity{PDUSessionID: 1, EBI: refreshTestEBI})
}

func reconcilePolicy(t *testing.T, s *smf.SMF, ref string, delta *models.SessionPolicyDelta) {
	t.Helper()

	err := s.ReconcileSmContext(context.Background(), &models.SessionReconcileRequest{
		SmContextRef: ref,
		Reason:       models.ReconcilePolicyChange,
		NewPolicy:    delta,
	})
	if err != nil {
		t.Fatalf("ReconcileSmContext failed: %v", err)
	}
}

func modificationCommand(t *testing.T, amfCb *fakeAMF) *fgs.PDUSessionModificationCommand {
	t.Helper()

	amfCb.mu.Lock()
	calls := len(amfCb.modifyCalls)
	amfCb.mu.Unlock()

	if calls != 1 {
		t.Fatalf("expected 1 N1N2 modify call, got %d", calls)
	}

	amfCb.mu.Lock()
	n1Msg := amfCb.modifyCalls[0].n1Msg
	amfCb.mu.Unlock()

	cmd, err := fgs.ParsePDUSessionModificationCommand(modificationSMPayload(t, n1Msg))
	if err != nil {
		t.Fatalf("decode PDU Session Modification Command: %v", err)
	}

	return cmd
}

func mappedContext(t *testing.T, cmd *fgs.PDUSessionModificationCommand) fgs.MappedEPSBearerContext {
	t.Helper()

	if len(cmd.MappedEPSBearerContexts) != 1 {
		t.Fatalf("got %d mapped EPS bearer contexts, want 1", len(cmd.MappedEPSBearerContexts))
	}

	return cmd.MappedEPSBearerContexts[0]
}

func mappedQCI(t *testing.T, mapped fgs.MappedEPSBearerContext) uint8 {
	t.Helper()

	for _, p := range mapped.Parameters {
		if p.Identifier != fgs.EPSParameterMappedEPSQoS {
			continue
		}

		qos, err := eps.ParseEPSQoS(p.Contents)
		if err != nil {
			t.Fatalf("mapped EPS QoS: %v", err)
		}

		return qos.QCI
	}

	t.Fatal("the mapped EPS bearer context carries no mapped EPS QoS")

	return 0
}

func mappedAPNAMBR(t *testing.T, mapped fgs.MappedEPSBearerContext) (uint64, uint64) {
	t.Helper()

	for _, p := range mapped.Parameters {
		if p.Identifier != fgs.EPSParameterAPNAMBR {
			continue
		}

		ambr, err := eps.ParseAPNAMBR(p.Contents)
		if err != nil {
			t.Fatalf("APN-AMBR: %v", err)
		}

		dl, ul, ok := ambr.Kbps()
		if !ok {
			t.Fatalf("APN-AMBR %v has no kbps representation", ambr)
		}

		return dl, ul
	}

	t.Fatal("the mapped EPS bearer context carries no APN-AMBR")

	return 0, 0
}

func TestTS24501_6_3_2_2_FiveQIChangeRefreshesTheMappedEPSBearerContext(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb)

	smCtx, ref := setupInterworkingSession(t, s)

	reconcilePolicy(t, s, ref, &models.SessionPolicyDelta{
		SessionAmbrUplink:   "100 Mbps",
		SessionAmbrDownlink: "200 Mbps",
		Var5qi:              6,
		Arp:                 1,
	})

	mapped := mappedContext(t, modificationCommand(t, amfCb))

	if mapped.EPSBearerIdentity != refreshTestEBI {
		t.Fatalf("EPS bearer identity = %d, want %d", mapped.EPSBearerIdentity, refreshTestEBI)
	}

	if mapped.Operation != fgs.MappedEPSBearerOpModify {
		t.Fatalf("operation = %s, want modify", mapped.Operation)
	}

	if qci := mappedQCI(t, mapped); qci != 6 {
		t.Fatalf("mapped QCI = %d, want the new 5QI 6", qci)
	}

	if dl, ul := mappedAPNAMBR(t, mapped); dl != 200_000 || ul != 100_000 {
		t.Fatalf("APN-AMBR = %d/%d kbps, want 200000/100000", dl, ul)
	}

	if _, err := s.UpdateSmContextN1Msg(context.Background(), ref,
		buildPDUSessionModificationComplete(smCtx.PDUSessionID, 0)); err != nil {
		t.Fatalf("modification complete: %v", err)
	}

	smCtx.Mutex.Lock()
	committed := smCtx.PolicyData.QosData.Var5qi
	smCtx.Mutex.Unlock()

	if committed != 6 {
		t.Fatalf("stored 5QI = %d, want 6 after the UE accepted", committed)
	}

	if ptiInUse(t, smCtx, 0) {
		t.Fatal("the procedure is still in flight after the PDU Session Modification Complete")
	}
}

func TestTS24501_6_3_2_2_SessionAMBRChangeRefreshesTheMappedEPSBearerContext(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb)

	_, ref := setupInterworkingSession(t, s)

	reconcilePolicy(t, s, ref, &models.SessionPolicyDelta{
		SessionAmbrUplink:   "300 Mbps",
		SessionAmbrDownlink: "400 Mbps",
		Var5qi:              9,
		Arp:                 1,
	})

	mapped := mappedContext(t, modificationCommand(t, amfCb))

	if dl, ul := mappedAPNAMBR(t, mapped); dl != 400_000 || ul != 300_000 {
		t.Fatalf("APN-AMBR = %d/%d kbps, want 400000/300000", dl, ul)
	}

	if qci := mappedQCI(t, mapped); qci != 9 {
		t.Fatalf("mapped QCI = %d, want the unchanged 5QI 9", qci)
	}
}

func TestTS24501_6_1_4_1_PolicyChangeRefreshesNoMappedContextWithoutAnEPSBearer(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb)

	_, ref := setupSessionWithTunnel(t, s)

	reconcilePolicy(t, s, ref, &models.SessionPolicyDelta{
		SessionAmbrUplink:   "300 Mbps",
		SessionAmbrDownlink: "400 Mbps",
		Var5qi:              6,
		Arp:                 1,
	})

	if cmd := modificationCommand(t, amfCb); cmd.MappedEPSBearerContexts != nil {
		t.Fatal("a session with no EPS bearer identity has no mapped context to refresh")
	}
}

func TestTS24501_6_3_2_2_ARPOnlyChangeRefreshesNoMappedEPSBearerContext(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb)

	_, ref := setupInterworkingSession(t, s)

	reconcilePolicy(t, s, ref, &models.SessionPolicyDelta{
		SessionAmbrUplink:   "100 Mbps",
		SessionAmbrDownlink: "200 Mbps",
		Var5qi:              9,
		Arp:                 14,
	})

	if cmd := modificationCommand(t, amfCb); cmd.MappedEPSBearerContexts != nil {
		t.Fatal("ARP rides the E-RAB, not the mapped EPS bearer context: nothing the UE holds changed")
	}
}

func TestTS24501_6_3_2_2_UnchangedPolicySendsNoModificationCommand(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb)

	_, ref := setupInterworkingSession(t, s)

	reconcilePolicy(t, s, ref, &models.SessionPolicyDelta{
		SessionAmbrUplink:   "100 Mbps",
		SessionAmbrDownlink: "200 Mbps",
		Var5qi:              9,
		Arp:                 1,
	})

	amfCb.mu.Lock()
	calls := len(amfCb.modifyCalls)
	amfCb.mu.Unlock()

	if calls != 0 {
		t.Fatalf("expected no N1N2 modify call, got %d", calls)
	}
}
