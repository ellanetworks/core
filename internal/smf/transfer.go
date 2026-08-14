// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package smf

import (
	"context"
	"fmt"
	"time"

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/smf/ngap"
	"go.uber.org/zap"
)

var (
	ErrSessionNotTransferable = models.ErrSessionNotTransferable
	ErrSessionOnOtherDNN      = models.ErrSessionOnOtherDNN
	ErrSessionNotMovable      = models.ErrSessionNotMovable
)

type transferRequest struct {
	Access AccessType
	EBI    uint8
	Dnn    string
	Snssai *models.Snssai
	Policy *Policy
}

type pendingTransfer struct {
	to     AccessType
	ebi    uint8
	policy *Policy
}

type droppedSource struct {
	supi     etsi.SUPI
	access   AccessType
	id       SessionIdentity
	upActive bool
}

func (s *SMF) findTransferable(supi etsi.SUPI, pduSessionID uint8, req transferRequest) (*SMContext, error) {
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

func (sc *SMContext) transferable(req transferRequest) error {
	if sc.Access == req.Access {
		return fmt.Errorf("%w: PDU session %d is already on %s", ErrSessionNotMovable, sc.PDUSessionID, req.Access)
	}

	if sc.pending != nil {
		return fmt.Errorf("%w: PDU session %d is already moving to %s", ErrSessionNotMovable, sc.PDUSessionID, sc.pending.to)
	}

	if sc.Dnn != req.Dnn {
		return fmt.Errorf("%w: PDU session %d is on data network %q, not %q", ErrSessionOnOtherDNN, sc.PDUSessionID, sc.Dnn, req.Dnn)
	}

	if req.Access == Access5G && (sc.Snssai == nil || req.Snssai == nil) {
		return fmt.Errorf("%w: PDU session %d cannot have its slice compared", ErrSessionNotMovable, sc.PDUSessionID)
	}

	if sc.Snssai != nil && req.Snssai != nil && !sc.Snssai.Equal(*req.Snssai) {
		return fmt.Errorf("%w: PDU session %d is on slice %+v, not %+v", ErrSessionNotMovable, sc.PDUSessionID, sc.Snssai, req.Snssai)
	}

	if sc.releasing {
		return fmt.Errorf("%w: PDU session %d is being released", ErrSessionNotMovable, sc.PDUSessionID)
	}

	if !sc.hasUserPlane() {
		return fmt.Errorf("%w: PDU session %d has no user plane to move", ErrSessionNotMovable, sc.PDUSessionID)
	}

	return nil
}

func (sc *SMContext) hasUserPlane() bool {
	return sc.Tunnel != nil && sc.PFCPContext != nil && sc.PFCPContext.Established
}

func (s *SMF) prepareTransfer(sc *SMContext, req transferRequest) error {
	sc.Mutex.Lock()
	defer sc.Mutex.Unlock()

	if err := sc.transferable(req); err != nil {
		return err
	}

	if req.EBI != 0 {
		if err := s.epsBearerIdentityAvailable(sc, req.EBI); err != nil {
			return err
		}
	}

	if req.Access == Access5G {
		sc.discardOutstandingProcedures()
	}

	move := &pendingTransfer{to: req.Access, ebi: req.EBI, policy: req.Policy}
	sc.pending = move

	sc.transferGuard.ArmOnce(transferSupervision, func() {
		sc.Mutex.Lock()

		if sc.pending != move {
			sc.Mutex.Unlock()
			return
		}

		sc.pending = nil
		supi, pduSessionID, ref := sc.Supi, sc.PDUSessionID, sc.Ref
		sc.handoverTargetAN = nil

		sc.Mutex.Unlock()

		logger.SmfLog.Warn("abandoning a move the target access never bound",
			logger.SUPI(supi.String()), logger.PDUSessionID(pduSessionID),
			zap.Stringer("to", move.to))

		if move.to == Access5G && s.amf != nil {
			s.amf.SessionDropped(context.Background(), supi, pduSessionID, ref, nil)
		}
	})

	return nil
}

var transferSupervision = 30 * time.Second

func (sc *SMContext) clearPendingLocked() {
	sc.pending = nil
	sc.transferGuard.Stop()
}

func (sc *SMContext) abandonPendingLocked() {
	sc.clearPendingLocked()
	sc.handoverTargetAN = nil
}

func (sc *SMContext) abandonTransferTo(access AccessType) {
	sc.Mutex.Lock()
	defer sc.Mutex.Unlock()

	if sc.pending != nil && sc.pending.to == access {
		sc.abandonPendingLocked()
	}
}

// transferCommit is the non-data-plane half of a move: the session's access and
// EPS bearer identity, already switched to the target, and what it takes to put
// them back if the data plane refuses the move.
type transferCommit struct {
	source   AccessType
	sourceID SessionIdentity
	sourceUP bool
	policy   *Policy
	restore  func()
}

func (s *SMF) beginTransferCommit(ctx context.Context, sc *SMContext, access AccessType) (*transferCommit, error) {
	if sc.pending == nil || sc.pending.to != access {
		return nil, nil
	}

	move := sc.pending
	sc.clearPendingLocked()

	if sc.releasing {
		return nil, fmt.Errorf("session %q is being released", sc.Ref)
	}

	if !sc.hasUserPlane() {
		return nil, fmt.Errorf("session %q has no user plane to move", sc.Ref)
	}

	source, sourceID, sourceUP := sc.Access, sc.SessionIdentity, sc.upConnectionActive()

	if err := s.setEPSBearerIdentity(sc, move.ebi); err != nil {
		return nil, err
	}

	sc.Access = access

	return &transferCommit{
		source:   source,
		sourceID: sourceID,
		sourceUP: sourceUP,
		policy:   transferPolicy(sc.PolicyData, move.policy),
		restore: func() {
			sc.Access = source
			s.assignEPSBearerIdentity(ctx, sc, sourceID.EBI)
		},
	}, nil
}

func (sc *SMContext) finishTransferCommit(c *transferCommit) *droppedSource {
	sc.PolicyData = c.policy

	if sc.Access == Access4G {
		sc.discardOutstandingProcedures()
	}

	return &droppedSource{supi: sc.Supi, access: c.source, id: c.sourceID, upActive: c.sourceUP}
}

func transferPolicy(current, target *Policy) *Policy {
	if current == nil {
		return target
	}

	merged := *target

	if len(merged.NetworkRules) == 0 {
		merged.NetworkRules = current.NetworkRules
	}

	if merged.QosData == (models.QosData{}) {
		merged.QosData = current.QosData
	}

	return &merged
}

func (s *SMF) dropSourceRouting(ctx context.Context, ref string, dropped *droppedSource) {
	if dropped == nil {
		return
	}

	switch dropped.access {
	case Access5G:
		if s.amf == nil {
			return
		}

		// TS 23.502 §4.11.2.2 step 14
		var n2Release []byte

		if dropped.upActive {
			built, err := ngap.BuildPDUSessionResourceReleaseCommandTransfer()
			if err != nil {
				logger.WithTrace(ctx, logger.SmfLog).Warn("failed to build the N2 release for a moved session; dropping routing only",
					zap.Error(err), logger.SUPI(dropped.supi.String()), logger.PDUSessionID(dropped.id.PDUSessionID))
			} else {
				n2Release = built
			}
		}

		s.amf.SessionDropped(ctx, dropped.supi, dropped.id.PDUSessionID, ref, n2Release)
	case Access4G:
		if s.mme != nil {
			s.mme.SessionDropped(ctx, dropped.supi.IMSI(), dropped.id.EBI, ref)
		}
	}
}

func (sc *SMContext) discardOutstandingProcedures() {
	sc.stopProcedureTimer()
	sc.pendingPolicy = nil
	sc.establishmentPTI = 0
	sc.outstandingPTIs = nil
}

func (sc *SMContext) takeAbandonedMoveTo(access AccessType) (etsi.SUPI, uint8, string, bool) {
	sc.Mutex.Lock()
	defer sc.Mutex.Unlock()

	if sc.pending == nil || sc.pending.to != access {
		return sc.Supi, sc.PDUSessionID, sc.Ref, false
	}

	sc.clearPendingLocked()

	return sc.Supi, sc.PDUSessionID, sc.Ref, true
}
