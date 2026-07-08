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

// egress finalizes a nasreply.Disposition on the connection a NAS message arrived on. An EMM
// or ESM STATUS is protected once the UE holds a security context and sent plain otherwise, so
// a peer the MME could not resolve to a secured context still receives the mandated STATUS.
// Unlike the AMF — which relays 5GSM to the SMF — the MME hosts ESM, so it emits the ESM
// STATUS itself (TS 24.301 §8.3.13).
type egress struct{ conn *mme.UeConn }

type nasMarshaler interface {
	Marshal() ([]byte, error)
}

func (e egress) SendMMStatus(ctx context.Context, cause uint8) {
	e.emit(ctx, &eps.EMMStatus{EMMCause: cause})
}

func (e egress) SendSMStatus(ctx context.Context, cause uint8) {
	e.emit(ctx, &eps.ESMStatus{ESMCause: cause})
}

func (e egress) emit(ctx context.Context, msg nasMarshaler) {
	if e.conn == nil {
		return
	}

	if ue := e.conn.UeContext(); ue != nil && ue.Secured() {
		e.conn.SendDownlinkProtected(ctx, msg)
		return
	}

	e.conn.SendDownlinkMessage(ctx, msg)
}

func (e egress) Discard(ctx context.Context, reason nasreply.Reason) {
	logger.From(ctx, logger.MmeLog).Debug("inbound NAS discarded", zap.String("reason", reason.String()))
}
