package ngap

import (
	"context"
	"encoding/hex"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/amf/ngap/decode"
	"github.com/ellanetworks/core/internal/amf/util"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	"github.com/free5gc/ngap/ngapType"
	"go.uber.org/zap"
)

func HandleRanConfigurationUpdate(ctx context.Context, amfInstance *amf.AMF, ran *amf.Radio, msg decode.RANConfigurationUpdate) {
	var cause ngapType.Cause

	if msg.SupportedTAItems == nil {
		logger.WithTrace(ctx, ran.Log).Warn("SupportedTAList IE is missing in RANConfigurationUpdate")
	}

	for _, supportedTAItem := range msg.SupportedTAItems {
		tac := hex.EncodeToString(supportedTAItem.TAC.Value)
		capOfSupportTai := cap(ran.SupportedTAIs)

		for _, broadcastPLMNItem := range supportedTAItem.BroadcastPLMNList.List {
			supportedTAI := amf.SupportedTAI{}
			supportedTAI.SNssaiList = make([]models.Snssai, 0)
			supportedTAI.Tai.Tac = tac
			plmnID := util.PlmnIDToModels(broadcastPLMNItem.PLMNIdentity)
			supportedTAI.Tai.PlmnID = &plmnID
			capOfSNssaiList := cap(supportedTAI.SNssaiList)

			for _, tAISliceSupportItem := range broadcastPLMNItem.TAISliceSupportList.List {
				if len(supportedTAI.SNssaiList) < capOfSNssaiList {
					supportedTAI.SNssaiList = append(supportedTAI.SNssaiList, util.SNssaiToModels(tAISliceSupportItem.SNSSAI))
				} else {
					break
				}
			}

			logger.WithTrace(ctx, ran.Log).Debug("handle ran configuration update", zap.Any("PLMN_ID", plmnID), zap.String("TAC", tac))

			if len(ran.SupportedTAIs) < capOfSupportTai {
				ran.SupportedTAIs = append(ran.SupportedTAIs, supportedTAI)
			} else {
				break
			}
		}
	}

	if len(ran.SupportedTAIs) == 0 {
		logger.WithTrace(ctx, ran.Log).Warn("RanConfigurationUpdate failure: No supported TA exist in RanConfigurationUpdate")

		cause.Present = ngapType.CausePresentMisc
		cause.Misc = &ngapType.CauseMisc{
			Value: ngapType.CauseMiscPresentUnspecified,
		}
	} else {
		operatorInfo, err := amfInstance.GetOperatorInfo(ctx)
		if err != nil {
			logger.WithTrace(ctx, ran.Log).Error("Could not get operator info", zap.Error(err))

			cause.Present = ngapType.CausePresentMisc
			cause.Misc = &ngapType.CauseMisc{
				Value: ngapType.CauseMiscPresentUnspecified,
			}

			return
		}

		var found bool

		for i, tai := range ran.SupportedTAIs {
			if amf.InTaiList(tai.Tai, operatorInfo.Tais) {
				logger.WithTrace(ctx, ran.Log).Debug("handle ran configuration update", zap.Any("SERVED_TAI_INDEX", i))

				found = true

				break
			}
		}

		if !found {
			logger.WithTrace(ctx, ran.Log).Warn("Cannot find Served TAI in Core")

			cause.Present = ngapType.CausePresentMisc
			cause.Misc = &ngapType.CauseMisc{
				Value: ngapType.CauseMiscPresentUnknownPLMN,
			}
		}
	}

	if cause.Present == ngapType.CausePresentNothing {
		err := ran.NGAPSender.SendRanConfigurationUpdateAcknowledge(ctx, nil)
		if err != nil {
			logger.WithTrace(ctx, ran.Log).Error("error sending ran configuration update acknowledge", zap.Error(err))
		}

		logger.WithTrace(ctx, ran.Log).Info("sent ran configuration update acknowledge to target ran", zap.Any("RAN ID", ran.RanID))
	} else {
		err := ran.NGAPSender.SendRanConfigurationUpdateFailure(ctx, cause, nil)
		if err != nil {
			logger.WithTrace(ctx, ran.Log).Error("error sending ran configuration update failure", zap.Error(err))
		}

		logger.WithTrace(ctx, ran.Log).Info("sent ran configuration update failure to target ran", zap.Any("RAN ID", ran.RanID))
	}
}
