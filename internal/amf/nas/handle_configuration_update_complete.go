// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/logger"
	"go.uber.org/zap"
)

func handleConfigurationUpdateComplete(amfInstance *amf.AMF, ue *amf.UeContext) {
	if state := ue.State(); state != amf.Registered {
		logger.AmfLog.Warn("state mismatch: receive Configuration Update Complete message", zap.String("state", string(state)))
		return
	}

	if conn := ue.Conn(); conn != nil {
		conn.StopNASGuard()
	}

	amfInstance.CommitGUTIRealloc(ue)
}
