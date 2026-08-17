// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package gnb

import (
	"fmt"
	"sync"

	"github.com/ellanetworks/core/internal/sctp"
	"github.com/ellanetworks/core/internal/tester/logger"
	"github.com/ellanetworks/core/ngap"
	"go.uber.org/zap"
)

// perUEQueueDepth is the capacity of each per-UE frame queue: deep enough to
// absorb the burst of a single procedure without making the read loop wait,
// shallow enough that a handler falling behind is reported early. Passing half
// depth is warned about once per UE.
const perUEQueueDepth = 16

// queuedFrame is one frame waiting for its UE's worker to handle it.
type queuedFrame struct {
	frame SCTPFrame
	done  func() // called by the worker after HandleFrame returns
}

// dispatcher gives each RAN UE NGAP ID its own FIFO queue and worker
// goroutine. This satisfies the TS 38.412 §7 / TS 38.413 §6 ordering
// invariant (one stream per UE-associated signalling) without cross-UE
// head-of-line blocking.
//
// Queues are owned by the GnodeB, not by a receiver: after a failover the
// same UEs continue over the new association and must keep their queue (and
// thus their ordering) intact.
//
// The key is the RAN UE NGAP ID, not the UE: a UE that is assigned a new
// RAN UE NGAP ID (N2 handover, path switch, reconnect after a context
// release) gets a new queue, and ordering between the old and the new one
// is not guaranteed. The IDs do not overlap in time in any scenario today.
//
// Queues and workers live until Close: there is no RemoveUE, so a long
// multi-UE run holds one goroutine per RAN UE NGAP ID it ever saw (150 in
// the worst scenario today).
type dispatcher struct {
	mu     sync.Mutex
	queues map[int64]chan queuedFrame // RAN UE NGAP ID -> that UE's FIFO
	warned map[int64]bool             // RAN UE NGAP IDs whose backlog was already reported
	stop   chan struct{}              // closed once, by closeAll
	closed bool
	wg     sync.WaitGroup // worker goroutines
	gnb    *GnodeB        // passed to HandleFrame for each worker
}

func newDispatcher(gnb *GnodeB) *dispatcher {
	return &dispatcher{
		queues: make(map[int64]chan queuedFrame),
		warned: make(map[int64]bool),
		stop:   make(chan struct{}),
		gnb:    gnb,
	}
}

// dispatch sends frame to the worker for ranUEID, creating the queue and
// worker lazily on the first frame. The send blocks when the queue is full,
// which means that UE's handler is wedged. Dropping the frame instead would
// surface as an unexplainable scenario timeout, so the reader waits;
// closeAll releases it.
func (d *dispatcher) dispatch(ranUEID int64, frame SCTPFrame, done func()) {
	d.mu.Lock()

	if d.closed {
		d.mu.Unlock()
		done()

		return
	}

	q, ok := d.queues[ranUEID]
	if !ok {
		q = make(chan queuedFrame, perUEQueueDepth)
		d.queues[ranUEID] = q

		d.wg.Add(1)

		go d.worker(q)
	}

	backlog := len(q)
	report := d.noteBacklogLocked(ranUEID, backlog)

	d.mu.Unlock()

	if report {
		logger.GnbLogger.Warn("gnb: a UE's frame queue is past half depth, its handler is falling behind",
			zap.Int64("RAN UE NGAP ID", ranUEID),
			zap.Int("queued", backlog),
			zap.Int("depth", perUEQueueDepth))
	}

	select {
	case q <- queuedFrame{frame: frame, done: done}:
	case <-d.stop:
		done()
	}
}

// noteBacklogLocked reports whether a queue at backlog frames is worth warning
// about: at or past half depth, and not already reported for that UE. One warning
// per UE keeps a handler that falls behind visible without burying the scenario
// log under one line per frame.
//
// Must be called with d.mu held.
func (d *dispatcher) noteBacklogLocked(ranUEID int64, backlog int) bool {
	if backlog < perUEQueueDepth/2 || d.warned[ranUEID] {
		return false
	}

	d.warned[ranUEID] = true

	return true
}

// worker drains one UE's queue, calling HandleFrame for each frame.
func (d *dispatcher) worker(q chan queuedFrame) {
	defer d.wg.Done()

	for {
		select {
		case item := <-q:
			if err := HandleFrame(d.gnb, item.frame); err != nil {
				logger.GnbLogger.Error("could not handle SCTP frame", zap.Error(err))
			}

			item.done()
		case <-d.stop:
			drainPending(q)

			return
		}
	}
}

