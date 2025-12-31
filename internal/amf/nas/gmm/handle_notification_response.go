package gmm

import (
	"context"
	"fmt"

	amfContext "github.com/ellanetworks/core/internal/amf/context"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/smf/pdusession"
	"github.com/free5gc/nas"
	"github.com/free5gc/nas/nasConvert"
	"go.opentelemetry.io/otel/attribute"
	"go.uber.org/zap"
)

// TS 24501 5.6.3.2
func handleNotificationResponse(ctx context.Context, ue *amfContext.AmfUe, msg *nas.GmmMessage) error {
	logger.AmfLog.Debug("Handle Notification Response", zap.String("supi", ue.Supi))

	_, span := tracer.Start(ctx, "AMF NAS HandleNotificationResponse")

	span.SetAttributes(
		attribute.String("ue", ue.Supi),
		attribute.String("state", string(ue.State)),
	)
	defer span.End()

	if ue.State != amfContext.Registered {
		return fmt.Errorf("state mismatch: receive Notification Response message in state %s", ue.State)
	}

	if ue.MacFailed {
		return fmt.Errorf("NAS message integrity check failed")
	}

	if ue.T3565 != nil {
		ue.T3565.Stop()
		ue.T3565 = nil // clear the timer
	}

	if msg.NotificationResponse != nil && msg.NotificationResponse.PDUSessionStatus != nil {
		psiArray := nasConvert.PSIToBooleanArray(msg.NotificationResponse.Buffer)

		for psi := 1; psi <= 15; psi++ {
			pduSessionID := uint8(psi)
			if smContext, ok := ue.SmContextFindByPDUSessionID(pduSessionID); ok {
				if !psiArray[psi] {
					err := pdusession.ReleaseSmContext(ctx, smContext.Ref)
					if err != nil {
						return fmt.Errorf("failed to release sm context: %s", err)
					}
				}
			}
		}
	}

	return nil
}
