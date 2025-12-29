package pdusession

import (
	"context"
	"fmt"

	"github.com/ellanetworks/core/internal/logger"
	smfContext "github.com/ellanetworks/core/internal/smf/context"
	"go.opentelemetry.io/otel/attribute"
	"go.uber.org/zap"
)

func ReleaseSmContext(ctx context.Context, smContextRef string) error {
	ctx, span := tracer.Start(ctx, "SMF Release SmContext")
	defer span.End()
	span.SetAttributes(
		attribute.String("smf.smContextRef", smContextRef),
	)

	smContext := smfContext.GetSMContext(smContextRef)
	if smContext == nil {
		return fmt.Errorf("sm context not found: %s", smContextRef)
	}

	smContext.Mutex.Lock()
	defer smContext.Mutex.Unlock()

	err := smfContext.ReleaseUeIPAddr(ctx, smContext.Supi)
	if err != nil {
		logger.SmfLog.Error("release UE IP address failed", zap.Error(err), zap.String("supi", smContext.Supi), zap.Uint8("pduSessionID", smContext.PDUSessionID))
	}

	err = releaseTunnel(ctx, smContext)
	if err != nil {
		smfContext.RemoveSMContext(ctx, smfContext.CanonicalName(smContext.Supi, smContext.PDUSessionID))
		return fmt.Errorf("release tunnel failed: %v", err)
	}

	smfContext.RemoveSMContext(ctx, smfContext.CanonicalName(smContext.Supi, smContext.PDUSessionID))

	return nil
}
