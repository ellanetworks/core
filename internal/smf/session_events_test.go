// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package smf_test

import (
	"context"
	"net"
	"net/netip"
	"slices"
	"sync"
	"testing"

	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/smf"
	"github.com/ellanetworks/core/nas/fgs"
)

type eventLog struct {
	mu     sync.Mutex
	events []smf.SessionEvent
}

func (l *eventLog) observe(e smf.SessionEvent) {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.events = append(l.events, e)
}

func (l *eventLog) all() []smf.SessionEvent {
	l.mu.Lock()
	defer l.mu.Unlock()

	return slices.Clone(l.events)
}

func (l *eventLog) kinds() []smf.SessionEventKind {
	var kinds []smf.SessionEventKind

	for _, e := range l.all() {
		if e.Kind != smf.SessionUsageApplied {
			kinds = append(kinds, e.Kind)
		}
	}

	return kinds
}

func (l *eventLog) of(kind smf.SessionEventKind) []smf.SessionEvent {
	var out []smf.SessionEvent

	for _, e := range l.all() {
		if e.Kind == kind {
			out = append(out, e)
		}
	}

	return out
}

func eutraUEInfo(cell string) models.SessionUEInfo {
	return models.SessionUEInfo{
		Location:       eutraLocation(cell),
		RATType:        models.RATTypeEUTRA,
		TimeZone:       "+00:00",
		ServingNode:    models.ServingNode{Functionality: models.ServingNodeSGW},
		ServingNetwork: models.PlmnID{Mcc: "001", Mnc: "01"},
	}
}

func establish5GSWithUE(t *testing.T, s *smf.SMF, ue models.SessionUEInfo) *smf.SMContext {
	t.Helper()

	ctx := context.Background()

	ref, reject, err := s.CreateSmContext(ctx, models.SmContextCreateRequest{SUPI: testSUPI(), PDUSessionID: movedPDUSessionID, DNN: testDNN, Snssai: testSnssai, RequestType: fgs.RequestTypeInitialRequest, N1SmMsg: buildDualStackPDUSessionEstRequest(), UE: ue})
	if err != nil || reject != nil {
		t.Fatalf("CreateSmContext: %v (reject %d bytes)", err, len(reject))
	}

	n2, err := buildPDUSessionResourceSetupResponseTransfer(0x7001, net.ParseIP("10.3.0.9"))
	if err != nil {
		t.Fatalf("build N2 setup response: %v", err)
	}

	if err := s.UpdateSmContextN2InfoPduResSetupRsp(ctx, ref, n2); err != nil {
		t.Fatalf("UpdateSmContextN2InfoPduResSetupRsp: %v", err)
	}

	return s.GetSession(ref)
}

func observedInterworkingSMF() (*smf.SMF, *eventLog, *fakeUPF) {
	pcf, store, upf, amfCb, mmeCb := interworkingFakes()
	log := &eventLog{}
	s := newTestSMF(pcf, store, upf, amfCb, smf.WithSessionObserver(log.observe))
	s.SetMME(mmeCb)

	return s, log, upf
}

func TestEstablishmentOpensTheSessionEvents(t *testing.T) {
	s, log, _ := observedInterworkingSMF()

	sc := establish5GSWithUE(t, s, fiveGUEInfo("000000001"))

	events := log.all()
	if len(events) == 0 || events[0].Kind != smf.SessionEstablished {
		t.Fatalf("events %v, want the session to open with established", log.kinds())
	}

	e := events[0]
	if e.Ref != sc.Ref || e.ChargingID != sc.ChargingID || e.Next.RATType != models.RATTypeNR {
		t.Fatalf("established event %+v, want the session's reference, Charging ID and NR conditions", e)
	}

	if len(e.Next.Flows) != 1 || e.Next.Flows[0].Key.Dedicated || e.Next.Flows[0].ChargingID != sc.ChargingID {
		t.Fatalf("flows at establishment %+v, want the default flow under the session's Charging ID", e.Next.Flows)
	}
}

func TestAnIntraRATCellChangeIsOnlyAUserLocationChange(t *testing.T) {
	s, log, _ := observedInterworkingSMF()
	ctx := context.Background()

	sc := establish5GSWithUE(t, s, fiveGUEInfo("000000001"))

	moved := nrLocation("000000002")
	s.UpdateUEInfo(ctx, testSUPI(), models.SessionUEInfoUpdate{Location: &moved})

	if n := len(log.of(smf.SessionConditionsChanged)); n != 0 {
		t.Fatalf("%d condition changes before the session was next touched, want none", n)
	}

	s.CollectUsageForTest(ctx)

	changes := log.of(smf.SessionConditionsChanged)
	if len(changes) != 1 {
		t.Fatalf("condition changes %+v, want one", changes)
	}

	if got := changes[0].Triggers; !slices.Equal(got, []smf.TriggerType{smf.TriggerUserLocationChange}) {
		t.Fatalf("triggers %v, want only USER_LOCATION_CHANGE", got)
	}

	if !changes[0].At.Equal(sc.UEInfoChangedAt()) {
		t.Fatalf("change time %v, want when the UE information changed (%v)", changes[0].At, sc.UEInfoChangedAt())
	}
}