// drainPending releases the drain accounting of the frames left in q at
// shutdown. They are not handled: the gNB is going away and its SCTP
// associations are already closed, so any reply would be written to a dead
// socket.
func drainPending(q chan queuedFrame) {
	for {
		select {
		case item := <-q:
			item.done()
		default:
			return
		}
	}
}

// closeAll stops every worker and waits for the frame each is handling. It
// is idempotent, since GnodeB.Close can run more than once.
//
// The per-UE queues are deliberately never closed: dispatch sends outside
// d.mu, so closing them would race a send and panic. A frame that lands
// after its worker has gone is left unhandled and its drain accounting is
// released by the receiver's own bound (see waitTimeout in server.go), not
// here.
func (d *dispatcher) closeAll() {
	d.mu.Lock()

	if d.closed {
		d.mu.Unlock()

		return
	}

	d.closed = true
	close(d.stop)

	queues := make([]chan queuedFrame, 0, len(d.queues))
	for _, q := range d.queues {
		queues = append(queues, q)
	}

	d.mu.Unlock()

	d.wg.Wait()

	for _, q := range queues {
		drainPending(q)
	}
}

// frameRANUEID returns the RAN UE NGAP ID a frame targets, and whether it
// has one.
//
// Only initiating messages are keyed. Everything else is unkeyed and
// runs inline in the read loop. None of the unkeyed messages carries a NAS
// PDU, so none of them can reorder NAS.
//
// A parse failure yields false; the inline handler reports the same error.
func frameRANUEID(f SCTPFrame) (int64, bool) {
	if f.Category != Initiating {
		return 0, false
	}

	switch f.ProcedureCode {
	case ngap.ProcDownlinkNASTransport:
		m, err := ngap.ParseDownlinkNASTransport(f.Value)
		if err != nil {
			return 0, false
		}

		return int64(m.RANUENGAPID), true

	case ngap.ProcInitialContextSetup:
		m, err := ngap.ParseInitialContextSetupRequest(f.Value)
		if err != nil {
			return 0, false
		}

		return int64(m.RANUENGAPID), true

	case ngap.ProcPDUSessionResourceSetup:
		m, err := ngap.ParsePDUSessionResourceSetupRequest(f.Value)
		if err != nil {
			return 0, false
		}

		return int64(m.RANUENGAPID), true

	case ngap.ProcPDUSessionResourceModify:
		m, err := ngap.ParsePDUSessionResourceModifyRequest(f.Value)
		if err != nil {
			return 0, false
		}

		return int64(m.RANUENGAPID), true

	case ngap.ProcPDUSessionResourceRelease:
		m, err := ngap.ParsePDUSessionResourceReleaseCommand(f.Value)
		if err != nil {
			return 0, false
		}

		return int64(m.RANUENGAPID), true

	case ngap.ProcUEContextRelease:
		m, err := ngap.ParseUEContextReleaseCommand(f.Value)
		if err != nil {
			return 0, false
		}

		if m.UENGAPIDs.Pair {
			return int64(m.UENGAPIDs.RANUENGAPID), true
		}

		return 0, false

	case ngap.ProcDownlinkUEAssociatedNRPPaTransport:
		m, err := ngap.ParseDownlinkUEAssociatedNRPPaTransport(f.Value)
		if err != nil {
			return 0, false
		}

		return int64(m.RANUENGAPID), true
	}

	return 0, false
}

// decodeFrame fills in Category, ProcedureCode and Value from the NGAP
// envelope.
func decodeFrame(raw []byte, info *sctp.SndRcvInfo) (SCTPFrame, error) {
	pdu, err := ngap.Unmarshal(raw)
	if err != nil {
		return SCTPFrame{}, fmt.Errorf("decode NGAP: %w", err)
	}

	var cat Category

	var code ngap.ProcedureCode

	var value []byte

	switch m := pdu.(type) {
	case *ngap.InitiatingMessage:
		cat, code, value = Initiating, m.ProcedureCode, m.Value
	case *ngap.SuccessfulOutcome:
		cat, code, value = Successful, m.ProcedureCode, m.Value
	case *ngap.UnsuccessfulOutcome:
		cat, code, value = Unsuccessful, m.ProcedureCode, m.Value
	default:
		return SCTPFrame{}, fmt.Errorf("NGAP PDU alternative is invalid: %T", pdu)
	}

	return SCTPFrame{
		Category:      cat,
		ProcedureCode: code,
		Value:         value,
		Data:          raw,
		Info:          info,
	}, nil
}
