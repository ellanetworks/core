// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"context"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/nasreply"
	"go.uber.org/zap"
)

func handleConfigurationUpdateComplete(amfInstance *amf.AMF, ue *amf.UeContext) nasreply.Disposition {
	if state := ue.State(); state != amf.Registered {
		logger.AmfLog.Warn("state mismatch: receive Configuration Update Complete message", zap.String("state", string(state)))
		return nasreply.Silent(nasreply.ReasonOutOfState)
	}

	conn := ue.Conn()
	if conn != nil {
		conn.StopNASGuard()
	}

	amfInstance.CommitGUTIRealloc(ue)

	if req := ue.PagingPending().Request(); req != nil && req.Standalone() {
		ue.PagingDelivered()

		if err := amf.DeliverStandaloneN1N2(context.Background(), ue, conn, req); err != nil {
			logger.AmfLog.Warn("failed to deliver buffered standalone N1N2 message", zap.Error(err))
		}
	}

	return nasreply.Handled()
}
