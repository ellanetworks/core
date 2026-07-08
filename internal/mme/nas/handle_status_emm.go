// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/nasreply"
	"github.com/ellanetworks/core/nas/eps"
	"go.uber.org/zap"
)

// handleEMMStatus logs an inbound EMM STATUS; per TS 24.301 §5.7 no state
// transition and no radio-interface action is taken.
func handleEMMStatus(plain []byte) nasreply.Disposition {
	msg, err := eps.ParseEMMStatus(plain)
	if err != nil {
		logger.MmeLog.Warn("failed to decode EMM STATUS", zap.Error(err))
		return nasreply.Handled()
	}

	logger.MmeLog.Error("received EMM STATUS", zap.Uint8("emm-cause", msg.EMMCause))

	return nasreply.Handled()
}
