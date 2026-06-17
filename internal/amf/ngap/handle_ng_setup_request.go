// Copyright 2026 Ella Networks
// SPDX-FileCopyrightText: 2022-present Intel Corporation
// SPDX-FileCopyrightText: 2021 Open Networking Foundation <info@opennetworking.org>
// Copyright 2019 free5GC.org
// SPDX-License-Identifier: Apache-2.0

package ngap

import (
	"context"
	"encoding/hex"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/amf/ngap/decode"
	"github.com/ellanetworks/core/internal/amf/ngap/send"
	"github.com/ellanetworks/core/internal/amf/util"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/free5gc/ngap/ngapType"
	"go.uber.org/zap"
)

func HandleNGSetupRequest(ctx context.Context, amfInstance *amf.AMF, ran *amf.Radio, msg decode.NGSetupRequest) {
	if msg.RANNodeName != "" {
		ran.Name = msg.RANNodeName

		if realSender, ok := ran.NGAPSender.(*send.RealNGAPSender); ok {
			realSender.RadioName = ran.Name
		}
	}

	if len(ran.SupportedTAIs) != 0 {
		ran.SupportedTAIs = make([]amf.SupportedTAI, 0)
	}

	if len(msg.SupportedTAItems) == 0 {
		err := ran.NGAPSender.SendNGSetupFailure(ctx, &ngapType.Cause{
			Present: ngapType.CausePresentMisc,
			Misc: &ngapType.CauseMisc{
				Value: ngapType.CauseMiscPresentUnspecified,
			},
		})
		if err != nil {
			logger.WithTrace(ctx, ran.Log).Error("error sending NG Setup Failure", zap.Error(err))
			return
		}

		logger.WithTrace(ctx, ran.Log).Warn("NG Setup failure: No supported TA exist in NG Setup request")

		return
	}

	for i := 0; i < len(msg.SupportedTAItems); i++ {
		supportedTAItem := msg.SupportedTAItems[i]

		tac := hex.EncodeToString(supportedTAItem.TAC.Value)
		for j := 0; j < len(supportedTAItem.BroadcastPLMNList.List); j++ {
			supportedTAI := amf.SupportedTAI{}
			supportedTAI.Tai.Tac = tac
			broadcastPLMNItem := supportedTAItem.BroadcastPLMNList.List[j]
			plmnID := util.PlmnIDToModels(broadcastPLMNItem.PLMNIdentity)
			supportedTAI.Tai.PlmnID = &plmnID

			for k := 0; k < len(broadcastPLMNItem.TAISliceSupportList.List); k++ {
				tAISliceSupportItem := broadcastPLMNItem.TAISliceSupportList.List[k]
				supportedTAI.SNssaiList = append(supportedTAI.SNssaiList, util.SNssaiToModels(tAISliceSupportItem.SNSSAI))
			}

			ran.SupportedTAIs = append(ran.SupportedTAIs, supportedTAI)
		}
	}

	operatorInfo, err := amfInstance.GetOperatorInfo(ctx)
	if err != nil {
		logger.WithTrace(ctx, ran.Log).Error("Could not get operator info", zap.Error(err))

		_ = ran.NGAPSender.SendNGSetupFailure(ctx, &ngapType.Cause{
			Present: ngapType.CausePresentMisc,
			Misc:    &ngapType.CauseMisc{Value: ngapType.CauseMiscPresentUnspecified},
		})

		return
	}

	if !amf.AnyPLMNMatch(ran.SupportedTAIs, operatorInfo.Guami.PlmnID) {
		err := ran.NGAPSender.SendNGSetupFailure(ctx, &ngapType.Cause{
			Present: ngapType.CausePresentMisc,
			Misc:    &ngapType.CauseMisc{Value: ngapType.CauseMiscPresentUnknownPLMN},
		})
		if err != nil {
			logger.WithTrace(ctx, ran.Log).Error("error sending NG Setup Failure", zap.Error(err))
			return
		}

		logger.WithTrace(ctx, ran.Log).Warn("No broadcast PLMN matches operator", zap.Any("gnb_tai_list", ran.SupportedTAIs), zap.Any("operator_plmn", operatorInfo.Guami.PlmnID))

		return
	}

	var taiFound bool

	for i, tai := range ran.SupportedTAIs {
		if amf.InTaiList(tai.Tai, operatorInfo.Tais) {
			logger.WithTrace(ctx, ran.Log).Debug("Found served TAI in Core", zap.Any("served_tai", tai.Tai), zap.Int("index", i))

			taiFound = true

			break
		}
	}

	if !taiFound {
		err := ran.NGAPSender.SendNGSetupFailure(ctx, &ngapType.Cause{
			Present: ngapType.CausePresentMisc,
			Misc:    &ngapType.CauseMisc{Value: ngapType.CauseMiscPresentUnspecified},
		})
		if err != nil {
			logger.WithTrace(ctx, ran.Log).Error("error sending NG Setup Failure", zap.Error(err))
			return
		}

		logger.WithTrace(ctx, ran.Log).Warn("PLMN matches but no served TAC found", zap.Any("gnb_tai_list", ran.SupportedTAIs), zap.Any("core_tai_list", operatorInfo.Tais))

		return
	}

	snssaiList, err := amfInstance.ListOperatorSnssai(ctx)
	if err != nil {
		logger.WithTrace(ctx, ran.Log).Error("Could not list operator SNSSAI", zap.Error(err))

		_ = ran.NGAPSender.SendNGSetupFailure(ctx, &ngapType.Cause{
			Present: ngapType.CausePresentMisc,
			Misc:    &ngapType.CauseMisc{Value: ngapType.CauseMiscPresentUnspecified},
		})

		return
	}

	hasSliceOverlap := false

	for _, tai := range ran.SupportedTAIs {
		for _, gnbSlice := range tai.SNssaiList {
			for _, coreSlice := range snssaiList {
				if gnbSlice.Equal(coreSlice) {
					hasSliceOverlap = true

					break
				}
			}

			if hasSliceOverlap {
				break
			}
		}

		if hasSliceOverlap {
			break
		}
	}

	if !hasSliceOverlap {
		logger.WithTrace(ctx, ran.Log).Warn("gNB advertises no S-NSSAIs overlapping with operator",
			zap.Any("gnb_tai_list", ran.SupportedTAIs),
			zap.Any("core_slices", snssaiList))
	}

	// Claim RanID only after validation passes; the dispatcher's
	// ran.RanID != nil guard gates all other NGAP handlers.
	evicted := amfInstance.ClaimRanID(ran, msg.GlobalRANNodeID.Raw())
	if evicted != nil {
		evictedRemote := ""
		if evicted.Conn != nil && evicted.Conn.RemoteAddr() != nil {
			evictedRemote = evicted.Conn.RemoteAddr().String()
		}

		logger.WithTrace(ctx, ran.Log).Warn("Evicted existing NG-C association with duplicate Global RAN Node ID",
			zap.String("evicted_remote", evictedRemote),
			zap.String("evicted_name", evicted.Name),
		)
	}

	err = ran.NGAPSender.SendNGSetupResponse(ctx, operatorInfo.Guami, snssaiList, amfInstance.Name, amfInstance.RelativeCapacity)
	if err != nil {
		logger.WithTrace(ctx, ran.Log).Error("error sending NG Setup Response", zap.Error(err))
		return
	}

	logger.WithTrace(ctx, ran.Log).Info("Radio completed NG Setup", zap.String("name", ran.Name))
}
