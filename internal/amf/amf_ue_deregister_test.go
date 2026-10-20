// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package amf

import (
	"context"
	"testing"
	"time"

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/smf"
	"go.opentelemetry.io/otel/trace"
)

type deregisterTestSmf struct {
	releaseCalls          []string
	releaseCauses         []models.SessionReleaseRequest
	ueUpdates             []models.SessionUEInfoUpdate
	deactivateCalls       []string
	suppressCalls         int
	transferFailures      []models.N1N2MessageTransferCause
	failedSessions        []uint8
	clearSuppressionCalls int
	onRelease             func(context.Context, string) error

	session       func(ref string) *smf.SMContext
	reconcileRefs []string
}

func (s *deregisterTestSmf) GetSession(ref string) *smf.SMContext {
	if s.session != nil {
		return s.session(ref)
	}

	return nil
}

func (s *deregisterTestSmf) SessionsByDNN(string) []*smf.SMContext { return nil }

func (s *deregisterTestSmf) SessionCount() int { return 0 }

func (s *deregisterTestSmf) CreateSmContext(_ context.Context, _ models.SmContextCreateRequest) (string, []byte, error) {
	return "", nil, nil
}

func (s *deregisterTestSmf) ActivateSmContext(context.Context, string) ([]byte, error) {
	return nil, nil
}

func (s *deregisterTestSmf) DeactivateSmContext(_ context.Context, smContextRef string, _ bool) error {
	s.deactivateCalls = append(s.deactivateCalls, smContextRef)
	return nil
}

func (s *deregisterTestSmf) HandleN1N2TransferFailure(_ context.Context, _ etsi.SUPI, pduSessionID uint8, cause models.N1N2MessageTransferCause) error {
	s.suppressCalls++
	s.transferFailures = append(s.transferFailures, cause)
	s.failedSessions = append(s.failedSessions, pduSessionID)

	return nil
}

func (s *deregisterTestSmf) ClearPagingSuppression(_ context.Context, _ etsi.SUPI, _ uint8) error {
	s.clearSuppressionCalls++
	return nil
}

func (s *deregisterTestSmf) ReleaseSmContext(ctx context.Context, smContextRef string, cause models.SessionReleaseRequest) error {
	s.releaseCalls = append(s.releaseCalls, smContextRef)
	s.releaseCauses = append(s.releaseCauses, cause)

	if s.onRelease != nil {
		return s.onRelease(ctx, smContextRef)
	}

	return nil
}

func (s *deregisterTestSmf) UpdateSmContextN1Msg(context.Context, string, []byte) (*smf.UpdateResult, error) {
	return nil, nil
}

func (s *deregisterTestSmf) UpdateSmContextN2InfoPduResSetupRsp(context.Context, string, []byte) error {
	return nil
}

func (s *deregisterTestSmf) UpdateSmContextN2InfoPduResSetupFail(context.Context, string, []byte) error {
	return nil
}

func (s *deregisterTestSmf) UpdateSmContextN2InfoPduResRelRsp(context.Context, string) (bool, error) {
	return false, nil
}

func (s *deregisterTestSmf) UpdateSmContextCauseDuplicatePDUSessionID(context.Context, string) ([]byte, error) {
	return nil, nil
}

func (s *deregisterTestSmf) TransferIdleTo5GS(_ context.Context, _ models.SessionTransferRequest) (string, []uint8, error) {
	return "", nil, nil
}

func (s *deregisterTestSmf) PrepareSmContextFromEPS(_ context.Context, _ models.SessionTransferRequest) (string, []byte, []uint8, error) {
	return "", nil, nil, nil
}

func (s *deregisterTestSmf) HandoverAdmittedFlowEBIs(string) []uint8 { return nil }

func (s *deregisterTestSmf) ReleaseInactiveEPSBearers(context.Context, string, []uint8) {}

func (s *deregisterTestSmf) UpdateSmContextN2HandoverPreparing(context.Context, string, []byte) ([]byte, error) {
	return nil, nil
}

func (s *deregisterTestSmf) UpdateSmContextN2HandoverPrepared(context.Context, string, []byte) ([]byte, error) {
	return nil, nil
}

func (s *deregisterTestSmf) UpdateSmContextN2HandoverCanceled(context.Context, string) error {
	return nil
}

func (s *deregisterTestSmf) UpdateSmContextN2HandoverComplete(context.Context, string) error {
	return nil
}

func (s *deregisterTestSmf) UpdateSmContextXnHandoverPathSwitchReq(context.Context, string, []byte) ([]byte, error) {
	return nil, nil
}

func (s *deregisterTestSmf) UpdateSmContextN2ModifyIndication(context.Context, string, []byte) ([]byte, error) {
	return nil, nil
}

func (s *deregisterTestSmf) UpdateSmContextN2HandoverFailed(context.Context, string, []byte) error {
	return nil
}

