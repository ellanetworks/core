// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package amf

// hoState is the stage of an in-flight N2 handover, validated on each transition
// so an out-of-order NGAP message cannot advance it. It mirrors the 4G MME's
// hoState (TS 38.413 §8.4).
type hoState uint8

const (
	hoPreparing  hoState = iota // HANDOVER REQUEST sent, awaiting Handover Request Acknowledge
	hoPrepared                  // HANDOVER COMMAND sent, awaiting Handover Notify
	hoCommitting                // HANDOVER NOTIFY received, the N2 transfer is in progress
)

// handoverContext is the explicit N2 handover FSM for one UE: the single source of
// truth for the source/target RanUe pair and the procedure's stage. It mirrors the
// 4G MME's handoverContext and is guarded by UeContext.mu. The procedure registry
// tracks the same handover for conflict and supervision (§6.9.5.1) and is cleared in
// lockstep with it; the SMF owns the per-session N2 transfer; this FSM owns only the
// source/target relationship and the ordering.
type handoverContext struct {
	state  hoState
	source *RanUe
	target *RanUe
}

// BeginHandover installs the handover FSM at hoPreparing for the source→target
// pair. The procedure registry is the primary guard against concurrent handovers,
// so this overwrites any stale context.
func (ue *UeContext) BeginHandover(source, target *RanUe) {
	if ue == nil {
		return
	}

	ue.mu.Lock()
	ue.handover = &handoverContext{state: hoPreparing, source: source, target: target}
	ue.mu.Unlock()
}

// MarkHandoverCommitting advances the FSM from hoPrepared to hoCommitting when the
// UE reaches the target (HANDOVER NOTIFY). It returns false when there is no
// handover or it is not at hoPrepared, so an out-of-order HANDOVER NOTIFY is
// rejected. The AMF commits synchronously, so the context is then ended by the
// caller via ClearHandover.
func (ue *UeContext) MarkHandoverCommitting() bool {
	if ue == nil {
		return false
	}

	ue.mu.Lock()
	defer ue.mu.Unlock()

	if ue.handover == nil || ue.handover.state != hoPrepared {
		return false
	}

	ue.handover.state = hoCommitting

	return true
}

// ClearHandover ends the handover FSM, leaving no in-flight handover. Idempotent;
// safe on a nil receiver. Called wherever End(N2Handover) is called, so the FSM
// stays in lockstep with the procedure registry.
func (ue *UeContext) ClearHandover() {
	if ue == nil {
		return
	}

	ue.mu.Lock()
	ue.handover = nil
	ue.mu.Unlock()
}

// MarkHandoverPrepared advances the FSM from hoPreparing to hoPrepared when the
// target acknowledges (HANDOVER COMMAND about to be sent). It returns false when
// there is no handover or it is not at hoPreparing, so a duplicate or out-of-order
// HANDOVER REQUEST ACKNOWLEDGE is rejected.
func (ue *UeContext) MarkHandoverPrepared() bool {
	if ue == nil {
		return false
	}

	ue.mu.Lock()
	defer ue.mu.Unlock()

	if ue.handover == nil || ue.handover.state != hoPreparing {
		return false
	}

	ue.handover.state = hoPrepared

	return true
}

// HandoverSource returns the source RanUe of the in-flight handover, or nil.
func (ue *UeContext) HandoverSource() *RanUe {
	if ue == nil {
		return nil
	}

	ue.mu.RLock()
	defer ue.mu.RUnlock()

	if ue.handover == nil {
		return nil
	}

	return ue.handover.source
}

// HandoverTarget returns the target RanUe of the in-flight handover, or nil.
func (ue *UeContext) HandoverTarget() *RanUe {
	if ue == nil {
		return nil
	}

	ue.mu.RLock()
	defer ue.mu.RUnlock()

	if ue.handover == nil {
		return nil
	}

	return ue.handover.target
}

// HandoverInProgress reports whether a handover FSM is installed for this UE.
func (ue *UeContext) HandoverInProgress() bool {
	if ue == nil {
		return false
	}

	ue.mu.RLock()
	defer ue.mu.RUnlock()

	return ue.handover != nil
}
