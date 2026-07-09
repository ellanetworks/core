// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"context"
	"encoding/hex"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/amf/ngap/decode"
	"github.com/ellanetworks/core/internal/amf/ngap/send"
	"github.com/ellanetworks/core/internal/amf/util"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/free5gc/aper"
	"github.com/free5gc/ngap/ngapType"
	"go.uber.org/zap"
)

// HandleRanConfigurationUpdate applies an NG-RAN node's configuration update. Per
// TS 38.413 §8.7.2.2 an absent IE leaves the corresponding configuration
// unchanged, so a name- or DRX-only update is accepted: only a present Supported
// TA List overwrites the stored TAs, and a present list broadcasting no served TAI
// is rejected with a Failure.
func HandleRanConfigurationUpdate(ctx context.Context, amfInstance *amf.AMF, ran *amf.Radio, msg decode.RANConfigurationUpdate) {
	var cause ngapType.Cause

	if msg.SupportedTAItems != nil {
		tais := ranConfigUpdateTAIs(ctx, ran, msg.SupportedTAItems)
		cause = validateRanSupportedTAIs(ctx, amfInstance, ran, tais)

		if cause.Present == ngapType.CausePresentNothing {
			amfInstance.UpdateRadioSupportedTAIs(ran, tais)
		}
	}

	if cause.Present == ngapType.CausePresentNothing && msg.RANNodeName != "" {
		amfInstance.UpdateRadioName(ran, msg.RANNodeName)
	}

	if cause.Present == ngapType.CausePresentNothing {
		pkt, err := send.BuildRanConfigurationUpdateAcknowledge(nil)
		if err != nil {
			logger.WithTrace(ctx, ran.Log).Error("error building ran configuration update acknowledge", zap.Error(err))
			return
		}

		ran.SendToRadio(ctx, send.NGAPProcedureRanConfigurationUpdateAcknowledge, pkt)
	} else {
		pkt, err := send.BuildRanConfigurationUpdateFailure(cause, nil)
		if err != nil {
			logger.WithTrace(ctx, ran.Log).Error("error building ran configuration update failure", zap.Error(err))
			return
		}

		ran.SendToRadio(ctx, send.NGAPProcedureRanConfigurationUpdateFailure, pkt)
	}
}

// ranConfigUpdateTAIs flattens the Supported TA List into one AMF TAI per
// (broadcast PLMN, TAC) pair with its supported slices.
func ranConfigUpdateTAIs(ctx context.Context, ran *amf.Radio, items []ngapType.SupportedTAItem) []amf.SupportedTAI {
	tais := make([]amf.SupportedTAI, 0)

	for _, supportedTAItem := range items {
		tac := hex.EncodeToString(supportedTAItem.TAC.Value)

		for _, broadcastPLMNItem := range supportedTAItem.BroadcastPLMNList.List {
			supportedTAI := amf.SupportedTAI{}
			supportedTAI.Tai.Tac = tac
			plmnID := util.PlmnIDToModels(broadcastPLMNItem.PLMNIdentity)
			supportedTAI.Tai.PlmnID = &plmnID

			for _, tAISliceSupportItem := range broadcastPLMNItem.TAISliceSupportList.List {
				supportedTAI.SNssaiList = append(supportedTAI.SNssaiList, util.SNssaiToModels(tAISliceSupportItem.SNSSAI))
			}

			logger.WithTrace(ctx, ran.Log).Debug("handle ran configuration update", zap.Any("PLMN_ID", plmnID), zap.String("TAC", tac))

			tais = append(tais, supportedTAI)
		}
	}

	return tais
}

// validateRanSupportedTAIs reports a Cause when the Supported TA List broadcasts
// no served TAI, and CausePresentNothing when it may be taken into use
// (TS 38.413 §8.7.2.3).
func validateRanSupportedTAIs(ctx context.Context, amfInstance *amf.AMF, ran *amf.Radio, tais []amf.SupportedTAI) ngapType.Cause {
	misc := func(v aper.Enumerated) ngapType.Cause {
		return ngapType.Cause{Present: ngapType.CausePresentMisc, Misc: &ngapType.CauseMisc{Value: v}}
	}

	if len(tais) == 0 {
		logger.WithTrace(ctx, ran.Log).Warn("RanConfigurationUpdate failure: No supported TA in the Supported TA List")
		return misc(ngapType.CauseMiscPresentUnspecified)
	}

	operatorInfo, err := amfInstance.OperatorInfo(ctx)
	if err != nil {
		logger.WithTrace(ctx, ran.Log).Error("Could not get operator info", zap.Error(err))
		return misc(ngapType.CauseMiscPresentUnspecified)
	}

	if !amf.AnyPLMNMatch(tais, operatorInfo.Guami.PlmnID) {
		logger.WithTrace(ctx, ran.Log).Warn("No broadcast PLMN matches operator", zap.Any("gnb_tai_list", tais), zap.Any("operator_plmn", operatorInfo.Guami.PlmnID))
		return misc(ngapType.CauseMiscPresentUnknownPLMN)
	}

	for i, tai := range tais {
		if amf.InTaiList(tai.Tai, operatorInfo.Tais) {
			logger.WithTrace(ctx, ran.Log).Debug("handle ran configuration update", zap.Any("SERVED_TAI_INDEX", i))
			return ngapType.Cause{}
		}
	}

	logger.WithTrace(ctx, ran.Log).Warn("PLMN matches but no served TAC found", zap.Any("gnb_tai_list", tais), zap.Any("core_tai_list", operatorInfo.Tais))

	return misc(ngapType.CauseMiscPresentUnspecified)
}
