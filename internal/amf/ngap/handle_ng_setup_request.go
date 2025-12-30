package ngap

import (
	"context"
	"encoding/hex"

	amfContext "github.com/ellanetworks/core/internal/amf/context"
	"github.com/ellanetworks/core/internal/amf/util"
	"github.com/free5gc/ngap/ngapType"
	"go.uber.org/zap"
)

func HandleNGSetupRequest(ctx context.Context, amf *amfContext.AMF, ran *amfContext.Radio, msg *ngapType.NGSetupRequest) {
	if msg == nil {
		ran.Log.Error("NG Setup Request Message is nil")
		return
	}

	var globalRANNodeID *ngapType.GlobalRANNodeID
	var rANNodeName *ngapType.RANNodeName
	var supportedTAList *ngapType.SupportedTAList
	var pagingDRX *ngapType.PagingDRX

	for i := 0; i < len(msg.ProtocolIEs.List); i++ {
		ie := msg.ProtocolIEs.List[i]
		switch ie.Id.Value {
		case ngapType.ProtocolIEIDGlobalRANNodeID:
			globalRANNodeID = ie.Value.GlobalRANNodeID
			if globalRANNodeID == nil {
				ran.Log.Error("GlobalRANNodeID is nil")
				return
			}
		case ngapType.ProtocolIEIDSupportedTAList:
			supportedTAList = ie.Value.SupportedTAList
			if supportedTAList == nil {
				ran.Log.Error("SupportedTAList is nil")
				return
			}
		case ngapType.ProtocolIEIDRANNodeName:
			rANNodeName = ie.Value.RANNodeName
			if rANNodeName == nil {
				ran.Log.Error("RANNodeName is nil")
				return
			}
		case ngapType.ProtocolIEIDDefaultPagingDRX:
			pagingDRX = ie.Value.DefaultPagingDRX
			if pagingDRX == nil {
				ran.Log.Error("DefaultPagingDRX is nil")
				return
			}
		}
	}
	if globalRANNodeID != nil {
		ran.SetRanID(globalRANNodeID)
	}

	if rANNodeName != nil {
		ran.Name = rANNodeName.Value
	}

	// Clearing any existing contents of ran.SupportedTAList
	if len(ran.SupportedTAIs) != 0 {
		ran.SupportedTAIs = make([]amfContext.SupportedTAI, 0)
	}

	if supportedTAList == nil || len(supportedTAList.List) == 0 {
		err := ran.NGAPSender.SendNGSetupFailure(ctx, &ngapType.Cause{
			Present: ngapType.CausePresentMisc,
			Misc: &ngapType.CauseMisc{
				Value: ngapType.CauseMiscPresentUnspecified,
			},
		})
		if err != nil {
			ran.Log.Error("error sending NG Setup Failure", zap.Error(err))
			return
		}
		ran.Log.Warn("NG Setup failure: No supported TA exist in NG Setup request")
		return
	}

	for i := 0; i < len(supportedTAList.List); i++ {
		supportedTAItem := supportedTAList.List[i]
		tac := hex.EncodeToString(supportedTAItem.TAC.Value)
		for j := 0; j < len(supportedTAItem.BroadcastPLMNList.List); j++ {
			supportedTAI := amfContext.SupportedTAI{}
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

	operatorInfo, err := amf.GetOperatorInfo(ctx)
	if err != nil {
		ran.Log.Error("Could not get operator info", zap.Error(err))
		return
	}

	var found bool

	for i, tai := range ran.SupportedTAIs {
		if amfContext.InTaiList(tai.Tai, operatorInfo.Tais) {
			ran.Log.Debug("Found served TAI in Core", zap.Any("served_tai", tai.Tai), zap.Int("index", i))
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
			ran.Log.Error("error sending NG Setup Failure", zap.Error(err))
			return
		}
		ran.Log.Warn("Could not find Served TAI in Core", zap.Any("gnb_tai_list", ran.SupportedTAIs), zap.Any("core_tai_list", operatorInfo.Tais))
		return
	}

	err = ran.NGAPSender.SendNGSetupResponse(ctx, operatorInfo.Guami, operatorInfo.SupportedPLMN, amf.Name, amf.RelativeCapacity)
	if err != nil {
		ran.Log.Error("error sending NG Setup Response", zap.Error(err))
		return
	}
}
