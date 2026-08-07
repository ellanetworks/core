// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package smf

import (
	"context"
	"errors"
	"fmt"

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/smf/ngap"
	"go.uber.org/zap"
)

var (
	// ErrSessionNotTransferable reports that no session answers the identity the
	// UE named. Each access maps it to #54.
	ErrSessionNotTransferable = errors.New("session does not exist on the other access")

	// ErrSessionOnOtherDNN reports a session that exists on a different data
	// network from the one the UE named.
	ErrSessionOnOtherDNN = errors.New("session is on another data network")

	// ErrSessionNotMovable reports a session that exists and matches, but cannot
	// move as the request describes.
	ErrSessionNotMovable = errors.New("session cannot move as described")
)

type transferRequest struct {
	Access AccessType
	// EBI is 0 on 5GS.
	EBI    uint8
	Dnn    string
	Snssai *models.Snssai
	Policy *Policy
}

// pendingTransfer is a move the UE asked for and the target access has not yet
// bound. Guarded by mu.
type pendingTransfer struct {
	to     AccessType
	ebi    uint8 // 0 when the target is 5GS
	policy *Policy
}

// droppedSource is the access a committed move left, and the subscriber and
// identity the session held there.
type droppedSource struct {
	supi   etsi.SUPI
	access AccessType
	id     SessionIdentity
}

// The PDU session identity correlates the two accesses (TS 23.502 §4.11.2.2
// step 13, §4.11.2.3 step 9).
func (s *SMF) findTransferable(supi etsi.SUPI, pduSessionID uint8, req transferRequest) (*SMContext, error) {
	if pduSessionID == 0 {
		return nil, fmt.Errorf("%w: the UE named no PDU session identity", ErrSessionNotTransferable)
	}

	if pduSessionID > 15 {
		return nil, fmt.Errorf("%w: PDU session identity %d is outside the range a UE may allocate", ErrSessionNotTransferable, pduSessionID)
	}

	sc := s.currentPDUSession(supi, pduSessionID)
	if sc == nil {
		return nil, fmt.Errorf("%w: no session with PDU session id %d", ErrSessionNotTransferable, pduSessionID)
	}

	if s.GetSession(sc.Ref) != sc {
		return nil, fmt.Errorf("%w: PDU session %d is not in the pool", ErrSessionNotMovable, pduSessionID)
	}

	sc.mu.Lock()
	defer sc.mu.Unlock()

	if err := sc.transferable(req); err != nil {
		return nil, err
	}

	return sc, nil
}

// Caller holds sc.mu.
func (sc *SMContext) transferable(req transferRequest) error {
	// #54 tells the UE the network has no information about the PDU session
	// (TS 24.501 annex B), which is untrue when it exists, so the cases where it
	// does carry their own errors.
	if sc.Access == req.Access {
		return fmt.Errorf("%w: PDU session %d is already on %s", ErrSessionNotMovable, sc.PDUSessionID, req.Access)
	}

	// A second request for a move the target has not bound would hand two accesses
	// a bearer for the same session.
	if sc.pending != nil {
		return fmt.Errorf("%w: PDU session %d is already moving to %s", ErrSessionNotMovable, sc.PDUSessionID, sc.pending.to)
	}

	if sc.Dnn != req.Dnn {
		return fmt.Errorf("%w: PDU session %d is on data network %q, not %q", ErrSessionOnOtherDNN, sc.PDUSessionID, sc.Dnn, req.Dnn)
	}

	// Only the UE moving into 5GS names a slice: TS 24.501 §6.4.1.2 c)2) has it
	// send the S-NSSAI it holds for the session, so a mismatch is the UE naming
	// another session. Leaving 5GS it sends none (§6.1.4.2 a-e) and the network
	// determines one (TS 23.501 §5.15.7.1), so there is nothing to compare against
	// that the UE chose.
	if req.Access == Access5G {
		if sc.Snssai == nil || req.Snssai == nil {
			return fmt.Errorf("%w: PDU session %d cannot have its slice compared", ErrSessionNotMovable, sc.PDUSessionID)
		}

		if !sc.Snssai.Equal(*req.Snssai) {
			return fmt.Errorf("%w: PDU session %d is on slice %+v, not %+v", ErrSessionNotMovable, sc.PDUSessionID, sc.Snssai, req.Snssai)
		}
	}

	if sc.releasing {
		return fmt.Errorf("%w: PDU session %d is being released", ErrSessionNotMovable, sc.PDUSessionID)
	}

	if !sc.hasUserPlane() {
		return fmt.Errorf("%w: PDU session %d has no user plane to move", ErrSessionNotMovable, sc.PDUSessionID)
	}

	return nil
}

