// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"context"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/nasreply"
	"github.com/free5gc/nas/nasMessage"
)

func handleStatus5GMM(ctx context.Context, ue *amf.UeContext, msg *nasMessage.Status5GMM) nasreply.Disposition {
	if ue.State() == amf.Deregistered {
		logger.From(ctx, logger.AmfLog).Warn("UE is in amf.Deregistered state, ignore Status 5GMM message")
		return nasreply.Silent(nasreply.ReasonOutOfState)
	}

	logger.From(ctx, logger.AmfLog).Error("Received Status 5GMM with cause", logger.Cause(nasMessage.Cause5GMMToString(msg.GetCauseValue())))

	return nasreply.Handled()
}
