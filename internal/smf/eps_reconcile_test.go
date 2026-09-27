// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package smf_test

import (
	"context"
	"errors"
	"net/netip"
	"slices"
	"testing"

	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/smf"
	"github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/nas/fgs"
)

func epsReconcileFixture(t *testing.T, pduSessionID uint8) (*smf.SMF, *fakePCF, *fakeUPF, *fakeMME, string) {
	t.Helper()

	store, upf := epsTestSMF()
	pcf := &fakePCF{policy: epsPolicy()}
	s := newTestSMF(pcf, store, upf, &fakeAMF{})

	mmeCb := &fakeMME{}
	s.SetMME(mmeCb)

	req := epsRequest(1)
	req.PDUSessionID = pduSessionID

	bearer, err := s.CreateEPSSession(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}

	enb := models.FTEID{TEID: 0x55, Addr: netip.AddrFrom4([4]byte{10, 3, 0, 3})}
	if err := s.ModifyEPSSession(context.Background(), bearer.Ref, epsTestEBI, enb); err != nil {
		t.Fatal(err)
	}

	return s, pcf, upf, mmeCb, bearer.Ref
}

func changePolicy(pcf *fakePCF, mutate func(*smf.Policy)) {
	pcf.mu.Lock()
	defer pcf.mu.Unlock()

	next := *pcf.policy
	mutate(&next)
	pcf.policy = &next
}

func committedPolicy(s *smf.SMF, ref string) smf.Policy {
	sc := s.GetSession(ref)

	sc.Mutex.Lock()
	defer sc.Mutex.Unlock()

	return *sc.PolicyData
}

func TestCreateEPSSessionReturnsTheAuthorizedQoS(t *testing.T) {
	store, upf := epsTestSMF()
	s := newTestSMF(&fakePCF{policy: epsPolicy()}, store, upf, &fakeAMF{})

	bearer, err := s.CreateEPSSession(context.Background(), epsRequest(1))
	if err != nil {
		t.Fatal(err)
	}

	want := models.EPSBearerQoS{QCI: 9, ARP: 1, APNAMBR: epsPolicy().Ambr}
	if bearer.QoS != want {
		t.Errorf("bearer QoS = %+v, want %+v", bearer.QoS, want)
	}

	if bearer.MTU != 1400 {
		t.Errorf("bearer MTU = %d, want 1400", bearer.MTU)
	}

	if bearer.MappedFiveGSQoS != nil {
		t.Error("mapped 5GS QoS was returned for a PDN connection the UE gave no PDU session identity")
	}
}

func TestCreateEPSSessionSelectsTheSlice(t *testing.T) {
	store, upf := epsTestSMF()
	s := newTestSMF(&fakePCF{policy: epsPolicy()}, store, upf, &fakeAMF{})

	req := epsRequest(1)
	req.PDUSessionID = 5

	bearer, err := s.CreateEPSSession(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}

	if bearer.Snssai == nil || !bearer.Snssai.Equal(*testSnssai) {
		t.Fatalf("bearer S-NSSAI = %+v, want the policy's %+v", bearer.Snssai, testSnssai)
	}

	ids := make([]uint16, 0, len(bearer.MappedFiveGSQoS))
	for _, c := range bearer.MappedFiveGSQoS {
		ids = append(ids, c.ID)
	}

	for _, id := range []uint16{nas.PCOContainerQoSRules, nas.PCOContainerSessionAMBR, nas.PCOContainerQoSFlowDescriptions} {
		if !slices.Contains(ids, id) {
			t.Errorf("mapped 5GS QoS lacks container %#04x", id)
		}
	}
}

