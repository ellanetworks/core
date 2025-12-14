package ngap

import (
	ctxt "context"

	"github.com/ellanetworks/core/internal/amf/consumer"
	"github.com/ellanetworks/core/internal/amf/context"
	"github.com/ellanetworks/core/internal/amf/ngap/message"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	"github.com/free5gc/ngap/ngapType"
	"go.uber.org/zap"
)

// TS 23.502 4.9.1
func HandlePathSwitchRequest(ctx ctxt.Context, ran *context.AmfRan, msg *ngapType.NGAPPDU) {
	var rANUENGAPID *ngapType.RANUENGAPID
	var sourceAMFUENGAPID *ngapType.AMFUENGAPID
	var userLocationInformation *ngapType.UserLocationInformation
	var uESecurityCapabilities *ngapType.UESecurityCapabilities
	var pduSessionResourceToBeSwitchedInDLList *ngapType.PDUSessionResourceToBeSwitchedDLList
	var pduSessionResourceFailedToSetupList *ngapType.PDUSessionResourceFailedToSetupListPSReq

	var ranUe *context.RanUe

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

	pathSwitchRequest := initiatingMessage.Value.PathSwitchRequest
	if pathSwitchRequest == nil {
		ran.Log.Error("PathSwitchRequest is nil")
		return
	}

	for _, ie := range pathSwitchRequest.ProtocolIEs.List {
		switch ie.Id.Value {
		case ngapType.ProtocolIEIDRANUENGAPID: // reject
			rANUENGAPID = ie.Value.RANUENGAPID
			if rANUENGAPID == nil {
				ran.Log.Error("RanUeNgapID is nil")
				return
			}
		case ngapType.ProtocolIEIDSourceAMFUENGAPID: // reject
			sourceAMFUENGAPID = ie.Value.SourceAMFUENGAPID
			if sourceAMFUENGAPID == nil {
				ran.Log.Error("SourceAmfUeNgapID is nil")
				return
			}
		case ngapType.ProtocolIEIDUserLocationInformation: // ignore
			userLocationInformation = ie.Value.UserLocationInformation
		case ngapType.ProtocolIEIDUESecurityCapabilities: // ignore
			uESecurityCapabilities = ie.Value.UESecurityCapabilities
		case ngapType.ProtocolIEIDPDUSessionResourceToBeSwitchedDLList: // reject
			pduSessionResourceToBeSwitchedInDLList = ie.Value.PDUSessionResourceToBeSwitchedDLList
			if pduSessionResourceToBeSwitchedInDLList == nil {
				ran.Log.Error("PDUSessionResourceToBeSwitchedDLList is nil")
				return
			}
		case ngapType.ProtocolIEIDPDUSessionResourceFailedToSetupListPSReq: // ignore
			pduSessionResourceFailedToSetupList = ie.Value.PDUSessionResourceFailedToSetupListPSReq
		}
	}

	if sourceAMFUENGAPID == nil {
		ran.Log.Error("SourceAmfUeNgapID is nil")
		return
	}
	ranUe = context.AMFSelf().RanUeFindByAmfUeNgapID(sourceAMFUENGAPID.Value)
	if ranUe == nil {
		ran.Log.Error("Cannot find UE from sourceAMfUeNgapID", zap.Int64("sourceAMFUENGAPID", sourceAMFUENGAPID.Value))
		err := message.SendPathSwitchRequestFailure(ctx, ran, sourceAMFUENGAPID.Value, rANUENGAPID.Value, nil, nil)
		if err != nil {
			ran.Log.Error("error sending path switch request failure", zap.Error(err))
			return
		}
		ran.Log.Info("sent path switch request failure", zap.Int64("sourceAMFUENGAPID", sourceAMFUENGAPID.Value))
		return
	}

	ranUe.Ran = ran
	ranUe.Log.Debug("Handle Path Switch Request", zap.Int64("AmfUeNgapID", ranUe.AmfUeNgapID), zap.Int64("RanUeNgapID", ranUe.RanUeNgapID))

	amfUe := ranUe.AmfUe
	if amfUe == nil {
		ranUe.Log.Error("AmfUe is nil")
		err := message.SendPathSwitchRequestFailure(ctx, ran, sourceAMFUENGAPID.Value, rANUENGAPID.Value, nil, nil)
		if err != nil {
			ranUe.Log.Error("error sending path switch request failure", zap.Error(err))
			return
		}
		ranUe.Log.Info("sent path switch request failure", zap.String("supi", amfUe.Supi))
		return
	}

	if amfUe.SecurityContextIsValid() {
		// Update NH
		amfUe.UpdateNH()
	} else {
		ranUe.Log.Error("No Security Context", zap.String("supi", amfUe.Supi))
		err := message.SendPathSwitchRequestFailure(ctx, ran, sourceAMFUENGAPID.Value, rANUENGAPID.Value, nil, nil)
		if err != nil {
			ranUe.Log.Error("error sending path switch request failure", zap.Error(err))
			return
		}
		ranUe.Log.Info("sent path switch request failure", zap.String("supi", amfUe.Supi))
		return
	}

	if uESecurityCapabilities != nil {
		amfUe.UESecurityCapability.SetEA1_128_5G((uESecurityCapabilities.NRencryptionAlgorithms.Value.Bytes[0] & 0x80) >> 7)
		amfUe.UESecurityCapability.SetEA2_128_5G((uESecurityCapabilities.NRencryptionAlgorithms.Value.Bytes[0] & 0x40) >> 6)
		amfUe.UESecurityCapability.SetEA3_128_5G((uESecurityCapabilities.NRencryptionAlgorithms.Value.Bytes[0] & 0x20) >> 5)
		amfUe.UESecurityCapability.SetIA1_128_5G((uESecurityCapabilities.NRintegrityProtectionAlgorithms.Value.Bytes[0] & 0x80) >> 7)
		amfUe.UESecurityCapability.SetIA2_128_5G((uESecurityCapabilities.NRintegrityProtectionAlgorithms.Value.Bytes[0] & 0x40) >> 6)
		amfUe.UESecurityCapability.SetIA3_128_5G((uESecurityCapabilities.NRintegrityProtectionAlgorithms.Value.Bytes[0] & 0x20) >> 5)
		// not support any E-UTRA algorithms
	}

	if rANUENGAPID != nil {
		ranUe.RanUeNgapID = rANUENGAPID.Value
	}

	ranUe.UpdateLocation(ctx, userLocationInformation)

	var pduSessionResourceSwitchedList ngapType.PDUSessionResourceSwitchedList
	var pduSessionResourceReleasedListPSAck ngapType.PDUSessionResourceReleasedListPSAck
	var pduSessionResourceReleasedListPSFail ngapType.PDUSessionResourceReleasedListPSFail

	if pduSessionResourceToBeSwitchedInDLList != nil {
		for _, item := range pduSessionResourceToBeSwitchedInDLList.List {
			pduSessionID := int32(item.PDUSessionID.Value)
			transfer := item.PathSwitchRequestTransfer
			smContext, ok := amfUe.SmContextFindByPDUSessionID(pduSessionID)
			if !ok {
				ranUe.Log.Error("SmContext not found", zap.Int32("PduSessionID", pduSessionID))
			}
			response, err := consumer.SendUpdateSmContextXnHandover(ctx, amfUe, smContext,
				models.N2SmInfoTypePathSwitchReq, transfer)
			if err != nil {
				ranUe.Log.Error("SendUpdateSmContextXnHandover[PathSwitchRequestTransfer] Error", zap.Error(err))
			}
			if response != nil && response.BinaryDataN2SmInformation != nil {
				pduSessionResourceSwitchedItem := ngapType.PDUSessionResourceSwitchedItem{}
				pduSessionResourceSwitchedItem.PDUSessionID.Value = int64(pduSessionID)
				pduSessionResourceSwitchedItem.PathSwitchRequestAcknowledgeTransfer = response.BinaryDataN2SmInformation
				pduSessionResourceSwitchedList.List = append(pduSessionResourceSwitchedList.List, pduSessionResourceSwitchedItem)
			}
		}
	}

	if pduSessionResourceFailedToSetupList != nil {
		for _, item := range pduSessionResourceFailedToSetupList.List {
			pduSessionID := int32(item.PDUSessionID.Value)
			transfer := item.PathSwitchRequestSetupFailedTransfer
			smContext, ok := amfUe.SmContextFindByPDUSessionID(pduSessionID)
			if !ok {
				ranUe.Log.Error("SmContext not found", zap.Int32("PduSessionID", pduSessionID))
			}
			response, err := consumer.SendUpdateSmContextXnHandoverFailed(ctx, amfUe, smContext,
				models.N2SmInfoTypePathSwitchSetupFail, transfer)
			if err != nil {
				ranUe.Log.Error("SendUpdateSmContextXnHandoverFailed[PathSwitchRequestSetupFailedTransfer] Error", zap.Error(err))
			}
			if response != nil && response.BinaryDataN2SmInformation != nil {
				pduSessionResourceReleasedItem := ngapType.PDUSessionResourceReleasedItemPSAck{}
				pduSessionResourceReleasedItem.PDUSessionID.Value = int64(pduSessionID)
				pduSessionResourceReleasedItem.PathSwitchRequestUnsuccessfulTransfer = response.BinaryDataN2SmInformation
				pduSessionResourceReleasedListPSAck.List = append(pduSessionResourceReleasedListPSAck.List,
					pduSessionResourceReleasedItem)
			}
		}
	}

	// TS 23.502 4.9.1.2.2 step 7: send ack to Target NG-RAN. If none of the requested PDU Sessions have been switched
	// successfully, the AMF shall send an N2 Path Switch Request Failure message to the Target NG-RAN
	if len(pduSessionResourceSwitchedList.List) > 0 {
		err := ranUe.SwitchToRan(ran, rANUENGAPID.Value)
		if err != nil {
			ranUe.Log.Error(err.Error())
			return
		}
		operatorInfo, err := context.GetOperatorInfo(ctx)
		if err != nil {
			ranUe.Log.Error("Get Operator Info Error", zap.Error(err))
			return
		}
		err = message.SendPathSwitchRequestAcknowledge(ctx, ranUe, pduSessionResourceSwitchedList, pduSessionResourceReleasedListPSAck, false, nil, nil, nil, operatorInfo.SupportedPLMN)
		if err != nil {
			ranUe.Log.Error("error sending path switch request acknowledge", zap.Error(err))
			return
		}
		ranUe.Log.Info("sent path switch request acknowledge")
	} else if len(pduSessionResourceReleasedListPSFail.List) > 0 {
		err := message.SendPathSwitchRequestFailure(ctx, ran, sourceAMFUENGAPID.Value, rANUENGAPID.Value, &pduSessionResourceReleasedListPSFail, nil)
		if err != nil {
			ranUe.Log.Error("error sending path switch request failure", zap.Error(err))
			return
		}
		ranUe.Log.Info("sent path switch request failure")
	} else {
		err := message.SendPathSwitchRequestFailure(ctx, ran, sourceAMFUENGAPID.Value, rANUENGAPID.Value, nil, nil)
		if err != nil {
			ranUe.Log.Error("error sending path switch request failure", zap.Error(err))
			return
		}
		ranUe.Log.Info("sent path switch request failure")
	}
}
