// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package jobs

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/dbwriter"
)

func newRetentionTestDB(t *testing.T) *db.Database {
	t.Helper()

	ctx := context.Background()

	database, err := db.NewDatabaseWithoutRaft(ctx, filepath.Join(t.TempDir(), "ella.db"))
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() { _ = database.Close() })

	return database
}

func setOneDayPolicies(t *testing.T, database *db.Database) {
	t.Helper()

	ctx := context.Background()

	categories := []db.RetentionCategory{
		db.CategoryAuditLogs,
		db.CategoryRadioLogs,
		db.CategorySubscriberUsage,
		db.CategoryFlowReports,
	}

	for _, category := range categories {
		if err := database.SetRetentionPolicy(ctx, &db.RetentionPolicy{Category: category, Days: 1}); err != nil {
			t.Fatalf("set %s retention policy: %v", category, err)
		}
	}
}

func createSubscriber(t *testing.T, database *db.Database, imsi string) {
	t.Helper()

	ctx := context.Background()

	profile, err := database.GetProfile(ctx, db.InitialProfileName)
	if err != nil {
		t.Fatalf("get default profile: %v", err)
	}

	if err := database.CreateSubscriber(ctx, &db.Subscriber{
		Imsi:           imsi,
		SequenceNumber: "000000000022",
		PermanentKey:   "6f30087629feb0b089783c81d0ae09b5",
		Opc:            "21a7e1897dfb481d62439142cdf1b6ee",
		ProfileID:      profile.ID,
	}); err != nil {
		t.Fatalf("create subscriber: %v", err)
	}
}

func seedRetentionRows(t *testing.T, database *db.Database) {
	t.Helper()

	ctx := context.Background()

	const imsi = "001010000000001"

	createSubscriber(t, database, imsi)

	old := time.Now().UTC().AddDate(0, 0, -3).Format(time.RFC3339)
	recent := time.Now().UTC().Format(time.RFC3339)

	for _, ts := range []string{old, recent} {
		if err := database.InsertRadioEvent(ctx, &dbwriter.RadioEvent{
			Timestamp:   ts,
			Protocol:    "ngap",
			MessageType: "InitialUEMessage",
			Direction:   "inbound",
			RadioName:   "gnb-1",
			Raw:         []byte{0x00},
		}); err != nil {
			t.Fatalf("insert radio event: %v", err)
		}

		if err := database.InsertFlowReports(ctx, []*dbwriter.FlowReport{{
			SubscriberID:  imsi,
			SourceIP:      "10.45.0.2",
			DestinationIP: "1.1.1.1",
			Protocol:      6,
			StartTime:     ts,
			EndTime:       ts,
			Direction:     "uplink",
		}}); err != nil {
			t.Fatalf("insert flow report: %v", err)
		}

		if err := database.InsertAuditLog(ctx, &dbwriter.AuditLog{
			ID:        "audit-" + ts,
			Timestamp: ts,
			Level:     "info",
			Actor:     "tester",
			Action:    "test",
			IP:        "127.0.0.1",
		}); err != nil {
			t.Fatalf("insert audit log: %v", err)
		}

		day, err := time.Parse(time.RFC3339, ts)
		if err != nil {
			t.Fatalf("parse timestamp: %v", err)
		}

		if err := database.IncrementDailyUsage(ctx, db.DailyUsage{
			EpochDay:      day.Unix() / 86400,
			IMSI:          imsi,
			BytesUplink:   1,
			BytesDownlink: 1,
		}); err != nil {
			t.Fatalf("increment daily usage: %v", err)
		}
	}
}

func countUsageDays(t *testing.T, database *db.Database, imsi string) int {
	t.Helper()

	rows, err := database.GetUsagePerDay(context.Background(), imsi,
		time.Now().UTC().AddDate(0, 0, -30), time.Now().UTC().AddDate(0, 0, 1))
	if err != nil {
		t.Fatalf("get usage per day: %v", err)
	}

	return len(rows)
}

func countRows(t *testing.T, database *db.Database) (radio, flow, audit int) {
	t.Helper()

	ctx := context.Background()

	_, radio, err := database.ListRadioEvents(ctx, 1, 100, nil)
	if err != nil {
		t.Fatalf("list radio events: %v", err)
	}

	_, flow, err = database.ListFlowReports(ctx, 1, 100, nil)
	if err != nil {
		t.Fatalf("list flow reports: %v", err)
	}

	_, audit, err = database.ListAuditLogsPage(ctx, nil, 1, 100)
	if err != nil {
		t.Fatalf("list audit logs: %v", err)
	}

	return radio, flow, audit
}

func TestRetentionPassPrunesNodeLocalTablesWhenNotLeader(t *testing.T) {
	database := newRetentionTestDB(t)

	setOneDayPolicies(t, database)
	seedRetentionRows(t, database)

	runRetentionPass(context.Background(), database, false)

	radio, flow, audit := countRows(t, database)

	if radio != 1 {
		t.Errorf("radio events: got %d, want 1", radio)
	}

	if flow != 1 {
		t.Errorf("flow reports: got %d, want 1", flow)
	}

	if audit != 1 {
		t.Errorf("audit logs: got %d, want 1", audit)
	}

	if got := countUsageDays(t, database, "001010000000001"); got != 2 {
		t.Errorf("daily usage days: got %d, want 2", got)
	}
}

func TestRetentionPassPrunesReplicatedTablesWhenLeader(t *testing.T) {
	database := newRetentionTestDB(t)

	setOneDayPolicies(t, database)
	seedRetentionRows(t, database)

	runRetentionPass(context.Background(), database, true)

	radio, flow, audit := countRows(t, database)

	if radio != 1 {
		t.Errorf("radio events: got %d, want 1", radio)
	}

	if flow != 1 {
		t.Errorf("flow reports: got %d, want 1", flow)
	}

	if audit != 1 {
		t.Errorf("audit logs: got %d, want 1", audit)
	}

	if got := countUsageDays(t, database, "001010000000001"); got != 1 {
		t.Errorf("daily usage days: got %d, want 1", got)
	}
}

func TestRetentionDaysMissingPolicyIsNotAnError(t *testing.T) {
	database := newRetentionTestDB(t)

	days, ok, err := retentionDays(context.Background(), database, db.RetentionCategory("unseeded"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if ok {
		t.Fatalf("expected no policy, got %d days", days)
	}
}

func TestDataRetentionWorkerPrunesLeaderTablesWithoutRaft(t *testing.T) {
	database := newRetentionTestDB(t)

	setOneDayPolicies(t, database)
	seedRetentionRows(t, database)

	if !database.IsLeader() {
		t.Fatal("a database without raft must report itself leader")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan struct{})

	go func() {
		RunDataRetentionWorker(ctx, database)
		close(done)
	}()

	deadline := time.Now().Add(10 * time.Second)

	for countUsageDays(t, database, "001010000000001") != 1 {
		if time.Now().After(deadline) {
			t.Fatal("worker did not prune daily usage on a non-clustered node")
		}

		time.Sleep(10 * time.Millisecond)
	}

	cancel()
	<-done
}
