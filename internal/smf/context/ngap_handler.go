// Copyright 2024 Ella Networks
// Copyright 2019 free5GC.org
// SPDX-License-Identifier: Apache-2.0

package context

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/free5gc/aper"
	"github.com/free5gc/ngap/ngapType"
	"go.uber.org/zap"
)

func HandlePDUSessionResourceSetupResponseTransfer(b []byte, ctx *SMContext) error {
	resourceSetupResponseTransfer := ngapType.PDUSessionResourceSetupResponseTransfer{}
	err := aper.UnmarshalWithParams(b, &resourceSetupResponseTransfer, "valueExt")
	if err != nil {
		return fmt.Errorf("failed to unmarshall resource setup response transfer: %s", err.Error())
	}

	QosFlowPerTNLInformation := resourceSetupResponseTransfer.DLQosFlowPerTNLInformation

	if QosFlowPerTNLInformation.UPTransportLayerInformation.Present !=
		ngapType.UPTransportLayerInformationPresentGTPTunnel {
		return fmt.Errorf("expected qos flow per tnl information up transport layer information present to be gtp tunnel")
	}

	gtpTunnel := QosFlowPerTNLInformation.UPTransportLayerInformation.GTPTunnel

	teid := binary.BigEndian.Uint32(gtpTunnel.GTPTEID.Value)

	ctx.Tunnel.ANInformation.IPAddress = gtpTunnel.TransportLayerAddress.Value.Bytes
	ctx.Tunnel.ANInformation.TEID = teid

	dataPath := ctx.Tunnel.DataPath
	if dataPath.Activated {
		ANUPF := dataPath.DPNode
		ANUPF.DownLinkTunnel.PDR.FAR.ForwardingParameters.OuterHeaderCreation = new(OuterHeaderCreation)
		dlOuterHeaderCreation := ANUPF.DownLinkTunnel.PDR.FAR.ForwardingParameters.OuterHeaderCreation
		dlOuterHeaderCreation.OuterHeaderCreationDescription = OuterHeaderCreationGtpUUdpIpv4
		dlOuterHeaderCreation.TeID = teid
		dlOuterHeaderCreation.IPv4Address = ctx.Tunnel.ANInformation.IPAddress.To4()
	}

	return nil
}

func HandlePDUSessionResourceSetupUnsuccessfulTransfer(b []byte) error {
	resourceSetupUnsuccessfulTransfer := ngapType.PDUSessionResourceSetupUnsuccessfulTransfer{}

	err := aper.UnmarshalWithParams(b, &resourceSetupUnsuccessfulTransfer, "valueExt")
	if err != nil {
		return fmt.Errorf("failed to unmarshall resource setup unsuccessful transfer: %s", err.Error())
	}

	switch resourceSetupUnsuccessfulTransfer.Cause.Present {
	case ngapType.CausePresentRadioNetwork:
		logger.SmfLog.Warn("PDU Session Resource Setup Unsuccessful by RadioNetwork", zap.String("Cause", radioNetworkCauseString(resourceSetupUnsuccessfulTransfer.Cause.RadioNetwork.Value)))
	case ngapType.CausePresentTransport:
		logger.SmfLog.Warn("PDU Session Resource Setup Unsuccessful by Transport", zap.String("Cause", transportCauseString(resourceSetupUnsuccessfulTransfer.Cause.Transport.Value)))
	case ngapType.CausePresentNas:
		logger.SmfLog.Warn("PDU Session Resource Setup Unsuccessful by NAS", zap.String("Cause", nasCauseString(resourceSetupUnsuccessfulTransfer.Cause.Nas.Value)))
	case ngapType.CausePresentProtocol:
		logger.SmfLog.Warn("PDU Session Resource Setup Unsuccessful by Protocol", zap.String("Cause", protocolCauseString(resourceSetupUnsuccessfulTransfer.Cause.Protocol.Value)))
	case ngapType.CausePresentMisc:
		logger.SmfLog.Warn("PDU Session Resource Setup Unsuccessful by Misc", zap.String("Cause", miscCauseString(resourceSetupUnsuccessfulTransfer.Cause.Misc.Value)))
	case ngapType.CausePresentChoiceExtensions:
		logger.SmfLog.Warn("PDU Session Resource Setup Unsuccessful by ChoiceExtensions", zap.Any("Cause", resourceSetupUnsuccessfulTransfer.Cause.ChoiceExtensions))
	}

	return nil
}

