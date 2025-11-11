package deregister

import (
	ctxt "context"

	"github.com/ellanetworks/core/internal/amf/context"
	"github.com/ellanetworks/core/internal/logger"
	"go.uber.org/zap"
)

func DeregisterSubscriber(ctx ctxt.Context, supi string) error {
	amfSelf := context.AMFSelf()

	ue, ok := amfSelf.AmfUeFindBySupi(supi)
	if !ok {
		logger.AmfLog.Debug("UE with SUPI %s not found", zap.String("supi", supi))
		return nil
	}

	ue.Remove()

	return nil
}
