package pdusession

import (
	"context"
	"fmt"

	smfContext "github.com/ellanetworks/core/internal/smf/context"
	"github.com/ellanetworks/core/internal/smf/ngap"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

func UpdateSmContextCauseDuplicatePDUSessionID(ctx context.Context, smContextRef string) ([]byte, error) {
	ctx, span := tracer.Start(
		ctx,
		"SMF Update SmContext Cause Duplicate PDU Session ID",
		trace.WithAttributes(
			attribute.String("smf.smContextRef", smContextRef),
		),
	)
	defer span.End()

	if smContextRef == "" {
		return nil, fmt.Errorf("SM Context reference is missing")
	}

	smf := smfContext.SMFSelf()

	smContext := smf.GetSMContext(smContextRef)
	if smContext == nil {
		return nil, fmt.Errorf("sm context not found: %s", smContextRef)
	}

	smContext.Mutex.Lock()
	defer smContext.Mutex.Unlock()

	smContext.PDUSessionReleaseDueToDupPduID = true

	n2Rsp, err := ngap.BuildPDUSessionResourceReleaseCommandTransfer()
	if err != nil {
		return nil, fmt.Errorf("build PDUSession Resource Release Command Transfer Error: %v", err)
	}

	err = releaseTunnel(ctx, smf, smContext)
	if err != nil {
		return nil, fmt.Errorf("failed to release tunnel: %v", err)
	}

	return n2Rsp, nil
}