func HandlePathSwitchRequestTransfer(b []byte, ctx *SMContext) error {
	pathSwitchRequestTransfer := ngapType.PathSwitchRequestTransfer{}

	if err := aper.UnmarshalWithParams(b, &pathSwitchRequestTransfer, "valueExt"); err != nil {
		return err
	}

	if pathSwitchRequestTransfer.DLNGUUPTNLInformation.Present != ngapType.UPTransportLayerInformationPresentGTPTunnel {
		return errors.New("pathSwitchRequestTransfer.DLNGUUPTNLInformation.Present")
	}

	gtpTunnel := pathSwitchRequestTransfer.DLNGUUPTNLInformation.GTPTunnel

	teid := binary.BigEndian.Uint32(gtpTunnel.GTPTEID.Value)

	ctx.Tunnel.ANInformation.IPAddress = gtpTunnel.TransportLayerAddress.Value.Bytes
	ctx.Tunnel.ANInformation.TEID = teid
	dataPath := ctx.Tunnel.DataPath
	if dataPath.Activated {
		ANUPF := dataPath.DPNode
		ANUPF.DownLinkTunnel.PDR.FAR.ForwardingParameters.OuterHeaderCreation = new(OuterHeaderCreation)
		dlOuterHeaderCreation := ANUPF.DownLinkTunnel.PDR.FAR.ForwardingParameters.OuterHeaderCreation
		dlOuterHeaderCreation.OuterHeaderCreationDescription = OuterHeaderCreationGtpUUdpIpv4
		dlOuterHeaderCreation.TeID = teid
		dlOuterHeaderCreation.IPv4Address = gtpTunnel.TransportLayerAddress.Value.Bytes
		ANUPF.DownLinkTunnel.PDR.FAR.State = RuleUpdate
		ANUPF.DownLinkTunnel.PDR.FAR.ForwardingParameters.PFCPSMReqFlags = new(PFCPSMReqFlags)
		ANUPF.DownLinkTunnel.PDR.FAR.ForwardingParameters.PFCPSMReqFlags.Sndem = true
	}

	return nil
}

func HandlePathSwitchRequestSetupFailedTransfer(b []byte) error {
	pathSwitchRequestSetupFailedTransfer := ngapType.PathSwitchRequestSetupFailedTransfer{}
	err := aper.UnmarshalWithParams(b, &pathSwitchRequestSetupFailedTransfer, "valueExt")
	if err != nil {
		return fmt.Errorf("failed to unmarshall path switch request setup failed transfer: %s", err.Error())
	}
	return nil
}

func HandleHandoverRequiredTransfer(b []byte) error {
	handoverRequiredTransfer := ngapType.HandoverRequiredTransfer{}
	err := aper.UnmarshalWithParams(b, &handoverRequiredTransfer, "valueExt")
	if err != nil {
		return fmt.Errorf("failed to unmarshall handover required transfer: %s", err.Error())
	}
	return nil
}

func HandleHandoverRequestAcknowledgeTransfer(b []byte, ctx *SMContext) error {
	handoverRequestAcknowledgeTransfer := ngapType.HandoverRequestAcknowledgeTransfer{}

	err := aper.UnmarshalWithParams(b, &handoverRequestAcknowledgeTransfer, "valueExt")
	if err != nil {
		return fmt.Errorf("failed to unmarshall handover request acknowledge transfer: %s", err.Error())
	}
	DLNGUUPTNLInformation := handoverRequestAcknowledgeTransfer.DLNGUUPTNLInformation
	GTPTunnel := DLNGUUPTNLInformation.GTPTunnel
	TEIDReader := bytes.NewBuffer(GTPTunnel.GTPTEID.Value)

	teid, err := binary.ReadUvarint(TEIDReader)
	if err != nil {
		return fmt.Errorf("parse TEID error %s", err.Error())
	}

	dataPath := ctx.Tunnel.DataPath
	if dataPath.Activated {
		ANUPF := dataPath.DPNode
		ANUPF.DownLinkTunnel.PDR.FAR.ForwardingParameters.OuterHeaderCreation = new(OuterHeaderCreation)
		dlOuterHeaderCreation := ANUPF.DownLinkTunnel.PDR.FAR.ForwardingParameters.OuterHeaderCreation
		dlOuterHeaderCreation.OuterHeaderCreationDescription = OuterHeaderCreationGtpUUdpIpv4
		dlOuterHeaderCreation.TeID = uint32(teid)
		dlOuterHeaderCreation.IPv4Address = GTPTunnel.TransportLayerAddress.Value.Bytes
		ANUPF.DownLinkTunnel.PDR.FAR.State = RuleUpdate
	}

	return nil
}

