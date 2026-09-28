// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

type fakeRegistrar struct {
	mu          sync.Mutex
	registerErr error
	registered  []string
	superseded  map[string]bool
	checkedAt   map[string]int64
	purged      chan purgeCall
}

type purgeCall struct {
	imsi   string
	absent bool
}

func newFakeRegistrar() *fakeRegistrar {
	return &fakeRegistrar{
		superseded: map[string]bool{},
		checkedAt:  map[string]int64{},
		purged:     make(chan purgeCall, 8),
	}
}

func (f *fakeRegistrar) Register(_ context.Context, imsi string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.registerErr != nil {
		return f.registerErr
	}

	f.registered = append(f.registered, imsi)

	return nil
}

func (f *fakeRegistrar) Purge(_ context.Context, imsi string, stillAbsent func() bool) {
	f.purged <- purgeCall{imsi: imsi, absent: stillAbsent()}
}

func (f *fakeRegistrar) Superseded(_ context.Context, imsi string, registeredAt int64) bool {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.checkedAt[imsi] = registeredAt

	return f.superseded[imsi]
}

func newRegisteredTestUE(m *MME, imsi string) *UeContext {
	ue := NewUeContext()
	m.RegisterUEForTest(ue, imsi)
	ue.TransitionTo(context.Background(), EMMRegistrationInitiated)
	ue.TransitionTo(context.Background(), EMMRegistered)

	return ue
}

func TestRegisterUE_RecordsRegistrationTime(t *testing.T) {
	m := newTestMME(t)
	reg := newFakeRegistrar()
	m.Registrations = reg
	ue := newRegisteredTestUE(m, "001010000000001")

	before := time.Now().UnixMilli()

	if err := m.RegisterUE(context.Background(), ue); err != nil {
		t.Fatalf("RegisterUE: %s", err)
	}

	if got := ue.registeredAt.Load(); got < before {
		t.Fatalf("registeredAt = %d, want >= %d", got, before)
	}

	if len(reg.registered) != 1 || reg.registered[0] != "001010000000001" {
		t.Fatalf("unexpected registrations %v", reg.registered)
	}
}

func TestRegisterUE_ErrorKeepsRegistrationTime(t *testing.T) {
	m := newTestMME(t)
	reg := newFakeRegistrar()
	reg.registerErr = errors.New("no leader")
	m.Registrations = reg
	ue := newRegisteredTestUE(m, "001010000000001")

	if err := m.RegisterUE(context.Background(), ue); err == nil {
		t.Fatal("expected error")
	}

	if got := ue.registeredAt.Load(); got != 0 {
		t.Fatalf("registeredAt = %d, want 0", got)
	}
}

func TestRemoveUe_PurgesRegistration(t *testing.T) {
	m := newTestMME(t)
	reg := newFakeRegistrar()
	m.Registrations = reg
	ue := newRegisteredTestUE(m, "001010000000001")

	m.RemoveUe(ue)

	select {
	case call := <-reg.purged:
		if call.imsi != "001010000000001" || !call.absent {
			t.Fatalf("unexpected purge %+v", call)
		}
	case <-time.After(time.Second):
		t.Fatal("expected a purge")
	}
}

func TestRemoveUe_NotIndexedDoesNotPurge(t *testing.T) {
	m := newTestMME(t)
	reg := newFakeRegistrar()
	m.Registrations = reg
	old := newRegisteredTestUE(m, "001010000000001")
	newRegisteredTestUE(m, "001010000000001")

	m.RemoveUe(old)

	select {
	case call := <-reg.purged:
		t.Fatalf("unexpected purge %+v", call)
	case <-time.After(50 * time.Millisecond):
	}
}

func TestReconcileRegistrations_ReleasesSupersededUE(t *testing.T) {
	m := newTestMME(t)
	reg := newFakeRegistrar()
	m.Registrations = reg
	moved := newRegisteredTestUE(m, "001010000000001")
	kept := newRegisteredTestUE(m, "001010000000002")

	moved.registeredAt.Store(42)

	reg.superseded["001010000000001"] = true

	m.ReconcileRegistrations(context.Background())

	if _, ok := m.LookupUeByIMSI("001010000000001"); ok {
		t.Fatal("expected the superseded UE context to be removed")
	}

	if moved.EMMState() != EMMDeregistered {
		t.Fatalf("superseded UE state = %v, want Deregistered", moved.EMMState())
	}

	if held, ok := m.LookupUeByIMSI("001010000000002"); !ok || held != kept || kept.EMMState() != EMMRegistered {
		t.Fatal("expected the other UE to be kept")
	}

	if reg.checkedAt["001010000000001"] != 42 {
		t.Fatalf("Superseded checked with registeredAt %d, want 42", reg.checkedAt["001010000000001"])
	}
}

func TestReconcileRegistrations_IgnoresDeregisteredUE(t *testing.T) {
	m := newTestMME(t)
	reg := newFakeRegistrar()
	m.Registrations = reg
	ue := NewUeContext()
	m.RegisterUEForTest(ue, "001010000000001")

	reg.superseded["001010000000001"] = true

	m.ReconcileRegistrations(context.Background())

	if held, ok := m.LookupUeByIMSI("001010000000001"); !ok || held != ue {
		t.Fatal("expected the deregistered UE context to be left alone")
	}

	if _, checked := reg.checkedAt["001010000000001"]; checked {
		t.Fatal("expected no registration check for a deregistered UE")
	}
}
