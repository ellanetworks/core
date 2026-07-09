// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-FileCopyrightText: 2022-present Intel Corporation
// SPDX-FileCopyrightText: 2021 Open Networking Foundation <info@opennetworking.org>
// Copyright 2019 free5GC.org
// Modified by Ella Networks Inc.
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
	"github.com/free5gc/ngap/ngapType"
	"go.uber.org/zap"
)

func sendNGSetupFailure(ctx context.Context, ran *amf.Radio, cause *ngapType.Cause) {
	pkt, err := send.BuildNGSetupFailure(cause)
	if err != nil {
		logger.WithTrace(ctx, ran.Log).Error("error building NG Setup Failure", zap.Error(err))
		return
	}

	_ = ran.SendToRadio(ctx, send.NGAPProcedureNGSetupFailure, pkt)
}

func HandleNGSetupRequest(ctx context.Context, amfInstance *amf.AMF, ran *amf.Radio, msg decode.NGSetupRequest) {
	name := msg.RANNodeName

	if len(msg.SupportedTAItems) == 0 {
		sendNGSetupFailure(ctx, ran, &ngapType.Cause{
			Present: ngapType.CausePresentMisc,
			Misc: &ngapType.CauseMisc{
				Value: ngapType.CauseMiscPresentUnspecified,
			},
		})

		logger.WithTrace(ctx, ran.Log).Warn("NG Setup failure: No supported TA exist in NG Setup request")

		return
	}

	// Build the TAI list locally and validate it, then commit name+TAIs to the shared
	// Radio through the locked setters below — the status path must never read a
	// half-written field.
	tais := make([]amf.SupportedTAI, 0)

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

			tais = append(tais, supportedTAI)
		}
	}

	operatorInfo, err := amfInstance.OperatorInfo(ctx)
	if err != nil {
		logger.WithTrace(ctx, ran.Log).Error("Could not get operator info", zap.Error(err))

		sendNGSetupFailure(ctx, ran, &ngapType.Cause{
			Present: ngapType.CausePresentMisc,
			Misc:    &ngapType.CauseMisc{Value: ngapType.CauseMiscPresentUnspecified},
		})

		return
	}

	if !amf.AnyPLMNMatch(tais, operatorInfo.Guami.PlmnID) {
		sendNGSetupFailure(ctx, ran, &ngapType.Cause{
			Present: ngapType.CausePresentMisc,
			Misc:    &ngapType.CauseMisc{Value: ngapType.CauseMiscPresentUnknownPLMN},
		})

		logger.WithTrace(ctx, ran.Log).Warn("No broadcast PLMN matches operator", zap.Any("gnb_tai_list", tais), zap.Any("operator_plmn", operatorInfo.Guami.PlmnID))

		return
	}

	var taiFound bool

	for i, tai := range tais {
		if amf.InTaiList(tai.Tai, operatorInfo.Tais) {
			logger.WithTrace(ctx, ran.Log).Debug("Found served TAI in Core", zap.Any("served_tai", tai.Tai), zap.Int("index", i))

			taiFound = true

			break
		}
	}

	if !taiFound {
		sendNGSetupFailure(ctx, ran, &ngapType.Cause{
			Present: ngapType.CausePresentMisc,
			Misc:    &ngapType.CauseMisc{Value: ngapType.CauseMiscPresentUnspecified},
		})

		logger.WithTrace(ctx, ran.Log).Warn("PLMN matches but no served TAC found", zap.Any("gnb_tai_list", tais), zap.Any("core_tai_list", operatorInfo.Tais))

		return
	}

	snssaiList, err := amfInstance.ListOperatorSnssai(ctx)
	if err != nil {
		logger.WithTrace(ctx, ran.Log).Error("Could not list operator SNSSAI", zap.Error(err))

		sendNGSetupFailure(ctx, ran, &ngapType.Cause{
			Present: ngapType.CausePresentMisc,
			Misc:    &ngapType.CauseMisc{Value: ngapType.CauseMiscPresentUnspecified},
		})

		return
	}

	hasSliceOverlap := false

	for _, tai := range tais {
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
			zap.Any("gnb_tai_list", tais),
			zap.Any("core_slices", snssaiList))
	}

	if name != "" {
		amfInstance.UpdateRadioName(ran, name)
	}

	amfInstance.UpdateRadioSupportedTAIs(ran, tais)

	// Claim RanID only after validation passes; the dispatcher's
	// ran.RanID != nil guard gates all other NGAP handlers.
	evicted := amfInstance.ClaimRanID(ran, msg.GlobalRANNodeID.Raw())
	if evicted != nil {
		logger.WithTrace(ctx, ran.Log).Warn("Evicted existing NG-C association with duplicate Global RAN Node ID",
			zap.String("evicted_remote", amf.AddrString(evicted.RemoteAddr())),
			zap.String("evicted_name", evicted.NodeName()),
		)
	}

	pkt, err := send.BuildNGSetupResponse(operatorInfo.Guami, snssaiList, amfInstance.Name, amfInstance.RelativeCapacity)
	if err != nil {
		logger.WithTrace(ctx, ran.Log).Error("error building NG Setup Response", zap.Error(err))
		return
	}

	_ = ran.SendToRadio(ctx, send.NGAPProcedureNGSetupResponse, pkt)

	logger.WithTrace(ctx, ran.Log).Info("Radio completed NG Setup", zap.String("name", name))
}
