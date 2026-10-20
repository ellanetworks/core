// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package smf_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/ellanetworks/core/internal/models"
)

func today() int64 {
	return time.Now().UTC().Unix() / 86400
}

func TestCollectUsageTakesEverySessionIntoOneBatch(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb)
	ctx := context.Background()

	sc, _ := setupSessionWithTunnel(t, s)
	upf.count(sc.PFCPContext.SEID, 500, 300)

	s.CollectUsageForTest(ctx)

	store.mu.Lock()
	defer store.mu.Unlock()

	if store.batchCalls != 1 {
		t.Fatalf("got %d store calls, want 1", store.batchCalls)
	}

	want := usageEntry{epochDay: today(), imsi: sc.Supi.IMSI(), uplinkBytes: 500, downlinkBytes: 300}
	if len(store.usageLog) != 1 || store.usageLog[0] != want {
		t.Fatalf("usage = %+v, want [%+v]", store.usageLog, want)
	}
}

func TestCollectUsageSkipsABusySessionWithoutLosingItsBytes(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb)
	ctx := context.Background()

	sc, _ := setupSessionWithTunnel(t, s)
	upf.count(sc.PFCPContext.SEID, 40, 60)

	sc.Mutex.Lock()
	s.CollectUsageForTest(ctx)
	sc.Mutex.Unlock()

	upf.mu.Lock()
	queried := len(upf.queryCalls)
	upf.mu.Unlock()

	if queried != 0 {
		t.Fatalf("a busy session was queried %d times", queried)
	}

	s.CollectUsageForTest(ctx)

	store.mu.Lock()
	defer store.mu.Unlock()

	if len(store.usageLog) != 1 || store.usageLog[0].uplinkBytes != 40 || store.usageLog[0].downlinkBytes != 60 {
		t.Fatalf("usage = %+v, want the 40/60 left in the counters", store.usageLog)
	}
}

func TestCollectUsageSkipsASessionTheUPFHasNotAccepted(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb)

	sc, _ := setupSessionWithTunnel(t, s)
	sc.PFCPContext.Established = false

	s.CollectUsageForTest(context.Background())

	upf.mu.Lock()
	defer upf.mu.Unlock()

	if len(upf.queryCalls) != 0 {
		t.Fatalf("queried SEIDs %v before the UPF accepted the session", upf.queryCalls)
	}
}

func TestCollectUsageKeepsAPartialReading(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb)

	sc, _ := setupSessionWithTunnel(t, s)
	upf.count(sc.PFCPContext.SEID, 7, 0)
	upf.queryErr = errors.New("read URR 2: boom")

	s.CollectUsageForTest(context.Background())

	store.mu.Lock()
	defer store.mu.Unlock()

	if len(store.usageLog) != 1 || store.usageLog[0].uplinkBytes != 7 {
		t.Fatalf("usage = %+v, want the 7 bytes read before the error", store.usageLog)
	}
}

func TestReleaseRecordsTheFinalUsageOnce(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb)
	ctx := context.Background()

	sc, ref := setupSessionWithTunnel(t, s)
	seid := sc.PFCPContext.SEID
	imsi := sc.Supi.IMSI()

	upf.count(seid, 100, 10)
	s.CollectUsageForTest(ctx)

	upf.count(seid, 5, 1)

	if err := s.ReleaseSmContext(ctx, ref); err != nil {
		t.Fatalf("ReleaseSmContext: %v", err)
	}

	s.CollectUsageForTest(ctx)

	store.mu.Lock()
	defer store.mu.Unlock()

	var uplink, downlink uint64

	for _, u := range store.usageLog {
		if u.imsi != imsi {
			t.Fatalf("usage charged to %q, want %q", u.imsi, imsi)
		}

		uplink += u.uplinkBytes
		downlink += u.downlinkBytes
	}

	if uplink != 105 || downlink != 11 {
		t.Fatalf("recorded %d/%d, want 105/11", uplink, downlink)
	}
}

func TestFlushRetriesARetryableStoreError(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb)
	ctx := context.Background()

	sc, _ := setupSessionWithTunnel(t, s)

	store.mu.Lock()
	store.usageErr = errors.New("database is locked")
	store.mu.Unlock()

	upf.count(sc.PFCPContext.SEID, 10, 20)
	s.CollectUsageForTest(ctx)

	store.mu.Lock()
	store.usageErr = nil
	store.mu.Unlock()

	upf.count(sc.PFCPContext.SEID, 1, 2)
	s.CollectUsageForTest(ctx)

	store.mu.Lock()
	defer store.mu.Unlock()

	want := usageEntry{epochDay: today(), imsi: sc.Supi.IMSI(), uplinkBytes: 11, downlinkBytes: 22}
	if len(store.usageLog) != 1 || store.usageLog[0] != want {
		t.Fatalf("usage = %+v, want the failed flush merged into [%+v]", store.usageLog, want)
	}
}

