// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"context"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/mme"
	"github.com/ellanetworks/core/internal/nasreply"
	"github.com/ellanetworks/core/nas/eps"
	"go.uber.org/zap"
)

func handleSecurityModeReject(m *mme.MME, ctx context.Context, ue *mme.UeContext, plain []byte) nasreply.Disposition {
	// A SECURITY MODE REJECT is valid only during the security mode sub-phase; an
	// out-of-order one (admissible without integrity, TS 24.301 §4.4.4.3) must not
	// release the UE.
	if ue.RegStep() != mme.RegStepSecurityMode {
		logger.From(ctx, logger.MmeLog).Warn("ignoring Security Mode Reject outside the security mode sub-phase")

		return nasreply.Silent(nasreply.ReasonOutOfState)
	}

	ue.Conn().StopNASGuard()

	var cause uint8
	if rej, err := eps.ParseSecurityModeReject(plain); err == nil {
		cause = rej.Cause
	}

	logger.From(ctx, logger.MmeLog).Warn("Security Mode Reject",
		zap.Uint8("emm-cause", cause))

	m.ReleaseUEContext(ctx, ue, mme.CauseNASUnspecified)

	return nasreply.Handled()
}
