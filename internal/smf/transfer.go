// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package smf

import (
	"context"
	"fmt"

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/smf/ngap"
	"github.com/ellanetworks/core/internal/smf/procedure"
	"go.uber.org/zap"
)

// The refusal reasons live on the MME/SMF boundary (internal/models), since each
// access maps them to a NAS cause of its own and neither NF imports the other.
var (
	ErrSessionNotTransferable = models.ErrSessionNotTransferable
	ErrSessionOnOtherDNN      = models.ErrSessionOnOtherDNN
	ErrSessionNotMovable      = models.ErrSessionNotMovable
)

// transferRequest is what the target access says about the session it is
// claiming: which access it is, the identity it will hold there, and the data
// network and slice the UE named.
type transferRequest struct {
	Access AccessType
	// EBI is the default bearer the MME allocated, 0 when the target is 5GS.
	EBI    uint8
	Dnn    string
	Snssai *models.Snssai
	Policy *Policy
}

// pendingTransfer is a move the UE asked for and the target access has not yet
// bound. Guarded by SMContext.Mutex.
type pendingTransfer struct {
	to     AccessType
	ebi    uint8 // 0 when the target is 5GS
	policy *Policy
}

// droppedSource is the access a committed move left, and the identity the
// session held there, so the access can be told to stop routing it.
type droppedSource struct {
	supi   etsi.SUPI
	access AccessType
	id     SessionIdentity
}

// findTransferable resolves the session the UE named for a move. The PDU session
// identity is what correlates the two accesses (TS 23.502 §4.11.2.2 step 13,
// §4.11.2.3 step 9); the UE holds it on both, and the anchor indexes the session
// under it whichever access established it.
func (s *SMF) findTransferable(supi etsi.SUPI, pduSessionID uint8, req transferRequest) (*SMContext, error) {
	// The range a UE may allocate, checked here so both accesses share the
	// precondition rather than each policing its own entry point.
	if pduSessionID == 0 {
		return nil, fmt.Errorf("%w: the UE named no PDU session identity", ErrSessionNotTransferable)
	}

	if pduSessionID > 15 {
		return nil, fmt.Errorf("%w: PDU session identity %d is outside the range a UE may allocate", ErrSessionNotTransferable, pduSessionID)
	}

	sc := s.currentPDUSession(supi, pduSessionID)
	if sc == nil {
		return nil, fmt.Errorf("%w: no session with PDU session identity %d", ErrSessionNotTransferable, pduSessionID)
	}

	if s.GetSession(sc.Ref) != sc {
		return nil, fmt.Errorf("%w: PDU session %d is not in the pool", ErrSessionNotMovable, pduSessionID)
	}

	sc.Mutex.Lock()
	defer sc.Mutex.Unlock()

	if err := sc.transferable(req); err != nil {
		return nil, err
	}

	return sc, nil
}

