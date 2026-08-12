// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package amf

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/ellanetworks/core/internal/amf/procedure"
	"github.com/ellanetworks/core/internal/interworking"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/ngap"
	"go.uber.org/zap"
)

// hoState is the stage of an in-flight N2 handover, validated on each transition
// so an out-of-order NGAP message cannot advance it (TS 38.413).
type hoState uint8

const (
	hoPreparing  hoState = iota // HANDOVER REQUEST sent, awaiting Handover Request Acknowledge
	hoPrepared                  // HANDOVER COMMAND sent, awaiting Handover Notify
	hoCommitting                // HANDOVER NOTIFY received, the N2 transfer is in progress
)

type HandoverCandidate struct {
	PDUSessionID ngap.PDUSessionID
	Cause        *ngap.Cause
}

type handoverContext struct {
	state        hoState
	source       *UeConn
	target       *UeConn
	candidates   []HandoverCandidate
	admitted     map[uint8]struct{}
	toEPS        bool
	fromEPS      bool
	relocationID interworking.RelocationID
	relocation   chan relocationOutcome
}

type relocationOutcome struct {
	targetToSource []byte
	unadmitted     []HandoverCandidate
	err            error
}

var ErrRelocationAbandoned = errors.New("amf: handover preparation abandoned")

func deliverRelocationLocked(ho *handoverContext, out relocationOutcome) {
	if ho == nil || ho.relocation == nil {
		return
	}

	ho.relocation <- out

	ho.relocation = nil
}

func (a *AMF) PrepareHandover(ctx context.Context, ue *UeContext, source *UeConn, targetRan *Radio, candidates []HandoverCandidate) (target *UeConn, nh [32]uint8, ncc uint8, ok bool) {
	if ue == nil {
		return nil, [32]uint8{}, 0, false
	}

	if !ue.BeginKeyChainProc(procedure.N2Handover) {
		logger.From(ctx, logger.AmfLog).Info("N2Handover rejected by procedure registry")
		return nil, [32]uint8{}, 0, false
	}

	target, err := a.NewUeConn(targetRan, models.RanUeNgapIDUnspecified)
	if err != nil {
		ue.EndKeyChainProc(procedure.N2Handover)
		logger.From(ctx, logger.AmfLog).Error("error creating target ue", zap.Error(err))

		return nil, [32]uint8{}, 0, false
	}

	nh, ncc, ok = a.stageHandover(ue, source, target, candidates)
	if !ok {
		ue.EndKeyChainProc(procedure.N2Handover)

		if rerr := a.RemoveUeConn(ctx, target); rerr != nil {
			logger.From(ctx, logger.AmfLog).Error("error removing target ue after failed handover preparation", zap.Error(rerr))
		}

		return nil, [32]uint8{}, 0, false
	}

	return target, nh, ncc, true
}

func (a *AMF) stageRelocationToEPS(ue *UeContext, source *UeConn, candidates []HandoverCandidate) (interworking.RelocationID, bool) {
	if ue == nil {
		return 0, false
	}

	if !ue.BeginKeyChainProc(procedure.N2Handover) {
		return 0, false
	}

	id := interworking.RelocationID(a.relocationIDs.Add(1))

	a.mu.Lock()
	ue.handover = &handoverContext{
		state: hoPreparing, source: source, candidates: candidates, toEPS: true, relocationID: id,
	}
	a.mu.Unlock()

	return id, true
}

func (a *AMF) prepareRelocationFromEPS(ue *UeContext, targetRan *Radio, candidates []HandoverCandidate) (*UeConn, <-chan relocationOutcome, bool) {
	if ue == nil {
		return nil, nil, false
	}

	if !ue.BeginKeyChainProc(procedure.N2Handover) {
		return nil, nil, false
	}

	target, err := a.NewUeConn(targetRan, models.RanUeNgapIDUnspecified)
	if err != nil {
		ue.EndKeyChainProc(procedure.N2Handover)
		logger.AmfLog.Error("error creating the target ue for a handover from EPS", zap.Error(err))

		return nil, nil, false
	}

	delivery := make(chan relocationOutcome, 1)

	a.mu.Lock()
	target.ue.Store(ue)
	ue.handover = &handoverContext{
		state:      hoPreparing,
		target:     target,
		candidates: candidates,
		fromEPS:    true,
		relocation: delivery,
	}
	a.mu.Unlock()

	return target, delivery, true
}