func TestAnIdleTransferChangesRATAndServingNodeWithoutHandoverEvents(t *testing.T) {
	s, log, _ := observedInterworkingSMF()

	establish5GSWithUE(t, s, fiveGUEInfo("000000001"))

	if _, err := s.TransferIdle(context.Background(), models.SessionTransferRequest{SUPI: testSUPI(), PDUSessionID: movedPDUSessionID, EPSBearerIdentity: epsTestEBI, DNN: testDNN, Snssai: testSnssai, UE: eutraUEInfo("0000001")}, smf.Access4G); err != nil {
		t.Fatalf("TransferIdle: %v", err)
	}

	if got := log.kinds(); !slices.Equal(got, []smf.SessionEventKind{smf.SessionEstablished, smf.SessionConditionsChanged}) {
		t.Fatalf("events %v, want one condition change and no handover event", got)
	}

	triggers := log.of(smf.SessionConditionsChanged)[0].Triggers
	for _, want := range []smf.TriggerType{smf.TriggerRATChange, smf.TriggerServingNodeChange, smf.TriggerUserLocationChange, smf.TriggerECGIChange} {
		if !slices.Contains(triggers, want) {
			t.Errorf("triggers %v, want %s", triggers, want)
		}
	}
}

func TestAHandoverToEPSStartsThenCompletes(t *testing.T) {
	s, log, _ := observedInterworkingSMF()
	ctx := context.Background()

	establish5GSWithUE(t, s, fiveGUEInfo("000000001"))

	req := epsMove(movedPDUSessionID)
	req.UE = eutraUEInfo("0000001")

	bearer, err := s.CreateEPSSession(ctx, req)
	if err != nil {
		t.Fatalf("move to EPS: %v", err)
	}

	if got := log.kinds(); !slices.Equal(got, []smf.SessionEventKind{smf.SessionEstablished, smf.SessionHandoverStarted}) {
		t.Fatalf("events after the move request %v, want the handover started", got)
	}

	enb := models.FTEID{TEID: 0x6001, Addr: netip.MustParseAddr("192.168.40.10")}
	if err := s.ModifyEPSSession(ctx, bearer.Ref, epsTestEBI, enb, nil); err != nil {
		t.Fatalf("ModifyEPSSession: %v", err)
	}

	want := []smf.SessionEventKind{smf.SessionEstablished, smf.SessionHandoverStarted, smf.SessionConditionsChanged, smf.SessionHandoverCompleted}
	if got := log.kinds(); !slices.Equal(got, want) {
		t.Fatalf("events %v, want %v", got, want)
	}

	if triggers := log.of(smf.SessionConditionsChanged)[0].Triggers; !slices.Contains(triggers, smf.TriggerRATChange) {
		t.Fatalf("triggers at the handover %v, want RAT_CHANGE", triggers)
	}
}

func TestACancelledHandoverIsReported(t *testing.T) {
	s, log, upf := observedInterworkingSMF()
	ctx := context.Background()

	sc, ref, _, _ := prepareArrival(t, s, upf)

	if err := s.UpdateSmContextN2HandoverCanceled(ctx, ref); err != nil {
		t.Fatalf("UpdateSmContextN2HandoverCanceled: %v", err)
	}

	want := []smf.SessionEventKind{smf.SessionEstablished, smf.SessionHandoverStarted, smf.SessionHandoverCancelled}
	if got := log.kinds(); !slices.Equal(got, want) {
		t.Fatalf("events %v, want %v", got, want)
	}

	if cancelled := log.of(smf.SessionHandoverCancelled)[0]; cancelled.Ref != sc.Ref || cancelled.Next.RATType != cancelled.Prev.RATType {
		t.Fatalf("cancel event %+v, want the session's, with its conditions unchanged", cancelled)
	}
}

func TestADedicatedFlowStartsAndTerminatesWithoutAQoSChange(t *testing.T) {
	s, log, _ := observedInterworkingSMF()
	ctx := context.Background()

	sc := establishEPSOnENB(t, s)

	pushRules(t, s, sc.Ref, voiceRule())

	started := log.of(smf.SessionSDFStarted)
	if len(started) != 1 || !started[0].Flow.Key.Dedicated || started[0].Flow.ChargingID == 0 || started[0].Flow.ChargingID == sc.ChargingID {
		t.Fatalf("started flows %+v, want the dedicated bearer's under its own Charging ID", started)
	}

	if started[0].Flow.QoS.FiveQI != 1 {
		t.Fatalf("started flow QoS %+v, want QCI 1", started[0].Flow.QoS)
	}

	enb := models.FTEID{TEID: 0x66, Addr: netip.MustParseAddr("10.3.0.3")}
	if err := s.DedicatedBearerActivated(ctx, sc.Ref, dedicatedUplinkTEID, 6, enb); err != nil {
		t.Fatalf("DedicatedBearerActivated: %v", err)
	}

	pushRules(t, s, sc.Ref)
	s.DedicatedBearerReleased(ctx, sc.Ref, dedicatedUplinkTEID)

	terminated := log.of(smf.SessionSDFTerminated)
	if len(terminated) != 1 || terminated[0].Flow != started[0].Flow {
		t.Fatalf("terminated flows %+v, want the started one", terminated)
	}

	if terminated[0].LastOfRatingGroup {
		t.Fatal("the default flow still carries the rating group, so the dedicated one was not its last")
	}

	if n := len(log.of(smf.SessionConditionsChanged)); n != 0 {
		t.Fatalf("%d condition changes, want none for a dedicated flow", n)
	}
}

