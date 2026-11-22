// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package logger

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/internal/dbwriter"
)

type recordingDBWriter struct {
	radioEvents []*dbwriter.RadioEvent
}

func (w *recordingDBWriter) InsertRadioEvent(_ context.Context, e *dbwriter.RadioEvent) error {
	w.radioEvents = append(w.radioEvents, e)
	return nil
}

func (w *recordingDBWriter) InsertFlowReports(context.Context, []*dbwriter.FlowReport) error {
	return nil
}

func (w *recordingDBWriter) InsertAuditLog(context.Context, *dbwriter.AuditLog) error {
	return nil
}

func TestLogNetworkEventHonoursRadioEventSetting(t *testing.T) {
	previous := dbInstance

	t.Cleanup(func() {
		dbInstance = previous

		SetRadioEventsEnabled(true)
	})

	w := &recordingDBWriter{}
	dbInstance = w

	logEvent := func() {
		LogNetworkEvent(context.Background(), NGAPNetworkProtocol, "NGSetupRequest", DirectionInbound, "10.0.0.1:38412", "10.0.0.2:38412", "gnb", "ref", "1", []byte{0x00})
	}

	logEvent()

	if len(w.radioEvents) != 1 {
		t.Fatalf("stored %d radio events with the default setting, want 1", len(w.radioEvents))
	}

	SetRadioEventsEnabled(false)
	logEvent()

	if len(w.radioEvents) != 1 {
		t.Fatalf("stored %d radio events after disabling, want 1", len(w.radioEvents))
	}

	SetRadioEventsEnabled(true)
	logEvent()

	if len(w.radioEvents) != 2 {
		t.Fatalf("stored %d radio events after re-enabling, want 2", len(w.radioEvents))
	}
}
