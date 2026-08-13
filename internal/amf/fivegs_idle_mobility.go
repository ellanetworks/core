// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package amf

import (
	"context"
	"fmt"

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/interworking"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/nas/fgs"
	"go.uber.org/zap"
)

func (a *AMF) EPSContext(ctx context.Context, req interworking.EPSContextRequest) (interworking.EPSContextResponse, error) {
	none := interworking.EPSContextResponse{}

	operatorInfo, err := a.OperatorInfo(ctx)
	if err != nil {
		return none, fmt.Errorf("amf: get operator info: %w", err)
	}

	presented, err := etsi.NewGUTI5GFromNAS(fgs.GUTIIdentity(req.Mapped5GGUTI))
	if err != nil {
		return none, fmt.Errorf("%w: %w", interworking.ErrUnknownUEContext, err)
	}

	ue, ok := a.LookupUeByGuti(operatorInfo.Guami, presented)
	if !ok {
		return none, fmt.Errorf("%w: no context for 5G-GUTI %s", interworking.ErrUnknownUEContext, presented.String())
	}

	if _, err := ue.VerifyEnclosedEPSNAS(req.EPSNAS); err != nil {
		return none, fmt.Errorf("%w: %w", interworking.ErrIntegrityCheckFailed, err)
	}

	security, err := ue.MapSecurityContextToEPSOnIdleMobility()
	if err != nil {
		return none, err
	}

	ambr := ue.Ambr
	if ambr == nil {
		return none, fmt.Errorf("amf: UE has no AMBR")
	}

	sessions := ue.AllTransferableEPSSessions()

	logger.From(ctx, logger.AmfLog).Info("handing the UE's context to EPS for an idle-mode change",
		logger.SUPI(ue.Supi().String()), zap.Int("pdu-sessions", len(sessions)))

	return interworking.EPSContextResponse{
		SUPI:           ue.Supi(),
		Security:       security,
		PDNConnections: sessions,
		AMBRUplink:     ambr.Uplink,
		AMBRDownlink:   ambr.Downlink,
	}, nil
}

func (a *AMF) EPSContextAck(ctx context.Context, supi etsi.SUPI, transferred []uint8) error {
	ue, ok := a.LookupUeBySupi(supi)
	if !ok {
		return fmt.Errorf("%w: %s", interworking.ErrUnknownUEContext, supi)
	}

	for _, pduSessionID := range transferred {
		ue.DeleteSmContext(pduSessionID)
	}

	ue.Deregister(ctx)

	logger.From(ctx, logger.AmfLog).Info("UE moved to EPS in idle mode; keeping its 5G security context for a return",
		logger.SUPI(supi.String()), zap.Int("adopted", len(transferred)))

	return nil
}