func (s *deregisterTestSmf) UpdateSmContextXnHandoverFailed(context.Context, string, []byte) error {
	return nil
}

func (s *deregisterTestSmf) ReconcileSession(_ context.Context, ref string) error {
	s.reconcileRefs = append(s.reconcileRefs, ref)
	return nil
}

func TestDeregister_DoesNotHoldLockDuringSmfRelease(t *testing.T) {
	ue := NewUeContext()
	ue.SmContextList[1] = &SmContext{Ref: "ref-1"}
	ue.SmContextList[2] = &SmContext{Ref: "ref-2"}

	fakeSmf := &deregisterTestSmf{}
	relockCount := 0
	fakeSmf.onRelease = func(_ context.Context, _ string) error {
		ue.mu.Lock()
		_ = ue.state
		ue.mu.Unlock()

		relockCount++

		return nil
	}
	ue.smf = fakeSmf

	done := make(chan struct{})

	go func() {
		ue.Deregister(context.Background(), models.ReleaseFor(models.ReleaseDeregistration))
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Deregister appears blocked; likely held ue.Mutex while calling SMF")
	}

	if ue.state != Deregistered {
		t.Fatalf("expected state %q, got %q", Deregistered, ue.state)
	}

	if len(ue.SmContextList) != 0 {
		t.Fatalf("expected SmContextList to be cleared, got %d entries", len(ue.SmContextList))
	}

	if len(fakeSmf.releaseCalls) != 2 {
		t.Fatalf("expected 2 SM context releases, got %d", len(fakeSmf.releaseCalls))
	}

	if relockCount != 2 {
		t.Fatalf("expected to re-lock UE mutex during each release call, got %d", relockCount)
	}
}

// TS 23.501 §5.3.3.2.4
func TestRemoveAllUeInRan_Registered_DeactivatesUserPlane(t *testing.T) {
	radio := &Radio{}
	radio.BindAMFForTest(New(nil, nil, nil))

	ueConn := NewUeConnForTest(radio, 1, 10)

	ue := NewUeContext()
	ue.smf = &deregisterTestSmf{}
	ue.SmContextList[1] = &SmContext{Ref: "ref-1"}
	ue.SmContextList[2] = &SmContext{Ref: "ref-2"}
	ueConn.AMFForTest().AttachUeConn(t.Context(), ue, ueConn)
	ueConn.SetN2SessionActive(1)
	ueConn.SetN2SessionActive(2)
	ue.ForceStateForTest(Registered)

	radio.amf.RemoveAllUeInRan(context.Background(), radio)

	fakeSmf := ue.smf.(*deregisterTestSmf)

	if len(fakeSmf.deactivateCalls) != 2 {
		t.Fatalf("expected 2 SM context deactivations, got %d", len(fakeSmf.deactivateCalls))
	}

	if len(fakeSmf.releaseCalls) != 0 {
		t.Fatalf("expected no SM context releases (sessions preserved), got %d", len(fakeSmf.releaseCalls))
	}

	if len(ue.SmContextList) != 2 {
		t.Fatalf("expected SmContextList preserved with 2 entries, got %d", len(ue.SmContextList))
	}

	if ue.State() != Registered {
		t.Fatalf("expected state Registered, got %q", ue.State())
	}
}

func TestRadioRemoveUe_Registered_DeactivatesUserPlane(t *testing.T) {
	radio := &Radio{}
	radio.BindAMFForTest(New(nil, nil, nil))

	ueConn := NewUeConnForTest(radio, 1, 10)

	ue := NewUeContext()
	ue.smf = &deregisterTestSmf{}
	ue.SmContextList[1] = &SmContext{Ref: "ref-1"}
	ueConn.AMFForTest().AttachUeConn(t.Context(), ue, ueConn)
	ueConn.SetN2SessionActive(1)
	ue.ForceStateForTest(Registered)

	if err := radio.amf.RemoveUe(context.Background(), ueConn); err != nil {
		t.Fatalf("RemoveUe: %v", err)
	}

	fakeSmf := ue.smf.(*deregisterTestSmf)

	if len(fakeSmf.deactivateCalls) != 1 {
		t.Fatalf("expected 1 SM context deactivation, got %d", len(fakeSmf.deactivateCalls))
	}

	if ue.State() != Registered {
		t.Fatalf("expected state Registered, got %q", ue.State())
	}

	if radio.NumUEsForTest() != 0 {
		t.Fatalf("expected the RAN UE removed, got %d", radio.NumUEsForTest())
	}
}