func TestEPSSessionMovedFrom5GSKeepsItsSlicesPolicy(t *testing.T) {
	pcf, store, upf, amfCb, mmeCb := interworkingFakes()
	s := newTestSMF(pcf, store, upf, amfCb)
	s.SetMME(mmeCb)

	sc := establish5GS(t, s)

	slice := *sc.Snssai
	pcf.lastSnssai = nil

	req := epsMove(sc.PDUSessionID)
	req.Snssai = &slice

	if _, err := s.CreateEPSSession(context.Background(), req); err != nil {
		t.Fatalf("CreateEPSSession: %v", err)
	}

	if pcf.lastSnssai == nil || !pcf.lastSnssai.Equal(slice) {
		t.Fatalf("policy resolved for slice %+v, want the moving session's %+v", pcf.lastSnssai, slice)
	}

	if pcf.apnLookups != 0 {
		t.Fatal("the policy of a session moving with its slice was resolved by APN alone")
	}
}

func TestUERequestedEPSHandoverKeepsTheSessionsSlice(t *testing.T) {
	pcf, store, upf, amfCb, mmeCb := interworkingFakes()
	s := newTestSMF(pcf, store, upf, amfCb)
	s.SetMME(mmeCb)

	sc := establish5GS(t, s)
	pcf.lastSnssai = nil

	req := epsMove(sc.PDUSessionID)
	req.EPSBearerIdentity = 7

	bearer, err := s.CreateEPSSession(context.Background(), req)
	if err != nil {
		t.Fatalf("CreateEPSSession: %v", err)
	}

	if pcf.apnLookups != 0 || pcf.lastSnssai == nil || !pcf.lastSnssai.Equal(*sc.Snssai) {
		t.Fatalf("policy resolved for slice %+v after %d APN lookups, want the moving session's %+v", pcf.lastSnssai, pcf.apnLookups, sc.Snssai)
	}

	var flows []byte

	for _, c := range bearer.MappedFiveGSQoS {
		if c.ID == nas.PCOContainerQoSFlowDescriptions {
			flows = c.Content
		}
	}

	parsed, err := fgs.ParseQoSFlowDescriptions(flows)
	if err != nil || len(parsed) != 1 {
		t.Fatalf("mapped QoS flow descriptions = %v (%v), want one", parsed, err)
	}

	if ebi, ok := parsed[0].EPSBearerID(); !ok || ebi != 7 {
		t.Fatalf("mapped QoS flow names EPS bearer %d, want the bearer being activated (7)", ebi)
	}
}

func TestEPSCommitIgnoresASessionNowOn5GS(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb)

	smCtx, ref := setupSessionWithTunnel(t, s)

	reconcileAmbrChange(t, s, pcf, ref)

	s.CommitEPSBearerModification(context.Background(), ref, true)

	smCtx.Mutex.Lock()
	dl := smCtx.PolicyData.Ambr.Downlink
	smCtx.Mutex.Unlock()

	if dl != models.MustParseBitRate("200 Mbps") {
		t.Fatalf("a late EPS accept committed a 5GS modification: downlink %s", dl)
	}
}

func TestUserPlaneReactivationAbortsAPendingModification(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb)

	smCtx, ref := setupSessionWithTunnel(t, s)

	reconcileAmbrChange(t, s, pcf, ref)

	if _, err := s.ActivateSmContext(context.Background(), ref); err != nil {
		t.Fatalf("ActivateSmContext: %v", err)
	}

	if _, err := s.UpdateSmContextN1Msg(context.Background(), ref, buildPDUSessionModificationComplete(smCtx.PDUSessionID, 0)); err != nil {
		t.Fatalf("modification complete: %v", err)
	}

	smCtx.Mutex.Lock()
	dl := smCtx.PolicyData.Ambr.Downlink
	smCtx.Mutex.Unlock()

	if dl != models.MustParseBitRate("200 Mbps") {
		t.Fatalf("a modification aborted by the user-plane re-establishment was committed: downlink %s", dl)
	}
}