func radioNetworkCauseString(cause aper.Enumerated) string {
	switch cause {
	case ngapType.CauseRadioNetworkPresentUnspecified:
		return "unspecified"
	case ngapType.CauseRadioNetworkPresentTxnrelocoverallExpiry:
		return "txNRelocOverallExpiry"
	case ngapType.CauseRadioNetworkPresentSuccessfulHandover:
		return "successfulHandover"
	case ngapType.CauseRadioNetworkPresentReleaseDueToNgranGeneratedReason:
		return "releaseDueToNgranGeneratedReason"
	case ngapType.CauseRadioNetworkPresentReleaseDueTo5gcGeneratedReason:
		return "releaseDueTo5gcGeneratedReason"
	case ngapType.CauseRadioNetworkPresentHandoverCancelled:
		return "handoverCancelled"
	case ngapType.CauseRadioNetworkPresentPartialHandover:
		return "partialHandover"
	case ngapType.CauseRadioNetworkPresentHoFailureInTarget5GCNgranNodeOrTargetSystem:
		return "hoFailureInTarget5GCNgranNodeOrTargetSystem"
	case ngapType.CauseRadioNetworkPresentHoTargetNotAllowed:
		return "hoTargetNotAllowed"
	case ngapType.CauseRadioNetworkPresentTngrelocoverallExpiry:
		return "tnGRelocOverallExpiry"
	case ngapType.CauseRadioNetworkPresentTngrelocprepExpiry:
		return "tnGRelocPrepExpiry"
	case ngapType.CauseRadioNetworkPresentCellNotAvailable:
		return "cellNotAvailable"
	case ngapType.CauseRadioNetworkPresentUnknownTargetID:
		return "unknownTargetID"
	case ngapType.CauseRadioNetworkPresentNoRadioResourcesAvailableInTargetCell:
		return "noRadioResourcesAvailableInTargetCell"
	case ngapType.CauseRadioNetworkPresentUnknownLocalUENGAPID:
		return "unknownLocalUENGAPID"
	case ngapType.CauseRadioNetworkPresentInconsistentRemoteUENGAPID:
		return "inconsistentRemoteUENGAPID"
	case ngapType.CauseRadioNetworkPresentHandoverDesirableForRadioReason:
		return "handoverDesirableForRadioReason"
	case ngapType.CauseRadioNetworkPresentTimeCriticalHandover:
		return "timeCriticalHandover"
	case ngapType.CauseRadioNetworkPresentResourceOptimisationHandover:
		return "resourceOptimisationHandover"
	case ngapType.CauseRadioNetworkPresentReduceLoadInServingCell:
		return "reduceLoadInServingCell"
	case ngapType.CauseRadioNetworkPresentUserInactivity:
		return "userInactivity"
	case ngapType.CauseRadioNetworkPresentRadioConnectionWithUeLost:
		return "radioConnectionWithUeLost"
	case ngapType.CauseRadioNetworkPresentRadioResourcesNotAvailable:
		return "radioResourcesNotAvailable"
	case ngapType.CauseRadioNetworkPresentInvalidQosCombination:
		return "invalidQosCombination"
	case ngapType.CauseRadioNetworkPresentFailureInRadioInterfaceProcedure:
		return "failureInRadioInterfaceProcedure"
	case ngapType.CauseRadioNetworkPresentInteractionWithOtherProcedure:
		return "interactionWithOtherProcedure"
	case ngapType.CauseRadioNetworkPresentUnknownPDUSessionID:
		return "unknownPDUSessionID"
	case ngapType.CauseRadioNetworkPresentUnkownQosFlowID:
		return "unkownQosFlowID"
	case ngapType.CauseRadioNetworkPresentMultiplePDUSessionIDInstances:
		return "multiplePDUSessionIDInstances"
	case ngapType.CauseRadioNetworkPresentMultipleQosFlowIDInstances:
		return "multipleQosFlowIDInstances"
	case ngapType.CauseRadioNetworkPresentEncryptionAndOrIntegrityProtectionAlgorithmsNotSupported:
		return "encryptionAndOrIntegrityProtectionAlgorithmsNotSupported"
	case ngapType.CauseRadioNetworkPresentNgIntraSystemHandoverTriggered:
		return "ngIntraSystemHandoverTriggered"
	case ngapType.CauseRadioNetworkPresentNgInterSystemHandoverTriggered:
		return "ngInterSystemHandoverTriggered"
	case ngapType.CauseRadioNetworkPresentXnHandoverTriggered:
		return "xnHandoverTriggered"
	case ngapType.CauseRadioNetworkPresentNotSupported5QIValue:
		return "notSupported5QIValue"
	case ngapType.CauseRadioNetworkPresentUeContextTransfer:
		return "ueContextTransfer"
	case ngapType.CauseRadioNetworkPresentImsVoiceEpsFallbackOrRatFallbackTriggered:
		return "imsVoiceEpsFallbackOrRatFallbackTriggered"
	case ngapType.CauseRadioNetworkPresentUpIntegrityProtectionNotPossible:
		return "upIntegrityProtectionNotPossible"
	case ngapType.CauseRadioNetworkPresentUpConfidentialityProtectionNotPossible:
		return "upConfidentialityProtectionNotPossible"
	case ngapType.CauseRadioNetworkPresentSliceNotSupported:
		return "sliceNotSupported"
	case ngapType.CauseRadioNetworkPresentUeInRrcInactiveStateNotReachable:
		return "ueInRrcInactiveStateNotReachable"
	case ngapType.CauseRadioNetworkPresentRedirection:
		return "redirection"
	case ngapType.CauseRadioNetworkPresentResourcesNotAvailableForTheSlice:
		return "resourcesNotAvailableForTheSlice"
	case ngapType.CauseRadioNetworkPresentUeMaxIntegrityProtectedDataRateReason:
		return "ueMaxIntegrityProtectedDataRateReason"
	case ngapType.CauseRadioNetworkPresentReleaseDueToCnDetectedMobility:
		return "releaseDueToCnDetectedMobility"
	case ngapType.CauseRadioNetworkPresentN26InterfaceNotAvailable:
		return "n26InterfaceNotAvailable"
	case ngapType.CauseRadioNetworkPresentReleaseDueToPreEmption:
		return "releaseDueToPreEmption"
	}

	return "unknown"
}

