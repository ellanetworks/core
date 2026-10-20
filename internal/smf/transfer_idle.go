// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package smf

import (
	"context"
	"fmt"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/tracing/attrs"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

func (s *SMF) TransferIdleTo5GS(ctx context.Context, req models.SessionTransferRequest) (string, []uint8, error) {
	ref, err := s.TransferIdle(ctx, req, Access5G)
	if err != nil {
		return "", nil, err
	}

	sc := s.GetSession(ref)
	if sc == nil {
		return "", nil, fmt.Errorf("%w: session %q left the pool as it moved", ErrSessionNotMovable, ref)
	}

	sc.Mutex.Lock()
	defer sc.Mutex.Unlock()

	return ref, sc.flowEBIsLocked(), nil
}

func (s *SMF) TransferIdleToEPS(ctx context.Context, req models.SessionTransferRequest) (models.EPSBearer, error) {
	ebi := req.EPSBearerIdentity

	ref, err := s.TransferIdle(ctx, req, Access4G)
	if err != nil {
		return models.EPSBearer{}, err
	}

	sc := s.GetSession(ref)
	if sc == nil {
		return models.EPSBearer{}, fmt.Errorf("%w: session %q left the pool as it moved", ErrSessionNotMovable, ref)
	}

	sc.Mutex.Lock()
	policy := sc.PolicyData
	sc.Mutex.Unlock()

	if policy == nil {
		return models.EPSBearer{}, fmt.Errorf("%w: session %q has no policy", ErrSessionNotMovable, ref)
	}

	return epsBearerForSession(sc, policy, ebi)
}

func (s *SMF) TransferIdle(ctx context.Context, req models.SessionTransferRequest, access AccessType) (string, error) {
	supi, pduSessionID, ebi, dnn, snssai, ue := req.SUPI, req.PDUSessionID, req.EPSBearerIdentity, req.DNN, req.Snssai, req.UE

	ctx, span := tracer.Start(ctx, "smf/transfer_idle",
		trace.WithAttributes(
			attrs.SUPI(supi.String()),
			attrs.PDUSessionID(pduSessionID),
			attribute.Int("eps.bearer_id", int(ebi)),
			attrs.DNN(dnn),
			attribute.String("smf.access", access.String()),
		),
	)
	defer span.End()

	move := transferRequest{Access: access, EBI: ebi, Dnn: dnn, Snssai: snssai, UE: ue}

	sc, err := s.findTransferable(supi, pduSessionID, move)
	if err != nil {
		return "", fmt.Errorf("no session to move to %s in idle mode: %w", access, err)
	}

	if err := s.checkSubscribed(ctx, supi, snssai, dnn); err != nil {
		return "", fmt.Errorf("no subscription for a session moving to %s in idle mode: %w", access, err)
	}

	if err := s.prepareTransfer(ctx, sc, move); err != nil {
		return "", fmt.Errorf("failed to prepare a session move to %s in idle mode: %w", access, err)
	}

	dropped, err := s.commitIdleTransfer(ctx, sc, access)
	if err != nil {
		s.abandonTransfer(ctx, sc, access)

		return "", err
	}

	logger.From(ctx, logger.SmfLog).Info("moved a session between systems in idle mode",
		logger.SUPI(supi.String()), logger.PDUSessionID(pduSessionID),
		zap.Uint8("ebi", ebi), logger.DNN(dnn), zap.Stringer("to", access))

	if dropped != nil {
		dropped.upActive = false
	}

	s.dropSourceRouting(ctx, sc.Ref, dropped)

	return sc.Ref, nil
}

func (s *SMF) commitIdleTransfer(ctx context.Context, sc *SMContext, access AccessType) (*droppedSource, error) {
	sc.Mutex.Lock()
	defer sc.Mutex.Unlock()

	commit, err := s.beginTransferCommit(ctx, sc, access)
	if err != nil {
		return nil, fmt.Errorf("failed to move session %q to %s: %w", sc.Ref, access, err)
	}

	if commit == nil {
		return nil, fmt.Errorf("%w: the move of session %q to %s was abandoned before it committed", ErrSessionNotMovable, sc.Ref, access)
	}

	next := sc.Tunnel.dataPlane
	next.Access = access
	next.TargetUplink = false
	next.Downlink = DownlinkBuffering
	next.AN = AnchorBinding{}
	next.QFI, next.AMBR, next.RatingGroup = sc.PolicyData.QosData.QFI, sc.PolicyData.Ambr, sc.PolicyData.RatingGroup

	if access == Access4G {
		s.interruptFlowProcedureLocked(ctx, sc, false)
	}

	flows := sc.movingFlowsLocked(access, false)
	next.Bearers = flows.legs

	if err := s.applyDataPlane(ctx, sc, next, sc.PolicyData.PolicyID); err != nil {
		commit.restore()

		return nil, err
	}

	s.adoptMovedFlowsLocked(ctx, sc, flows, access == Access4G)

	return sc.finishTransferCommit(commit), nil
}
