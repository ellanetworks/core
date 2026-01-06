package pdusession

import (
	"fmt"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/smf/context"
	"github.com/free5gc/aper"
	"github.com/free5gc/ngap/ngapType"
	"go.uber.org/zap"
)

func UpdateSmContextN2InfoPduResSetupFail(smContextRef string, n2Data []byte) error {
	if smContextRef == "" {
		return fmt.Errorf("SM Context reference is missing")
	}

	smf := context.SMFSelf()

	smContext := smf.GetSMContext(smContextRef)
	if smContext == nil {
		return fmt.Errorf("sm context not found: %s", smContextRef)
	}

	err := handlePDUSessionResourceSetupUnsuccessfulTransfer(n2Data)
	if err != nil {
		return fmt.Errorf("handle PDUSessionResourceSetupUnsuccessfulTransfer failed: %v", err)
	}

	return nil
}

func handlePDUSessionResourceSetupUnsuccessfulTransfer(b []byte) error {
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
