// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package db_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/ellanetworks/core/internal/db"
)

// TestApplyCommand_PublishesTopicForOp verifies the standalone-mode
// path: a typed op invoked locally fires changefeed events for the
// topics it declared.
func TestApplyCommand_PublishesTopicForOp(t *testing.T) {
	tempDir := t.TempDir()

	dbInstance, err := db.NewDatabaseWithoutRaft(context.Background(), filepath.Join(tempDir, "test.db"))
	if err != nil {
		t.Fatalf("create db: %v", err)
	}

	defer func() { _ = dbInstance.Close() }()

	wakeup, stop := dbInstance.Changefeed().Wakeup(db.TopicN3Settings)
	defer stop()

	if err := dbInstance.UpdateN3Settings(context.Background(), "192.0.2.1"); err != nil {
		t.Fatalf("UpdateN3Settings: %v", err)
	}

	select {
	case <-wakeup:
	case <-time.After(time.Second):
		t.Fatal("did not receive N3 settings change event")
	}
}

// TestApplyCommand_DoesNotPublishForUnannotatedOps verifies that an op
// does not wake subscribers of topics it did not declare: an N3 settings
// update must not wake flow-accounting subscribers.
func TestApplyCommand_DoesNotPublishForUnannotatedOps(t *testing.T) {
	tempDir := t.TempDir()

	dbInstance, err := db.NewDatabaseWithoutRaft(context.Background(), filepath.Join(tempDir, "test.db"))
	if err != nil {
		t.Fatalf("create db: %v", err)
	}

	defer func() { _ = dbInstance.Close() }()

	wakeup, stop := dbInstance.Changefeed().Wakeup(db.TopicFlowAccountingSettings)
	defer stop()

	// Update an unrelated topic; subscriber should see nothing.
	if err := dbInstance.UpdateN3Settings(context.Background(), "192.0.2.1"); err != nil {
		t.Fatalf("UpdateN3Settings: %v", err)
	}

	select {
	case <-wakeup:
		t.Fatal("did not expect a wakeup")
	case <-time.After(100 * time.Millisecond):
	}
}

// TestApplyCommand_PublishesFlowAccountingEvent covers another
// topic-wired local write path.
func TestApplyCommand_PublishesFlowAccountingEvent(t *testing.T) {
	tempDir := t.TempDir()

	dbInstance, err := db.NewDatabaseWithoutRaft(context.Background(), filepath.Join(tempDir, "test.db"))
	if err != nil {
		t.Fatalf("create db: %v", err)
	}

	defer func() { _ = dbInstance.Close() }()

	wakeup, stop := dbInstance.Changefeed().Wakeup(db.TopicFlowAccountingSettings)
	defer stop()

	if err := dbInstance.UpdateFlowAccountingSettings(context.Background(), true); err != nil {
		t.Fatalf("UpdateFlowAccountingSettings: %v", err)
	}

	select {
	case <-wakeup:
	case <-time.After(time.Second):
		t.Fatal("did not receive flow-accounting change event")
	}
}

func TestOperatorIdentityChangesWakeTheDiameterNode(t *testing.T) {
	dbInstance, err := db.NewDatabaseWithoutRaft(context.Background(), filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("create db: %v", err)
	}

	defer func() { _ = dbInstance.Close() }()

	wakeup, stop := dbInstance.Changefeed().Wakeup(db.TopicOperatorIdentity)
	defer stop()

	for name, update := range map[string]func() error{
		"MCC/MNC":      func() error { return dbInstance.UpdateOperatorID(context.Background(), "002", "02") },
		"AMF identity": func() error { return dbInstance.UpdateOperatorAMFIdentity(context.Background(), 3, 4) },
	} {
		if err := update(); err != nil {
			t.Fatalf("%s: %v", name, err)
		}

		select {
		case <-wakeup:
		case <-time.After(time.Second):
			t.Fatalf("a %s change did not publish an operator identity event", name)
		}
	}
}

func TestReplicatedWriteHoldsTheDatapathRevisionUntilApplied(t *testing.T) {
	database := setupRaftTestDB(t)

	tracker := database.Changefeed().Track(db.TopicDataNetworks)
	defer tracker.Stop()

	database.Changefeed().Arm()
	tracker.Done(tracker.Begin())

	before := database.AppliedIndex()
	if got := database.DatapathRevision(); got != before {
		t.Fatalf("DatapathRevision() = %d with the tracker synced, want the applied index %d", got, before)
	}

	if err := database.CreateDataNetwork(t.Context(), &db.DataNetwork{Name: "revision", IPv4Pool: "10.46.0.0/24"}); err != nil {
		t.Fatalf("CreateDataNetwork: %v", err)
	}

	applied := database.AppliedIndex()
	if applied <= before {
		t.Fatalf("applied index %d did not advance past %d", applied, before)
	}

	if got := database.DatapathRevision(); got != before {
		t.Fatalf("DatapathRevision() = %d before the tracker applied the write, want %d", got, before)
	}

	tracker.Done(tracker.Begin())

	if got := database.DatapathRevision(); got != applied {
		t.Fatalf("DatapathRevision() = %d after the tracker applied the write, want %d", got, applied)
	}
}
