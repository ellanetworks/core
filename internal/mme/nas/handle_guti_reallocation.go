// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"context"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/mme"
	"github.com/ellanetworks/core/internal/nasreply"
)

// handleGUTIReallocationComplete finalises a standalone GUTI reallocation: it stops
// T3450 and commits the new GUTI, freeing the old one (TS 24.301 §5.4.1.4).
func handleGUTIReallocationComplete(ctx context.Context, m *mme.MME, ue *mme.UeContext, ueConn *mme.UeConn) nasreply.Disposition {
	if ue.EMMState() != mme.EMMRegistered {
		logger.From(ctx, logger.MmeLog).Warn("ignoring GUTI Reallocation Complete outside EMM-REGISTERED")
		return nasreply.Silent(nasreply.ReasonOutOfState)
	}

	ueConn.StopNASGuard()
	m.CommitGUTIRealloc(ue)

	return nasreply.Handled()
}