func TestFlushDropsUsageWhoseOutcomeIsUnknown(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb)
	ctx := context.Background()

	sc, _ := setupSessionWithTunnel(t, s)

	store.mu.Lock()
	store.usageErr = fmt.Errorf("propose: %w", models.ErrUsageOutcomeUnknown)
	store.mu.Unlock()

	upf.count(sc.PFCPContext.SEID, 10, 20)
	s.CollectUsageForTest(ctx)

	store.mu.Lock()
	store.usageErr = nil
	store.mu.Unlock()

	s.FlushUsageForTest(ctx)

	store.mu.Lock()
	defer store.mu.Unlock()

	if len(store.usageLog) != 0 {
		t.Fatalf("usage = %+v, want nothing: a write of unknown outcome may have committed", store.usageLog)
	}
}

func TestUsageIsRecordedOnTheDayItWasTaken(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb)
	ctx := context.Background()

	sc, _ := setupSessionWithTunnel(t, s)
	beforeMidnight := time.Date(2026, 10, 19, 23, 59, 59, 0, time.UTC)

	s.ApplyUsageForTest(ctx, sc, models.UsageReport{UplinkVolume: 3}, beforeMidnight)
	s.ApplyUsageForTest(ctx, sc, models.UsageReport{UplinkVolume: 4}, beforeMidnight.Add(2*time.Second))
	s.FlushUsageForTest(ctx)

	store.mu.Lock()
	defer store.mu.Unlock()

	day := beforeMidnight.Unix() / 86400
	want := []usageEntry{
		{epochDay: day, imsi: sc.Supi.IMSI(), uplinkBytes: 3},
		{epochDay: day + 1, imsi: sc.Supi.IMSI(), uplinkBytes: 4},
	}

	if len(store.usageLog) != 2 || store.usageLog[0] != want[0] || store.usageLog[1] != want[1] {
		t.Fatalf("usage = %+v, want %+v", store.usageLog, want)
	}
}

func TestStopUsageCollectionTakesAndFlushesTheRemainder(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb)
	ctx := context.Background()

	sc, _ := setupSessionWithTunnel(t, s)

	s.StartUsageCollection()

	upf.count(sc.PFCPContext.SEID, 8, 9)
	s.StopUsageCollection(ctx)

	s.ApplyUsageForTest(ctx, sc, models.UsageReport{UplinkVolume: 1}, time.Now())
	s.FlushUsageForTest(ctx)

	store.mu.Lock()
	defer store.mu.Unlock()

	if len(store.usageLog) != 1 || store.usageLog[0].uplinkBytes != 8 || store.usageLog[0].downlinkBytes != 9 {
		t.Fatalf("usage = %+v, want only the 8/9 taken at shutdown", store.usageLog)
	}
}

func TestStopUsageCollectionWaitsForABusySession(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb)

	sc, _ := setupSessionWithTunnel(t, s)
	upf.count(sc.PFCPContext.SEID, 8, 9)

	sc.Mutex.Lock()

	go func() {
		time.Sleep(50 * time.Millisecond)
		sc.Mutex.Unlock()
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	s.StopUsageCollection(ctx)

	store.mu.Lock()
	defer store.mu.Unlock()

	if len(store.usageLog) != 1 || store.usageLog[0].uplinkBytes != 8 {
		t.Fatalf("usage = %+v, want the busy session's 8/9 once it was released", store.usageLog)
	}
}

func TestReleaseFlushesTheFinalUsageWithoutWaitingForTheNextCollection(t *testing.T) {
	pcf, store, upf, amfCb := defaultFakes()
	s := newTestSMF(pcf, store, upf, amfCb)
	ctx := context.Background()

	sc, ref := setupSessionWithTunnel(t, s)
	upf.count(sc.PFCPContext.SEID, 3, 4)

	s.StartUsageCollection()
	t.Cleanup(func() { s.StopUsageCollection(ctx) })

	if err := s.ReleaseSmContext(ctx, ref); err != nil {
		t.Fatalf("ReleaseSmContext: %v", err)
	}

	deadline := time.Now().Add(5 * time.Second)

	for {
		store.mu.Lock()
		recorded := len(store.usageLog)
		store.mu.Unlock()

		if recorded == 1 {
			return
		}

		if time.Now().After(deadline) {
			t.Fatal("the released session's usage was not flushed")
		}

		time.Sleep(10 * time.Millisecond)
	}
}
