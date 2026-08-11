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

func (a *AMF) PrepareHandoverToEPS(ue *UeContext, sourceUe *UeConn, target interworking.ENBIdentity, sourceToTarget []byte, requested []uint8) (*RelocationPreparation, error) {
	if a.EPS == nil {
		return nil, ErrNoEPSPeer
	}

	if ue == nil {
		return nil, fmt.Errorf("amf: handover to EPS without a UE context")
	}

	if ue.Supi().IMSI() == "" {
		return nil, ErrNoUEIdentity
	}

	candidates := make([]HandoverCandidate, 0, len(requested))
	for _, pduSessionID := range requested {
		candidates = append(candidates, HandoverCandidate{PDUSessionID: ngap.PDUSessionID(pduSessionID)})
	}

	if !a.stageRelocationToEPS(ue, sourceUe, candidates) {
		return nil, ErrRelocationRefused
	}

	req, mapped, err := ue.BuildForwardRelocationRequest(target, sourceToTarget, requested)
	if err != nil {
		a.ClearHandover(ue)

		return nil, err
	}

	return &RelocationPreparation{Request: req, Container: mapped.Container}, nil
}

func (a *AMF) ForwardRelocation(ctx context.Context, req interworking.ForwardRelocationRequest) (interworking.ForwardRelocationResponse, error) {
	if a.EPS == nil {
		return interworking.ForwardRelocationResponse{}, ErrNoEPSPeer
	}

	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), a.handoverGuardTimeout)
	defer cancel()

	return a.EPS.ForwardRelocation(ctx, req)
}

func (a *AMF) AbandonHandoverToEPS(ctx context.Context, ue *UeContext) {
	a.ClearHandover(ue)
	a.CancelRelocationToEPS(ctx, ue)
}

func (a *AMF) CancelRelocationToEPS(ctx context.Context, ue *UeContext) {
	if a.EPS == nil || ue == nil {
		return
	}

	imsi := ue.Supi().IMSI()
	if imsi == "" {
		return
	}

	if err := a.EPS.RelocationCancel(ctx, imsi); err != nil {
		logger.From(ctx, logger.AmfLog).Info("the EPS peer had no handover to cancel",
			zap.String("imsi", imsi), zap.Error(err))
	}
}

func (a *AMF) RelocationComplete(ctx context.Context, imsi string) error {
	supi, err := etsi.NewSUPIFromIMSI(imsi)
	if err != nil {
		return fmt.Errorf("amf: relocation complete for %q: %w", imsi, err)
	}

	ue, ok := a.LookupUeBySupi(supi)
	if !ok {
		return fmt.Errorf("%w: %s", ErrNoRelocatingUe, imsi)
	}

	if !a.HandoverToEPSInProgress(ue) {
		return fmt.Errorf("%w: %s", ErrRelocationNotToEPS, imsi)
	}

	source := a.HandoverSource(ue)

	a.ClearHandover(ue)

	ue.releaseSmContexts(ctx)
	ue.ReleaseAllEPSBearerIdentities()

	logger.From(ctx, logger.AmfLog).Info("UE handed over to EPS; releasing its 5GS resources",
		zap.String("imsi", imsi))

	if source == nil {
		return nil
	}

	source.ReleaseAction = UeContextReleaseHandover
	source.SendUEContextReleaseCommand(ctx, ngap.Cause{Group: ngap.CauseGroupRadioNetwork, Value: ngap.CauseRadioNetworkSuccessfulHandover})

	return nil
}

func (a *AMF) SuperviseHandoverToEPS(ue *UeContext) {
	if ue == nil {
		return
	}

	ue.SuperviseKeyChainProc(procedure.N2Handover,
		time.Now().Add(a.handoverGuardTimeout),
		func(cctx context.Context) error {
			if !a.abandonHandover(ue) {
				return nil
			}

			logger.From(cctx, logger.AmfLog).Warn("handover to EPS abandoned: the UE did not arrive in time",
				zap.String("imsi", ue.Supi().IMSI()))

			a.CancelRelocationToEPS(cctx, ue)

			return nil
		})
}
