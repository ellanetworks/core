package ngap

import (
	ctxt "context"

	"github.com/ellanetworks/core/internal/amf/context"
	"github.com/ellanetworks/core/internal/amf/ngap/message"
	"github.com/ellanetworks/core/internal/amf/util"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/free5gc/ngap/ngapType"
	"go.uber.org/zap"
)

func HandleUplinkRanConfigurationTransfer(ctx ctxt.Context, ran *context.AmfRan, msg *ngapType.NGAPPDU) {
	var sONConfigurationTransferUL *ngapType.SONConfigurationTransfer

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
		ran.Log.Error("InitiatingMessage is nil")
		return
	}

	uplinkRANConfigurationTransfer := initiatingMessage.Value.UplinkRANConfigurationTransfer
	if uplinkRANConfigurationTransfer == nil {
		ran.Log.Error("ErrorIndication is nil")
		return
	}

	for _, ie := range uplinkRANConfigurationTransfer.ProtocolIEs.List {
		switch ie.Id.Value {
		case ngapType.ProtocolIEIDSONConfigurationTransferUL: // optional, ignore
			sONConfigurationTransferUL = ie.Value.SONConfigurationTransferUL
			if sONConfigurationTransferUL == nil {
				ran.Log.Warn("sONConfigurationTransferUL is nil")
			}
		}
	}

	if sONConfigurationTransferUL != nil {
		targetRanNodeID := util.RanIDToModels(sONConfigurationTransferUL.TargetRANNodeID.GlobalRANNodeID)

		if targetRanNodeID.GNbID.GNBValue != "" {
			ran.Log.Debug("targetRanID", zap.String("targetRanID", targetRanNodeID.GNbID.GNBValue))
		}

		aMFSelf := context.AMFSelf()

		targetRan, ok := aMFSelf.AmfRanFindByRanID(targetRanNodeID)
		if !ok {
			ran.Log.Warn("targetRan is nil")
			return
		}

		err := message.SendDownlinkRanConfigurationTransfer(ctx, targetRan, sONConfigurationTransferUL)
		if err != nil {
			ran.Log.Error("error sending downlink ran configuration transfer", zap.Error(err))
		}
		ran.Log.Info("sent downlink ran configuration transfer to target ran", zap.Any("RAN ID", targetRan.RanID))
	}
}
