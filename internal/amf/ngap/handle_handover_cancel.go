package ngap

import (
	ctxt "context"

	"github.com/ellanetworks/core/internal/amf/consumer"
	"github.com/ellanetworks/core/internal/amf/context"
	"github.com/ellanetworks/core/internal/amf/ngap/message"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/free5gc/ngap/ngapType"
	"go.uber.org/zap"
)

func HandleHandoverCancel(ctx ctxt.Context, ran *context.AmfRan, msg *ngapType.NGAPPDU) {
	var aMFUENGAPID *ngapType.AMFUENGAPID
	var rANUENGAPID *ngapType.RANUENGAPID
	var cause *ngapType.Cause

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
	HandoverCancel := initiatingMessage.Value.HandoverCancel
	if HandoverCancel == nil {
		ran.Log.Error("Handover Cancel is nil")
		return
	}

	for i := 0; i < len(HandoverCancel.ProtocolIEs.List); i++ {
		ie := HandoverCancel.ProtocolIEs.List[i]
		switch ie.Id.Value {
		case ngapType.ProtocolIEIDAMFUENGAPID:
			aMFUENGAPID = ie.Value.AMFUENGAPID
			if aMFUENGAPID == nil {
				ran.Log.Error("AMFUENGAPID is nil")
				return
			}
		case ngapType.ProtocolIEIDRANUENGAPID:
			rANUENGAPID = ie.Value.RANUENGAPID
			if rANUENGAPID == nil {
				ran.Log.Error("RANUENGAPID is nil")
				return
			}
		case ngapType.ProtocolIEIDCause:
			cause = ie.Value.Cause
			if cause == nil {
				ran.Log.Error("Cause is nil")
				return
			}
		}
	}

	sourceUe := ran.RanUeFindByRanUeNgapID(rANUENGAPID.Value)
	if sourceUe == nil {
		ran.Log.Error("No UE Context", zap.Int64("RanUeNgapID", rANUENGAPID.Value))
		cause := ngapType.Cause{
			Present: ngapType.CausePresentRadioNetwork,
			RadioNetwork: &ngapType.CauseRadioNetwork{
				Value: ngapType.CauseRadioNetworkPresentUnknownLocalUENGAPID,
			},
		}
		err := message.SendErrorIndication(ctx, ran, nil, nil, &cause, nil)
		if err != nil {
			ran.Log.Error("error sending error indication", zap.Error(err), zap.Int64("RAN_UE_NGAP_ID", rANUENGAPID.Value))
			return
		}
		ran.Log.Info("sent error indication to source UE")
		return
	}

	if sourceUe.AmfUeNgapID != aMFUENGAPID.Value {
		ran.Log.Warn("Conflict AMF_UE_NGAP_ID", zap.Int64("sourceUe.AmfUeNgapID", sourceUe.AmfUeNgapID), zap.Int64("aMFUENGAPID.Value", aMFUENGAPID.Value))
	}
	ran.Log.Debug("Handle Handover Cancel", zap.Int64("sourceRanUeNgapID", sourceUe.RanUeNgapID), zap.Int64("sourceAmfUeNgapID", sourceUe.AmfUeNgapID))
	causePresent := ngapType.CausePresentRadioNetwork
	causeValue := ngapType.CauseRadioNetworkPresentHoFailureInTarget5GCNgranNodeOrTargetSystem
	var err error
	if cause != nil {
		ran.Log.Debug("Handover Cancel Cause", zap.String("Cause", causeToString(*cause)))
		causePresent, causeValue, err = getCause(cause)
		if err != nil {
			ran.Log.Error("Get Cause from Handover Failure Error", zap.Error(err))
			return
		}
	}
	targetUe := sourceUe.TargetUe
	if targetUe == nil {
		ran.Log.Error("N2 Handover between AMF has not been implemented yet")
	} else {
		ran.Log.Debug("handle handover cancel", zap.Int64("targetRanUeNgapID", targetUe.RanUeNgapID), zap.Int64("targetAmfUeNgapID", targetUe.AmfUeNgapID),
			zap.Int64("sourceRanUeNgapID", sourceUe.RanUeNgapID), zap.Int64("sourceAmfUeNgapID", sourceUe.AmfUeNgapID))
		amfUe := sourceUe.AmfUe
		if amfUe != nil {
			amfUe.Mutex.Lock()
			for pduSessionID, smContext := range amfUe.SmContextList {
				_, err := consumer.SendUpdateSmContextN2HandoverCanceled(ctx, amfUe, smContext)
				if err != nil {
					sourceUe.Log.Error("Send UpdateSmContextN2HandoverCanceled Error", zap.Error(err), zap.Int32("PduSessionID", pduSessionID))
				}
			}
			amfUe.Mutex.Unlock()
		}
		err := message.SendUEContextReleaseCommand(ctx, targetUe, context.UeContextReleaseHandover, causePresent, causeValue)
		if err != nil {
			ran.Log.Error("error sending UE Context Release Command to target UE", zap.Error(err))
			return
		}
		err = message.SendHandoverCancelAcknowledge(ctx, sourceUe, nil)
		if err != nil {
			ran.Log.Error("error sending handover cancel acknowledge to source UE", zap.Error(err))
			return
		}
		ran.Log.Info("sent handover cancel acknowledge to source UE")
	}
}
