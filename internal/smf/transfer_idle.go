// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package smf

import (
	"context"
	"fmt"

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

func (s *SMF) TransferIdleTo5GS(ctx context.Context, supi etsi.SUPI, pduSessionID, ebi uint8, dnn string, snssai *models.Snssai) (string, error) {
	return s.TransferIdle(ctx, supi, pduSessionID, ebi, dnn, snssai, Access5G)
}

func (s *SMF) TransferIdleToEPS(ctx context.Context, supi etsi.SUPI, pduSessionID, ebi uint8, dnn string, snssai *models.Snssai) (models.EPSBearer, error) {
	ref, err := s.TransferIdle(ctx, supi, pduSessionID, ebi, dnn, snssai, Access4G)
	if err != nil {
		return models.EPSBearer{}, err
	}

	sc := s.GetSession(ref)
	if sc == nil {
		return models.EPSBearer{}, fmt.Errorf("%w: session %q left the pool as it moved", ErrSessionNotMovable, ref)
	}

	return epsBearerForSession(sc, sessionDNS(sc))
}

func (s *SMF) TransferIdle(ctx context.Context, supi etsi.SUPI, pduSessionID, ebi uint8, dnn string, snssai *models.Snssai, access AccessType) (string, error) {
	ctx, span := tracer.Start(ctx, "smf/transfer_idle",
		trace.WithAttributes(
			attribute.String("ue.supi", supi.String()),
			attribute.Int("smf.pdu_session_id", int(pduSessionID)),
			attribute.Int("eps.bearer_id", int(ebi)),
			attribute.String("smf.dnn", dnn),
			attribute.String("smf.access", access.String()),
		),
	)
	defer span.End()

	policy, err := s.GetSessionPolicy(ctx, supi, snssai, dnn)
	if err != nil {
		return "", fmt.Errorf("no policy for a session moving to %s in idle mode: %w", access, err)
	}

	move := transferRequest{Access: access, EBI: ebi, Dnn: dnn, Snssai: snssai, Policy: policy}

	sc, err := s.findTransferable(supi, pduSessionID, move)
	if err != nil {
		return "", fmt.Errorf("no session to move to %s in idle mode: %w", access, err)
	}

	if err := s.prepareTransfer(sc, move); err != nil {
		return "", fmt.Errorf("failed to prepare a session move to %s in idle mode: %w", access, err)
	}

	dropped, err := s.commitIdleTransfer(ctx, sc, access)
	if err != nil {
		sc.abandonTransferTo(access)

		return "", err
	}

	logger.WithTrace(ctx, logger.SmfLog).Info("moved a session between systems in idle mode",
		logger.SUPI(supi.String()), logger.PDUSessionID(pduSessionID),
		zap.Uint8("ebi", ebi), zap.String("dnn", dnn), zap.Stringer("to", access))

	s.dropSourceRouting(ctx, sc.Ref, dropped)

	return sc.Ref, nil
}

// The UE is idle on both sides of the move, so the downlink buffers and pages on
// the target access; no access-network endpoint is bound until it reactivates.
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
	next.Downlink = DownlinkBuffering
	// The endpoint belonged to the access the UE has left; the target's arrives
	// when it reactivates.
	next.AN = AnchorBinding{}
	next.QFI, next.AMBR = commit.policy.QosData.QFI, commit.policy.Ambr

	if err := s.applyDataPlane(ctx, sc, next, commit.policy.PolicyID); err != nil {
		commit.restore()

		return nil, err
	}

	return sc.finishTransferCommit(commit), nil
}