func TestEPSReconcileModifiesTheBearerAndCommitsOnAccept(t *testing.T) {
	s, pcf, upf, mmeCb, ref := epsReconcileFixture(t, 0)

	before := len(upf.modifyCalls)
	newAmbr := models.Ambr{Uplink: models.MustParseBitRate("300 Mbps"), Downlink: models.MustParseBitRate("400 Mbps")}

	changePolicy(pcf, func(p *smf.Policy) { p.Ambr = newAmbr })

	if err := s.ReconcileSession(context.Background(), ref); err != nil {
		t.Fatal(err)
	}

	mods := mmeCb.modifications()
	if len(mods) != 1 {
		t.Fatalf("EPS bearer modifications = %d, want 1", len(mods))
	}

	if mods[0].APNAMBR == nil || *mods[0].APNAMBR != newAmbr || mods[0].QoS != nil || mods[0].DNS.IsValid() {
		t.Fatalf("modification = %+v, want the new APN-AMBR alone", mods[0])
	}

	if len(upf.modifyCalls) != before {
		t.Error("the UPF QER was updated before the UE accepted the modification")
	}

	if committedPolicy(s, ref).Ambr == newAmbr {
		t.Fatal("the new policy was committed before the UE accepted it")
	}

	s.CommitEPSBearerModification(context.Background(), ref, true)

	if committedPolicy(s, ref).Ambr != newAmbr {
		t.Fatal("an accepted modification was not committed")
	}

	if len(upf.modifyCalls) == before {
		t.Error("the UPF QER was not updated once the UE accepted")
	}
}

func TestEPSReconcileRetriesARejectedModification(t *testing.T) {
	s, pcf, _, mmeCb, ref := epsReconcileFixture(t, 0)

	changePolicy(pcf, func(p *smf.Policy) { p.QosData.Var5qi = 8; p.QosData.Arp = &models.Arp{PriorityLevel: 2} })

	if err := s.ReconcileSession(context.Background(), ref); err != nil {
		t.Fatal(err)
	}

	if err := s.ReconcileSession(context.Background(), ref); err != nil {
		t.Fatal(err)
	}

	if got := len(mmeCb.modifications()); got != 1 {
		t.Fatalf("modifications while one is outstanding = %d, want 1", got)
	}

	mod := mmeCb.modifications()[0]
	if mod.QoS == nil || *mod.QoS != (models.EPSBearerQoS{QCI: 8, ARP: 2, APNAMBR: epsPolicy().Ambr}) {
		t.Fatalf("modification QoS = %+v, want QCI 8 ARP 2", mod.QoS)
	}

	s.CommitEPSBearerModification(context.Background(), ref, false)

	if committedPolicy(s, ref).QosData.Var5qi != 9 {
		t.Fatal("a rejected modification was committed")
	}

	if err := s.ReconcileSession(context.Background(), ref); err != nil {
		t.Fatal(err)
	}

	if got := len(mmeCb.modifications()); got != 2 {
		t.Fatalf("modifications after the reject = %d, want a retry", got)
	}
}

func TestEPSReconcileRetriesWhenTheMMECannotSignal(t *testing.T) {
	s, pcf, _, mmeCb, ref := epsReconcileFixture(t, 0)

	mmeCb.modifyErr = smf.ErrUENotReachable

	changePolicy(pcf, func(p *smf.Policy) { p.Ambr.Downlink = models.MustParseBitRate("400 Mbps") })

	if err := s.ReconcileSession(context.Background(), ref); err != nil {
		t.Fatalf("an unreachable UE failed the reconcile: %v", err)
	}

	mmeCb.mu.Lock()
	mmeCb.modifyErr = nil
	mmeCb.mu.Unlock()

	if err := s.ReconcileSession(context.Background(), ref); err != nil {
		t.Fatal(err)
	}

	if got := len(mmeCb.modifications()); got != 1 {
		t.Fatalf("modifications once the UE is reachable = %d, want 1", got)
	}
}

func TestEPSReconcileReactivatesOnAPoolOrMTUChange(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mutate func(*smf.Policy)
	}{
		{"IPv4 pool", func(p *smf.Policy) { p.IPv4Pool = "10.99.0.0/22" }},
		{"MTU", func(p *smf.Policy) { p.MTU = 1300 }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, pcf, _, mmeCb, ref := epsReconcileFixture(t, 0)

			changePolicy(pcf, tc.mutate)

			if err := s.ReconcileSession(context.Background(), ref); err != nil {
				t.Fatal(err)
			}

			if got := mmeCb.reactivations(); !slices.Equal(got, []uint8{epsTestEBI}) {
				t.Fatalf("reactivations = %v, want [%d]", got, epsTestEBI)
			}

			if len(mmeCb.modifications()) != 0 {
				t.Fatal("a change the UE cannot adopt in place was sent as a modification")
			}
		})
	}
}

