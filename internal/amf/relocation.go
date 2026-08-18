// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package amf

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/amf/procedure"
	"github.com/ellanetworks/core/internal/interworking"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/nas/fgs"
	"github.com/ellanetworks/core/ngap"
	"go.uber.org/zap"
)

var (
	ErrNoEPSPeer          = errors.New("amf: N26 interworking is not wired")
	ErrRelocationRefused  = errors.New("amf: another procedure holds the UE's key chain")
	ErrNoRelocatingUe     = errors.New("amf: no handover to EPS is in progress for this subscriber")
	ErrRelocationNotToEPS = errors.New("amf: the UE's handover is not a move to EPS")
	ErrNoUEIdentity       = errors.New("amf: the UE has no IMSI to hand over under")
)

type RelocationPreparation struct {
	Request   interworking.ForwardRelocationRequest
	Container fgs.N1ModeToS1ModeNASTransparentContainer
}

func (a *AMF) PrepareHandoverToEPS(ue *UeContext, sourceUe *UeConn, target interworking.ENBIdentity, sourceToTarget []byte, requested []uint8, cause *ngap.Cause) (*RelocationPreparation, error) {
	if a.EPS == nil {
		return nil, ErrNoEPSPeer
	}

	if ue == nil {
		return nil, fmt.Errorf("amf: handover to EPS without a UE context")
	}

	if ue.Supi().IMSI() == "" {
		return nil, ErrNoUEIdentity
	}

	if !ue.EPSInterworkingAllowed() {
		return nil, ErrEPSMobilityBarred
	}

	candidates := make([]HandoverCandidate, 0, len(requested))
	for _, pduSessionID := range requested {
		candidates = append(candidates, HandoverCandidate{PDUSessionID: ngap.PDUSessionID(pduSessionID)})
	}

	id, ok := a.stageRelocationToEPS(ue, sourceUe, candidates)
	if !ok {
		return nil, ErrRelocationRefused
	}

	req, mapped, err := ue.BuildForwardRelocationRequest(target, sourceToTarget, requested, cause)
	if err != nil {
		a.ClearHandover(ue)

		return nil, err
	}

	req.ID = id

	return &RelocationPreparation{Request: req, Container: mapped.Container}, nil
}

func (a *AMF) RequestRelocationToEPS(ctx context.Context, req interworking.ForwardRelocationRequest) (interworking.ForwardRelocationResponse, error) {
	if a.EPS == nil {
		return interworking.ForwardRelocationResponse{}, ErrNoEPSPeer
	}

	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), a.handoverGuardTimeout)
	defer cancel()

	return a.EPS.ForwardRelocation(ctx, req)
}

func (a *AMF) AbandonHandoverToEPS(ctx context.Context, ue *UeContext, id interworking.RelocationID) bool {
	if !a.ClearRelocationToEPS(ue, id) {
		logger.From(ctx, logger.AmfLog).Info("handover to EPS already unwound by whoever cancelled it",
			zap.Uint64("relocation", uint64(id)))

		return false
	}

	if err := a.CancelRelocationToEPS(ctx, ue, id); err != nil {
		logger.From(ctx, logger.AmfLog).Info("the EPS peer had no handover to cancel", zap.Error(err))
	}

	return true
}

func (a *AMF) CancelRelocationToEPS(ctx context.Context, ue *UeContext, id interworking.RelocationID) error {
	if a.EPS == nil || ue == nil {
		return nil
	}

	return a.EPS.RelocationCancel(ctx, ue.Supi(), id)
}

func (a *AMF) RelocationComplete(ctx context.Context, supi etsi.SUPI, id interworking.RelocationID) error {
	ue, ok := a.LookupUeBySupi(supi)
	if !ok {
		return fmt.Errorf("%w: %s", ErrNoRelocatingUe, supi)
	}

	source := a.HandoverSource(ue)
	if source == nil {
		source = ue.Conn()
	}

	if !a.ClearRelocationToEPS(ue, id) {
		logger.From(ctx, logger.AmfLog).Info("the UE reached EPS on a handover this AMF no longer tracks; releasing anyway",
			logger.SUPI(supi.String()), zap.Uint64("relocation", uint64(id)))
	}

	a.releaseToEPS(ctx, ue)

	logger.From(ctx, logger.AmfLog).Info("UE handed over to EPS; releasing its 5GS resources and keeping its 5G security context for a return",
		logger.SUPI(supi.String()))

	if source == nil {
		return nil
	}

	source.ReleaseAction = UeContextReleaseHandover
	source.SendUEContextReleaseCommand(ctx, ngap.Cause{Group: ngap.CauseGroupRadioNetwork, Value: ngap.CauseRadioNetworkSuccessfulHandover})

	return nil
}

func (a *AMF) SuperviseHandoverToEPS(ue *UeContext, id interworking.RelocationID) {
	if ue == nil {
		return
	}

	ue.SuperviseKeyChainProc(procedure.N2Handover,
		time.Now().Add(a.handoverGuardTimeout),
		func(cctx context.Context) (procedure.Disposition, error) {
			switch a.unwindHandover(ue) {
			case handoverCommitting:
				return procedure.Retain, nil
			case handoverNotInProgress:
				return procedure.Release, nil
			}

			logger.From(cctx, logger.AmfLog).Warn("handover to EPS abandoned: the UE did not arrive in time",
				zap.String("imsi", ue.Supi().IMSI()))

			if err := a.CancelRelocationToEPS(cctx, ue, id); err != nil {
				logger.From(cctx, logger.AmfLog).Info("the peer had no handover to cancel", zap.Error(err))
			}

			return procedure.Release, nil
		})
}
