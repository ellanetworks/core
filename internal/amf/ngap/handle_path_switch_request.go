package ngap

import (
	"context"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/free5gc/ngap/ngapType"
	"go.uber.org/zap"
)

// TS 23.502 4.9.1
func HandlePathSwitchRequest(ctx context.Context, amfInstance *amf.AMF, ran *amf.Radio, msg *ngapType.PathSwitchRequest) {
	if msg == nil {
		logger.WithTrace(ctx, ran.Log).Error("NGAP Message is nil")
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
				logger.WithTrace(ctx, ran.Log).Error("RanUeNgapID is nil")
				return
			}
		case ngapType.ProtocolIEIDSourceAMFUENGAPID: // reject
			sourceAMFUENGAPID = ie.Value.SourceAMFUENGAPID
			if sourceAMFUENGAPID == nil {
				logger.WithTrace(ctx, ran.Log).Error("SourceAmfUeNgapID is nil")
				return
			}
		case ngapType.ProtocolIEIDUserLocationInformation: // ignore
			userLocationInformation = ie.Value.UserLocationInformation
		case ngapType.ProtocolIEIDUESecurityCapabilities: // ignore
			uESecurityCapabilities = ie.Value.UESecurityCapabilities
		case ngapType.ProtocolIEIDPDUSessionResourceToBeSwitchedDLList: // reject
			pduSessionResourceToBeSwitchedInDLList = ie.Value.PDUSessionResourceToBeSwitchedDLList
			if pduSessionResourceToBeSwitchedInDLList == nil {
				logger.WithTrace(ctx, ran.Log).Error("PDUSessionResourceToBeSwitchedDLList is nil")
				return
			}
		case ngapType.ProtocolIEIDPDUSessionResourceFailedToSetupListPSReq: // ignore
			pduSessionResourceFailedToSetupList = ie.Value.PDUSessionResourceFailedToSetupListPSReq
		}
	}

	if sourceAMFUENGAPID == nil {
		logger.WithTrace(ctx, ran.Log).Error("SourceAmfUeNgapID is nil")
		return
	}

	if rANUENGAPID == nil {
		logger.WithTrace(ctx, ran.Log).Error("RANUENGAPID IE (mandatory) is missing in PathSwitchRequest")
		return
	}

	ranUe := amfInstance.FindRanUeByAmfUeNgapID(sourceAMFUENGAPID.Value)
	if ranUe == nil {
		logger.WithTrace(ctx, ran.Log).Error("Cannot find UE from sourceAMfUeNgapID", zap.Int64("sourceAMFUENGAPID", sourceAMFUENGAPID.Value))

		err := ran.NGAPSender.SendPathSwitchRequestFailure(ctx, sourceAMFUENGAPID.Value, rANUENGAPID.Value, nil, nil)
		if err != nil {
			logger.WithTrace(ctx, ran.Log).Error("error sending path switch request failure", zap.Error(err))
			return
		}

		logger.WithTrace(ctx, ran.Log).Info("sent path switch request failure", zap.Int64("sourceAMFUENGAPID", sourceAMFUENGAPID.Value))

		return
	}

	ranUe.Radio = ran
	ranUe.TouchLastSeen()
	logger.WithTrace(ctx, ranUe.Log).Debug("Handle Path Switch Request", zap.Int64("AmfUeNgapID", ranUe.AmfUeNgapID), zap.Int64("RanUeNgapID", ranUe.RanUeNgapID))

	amfUe := ranUe.AmfUe()
	if amfUe == nil {
		logger.WithTrace(ctx, ranUe.Log).Error("AmfUe is nil")

		err := ran.NGAPSender.SendPathSwitchRequestFailure(ctx, sourceAMFUENGAPID.Value, rANUENGAPID.Value, nil, nil)
		if err != nil {
			logger.WithTrace(ctx, ranUe.Log).Error("error sending path switch request failure", zap.Error(err))
			return
		}

		logger.WithTrace(ctx, ranUe.Log).Info("sent path switch request failure")

		return
	}

	if !amfUe.SecurityContextIsValid() {
		logger.WithTrace(ctx, ranUe.Log).Error("No Security Context", logger.SUPI(amfUe.Supi.String()))

		err := ran.NGAPSender.SendPathSwitchRequestFailure(ctx, sourceAMFUENGAPID.Value, rANUENGAPID.Value, nil, nil)
		if err != nil {
			logger.WithTrace(ctx, ranUe.Log).Error("error sending path switch request failure", zap.Error(err))
			return
		}

		logger.WithTrace(ctx, ranUe.Log).Info("sent path switch request failure", logger.SUPI(amfUe.Supi.String()))

		return
	}

	err := amfUe.UpdateNH()
	if err != nil {
		logger.WithTrace(ctx, ranUe.Log).Error("error updating NH", zap.Error(err))
		return
	}

	verifyUESecurityCapabilitiesOnPathSwitch(ctx, ranUe, amfUe, uESecurityCapabilities)

	ranUe.RanUeNgapID = rANUENGAPID.Value

	ranUe.UpdateLocation(ctx, amfInstance, userLocationInformation)

	var (
		pduSessionResourceSwitchedList       ngapType.PDUSessionResourceSwitchedList
		pduSessionResourceReleasedListPSAck  ngapType.PDUSessionResourceReleasedListPSAck
		pduSessionResourceReleasedListPSFail ngapType.PDUSessionResourceReleasedListPSFail
	)

	if pduSessionResourceToBeSwitchedInDLList != nil {
		for _, item := range pduSessionResourceToBeSwitchedInDLList.List {
			if item.PDUSessionID.Value < 1 || item.PDUSessionID.Value > 15 {
				logger.WithTrace(ctx, ranUe.Log).Error("invalid PDU session ID from gNB, skipping", zap.Int64("pduSessionID", item.PDUSessionID.Value))
				continue
			}

			pduSessionID := uint8(item.PDUSessionID.Value)
			transfer := item.PathSwitchRequestTransfer

			smContext, ok := amfUe.SmContextFindByPDUSessionID(pduSessionID)
			if !ok {
				logger.WithTrace(ctx, ranUe.Log).Error("SmContext not found", zap.Uint8("PduSessionID", pduSessionID))
				continue
			}

			n2Rsp, err := amfInstance.Smf.UpdateSmContextXnHandoverPathSwitchReq(ctx, smContext.Ref, transfer)
			if err != nil {
				logger.WithTrace(ctx, ranUe.Log).Error("SendUpdateSmContextXnHandover[PathSwitchRequestTransfer] Error", zap.Error(err))
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
			if item.PDUSessionID.Value < 1 || item.PDUSessionID.Value > 15 {
				logger.WithTrace(ctx, ranUe.Log).Error("invalid PDU session ID from gNB, skipping", zap.Int64("pduSessionID", item.PDUSessionID.Value))
				continue
			}

			pduSessionID := uint8(item.PDUSessionID.Value)
			transfer := item.PathSwitchRequestSetupFailedTransfer

			smContext, ok := amfUe.SmContextFindByPDUSessionID(pduSessionID)
			if !ok {
				logger.WithTrace(ctx, ranUe.Log).Error("SmContext not found", zap.Uint8("PduSessionID", pduSessionID))
				continue
			}

			err := amfInstance.Smf.UpdateSmContextHandoverFailed(ctx, smContext.Ref, transfer)
			if err != nil {
				logger.WithTrace(ctx, ranUe.Log).Error("SendUpdateSmContextXnHandoverFailed[PathSwitchRequestSetupFailedTransfer] Error", zap.Error(err))
			}
		}
	}

	// TS 23.502 4.9.1.2.2 step 7: send ack to Target NG-RAN. If none of the requested PDU Sessions have been switched
	// successfully, the AMF shall send an N2 Path Switch Request Failure message to the Target NG-RAN
	if len(pduSessionResourceSwitchedList.List) > 0 {
		err := ranUe.SwitchToRan(ran, rANUENGAPID.Value)
		if err != nil {
			logger.WithTrace(ctx, ranUe.Log).Error(err.Error())
			return
		}

		snssaiList, err := amfInstance.ListOperatorSnssai(ctx)
		if err != nil {
			logger.WithTrace(ctx, ranUe.Log).Error("List Operator SNSSAI Error", zap.Error(err))
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
			snssaiList,
		)
		if err != nil {
			logger.WithTrace(ctx, ranUe.Log).Error("error sending path switch request acknowledge", zap.Error(err))
			return
		}

		logger.WithTrace(ctx, ranUe.Log).Info("sent path switch request acknowledge")
	} else if len(pduSessionResourceReleasedListPSFail.List) > 0 {
		err := ran.NGAPSender.SendPathSwitchRequestFailure(ctx, sourceAMFUENGAPID.Value, rANUENGAPID.Value, &pduSessionResourceReleasedListPSFail, nil)
		if err != nil {
			logger.WithTrace(ctx, ranUe.Log).Error("error sending path switch request failure", zap.Error(err))
			return
		}

		logger.WithTrace(ctx, ranUe.Log).Info("sent path switch request failure")
	} else {
		err := ran.NGAPSender.SendPathSwitchRequestFailure(ctx, sourceAMFUENGAPID.Value, rANUENGAPID.Value, nil, nil)
		if err != nil {
			logger.WithTrace(ctx, ranUe.Log).Error("error sending path switch request failure", zap.Error(err))
			return
		}

		logger.WithTrace(ctx, ranUe.Log).Info("sent path switch request failure")
	}
}