func (a *AMF) FinishRelocationPreparation(ue *UeContext, targetToSource []byte, unadmitted []HandoverCandidate) {
	if ue == nil {
		return
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	deliverRelocationLocked(ue.handover, relocationOutcome{targetToSource: targetToSource, unadmitted: unadmitted})
}

func (a *AMF) FailRelocationPreparation(ue *UeContext, err error) {
	if ue == nil {
		return
	}

	a.mu.Lock()

	ho := ue.handover
	if ho == nil {
		a.mu.Unlock()

		return
	}

	deliverRelocationLocked(ho, relocationOutcome{err: err})
	detachAbandonedTargetLocked(ue, ho)
	ue.handover = nil

	a.mu.Unlock()

	ue.EndKeyChainProc(procedure.N2Handover)
}

func (a *AMF) HandoverFromEPS(ue *UeContext) bool {
	if ue == nil {
		return false
	}

	a.mu.RLock()
	defer a.mu.RUnlock()

	return ue.handover != nil && ue.handover.fromEPS
}

func (a *AMF) RelocationToEPS(ue *UeContext) (interworking.RelocationID, bool) {
	if ue == nil {
		return 0, false
	}

	a.mu.RLock()
	defer a.mu.RUnlock()

	if ue.handover == nil || !ue.handover.toEPS {
		return 0, false
	}

	return ue.handover.relocationID, true
}

func (a *AMF) HandoverToEPSInProgress(ue *UeContext) bool {
	if ue == nil {
		return false
	}

	a.mu.RLock()
	defer a.mu.RUnlock()

	return ue.handover != nil && ue.handover.toEPS
}

func (a *AMF) SuperviseHandover(ue *UeContext, source, target *UeConn) {
	ue.SuperviseKeyChainProc(procedure.N2Handover,
		time.Now().Add(a.handoverGuardTimeout),
		handoverGuardExpiry(a, source, target))
}

func (a *AMF) stageHandover(ue *UeContext, source, target *UeConn, candidates []HandoverCandidate) (nh [32]uint8, ncc uint8, ok bool) {
	ue.mu.Lock()

	nh, ncc, err := ue.deriveNextNHLocked()
	if err == nil {
		ue.nh, ue.ncc = nh, ncc
	}
	ue.mu.Unlock()

	if err != nil {
		logger.AmfLog.Error("failed to advance NH for handover", zap.Error(err))
		return [32]uint8{}, 0, false
	}

	a.mu.Lock()
	if target != nil {
		target.ue.Store(ue)
	}

	ue.handover = &handoverContext{state: hoPreparing, source: source, target: target, candidates: candidates}
	a.mu.Unlock()

	return nh, ncc, true
}

func handoverGuardExpiry(a *AMF, sourceUe, targetUe *UeConn) func(context.Context) error {
	return func(cctx context.Context) error {
		amfUe := sourceUe.UeContext()

		if !a.abandonHandover(amfUe) {
			if amfUe != nil && !amfUe.BeginKeyChainProc(procedure.N2Handover) {
				logger.WithTrace(cctx, sourceUe.Log).Error("could not re-claim the key chain for a committing handover")
			}

			return nil
		}

		logger.WithTrace(cctx, sourceUe.Log).Warn("N2 handover abandoned: target gNB did not complete it in time, releasing target")

		a.UnbindHandoverTarget(cctx, amfUe)

		targetUe.ReleaseAction = UeContextReleaseHandover

		targetUe.SendUEContextReleaseCommand(cctx,
			ngap.Cause{Group: ngap.CauseGroupRadioNetwork, Value: ngap.CauseRadioNetworkTNGRelocOverallExpiry})

		return nil
	}
}

func (a *AMF) abandonHandover(ue *UeContext) bool {
	if ue == nil {
		return false
	}

	a.mu.Lock()

	ho := ue.handover
	if ho == nil || ho.state == hoCommitting {
		a.mu.Unlock()
		return false
	}

	deliverRelocationLocked(ho, relocationOutcome{err: ErrRelocationAbandoned})
	detachAbandonedTargetLocked(ue, ho)
	ue.handover = nil

	a.mu.Unlock()

	ue.EndKeyChainProc(procedure.N2Handover)

	return true
}

// SetHandoverForTest installs a preparing handover FSM for a source→target pair without
// claiming the procedure or arming supervision, optionally recording the PDU sessions
// the source asked to hand over. For tests only.
func SetHandoverForTest(sourceUe, targetUe *UeConn, candidates ...HandoverCandidate) error {
	if sourceUe == nil || targetUe == nil {
		return fmt.Errorf("source or target ue is nil")
	}

	amfUe := sourceUe.ue.Load()
	if amfUe == nil {
		return fmt.Errorf("amf ue is nil")
	}

	if _, _, ok := sourceUe.amf.stageHandover(amfUe, sourceUe, targetUe, candidates); !ok {
		return fmt.Errorf("stage handover failed")
	}

	return nil
}

func (a *AMF) ForceHandoverCommittingForTest(ue *UeContext) bool {
	if ue == nil {
		return false
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	if ue.handover == nil {
		return false
	}

	ue.handover.state = hoCommitting

	return true
}

// StageHandoverForTest stages the handover FSM on a bare UE, returning the {NH, NCC}. For tests only.
func (a *AMF) StageHandoverForTest(ue *UeContext) (nh [32]uint8, ncc uint8, ok bool) {
	return a.stageHandover(ue, nil, nil, nil)
}

// MarkHandoverCommitting advances the FSM from hoPrepared to hoCommitting when the
// UE reaches the target (HANDOVER NOTIFY) and returns the target-admitted PDU session
// IDs. ok is false when there is no handover, it is not at hoPrepared, or the notifier
// is not the prepared target — so an out-of-order or spurious HANDOVER NOTIFY is
// rejected before any SMF side effect.
func (a *AMF) MarkHandoverCommitting(ue *UeContext, targetUe *UeConn) (admitted map[uint8]struct{}, ok bool) {
	if ue == nil {
		return nil, false
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	ho := ue.handover
	if ho == nil || ho.state != hoPrepared || ho.target != targetUe {
		return nil, false
	}

	ho.state = hoCommitting

	return ho.admitted, true
}

// FinishHandoverCommit completes a committing handover: it moves the UE onto the
// target UeConn and clears the FSM, atomically under the registry lock. It returns
// false — leaving the UE where it was — when the handover is not in a committing state
// or the target UeConn was released during the (unlocked) user-plane switch, so a
// handover cannot complete onto a UE that has gone away (TS 23.502).
func (a *AMF) FinishHandoverCommit(ue *UeContext, targetUe *UeConn) bool {
	if ue == nil {
		return false
	}

	a.mu.Lock()

	if ue.handover == nil || ue.handover.state != hoCommitting {
		a.mu.Unlock()
		return false
	}

	// The finalize must move the UE onto the prepared target, not whichever UeConn
	// happened to carry the notify, and only while that target is still present after
	// the unlocked user-plane switch.
	if targetUe == nil || ue.handover.target != targetUe || a.conns[int64(targetUe.AmfUeNgapID)] != targetUe {
		a.mu.Unlock()
		return false
	}

	ue.handover = nil
	// The source connection is managed by the handover flow, not released here.
	_ = a.attachUeConnLocked(ue, targetUe)

	targetUe.MarkICSCompleted()

	a.mu.Unlock()

	ue.EndKeyChainProc(procedure.N2Handover)

	return true
}

func (a *AMF) CancelHandover(ue *UeContext) (target *UeConn, aborted bool) {
	if ue == nil {
		return nil, false
	}

	a.mu.Lock()

	ho := ue.handover
	switch {
	case ho == nil:
		// Nothing to cancel; the caller still acknowledges (TS 38.413 §8.4.5).
	case ho.state == hoCommitting:
		// Too late to cancel: acknowledge but let the in-flight NOTIFY finish.
	default:
		target = ho.target
		deliverRelocationLocked(ho, relocationOutcome{err: ErrRelocationAbandoned})
		detachAbandonedTargetLocked(ue, ho)
		ue.handover = nil
		aborted = true
	}

	a.mu.Unlock()

	if aborted {
		ue.EndKeyChainProc(procedure.N2Handover)
	}

	return target, aborted
}

func (a *AMF) UnbindHandoverTarget(ctx context.Context, ue *UeContext) {
	if ue == nil {
		return
	}

	for _, ref := range ue.SmContextRefs() {
		if err := a.Session.UpdateSmContextN2HandoverCanceled(ctx, ref.Ref); err != nil {
			logger.From(ctx, logger.AmfLog).Error("failed to restore the source access tunnel after an abandoned handover",
				zap.Error(err), zap.Uint8("pdu-session-id", ref.PduSessionID))
		}
	}
}

func detachAbandonedTargetLocked(ue *UeContext, ho *handoverContext) {
	if ho == nil || ho.target == nil {
		return
	}

	if ho.target.ue.Load() == ue {
		ho.target.ue.Store(nil)
	}
}

// ClearHandover ends the handover FSM and its key-chain procedure. Idempotent; safe on a nil UE.
func (a *AMF) ClearHandover(ue *UeContext) {
	if ue == nil {
		return
	}

	a.mu.Lock()
	deliverRelocationLocked(ue.handover, relocationOutcome{err: ErrRelocationAbandoned})
	detachAbandonedTargetLocked(ue, ue.handover)
	ue.handover = nil
	a.mu.Unlock()

	ue.EndKeyChainProc(procedure.N2Handover)
}

func (a *AMF) ClearRelocationToEPS(ue *UeContext, id interworking.RelocationID) bool {
	if ue == nil {
		return false
	}

	a.mu.Lock()

	ho := ue.handover
	if ho == nil || !ho.toEPS || ho.relocationID != id {
		a.mu.Unlock()

		return false
	}

	deliverRelocationLocked(ho, relocationOutcome{err: ErrRelocationAbandoned})
	detachAbandonedTargetLocked(ue, ho)
	ue.handover = nil

	a.mu.Unlock()

	ue.EndKeyChainProc(procedure.N2Handover)

	return true
}

func (a *AMF) MarkHandoverPrepared(ue *UeContext, admitted map[uint8]struct{}) (toRelease []HandoverCandidate, ok bool) {
	if ue == nil {
		return nil, false
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	if ue.handover == nil || ue.handover.state != hoPreparing {
		return nil, false
	}

	reported := make(map[ngap.PDUSessionID]struct{}, len(ue.handover.candidates))

	for _, c := range ue.handover.candidates {
		if _, isAdmitted := admitted[uint8(c.PDUSessionID)]; isAdmitted {
			continue
		}

		if _, dup := reported[c.PDUSessionID]; dup {
			continue
		}

		reported[c.PDUSessionID] = struct{}{}
		toRelease = append(toRelease, c)
	}

	ue.handover.admitted = admitted
	ue.handover.state = hoPrepared

	return toRelease, true
}

func (a *AMF) HandoverPreparing(ue *UeContext) bool {
	if ue == nil {
		return false
	}

	a.mu.RLock()
	defer a.mu.RUnlock()

	return ue.handover != nil && ue.handover.state == hoPreparing
}

// HandoverSource returns the source UeConn of the in-flight handover, or nil.
func (a *AMF) HandoverSource(ue *UeContext) *UeConn {
	if ue == nil {
		return nil
	}

	a.mu.RLock()
	defer a.mu.RUnlock()

	if ue.handover == nil {
		return nil
	}

	return ue.handover.source
}

// HandoverTarget returns the target UeConn of the in-flight handover, or nil.
func (a *AMF) HandoverTarget(ue *UeContext) *UeConn {
	if ue == nil {
		return nil
	}

	a.mu.RLock()
	defer a.mu.RUnlock()

	if ue.handover == nil {
		return nil
	}

	return ue.handover.target
}

func (a *AMF) HandoverInProgress(ue *UeContext) bool {
	if ue == nil {
		return false
	}

	a.mu.RLock()
	defer a.mu.RUnlock()

	return ue.handover != nil
}
