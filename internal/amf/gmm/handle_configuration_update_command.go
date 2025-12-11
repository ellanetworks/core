package gmm

import (
	ctxt "context"
	"fmt"

	"github.com/ellanetworks/core/internal/amf/context"
	"github.com/ellanetworks/core/internal/logger"
	"go.opentelemetry.io/otel/attribute"
	"go.uber.org/zap"
)

func handleConfigurationUpdateComplete(ctx ctxt.Context, ue *context.AmfUe) error {
	logger.AmfLog.Debug("Handle Configuration Update Complete", zap.String("supi", ue.Supi))

	_, span := tracer.Start(ctx, "AMF NAS HandleConfigurationUpdateComplete")
	span.SetAttributes(
		attribute.String("ue", ue.Supi),
		attribute.String("state", string(ue.State.Current())),
	)
	defer span.End()

	if ue.State.Current() != context.Registered {
		return fmt.Errorf("state mismatch: receive Configuration Update Complete message in state %s", ue.State.Current())
	}

	if ue.MacFailed {
		return fmt.Errorf("NAS message integrity check failed")
	}

	if ue.T3555 != nil {
		ue.T3555.Stop()
		ue.T3555 = nil // clear the timer
	}

	amfSelf := context.AMFSelf()
	amfSelf.FreeOldGuti(ue)

	return nil
}
