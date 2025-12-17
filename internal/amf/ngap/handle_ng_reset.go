package ngap

import (
	ctxt "context"

	"github.com/ellanetworks/core/internal/amf/context"
	"github.com/ellanetworks/core/internal/amf/ngap/message"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/free5gc/ngap/ngapType"
	"go.uber.org/zap"
)

func HandleNGReset(ctx ctxt.Context, ran *context.AmfRan, msg *ngapType.NGAPPDU) {
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

	nGReset := initiatingMessage.Value.NGReset
	if nGReset == nil {
		ran.Log.Error("NGReset is nil")
		return
	}

	var cause *ngapType.Cause
	var resetType *ngapType.ResetType

	for _, ie := range nGReset.ProtocolIEs.List {
		switch ie.Id.Value {
		case ngapType.ProtocolIEIDCause:
			cause = ie.Value.Cause
			if cause == nil {
				ran.Log.Error("Cause is nil")
				return
			}
		case ngapType.ProtocolIEIDResetType:
			resetType = ie.Value.ResetType
			if resetType == nil {
				ran.Log.Error("ResetType is nil")
				return
			}
		}
	}

	logger.AmfLog.Debug("Received NG Reset with Cause", zap.String("Cause", causeToString(*cause)))

	switch resetType.Present {
	case ngapType.ResetTypePresentNGInterface:
		ran.Log.Debug("ResetType Present: NG Interface")
		ran.RemoveAllUeInRan()
		ran.Log.Debug("All UE Context in RAN have been removed")
		err := message.SendNGResetAcknowledge(ctx, ran, nil)
		if err != nil {
			ran.Log.Error("error sending NG Reset Acknowledge", zap.Error(err))
			return
		}
	case ngapType.ResetTypePresentPartOfNGInterface:
		ran.Log.Debug("ResetType Present: Part of NG Interface")

		partOfNGInterface := resetType.PartOfNGInterface
		if partOfNGInterface == nil {
			ran.Log.Error("PartOfNGInterface is nil")
			return
		}

		var ranUe *context.RanUe

		for _, ueAssociatedLogicalNGConnectionItem := range partOfNGInterface.List {
			if ueAssociatedLogicalNGConnectionItem.AMFUENGAPID != nil {
				ran.Log.Debug("NG Reset with AMFUENGAPID", zap.Int64("AmfUeNgapID", ueAssociatedLogicalNGConnectionItem.AMFUENGAPID.Value))
				for _, ue := range ran.RanUePool {
					if ue.AmfUeNgapID == ueAssociatedLogicalNGConnectionItem.AMFUENGAPID.Value {
						ranUe = ue
						break
					}
				}
			} else if ueAssociatedLogicalNGConnectionItem.RANUENGAPID != nil {
				ran.Log.Debug("NG Reset with RANUENGAPID", zap.Int64("RanUeNgapID", ueAssociatedLogicalNGConnectionItem.RANUENGAPID.Value))
				ranUe = ran.RanUeFindByRanUeNgapID(ueAssociatedLogicalNGConnectionItem.RANUENGAPID.Value)
			}

			if ranUe == nil {
				ran.Log.Warn("Cannot not find UE Context")
				if ueAssociatedLogicalNGConnectionItem.AMFUENGAPID != nil {
					ran.Log.Warn("AMFUENGAPID is not empty", zap.Int64("AmfUeNgapID", ueAssociatedLogicalNGConnectionItem.AMFUENGAPID.Value))
				}
				if ueAssociatedLogicalNGConnectionItem.RANUENGAPID != nil {
					ran.Log.Warn("RANUENGAPID is not empty", zap.Int64("RanUeNgapID", ueAssociatedLogicalNGConnectionItem.RANUENGAPID.Value))
				}
			}

			err := ranUe.Remove()
			if err != nil {
				ran.Log.Error(err.Error())
			}
		}

		err := message.SendNGResetAcknowledge(ctx, ran, partOfNGInterface)
		if err != nil {
			ran.Log.Error("error sending NG Reset Acknowledge", zap.Error(err))
			return
		}
	default:
		ran.Log.Warn("Invalid ResetType", zap.Any("ResetType", resetType.Present))
	}
}
