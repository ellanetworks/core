package pdusession

import (
	"context"
	"fmt"

	smfContext "github.com/ellanetworks/core/internal/smf/context"
	"go.opentelemetry.io/otel/attribute"
)

func UpdateSmContextN2InfoPduResRelRsp(ctx context.Context, smContextRef string) error {
	ctx, span := tracer.Start(ctx, "SMF Update SmContext PDU Resource Release Response")
	defer span.End()
	span.SetAttributes(
		attribute.String("smf.smContextRef", smContextRef),
	)

	if smContextRef == "" {
		return fmt.Errorf("SM Context reference is missing")
	}

	smf := smfContext.SMFSelf()

	smContext := smf.GetSMContext(smContextRef)
	if smContext == nil {
		return fmt.Errorf("sm context not found: %s", smContextRef)
	}

	smContext.Mutex.Lock()
	defer smContext.Mutex.Unlock()

	if !smContext.PDUSessionReleaseDueToDupPduID {
		return nil
	}

	smContext.PDUSessionReleaseDueToDupPduID = false

	smf.RemoveSMContext(ctx, smfContext.CanonicalName(smContext.Supi, smContext.PDUSessionID))

	return nil
}
