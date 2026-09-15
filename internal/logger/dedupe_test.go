// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package logger

import (
	"testing"

	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

func TestRecordNamesAKeyOnlyOnce(t *testing.T) {
	core, logs := observer.New(zapcore.DebugLevel)

	t.Cleanup(SwapSystemCore(core))

	ctx := Into(t.Context(), SUPI("imsi-ambient"), MMEUeS1apID(7))

	From(ctx, MmeLog).With(SUPI("imsi-connection")).Info("UE idle", SUPI("imsi-statement"))

	entries := logs.All()
	if len(entries) != 1 {
		t.Fatalf("expected 1 log entry, got %d", len(entries))
	}

	var supis int

	for _, f := range entries[0].Context {
		if f.Key == "supi" {
			supis++
		}
	}

	if supis != 1 {
		t.Fatalf("supi appears %d times in one record, want 1", supis)
	}

	ctxMap := entries[0].ContextMap()
	if ctxMap["supi"] != "imsi-statement" {
		t.Errorf("expected the log statement's subscriber to win, got %v", ctxMap["supi"])
	}

	if ctxMap["mme_ue_s1ap_id"] != uint32(7) {
		t.Errorf("expected the ambient MME-UE-S1AP-ID to survive, got %v", ctxMap["mme_ue_s1ap_id"])
	}
}
