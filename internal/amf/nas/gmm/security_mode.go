package gmm

import (
	"context"
	"fmt"

	amfContext "github.com/ellanetworks/core/internal/amf/context"
	"github.com/ellanetworks/core/internal/amf/nas/gmm/message"
	"github.com/ellanetworks/core/internal/logger"
	"go.uber.org/zap"
)

func securityMode(ctx context.Context, ue *amfContext.AmfUe) error {
	logger.AmfLog.Debug("Security Mode Procedure", zap.String("supi", ue.Supi))

	ctx, span := tracer.Start(ctx, "securityMode")
	defer span.End()

	ue.Log = ue.Log.With(zap.String("supi", ue.Supi))

	if ue.SecurityContextIsValid() {
		ue.Log.Debug("UE has a valid security context - skip security mode control procedure")
		ue.State = amfContext.ContextSetup
		return contextSetup(ctx, ue, ue.RegistrationRequest)
	}

	amfSelf := amfContext.AMFSelf()
	ue.SelectSecurityAlg(amfSelf.SecurityAlgorithm.IntegrityOrder, amfSelf.SecurityAlgorithm.CipheringOrder)

	err := ue.DerivateAlgKey()
	if err != nil {
		return fmt.Errorf("error deriving algorithm key: %v", err)
	}

	err = message.SendSecurityModeCommand(ctx, ue.RanUe)
	if err != nil {
		return fmt.Errorf("error sending security mode command: %v", err)
	}

	return nil
}
