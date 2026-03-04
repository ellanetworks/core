package deregister

import (
	"context"

	"github.com/ellanetworks/core/etsi"
	amfContext "github.com/ellanetworks/core/internal/amf/context"
	"github.com/ellanetworks/core/internal/logger"
	"go.uber.org/zap"
)

func DeregisterSubscriber(ctx context.Context, amf *amfContext.AMF, supi etsi.SUPI) error {
	ue, ok := amf.FindAMFUEBySupi(supi)
	if !ok {
		logger.AmfLog.Debug("UE with SUPI not found", zap.String("supi", supi.String()))
		return nil
	}

	amf.DeregisterAndRemoveAMFUE(ue)

	logger.AmfLog.Info("removed ue context", zap.String("supi", supi.String()))

	return nil
}
