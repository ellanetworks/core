package deregister

import (
	"context"

	amfContext "github.com/ellanetworks/core/internal/amf/context"
	"github.com/ellanetworks/core/internal/logger"
	"go.uber.org/zap"
)

func DeregisterSubscriber(ctx context.Context, amf *amfContext.AMF, supi string) error {
	ue, ok := amf.FindAMFUEBySupi(supi)
	if !ok {
		logger.AmfLog.Debug("UE with SUPI not found", zap.String("supi", supi))
		return nil
	}

	amf.DeregisterAndRemoveAMFUE(ue)

	logger.AmfLog.Info("removed ue context", zap.String("supi", supi))

	return nil
}