func TestReconcileSessionsForUE_AsksTheSMFPerSession(t *testing.T) {
	fakeSmf := &deregisterTestSmf{}

	amfInstance := New(nil, nil, fakeSmf)

	ue := NewUeContext()
	ue.SmContextList[1] = &SmContext{Ref: "ref-1"}

	amfInstance.ReconcileSessionsForUE(context.Background(), ue)

	if len(fakeSmf.reconcileRefs) != 1 || fakeSmf.reconcileRefs[0] != "ref-1" {
		t.Fatalf("reconcile calls = %v, want [ref-1]", fakeSmf.reconcileRefs)
	}
}

func TestAttachUeConn_ClearsPagingSuppression(t *testing.T) {
	fake := &deregisterTestSmf{}
	a := New(nil, nil, nil)
	a.Session = fake

	radio := &Radio{}
	radio.BindAMFForTest(a)

	ueConn := NewUeConnForTest(radio, 1, 10)

	ue := NewUeContext()
	ue.SmContextList[1] = &SmContext{Ref: "ref-1"}
	ue.SmContextList[2] = &SmContext{Ref: "ref-2"}

	a.AttachUeConn(t.Context(), ue, ueConn)

	if fake.clearSuppressionCalls != 2 {
		t.Fatalf("clear-suppression calls = %d, want 2 (one per SM context)", fake.clearSuppressionCalls)
	}
}

func TestAbandonPaging_SuppressesAllSessions(t *testing.T) {
	fake := &deregisterTestSmf{}
	a := New(nil, nil, nil)
	a.Session = fake

	ue := NewUeContext()
	ue.SmContextList[1] = &SmContext{Ref: "ref-1"}
	ue.SmContextList[2] = &SmContext{Ref: "ref-2"}

	a.abandonPaging(trace.SpanContext{}, ue, ue.paging.attempt)

	if fake.suppressCalls != 2 {
		t.Fatalf("suppress calls = %d, want 2 (one per SM context)", fake.suppressCalls)
	}
}

// TS 23.502 §4.2.6: a UE establishing another NAS signalling connection releases the old
// one, and that release deactivates the UP connections it carried.
func TestAttachUeConn_DeactivatesTheDisplacedConnectionsUserPlane(t *testing.T) {
	fake := &deregisterTestSmf{}
	a := New(nil, nil, nil)
	a.Session = fake

	radio := &Radio{}
	radio.BindAMFForTest(a)

	ue := NewUeContext()
	ue.SmContextList[1] = &SmContext{Ref: "ref-1"}
	ue.SmContextList[2] = &SmContext{Ref: "ref-2"}
	ue.ForceStateForTest(Registered)

	first := NewUeConnForTest(radio, 1, 10)
	a.AttachUeConn(t.Context(), ue, first)
	first.SetN2SessionActive(1)

	second := NewUeConnForTest(radio, 2, 11)
	a.AttachUeConn(t.Context(), ue, second)

	if got := fake.deactivateCalls; len(got) != 1 || got[0] != "ref-1" {
		t.Errorf("DeactivateSmContext calls = %v, want only ref-1: the user plane of the displaced connection is left pointing at a released NG-RAN context", got)
	}

	if !first.N2SessionInactive(1) {
		t.Error("the displaced connection still claims to hold the AN resources it just gave up")
	}
}

func TestAttachUeConn_DoesNotDeactivateASessionTheDisplacedConnectionNeverServed(t *testing.T) {
	fake := &deregisterTestSmf{}
	a := New(nil, nil, nil)
	a.Session = fake

	radio := &Radio{}
	radio.BindAMFForTest(a)

	ue := NewUeContext()
	ue.SmContextList[1] = &SmContext{Ref: "ref-1"}
	ue.ForceStateForTest(Registered)

	first := NewUeConnForTest(radio, 1, 10)
	a.AttachUeConn(t.Context(), ue, first)

	second := NewUeConnForTest(radio, 2, 11)
	a.AttachUeConn(t.Context(), ue, second)

	if got := fake.deactivateCalls; len(got) != 0 {
		t.Errorf("DeactivateSmContext calls = %v, want none: the displaced connection held no AN resources", got)
	}
}

func (*deregisterTestSmf) UpdateSmContextN2InfoPduResModifyRsp(context.Context, string, []byte) error {
	return nil
}

func (*deregisterTestSmf) UpdateSmContextN2InfoPduResModifyFail(context.Context, string, []byte) error {
	return nil
}

func (*deregisterTestSmf) UpdateSmContextN2InfoNotify(context.Context, string, []byte) error {
	return nil
}

func TestDeregister_ReleasesSessionsAsADeregistration(t *testing.T) {
	ue := NewUeContext()
	ue.SmContextList[1] = &SmContext{Ref: "ref-1"}

	fakeSmf := &deregisterTestSmf{}
	ue.smf = fakeSmf

	ue.Deregister(context.Background(), models.ReleaseFor(models.ReleaseDeregistration))

	if len(fakeSmf.releaseCauses) != 1 || fakeSmf.releaseCauses[0].Reason != models.ReleaseDeregistration {
		t.Fatalf("release causes = %+v, want one deregistration", fakeSmf.releaseCauses)
	}
}