// prepareTransfer records the move the UE asked for. The downlink stays on the
// access serving the session: TS 23.401 §5.10.2 step 5 holds it at the PDN GW
// until step 13a, and TS 23.502 §4.11.2.3 step 9 defers the switch to
// §4.3.2.2.1 step 16a. The request answers the UE out of invariants a move
// preserves — the UE address, the uplink F-TEID, the PDU/PDN type and the slice.
// Caller must not hold sc.mu.
func (s *SMF) prepareTransfer(sc *SMContext, req transferRequest) error {
	sc.mu.Lock()
	defer sc.mu.Unlock()

	if err := sc.transferable(req); err != nil {
		return err
	}

	// Validated here and claimed at the commit: an identity claimed for a move the
	// target abandons is held by a session reachable under neither, and a later
	// initial attach then supersedes a live session.
	if req.EBI != 0 {
		if err := s.epsBearerIdentityAvailable(sc, req.EBI); err != nil {
			return err
		}
	}

	// TS 24.501 §6.3.2.5 a) retransmits the PDU SESSION MODIFICATION COMMAND four
	// times and then aborts, and the command names the access the session leaves.
	// The Establishment Accept assigns the session's PTI after this returns, so a
	// 5GS target discards here and an EPS target at the commit.
	if req.Access == Access5G {
		sc.discardOutstandingProcedures()
	}

	sc.pending = &pendingTransfer{to: req.Access, ebi: req.EBI, policy: req.Policy}

	return nil
}

// abandonTransfer drops a move the target access will not bind, leaving the
// session on the access serving it. Caller must not hold sc.mu.
func (sc *SMContext) abandonTransfer() {
	sc.mu.Lock()
	defer sc.mu.Unlock()

	sc.pending = nil
}

// transferCommit is the staged half of a move: the target access's identity and
// QoS are on the session, and the binding entry point's own PFCP modification
// carries them.
type transferCommit struct {
	source   AccessType
	sourceID SessionIdentity
	policy   *Policy
	qers     []*QER
	restore  func()
}

// beginTransferCommit puts the session on the access that is binding its
// downlink, which is where the switch belongs: TS 23.401 §5.10.2 step 13a
// prompts the PDN GW to start routing to the new endpoint, and TS 23.502
// §4.3.2.2.1 step 16a NOTE 11 switches at the N2 response. It returns nil when
// the access binding is the one already serving the session. Caller holds
// sc.mu.
func (s *SMF) beginTransferCommit(ctx context.Context, sc *SMContext, access AccessType) (*transferCommit, error) {
	if sc.pending == nil || sc.pending.to != access {
		return nil, nil
	}

	// The session is unlocked between the request and this bind, so the
	// preconditions the request checked are checked again: a release sets
	// releasing, and a deactivation the UPF refuses drops the tunnel and the PFCP
	// context.
	if sc.releasing {
		return nil, fmt.Errorf("session %q is being released", sc.Ref)
	}

	if !sc.hasUserPlane() {
		return nil, fmt.Errorf("session %q has no user plane to move", sc.Ref)
	}

	source, sourceID := sc.Access, sc.SessionIdentity

	if err := s.setEPSBearerIdentity(sc, sc.pending.ebi); err != nil {
		return nil, err
	}

	sc.Access = access

	policy := transferPolicy(sc.PolicyData, sc.pending.policy)

	qers, restoreQERs, err := stageSessionQERs(sc, policy.QosData.QFI, policy.Ambr.Uplink, policy.Ambr.Downlink)
	if err != nil {
		sc.Access = source
		s.assignEPSBearerIdentity(ctx, sc, sourceID.EBI)

		return nil, fmt.Errorf("stage target access QoS: %w", err)
	}

	return &transferCommit{
		source:   source,
		sourceID: sourceID,
		policy:   policy,
		qers:     qers,
		restore: func() {
			restoreQERs()

			sc.Access = source
			s.assignEPSBearerIdentity(ctx, sc, sourceID.EBI)
		},
	}, nil
}

