// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package upf

import (
	"context"
	"slices"
	"testing"
	"time"
)

func TestNextCleanupDelay(t *testing.T) {
	for _, tc := range []struct {
		name            string
		delay           time.Duration
		cleaned, remain int
		want            time.Duration
	}{
		{"nothing pending", 4 * time.Minute, 0, 0, cleanupRetryInitial},
		{"progress resets", 4 * time.Minute, 1, 3, cleanupRetryInitial},
		{"no progress backs off", cleanupRetryInitial, 0, 2, 2 * cleanupRetryInitial},
		{"capped", 4 * time.Minute, 0, 1, cleanupRetryMax},
		{"stays capped", cleanupRetryMax, 0, 1, cleanupRetryMax},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := nextCleanupDelay(tc.delay, tc.cleaned, tc.remain); got != tc.want {
				t.Fatalf("nextCleanupDelay(%v, %d, %d) = %v, want %v", tc.delay, tc.cleaned, tc.remain, got, tc.want)
			}
		})
	}
}

type fakeReaper struct {
	cleaned []uint64
	pending int
}

func (f fakeReaper) ReapDeletedSessions(context.Context) ([]uint64, int) {
	return f.cleaned, f.pending
}

type recordingCleanupReport struct {
	cleaned []uint64
	pending []int
}

func (r *recordingCleanupReport) UserPlaneSessionCleaned(_ context.Context, seid uint64) {
	r.cleaned = append(r.cleaned, seid)
}

func (r *recordingCleanupReport) pendingCleanup(pending int) {
	r.pending = append(r.pending, pending)
}

func TestReapOnceReportsEachCleanedSession(t *testing.T) {
	report := &recordingCleanupReport{}

	cleaned, pending := reapOnce(t.Context(), fakeReaper{cleaned: []uint64{7, 9}, pending: 2}, report)
	if cleaned != 2 || pending != 2 {
		t.Fatalf("reapOnce = (%d, %d), want (2, 2)", cleaned, pending)
	}

	if !slices.Equal(report.cleaned, []uint64{7, 9}) || !slices.Equal(report.pending, []int{2}) {
		t.Fatalf("reported cleaned=%v pending=%v, want [7 9] and [2]", report.cleaned, report.pending)
	}
}

func TestRunCleanupReaperStopsOnCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan struct{})

	go func() {
		defer close(done)

		runCleanupReaper(ctx, fakeReaper{}, &recordingCleanupReport{})
	}()

	cancel()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("the cleanup reaper did not stop when its context was cancelled")
	}
}
