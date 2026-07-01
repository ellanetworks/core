// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package amf

// hoState is the stage of an in-flight N2 handover, validated on each transition
// so an out-of-order NGAP message cannot advance it (TS 38.413).
type hoState uint8

const (
	hoPreparing  hoState = iota // HANDOVER REQUEST sent, awaiting Handover Request Acknowledge
	hoPrepared                  // HANDOVER COMMAND sent, awaiting Handover Notify
	hoCommitting                // HANDOVER NOTIFY received, the N2 transfer is in progress
)

// handoverContext is the explicit N2 handover FSM for one UE: the single source of
// truth for the source/target RanUe pair and the procedure's stage. It coordinates
// the source and target connections — which are themselves registry state — so it is
// guarded by AMF.mu, not the per-UE lock. The procedure registry tracks the same
// handover for conflict and supervision and is cleared in lockstep; the SMF owns the
// per-session N2 transfer, this FSM owns only the source/target relationship and
// ordering.
type handoverContext struct {
	state  hoState
	source *RanUe
	target *RanUe
	// {NH, NCC} advanced for the target, staged at preparation and committed to the
	// UE only at HANDOVER NOTIFY (TS 33.501 §6.9.2.1.1); discarded if the handover is
	// abandoned, so a failed handover never advances the live AS key chain.
	newNH  [32]uint8
	newNCC uint8
}

// BeginHandover installs the handover FSM at hoPreparing for the source→target
// pair and stages the next {NH, NCC} of the AS key chain (sent to the target in
// HANDOVER REQUEST, committed at NOTIFY). The procedure registry is the primary
// guard against concurrent handovers, so this overwrites any stale context. The NH
// is derived under the per-UE lock (key material); the FSM is installed under the
// registry lock.
func (a *AMF) BeginHandover(ue *UeContext, source, target *RanUe) error {
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
// UE reaches the target (HANDOVER NOTIFY). It returns false when there is no
// handover or it is not at hoPrepared, so an out-of-order HANDOVER NOTIFY is
// rejected.
func (a *AMF) MarkHandoverCommitting(ue *UeContext) bool {
	if ue == nil {
		return false
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	if ue.handover == nil || ue.handover.state != hoPrepared {
		return false
	}

	ue.handover.state = hoCommitting

	// Commit the staged AS key chain now that the UE has reached the target
	// (TS 33.501 §6.9.2.1.1), under the per-UE lock. An abandoned handover clears the
	// FSM without reaching here, so the live {NH, NCC} is never advanced for a
	// handover that did not complete.
	ue.mu.Lock()
	ue.nh = ue.handover.newNH
	ue.ncc = ue.handover.newNCC
	ue.mu.Unlock()

	return true
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
// target acknowledges (HANDOVER COMMAND about to be sent). It returns false when
// there is no handover or it is not at hoPreparing, so a duplicate or out-of-order
// HANDOVER REQUEST ACKNOWLEDGE is rejected.
func (a *AMF) MarkHandoverPrepared(ue *UeContext) bool {
	if ue == nil {
		return false
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	if ue.handover == nil || ue.handover.state != hoPreparing {
		return false
	}

	ue.handover.state = hoPrepared

	return true
}

// HandoverSource returns the source RanUe of the in-flight handover, or nil.
func (a *AMF) HandoverSource(ue *UeContext) *RanUe {
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

// HandoverTarget returns the target RanUe of the in-flight handover, or nil.
func (a *AMF) HandoverTarget(ue *UeContext) *RanUe {
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
