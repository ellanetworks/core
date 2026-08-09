// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package amf

import (
	"context"
	"fmt"
	"time"

	"github.com/ellanetworks/core/internal/amf/procedure"
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
	state      hoState
	source     *UeConn
	target     *UeConn
	candidates []HandoverCandidate
	admitted   map[uint8]struct{}
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

// handoverGuardExpiry abandons a stalled N2 handover when the supervision deadline elapses
// before HANDOVER NOTIFY. It releases the half-prepared target; the source is left in place,
// aborted on the radio by its own TNGRELOCprep/Overall timers (TS 38.413).
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

	a.mu.Unlock()

	ue.EndKeyChainProc(procedure.N2Handover)

	return true
}

// CancelHandover resolves a source-initiated HANDOVER CANCEL against the FSM,
// atomically under the registry lock. A cancellable handover (preparing or
// prepared) is cleared and aborted is true; a committing handover is left intact
// for HANDOVER NOTIFY to finish (aborted false), so a cancel racing the unlocked
// user-plane switch cannot strand the UE on a released target (TS 38.413 §8.4.5).
//
// target is the reserved target UeConn to release, non-nil whenever a handover is
// aborted (hoPreparing or hoPrepared): TS 38.413 §8.4.5 requires freeing the target's
// reserved resources on cancel. A prepared target is addressed by its full UE NGAP ID
// pair; a still-preparing target (whose RAN-UE-NGAP-ID has not yet arrived, so it holds
// RanUeNgapIDUnspecified) by its AMF-UE-NGAP-ID alone — ueContextReleaseCommandBytes
// selects the alternative.
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

// UnbindHandoverTarget restores the source access tunnel after an abandoned N2
// handover. It must run on every abandonment path, not just the ones that answer
// the gNB: HANDOVER REQUEST ACKNOWLEDGE points the SMF's downlink FAR at the
// target, and a snapshot left behind here is later restored onto a gNB the UE
// was never on. Idempotent — the SMF no-ops when it holds no snapshot — so it is
// safe on paths that never reached the acknowledge. Must not be called holding
// a.mu; it calls into the SMF.
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

// detachAbandonedTargetLocked unbinds the UE from a target UeConn staged for a
// handover that will not complete. Left bound, the target's release takes the
// "connection still carries a UE" arm of ReleaseUeConn and deactivates every PDU
// session of a UE that never left the source — undoing the tunnel restore. The
// success path must not use this: attachUeConnLocked moves the UE onto the
// target and detaches the source instead. Caller holds a.mu.
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
	detachAbandonedTargetLocked(ue, ue.handover)
	ue.handover = nil
	a.mu.Unlock()

	ue.EndKeyChainProc(procedure.N2Handover)
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

	// TS 38.413 §8.4.1.4 defines no abnormal condition for a HANDOVER REQUIRED that
	// names the same PDU session twice, so it is carried as-is — but the list this
	// builds must still name each session once.
	reported := make(map[ngap.PDUSessionID]struct{}, len(ue.handover.candidates))

	for _, c := range ue.handover.candidates {
		// An out-of-range PDU session ID is never offered and so never admitted;
		// the lookup is safe because PDUSessionID and the admitted key are both
		// one octet.
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

// HandoverPreparing reports whether a handover is in progress and still at the
// preparing stage, without advancing it. It lets HANDOVER REQUEST ACKNOWLEDGE
// drop a duplicate before validating the admitted-session list, so a duplicate
// cannot tear down an already-prepared handover.
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
