package gmm

import (
	ctxt "context"
	"fmt"

	"github.com/ellanetworks/core/internal/amf/context"
	"go.opentelemetry.io/otel/attribute"
)

func handleConfigurationUpdateComplete(ctx ctxt.Context, ue *context.AmfUe) error {
	_, span := tracer.Start(ctx, "AMF HandleConfigurationUpdateComplete")
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
