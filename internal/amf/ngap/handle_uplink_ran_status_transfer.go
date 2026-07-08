// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"context"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/amf/ngap/decode"
	"github.com/ellanetworks/core/internal/amf/ngap/send"
	"github.com/ellanetworks/core/internal/logger"
	"go.uber.org/zap"
)

// HandleUplinkRanStatusTransfer relays the source NG-RAN's PDCP SN/HFN status container
// to the handover target as a DOWNLINK RAN STATUS TRANSFER (TS 38.413 §8.4.6/§8.4.7),
// so an N2 handover of PDCP-SN-preserving DRBs is lossless. The transfer is optional
// (the source may omit it) and non-gating: a missing in-progress handover just drops it.
func HandleUplinkRanStatusTransfer(ctx context.Context, amfInstance *amf.AMF, ran *amf.Radio, msg decode.UplinkRANStatusTransfer) {
	ueConn, ok := resolveUE(ctx, amfInstance, ran, &msg.RANUENGAPID, &msg.AMFUENGAPID)
	if !ok {
		return
	}

	ueConn.TouchLastSeen()

	target := amfInstance.HandoverTarget(ueConn.UeContext())
	if target == nil {
		logger.WithTrace(ctx, ueConn.Log).Warn("RAN Status Transfer with no handover in progress")
		return
	}

	pkt, err := send.BuildDownlinkRanStatusTransfer(target.AmfUeNgapID, target.RanUeNgapID, msg.Container)
	if err != nil {
		logger.WithTrace(ctx, ueConn.Log).Error("failed to build Downlink RAN Status Transfer", zap.Error(err))
		return
	}

	if err := target.SendNGAP(ctx, send.NGAPProcedureDownlinkRanStatusTransfer, pkt); err != nil {
		logger.WithTrace(ctx, ueConn.Log).Error("failed to send Downlink RAN Status Transfer", zap.Error(err))
	}
}
