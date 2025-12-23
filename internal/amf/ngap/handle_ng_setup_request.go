package ngap

import (
	ctxt "context"
	"encoding/hex"

	"github.com/ellanetworks/core/internal/amf/context"
	"github.com/ellanetworks/core/internal/amf/util"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/free5gc/ngap/ngapType"
	"go.uber.org/zap"
)

func HandleNGSetupRequest(ctx ctxt.Context, amf *context.AMFContext, ran *context.AmfRan, msg *ngapType.NGAPPDU) {
	if ran == nil {
		logger.AmfLog.Error("ran is nil")
		return
	}

	if msg == nil {
		ran.Log.Error("NGAP Message is nil")
		return
	}

	initiatingMessage := msg.InitiatingMessage
	if initiatingMessage == nil {
		ran.Log.Fatal("Initiating Message is nil")
		return
	}

	nGSetupRequest := initiatingMessage.Value.NGSetupRequest
	if nGSetupRequest == nil {
		ran.Log.Fatal("NGSetupRequest is nil")
		return
	}

	var globalRANNodeID *ngapType.GlobalRANNodeID
	var rANNodeName *ngapType.RANNodeName
	var supportedTAList *ngapType.SupportedTAList
	var pagingDRX *ngapType.PagingDRX

	for i := 0; i < len(nGSetupRequest.ProtocolIEs.List); i++ {
		ie := nGSetupRequest.ProtocolIEs.List[i]
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
	if len(ran.SupportedTAList) != 0 {
		ran.SupportedTAList = make([]context.SupportedTAI, 0)
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
			supportedTAI := context.SupportedTAI{}
			supportedTAI.Tai.Tac = tac
			broadcastPLMNItem := supportedTAItem.BroadcastPLMNList.List[j]
			plmnID := util.PlmnIDToModels(broadcastPLMNItem.PLMNIdentity)
			supportedTAI.Tai.PlmnID = &plmnID
			for k := 0; k < len(broadcastPLMNItem.TAISliceSupportList.List); k++ {
				tAISliceSupportItem := broadcastPLMNItem.TAISliceSupportList.List[k]
				supportedTAI.SNssaiList = append(supportedTAI.SNssaiList, util.SNssaiToModels(tAISliceSupportItem.SNSSAI))
			}

			ran.SupportedTAList = append(ran.SupportedTAList, supportedTAI)
		}
	}

	operatorInfo, err := amf.GetOperatorInfo(ctx)
	if err != nil {
		ran.Log.Error("Could not get operator info", zap.Error(err))
		return
	}

	var found bool

	for i, tai := range ran.SupportedTAList {
		if context.InTaiList(tai.Tai, operatorInfo.Tais) {
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
		ran.Log.Warn("Could not find Served TAI in Core", zap.Any("gnb_tai_list", ran.SupportedTAList), zap.Any("core_tai_list", operatorInfo.Tais))
		return
	}

	err = ran.NGAPSender.SendNGSetupResponse(ctx, operatorInfo.Guami, operatorInfo.SupportedPLMN, amf.Name, amf.RelativeCapacity)
	if err != nil {
		ran.Log.Error("error sending NG Setup Response", zap.Error(err))
		return
	}
}
