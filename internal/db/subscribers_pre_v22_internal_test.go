// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package db

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/canonical/sqlair"
)

func newDatabaseAtV21(t *testing.T) *Database {
	t.Helper()

	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "db.sqlite3")

	conn, err := openSQLiteConnection(ctx, dbPath)
	if err != nil {
		t.Fatalf("open: %v", err)
	}

	if err := runMigrations(ctx, conn, 21); err != nil {
		t.Fatalf("runMigrations(21): %v", err)
	}

	if err := ensureFsmStateTable(ctx, conn); err != nil {
		t.Fatalf("ensure fsm_state table: %v", err)
	}

	d := new(Database)
	d.connPtr.Store(sqlair.NewDB(conn))
	d.dbPath = dbPath
	d.dataDir = filepath.Dir(dbPath)
	d.changefeed = NewChangefeed()

	t.Cleanup(func() {
		if err := d.Close(); err != nil {
			t.Fatalf("close: %v", err)
		}
	})

	if err := d.refreshAppliedSchema(ctx); err != nil {
		t.Fatalf("refresh applied schema: %v", err)
	}

	if err := d.PrepareStatements(); err != nil {
		t.Fatalf("prepare statements: %v", err)
	}

	RegisterMetrics(d)

	return d
}

func TestSMSReadsWorkBeforeV22(t *testing.T) {
	ctx := context.Background()
	d := newDatabaseAtV21(t)

	if _, err := d.conn().PlainDB().ExecContext(ctx, "PRAGMA foreign_keys = OFF"); err != nil {
		t.Fatalf("disable foreign keys: %v", err)
	}

	_, err := d.conn().PlainDB().ExecContext(ctx, fmt.Sprintf(
		"INSERT INTO %s (id, imsi, sequenceNumber, permanentKey, opc, profileID, description) VALUES ('01890000-0000-7000-8000-000000000001', '001010000000001', '000000000001', '00112233445566778899aabbccddeeff', '00112233445566778899aabbccddeeff', '01890000-0000-7000-8000-0000000000ff', 'gate reader')",
		SubscribersTableName))
	if err != nil {
		t.Fatalf("insert subscriber: %v", err)
	}

	sub, err := d.GetSubscriber(ctx, "001010000000001")
	if err != nil {
		t.Fatalf("GetSubscriber at schema 21: %v", err)
	}

	if sub.Description != "gate reader" || sub.Msisdn != "" {
		t.Fatalf("subscriber at schema 21 = %+v", sub)
	}

	search := "gate"

	subs, total, err := d.ListSubscribersPage(ctx, &SubscriberFilters{Search: &search}, 1, 25)
	if err != nil {
		t.Fatalf("ListSubscribersPage at schema 21: %v", err)
	}

	if total != 1 || len(subs) != 1 || subs[0].Imsi != "001010000000001" {
		t.Fatalf("search at schema 21: total=%d subs=%v", total, subs)
	}

	if _, err := d.GetSubscriberByMSISDN(ctx, "15551230001"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("GetSubscriberByMSISDN at schema 21: want ErrNotFound, got %v", err)
	}

	settings, err := d.GetSMSSettings(ctx)
	if err != nil {
		t.Fatalf("GetSMSSettings at schema 21: %v", err)
	}

	if *settings != DefaultSMSSettings() {
		t.Fatalf("SMS settings at schema 21 = %+v", settings)
	}
}

func TestSMSWritesAreGatedBeforeV22(t *testing.T) {
	ctx := context.Background()
	d := newDatabaseAtV21(t)

	sub := &Subscriber{
		ID:             "01890000-0000-7000-8000-000000000001",
		Imsi:           "001010000000001",
		SequenceNumber: "000000000001",
		PermanentKey:   "00112233445566778899aabbccddeeff",
		Opc:            "00112233445566778899aabbccddeeff",
		ProfileID:      "01890000-0000-7000-8000-0000000000ff",
	}

	if err := d.CreateSubscriber(ctx, sub); !errors.Is(err, ErrMigrationPending) {
		t.Fatalf("CreateSubscriber at schema 21: want ErrMigrationPending, got %v", err)
	}

	if err := d.UpdateSubscriberProfile(ctx, sub); !errors.Is(err, ErrMigrationPending) {
		t.Fatalf("UpdateSubscriberProfile at schema 21: want ErrMigrationPending, got %v", err)
	}

	settings := SMSSettings{SMSNumber: "15550001111"}
	if err := d.UpdateSMSSettings(ctx, &settings); !errors.Is(err, ErrMigrationPending) {
		t.Fatalf("UpdateSMSSettings at schema 21: want ErrMigrationPending, got %v", err)
	}

	peer := SMSCPeer{ID: "01890000-0000-7000-8000-000000000010", DiameterIdentity: "smsc-192-0-2-1.example.org", Address: "192.0.2.1", Port: DefaultSMSCPort, ServiceCentres: []string{"15550000000"}}
	if err := d.CreateSMSCPeer(ctx, &peer); !errors.Is(err, ErrMigrationPending) {
		t.Fatalf("CreateSMSCPeer at schema 21: want ErrMigrationPending, got %v", err)
	}
}