func TestReleaseAppliesTheFinalUsageBeforeReleased(t *testing.T) {
	s, log, upf := observedInterworkingSMF()
	ctx := context.Background()

	sc := establish5GSWithUE(t, s, fiveGUEInfo("000000001"))
	upf.count(sc.PFCPContext.SEID, 100, 200)

	final := nrLocation("000000009")
	if err := s.ReleaseSmContext(ctx, sc.Ref, models.ReleaseFor(models.ReleaseUERequested).WithUE(models.SessionUEInfoUpdate{Location: &final})); err != nil {
		t.Fatalf("ReleaseSmContext: %v", err)
	}

	events := log.all()
	if len(events) < 2 {
		t.Fatalf("events %v, want the final usage then released", log.kinds())
	}

	usage, released := events[len(events)-2], events[len(events)-1]

	if usage.Kind != smf.SessionUsageApplied || usage.Usage[0].UplinkVolume != 100 {
		t.Fatalf("event before released %+v, want the final usage", usage)
	}

	if released.Kind != smf.SessionReleased || released.Release.Reason != models.ReleaseUERequested {
		t.Fatalf("last event %+v, want released for the UE request", released)
	}

	if cellOf(models.SessionUEInfo{Location: released.Prev.Location}) != "000000001" || cellOf(models.SessionUEInfo{Location: released.Next.Location}) != "000000009" {
		t.Fatalf("released conditions %+v -> %+v, want the last settled cell then the release's", released.Prev.Location, released.Next.Location)
	}

	if n := len(log.of(smf.SessionConditionsChanged)); n != 0 {
		t.Fatalf("%d condition changes around the release, want none", n)
	}
}

func TestAPolicyChangeIsOneConditionChangeWithEveryTrigger(t *testing.T) {
	s, log, _ := observedInterworkingSMF()

	sc := establishEPSOnENB(t, s)
	policy := committedPolicy(s, sc.Ref)
	nextRevision++

	ambr := policy.Ambr
	ambr.Downlink = models.BitRateFromBps(ambr.Downlink.Bps() * 2)

	d := &smf.PolicyDecision{
		Revision:    nextRevision,
		PolicyID:    policy.PolicyID,
		Var5qi:      policy.QosData.Var5qi + 1,
		Arp:         policy.QosData.Arp.PriorityLevel,
		SessionAMBR: ambr,
	}

	if err := s.UpdateNotify(context.Background(), sc.Ref, d); err != nil {
		t.Fatalf("UpdateNotify: %v", err)
	}

	if n := len(log.of(smf.SessionConditionsChanged)); n != 0 {
		t.Fatalf("%d condition changes before the MME accepted the modification, want none", n)
	}

	s.CommitEPSBearerModification(context.Background(), sc.Ref, true)

	changes := log.of(smf.SessionConditionsChanged)
	if len(changes) != 1 {
		t.Fatalf("condition changes %+v, want one", changes)
	}

	if got := changes[0].Triggers; !slices.Equal(got, []smf.TriggerType{smf.TriggerQoSChange, smf.TriggerSessionAMBRChange}) {
		t.Fatalf("triggers %v, want QOS_CHANGE and SESSION_AMBR_CHANGE", got)
	}
}

func TestAnObserverMayCallBackIntoTheSession(t *testing.T) {
	pcf, store, upf, amfCb, mmeCb := interworkingFakes()
	log := &eventLog{}

	var s *smf.SMF

	s = newTestSMF(pcf, store, upf, amfCb, smf.WithSessionObserver(func(e smf.SessionEvent) {
		if sc := s.GetSession(e.Ref); sc != nil {
			sc.Mutex.Lock()
			_ = sc.Access
			sc.Mutex.Unlock()
		}

		log.observe(e)
	}))
	s.SetMME(mmeCb)

	establish5GSWithUE(t, s, fiveGUEInfo("000000001"))

	if _, err := s.TransferIdle(context.Background(), models.SessionTransferRequest{SUPI: testSUPI(), PDUSessionID: movedPDUSessionID, EPSBearerIdentity: epsTestEBI, DNN: testDNN, Snssai: testSnssai, UE: eutraUEInfo("0000001")}, smf.Access4G); err != nil {
		t.Fatalf("TransferIdle: %v", err)
	}

	if got := log.kinds(); !slices.Equal(got, []smf.SessionEventKind{smf.SessionEstablished, smf.SessionConditionsChanged}) {
		t.Fatalf("events %v, want them in order", got)
	}
}
