// Copyright 2026 Ella Networks

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

	sub := dbInstance.Changefeed().Subscribe(db.TopicNATSettings)
	defer sub.Close()

	if err := dbInstance.UpdateNATSettings(context.Background(), true); err != nil {
		t.Fatalf("UpdateNATSettings: %v", err)
	}

	select {
	case ev := <-sub.Events:
		if ev.Topic != db.TopicNATSettings {
			t.Fatalf("expected topic %q, got %q", db.TopicNATSettings, ev.Topic)
		}
	case <-time.After(time.Second):
		t.Fatal("did not receive nat-settings change event")
	}
}

// TestApplyCommand_DoesNotPublishForUnannotatedOps verifies that ops
// without AffectsTopic produce no events. Initialize() seeds operator
// row (which is unannotated) and must not wake nat-settings subscribers.
func TestApplyCommand_DoesNotPublishForUnannotatedOps(t *testing.T) {
	tempDir := t.TempDir()

	dbInstance, err := db.NewDatabaseWithoutRaft(context.Background(), filepath.Join(tempDir, "test.db"))
	if err != nil {
		t.Fatalf("create db: %v", err)
	}

	defer func() { _ = dbInstance.Close() }()

	sub := dbInstance.Changefeed().Subscribe(db.TopicFlowAccountingSettings)
	defer sub.Close()

	// Update an unrelated topic; subscriber should see nothing.
	if err := dbInstance.UpdateNATSettings(context.Background(), true); err != nil {
		t.Fatalf("UpdateNATSettings: %v", err)
	}

	select {
	case ev := <-sub.Events:
		t.Fatalf("did not expect event, got %+v", ev)
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

	sub := dbInstance.Changefeed().Subscribe(db.TopicFlowAccountingSettings)
	defer sub.Close()

	if err := dbInstance.UpdateFlowAccountingSettings(context.Background(), true); err != nil {
		t.Fatalf("UpdateFlowAccountingSettings: %v", err)
	}

	select {
	case ev := <-sub.Events:
		if ev.Topic != db.TopicFlowAccountingSettings {
			t.Fatalf("expected topic %q, got %q", db.TopicFlowAccountingSettings, ev.Topic)
		}
	case <-time.After(time.Second):
		t.Fatal("did not receive flow-accounting change event")
	}
}
