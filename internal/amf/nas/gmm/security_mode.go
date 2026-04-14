package gmm

import (
	"context"
	"fmt"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/amf/nas/gmm/message"
	"github.com/ellanetworks/core/internal/amf/procedure"
	"github.com/ellanetworks/core/internal/logger"
)

func securityMode(ctx context.Context, amfInstance *amf.AMF, ue *amf.AmfUe) error {
	logger.WithTrace(ctx, logger.AmfLog).Debug("Security Mode Procedure", logger.SUPI(ue.Supi.String()))

	ctx, span := tracer.Start(ctx, "nas/security_mode")
	defer span.End()

	ue.TransitionTo(amf.SecurityMode)

	ue.Log = ue.Log.With(logger.SUPI(ue.Supi.String()))

	if ue.SecurityContextIsValid() {
		ue.Log.Debug("UE has a valid security context - skip security mode control procedure")
		return contextSetup(ctx, amfInstance, ue, ue.RegistrationRequest)
	}

	integrityOrder, cipheringOrder, err := amfInstance.GetSecurityAlgorithms(ctx)
	if err != nil {
		return fmt.Errorf("error getting security algorithms: %v", err)
	}

	err = ue.SelectSecurityAlg(integrityOrder, cipheringOrder)
	if err != nil {
		return fmt.Errorf("NAS security algorithm negotiation failed: %v", err)
	}

	err = ue.DerivateAlgKey()
	if err != nil {
		return fmt.Errorf("error deriving algorithm key: %v", err)
	}

	ranUe := ue.RanUe()
	if ranUe == nil {
		return fmt.Errorf("ue is not connected to RAN")
	}

	if _, beginErr := ue.Procedures.Begin(ctx, procedure.Procedure{Type: procedure.SecurityMode}); beginErr != nil {
		return fmt.Errorf("security mode blocked by conflict: %w", beginErr)
	}

	err = message.SendSecurityModeCommand(ctx, amfInstance, ranUe)
	if err != nil {
		ue.Procedures.End(procedure.SecurityMode)

		return fmt.Errorf("error sending security mode command: %v", err)
	}

	return nil
}
