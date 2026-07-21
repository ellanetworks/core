// SPDX-FileCopyrightText: Ella Networks Inc.
//
// SPDX-License-Identifier: BUSL-1.1

package amf

import (
	"context"
	"testing"
	"time"

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/smf"
)

type deregisterTestSmf struct {
	releaseCalls    []string
	deactivateCalls []string
	onRelease       func(context.Context, string) error

	// Optional hooks for reconcile tests; nil keeps the default (no-op) behaviour.
	session       func(ref string) *smf.SMContext
	sessionPolicy func() (*smf.Policy, error)
	reconcileReqs []*models.SessionReconcileRequest
}

func (s *deregisterTestSmf) GetSession(ref string) *smf.SMContext {
	if s.session != nil {
		return s.session(ref)
	}

	return nil
}

func (s *deregisterTestSmf) SessionsByDNN(string) []*smf.SMContext { return nil }

func (s *deregisterTestSmf) SessionCount() int { return 0 }

func (s *deregisterTestSmf) CreateSmContext(context.Context, etsi.SUPI, uint8, string, *models.Snssai, []byte) (string, []byte, error) {
	return "", nil, nil
}

func (s *deregisterTestSmf) ActivateSmContext(context.Context, string) ([]byte, error) {
	return nil, nil
}

func (s *deregisterTestSmf) DeactivateSmContext(_ context.Context, smContextRef string) error {
	s.deactivateCalls = append(s.deactivateCalls, smContextRef)
	return nil
}

func (s *deregisterTestSmf) HandlePagingFailure(_ context.Context, _ etsi.SUPI, _ uint8) error {
	return nil
}

func (s *deregisterTestSmf) ReleaseSmContext(ctx context.Context, smContextRef string) error {
	s.releaseCalls = append(s.releaseCalls, smContextRef)
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

func (s *deregisterTestSmf) UpdateSmContextN2InfoPduResRelRsp(context.Context, string) error {
	return nil
}

func (s *deregisterTestSmf) UpdateSmContextCauseDuplicatePDUSessionID(context.Context, string) ([]byte, error) {
	return nil, nil
}

func (s *deregisterTestSmf) UpdateSmContextN2HandoverPreparing(context.Context, string, []byte) ([]byte, error) {
	return nil, nil
}

func (s *deregisterTestSmf) UpdateSmContextN2HandoverPrepared(context.Context, string, []byte) ([]byte, error) {
	return nil, nil
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

func (s *deregisterTestSmf) UpdateSmContextHandoverFailed(context.Context, string, []byte) error {
	return nil
}

func (s *deregisterTestSmf) ReconcileSmContext(_ context.Context, req *models.SessionReconcileRequest) error {
	s.reconcileReqs = append(s.reconcileReqs, req)
	return nil
}

func (s *deregisterTestSmf) GetSessionPolicy(context.Context, etsi.SUPI, *models.Snssai, string) (*smf.Policy, error) {
	if s.sessionPolicy != nil {
		return s.sessionPolicy()
	}

	return nil, nil
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
		ue.Deregister(context.Background())
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

// On abrupt NG-C loss (NG Reset / association drop) a registered UE deactivates
// the user plane so the UPF stops sending downlink toward the lost RAN and buffers
// it for paging, while the sessions are preserved (TS 23.501 §5.3.3.2.4).
func TestRemoveAllUeInRan_Registered_DeactivatesUserPlane(t *testing.T) {
	radio := &Radio{Log: logger.AmfLog}
	radio.BindAMFForTest(New(nil, nil, nil))

	ueConn := NewUeConnForTest(radio, 1, 10, logger.AmfLog)

	ue := NewUeContext()
	ue.smf = &deregisterTestSmf{}
	ue.SmContextList[1] = &SmContext{Ref: "ref-1"}
	ue.SmContextList[2] = &SmContext{Ref: "ref-2"}
	ueConn.AMFForTest().AttachUeConn(ue, ueConn)
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

// A partial NG Reset removes one specific UE; a registered one must get the same
// idle-supervision cleanup (deactivate user plane, stay registered) as a whole
// reset, so Radio.RemoveUe applies the stateful cleanup before removing.
func TestRadioRemoveUe_Registered_DeactivatesUserPlane(t *testing.T) {
	radio := &Radio{Log: logger.AmfLog}
	radio.BindAMFForTest(New(nil, nil, nil))

	ueConn := NewUeConnForTest(radio, 1, 10, logger.AmfLog)

	ue := NewUeContext()
	ue.smf = &deregisterTestSmf{}
	ue.SmContextList[1] = &SmContext{Ref: "ref-1"}
	ueConn.AMFForTest().AttachUeConn(ue, ueConn)
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

// ReconcileSessionsForUE is the per-UE reconcile the reactivation hook runs when a UE
// returns to CM-CONNECTED (item 8): it must re-resolve the current DB policy and pass
// it to the SMF for each of the UE's sessions, so an idle-deferred change is applied
// on reconnect.
func TestReconcileSessionsForUE_AppliesResolvedPolicy(t *testing.T) {
	fakeSmf := &deregisterTestSmf{
		session: func(string) *smf.SMContext {
			return &smf.SMContext{Dnn: "internet", Snssai: &models.Snssai{Sst: 1}}
		},
		sessionPolicy: func() (*smf.Policy, error) {
			return &smf.Policy{
				Ambr:    models.Ambr{Uplink: "1 Gbps", Downlink: "2 Gbps"},
				QosData: models.QosData{Var5qi: 9, Arp: &models.Arp{PriorityLevel: 1}},
			}, nil
		},
	}

	amfInstance := New(nil, nil, fakeSmf)

	ue := NewUeContext()
	ue.SmContextList[1] = &SmContext{Ref: "ref-1"}

	amfInstance.ReconcileSessionsForUE(context.Background(), ue)

	if len(fakeSmf.reconcileReqs) != 1 {
		t.Fatalf("expected 1 reconcile call, got %d", len(fakeSmf.reconcileReqs))
	}

	req := fakeSmf.reconcileReqs[0]

	if req.SmContextRef != "ref-1" {
		t.Fatalf("reconcile ref = %q, want ref-1", req.SmContextRef)
	}

	if req.Reason != models.ReconcilePolicyChange {
		t.Fatalf("reconcile reason = %q, want policy-change", req.Reason)
	}

	if req.NewPolicy == nil || req.NewPolicy.SessionAmbrDownlink != "2 Gbps" {
		t.Fatalf("reconcile did not carry the resolved policy: %+v", req.NewPolicy)
	}
}
