// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package amf

import "fmt"

// AttachSourceUeTargetUe binds a fresh target UeConn to the source's UE context and
// installs the N2 handover FSM (source→target). It rolls back the binding if the
// handover cannot begin (TS 38.413 §8.4).
func AttachSourceUeTargetUe(sourceUe, targetUe *UeConn) error {
	if sourceUe == nil {
		return fmt.Errorf("source ue is nil")
	}

	if targetUe == nil {
		return fmt.Errorf("target ue is nil")
	}

	amfUe := sourceUe.ue
	if amfUe == nil {
		return fmt.Errorf("amf ue is nil")
	}

	targetUe.ue = amfUe

	if err := sourceUe.amf.PrepareHandover(amfUe, sourceUe, targetUe); err != nil {
		targetUe.ue = nil

		return fmt.Errorf("begin handover: %w", err)
	}

	return nil
}

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

// PrepareHandover installs the handover FSM at hoPreparing for the source→target
// pair and stages the next {NH, NCC} of the AS key chain (sent to the target in
// HANDOVER REQUEST, committed at NOTIFY). The procedure registry is the primary
// guard against concurrent handovers, so this overwrites any stale context. The NH
// is derived under the per-UE lock (key material); the FSM is installed under the
// registry lock.
func (a *AMF) PrepareHandover(ue *UeContext, source, target *UeConn) error {
	if ue == nil {
		return nil
	}

	ue.mu.Lock()
	nh, ncc, err := ue.deriveNextNHLocked()
	ue.mu.Unlock()

	if err != nil {
		return err
	}

	a.mu.Lock()
	ue.handover = &handoverContext{state: hoPreparing, source: source, target: target, newNH: nh, newNCC: ncc}
	a.mu.Unlock()

	return nil
}

// StagedHandoverNH returns the {NH, NCC} staged for the in-flight handover — the
// value sent to the target in HANDOVER REQUEST. ok is false when no handover is in
// progress.
func (a *AMF) StagedHandoverNH(ue *UeContext) (nh [32]uint8, ncc uint8, ok bool) {
	if ue == nil {
		return [32]uint8{}, 0, false
	}

	a.mu.RLock()
	defer a.mu.RUnlock()

	if ue.handover == nil {
		return [32]uint8{}, 0, false
	}

	return ue.handover.newNH, ue.handover.newNCC, true
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
	defer a.mu.Unlock()

	if ue.handover == nil || ue.handover.state != hoCommitting {
		return false
	}

	// The finalize must move the UE onto the prepared target, not whichever UeConn
	// happened to carry the notify, and only while that target is still present after
	// the unlocked user-plane switch.
	if targetUe == nil || ue.handover.target != targetUe || a.conns[targetUe.AmfUeNgapID] != targetUe {
		return false
	}

	ue.handover = nil
	// The source connection is managed by the handover flow, not released here.
	_ = a.attachUeConnLocked(ue, targetUe)

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
	defer a.mu.Unlock()

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

	return target, aborted
}

// ClearHandover ends the handover FSM, leaving no in-flight handover. Idempotent;
// safe on a nil UE. Kept in lockstep with the procedure registry's End(N2Handover).
func (a *AMF) ClearHandover(ue *UeContext) {
	if ue == nil {
		return
	}

	a.mu.Lock()
	ue.handover = nil
	a.mu.Unlock()
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