// verifyUESecurityCapabilitiesOnPathSwitch compares the UE 5G security
// capabilities reported by the target gNB against the AMF's stored values
// per 3GPP TS 33.501 §6.7.3.1 and logs any mismatch.
func verifyUESecurityCapabilitiesOnPathSwitch(
	ctx context.Context,
	ranUe *amf.RanUe,
	amfUe *amf.AmfUe,
	received *ngapType.UESecurityCapabilities,
) {
	if received == nil {
		return
	}

	if amfUe.UESecurityCapability == nil {
		logger.WithTrace(ctx, ranUe.Log).Warn(
			"received UE security capabilities in PathSwitchRequest but AMF has no stored capabilities for this UE",
		)

		return
	}

	if len(received.NRencryptionAlgorithms.Value.Bytes) == 0 ||
		len(received.NRintegrityProtectionAlgorithms.Value.Bytes) == 0 {
		logger.WithTrace(ctx, ranUe.Log).Warn(
			"UE security capabilities from target gNB have empty NR algorithm bitstrings; ignoring and using locally stored values",
		)

		return
	}

	encByte := received.NRencryptionAlgorithms.Value.Bytes[0]
	intByte := received.NRintegrityProtectionAlgorithms.Value.Bytes[0]

	receivedEA1 := (encByte & 0x80) >> 7
	receivedEA2 := (encByte & 0x40) >> 6
	receivedEA3 := (encByte & 0x20) >> 5
	receivedIA1 := (intByte & 0x80) >> 7
	receivedIA2 := (intByte & 0x40) >> 6
	receivedIA3 := (intByte & 0x20) >> 5

	storedEA1 := amfUe.UESecurityCapability.GetEA1_128_5G()
	storedEA2 := amfUe.UESecurityCapability.GetEA2_128_5G()
	storedEA3 := amfUe.UESecurityCapability.GetEA3_128_5G()
	storedIA1 := amfUe.UESecurityCapability.GetIA1_128_5G()
	storedIA2 := amfUe.UESecurityCapability.GetIA2_128_5G()
	storedIA3 := amfUe.UESecurityCapability.GetIA3_128_5G()

	if receivedEA1 != storedEA1 || receivedEA2 != storedEA2 || receivedEA3 != storedEA3 ||
		receivedIA1 != storedIA1 || receivedIA2 != storedIA2 || receivedIA3 != storedIA3 {
		logger.WithTrace(ctx, ranUe.Log).Warn(
			"UE 5G security capabilities reported by target gNB differ from locally stored values; ignoring received values (TS 33.501 §6.7.3.1)",
			zap.Uint8("storedEA1", storedEA1),
			zap.Uint8("storedEA2", storedEA2),
			zap.Uint8("storedEA3", storedEA3),
			zap.Uint8("storedIA1", storedIA1),
			zap.Uint8("storedIA2", storedIA2),
			zap.Uint8("storedIA3", storedIA3),
			zap.Uint8("receivedEA1", receivedEA1),
			zap.Uint8("receivedEA2", receivedEA2),
			zap.Uint8("receivedEA3", receivedEA3),
			zap.Uint8("receivedIA1", receivedIA1),
			zap.Uint8("receivedIA2", receivedIA2),
			zap.Uint8("receivedIA3", receivedIA3),
		)
	}
}
