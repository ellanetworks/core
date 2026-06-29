// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"context"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/mme"
	"github.com/ellanetworks/core/nas/eps"
	"go.uber.org/zap"
)

// handleSecurityModeReject handles a SECURITY MODE REJECT (TS 24.301): the
// UE rejected the selected NAS security algorithms, so the security mode control
// procedure — and the attach/service procedure that triggered it — is aborted
// and the UE's S1 context released.
func handleSecurityModeReject(m *mme.MME, ctx context.Context, ue *mme.UeContext, plain []byte) {
	m.StopNASGuard(ue)

	var cause uint8
	if rej, err := eps.ParseSecurityModeReject(plain); err == nil {
		cause = rej.Cause
	}

	logger.MmeLog.Warn("Security Mode Reject",
		zap.Uint32("mme-ue-id", uint32(ue.S1.MMEUES1APID)),
		zap.Uint8("emm-cause", cause))

	m.ReleaseUEContext(ctx, ue, mme.CauseNASUnspecified)
}
