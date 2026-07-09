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
	"github.com/free5gc/ngap/ngapType"
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

// handoverContext is the explicit N2 handover FSM for one UE: the single source of
// truth for the source/target UeConn pair and the procedure's stage. It coordinates
// the source and target connections — which are themselves registry state — so it is
// guarded by AMF.mu, not the per-UE lock. The procedure registry tracks the same
// handover for conflict and supervision and is cleared in lockstep; the SMF owns the
// per-session N2 transfer, this FSM owns only the source/target relationship and
// ordering.
type handoverContext struct {
	state  hoState
	source *UeConn
	target *UeConn
	// admitted is the set of PDU session IDs the target accepted (HANDOVER REQUEST
	// ACKNOWLEDGE); the rest are released at HANDOVER NOTIFY, since a session the
	// target did not accept cannot continue there (TS 23.501 §5.30.3.5).
	admitted map[uint8]struct{}
	// {NH, NCC} advanced for the target, staged at preparation and committed to the
	// UE only at HANDOVER NOTIFY (TS 33.501 §6.9.2.1.1); discarded if the handover is
	// abandoned, so a failed handover never advances the live AS key chain.
	newNH  [32]uint8
	newNCC uint8
}

// PrepareHandover begins N2 handover preparation for the source→target pair: it claims
// the key-chain procedure (exclusive with Security Mode / Path Switch), allocates the
// target UeConn, and stages the {NH, NCC} of the AS key chain (TS 38.413 §8.4, TS 33.501
// §6.9). ok is false with everything rolled back when preparation cannot begin.
func (a *AMF) PrepareHandover(ctx context.Context, ue *UeContext, source *UeConn, targetRan *Radio) (target *UeConn, nh [32]uint8, ncc uint8, ok bool) {
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

	nh, ncc, ok = a.stageHandover(ue, source, target)
	if !ok {
		ue.EndKeyChainProc(procedure.N2Handover)

		if rerr := a.RemoveUeConn(ctx, target); rerr != nil {
			logger.From(ctx, logger.AmfLog).Error("error removing target ue after failed handover preparation", zap.Error(rerr))
		}

		return nil, [32]uint8{}, 0, false
	}

	return target, nh, ncc, true
}

// SuperviseHandover arms the guard bounding HANDOVER REQUIRED → NOTIFY. Arm it only after
// the HANDOVER REQUEST is sent, so the guard timer cannot race the outbound request; on
// expiry it abandons the handover and releases the target.
func (a *AMF) SuperviseHandover(ue *UeContext, source, target *UeConn) {
	ue.SuperviseKeyChainProc(procedure.N2Handover,
		time.Now().Add(a.handoverGuardTimeout),
		handoverGuardExpiry(a, source, target))
}

// stageHandover derives the next {NH, NCC} of the AS key chain (per-UE lock, key material)
// and installs the handover FSM at hoPreparing (registry lock). It neither claims the
// procedure nor arms supervision.
func (a *AMF) stageHandover(ue *UeContext, source, target *UeConn) (nh [32]uint8, ncc uint8, ok bool) {
	ue.mu.Lock()
	nh, ncc, err := ue.deriveNextNHLocked()
	ue.mu.Unlock()

	if err != nil {
		logger.AmfLog.Error("failed to advance NH for handover", zap.Error(err))
		return [32]uint8{}, 0, false
	}

	a.mu.Lock()
	if target != nil {
		target.ue = ue
	}

	ue.handover = &handoverContext{state: hoPreparing, source: source, target: target, newNH: nh, newNCC: ncc}
	a.mu.Unlock()

	return nh, ncc, true
}

// handoverGuardExpiry abandons a stalled N2 handover when the supervision deadline elapses
// before HANDOVER NOTIFY. It releases the half-prepared target; the source is left in place,
// aborted on the radio by its own TNGRELOCprep/Overall timers (TS 38.413).
func handoverGuardExpiry(a *AMF, sourceUe, targetUe *UeConn) func(context.Context) error {
	return func(cctx context.Context) error {
		logger.WithTrace(cctx, sourceUe.Log).Warn("N2 handover abandoned: target gNB did not complete it in time, releasing target")

		a.ClearHandover(sourceUe.UeContext())

		targetUe.ReleaseAction = UeContextReleaseHandover

		targetUe.SendUEContextReleaseCommand(cctx,
			ngapType.CausePresentRadioNetwork,
			ngapType.CauseRadioNetworkPresentTngrelocoverallExpiry)

		return nil
	}
}

// SetHandoverForTest installs a preparing handover FSM for a source→target pair without
// claiming the procedure or arming supervision. For tests only.
func SetHandoverForTest(sourceUe, targetUe *UeConn) error {
	if sourceUe == nil || targetUe == nil {
		return fmt.Errorf("source or target ue is nil")
	}

	amfUe := sourceUe.ue
	if amfUe == nil {
		return fmt.Errorf("amf ue is nil")
	}

	if _, _, ok := sourceUe.amf.stageHandover(amfUe, sourceUe, targetUe); !ok {
		return fmt.Errorf("stage handover failed")
	}

	return nil
}

// StageHandoverForTest stages the handover FSM on a bare UE, returning the {NH, NCC}. For tests only.
func (a *AMF) StageHandoverForTest(ue *UeContext) (nh [32]uint8, ncc uint8, ok bool) {
	return a.stageHandover(ue, nil, nil)
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

	// Commit the staged AS key chain now that the UE has reached the target
	// (TS 33.501 §6.9.2.1.1), under the per-UE lock. An abandoned handover clears the
	// FSM without reaching here, so the live {NH, NCC} is never advanced for a
	// handover that did not complete.
	ue.mu.Lock()
	ue.nh = ho.newNH
	ue.ncc = ho.newNCC
	ue.mu.Unlock()

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
// RanUeNgapIDUnspecified) by its AMF-UE-NGAP-ID alone — BuildUEContextReleaseCommand
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
		ue.handover = nil
		aborted = true
	}

	a.mu.Unlock()

	if aborted {
		ue.EndKeyChainProc(procedure.N2Handover)
	}

	return target, aborted
}

// ClearHandover ends the handover FSM and its key-chain procedure. Idempotent; safe on a nil UE.
func (a *AMF) ClearHandover(ue *UeContext) {
	if ue == nil {
		return
	}

	a.mu.Lock()
	ue.handover = nil
	a.mu.Unlock()

	ue.EndKeyChainProc(procedure.N2Handover)
}

// MarkHandoverPrepared advances the FSM from hoPreparing to hoPrepared when the
// target acknowledges (HANDOVER COMMAND about to be sent) and records the set of PDU
// session IDs the target admitted. It returns false when there is no handover or it
// is not at hoPreparing, so a duplicate or out-of-order HANDOVER REQUEST ACKNOWLEDGE
// is rejected.
func (a *AMF) MarkHandoverPrepared(ue *UeContext, admitted map[uint8]struct{}) bool {
	if ue == nil {
		return false
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	if ue.handover == nil || ue.handover.state != hoPreparing {
		return false
	}

	ue.handover.admitted = admitted
	ue.handover.state = hoPrepared

	return true
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