// finishTransferCommit adopts the target access's policy once the UPF has taken
// the bind, and names the access to stop routing the session (TS 23.502
// §4.11.2.2 step 14, §4.11.2.3 step 10). Caller holds sc.mu.
func (sc *SMContext) finishTransferCommit(c *transferCommit) *droppedSource {
	sc.PolicyData = c.policy
	sc.pending = nil

	// The 5GSM procedures the session carries name the access it left; S1-U
	// terminates ESM in the MME, so EPS runs none of them.
	if sc.Access == Access4G {
		sc.discardOutstandingProcedures()
	}

	return &droppedSource{supi: sc.Supi, access: c.source, id: c.sourceID}
}

// transferPolicy is the target access's policy with what is subscriber scoped
// carried over: network rules are provisioned per subscriber and policy, and the
// QER's QFI keys the UPF's downlink-notification state, so it stays stable
// across the move. Taken at the commit, so a reconcile landing meanwhile is kept.
// Caller holds sc.mu.
func transferPolicy(current, target *Policy) *Policy {
	if current == nil {
		return target
	}

	merged := *target

	if len(merged.NetworkRules) == 0 {
		merged.NetworkRules = current.NetworkRules
	}

	// S1-U carries no PDU Session Container, so EPS names no QoS flow. The 5QI and
	// ARP are the session's own, so both survive the move.
	if merged.QosData == (models.QosData{}) {
		merged.QosData = current.QosData
	}

	return &merged
}

// The session and the UE address survive (TS 23.502 §4.11.2.2 step 14,
// §4.11.2.3 step 10). Caller must not hold sc.mu: the access callbacks
// re-enter the SMF.
func (s *SMF) dropSourceRouting(ctx context.Context, supi etsi.SUPI, ref string, from AccessType, id SessionIdentity) {
	switch from {
	case Access5G:
		if s.amf == nil {
			return
		}

		// A UE in dual-registration mode stays on NG-RAN while it moves sessions one
		// at a time, stranding the moved session's radio resources.
		n2Release, err := ngap.BuildPDUSessionResourceReleaseCommandTransfer()
		if err != nil {
			logger.WithTrace(ctx, logger.SmfLog).Warn("failed to build the N2 release for a moved session; dropping routing only",
				zap.Error(err), logger.SUPI(supi.String()), logger.PDUSessionID(id.PDUSessionID))

			n2Release = nil
		}

		s.amf.SessionTransferred(ctx, supi, id.PDUSessionID, ref, n2Release)
	case Access4G:
		if s.mme != nil {
			s.mme.SessionTransferred(ctx, supi.IMSI(), id.EBI, ref)
		}
	}
}

// assignEPSBearerIdentity is setEPSBearerIdentity where the caller has no answer
// to a failure: it is unwinding a move whose bind the UPF refused, and the
// identity is then out of step with the access the session is on. Caller holds
// sc.mu.
func (s *SMF) assignEPSBearerIdentity(ctx context.Context, sc *SMContext, ebi uint8) {
	if err := s.setEPSBearerIdentity(sc, ebi); err != nil {
		logger.WithTrace(ctx, logger.SmfLog).Error("failed to set the EPS bearer identity of a moved session",
			zap.Error(err), logger.SUPI(sc.Supi.String()),
			logger.PDUSessionID(sc.PDUSessionID), zap.Uint8("ebi", ebi))
	}
}

// discardOutstandingProcedures drops the 5GSM procedures the session left behind
// on the access it moved off: TS 24.501 §6.3.2.5 a) retransmits the PDU SESSION
// MODIFICATION COMMAND four times and then aborts, and the command names that
// access. Caller holds mu.
func (sc *SMContext) discardOutstandingProcedures() {
	sc.stopProcedureTimer()
	sc.pendingPolicy = nil
	sc.establishmentPTI = 0
	sc.outstandingPTIs = nil
}

// reportRelease tells an access holding state for the session that it is gone,
// so it does not keep naming a dead one. Caller must not hold mu: the access
// callbacks re-enter the SMF.
func (s *SMF) reportRelease(ctx context.Context, sc *SMContext, access AccessType, id SessionIdentity) {
	switch {
	case access == Access4G && s.mme != nil:
		s.mme.SessionReleased(ctx, sc.Supi.IMSI(), id.EBI, sc.Ref)
	case access == Access5G && s.amf != nil:
		s.amf.SessionReleased(ctx, sc.Supi, id.PDUSessionID, sc.Ref)
	}
}
