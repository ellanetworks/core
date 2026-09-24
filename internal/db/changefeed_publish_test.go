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

	wakeup, stop := dbInstance.Changefeed().Wakeup(db.TopicNATSettings)
	defer stop()

	if err := dbInstance.UpdateNATSettings(context.Background(), true); err != nil {
		t.Fatalf("UpdateNATSettings: %v", err)
	}

	select {
	case <-wakeup:
	case <-time.After(time.Second):
		t.Fatal("did not receive nat-settings change event")
	}
}

// TestApplyCommand_DoesNotPublishForUnannotatedOps verifies that an op
// does not wake subscribers of topics it did not declare: a nat-settings
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
	if err := dbInstance.UpdateNATSettings(context.Background(), true); err != nil {
		t.Fatalf("UpdateNATSettings: %v", err)
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
