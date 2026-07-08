// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"context"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/free5gc/nas/nasConvert"
	"github.com/free5gc/nas/nasMessage"
	"go.uber.org/zap"
)

// TS 24501 5.6.3.2
func handleNotificationResponse(ctx context.Context, amfInstance *amf.AMF, ue *amf.UeContext, msg *nasMessage.NotificationResponse) {
	if state := ue.State(); state != amf.Registered {
		logger.From(ctx, logger.AmfLog).Warn("state mismatch: receive Notification Response message", zap.String("state", string(state)))
		return
	}

	if conn := ue.Conn(); conn != nil {
		conn.StopNASGuard()
	}

	if msg.PDUSessionStatus == nil {
		logger.WithTrace(ctx, logger.AmfLog).Debug("PDUSessionStatus IE is not present in Notification Response message, no PDU session to release", logger.SUPI(ue.Supi().String()))
		return
	}

	psiArray := nasConvert.PSIToBooleanArray(msg.Buffer)

	for psi := 1; psi <= 15; psi++ {
		pduSessionID := uint8(psi)
		if smContext, ok := ue.SmContextFindByPDUSessionID(pduSessionID); ok {
			if !psiArray[psi] {
				err := amfInstance.Session.ReleaseSmContext(ctx, smContext.Ref)
				if err != nil {
					logger.From(ctx, logger.AmfLog).Warn("failed to release sm context", zap.Error(err))
					return
				}
			}
		}
	}
}
