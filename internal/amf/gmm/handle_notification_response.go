package gmm

import (
	ctxt "context"
	"fmt"

	"github.com/ellanetworks/core/internal/amf/context"
	"github.com/ellanetworks/core/internal/smf/pdusession"
	"github.com/free5gc/nas"
	"github.com/free5gc/nas/nasConvert"
	"go.opentelemetry.io/otel/attribute"
)

// TS 24501 5.6.3.2
func handleNotificationResponse(ctx ctxt.Context, ue *context.AmfUe, msg *nas.GmmMessage) error {
	_, span := tracer.Start(ctx, "AMF HandleNotificationResponse")
	span.SetAttributes(
		attribute.String("ue", ue.Supi),
		attribute.String("state", string(ue.State.Current())),
	)
	defer span.End()

	if ue.State.Current() != context.Registered {
		return fmt.Errorf("state mismatch: receive Notification Response message in state %s", ue.State.Current())
	}

	if ue.MacFailed {
		return fmt.Errorf("NAS message integrity check failed")
	}

	if ue.T3565 != nil {
		ue.T3565.Stop()
		ue.T3565 = nil // clear the timer
	}

	if msg.NotificationResponse != nil && msg.NotificationResponse.PDUSessionStatus != nil {
		psiArray := nasConvert.PSIToBooleanArray(msg.NotificationResponse.PDUSessionStatus.Buffer)
		for psi := 1; psi <= 15; psi++ {
			pduSessionID := int32(psi)
			if smContext, ok := ue.SmContextFindByPDUSessionID(pduSessionID); ok {
				if !psiArray[psi] {
					err := pdusession.ReleaseSmContext(ctx, smContext.SmContextRef())
					if err != nil {
						return fmt.Errorf("failed to release sm context: %s", err)
					}
				}
			}
		}
	}

	return nil
}
