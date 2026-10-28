// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package db_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/ellanetworks/core/internal/db"
	ellaraft "github.com/ellanetworks/core/internal/raft"
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

type sets = map[db.IMSRegistrationSet]db.IMSSetRegistration

func pendingRegistration(server string) *db.IMSRegistration {
	return &db.IMSRegistration{
		ServerName:  server,
		OriginHost:  "scscf.ims.mnc001.mcc001.3gppnetwork.org",
		OriginRealm: "ims.mnc001.mcc001.3gppnetwork.org",
		Sets:        sets{db.IMSRegistrationSetMMTel: {State: db.IMSNotRegistered, AuthPending: true}},
	}
}

func registeredSet(defaultIdentity string) db.IMSSetRegistration {
	return db.IMSSetRegistration{State: db.IMSRegistered, DefaultIdentity: defaultIdentity, ProfileDigest: "digest-" + defaultIdentity}
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
	registered.PrivateIdentity = imsi + "@ims.mnc001.mcc001.3gppnetwork.org"
	registered.Sets = sets{db.IMSRegistrationSetMMTel: registeredSet("sip:+15550001@ims.example.org")}

	if _, swapped, err = dbInstance.CompareAndSwapIMSRegistration(ctx, imsi, pending, &registered); err != nil || !swapped {
		t.Fatalf("register: swapped=%v err=%v", swapped, err)
	}

	got, err = dbInstance.GetIMSRegistration(ctx, imsi)
	if err != nil {
		t.Fatalf("get: %v", err)
	}

	registered.IMSI = imsi
	if !reflect.DeepEqual(got, &registered) {
		t.Fatalf("stored registration = %+v, want %+v", got, registered)
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

func TestIMSRegistrationSetsAreIndependent(t *testing.T) {
	dbInstance := setupTestDB(t)
	ctx := context.Background()

	const imsi = "001010000000055"

	createIMSSubscriber(t, dbInstance, imsi)

	mmtel := &db.IMSRegistration{
		ServerName: "sip:scscf.example.org", OriginHost: "scscf.example.org", OriginRealm: "example.org", PrivateIdentity: imsi + "@ims.example.org",
		Sets: sets{db.IMSRegistrationSetMMTel: registeredSet("sip:+15550001@ims.example.org")},
	}

	if _, swapped, err := dbInstance.CompareAndSwapIMSRegistration(ctx, imsi, nil, mmtel); err != nil || !swapped {
		t.Fatalf("register mmtel: swapped=%v err=%v", swapped, err)
	}

	pendingMCPTT := *mmtel
	pendingMCPTT.Sets = sets{
		db.IMSRegistrationSetMMTel: mmtel.Sets[db.IMSRegistrationSetMMTel],
		db.IMSRegistrationSetMCPTT: {State: db.IMSNotRegistered, AuthPending: true},
	}

	if _, swapped, err := dbInstance.CompareAndSwapIMSRegistration(ctx, imsi, mmtel, &pendingMCPTT); err != nil || !swapped {
		t.Fatalf("authenticate mcptt: swapped=%v err=%v", swapped, err)
	}

	both := pendingMCPTT
	both.Sets = sets{
		db.IMSRegistrationSetMMTel: mmtel.Sets[db.IMSRegistrationSetMMTel],
		db.IMSRegistrationSetMCPTT: registeredSet("sip:mcptt-15550001@ims.example.org"),
	}

	if _, swapped, err := dbInstance.CompareAndSwapIMSRegistration(ctx, imsi, &pendingMCPTT, &both); err != nil || !swapped {
		t.Fatalf("register mcptt: swapped=%v err=%v", swapped, err)
	}

	if _, swapped, err := dbInstance.CompareAndSwapIMSRegistration(ctx, imsi, mmtel, mmtel); err != nil || swapped {
		t.Fatalf("swap expecting mmtel alone: swapped=%v err=%v, want refused", swapped, err)
	}

	got, err := dbInstance.GetIMSRegistration(ctx, imsi)
	if err != nil || got == nil || !reflect.DeepEqual(got.Sets, both.Sets) {
		t.Fatalf("stored registration = %+v, %v; want both sets", got, err)
	}

	mcpttOnly := both
	mcpttOnly.Sets = sets{db.IMSRegistrationSetMCPTT: both.Sets[db.IMSRegistrationSetMCPTT]}

	if _, swapped, err := dbInstance.CompareAndSwapIMSRegistration(ctx, imsi, &both, &mcpttOnly); err != nil || !swapped {
		t.Fatalf("deregister mmtel: swapped=%v err=%v", swapped, err)
	}

	got, err = dbInstance.GetIMSRegistration(ctx, imsi)
	if err != nil || got == nil || !reflect.DeepEqual(got.Sets, mcpttOnly.Sets) || got.ServerName != mmtel.ServerName {
		t.Fatalf("stored registration = %+v, %v; want the mcptt set at the same S-CSCF", got, err)
	}

	if n, err := dbInstance.CountIMSRegistrations(ctx, db.IMSRegistered); err != nil || n != 1 {
		t.Fatalf("CountIMSRegistrations = %d, %v; want 1", n, err)
	}
}

func TestIMSRegistrationCarriesTheIdentitiesTheSCSCFKnows(t *testing.T) {
	dbInstance := setupTestDB(t)
	ctx := context.Background()

	const imsi = "001010000000054"

	createIMSSubscriber(t, dbInstance, imsi)

	for name, reg := range map[string]db.IMSRegistration{
		"without a registration set":             {PrivateIdentity: imsi + "@ims.example.org"},
		"of an unknown registration set":         {PrivateIdentity: imsi + "@ims.example.org", Sets: sets{"rcs": registeredSet("sip:+15550001@ims.example.org")}},
		"registered without a private identity":  {Sets: sets{db.IMSRegistrationSetMCPTT: registeredSet("sip:+15550001@ims.example.org")}},
		"unregistered without a profile digest":  {PrivateIdentity: imsi + "@ims.example.org", Sets: sets{db.IMSRegistrationSetMMTel: {State: db.IMSUnregistered}}},
		"registered with an emergency only flag": {PrivateIdentity: imsi + "@ims.example.org", Sets: sets{db.IMSRegistrationSetMMTel: {State: db.IMSRegistered, ProfileDigest: "digest", Emergency: true}}},
		"not registered nor authenticating":      {PrivateIdentity: imsi + "@ims.example.org", Sets: sets{db.IMSRegistrationSetMMTel: {State: db.IMSNotRegistered}}},
	} {
		reg.ServerName, reg.OriginHost, reg.OriginRealm = "sip:scscf.example.org", "scscf.example.org", "example.org"

		if _, swapped, err := dbInstance.CompareAndSwapIMSRegistration(ctx, imsi, nil, &reg); err == nil || swapped {
			t.Fatalf("%s: swapped=%v err=%v, want refused", name, swapped, err)
		}

		if got, err := dbInstance.GetIMSRegistration(ctx, imsi); err != nil || got != nil {
			t.Fatalf("%s: registration = %+v, %v; want none", name, got, err)
		}
	}

	emergency := &db.IMSRegistration{
		ServerName: "sip:scscf.example.org", OriginHost: "scscf.example.org", OriginRealm: "example.org", PrivateIdentity: imsi + "@ims.example.org",
		Sets: sets{db.IMSRegistrationSetMMTel: {State: db.IMSUnregistered, ProfileDigest: "digest", Emergency: true}},
	}

	if _, swapped, err := dbInstance.CompareAndSwapIMSRegistration(ctx, imsi, nil, emergency); err != nil || !swapped {
		t.Fatalf("insert: swapped=%v err=%v", swapped, err)
	}

	if got, err := dbInstance.GetIMSRegistration(ctx, imsi); err != nil || got == nil || !got.Sets[db.IMSRegistrationSetMMTel].Emergency {
		t.Fatalf("stored registration = %+v, %v; want the emergency registration", got, err)
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

	if n, err := dbInstance.CountIMSRegistrations(ctx, db.IMSNotRegistered); err != nil || n != 0 {
		t.Fatalf("registration sets left after subscriber delete = %d, %v; want none", n, err)
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

		if state != db.IMSNotRegistered {
			reg.PrivateIdentity = imsi + "@ims.example.org"
			reg.Sets = sets{
				db.IMSRegistrationSetMMTel: {State: state, ProfileDigest: "digest"},
				db.IMSRegistrationSetMCPTT: {State: state, ProfileDigest: "digest"},
			}
		}

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

func TestRestoreKeepsIMSRegistrations(t *testing.T) {
	ctx := context.Background()
	tempDir := t.TempDir()

	dbInstance, err := db.NewDatabase(ctx, filepath.Join(tempDir, "data"), ellaraft.FastTestConfig())
	if err != nil {
		t.Fatalf("create database: %v", err)
	}

	t.Cleanup(func() { _ = dbInstance.Close() })

	if err := dbInstance.WaitUntilReady(t.Context()); err != nil {
		t.Fatalf("database never became ready: %v", err)
	}

	const imsi = "001010000000053"

	createIMSSubscriber(t, dbInstance, imsi)

	registered := &db.IMSRegistration{
		ServerName:      "sip:scscf.example.org",
		OriginHost:      "scscf.ims.mnc001.mcc001.3gppnetwork.org",
		OriginRealm:     "ims.mnc001.mcc001.3gppnetwork.org",
		PrivateIdentity: imsi + "@ims.mnc001.mcc001.3gppnetwork.org",
		Sets: sets{
			db.IMSRegistrationSetMMTel: registeredSet("sip:+15550001@ims.example.org"),
			db.IMSRegistrationSetMCPTT: registeredSet("sip:mcptt-15550001@ims.example.org"),
		},
	}

	if _, swapped, err := dbInstance.CompareAndSwapIMSRegistration(ctx, imsi, nil, registered); err != nil || !swapped {
		t.Fatalf("register: swapped=%v err=%v", swapped, err)
	}

	backup, err := os.Create(filepath.Join(tempDir, "backup.tar.gz"))
	if err != nil {
		t.Fatalf("create backup file: %v", err)
	}

	defer func() { _ = backup.Close() }()

	if err := dbInstance.Backup(ctx, backup); err != nil {
		t.Fatalf("backup: %v", err)
	}

	if _, swapped, err := dbInstance.CompareAndSwapIMSRegistration(ctx, imsi, registered, nil); err != nil || !swapped {
		t.Fatalf("deregister: swapped=%v err=%v", swapped, err)
	}

	if _, err := backup.Seek(0, 0); err != nil {
		t.Fatalf("rewind backup: %v", err)
	}

	if err := dbInstance.Restore(ctx, backup); err != nil {
		t.Fatalf("restore: %v", err)
	}

	got, err := dbInstance.GetIMSRegistration(ctx, imsi)
	if err != nil || got == nil {
		t.Fatalf("restored registration = %+v, %v; want the backed-up registration", got, err)
	}

	registered.IMSI = imsi
	if !reflect.DeepEqual(got, registered) {
		t.Fatalf("restored registration = %+v, want %+v", *got, *registered)
	}
}

func TestIMSRegistrationRefusesEmptyPeerIdentity(t *testing.T) {
	dbInstance := setupTestDB(t)
	ctx := context.Background()

	const imsi = "001010000000054"

	createIMSSubscriber(t, dbInstance, imsi)

	for name, blank := range map[string]func(*db.IMSRegistration){
		"serverName":  func(r *db.IMSRegistration) { r.ServerName = "" },
		"originHost":  func(r *db.IMSRegistration) { r.OriginHost = "" },
		"originRealm": func(r *db.IMSRegistration) { r.OriginRealm = "" },
	} {
		t.Run(name, func(t *testing.T) {
			reg := pendingRegistration("sip:scscf.example.org")
			blank(reg)

			if _, swapped, err := dbInstance.CompareAndSwapIMSRegistration(ctx, imsi, nil, reg); err == nil || swapped {
				t.Fatalf("stored a registration with an empty %s: swapped=%v err=%v", name, swapped, err)
			}

			if got, err := dbInstance.GetIMSRegistration(ctx, imsi); err != nil || got != nil {
				t.Fatalf("registration = %+v, %v; want none", got, err)
			}
		})
	}
}
