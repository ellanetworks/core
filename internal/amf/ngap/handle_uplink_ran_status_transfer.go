// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"context"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/ngap"
)

// HandleUplinkRanStatusTransfer relays the source NG-RAN's PDCP SN/HFN status container
// to the handover target as a DOWNLINK RAN STATUS TRANSFER (TS 38.413 §8.4.6/§8.4.7),
// so an N2 handover of PDCP-SN-preserving DRBs is lossless. The transfer is optional
// (the source may omit it) and non-gating: a missing in-progress handover just drops it.
func HandleUplinkRanStatusTransfer(ctx context.Context, amfInstance *amf.AMF, ran *amf.Radio, msg *ngap.UplinkRANStatusTransfer) {
	ueConn, ok := resolveUE(ctx, amfInstance, ran, msg.AMFUENGAPID, msg.RANUENGAPID)
	if !ok {
		return
	}

	ueConn.TouchLastSeen()

	target := amfInstance.HandoverTarget(ueConn.UeContext())
	if target == nil {
		logger.WithTrace(ctx, ueConn.Log).Warn("RAN Status Transfer with no handover in progress")
		return
	}

	target.SendDownlinkRANStatusTransfer(ctx, msg.Container)
}
