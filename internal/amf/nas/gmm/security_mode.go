package gmm

import (
	"context"
	"fmt"

	amfContext "github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/amf/nas/gmm/message"
	"github.com/ellanetworks/core/internal/logger"
)

func securityMode(ctx context.Context, amf *amfContext.AMF, ue *amfContext.AmfUe) error {
	logger.WithTrace(ctx, logger.AmfLog).Debug("Security Mode Procedure", logger.SUPI(ue.Supi.String()))

	ctx, span := tracer.Start(ctx, "nas/security_mode")
	defer span.End()

	ue.TransitionTo(amfContext.SecurityMode)

	ue.Log = ue.Log.With(logger.SUPI(ue.Supi.String()))

	if ue.SecurityContextIsValid() {
		ue.Log.Debug("UE has a valid security context - skip security mode control procedure")
		return contextSetup(ctx, amf, ue, ue.RegistrationRequest)
	}

	integrityOrder, cipheringOrder, err := amf.GetSecurityAlgorithms(ctx)
	if err != nil {
		return fmt.Errorf("error getting security algorithms: %v", err)
	}

	ue.SelectSecurityAlg(integrityOrder, cipheringOrder)

	err = ue.DerivateAlgKey()
	if err != nil {
		return fmt.Errorf("error deriving algorithm key: %v", err)
	}

	err = message.SendSecurityModeCommand(ctx, amf, ue.RanUe)
	if err != nil {
		return fmt.Errorf("error sending security mode command: %v", err)
	}

	return nil
}
