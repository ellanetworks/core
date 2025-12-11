package gmm

import (
	ctxt "context"

	"github.com/ellanetworks/core/internal/amf/context"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/free5gc/nas/nasMessage"
	"go.uber.org/zap"
)

func contextSetup(ctx ctxt.Context, ue *context.AmfUe, msg *nasMessage.RegistrationRequest) error {
	ctx, span := tracer.Start(ctx, "contextSetup")
	defer span.End()

	ue.RegistrationRequest = msg

	switch ue.RegistrationType5GS {
	case nasMessage.RegistrationType5GSInitialRegistration:
		if err := HandleInitialRegistration(ctx, ue); err != nil {
			logger.AmfLog.Error("Error handling initial registration", zap.Error(err))
		}
	case nasMessage.RegistrationType5GSMobilityRegistrationUpdating:
		fallthrough
	case nasMessage.RegistrationType5GSPeriodicRegistrationUpdating:
		if err := HandleMobilityAndPeriodicRegistrationUpdating(ctx, ue); err != nil {
			logger.AmfLog.Error("Error handling mobility and periodic registration updating", zap.Error(err))
		}
	}

	return nil
}
