// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"context"
	"errors"
	"sync"
	"testing"
)

type fakeRegistrar struct {
	mu          sync.Mutex
	registerErr error
	next        int64
	registered  []string
	confirmed   map[string]bool
	superseded  map[string]bool
	withdrawn   map[string]bool
	held        map[string]int64
	purged      []string
}

func newFakeRegistrar() *fakeRegistrar {
	return &fakeRegistrar{
		next:       100,
		confirmed:  map[string]bool{},
		superseded: map[string]bool{},
		withdrawn:  map[string]bool{},
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

func (f *fakeRegistrar) Reconcile(ctx context.Context, imsi string, held func() int64, release func(context.Context), withdraw func(context.Context)) {
	f.mu.Lock()
	f.held[imsi] = held()
	supersede := f.superseded[imsi]
	withdrawn := f.withdrawn[imsi]
	f.mu.Unlock()

	switch {
	case withdrawn:
		withdraw(ctx)
	case supersede:
		release(ctx)
	}
}

func newRegistrarTestMME(t *testing.T) (*MME, *fakeRegistrar) {
	t.Helper()

	m := newTestMME(t)
	reg := newFakeRegistrar()
	m.Registrations = reg

	return m, reg
}

func newRegisteredTestUE(m *MME, imsi string) *UeContext {
	ue := NewUeContext()
	m.RegisterUEForTest(ue, imsi)
	ue.TransitionTo(context.Background(), EMMRegistrationInitiated)
	ue.TransitionTo(context.Background(), EMMRegistered)

	return ue
}

func TestRegisterUE_StoresVersion(t *testing.T) {
	m, _ := newRegistrarTestMME(t)
	ue := newRegisteredTestUE(m, "001010000000001")

	if err := m.RegisterUE(context.Background(), ue); err != nil {
		t.Fatalf("RegisterUE: %s", err)
	}

	if got := ue.registrationVersion.Load(); got != 101 {
		t.Fatalf("registration version = %d, want 101", got)
	}
}

func TestRegisterUE_ErrorKeepsVersion(t *testing.T) {
	m, reg := newRegistrarTestMME(t)
	reg.registerErr = errors.New("no leader")
	ue := newRegisteredTestUE(m, "001010000000001")
	ue.registrationVersion.Store(7)

	if err := m.RegisterUE(context.Background(), ue); err == nil {
		t.Fatal("expected error")
	}

	if got := ue.registrationVersion.Load(); got != 7 {
		t.Fatalf("registration version = %d, want 7", got)
	}
}

func TestConfirmRegistration(t *testing.T) {
	m, reg := newRegistrarTestMME(t)
	ue := newRegisteredTestUE(m, "001010000000001")

	reg.confirmed["001010000000001"] = true

	if err := m.ConfirmRegistration(context.Background(), ue); err != nil {
		t.Fatalf("ConfirmRegistration: %s", err)
	}

	if len(reg.registered) != 0 {
		t.Fatalf("a confirmed registration must not be written again, got %v", reg.registered)
	}

	reg.confirmed["001010000000001"] = false

	if err := m.ConfirmRegistration(context.Background(), ue); err != nil {
		t.Fatalf("ConfirmRegistration: %s", err)
	}

	if len(reg.registered) != 1 || ue.registrationVersion.Load() != 101 {
		t.Fatalf("an unconfirmed registration must be written, got %v (version %d)", reg.registered, ue.registrationVersion.Load())
	}
}

func TestRemoveUe_QueuesPurge(t *testing.T) {
	m, reg := newRegistrarTestMME(t)
	ue := newRegisteredTestUE(m, "001010000000001")

	m.RemoveUe(ue)

	if len(reg.purged) != 1 || reg.purged[0] != "001010000000001" {
		t.Fatalf("purges = %v, want the removed UE", reg.purged)
	}
}

func TestRemoveUe_NotIndexedDoesNotPurge(t *testing.T) {
	m, reg := newRegistrarTestMME(t)
	old := newRegisteredTestUE(m, "001010000000001")
	newRegisteredTestUE(m, "001010000000001")

	m.RemoveUe(old)

	if len(reg.purged) != 0 {
		t.Fatalf("unexpected purges %v", reg.purged)
	}
}

func TestReconcileRegistration_ReleasesSupersededUE(t *testing.T) {
	m, reg := newRegistrarTestMME(t)
	moved := newRegisteredTestUE(m, "001010000000001")
	kept := newRegisteredTestUE(m, "001010000000002")

	moved.registrationVersion.Store(42)

	reg.superseded["001010000000001"] = true

	m.ReconcileRegistrations(context.Background())

	if m.HoldsUE("001010000000001") || moved.EMMState() != EMMDeregistered {
		t.Fatal("expected the superseded UE to be deregistered and removed")
	}

	if held, ok := m.LookupUeByIMSI("001010000000002"); !ok || held != kept || kept.EMMState() != EMMRegistered {
		t.Fatal("expected the other UE to be kept")
	}

	if reg.held["001010000000001"] != 42 {
		t.Fatalf("held version = %d, want 42", reg.held["001010000000001"])
	}
}

func TestReconcileRegistration_DetachesWithdrawnIdleUE(t *testing.T) {
	m, reg := newRegistrarTestMME(t)
	withdrawn := newRegisteredTestUE(m, "001010000000001")
	kept := newRegisteredTestUE(m, "001010000000002")

	withdrawn.registrationVersion.Store(42)

	reg.withdrawn["001010000000001"] = true

	m.ReconcileRegistrations(context.Background())

	if m.HoldsUE("001010000000001") || withdrawn.EMMState() != EMMDeregistered {
		t.Fatal("expected the withdrawn UE to be detached and removed")
	}

	if _, ok := m.LastSeenAll()["001010000000001"]; ok {
		t.Fatal("expected the withdrawn subscriber's last-seen entry to be forgotten")
	}

	if held, ok := m.LookupUeByIMSI("001010000000002"); !ok || held != kept || kept.EMMState() != EMMRegistered {
		t.Fatal("expected the other UE to be kept")
	}
}

func TestReconcileRegistration_IgnoresDeregisteredUE(t *testing.T) {
	m, reg := newRegistrarTestMME(t)
	ue := NewUeContext()
	m.RegisterUEForTest(ue, "001010000000001")

	reg.superseded["001010000000001"] = true

	m.ReconcileRegistration(context.Background(), "001010000000001")

	if held, ok := m.LookupUeByIMSI("001010000000001"); !ok || held != ue {
		t.Fatal("expected the deregistered UE context to be left alone")
	}

	if _, checked := reg.held["001010000000001"]; checked {
		t.Fatal("expected no registration check for a deregistered UE")
	}
}
