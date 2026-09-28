// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package amf

import (
	"context"

	"github.com/ellanetworks/core/internal/amf/procedure"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/ngap"
	"go.uber.org/zap"
)

func (ueConn *UeConn) HeldUEAMBR() (models.Ambr, bool) {
	if ueConn == nil {
		return models.Ambr{}, false
	}

	held := ueConn.ranUEAMBR.Load()
	if held == nil {
		return models.Ambr{}, false
	}

	return *held, true
}

func (ueConn *UeConn) holdUEAMBR(uplink, downlink models.BitRate) {
	if ueConn != nil {
		ueConn.ranUEAMBR.Store(&models.Ambr{Uplink: uplink, Downlink: downlink})
	}
}

func (ueConn *UeConn) ForgetUEAMBR() {
	if ueConn != nil {
		ueConn.ranUEAMBR.Store(nil)
	}
}

func ueContextModificationBytes(amfID ngap.AMFUENGAPID, ranID ngap.RANUENGAPID, ambr models.Ambr) ([]byte, error) {
	msg := &ngap.UEContextModificationRequest{
		AMFUENGAPID: amfID,
		RANUENGAPID: ranID,
		UEAggregateMaximumBitRate: &ngap.UEAggregateMaximumBitRate{
			DL: ngap.BitRate(ambr.Downlink.Bps()),
			UL: ngap.BitRate(ambr.Uplink.Bps()),
		},
	}

	return msg.Marshal()
}

func (ueConn *UeConn) SendUEContextModification(ctx context.Context, ambr models.Ambr) error {
	amfInstance, conn, err := ueConn.sendTarget()
	if err != nil {
		return err
	}

	pkt, err := ueContextModificationBytes(ngap.AMFUENGAPID(ueConn.AmfUeNgapID), ngap.RANUENGAPID(ueConn.RanUeNgapID()), ambr)
	if err != nil {
		return err
	}

	ueConn.holdUEAMBR(ambr.Uplink, ambr.Downlink)

	return amfInstance.SendToRadio(ctx, conn, NGAPProcedureUEContextModificationRequest, pkt)
}

func (amf *AMF) ueAMBRTarget(ue *UeContext) (*UeConn, bool) {
	amf.mu.RLock()
	defer amf.mu.RUnlock()

	ueConn := ue.Conn()
	if ue.State() != Registered || ueConn == nil || ueConn.releasing || ue.handover != nil || ueConn.ICS() != ICSCompleted {
		return nil, false
	}

	if ue.procedures != nil && ue.procedures.Active(procedure.PathSwitch) {
		return nil, false
	}

	return ueConn, true
}

func (ue *UeContext) hasPDUSessions() bool {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	return len(ue.SmContextList) > 0
}

func (amf *AMF) SyncUEAMBR(ctx context.Context, ue *UeContext) {
	ueConn, ok := amf.ueAMBRTarget(ue)
	if !ok {
		return
	}

	want := ue.Ambr()
	if want == nil {
		return
	}

	held, ok := ueConn.HeldUEAMBR()
	if ok && held.Uplink.Equal(want.Uplink) && held.Downlink.Equal(want.Downlink) {
		return
	}

	if !ok && !ue.hasPDUSessions() {
		return
	}

	ueConn.Log(ctx).Info("UE-AMBR changed; sending UE Context Modification Request",
		zap.Stringer("ue_ambr_uplink", want.Uplink), zap.Stringer("ue_ambr_downlink", want.Downlink))

	if err := ueConn.SendUEContextModification(ctx, *want); err != nil {
		ueConn.Log(ctx).Warn("failed to send UE Context Modification Request", zap.Error(err))
	}
}

func (amf *AMF) registeredUEs() []*UeContext {
	amf.mu.RLock()
	defer amf.mu.RUnlock()

	ues := make([]*UeContext, 0, len(amf.UEs))

	for _, ue := range amf.UEs {
		if ue.State() == Registered {
			ues = append(ues, ue)
		}
	}

	return ues
}

func (amf *AMF) ReconcileUEAMBR(ctx context.Context) {
	for _, ue := range amf.registeredUEs() {
		subscribed, err := amf.SubscribedUEAMBR(ctx, ue.Supi())
		if err != nil {
			logger.From(ctx, logger.AmfLog).Warn("UE-AMBR reconcile: failed to resolve the subscribed UE-AMBR",
				logger.SUPI(ue.Supi().String()), zap.Error(err))

			continue
		}

		ue.SetAmbr(subscribed)
		amf.SyncUEAMBR(ctx, ue)
	}
}
