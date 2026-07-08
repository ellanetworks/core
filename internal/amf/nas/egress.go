// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"context"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/nasreply"
	"go.uber.org/zap"
)

// egress finalizes a nasreply.Disposition on the 5GS connection a NAS message arrived on. The
// 5GMM STATUS is sent over the raw NGAP transport, so a peer the AMF could not resolve to a
// context still receives the STATUS the spec mandates.
type egress struct{ ue *amf.UeConn }

func (e egress) SendMMStatus(ctx context.Context, cause uint8) {
	pdu, err := amf.BuildStatus5GMM(cause)
	if err != nil {
		logger.From(ctx, logger.AmfLog).Error("failed to build 5GMM STATUS", zap.Error(err))
		return
	}

	if err := e.ue.SendDownlinkNASTransport(ctx, pdu, nil); err != nil {
		logger.From(ctx, logger.AmfLog).Warn("failed to send 5GMM STATUS", zap.Error(err))
	}
}

// SendSMStatus is never reached: 5GSM is relayed to the SMF (TS 24.501 §4.4), so the AMF's
// 5GSM handlers answer directly (forward, or a DL NAS "payload not forwarded") and never
// resolve to an SM-domain STATUS disposition.
func (e egress) SendSMStatus(ctx context.Context, cause uint8) {
	logger.From(ctx, logger.AmfLog).Error("unexpected 5GSM STATUS egress in the AMF", zap.Uint8("cause", cause))
}

func (e egress) Discard(ctx context.Context, reason nasreply.Reason) {
	logger.From(ctx, logger.AmfLog).Debug("inbound NAS discarded", zap.String("reason", reason.String()))
}
