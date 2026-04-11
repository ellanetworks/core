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
	ran.SetRanID(msg.GlobalRANNodeID.Raw())

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
		return
	}

	var found bool

	for i, tai := range ran.SupportedTAIs {
		if amf.InTaiList(tai.Tai, operatorInfo.Tais) {
			logger.WithTrace(ctx, ran.Log).Debug("Found served TAI in Core", zap.Any("served_tai", tai.Tai), zap.Int("index", i))

			found = true

			break
		}
	}

	if !found {
		err := ran.NGAPSender.SendNGSetupFailure(ctx, &ngapType.Cause{
			Present: ngapType.CausePresentMisc,
			Misc: &ngapType.CauseMisc{
				Value: ngapType.CauseMiscPresentUnknownPLMN,
			},
		})
		if err != nil {
			logger.WithTrace(ctx, ran.Log).Error("error sending NG Setup Failure", zap.Error(err))
			return
		}

		logger.WithTrace(ctx, ran.Log).Warn("Could not find Served TAI in Core", zap.Any("gnb_tai_list", ran.SupportedTAIs), zap.Any("core_tai_list", operatorInfo.Tais))

		return
	}

	snssaiList, err := amfInstance.ListOperatorSnssai(ctx)
	if err != nil {
		logger.WithTrace(ctx, ran.Log).Error("Could not list operator SNSSAI", zap.Error(err))
		return
	}

	err = ran.NGAPSender.SendNGSetupResponse(ctx, operatorInfo.Guami, snssaiList, amfInstance.Name, amfInstance.RelativeCapacity)
	if err != nil {
		logger.WithTrace(ctx, ran.Log).Error("error sending NG Setup Response", zap.Error(err))
		return
	}

	logger.WithTrace(ctx, ran.Log).Info("Radio completed NG Setup", zap.String("name", ran.Name))
}
