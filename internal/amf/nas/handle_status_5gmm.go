// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"context"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/nasreply"
	"github.com/ellanetworks/core/nas/fgs"
	"github.com/free5gc/nas/nasMessage"
	"go.uber.org/zap"
)

func handleStatus5GMM(ctx context.Context, ue *amf.UeContext, plainBody []byte) nasreply.Disposition {
	if ue.State() == amf.Deregistered {
		logger.From(ctx, logger.AmfLog).Warn("UE is in amf.Deregistered state, ignore Status 5GMM message")
		return nasreply.Silent(nasreply.ReasonOutOfState)
	}

	st, err := fgs.ParseStatus5GMM(plainBody)
	if err != nil {
		logger.From(ctx, logger.AmfLog).Warn("could not decode 5GMM STATUS", zap.Error(err))
		return nasreply.Silent(nasreply.ReasonUnspecified)
	}

	logger.From(ctx, logger.AmfLog).Error("Received Status 5GMM with cause", logger.Cause(nasMessage.Cause5GMMToString(st.Cause)))

	return nasreply.Handled()
}
