// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/nasreply"
	"github.com/ellanetworks/core/nas/eps"
)

// handleEMMStatus logs an inbound EMM STATUS; per TS 24.301 §5.7 no state
// transition and no radio-interface action is taken.
func handleEMMStatus(msg *eps.EMMStatus) nasreply.Disposition {
	logger.MmeLog.Error("received EMM STATUS", logger.Cause(msg.Cause.String()))

	return nasreply.Handled()
}
