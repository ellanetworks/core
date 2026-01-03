package ngap

import (
	"context"

	amfContext "github.com/ellanetworks/core/internal/amf/context"
	"github.com/ellanetworks/core/internal/amf/util"
	"github.com/free5gc/ngap/ngapType"
	"go.uber.org/zap"
)

func HandleUplinkRanConfigurationTransfer(ctx context.Context, amf *amfContext.AMF, ran *amfContext.Radio, msg *ngapType.UplinkRANConfigurationTransfer) {
	if msg == nil {
		ran.Log.Error("NGAP Message is nil")
		return
	}

	var sONConfigurationTransferUL *ngapType.SONConfigurationTransfer

	for _, ie := range msg.ProtocolIEs.List {
		switch ie.Id.Value {
		case ngapType.ProtocolIEIDSONConfigurationTransferUL: // optional, ignore
			sONConfigurationTransferUL = ie.Value.SONConfigurationTransferUL
			if sONConfigurationTransferUL == nil {
				ran.Log.Warn("sONConfigurationTransferUL is nil")
			}
		}
	}

	if sONConfigurationTransferUL == nil {
		ran.Log.Warn("sONConfigurationTransferUL is nil")
		return
	}

	targetRanNodeID := util.RanIDToModels(sONConfigurationTransferUL.TargetRANNodeID.GlobalRANNodeID)

	if targetRanNodeID.GNbID.GNBValue != "" {
		ran.Log.Debug("targetRanID", zap.String("targetRanID", targetRanNodeID.GNbID.GNBValue))
	}

	targetRan, ok := amf.FindRadioByRanID(targetRanNodeID)
	if !ok {
		ran.Log.Warn("targetRan is nil")
		return
	}

	err := targetRan.NGAPSender.SendDownlinkRanConfigurationTransfer(ctx, sONConfigurationTransferUL)
	if err != nil {
		ran.Log.Error("error sending downlink ran configuration transfer", zap.Error(err))
		return
	}

	ran.Log.Info("sent downlink ran configuration transfer to target ran", zap.Any("RAN ID", targetRan.RanID))
}
