// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"context"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/nasreply"
	"github.com/ellanetworks/core/nas/fgs"
	"go.uber.org/zap"
)

// egress finalizes a nasreply.Disposition on the 5GS connection a NAS message arrived on. A
// peer the AMF could not resolve to a context still receives the 5GMM STATUS the spec mandates,
// over the raw NGAP transport.
type egress struct{ ue *amf.UeConn }

func (e egress) SendMMStatus(ctx context.Context, cause uint8) {
	amf.SendStatus5GMM(ctx, e.ue, fgs.GMMCause(cause))
}

// SendSMStatus is never reached: 5GSM is relayed to the SMF (TS 24.501 §4.4), so the AMF's
// 5GSM handlers answer directly (forward, or a DL NAS "payload not forwarded") and never
// resolve to an SM-domain STATUS disposition.
func (e egress) SendSMStatus(ctx context.Context, cause uint8) {
	logger.From(ctx, logger.AmfLog).Error("unexpected 5GSM STATUS egress in the AMF", logger.Cause(fgs.GSMCause(cause).String()))
}

func (e egress) Discard(ctx context.Context, reason nasreply.Reason) {
	logger.From(ctx, logger.AmfLog).Debug("inbound NAS discarded", zap.String("reason", reason.String()))
}
