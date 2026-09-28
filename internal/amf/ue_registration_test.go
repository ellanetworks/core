// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package amf

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/ellanetworks/core/etsi"
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

func TestRegisterUE_RecordsRegistrationTime(t *testing.T) {
	a := New(nil, nil, nil)
	reg := newFakeRegistrar()
	a.Registrations = reg
	ue := newRegisteredTestUE(t, a, "001010000000001")

	before := time.Now().UnixMilli()

	if err := a.RegisterUE(context.Background(), ue); err != nil {
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
	a := New(nil, nil, nil)
	reg := newFakeRegistrar()
	reg.registerErr = errors.New("no leader")
	a.Registrations = reg
	ue := newRegisteredTestUE(t, a, "001010000000001")

	if err := a.RegisterUE(context.Background(), ue); err == nil {
		t.Fatal("expected error")
	}

	if got := ue.registeredAt.Load(); got != 0 {
		t.Fatalf("registeredAt = %d, want 0", got)
	}
}

func TestRegisterUE_WithoutRegistrar(t *testing.T) {
	a := New(nil, nil, nil)
	ue := newRegisteredTestUE(t, a, "001010000000001")

	if err := a.RegisterUE(context.Background(), ue); err != nil {
		t.Fatalf("RegisterUE: %s", err)
	}
}

func TestDeregisterAndRemoveUeContext_PurgesRegistration(t *testing.T) {
	a := New(nil, nil, nil)
	reg := newFakeRegistrar()
	a.Registrations = reg
	ue := newRegisteredTestUE(t, a, "001010000000001")

	a.DeregisterAndRemoveUeContext(context.Background(), ue)

	select {
	case call := <-reg.purged:
		if call.imsi != "001010000000001" || !call.absent {
			t.Fatalf("unexpected purge %+v", call)
		}
	case <-time.After(time.Second):
		t.Fatal("expected a purge")
	}
}

func TestDeregisterAndRemoveUeContext_SupersededContextDoesNotPurge(t *testing.T) {
	a := New(nil, nil, nil)
	reg := newFakeRegistrar()
	a.Registrations = reg
	old := newRegisteredTestUE(t, a, "001010000000001")
	newRegisteredTestUE(t, a, "001010000000001")

	a.DeregisterAndRemoveUeContext(context.Background(), old)

	select {
	case call := <-reg.purged:
		t.Fatalf("unexpected purge %+v", call)
	case <-time.After(50 * time.Millisecond):
	}
}

func TestReconcileRegistrations_ReleasesSupersededUE(t *testing.T) {
	a := New(nil, nil, nil)
	reg := newFakeRegistrar()
	a.Registrations = reg
	moved := newRegisteredTestUE(t, a, "001010000000001")
	kept := newRegisteredTestUE(t, a, "001010000000002")

	moved.registeredAt.Store(42)

	reg.superseded["001010000000001"] = true

	a.ReconcileRegistrations(context.Background())

	if a.ServesUeContext(moved) {
		t.Fatal("expected the superseded UE context to be removed")
	}

	if moved.State() != Deregistered {
		t.Fatalf("superseded UE state = %v, want Deregistered", moved.State())
	}

	if !a.ServesUeContext(kept) || kept.State() != Registered {
		t.Fatal("expected the other UE to be kept")
	}

	if reg.checkedAt["001010000000001"] != 42 {
		t.Fatalf("Superseded checked with registeredAt %d, want 42", reg.checkedAt["001010000000001"])
	}
}

func TestReconcileRegistrations_IgnoresDeregisteredUE(t *testing.T) {
	a := New(nil, nil, nil)
	reg := newFakeRegistrar()
	a.Registrations = reg
	ue := newRegisteredTestUE(t, a, "001010000000001")
	ue.ForceStateForTest(Deregistered)

	reg.superseded["001010000000001"] = true

	a.ReconcileRegistrations(context.Background())

	if !a.ServesUeContext(ue) {
		t.Fatal("expected the deregistered UE context to be left alone")
	}

	if _, checked := reg.checkedAt["001010000000001"]; checked {
		t.Fatal("expected no registration check for a deregistered UE")
	}
}
