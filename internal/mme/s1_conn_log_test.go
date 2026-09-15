// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"sync"
	"sync/atomic"
	"testing"

	"github.com/ellanetworks/core/internal/logger"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

func observeMmeLogAt(t *testing.T, lvl zapcore.Level) *observer.ObservedLogs {
	t.Helper()

	core, logs := observer.New(lvl)
	saved := logger.MmeLog
	logger.MmeLog = zap.New(core)

	t.Cleanup(func() { logger.MmeLog = saved })

	return logs
}

func trackRadioAt(m *MME, w S1APWriter, address string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	r := &Radio{Conn: w, m: m, address: address}
	r.refreshLogLocked()
	m.reg.Track(w, r)
}

func ranAddrOf(t *testing.T, e observer.LoggedEntry) string {
	t.Helper()

	for k, v := range e.ContextMap() {
		if k == "ran_addr" {
			s, _ := v.(string)
			return s
		}
	}

	return ""
}

// TS 36.413 §8.6.1
func TestCommitPathSwitchRebindsConnLoggerToTargetENB(t *testing.T) {
	logs := observeMmeLogAt(t, zapcore.InfoLevel)

	m := New(nil, nil, nil)

	source, target := &captureConn{}, &captureConn{}
	trackRadioAt(m, source, "10.0.0.1:36412")
	trackRadioAt(m, target, "10.0.0.2:36412")

	c := m.NewUeConn(source, 5)

	ue := &UeContext{}
	ue.active.Store(c)
	c.ue = ue

	c.Log().Info("before")

	if _, ok := m.CommitPathSwitch(ue, target, 7, [32]byte{}, 0); !ok {
		t.Fatal("CommitPathSwitch did not commit")
	}

	c.Log().Info("after")

	entries := logs.All()
	if len(entries) != 2 {
		t.Fatalf("expected 2 log entries, got %d", len(entries))
	}

	if got := ranAddrOf(t, entries[0]); got != "10.0.0.1:36412" {
		t.Fatalf("before path switch: ran_addr = %q, want the source eNB", got)
	}

	if got := ranAddrOf(t, entries[1]); got != "10.0.0.2:36412" {
		t.Fatalf("after path switch: ran_addr = %q, want the target eNB", got)
	}
}

func TestUeConnLogConcurrentAccessNoRace(t *testing.T) {
	m := New(nil, nil, nil)

	source, target := &captureConn{}, &captureConn{}
	trackRadioAt(m, source, "10.0.0.1:36412")
	trackRadioAt(m, target, "10.0.0.2:36412")

	c := m.NewUeConn(source, 5)

	ue := &UeContext{}
	ue.active.Store(c)
	c.ue = ue

	var (
		wg      sync.WaitGroup
		commits atomic.Int64
	)

	for _, op := range []func(){
		func() {
			if _, ok := m.CommitPathSwitch(ue, target, 7, [32]byte{}, 0); ok {
				commits.Add(1)
			}
		},
		func() { _ = c.Log() },
	} {
		wg.Add(1)

		go func(f func()) {
			defer wg.Done()

			for range 500 {
				f()
			}
		}(op)
	}

	wg.Wait()

	if commits.Load() == 0 {
		t.Fatal("CommitPathSwitch never committed; the logger write was not exercised")
	}
}

func countField(e observer.LoggedEntry, key string) int {
	n := 0

	for _, f := range e.Context {
		if f.Key == key {
			n++
		}
	}

	return n
}

func TestReleaseUEContextLocallyNamesTheSubscriberExactlyOnce(t *testing.T) {
	logs := observeMmeLogAt(t, zapcore.InfoLevel)

	m := New(nil, nil, nil)

	conn := &captureConn{}
	trackRadioAt(m, conn, "10.0.0.1:36412")

	c := m.NewUeConn(conn, 5)

	ue := &UeContext{}
	ue.active.Store(c)
	c.ue = ue

	m.SetIMSI(ue, "001010000000001")

	ctx := logger.Into(t.Context(), c.Log())

	m.ReleaseUEContextLocally(ctx, ue, "test")

	entries := logs.All()
	if len(entries) != 1 {
		t.Fatalf("expected 1 log entry, got %d", len(entries))
	}

	for _, key := range []string{"supi", "mme_ue_s1ap_id", "enb_ue_s1ap_id"} {
		if got := countField(entries[0], key); got > 1 {
			t.Errorf("%q appears %d times in one record", key, got)
		}
	}

	if countField(entries[0], "supi") != 1 {
		t.Error("the release record does not name the subscriber")
	}
}
