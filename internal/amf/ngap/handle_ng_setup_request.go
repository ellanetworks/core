package ngap

import (
	ctxt "context"
	"encoding/hex"

	"github.com/ellanetworks/core/internal/amf/context"
	"github.com/ellanetworks/core/internal/amf/ngap/message"
	"github.com/ellanetworks/core/internal/amf/util"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	"github.com/free5gc/ngap/ngapType"
	"go.uber.org/zap"
)

func HandleNGSetupRequest(ctx ctxt.Context, ran *context.AmfRan, msg *ngapType.NGAPPDU) {
	var globalRANNodeID *ngapType.GlobalRANNodeID
	var rANNodeName *ngapType.RANNodeName
	var supportedTAList *ngapType.SupportedTAList
	var pagingDRX *ngapType.PagingDRX

	var cause ngapType.Cause

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
		ran.Log.Error("Initiating Message is nil")
		return
	}
	nGSetupRequest := initiatingMessage.Value.NGSetupRequest
	if nGSetupRequest == nil {
		ran.Log.Error("NGSetupRequest is nil")
		return
	}

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
		ran.SupportedTAList = context.NewSupportedTAIList()
	}

	for i := 0; i < len(supportedTAList.List); i++ {
		supportedTAItem := supportedTAList.List[i]
		tac := hex.EncodeToString(supportedTAItem.TAC.Value)
		capOfSupportTai := cap(ran.SupportedTAList)
		for j := 0; j < len(supportedTAItem.BroadcastPLMNList.List); j++ {
			supportedTAI := context.SupportedTAI{}
			supportedTAI.Tai.Tac = tac
			broadcastPLMNItem := supportedTAItem.BroadcastPLMNList.List[j]
			plmnID := util.PlmnIDToModels(broadcastPLMNItem.PLMNIdentity)
			supportedTAI.Tai.PlmnID = &plmnID
			capOfSNssaiList := cap(supportedTAI.SNssaiList)
			for k := 0; k < len(broadcastPLMNItem.TAISliceSupportList.List); k++ {
				tAISliceSupportItem := broadcastPLMNItem.TAISliceSupportList.List[k]
				if len(supportedTAI.SNssaiList) < capOfSNssaiList {
					supportedTAI.SNssaiList = append(supportedTAI.SNssaiList, util.SNssaiToModels(tAISliceSupportItem.SNSSAI))
				} else {
					break
				}
			}

			if len(ran.SupportedTAList) < capOfSupportTai {
				ran.SupportedTAList = append(ran.SupportedTAList, supportedTAI)
			} else {
				break
			}
		}
	}

	operatorInfo, err := context.GetOperatorInfo(ctx)
	if err != nil {
		ran.Log.Error("Could not get operator info", zap.Error(err))
		return
	}

	if len(ran.SupportedTAList) == 0 {
		ran.Log.Warn("NG Setup failure: No supported TA exist in NG Setup request")
		cause.Present = ngapType.CausePresentMisc
		cause.Misc = &ngapType.CauseMisc{
			Value: ngapType.CauseMiscPresentUnspecified,
		}
	} else {
		var found bool

		taiList := make([]models.Tai, len(operatorInfo.Tais))
		copy(taiList, operatorInfo.Tais)
		for i := range taiList {
			tac, err := util.TACConfigToModels(taiList[i].Tac)
			if err != nil {
				ran.Log.Warn("tac is invalid", zap.String("tac", taiList[i].Tac))
				continue
			}
			taiList[i].Tac = tac
		}

		for i, tai := range ran.SupportedTAList {
			if context.InTaiList(tai.Tai, taiList) {
				ran.Log.Debug("Found served TAI in Core", zap.Any("served_tai", tai.Tai), zap.Int("index", i))
				found = true
				break
			}
		}
		if !found {
			ran.Log.Warn("Could not find Served TAI in Core", zap.Any("gnb_tai_list", ran.SupportedTAList), zap.Any("core_tai_list", taiList))
			cause.Present = ngapType.CausePresentMisc
			cause.Misc = &ngapType.CauseMisc{
				Value: ngapType.CauseMiscPresentUnknownPLMN,
			}
		}
	}

	if cause.Present == ngapType.CausePresentNothing {
		err := message.SendNGSetupResponse(ctx, ran, operatorInfo.Guami, operatorInfo.SupportedPLMN)
		if err != nil {
			ran.Log.Error("error sending NG Setup Response", zap.Error(err))
			return
		}
	} else {
		err := message.SendNGSetupFailure(ctx, ran, cause)
		if err != nil {
			ran.Log.Error("error sending NG Setup Failure", zap.Error(err))
			return
		}
	}
}
