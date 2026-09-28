// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package amf

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/ellanetworks/core/etsi"
)

type fakeRegistrar struct {
	mu          sync.Mutex
	registerErr error
	next        int64
	registered  []string
	confirmed   map[string]bool
	superseded  map[string]bool
	held        map[string]int64
	purged      []string
}

func newFakeRegistrar() *fakeRegistrar {
	return &fakeRegistrar{
		next:       100,
		confirmed:  map[string]bool{},
		superseded: map[string]bool{},
		held:       map[string]int64{},
	}
}

func (f *fakeRegistrar) Register(_ context.Context, imsi string) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.registerErr != nil {
		return 0, f.registerErr
	}

	f.next++
	f.registered = append(f.registered, imsi)

	return f.next, nil
}

func (f *fakeRegistrar) Confirmed(_ context.Context, imsi string, _ int64) bool {
	f.mu.Lock()
	defer f.mu.Unlock()

	return f.confirmed[imsi]
}

func (f *fakeRegistrar) Purge(imsi string) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.purged = append(f.purged, imsi)
}

func (f *fakeRegistrar) Reconcile(ctx context.Context, imsi string, held func() int64, release func(context.Context)) {
	f.mu.Lock()
	f.held[imsi] = held()
	supersede := f.superseded[imsi]
	f.mu.Unlock()

	if supersede {
		release(ctx)
	}
}

func newRegisteredTestUE(t *testing.T, a *AMF, imsi string) *UeContext {
	t.Helper()

	supi, err := etsi.NewSUPIFromIMSI(imsi)
	if err != nil {
		t.Fatalf("NewSUPIFromIMSI: %s", err)
	}

	ue := NewUeContext()
	ue.SetSupiForTest(supi)
	ue.ForceStateForTest(Registered)

	if err := a.AddUeContextToPoolForTest(ue); err != nil {
		t.Fatalf("AddUeContextToPoolForTest: %s", err)
	}

	return ue
}

func newRegistrarTestAMF() (*AMF, *fakeRegistrar) {
	a := New(nil, nil, nil)
	reg := newFakeRegistrar()
	a.Registrations = reg

	return a, reg
}

func connectTestUE(t *testing.T, a *AMF, ue *UeContext) *UeConn {
	t.Helper()

	radio := &Radio{Conn: nopNGAPSender{}}
	radio.BindAMFForTest(a)

	ueConn := NewUeConnForTest(radio, 1, 10)
	a.AttachUeConn(t.Context(), ue, ueConn)

	return ueConn
}

func TestRegisterUE_StoresVersion(t *testing.T) {
	a, _ := newRegistrarTestAMF()
	ue := newRegisteredTestUE(t, a, "001010000000001")

	if err := a.RegisterUE(context.Background(), ue); err != nil {
		t.Fatalf("RegisterUE: %s", err)
	}

	if got := ue.registrationVersion.Load(); got != 101 {
		t.Fatalf("registration version = %d, want 101", got)
	}
}

func TestRegisterUE_ErrorKeepsVersion(t *testing.T) {
	a, reg := newRegistrarTestAMF()
	reg.registerErr = errors.New("no leader")
	ue := newRegisteredTestUE(t, a, "001010000000001")
	ue.registrationVersion.Store(7)

	if err := a.RegisterUE(context.Background(), ue); err == nil {
		t.Fatal("expected error")
	}

	if got := ue.registrationVersion.Load(); got != 7 {
		t.Fatalf("registration version = %d, want 7", got)
	}
}

func TestRegisterUE_WithoutRegistrar(t *testing.T) {
	a := New(nil, nil, nil)
	ue := newRegisteredTestUE(t, a, "001010000000001")

	if err := a.RegisterUE(context.Background(), ue); err != nil {
		t.Fatalf("RegisterUE: %s", err)
	}
}

func TestConfirmRegistration(t *testing.T) {
	a, reg := newRegistrarTestAMF()
	ue := newRegisteredTestUE(t, a, "001010000000001")

	reg.confirmed["001010000000001"] = true

	if err := a.ConfirmRegistration(context.Background(), ue); err != nil {
		t.Fatalf("ConfirmRegistration: %s", err)
	}

	if len(reg.registered) != 0 {
		t.Fatalf("a confirmed registration must not be written again, got %v", reg.registered)
	}

	reg.confirmed["001010000000001"] = false

	if err := a.ConfirmRegistration(context.Background(), ue); err != nil {
		t.Fatalf("ConfirmRegistration: %s", err)
	}

	if len(reg.registered) != 1 || ue.registrationVersion.Load() != 101 {
		t.Fatalf("an unconfirmed registration must be written, got %v (version %d)", reg.registered, ue.registrationVersion.Load())
	}
}

