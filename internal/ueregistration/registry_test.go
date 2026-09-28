// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ueregistration_test

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/ueregistration"
	"go.uber.org/zap"
)

const imsi = "001010000000001"

type fakeStore struct {
	mu        sync.Mutex
	rows      map[string]db.UERegistration
	now       int64
	err       error
	registers int
	purges    int
}

func newFakeStore() *fakeStore {
	return &fakeStore{rows: map[string]db.UERegistration{}, now: 1000}
}

func (f *fakeStore) RegisterUE(_ context.Context, imsi, regType, nodeID string, cancel ...string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.registers++

	if f.err != nil {
		return f.err
	}

	f.now++

	row, ok := f.rows[regType]
	if !ok || row.NodeID != nodeID || row.Purged {
		f.rows[regType] = db.UERegistration{Imsi: imsi, Type: regType, NodeID: nodeID, RegistrationTime: f.now}
	}

	for _, c := range cancel {
		if row, ok := f.rows[c]; ok {
			row.Purged = true
			f.rows[c] = row
		}
	}

	return nil
}

func (f *fakeStore) PurgeUERegistration(_ context.Context, _, regType, nodeID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.purges++

	if f.err != nil {
		return f.err
	}

	if row, ok := f.rows[regType]; ok && row.NodeID == nodeID {
		row.Purged = true
		f.rows[regType] = row
	}

	return nil
}

func (f *fakeStore) GetUERegistration(_ context.Context, _, regType string) (*db.UERegistration, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	row, ok := f.rows[regType]
	if !ok {
		return nil, db.ErrNotFound
	}

	return &row, nil
}

func (f *fakeStore) set(regType, nodeID string, purged bool, at int64) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.rows[regType] = db.UERegistration{Imsi: imsi, Type: regType, NodeID: nodeID, Purged: purged, RegistrationTime: at}
}

func (f *fakeStore) get(regType string) (db.UERegistration, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()

	row, ok := f.rows[regType]

	return row, ok
}

func newAMFRegistry(store ueregistration.Store) *ueregistration.Registry {
	return ueregistration.New(store, "node-a", db.UERegistrationTypeAMF3GPPAccess, db.UERegistrationTypeMME, zap.NewNop())
}

func TestRegister_CancelsOtherType(t *testing.T) {
	store := newFakeStore()
	store.set(db.UERegistrationTypeMME, "node-b", false, 10)

	if err := newAMFRegistry(store).Register(context.Background(), imsi); err != nil {
		t.Fatalf("Register: %s", err)
	}

	amf, ok := store.get(db.UERegistrationTypeAMF3GPPAccess)
	if !ok || amf.NodeID != "node-a" || amf.Purged {
		t.Fatalf("unexpected AMF registration %+v", amf)
	}

	mme, _ := store.get(db.UERegistrationTypeMME)
	if !mme.Purged {
		t.Fatal("expected the MME registration to be cancelled")
	}
}

func TestRegister_MigrationPendingIsNotAnError(t *testing.T) {
	store := newFakeStore()
	store.err = db.ErrMigrationPending

	if err := newAMFRegistry(store).Register(context.Background(), imsi); err != nil {
		t.Fatalf("expected nil, got %s", err)
	}
}

func TestRegister_ReturnsStoreError(t *testing.T) {
	store := newFakeStore()
	store.err = errors.New("no leader")

	if err := newAMFRegistry(store).Register(context.Background(), imsi); err == nil {
		t.Fatal("expected error")
	}
}

func TestPurge(t *testing.T) {
	cases := []struct {
		name        string
		stillAbsent bool
		wantPurged  bool
	}{
		{"context gone", true, true},
		{"context back", false, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			store := newFakeStore()
			store.set(db.UERegistrationTypeAMF3GPPAccess, "node-a", false, 10)

			newAMFRegistry(store).Purge(context.Background(), imsi, func() bool { return tc.stillAbsent })

			row, _ := store.get(db.UERegistrationTypeAMF3GPPAccess)
			if row.Purged != tc.wantPurged {
				t.Fatalf("purged = %v, want %v", row.Purged, tc.wantPurged)
			}
		})
	}
}

func TestSuperseded(t *testing.T) {
	type row struct {
		nodeID string
		purged bool
		at     int64
	}

	cases := []struct {
		name         string
		amf          *row
		mme          *row
		registeredAt int64
		want         bool
	}{
		{"registered here", &row{"node-a", false, 10}, nil, 10, false},
		{"newer registration on another node", &row{"node-b", false, 20}, nil, 10, true},
		{"older registration on another node", &row{"node-b", false, 5}, nil, 10, false},
		{"cancelled by a newer EPS registration on another node", &row{"node-a", true, 10}, &row{"node-b", false, 20}, 10, true},
		{"cancelled by an older EPS registration on another node", &row{"node-a", true, 10}, &row{"node-b", false, 5}, 10, false},
		{"cancelled by an EPS registration on this node", &row{"node-a", true, 10}, &row{"node-a", false, 20}, 10, false},
		{"cancelled, EPS registration purged", &row{"node-a", true, 10}, &row{"node-b", true, 20}, 10, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			store := newFakeStore()
			if tc.amf != nil {
				store.set(db.UERegistrationTypeAMF3GPPAccess, tc.amf.nodeID, tc.amf.purged, tc.amf.at)
			}

			if tc.mme != nil {
				store.set(db.UERegistrationTypeMME, tc.mme.nodeID, tc.mme.purged, tc.mme.at)
			}

			if got := newAMFRegistry(store).Superseded(context.Background(), imsi, tc.registeredAt); got != tc.want {
				t.Fatalf("Superseded = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestSuperseded_MissingRegistrationIsRestored(t *testing.T) {
	store := newFakeStore()

	if newAMFRegistry(store).Superseded(context.Background(), imsi, 0) {
		t.Fatal("expected the UE to be kept")
	}

	row, ok := store.get(db.UERegistrationTypeAMF3GPPAccess)
	if !ok || row.NodeID != "node-a" {
		t.Fatalf("expected the UE to be registered on node-a, got %+v", row)
	}
}
