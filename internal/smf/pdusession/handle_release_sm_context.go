package pdusession

import (
	ctxt "context"
	"fmt"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/smf/context"
	"go.opentelemetry.io/otel/attribute"
	"go.uber.org/zap"
)

func ReleaseSmContext(ctx ctxt.Context, smContextRef string) error {
	ctx, span := tracer.Start(ctx, "SMF Release SmContext")
	defer span.End()
	span.SetAttributes(
		attribute.String("smf.smContextRef", smContextRef),
	)

	smContext := context.GetSMContext(smContextRef)
	if smContext == nil {
		return fmt.Errorf("sm context not found: %s", smContextRef)
	}

	smContext.Mutex.Lock()
	defer smContext.Mutex.Unlock()

	err := context.ReleaseUeIPAddr(ctx, smContext.Supi)
	if err != nil {
		logger.SmfLog.Error("release UE IP address failed", zap.Error(err), zap.String("supi", smContext.Supi), zap.Uint8("pduSessionID", smContext.PDUSessionID))
	}

	err = releaseTunnel(ctx, smContext)
	if err != nil {
		context.RemoveSMContext(ctx, context.CanonicalName(smContext.Supi, smContext.PDUSessionID))
		return fmt.Errorf("release tunnel failed: %v", err)
	}

	context.RemoveSMContext(ctx, context.CanonicalName(smContext.Supi, smContext.PDUSessionID))

	return nil
}
