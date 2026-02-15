package ngap

import (
	"context"

	amfContext "github.com/ellanetworks/core/internal/amf/context"
	"github.com/free5gc/ngap/ngapType"
	"go.uber.org/zap"
)

// TS 23.502 4.9.1
func HandlePathSwitchRequest(ctx context.Context, amf *amfContext.AMF, ran *amfContext.Radio, msg *ngapType.PathSwitchRequest) {
	if msg == nil {
		ran.Log.Error("NGAP Message is nil")
		return
	}

	var (
		rANUENGAPID                            *ngapType.RANUENGAPID
		sourceAMFUENGAPID                      *ngapType.AMFUENGAPID
		userLocationInformation                *ngapType.UserLocationInformation
		uESecurityCapabilities                 *ngapType.UESecurityCapabilities
		pduSessionResourceToBeSwitchedInDLList *ngapType.PDUSessionResourceToBeSwitchedDLList
		pduSessionResourceFailedToSetupList    *ngapType.PDUSessionResourceFailedToSetupListPSReq
	)

	for _, ie := range msg.ProtocolIEs.List {
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

	ranUe := amf.FindRanUeByAmfUeNgapID(sourceAMFUENGAPID.Value)
	if ranUe == nil {
		ran.Log.Error("Cannot find UE from sourceAMfUeNgapID", zap.Int64("sourceAMFUENGAPID", sourceAMFUENGAPID.Value))

		err := ran.NGAPSender.SendPathSwitchRequestFailure(ctx, sourceAMFUENGAPID.Value, rANUENGAPID.Value, nil, nil)
		if err != nil {
			ran.Log.Error("error sending path switch request failure", zap.Error(err))
			return
		}

		ran.Log.Info("sent path switch request failure", zap.Int64("sourceAMFUENGAPID", sourceAMFUENGAPID.Value))

		return
	}

	ranUe.Radio = ran
	ranUe.Log.Debug("Handle Path Switch Request", zap.Int64("AmfUeNgapID", ranUe.AmfUeNgapID), zap.Int64("RanUeNgapID", ranUe.RanUeNgapID))

	amfUe := ranUe.AmfUe
	if amfUe == nil {
		ranUe.Log.Error("AmfUe is nil")

		err := ran.NGAPSender.SendPathSwitchRequestFailure(ctx, sourceAMFUENGAPID.Value, rANUENGAPID.Value, nil, nil)
		if err != nil {
			ranUe.Log.Error("error sending path switch request failure", zap.Error(err))
			return
		}

		ranUe.Log.Info("sent path switch request failure")

		return
	}

	if !amfUe.SecurityContextIsValid() {
		ranUe.Log.Error("No Security Context", zap.String("supi", amfUe.Supi))

		err := ran.NGAPSender.SendPathSwitchRequestFailure(ctx, sourceAMFUENGAPID.Value, rANUENGAPID.Value, nil, nil)
		if err != nil {
			ranUe.Log.Error("error sending path switch request failure", zap.Error(err))
			return
		}

		ranUe.Log.Info("sent path switch request failure", zap.String("supi", amfUe.Supi))

		return
	}

	err := amfUe.UpdateNH()
	if err != nil {
		ranUe.Log.Error("error updating NH", zap.Error(err))
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

	ranUe.UpdateLocation(ctx, amf, userLocationInformation)

	var (
		pduSessionResourceSwitchedList       ngapType.PDUSessionResourceSwitchedList
		pduSessionResourceReleasedListPSAck  ngapType.PDUSessionResourceReleasedListPSAck
		pduSessionResourceReleasedListPSFail ngapType.PDUSessionResourceReleasedListPSFail
	)

	if pduSessionResourceToBeSwitchedInDLList != nil {
		for _, item := range pduSessionResourceToBeSwitchedInDLList.List {
			pduSessionID := uint8(item.PDUSessionID.Value)
			transfer := item.PathSwitchRequestTransfer

			smContext, ok := amfUe.SmContextFindByPDUSessionID(pduSessionID)
			if !ok {
				ranUe.Log.Error("SmContext not found", zap.Uint8("PduSessionID", pduSessionID))
				continue
			}

			n2Rsp, err := amf.Smf.UpdateSmContextXnHandoverPathSwitchReq(ctx, smContext.Ref, transfer)
			if err != nil {
				ranUe.Log.Error("SendUpdateSmContextXnHandover[PathSwitchRequestTransfer] Error", zap.Error(err))
				continue
			}

			pduSessionResourceSwitchedItem := ngapType.PDUSessionResourceSwitchedItem{}
			pduSessionResourceSwitchedItem.PDUSessionID.Value = int64(pduSessionID)
			pduSessionResourceSwitchedItem.PathSwitchRequestAcknowledgeTransfer = n2Rsp
			pduSessionResourceSwitchedList.List = append(pduSessionResourceSwitchedList.List, pduSessionResourceSwitchedItem)
		}
	}

	if pduSessionResourceFailedToSetupList != nil {
		for _, item := range pduSessionResourceFailedToSetupList.List {
			pduSessionID := uint8(item.PDUSessionID.Value)
			transfer := item.PathSwitchRequestSetupFailedTransfer

			smContext, ok := amfUe.SmContextFindByPDUSessionID(pduSessionID)
			if !ok {
				ranUe.Log.Error("SmContext not found", zap.Uint8("PduSessionID", pduSessionID))
				continue
			}

			err := amf.Smf.UpdateSmContextHandoverFailed(smContext.Ref, transfer)
			if err != nil {
				ranUe.Log.Error("SendUpdateSmContextXnHandoverFailed[PathSwitchRequestSetupFailedTransfer] Error", zap.Error(err))
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

		operatorInfo, err := amf.GetOperatorInfo(ctx)
		if err != nil {
			ranUe.Log.Error("Get Operator Info Error", zap.Error(err))
			return
		}

		err = ranUe.Radio.NGAPSender.SendPathSwitchRequestAcknowledge(
			ctx,
			ranUe.AmfUeNgapID,
			ranUe.RanUeNgapID,
			amfUe.UESecurityCapability,
			amfUe.NCC,
			amfUe.NH,
			pduSessionResourceSwitchedList,
			pduSessionResourceReleasedListPSAck,
			operatorInfo.SupportedPLMN,
		)
		if err != nil {
			ranUe.Log.Error("error sending path switch request acknowledge", zap.Error(err))
			return
		}

		ranUe.Log.Info("sent path switch request acknowledge")
	} else if len(pduSessionResourceReleasedListPSFail.List) > 0 {
		err := ran.NGAPSender.SendPathSwitchRequestFailure(ctx, sourceAMFUENGAPID.Value, rANUENGAPID.Value, &pduSessionResourceReleasedListPSFail, nil)
		if err != nil {
			ranUe.Log.Error("error sending path switch request failure", zap.Error(err))
			return
		}

		ranUe.Log.Info("sent path switch request failure")
	} else {
		err := ran.NGAPSender.SendPathSwitchRequestFailure(ctx, sourceAMFUENGAPID.Value, rANUENGAPID.Value, nil, nil)
		if err != nil {
			ranUe.Log.Error("error sending path switch request failure", zap.Error(err))
			return
		}

		ranUe.Log.Info("sent path switch request failure")
	}
}
