package pdusession

import (
	ctxt "context"
	"fmt"

	"github.com/ellanetworks/core/internal/smf/context"
	"go.opentelemetry.io/otel/attribute"
)

func UpdateSmContextN2InfoPduResRelRsp(ctx ctxt.Context, smContextRef string) error {
	ctx, span := tracer.Start(ctx, "SMF Update SmContext PDU Resource Release Response")
	defer span.End()
	span.SetAttributes(
		attribute.String("smf.smContextRef", smContextRef),
	)

	if smContextRef == "" {
		return fmt.Errorf("SM Context reference is missing")
	}

	smContext := context.GetSMContext(smContextRef)
	if smContext == nil {
		return fmt.Errorf("sm context not found: %s", smContextRef)
	}

	smContext.Mutex.Lock()
	defer smContext.Mutex.Unlock()

	if !smContext.PDUSessionReleaseDueToDupPduID {
		return nil
	}

	smContext.PDUSessionReleaseDueToDupPduID = false

	context.RemoveSMContext(ctx, context.CanonicalName(smContext.Supi, smContext.PDUSessionID))

	return nil
}
