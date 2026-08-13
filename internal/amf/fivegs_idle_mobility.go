// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package amf

import (
	"context"
	"fmt"
	"time"

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/interworking"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/nas/eps"
	"github.com/ellanetworks/core/nas/fgs"
	"go.uber.org/zap"
)

// epsContextRetention keeps the 5G context exportable for the UE's own tracking
// area updating budget — five T3430 expiries — so a repeated update still finds a
// context to map (TS 24.301 §5.5.3.2.7 d).
const epsContextRetention = 5 * 15 * time.Second

func (a *AMF) EPSContext(ctx context.Context, req interworking.EPSContextRequest) (interworking.EPSContextResponse, error) {
	none := interworking.EPSContextResponse{}

	presented, err := etsi.NewGUTI5GFromNAS(fgs.GUTIIdentity(req.Mapped5GGUTI))
	if err != nil {
		return none, fmt.Errorf("%w: %w", interworking.ErrUnknownUEContext, err)
	}

	operatorInfo, err := a.OperatorInfo(ctx)
	if err != nil {
		return none, fmt.Errorf("amf: get operator info: %w", err)
	}

	ue, ok := a.LookupUeByGuti(operatorInfo.Guami, presented)
	if !ok {
		return none, fmt.Errorf("%w: no context for 5G-GUTI %s", interworking.ErrUnknownUEContext, presented.String())
	}

	if !ue.ExportableToEPS() || !ue.Secured() {
		return none, fmt.Errorf("%w: the context for 5G-GUTI %s is not a registered, secured one",
			interworking.ErrUnknownUEContext, presented.String())
	}

	ambrUplink, ambrDownlink, ok := ue.AmbrRates()
	if !ok {
		return none, fmt.Errorf("amf: UE has no AMBR")
	}

	spm, err := eps.ParseSecurityProtectedMessage(req.EPSNAS)
	if err != nil {
		return none, fmt.Errorf("%w: %w", interworking.ErrIntegrityCheckFailed, err)
	}

	// TS 33.501 §8.5.2 steps 3-4
	if mt, err := eps.PeekMessageType(spm.UnverifiedPayload); err != nil || mt != eps.MsgTrackingAreaUpdateRequest {
		return none, fmt.Errorf("%w: the container holds no TRACKING AREA UPDATE REQUEST", interworking.ErrIntegrityCheckFailed)
	}

	if err := ue.CitesCurrentSecurityContext(spm.UnverifiedPayload); err != nil {
		return none, fmt.Errorf("%w: %w", interworking.ErrIntegrityCheckFailed, err)
	}

	if _, err := ue.VerifyEnclosedEPSNAS(req.EPSNAS); err != nil {
		return none, fmt.Errorf("%w: %w", interworking.ErrIntegrityCheckFailed, err)
	}

	security, err := ue.MapSecurityContextToEPSOnIdleMobility()
	if err != nil {
		return none, err
	}

	sessions := ue.AllTransferableEPSSessions()

	logger.From(ctx, logger.AmfLog).Info("handing the UE's context to EPS for an idle-mode change",
		logger.SUPI(ue.Supi().String()), zap.Int("pdu-sessions", len(sessions)))

	return interworking.EPSContextResponse{
		SUPI:           ue.Supi(),
		Security:       security,
		PDNConnections: sessions,
		AMBRUplink:     ambrUplink,
		AMBRDownlink:   ambrDownlink,
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

	ue.RetainForEPS(epsContextRetention)
	ue.Deregister(ctx)

	// Supervision is still running against the deadline set when the UE went idle on
	// NR, which can be moments away. Restart it from the transfer so the escalation
	// cannot remove the context this ack just chose to retain.
	a.StartMobileReachable(ue)

	logger.From(ctx, logger.AmfLog).Info("UE moved to EPS in idle mode; keeping its 5G security context for a return",
		logger.SUPI(supi.String()), zap.Int("adopted", len(transferred)))

	return nil
}
