// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"context"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/amf/util"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/ngap"
	"go.uber.org/zap"
)

// HandleUplinkRANConfigurationTransfer relays a SON Configuration Transfer IE
// verbatim from UPLINK RAN CONFIGURATION TRANSFER into a DOWNLINK RAN
// CONFIGURATION TRANSFER toward the NG-RAN node named in its Target RAN Node ID
// (TS 38.413 §8.8.1.2/§8.8.2.2). Non-UE, fire-and-forget: an absent IE or an
// unconnected target is dropped.
func HandleUplinkRANConfigurationTransfer(ctx context.Context, amfInstance *amf.AMF, ran *amf.Radio, msg *ngap.UplinkRANConfigurationTransfer) {
	if msg.SONConfigurationTransfer == nil {
		return
	}

	target, err := msg.SONConfigurationTransfer.TargetRANNodeID()
	if err != nil {
		logger.WithTrace(ctx, ran.Log).Warn("could not decode Target RAN Node ID from SON Configuration Transfer", zap.Error(err))
		return
	}

	targetID := util.RANNodeIDToModels(target.GlobalRANNodeID)

	targetRadio, ok := amfInstance.FindConnectedRadioByRanID(targetID)
	if !ok {
		logger.WithTrace(ctx, ran.Log).Warn("SON Configuration Transfer target NG-RAN node not connected",
			zap.Any("target-ran-node-id", targetID))

		return
	}

	dl := &ngap.DownlinkRANConfigurationTransfer{SONConfigurationTransfer: msg.SONConfigurationTransfer}

	b, err := dl.Marshal()
	if err != nil {
		logger.WithTrace(ctx, ran.Log).Error("failed to marshal Downlink RAN Configuration Transfer", zap.Error(err))
		return
	}

	targetRadio.SendToRadio(ctx, amf.NGAPProcedureDownlinkRANConfigurationTransfer, b)
}
