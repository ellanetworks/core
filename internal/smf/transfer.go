// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package smf

import (
	"context"
	"errors"
	"fmt"

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/guard"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/smf/ngap"
	"github.com/ellanetworks/core/internal/smf/procedure"
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

	// Only the session the pool holds under this ref is movable; membership is a
	// precondition of the move, stated here.
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

// Caller holds sc.Mutex.
func (sc *SMContext) transferable(req transferRequest) error {
	// #54 tells the UE the network has no information about the PDU session
	// (TS 24.501 annex B), which is untrue when it exists, so the cases where it
	// does carry their own errors.
	if sc.Access == req.Access {
		return fmt.Errorf("%w: PDU session %d is already on %s", ErrSessionNotMovable, sc.PDUSessionID, req.Access)
	}

	if sc.Dnn != req.Dnn {
		return fmt.Errorf("%w: PDU session %d is on data network %q, not %q", ErrSessionOnOtherDNN, sc.PDUSessionID, sc.Dnn, req.Dnn)
	}

	// The only slice boundary on the transfer path, so an absent S-NSSAI on either
	// side fails closed.
	if sc.Snssai == nil || req.Snssai == nil {
		return fmt.Errorf("%w: PDU session %d cannot have its slice compared", ErrSessionNotMovable, sc.PDUSessionID)
	}

	if !sc.Snssai.Equal(*req.Snssai) {
		return fmt.Errorf("%w: PDU session %d is on slice %+v, not %+v", ErrSessionNotMovable, sc.PDUSessionID, sc.Snssai, req.Snssai)
	}

	if sc.releasing {
		return fmt.Errorf("%w: PDU session %d is being released", ErrSessionNotMovable, sc.PDUSessionID)
	}

	if sc.Tunnel == nil || sc.Tunnel.DataPath == nil || sc.Tunnel.DataPath.DownLinkTunnel == nil ||
		sc.Tunnel.DataPath.DownLinkTunnel.PDR == nil || sc.PFCPContext == nil {
		return fmt.Errorf("%w: PDU session %d has no user plane to move", ErrSessionNotMovable, sc.PDUSessionID)
	}

	return nil
}

// Caller holds sc.Mutex.
func downlinkSnapshot(dl *PDR) func() {
	state, action := dl.State, dl.FAR.ApplyAction
	farState := dl.FAR.State

	var ohc *models.OuterHeaderCreation
	if dl.FAR.ForwardingParameters != nil {
		ohc = dl.FAR.ForwardingParameters.OuterHeaderCreation
	}

	return func() {
		dl.State, dl.FAR.ApplyAction, dl.FAR.State = state, action, farState
		if dl.FAR.ForwardingParameters != nil {
			dl.FAR.ForwardingParameters.OuterHeaderCreation = ohc
		}
	}
}

// Session continuity (TS 23.501 §5.17.2): the UE address, the UPF session, its
// SEID and its uplink F-TEID all survive the move. The downlink buffers until
// the target access binds its own RAN endpoint.
func (s *SMF) transferSession(ctx context.Context, sc *SMContext, req transferRequest) error {
	// A conflicting procedure holds sc.Mutex across blocking UPF calls, so the
	// registry refuses the move instead of queueing it behind one.
	if err := sc.procedures.Begin(procedure.Transfer); err != nil {
		return fmt.Errorf("%w: %v", ErrSessionNotTransferable, err)
	}

	defer sc.procedures.End(procedure.Transfer)

	return s.moveSession(ctx, sc, req)
}

// transferState is what a session carries from the commit of a move until the
// target access binds its downlink.
type transferState struct {
	// epoch counts the moves this session has made. Ref is invariant across a
	// transfer, so it cannot tell a session that came back from the one a stale
	// notification names; the epoch can.
	epoch uint64

	// timer supervises the target access's bind. Its expiry releases the session,
	// so a RAN that never answers cannot leave one access routing a session
	// another owns.
	timer guard.Guard

	// pendingReleases names each access a transfer moved the session off, held
	// until the target access binds its downlink (TS 23.502 §4.11.2.2 step 14,
	// §4.11.2.3 step 10). A round trip taken before the first is dropped leaves two.
	pendingReleases []sourceRelease
}

// sourceRelease is the access a transfer moved a session off, and the identity
// it held there.
type sourceRelease struct {
	from  AccessType
	id    SessionIdentity
	epoch uint64
}

// superviseTargetBind arms the release that bounds the window in which the source
// access still routes a moved session. Caller holds sc.Mutex.
func (s *SMF) superviseTargetBind(sc *SMContext) {
	ref, supi, pduSessionID := sc.Ref, sc.Supi, sc.PDUSessionID

	sc.transfer.timer.Arm(s.transferBind, 0, nil, func() {
		live := s.GetSession(ref)
		if live == nil {
			return
		}

		// The guard drops its own lock before running this, so a Stop racing the
		// firing cannot cancel it. The outstanding release is the authority on
		// whether the bind is still pending, and it is cleared under sc.Mutex.
		live.Mutex.Lock()
		pending := len(live.transfer.pendingReleases) > 0
		target := live.Access
		live.Mutex.Unlock()

		if !pending {
			return
		}

		logger.SmfLog.Warn("target access never bound a transferred session; releasing it",
			logger.SUPI(supi.String()), logger.PDUSessionID(pduSessionID))

		// The target access installed its own state from the accept it already
		// sent, and the release below tells only the source, so it is told here.
		s.reportReleaseToTargetAccess(context.Background(), live, target)

		if err := s.releaseSmContext(context.Background(), ref, anyAccess); err != nil {
			logger.SmfLog.Error("failed to release a session whose target access never bound",
				zap.Error(err), zap.String("smContextRef", ref))
		}
	})
}

// reportReleaseToTargetAccess tells the access a transfer moved the session onto
// that its anchor session is gone, so it does not keep state naming a dead one.
// Caller must not hold sc.Mutex.
func (s *SMF) reportReleaseToTargetAccess(ctx context.Context, sc *SMContext, target AccessType) {
	sc.Mutex.Lock()
	id, ref := sc.SessionIdentity, sc.Ref
	sc.Mutex.Unlock()

	switch {
	case target == Access4G && s.mme != nil:
		s.mme.SessionReleased(ctx, sc.Supi.IMSI(), id.EBI, ref)
	case target == Access5G && s.amf != nil:
		s.amf.SessionReleased(ctx, sc.Supi, id.PDUSessionID, ref)
	}
}

// drainOnTeardown drops the routing a removed session left behind. A session
// still in the pool is mid-transfer, and its source access keeps routing until
// the target binds. Caller must not hold sc.Mutex.
func (s *SMF) drainOnTeardown(ctx context.Context, sc *SMContext) {
	if s.GetSession(sc.Ref) != nil {
		return
	}

	s.releaseTransferSource(ctx, sc)
}

// releaseTransferSource drops the routing the session left behind. Caller must
// not hold sc.Mutex.
func (s *SMF) releaseTransferSource(ctx context.Context, sc *SMContext) {
	// Claimed under one hold, so each release runs once however the transfer
	// concludes and no move can land between the epoch and the entries it filters.
	sc.Mutex.Lock()
	sc.transfer.timer.Stop()
	epoch := sc.transfer.epoch
	claimed := sc.transfer.pendingReleases
	sc.transfer.pendingReleases = nil
	sc.Mutex.Unlock()

	for _, pending := range claimed {
		// A move landing between the record and the drop makes this notification
		// stale: the access it names may be the one serving the session again.
		if pending.epoch != epoch {
			continue
		}

		s.dropSourceRouting(ctx, sc.Supi, sc.Ref, pending.from, pending.id)
	}
}

// The session and the UE address survive (TS 23.502 §4.11.2.2 step 14,
// §4.11.2.3 step 10).
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
// to a failure: it is either unwinding an abandoned move or has already
// committed one, and both leave the identity out of step with the access the
// session is on. Caller holds sc.Mutex.
func (s *SMF) assignEPSBearerIdentity(ctx context.Context, sc *SMContext, ebi uint8) {
	if err := s.setEPSBearerIdentity(sc, ebi); err != nil {
		logger.WithTrace(ctx, logger.SmfLog).Error("failed to set the EPS bearer identity of a moved session",
			zap.Error(err), logger.SUPI(sc.Supi.String()),
			logger.PDUSessionID(sc.PDUSessionID), zap.Uint8("ebi", ebi))
	}
}

// recordSourceRelease names the access the session is leaving, held until the
// target binds its downlink (TS 23.502 §4.11.2.2 step 14 follows step 13;
// §4.11.2.3 step 10 follows the user plane switch of step 9). Caller holds Mutex.
func (sc *SMContext) recordSourceRelease(source sourceRelease, target AccessType) {
	// A round trip taken before the first source was dropped returns the session
	// to an access it is recorded as having left. Ref is invariant across a
	// transfer, so that stale entry names the live session and the drop-callback
	// ref guards cannot tell them apart; it is discarded.
	kept := sc.transfer.pendingReleases[:0]

	for _, pending := range sc.transfer.pendingReleases {
		if pending.from != target {
			kept = append(kept, pending)
		}
	}

	sc.transfer.epoch++
	source.epoch = sc.transfer.epoch
	sc.transfer.pendingReleases = append(kept, source)
}

// discardOutstandingProcedures drops the 5GSM procedures the session left behind
// on the access it moved off: TS 24.501 §6.3.2.5 a) retransmits the PDU SESSION
// MODIFICATION COMMAND four times and then aborts, and the command names that
// access. Caller holds Mutex.
func (sc *SMContext) discardOutstandingProcedures() {
	sc.stopProcedureTimer()
	sc.pendingPolicy = nil
	sc.establishmentPTI = 0
	sc.outstandingPTIs = nil
}

// undoStack unwinds staged changes in the reverse of the order they were made,
// so each undo sees the state its own stage left behind. Running it empties it,
// so a caller that unwinds and returns cannot unwind twice.
type undoStack []func()

func (u *undoStack) push(undo func()) { *u = append(*u, undo) }

func (u *undoStack) run() {
	for i := len(*u) - 1; i >= 0; i-- {
		(*u)[i]()
	}

	*u = nil
}

func (s *SMF) moveSession(ctx context.Context, sc *SMContext, req transferRequest) error {
	sc.Mutex.Lock()
	defer sc.Mutex.Unlock()

	from, sourceID := sc.Access, sc.SessionIdentity

	// A release, a replacement or another transfer can land before this lock.
	if err := sc.transferable(req); err != nil {
		return err
	}

	// Everything up to the UPF modification is staged and undoable, so a refusal
	// anywhere in the ladder leaves the session on the access it started from.
	var unwind undoStack

	// Claiming the target access's bearer identity can fail, so it precedes the
	// UPF change. Giving up the source one cannot be undone reliably — another
	// session may claim it meanwhile — so it follows.
	if req.EBI != 0 {
		if err := s.setEPSBearerIdentity(sc, req.EBI); err != nil {
			return err
		}

		unwind.push(func() { s.assignEPSBearerIdentity(ctx, sc, sourceID.EBI) })
	}

	policy := req.Policy
	if len(policy.NetworkRules) == 0 && sc.PolicyData != nil {
		// Network rules are subscriber and policy scoped, not access scoped.
		policy.NetworkRules = sc.PolicyData.NetworkRules
	}

	// The QoS change and the downlink suspend travel in one PFCP modification, so
	// a rejected transfer cannot leave the UPF holding half of it.
	qerList, restoreQERs, err := stageSessionQERs(sc, policy.QosData.QFI, policy.Ambr.Uplink, policy.Ambr.Downlink)
	if err != nil {
		unwind.run()

		return fmt.Errorf("stage target access QoS: %w", err)
	}

	unwind.push(restoreQERs)

	unwind.push(downlinkSnapshot(sc.Tunnel.DataPath.DownLinkTunnel.PDR))

	farList, err := handleUpCnxStateDeactivate(sc)
	if err != nil {
		unwind.run()

		return fmt.Errorf("suspend downlink: %w", err)
	}

	if err := s.upf.ModifySession(ctx, BuildModifyRequest(sc.PFCPContext.RemoteSEID, policy.PolicyID, nil, farList, qerList)); err != nil {
		unwind.run()

		return fmt.Errorf("move session in the UPF: %w", err)
	}

	if req.EBI == 0 {
		s.assignEPSBearerIdentity(ctx, sc, 0)
	}

	// Recorded with the access change so a release landing between critical
	// sections cannot leave it on a dead context.
	sc.recordSourceRelease(sourceRelease{from: from, id: sourceID}, req.Access)
	s.superviseTargetBind(sc)

	sc.Access = req.Access
	sc.PolicyData = policy

	if req.Snssai != nil {
		sc.Snssai = req.Snssai
	}

	sc.discardOutstandingProcedures()

	logger.WithTrace(ctx, logger.SmfLog).Info("moved session to the other access",
		logger.SUPI(sc.Supi.String()), logger.PDUSessionID(sc.PDUSessionID),
		zap.Stringer("from", from), zap.Stringer("to", req.Access))

	return nil
}