func TestEPSReconcileReactivatesAnUnauthorizedAPN(t *testing.T) {
	s, pcf, _, mmeCb, ref := epsReconcileFixture(t, 0)

	pcf.mu.Lock()
	pcf.err = smf.ErrNoPolicyMatch
	pcf.mu.Unlock()

	if err := s.ReconcileSession(context.Background(), ref); err != nil {
		t.Fatal(err)
	}

	if got := mmeCb.reactivations(); !slices.Equal(got, []uint8{epsTestEBI}) {
		t.Fatalf("reactivations = %v, want [%d]", got, epsTestEBI)
	}
}

func TestEPSReconcileLeavesTheIdleUEToTheMME(t *testing.T) {
	s, pcf, upf, mmeCb, ref := epsReconcileFixture(t, 0)

	if err := s.DeactivateEPSSession(context.Background(), ref); err != nil {
		t.Fatal(err)
	}

	before := len(upf.modifyCalls)
	mmeCb.modifyErr = smf.ErrUENotReachable

	changePolicy(pcf, func(p *smf.Policy) { p.Ambr.Downlink = models.MustParseBitRate("400 Mbps") })

	if err := s.ReconcileSession(context.Background(), ref); err != nil {
		t.Fatal(err)
	}

	if len(upf.modifyCalls) != before {
		t.Error("the UPF was reconfigured for a change the idle UE never received")
	}

	if committedPolicy(s, ref).Ambr.Downlink.Equal(models.MustParseBitRate("400 Mbps")) {
		t.Error("the change was committed while the UE was idle")
	}

	mmeCb.mu.Lock()
	mmeCb.modifyErr = nil
	mmeCb.mu.Unlock()

	if err := s.ReconcileSession(context.Background(), ref); err != nil {
		t.Fatal(err)
	}

	if len(mmeCb.modifications()) != 1 {
		t.Fatal("the deferred change was not sent once the UE could be reached")
	}
}

func TestEPSReconcileRefreshesTheMappedFiveGSQoS(t *testing.T) {
	s, pcf, _, mmeCb, ref := epsReconcileFixture(t, 5)

	changePolicy(pcf, func(p *smf.Policy) { p.Ambr.Downlink = models.MustParseBitRate("400 Mbps") })

	if err := s.ReconcileSession(context.Background(), ref); err != nil {
		t.Fatal(err)
	}

	mods := mmeCb.modifications()
	if len(mods) != 1 {
		t.Fatalf("modifications = %d, want 1", len(mods))
	}

	ids := make([]uint16, 0, len(mods[0].MappedFiveGSQoS))
	for _, c := range mods[0].MappedFiveGSQoS {
		ids = append(ids, c.ID)
	}

	if !slices.Contains(ids, nas.PCOContainerSessionAMBR) || !slices.Contains(ids, nas.PCOContainerQoSFlowDescriptions) {
		t.Fatalf("mapped 5GS QoS containers = %#04x, want the Session-AMBR and QoS flow descriptions", ids)
	}

	if slices.Contains(ids, nas.PCOContainerQoSRules) {
		t.Error("the refresh re-sends the default QoS rule, which the UE rejects with 5GSM cause #83 (TS 24.501 §6.1.4.1 case a)7)")
	}
}

func TestEPSReconcileErrorsOnAFailedMMECall(t *testing.T) {
	s, pcf, _, mmeCb, ref := epsReconcileFixture(t, 0)

	mmeCb.modifyErr = errors.New("encode failure")

	changePolicy(pcf, func(p *smf.Policy) { p.Ambr.Downlink = models.MustParseBitRate("400 Mbps") })

	if err := s.ReconcileSession(context.Background(), ref); err == nil {
		t.Fatal("a failed modification was reported as success")
	}
}
