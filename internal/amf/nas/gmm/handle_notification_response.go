package gmm

import (
	"context"
	"fmt"

	amfContext "github.com/ellanetworks/core/internal/amf/context"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/smf/pdusession"
	"github.com/free5gc/nas/nasConvert"
	"github.com/free5gc/nas/nasMessage"
	"go.uber.org/zap"
)

// TS 24501 5.6.3.2
func handleNotificationResponse(ctx context.Context, ue *amfContext.AmfUe, msg *nasMessage.NotificationResponse) error {
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

	if msg.PDUSessionStatus == nil {
		logger.AmfLog.Debug("PDUSessionStatus IE is not present in Notification Response message, no PDU session to release", zap.String("supi", ue.Supi))
		return nil
	}

	psiArray := nasConvert.PSIToBooleanArray(msg.Buffer)

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

	return nil
}