// transferable reports whether the session can move as req describes. It is
// checked when the UE asks and again under the lock that performs the change,
// since the lock is dropped in between. Caller holds sc.Mutex.
func (sc *SMContext) transferable(req transferRequest) error {
	// #54 tells the UE the network has no information about the session
	// (TS 24.501 annex B), which is untrue when it exists, so every case below
	// carries its own error.
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
	// send the S-NSSAI it holds for the session, so a mismatch means it is naming
	// a different session. Leaving 5GS it sends none (§6.1.4.2) and the network
	// determines one (TS 23.501 §5.15.7.1), so there is nothing the UE chose to
	// compare against.
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

// hasUserPlane reports whether the session still has the UPF session and tunnel
// a move re-points. Caller holds Mutex.
func (sc *SMContext) hasUserPlane() bool {
	return sc.Tunnel != nil && sc.PFCPContext != nil && sc.PFCPContext.Established
}

// prepareTransfer records the move the UE asked for and admits it as this
// session's one procedure. The downlink stays on the access serving the session:
// TS 23.401 §5.10.2 step 5 holds it at the PDN GW until step 13a, and
// TS 23.502 §4.11.2.3 step 9 defers the switch to §4.3.2.2.1 step 16a. The
// caller answers the UE out of invariants a move preserves — the UE address, the
// uplink F-TEID, the PDU/PDN type and the slice. Caller must not hold sc.Mutex.
func (s *SMF) prepareTransfer(sc *SMContext, req transferRequest) error {
	if err := sc.procedures.Begin(procedure.Transfer); err != nil {
		return fmt.Errorf("%w: PDU session %d already has a transfer in flight: %v", ErrSessionNotMovable, sc.PDUSessionID, err)
	}

	committed := false

	defer func() {
		if !committed {
			sc.procedures.End(procedure.Transfer)
		}
	}()

	sc.Mutex.Lock()
	defer sc.Mutex.Unlock()

	if err := sc.transferable(req); err != nil {
		return err
	}

	// Validated here and claimed at the commit: an identity claimed for a move the
	// target abandons is held by a session reachable under neither access, and a
	// later attach that legitimately allocates it is then refused.
	if req.EBI != 0 {
		if err := s.epsBearerIdentityAvailable(sc, req.EBI); err != nil {
			return err
		}
	}

	// TS 24.501 §6.3.2.5 a) retransmits the PDU SESSION MODIFICATION COMMAND four
	// times and then aborts, and the command names the access the session is
	// leaving. The Establishment Accept assigns the session's PTI after this
	// returns, so a 5GS target discards here and an EPS target at the commit.
	if req.Access == Access5G {
		sc.discardOutstandingProcedures()
	}

	sc.pending = &pendingTransfer{to: req.Access, ebi: req.EBI, policy: req.Policy}
	committed = true

	return nil
}

// abandonTransfer drops a move the target access will not bind, leaving the
// session on the access serving it. Caller must not hold sc.Mutex.
func (sc *SMContext) abandonTransfer() {
	sc.Mutex.Lock()
	sc.pending = nil
	sc.Mutex.Unlock()

	sc.procedures.End(procedure.Transfer)
}

// transferCommit is the staged half of a move: the target access's identity and
// QoS are on the session, and the binding entry point's own statement to the UPF
// carries them, so the move costs no extra round trip to the data plane.
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
// the access binding is the one already serving the session, so a bind that is
// not part of a move is unaffected. Caller holds sc.Mutex.
func (s *SMF) beginTransferCommit(ctx context.Context, sc *SMContext, access AccessType) (*transferCommit, error) {
	if sc.pending == nil || sc.pending.to != access {
		return nil, nil
	}

	// The session is unlocked between the request and this bind, so the
	// preconditions the request checked are checked again: a release sets
	// releasing, and a deactivation the UPF refused drops the tunnel.
	if sc.releasing {
		return nil, fmt.Errorf("session %q is being released", sc.Ref)
	}

	if !sc.hasUserPlane() {
		return nil, fmt.Errorf("session %q has no user plane to move", sc.Ref)
	}

	source, sourceID := sc.Access, sc.SessionIdentity

	// Ahead of any call to the UPF, so a rejected bind unwinds bookkeeping only.
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
// §4.11.2.2 step 14, §4.11.2.3 step 10). Caller holds sc.Mutex.
func (sc *SMContext) finishTransferCommit(c *transferCommit) *droppedSource {
	sc.PolicyData = c.policy
	sc.pending = nil

	// The 5GSM procedures the session carries name the access it left; S1-U
	// terminates ESM in the MME, so EPS runs none of them.
	if sc.Access == Access4G {
		sc.discardOutstandingProcedures()
	}

	sc.procedures.End(procedure.Transfer)

	return &droppedSource{supi: sc.Supi, access: c.source, id: c.sourceID}
}

// transferPolicy is the target access's policy with what is subscriber-scoped
// carried over: network rules are provisioned per subscriber and policy, and the
// QER's QFI keys the UPF's downlink-notification state, so it stays stable
// across the move. Taken at the commit, so a reconcile landing meanwhile is
// kept. Caller holds sc.Mutex.
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

// dropSourceRouting tells the access the session left to forget it without
// releasing anything: the session and the UE address survive (TS 23.502
// §4.11.2.2 step 14, §4.11.2.3 step 10), and that access's own release path
// would otherwise tear down a session the UE is now using on the other one.
// Caller must not hold sc.Mutex: the access callbacks re-enter the SMF.
func (s *SMF) dropSourceRouting(ctx context.Context, ref string, dropped *droppedSource) {
	if dropped == nil {
		return
	}

	switch dropped.access {
	case Access5G:
		if s.amf == nil {
			return
		}

		// A UE in dual-registration mode stays on NG-RAN while it moves its sessions
		// one at a time, so the moved session's radio resources have to be released
		// explicitly or they leak at the gNB.
		n2Release, err := ngap.BuildPDUSessionResourceReleaseCommandTransfer()
		if err != nil {
			logger.WithTrace(ctx, logger.SmfLog).Warn("failed to build the N2 release for a moved session; dropping routing only",
				zap.Error(err), logger.SUPI(dropped.supi.String()), logger.PDUSessionID(dropped.id.PDUSessionID))

			n2Release = nil
		}

		s.amf.SessionTransferred(ctx, dropped.supi, dropped.id.PDUSessionID, ref, n2Release)
	case Access4G:
		if s.mme != nil {
			s.mme.SessionTransferred(ctx, dropped.supi.IMSI(), dropped.id.EBI, ref)
		}
	}
}

// discardOutstandingProcedures drops the 5GSM procedures the session left behind
// on the access it moved off: TS 24.501 §6.3.2.5 a) retransmits the PDU SESSION
// MODIFICATION COMMAND four times and then aborts, and the command names that
// access. The abort path drops the session from the pool without releasing its
// UPF session or IP lease, so leaving one armed across a move leaks both.
// Caller holds Mutex.
func (sc *SMContext) discardOutstandingProcedures() {
	sc.stopProcedureTimer()
	sc.pendingPolicy = nil
	sc.establishmentPTI = 0
	sc.outstandingPTIs = nil
}
