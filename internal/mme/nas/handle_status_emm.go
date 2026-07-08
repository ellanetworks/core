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

// handleEMMStatus logs an inbound EMM STATUS; per TS 24.301 §5.7 no state
// transition and no radio-interface action is taken.
func handleEMMStatus(plain []byte) {
	msg, err := eps.ParseEMMStatus(plain)
	if err != nil {
		logger.MmeLog.Warn("failed to decode EMM STATUS", zap.Error(err))
		return
	}

	logger.MmeLog.Error("received EMM STATUS", zap.Uint8("emm-cause", msg.EMMCause))
}

// sendEMMStatus reports an error condition detected on received EMM protocol data,
// so the UE is not left waiting on an unhandled or erroneous message (TS 24.301
// §5.7). It is integrity-protected and ciphered when the UE has a security context.
func sendEMMStatus(ctx context.Context, ue *mme.UeContext, cause uint8) {
	status := &eps.EMMStatus{EMMCause: cause}

	if ue.Secured() {
		ue.Conn().SendDownlinkProtected(ctx, status)
		return
	}

	ue.Conn().SendDownlinkMessage(ctx, status)
}