func transportCauseString(cause aper.Enumerated) string {
	switch cause {
	case ngapType.CauseTransportPresentTransportResourceUnavailable:
		return "transportResourceUnavailable"
	case ngapType.CauseTransportPresentUnspecified:
		return "unspecified"
	}

	return "unknown"
}

func nasCauseString(cause aper.Enumerated) string {
	switch cause {
	case ngapType.CauseNasPresentNormalRelease:
		return "normalRelease"
	case ngapType.CauseNasPresentAuthenticationFailure:
		return "authenticationFailure"
	case ngapType.CauseNasPresentDeregister:
		return "deregister"
	case ngapType.CauseNasPresentUnspecified:
		return "unspecified"
	}

	return "unknown"
}

func protocolCauseString(cause aper.Enumerated) string {
	switch cause {
	case ngapType.CauseProtocolPresentTransferSyntaxError:
		return "transferSyntaxError"
	case ngapType.CauseProtocolPresentAbstractSyntaxErrorReject:
		return "abstractSyntaxErrorReject"
	case ngapType.CauseProtocolPresentAbstractSyntaxErrorIgnoreAndNotify:
		return "abstractSyntaxErrorIgnoreAndNotify"
	case ngapType.CauseProtocolPresentMessageNotCompatibleWithReceiverState:
		return "messageNotCompatibleWithReceiverState"
	case ngapType.CauseProtocolPresentSemanticError:
		return "semanticError"
	case ngapType.CauseProtocolPresentAbstractSyntaxErrorFalselyConstructedMessage:
		return "abstractSyntaxErrorFalselyConstructedMessage"
	case ngapType.CauseProtocolPresentUnspecified:
		return "unspecified"
	}

	return "unknown"
}

func miscCauseString(cause aper.Enumerated) string {
	switch cause {
	case ngapType.CauseMiscPresentControlProcessingOverload:
		return "controlProcessingOverload"
	case ngapType.CauseMiscPresentNotEnoughUserPlaneProcessingResources:
		return "notEnoughUserPlaneProcessingResources"
	case ngapType.CauseMiscPresentHardwareFailure:
		return "hardwareFailure"
	case ngapType.CauseMiscPresentOmIntervention:
		return "omIntervention"
	case ngapType.CauseMiscPresentUnknownPLMN:
		return "unknownPLMN"
	case ngapType.CauseMiscPresentUnspecified:
		return "unspecified"
	}

	return "unknown"
}
