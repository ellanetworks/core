package pdusession

import (
	"context"
	"fmt"

	"github.com/ellanetworks/core/internal/logger"
	smfContext "github.com/ellanetworks/core/internal/smf/context"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

func ReleaseSmContext(ctx context.Context, smContextRef string) error {
	ctx, span := tracer.Start(
		ctx,
		"SMF Release SmContext",
		trace.WithAttributes(
			attribute.String("smf.smContextRef", smContextRef),
		),
	)
	defer span.End()

	smf := smfContext.SMFSelf()

	smContext := smf.GetSMContext(smContextRef)
	if smContext == nil {
		return fmt.Errorf("sm context not found: %s", smContextRef)
	}

	smContext.Mutex.Lock()
	defer smContext.Mutex.Unlock()

	err := smf.ReleaseUeIPAddr(ctx, smContext.Supi)
	if err != nil {
		logger.SmfLog.Error("release UE IP address failed", zap.Error(err), zap.String("supi", smContext.Supi), zap.Uint8("pduSessionID", smContext.PDUSessionID))
	}

	err = releaseTunnel(ctx, smf, smContext)
	if err != nil {
		smf.RemoveSMContext(ctx, smContext.CanonicalName())
		return fmt.Errorf("release tunnel failed: %v", err)
	}

	smf.RemoveSMContext(ctx, smContext.CanonicalName())

	return nil
}
