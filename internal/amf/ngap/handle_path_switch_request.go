package ngap

import (
	"context"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/free5gc/nas/nasType"
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
// capabilities reported by the target gNB against the AMF's stored
// values via the VerifyUESecurityCapability accessor and logs any
// mismatch (TS 33.501 §6.7.3.1). It never mutates amfUe — stored values
// are preserved by construction because the handler has no AuthProof.
func verifyUESecurityCapabilitiesOnPathSwitch(
	ctx context.Context,
	ranUe *amf.RanUe,
	amfUe *amf.AmfUe,
	received *ngapType.UESecurityCapabilities,
) {
	if received == nil {
		return
	}

	if len(received.NRencryptionAlgorithms.Value.Bytes) == 0 ||
		len(received.NRintegrityProtectionAlgorithms.Value.Bytes) == 0 {
		logger.WithTrace(ctx, ranUe.Log).Warn(
			"UE security capabilities from target gNB have empty NR algorithm bitstrings; ignoring and using locally stored values",
		)

		return
	}

	reported := ngapToNasUESecurityCapability(received)

	switch amfUe.VerifyUESecurityCapability(reported) {
	case amf.VerifyMatch:
		return
	case amf.VerifyNoStoredValue:
		logger.WithTrace(ctx, ranUe.Log).Warn(
			"received UE security capabilities in PathSwitchRequest but AMF has no stored capabilities for this UE",
		)

		return
	case amf.VerifyMismatch:
		logger.WithTrace(ctx, ranUe.Log).Warn(
			"UE 5G security capabilities reported by target gNB differ from locally stored values; ignoring received values (TS 33.501 §6.7.3.1)",
			zap.Binary("stored", amfUe.UESecurityCapability.Buffer),
			zap.Binary("received", reported.Buffer),
		)
	}
}

// ngapToNasUESecurityCapability converts the NGAP UESecurityCapabilities
// IE to the NAS UESecurityCapability type the AMF stores internally,
// using the common "first byte only" encoding (EA/IA 1..3 + EIA0).
//
// E-UTRA (EEA/EIA) bits carried by the NGAP IE are intentionally
// dropped: this AMF does not negotiate E-UTRA algorithms with the UE,
// so the verify path compares only the 5G NR columns. This matches the
// legacy behaviour of the pre-refactor handler.
func ngapToNasUESecurityCapability(received *ngapType.UESecurityCapabilities) *nasType.UESecurityCapability {
	out := &nasType.UESecurityCapability{}
	out.SetLen(2)

	encByte := received.NRencryptionAlgorithms.Value.Bytes[0]
	intByte := received.NRintegrityProtectionAlgorithms.Value.Bytes[0]

	out.SetEA1_128_5G((encByte & 0x80) >> 7)
	out.SetEA2_128_5G((encByte & 0x40) >> 6)
	out.SetEA3_128_5G((encByte & 0x20) >> 5)
	out.SetIA1_128_5G((intByte & 0x80) >> 7)
	out.SetIA2_128_5G((intByte & 0x40) >> 6)
	out.SetIA3_128_5G((intByte & 0x20) >> 5)

	return out
}
