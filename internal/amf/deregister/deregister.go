package deregister

import (
	ctxt "context"

	"github.com/ellanetworks/core/internal/amf/context"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/smf/pdusession"
	"go.uber.org/zap"
)

func DeregisterSubscriber(ctx ctxt.Context, supi string) error {
	amfSelf := context.AMFSelf()

	ue, ok := amfSelf.AmfUeFindBySupi(supi)
	if !ok {
		logger.AmfLog.Debug("UE with SUPI not found", zap.String("supi", supi))
		return nil
	}

	ue.Mutex.Lock()
	defer ue.Mutex.Unlock()

	for _, smContext := range ue.SmContextList {
		err := pdusession.ReleaseSmContext(ctx, smContext.SmContextRef())
		if err != nil {
			ue.Log.Debug("Error releasing SM context", zap.Error(err))
		} else {
			ue.Log.Debug("Released SM context", zap.String("smContextRef", smContext.SmContextRef()))
		}
	}

	ue.Remove()

	logger.AmfLog.Info("removed ue context", zap.String("supi", supi))

	return nil
}