func TestNetworkDeregistrationCauseAppliesOnlyWhileDeregistrationIsInitiated(t *testing.T) {
	ue := NewUeContext()
	withdrawn := models.ReleaseFor(models.ReleaseSubscriptionWithdrawn)
	fallback := models.ReleaseFor(models.ReleaseDeregistration)

	ue.ForceStateForTest(Registered)
	ue.startNetworkDeregistration(withdrawn)

	if got := ue.NetworkDeregistrationOr(fallback); got.Reason != models.ReleaseDeregistration {
		t.Fatalf("reason before the request is outstanding = %s, want the fallback", got.Reason)
	}

	ue.ForceStateForTest(DeregistrationInitiated)

	if got := ue.NetworkDeregistrationOr(fallback); got.Reason != models.ReleaseSubscriptionWithdrawn {
		t.Fatalf("reason while deregistration is initiated = %s, want subscription_withdrawn", got.Reason)
	}

	ue.ForceStateForTest(Deregistered)
	ue.TransitionTo(context.Background(), RegistrationInitiated)
	ue.ForceStateForTest(DeregistrationInitiated)

	if got := ue.NetworkDeregistrationOr(fallback); got.Reason != models.ReleaseDeregistration {
		t.Fatalf("reason after a new registration began = %s, want the fallback", got.Reason)
	}
}

func (s *deregisterTestSmf) UpdateUEInfo(_ context.Context, _ etsi.SUPI, update models.SessionUEInfoUpdate) {
	s.ueUpdates = append(s.ueUpdates, update)
}

func TestALocationChangeIsReportedToTheSMFOnce(t *testing.T) {
	ue := NewUeContext()
	fakeSmf := &deregisterTestSmf{}
	ue.smf = fakeSmf

	supi, err := etsi.NewSUPIFromIMSI("001010000000001")
	if err != nil {
		t.Fatalf("SUPI: %v", err)
	}

	ue.supi = supi

	plmn := &models.PlmnID{Mcc: "001", Mnc: "01"}
	tai := models.Tai{PlmnID: plmn, Tac: "000001"}
	cell := func(id string) models.UserLocation {
		return models.UserLocation{NrLocation: &models.NrLocation{Tai: &tai, Ncgi: &models.Ncgi{PlmnID: plmn, NrCellID: id}}}
	}

	for _, loc := range []models.UserLocation{cell("000000001"), cell("000000001"), cell("000000002")} {
		if ue.SetLocation(loc, tai) {
			ue.ReportLocationChange(context.Background())
		}
	}

	if len(fakeSmf.ueUpdates) != 2 {
		t.Fatalf("UE information updates = %d, want 2: the first fix and the cell change", len(fakeSmf.ueUpdates))
	}

	if rat := fakeSmf.ueUpdates[1].RATType; rat == nil || *rat != models.RATTypeNR {
		t.Fatalf("RAT type = %v, want NR", rat)
	}
}

func TestALearnedPEIIsReportedOnceAndAnEmptyLocationNever(t *testing.T) {
	ue := NewUeContext()
	fakeSmf := &deregisterTestSmf{}
	ue.smf = fakeSmf

	supi, err := etsi.NewSUPIFromIMSI("001010000000001")
	if err != nil {
		t.Fatalf("SUPI: %v", err)
	}

	ue.supi = supi

	imei, err := etsi.NewIMEIFromPEI("imeisv-3534567890123401")
	if err != nil {
		t.Fatalf("IMEI: %v", err)
	}

	for range 2 {
		if ue.SetImei(imei) {
			ue.ReportPEIChange(context.Background())
		}
	}

	if ue.SetLocation(models.UserLocation{}, models.Tai{}) {
		ue.ReportLocationChange(context.Background())
	}

	if len(fakeSmf.ueUpdates) != 1 || fakeSmf.ueUpdates[0].PEI == nil || *fakeSmf.ueUpdates[0].PEI != "imeisv-3534567890123401" {
		t.Fatalf("UE information updates = %+v, want only the PEI once", fakeSmf.ueUpdates)
	}
}

func TestAnN3IWFLocationIsPublishedOnBind(t *testing.T) {
	ue := NewUeContext()
	conn := &UeConn{}
	conn.Location = models.UserLocation{N3gaLocation: &models.N3gaLocation{UeIpv4Addr: "192.0.2.1"}}

	if !conn.PublishLocation(ue) {
		t.Fatal("an N3IWF location was not published")
	}

	if loc := ue.GetUserLocation(); loc.N3gaLocation == nil {
		t.Fatalf("UE location = %+v, want the N3IWF location", loc)
	}
}
