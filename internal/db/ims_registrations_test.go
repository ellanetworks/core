// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package db_test

import (
	"context"
	"errors"
	"testing"

	"github.com/ellanetworks/core/internal/db"
)

func createIMSSubscriber(t *testing.T, dbInstance *db.Database, imsi string) {
	t.Helper()

	profileID, _ := createPolicyDeps(t, dbInstance, t.Name())

	if err := dbInstance.CreateSubscriber(context.Background(), &db.Subscriber{
		Imsi:           imsi,
		PermanentKey:   "465b5ce8b199b49faa5f0a2ee238a6bc",
		Opc:            "cd63cb71954a9f4e48a5994e37a02baf",
		SequenceNumber: "000000000000",
		ProfileID:      profileID,
	}); err != nil {
		t.Fatalf("create subscriber: %v", err)
	}
}

func pendingRegistration(server string) *db.IMSRegistration {
	return &db.IMSRegistration{
		State:       db.IMSNotRegistered,
		ServerName:  server,
		AuthPending: true,
		OriginHost:  "scscf.ims.mnc001.mcc001.3gppnetwork.org",
		OriginRealm: "ims.mnc001.mcc001.3gppnetwork.org",
		UpdatedAt:   1000,
	}
}

func TestIMSRegistrationCompareAndSwap(t *testing.T) {
	dbInstance := setupTestDB(t)
	ctx := context.Background()

	const imsi = "001010000000051"

	createIMSSubscriber(t, dbInstance, imsi)

	got, err := dbInstance.GetIMSRegistration(ctx, imsi)
	if err != nil || got != nil {
		t.Fatalf("initial registration = %+v, %v; want none", got, err)
	}

	pending := pendingRegistration("sip:scscf-a.example.org")

	current, swapped, err := dbInstance.CompareAndSwapIMSRegistration(ctx, imsi, nil, pending)
	if err != nil || !swapped {
		t.Fatalf("insert: swapped=%v err=%v", swapped, err)
	}

	if current.IMSI != imsi || current.ServerName != pending.ServerName {
		t.Fatalf("insert returned %+v", current)
	}

	other := pendingRegistration("sip:scscf-b.example.org")

	current, swapped, err = dbInstance.CompareAndSwapIMSRegistration(ctx, imsi, nil, other)
	if err != nil || swapped {
		t.Fatalf("stale insert: swapped=%v err=%v", swapped, err)
	}

	if current == nil || current.ServerName != pending.ServerName {
		t.Fatalf("stale insert returned %+v, want the stored registration", current)
	}

	registered := *pending
	registered.State = db.IMSRegistered
	registered.AuthPending = false
	registered.UpdatedAt = 2000

	if _, swapped, err = dbInstance.CompareAndSwapIMSRegistration(ctx, imsi, pending, &registered); err != nil || !swapped {
		t.Fatalf("register: swapped=%v err=%v", swapped, err)
	}

	got, err = dbInstance.GetIMSRegistration(ctx, imsi)
	if err != nil {
		t.Fatalf("get: %v", err)
	}

	if got.State != db.IMSRegistered || got.AuthPending || got.ServerName != pending.ServerName || got.OriginHost != pending.OriginHost || got.UpdatedAt != 2000 {
		t.Fatalf("stored registration = %+v", got)
	}

	if _, swapped, err = dbInstance.CompareAndSwapIMSRegistration(ctx, imsi, &registered, &registered); err != nil || !swapped {
		t.Fatalf("unchanged: swapped=%v err=%v", swapped, err)
	}

	if _, swapped, err = dbInstance.CompareAndSwapIMSRegistration(ctx, imsi, &registered, nil); err != nil || !swapped {
		t.Fatalf("delete: swapped=%v err=%v", swapped, err)
	}

	got, err = dbInstance.GetIMSRegistration(ctx, imsi)
	if err != nil || got != nil {
		t.Fatalf("after delete = %+v, %v; want none", got, err)
	}
}

func TestIMSRegistrationForUnknownSubscriberIsNotFound(t *testing.T) {
	dbInstance := setupTestDB(t)

	_, _, err := dbInstance.CompareAndSwapIMSRegistration(context.Background(), "001010000000999", nil, pendingRegistration("sip:scscf.example.org"))
	if !errors.Is(err, db.ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestIMSRegistrationIsDeletedWithTheSubscriber(t *testing.T) {
	dbInstance := setupTestDB(t)
	ctx := context.Background()

	const imsi = "001010000000052"

	createIMSSubscriber(t, dbInstance, imsi)

	if _, _, err := dbInstance.CompareAndSwapIMSRegistration(ctx, imsi, nil, pendingRegistration("sip:scscf.example.org")); err != nil {
		t.Fatalf("insert: %v", err)
	}

	if err := dbInstance.DeleteSubscriber(ctx, imsi); err != nil {
		t.Fatalf("delete subscriber: %v", err)
	}

	got, err := dbInstance.GetIMSRegistration(ctx, imsi)
	if err != nil || got != nil {
		t.Fatalf("after subscriber delete = %+v, %v; want none", got, err)
	}
}

func TestCountIMSRegistrations(t *testing.T) {
	dbInstance := setupTestDB(t)
	ctx := context.Background()

	profileID, _ := createPolicyDeps(t, dbInstance, t.Name())

	for i, state := range []db.IMSRegistrationState{db.IMSRegistered, db.IMSRegistered, db.IMSUnregistered, db.IMSNotRegistered} {
		imsi := "00101000000006" + string(rune('0'+i))

		if err := dbInstance.CreateSubscriber(ctx, &db.Subscriber{
			Imsi:           imsi,
			PermanentKey:   "465b5ce8b199b49faa5f0a2ee238a6bc",
			Opc:            "cd63cb71954a9f4e48a5994e37a02baf",
			SequenceNumber: "000000000000",
			ProfileID:      profileID,
		}); err != nil {
			t.Fatalf("create subscriber %s: %v", imsi, err)
		}

		reg := pendingRegistration("sip:scscf.example.org")
		reg.State = state

		if _, swapped, err := dbInstance.CompareAndSwapIMSRegistration(ctx, imsi, nil, reg); err != nil || !swapped {
			t.Fatalf("insert %s: swapped=%v err=%v", imsi, swapped, err)
		}
	}

	for state, want := range map[db.IMSRegistrationState]int{db.IMSRegistered: 2, db.IMSUnregistered: 1, db.IMSNotRegistered: 1} {
		got, err := dbInstance.CountIMSRegistrations(ctx, state)
		if err != nil || got != want {
			t.Fatalf("CountIMSRegistrations(%d) = %d, %v; want %d", state, got, err, want)
		}
	}
}
