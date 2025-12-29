package ngap

import (
	"context"
	"encoding/hex"

	amfContext "github.com/ellanetworks/core/internal/amf/context"
	"github.com/ellanetworks/core/internal/amf/util"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	"github.com/free5gc/ngap/ngapType"
	"go.uber.org/zap"
)

func HandleRanConfigurationUpdate(ctx context.Context, ran *amfContext.AmfRan, msg *ngapType.NGAPPDU) {
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

	rANConfigurationUpdate := initiatingMessage.Value.RANConfigurationUpdate
	if rANConfigurationUpdate == nil {
		ran.Log.Error("RAN Configuration is nil")
		return
	}

	var rANNodeName *ngapType.RANNodeName
	var supportedTAList *ngapType.SupportedTAList
	var pagingDRX *ngapType.PagingDRX
	var cause ngapType.Cause

	for i := 0; i < len(rANConfigurationUpdate.ProtocolIEs.List); i++ {
		ie := rANConfigurationUpdate.ProtocolIEs.List[i]
		switch ie.Id.Value {
		case ngapType.ProtocolIEIDRANNodeName:
			rANNodeName = ie.Value.RANNodeName
			if rANNodeName == nil {
				ran.Log.Error("RAN Node Name is nil")
				return
			}
		case ngapType.ProtocolIEIDSupportedTAList:
			supportedTAList = ie.Value.SupportedTAList
			if supportedTAList == nil {
				ran.Log.Error("Supported TA List is nil")
				return
			}
		case ngapType.ProtocolIEIDDefaultPagingDRX:
			pagingDRX = ie.Value.DefaultPagingDRX
			if pagingDRX == nil {
				ran.Log.Error("PagingDRX is nil")
				return
			}
		}
	}

	for i := 0; i < len(supportedTAList.List); i++ {
		supportedTAItem := supportedTAList.List[i]
		tac := hex.EncodeToString(supportedTAItem.TAC.Value)
		capOfSupportTai := cap(ran.SupportedTAList)
		for j := 0; j < len(supportedTAItem.BroadcastPLMNList.List); j++ {
			supportedTAI := amfContext.SupportedTAI{}
			supportedTAI.SNssaiList = make([]models.Snssai, 0)
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
			ran.Log.Debug("handle ran configuration update", zap.Any("PLMN_ID", plmnID), zap.String("TAC", tac))
			if len(ran.SupportedTAList) < capOfSupportTai {
				ran.SupportedTAList = append(ran.SupportedTAList, supportedTAI)
			} else {
				break
			}
		}
	}

	if len(ran.SupportedTAList) == 0 {
		ran.Log.Warn("RanConfigurationUpdate failure: No supported TA exist in RanConfigurationUpdate")
		cause.Present = ngapType.CausePresentMisc
		cause.Misc = &ngapType.CauseMisc{
			Value: ngapType.CauseMiscPresentUnspecified,
		}
	} else {
		var found bool
		amfSelf := amfContext.AMFSelf()

		operatorInfo, err := amfSelf.GetOperatorInfo(ctx)
		if err != nil {
			ran.Log.Error("Could not get operator info", zap.Error(err))
			cause.Present = ngapType.CausePresentMisc
			cause.Misc = &ngapType.CauseMisc{
				Value: ngapType.CauseMiscPresentUnspecified,
			}
			return
		}

		for i, tai := range ran.SupportedTAList {
			if amfContext.InTaiList(tai.Tai, operatorInfo.Tais) {
				ran.Log.Debug("handle ran configuration update", zap.Any("SERVED_TAI_INDEX", i))
				found = true
				break
			}
		}
		if !found {
			ran.Log.Warn("Cannot find Served TAI in Core")
			cause.Present = ngapType.CausePresentMisc
			cause.Misc = &ngapType.CauseMisc{
				Value: ngapType.CauseMiscPresentUnknownPLMN,
			}
		}
	}

	if cause.Present == ngapType.CausePresentNothing {
		err := ran.NGAPSender.SendRanConfigurationUpdateAcknowledge(ctx, nil)
		if err != nil {
			ran.Log.Error("error sending ran configuration update acknowledge", zap.Error(err))
		}
		ran.Log.Info("sent ran configuration update acknowledge to target ran", zap.Any("RAN ID", ran.RanID))
	} else {
		err := ran.NGAPSender.SendRanConfigurationUpdateFailure(ctx, cause, nil)
		if err != nil {
			ran.Log.Error("error sending ran configuration update failure", zap.Error(err))
		}
		ran.Log.Info("sent ran configuration update failure to target ran", zap.Any("RAN ID", ran.RanID))
	}
}
