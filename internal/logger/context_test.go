// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package logger_test

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/internal/logger"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

func TestFromReturnsBaseWhenContextCarriesNothing(t *testing.T) {
	base := zap.NewNop()

	got := logger.From(context.Background(), base)
	if got != base {
		t.Errorf("expected base logger, got a different logger")
	}
}

func TestFromAddsContextFieldsToBase(t *testing.T) {
	core, logs := observer.New(zapcore.DebugLevel)

	ctx := logger.Into(context.Background(), logger.SUPI("imsi-001010000000001"))

	logger.From(ctx, zap.New(core).Named("MME")).Info("attach accepted")

	entries := logs.All()
	if len(entries) != 1 {
		t.Fatalf("expected 1 log entry, got %d", len(entries))
	}

	if entries[0].LoggerName != "MME" {
		t.Errorf("expected the named base logger to survive, got component %q", entries[0].LoggerName)
	}

	if got := entries[0].ContextMap()["supi"]; got != "imsi-001010000000001" {
		t.Errorf("expected the context subscriber on the record, got %v", got)
	}
}

func TestANestedIntoRefinesAnOuterOne(t *testing.T) {
	core, logs := observer.New(zapcore.DebugLevel)

	t.Cleanup(logger.SwapSystemCore(core))

	ctx := logger.Into(context.Background(), logger.SUPI("imsi-1"), logger.MMEUeS1apID(7))
	ctx = logger.Into(ctx, logger.SUPI("imsi-2"))

	logger.From(ctx, logger.MmeLog).Info("attach accepted")

	entries := logs.All()
	if len(entries) != 1 {
		t.Fatalf("expected 1 log entry, got %d", len(entries))
	}

	got := entries[0].ContextMap()
	if got["supi"] != "imsi-2" {
		t.Errorf("expected the nested subscriber to win, got %v", got["supi"])
	}

	if got["mme_ue_s1ap_id"] != uint32(7) {
		t.Errorf("expected the outer connection identity to survive, got %v", got["mme_ue_s1ap_id"])
	}
}

func TestIntoDropsSkippedFields(t *testing.T) {
	ctx := logger.Into(context.Background(), logger.SUPIFromIMSI("not-an-imsi"))

	if got := logger.Fields(ctx); len(got) != 0 {
		t.Errorf("expected an unparseable identity to add nothing, got %v", got)
	}
}
