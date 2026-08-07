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
	supi   etsi.SUPI
	access AccessType
	id     SessionIdentity
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

func (sc *SMContext) hasUserPlane() bool {
	return sc.Tunnel != nil && sc.PFCPContext != nil && sc.PFCPContext.Established
}

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

	if req.EBI != 0 {
		if err := s.epsBearerIdentityAvailable(sc, req.EBI); err != nil {
			return err
		}
	}

	if req.Access == Access5G {
		sc.discardOutstandingProcedures()
	}

	sc.pending = &pendingTransfer{to: req.Access, ebi: req.EBI, policy: req.Policy}
	committed = true

	return nil
}

func (sc *SMContext) abandonTransfer() {
	sc.Mutex.Lock()
	sc.pending = nil
	sc.Mutex.Unlock()

	sc.procedures.End(procedure.Transfer)
}

type transferCommit struct {
	source   AccessType
	sourceID SessionIdentity
	policy   *Policy
	qers     []*QER
	restore  func()
}

func (s *SMF) beginTransferCommit(ctx context.Context, sc *SMContext, access AccessType) (*transferCommit, error) {
	if sc.pending == nil || sc.pending.to != access {
		return nil, nil
	}

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

func (sc *SMContext) finishTransferCommit(c *transferCommit) *droppedSource {
	sc.PolicyData = c.policy
	sc.pending = nil

	if sc.Access == Access4G {
		sc.discardOutstandingProcedures()
	}

	sc.procedures.End(procedure.Transfer)

	return &droppedSource{supi: sc.Supi, access: c.source, id: c.sourceID}
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

func (sc *SMContext) discardOutstandingProcedures() {
	sc.stopProcedureTimer()
	sc.pendingPolicy = nil
	sc.establishmentPTI = 0
	sc.outstandingPTIs = nil
}
