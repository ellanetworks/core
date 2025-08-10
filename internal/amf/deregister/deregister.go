package deregister

import (
	ctxt "context"
	"fmt"

	"github.com/ellanetworks/core/internal/amf/context"
	"github.com/ellanetworks/core/internal/amf/gmm"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/util/fsm"
	"go.uber.org/zap"
)

func DeregisterSubscriber(ctx ctxt.Context, supi string) error {
	amfSelf := context.AMFSelf()

	ue, ok := amfSelf.AmfUeFindBySupi(supi)
	if !ok {
		logger.AmfLog.Debug("UE with SUPI %s not found", zap.String("supi", supi))
		return nil
	}

	ueFsmState := ue.State[models.AccessType3GPPAccess].Current()
	switch ueFsmState {
	case context.Deregistered:
		logger.AmfLog.Info("Removing the UE", zap.String("supi", ue.Supi))
		ue.Remove()
	case context.Registered:
		logger.AmfLog.Info("Deregistration triggered for the UE", zap.String("supi", ue.Supi))
		err := gmm.GmmFSM.SendEvent(ctx, ue.State[models.AccessType3GPPAccess], gmm.NwInitiatedDeregistrationEvent, fsm.ArgsType{
			gmm.ArgAmfUe:      ue,
			gmm.ArgAccessType: models.AccessType3GPPAccess,
		})
		if err != nil {
			return fmt.Errorf("failed to send deregistration event: %w", err)
		}
	}

	return nil
}
