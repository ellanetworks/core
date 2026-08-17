// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package gnb

import (
	"sync"
	"testing"
	"time"

	"github.com/ellanetworks/core/ngap"
)

func newDispatchTestGnodeB() *GnodeB {
	g := &GnodeB{receivedFrames: make(map[Category]map[ngap.ProcedureCode][]SCTPFrame)}
	g.cond = sync.NewCond(&g.mu)

	return g
}

func pagingFrame() SCTPFrame {
	return SCTPFrame{Category: Initiating, ProcedureCode: ngap.ProcPaging}
}

func TestDispatcherCloseAllIsIdempotent(t *testing.T) {
	g := newDispatchTestGnodeB()
	d := newDispatcher(g)

	q := make(chan queuedFrame, perUEQueueDepth)

	d.mu.Lock()
	d.queues[1] = q
	d.wg.Add(1)
	d.mu.Unlock()

	go d.worker(q)

	d.closeAll()
	d.closeAll()
}

func TestDispatcherDispatchAfterCloseReleasesTheCaller(t *testing.T) {
	g := newDispatchTestGnodeB()
	d := newDispatcher(g)

	d.closeAll()

	done := make(chan struct{})

	d.dispatch(1, pagingFrame(), func() { close(done) })

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("dispatch after close did not release the caller")
	}
}

func TestDispatcherCloseAllRacingDispatchDoesNotPanic(t *testing.T) {
	g := newDispatchTestGnodeB()
	d := newDispatcher(g)

	var wg sync.WaitGroup

	for i := 0; i < 8; i++ {
		wg.Add(1)

		go func() {
			defer wg.Done()

			for j := 0; j < 200; j++ {
				ranUEID := int64(j % 4)

				d.dispatch(ranUEID, pagingFrame(), func() {})
			}
		}()
	}

	go func() {
		time.Sleep(time.Millisecond)
		d.closeAll()
	}()

	done := make(chan struct{})

	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("racing dispatch + closeAll did not finish within 5s")
	}
}

func TestDispatcherPreservesPerUEOrder(t *testing.T) {
	g := newDispatchTestGnodeB()
	d := newDispatcher(g)

	ranUEID := int64(42)
	n := 32

	var wg sync.WaitGroup

	done := make(chan struct{})

	for i := 0; i < n; i++ {
		wg.Add(1)

		data := make([]byte, 4)
		data[0] = byte(i >> 24)
		data[1] = byte(i >> 16)
		data[2] = byte(i >> 8)
		data[3] = byte(i)

		frame := pagingFrame()
		frame.Data = data

		d.dispatch(ranUEID, frame, func() {
			wg.Done()
		})
	}

	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("expected all frames to be handled within 5s")
	}

	d.closeAll()

	frames := g.receivedFrames[Initiating][ngap.ProcPaging]

	if len(frames) != n {
		t.Fatalf("expected %d frames, got %d", n, len(frames))
	}

	for i, f := range frames {
		want := make([]byte, 4)
		want[0] = byte(i >> 24)
		want[1] = byte(i >> 16)
		want[2] = byte(i >> 8)
		want[3] = byte(i)

		if string(f.Data) != string(want) {
			t.Errorf("frame[%d]: Data = %v, want %v", i, f.Data, want)
		}
	}
}

// TestDispatcherNoteBacklogWarnsOncePerUE pins the reporting policy for a handler
// falling behind: the first frame that finds its UE's queue at or past half depth
// is worth a warning, later ones are not, and the budget is per RAN UE NGAP ID.
// Warning on every frame instead would bury the signal in a scenario log.
func TestDispatcherNoteBacklogWarnsOncePerUE(t *testing.T) {
	d := newDispatcher(newDispatchTestGnodeB())

	d.mu.Lock()
	defer d.mu.Unlock()

	for backlog := 0; backlog < perUEQueueDepth/2; backlog++ {
		if d.noteBacklogLocked(1, backlog) {
			t.Fatalf("warned about a backlog of %d frames, want no warning below half of %d",
				backlog, perUEQueueDepth)
		}
	}

	if !d.noteBacklogLocked(1, perUEQueueDepth/2) {
		t.Fatalf("no warning at a backlog of %d frames, want one at half of %d",
			perUEQueueDepth/2, perUEQueueDepth)
	}

	for backlog := perUEQueueDepth / 2; backlog < perUEQueueDepth; backlog++ {
		if d.noteBacklogLocked(1, backlog) {
			t.Errorf("warned twice about the same UE, at backlogs of %d and %d frames",
				perUEQueueDepth/2, backlog)
		}
	}

	if !d.noteBacklogLocked(2, perUEQueueDepth/2) {
		t.Error("a second UE's backlog went unwarned; the budget is per RAN UE NGAP ID, not global")
	}
}

// TestDispatcherDispatchReportsABacklog checks that dispatch feeds the queue's own
// depth to the policy above, so the warning fires on a queue nothing drains.
func TestDispatcherDispatchReportsABacklog(t *testing.T) {
	const ranUEID = int64(7)

	d := newDispatcher(newDispatchTestGnodeB())

	t.Cleanup(d.closeAll)

	// Register the queue without a worker, so nothing drains it and the backlog
	// grows by exactly one frame per dispatch.
	d.mu.Lock()
	d.queues[ranUEID] = make(chan queuedFrame, perUEQueueDepth)
	d.mu.Unlock()

	for i := 0; i < perUEQueueDepth; i++ {
		d.dispatch(ranUEID, pagingFrame(), func() {})
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	if !d.warned[ranUEID] {
		t.Fatalf("%d frames queued for one UE with nothing draining them went unreported",
			perUEQueueDepth)
	}
}
