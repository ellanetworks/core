// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"context"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/free5gc/nas/nasMessage"
	"go.uber.org/zap"
)

// sendStatus5GMM sends a 5GMM STATUS over the UE's connection to report an error
// condition (TS 24.501 §5.4.5, §7.4). It is the NAS layer's only STATUS emitter: the
// transport layer never answers a discarded or unresolved message
// (TS 24.501 §4.4.4.3, §7.1).
func sendStatus5GMM(ctx context.Context, ue *amf.UeContext, cause uint8) {
	conn := ue.Conn()
	if conn == nil {
		return
	}

	pdu, err := amf.BuildStatus5GMM(cause)
	if err != nil {
		logger.From(ctx, logger.AmfLog).Error("failed to build 5GMM STATUS", zap.Error(err))
		return
	}

	if err := conn.SendDownlinkNASTransport(ctx, pdu, nil); err != nil {
		logger.From(ctx, logger.AmfLog).Error("failed to send 5GMM STATUS", zap.Error(err))
	}
}

func handleStatus5GMM(ctx context.Context, ue *amf.UeContext, msg *nasMessage.Status5GMM) {
	if ue.State() == amf.Deregistered {
		logger.From(ctx, logger.AmfLog).Warn("UE is in amf.Deregistered state, ignore Status 5GMM message")
		return
	}

	logger.From(ctx, logger.AmfLog).Error("Received Status 5GMM with cause", logger.Cause(nasMessage.Cause5GMMToString(msg.GetCauseValue())))
}