func TestDeregisterAndRemoveUeContext_QueuesPurge(t *testing.T) {
	a, reg := newRegistrarTestAMF()
	ue := newRegisteredTestUE(t, a, "001010000000001")

	a.DeregisterAndRemoveUeContext(context.Background(), ue)

	if len(reg.purged) != 1 || reg.purged[0] != "001010000000001" {
		t.Fatalf("purges = %v, want the removed UE", reg.purged)
	}

	if a.HoldsUE("001010000000001") {
		t.Fatal("expected the UE context to be gone")
	}
}

func TestDeregisterAndRemoveUeContext_SupersededContextDoesNotPurge(t *testing.T) {
	a, reg := newRegistrarTestAMF()
	old := newRegisteredTestUE(t, a, "001010000000001")
	newRegisteredTestUE(t, a, "001010000000001")

	a.DeregisterAndRemoveUeContext(context.Background(), old)

	if len(reg.purged) != 0 {
		t.Fatalf("unexpected purges %v", reg.purged)
	}

	if !a.HoldsUE("001010000000001") {
		t.Fatal("expected the new context to remain")
	}
}

func TestReconcileRegistration_ReleasesIdleUE(t *testing.T) {
	a, reg := newRegistrarTestAMF()
	moved := newRegisteredTestUE(t, a, "001010000000001")
	kept := newRegisteredTestUE(t, a, "001010000000002")

	moved.registrationVersion.Store(42)

	reg.superseded["001010000000001"] = true

	a.ReconcileRegistrations(context.Background())

	if a.ServesUeContext(moved) || moved.State() != Deregistered {
		t.Fatal("expected the superseded UE to be deregistered and removed")
	}

	if !a.ServesUeContext(kept) || kept.State() != Registered {
		t.Fatal("expected the other UE to be kept")
	}

	if reg.held["001010000000001"] != 42 {
		t.Fatalf("held version = %d, want 42", reg.held["001010000000001"])
	}
}

func TestReconcileRegistration_ReleasesConnectedUE(t *testing.T) {
	a, reg := newRegistrarTestAMF()
	ue := newRegisteredTestUE(t, a, "001010000000001")
	ueConn := connectTestUE(t, a, ue)

	reg.superseded["001010000000001"] = true

	a.ReconcileRegistration(context.Background(), "001010000000001")

	if !a.ReleaseClaimed(ueConn) {
		t.Fatal("expected a UE Context Release Command")
	}

	if ueConn.ReleaseAction() != UeContextReleaseDueToNwInitiatedDeregistraion {
		t.Fatalf("release action = %v, want network-initiated deregistration", ueConn.ReleaseAction())
	}

	if ue.State() != Deregistered {
		t.Fatalf("state = %v, want Deregistered", ue.State())
	}
}

func TestReconcileRegistration_LeavesInFlightReleaseAlone(t *testing.T) {
	a, reg := newRegistrarTestAMF()
	ue := newRegisteredTestUE(t, a, "001010000000001")
	ueConn := connectTestUE(t, a, ue)

	ueConn.SetReleaseAction(UeContextReleaseHandover)

	if !a.claimRelease(ueConn) {
		t.Fatal("claimRelease")
	}

	reg.superseded["001010000000001"] = true

	a.ReconcileRegistration(context.Background(), "001010000000001")

	if ueConn.ReleaseAction() != UeContextReleaseHandover {
		t.Fatalf("release action = %v, want the in-flight handover release", ueConn.ReleaseAction())
	}

	if ue.State() != Registered {
		t.Fatalf("state = %v, want Registered until the in-flight release completes", ue.State())
	}
}

func TestReconcileRegistration_IgnoresDeregisteredUE(t *testing.T) {
	a, reg := newRegistrarTestAMF()
	ue := newRegisteredTestUE(t, a, "001010000000001")
	ue.ForceStateForTest(Deregistered)

	reg.superseded["001010000000001"] = true

	a.ReconcileRegistration(context.Background(), "001010000000001")

	if !a.ServesUeContext(ue) {
		t.Fatal("expected the deregistered UE context to be left alone")
	}

	if _, checked := reg.held["001010000000001"]; checked {
		t.Fatal("expected no registration check for a deregistered UE")
	}
}

func TestCompleteRelocationFromEPS_RegistersUE(t *testing.T) {
	a, reg := newRegistrarTestAMF()

	supi, err := etsi.NewSUPIFromIMSI("001010000000001")
	if err != nil {
		t.Fatalf("NewSUPIFromIMSI: %s", err)
	}

	ue := NewUeContext()
	ue.SetSupiForTest(supi)

	if !a.beginRelocationFromEPS(supi, 1, ue) {
		t.Fatal("beginRelocationFromEPS")
	}

	a.CompleteRelocationFromEPS(context.Background(), ue)

	if len(reg.registered) != 1 || reg.registered[0] != "001010000000001" || ue.registrationVersion.Load() == 0 {
		t.Fatalf("the arrived UE was not registered: %v (version %d)", reg.registered, ue.registrationVersion.Load())
	}
}
