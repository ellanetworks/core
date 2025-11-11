package ngap

import (
	"fmt"

	"github.com/free5gc/ngap/ngapType"
)

func causeToString(cause ngapType.Cause) string {
	switch cause.Present {
	case ngapType.CausePresentRadioNetwork:
		return fmt.Sprintf("Radio Network: %s", radioNetworkCauseToString(*cause.RadioNetwork))
	case ngapType.CausePresentTransport:
		return fmt.Sprintf("Transport: %s", transportCauseToString(*cause.Transport))
	case ngapType.CausePresentNas:
		return fmt.Sprintf("NAS: %s", nasCauseToString(*cause.Nas))
	case ngapType.CausePresentProtocol:
		return fmt.Sprintf("Protocol: %s", protocolCauseToString(*cause.Protocol))
	case ngapType.CausePresentMisc:
		return fmt.Sprintf("Misc: %s", miscCauseToString(*cause.Misc))
	default:
		return fmt.Sprintf("Unknown cause present: %d", cause.Present)
	}
}

func radioNetworkCauseToString(cause ngapType.CauseRadioNetwork) string {
	switch cause.Value {
	case ngapType.CauseRadioNetworkPresentUnspecified:
		return fmt.Sprintf("Unspecified (%d)", cause.Value)
	case ngapType.CauseRadioNetworkPresentTxnrelocoverallExpiry:
		return fmt.Sprintf("TxNRelocOverallExpiry (%d)", cause.Value)
	case ngapType.CauseRadioNetworkPresentSuccessfulHandover:
		return fmt.Sprintf("SuccessfulHandover (%d)", cause.Value)
	case ngapType.CauseRadioNetworkPresentReleaseDueToNgranGeneratedReason:
		return fmt.Sprintf("ReleaseDueToNgranGeneratedReason (%d)", cause.Value)
	case ngapType.CauseRadioNetworkPresentReleaseDueTo5gcGeneratedReason:
		return fmt.Sprintf("ReleaseDueTo5gcGeneratedReason (%d)", cause.Value)
	case ngapType.CauseRadioNetworkPresentHandoverCancelled:
		return fmt.Sprintf("HandoverCancelled (%d)", cause.Value)
	case ngapType.CauseRadioNetworkPresentPartialHandover:
		return fmt.Sprintf("PartialHandover (%d)", cause.Value)
	case ngapType.CauseRadioNetworkPresentHoFailureInTarget5GCNgranNodeOrTargetSystem:
		return fmt.Sprintf("HoFailureInTarget5GCNgranNodeOrTargetSystem (%d)", cause.Value)
	case ngapType.CauseRadioNetworkPresentHoTargetNotAllowed:
		return fmt.Sprintf("HoTargetNotAllowed (%d)", cause.Value)
	case ngapType.CauseRadioNetworkPresentTngrelocoverallExpiry:
		return fmt.Sprintf("TngRelocOverallExpiry (%d)", cause.Value)
	case ngapType.CauseRadioNetworkPresentTngrelocprepExpiry:
		return fmt.Sprintf("TngRelocPrepExpiry (%d)", cause.Value)
	case ngapType.CauseRadioNetworkPresentCellNotAvailable:
		return fmt.Sprintf("CellNotAvailable (%d)", ngapType.CauseRadioNetworkPresentCellNotAvailable)
	case ngapType.CauseRadioNetworkPresentUnknownTargetID:
		return fmt.Sprintf("UnknownTargetID (%d)", ngapType.CauseRadioNetworkPresentUnknownTargetID)
	case ngapType.CauseRadioNetworkPresentNoRadioResourcesAvailableInTargetCell:
		return fmt.Sprintf("NoRadioResourcesAvailableInTargetCell (%d)", ngapType.CauseRadioNetworkPresentNoRadioResourcesAvailableInTargetCell)
	case ngapType.CauseRadioNetworkPresentUnknownLocalUENGAPID:
		return fmt.Sprintf("UnknownLocalUENGAPID (%d)", ngapType.CauseRadioNetworkPresentUnknownLocalUENGAPID)
	case ngapType.CauseRadioNetworkPresentInconsistentRemoteUENGAPID:
		return fmt.Sprintf("InconsistentRemoteUENGAPID (%d)", ngapType.CauseRadioNetworkPresentInconsistentRemoteUENGAPID)
	case ngapType.CauseRadioNetworkPresentHandoverDesirableForRadioReason:
		return fmt.Sprintf("HandoverDesirableForRadioReason (%d)", ngapType.CauseRadioNetworkPresentHandoverDesirableForRadioReason)
	case ngapType.CauseRadioNetworkPresentTimeCriticalHandover:
		return fmt.Sprintf("TimeCriticalHandover (%d)", ngapType.CauseRadioNetworkPresentTimeCriticalHandover)
	case ngapType.CauseRadioNetworkPresentResourceOptimisationHandover:
		return fmt.Sprintf("ResourceOptimisationHandover (%d)", ngapType.CauseRadioNetworkPresentResourceOptimisationHandover)
	case ngapType.CauseRadioNetworkPresentReduceLoadInServingCell:
		return fmt.Sprintf("ReduceLoadInServingCell (%d)", ngapType.CauseRadioNetworkPresentReduceLoadInServingCell)
	case ngapType.CauseRadioNetworkPresentUserInactivity:
		return fmt.Sprintf("UserInactivity (%d)", ngapType.CauseRadioNetworkPresentUserInactivity)
	case ngapType.CauseRadioNetworkPresentRadioConnectionWithUeLost:
		return fmt.Sprintf("RadioConnectionWithUeLost (%d)", ngapType.CauseRadioNetworkPresentRadioConnectionWithUeLost)
	case ngapType.CauseRadioNetworkPresentRadioResourcesNotAvailable:
		return fmt.Sprintf("RadioResourcesNotAvailable (%d)", ngapType.CauseRadioNetworkPresentRadioResourcesNotAvailable)
	case ngapType.CauseRadioNetworkPresentInvalidQosCombination:
		return fmt.Sprintf("InvalidQosCombination (%d)", ngapType.CauseRadioNetworkPresentInvalidQosCombination)
	case ngapType.CauseRadioNetworkPresentFailureInRadioInterfaceProcedure:
		return fmt.Sprintf("FailureInRadioInterfaceProcedure (%d)", ngapType.CauseRadioNetworkPresentFailureInRadioInterfaceProcedure)
	case ngapType.CauseRadioNetworkPresentInteractionWithOtherProcedure:
		return fmt.Sprintf("InteractionWithOtherProcedure (%d)", ngapType.CauseRadioNetworkPresentInteractionWithOtherProcedure)
	case ngapType.CauseRadioNetworkPresentUnknownPDUSessionID:
		return fmt.Sprintf("UnknownPDUSessionID (%d)", ngapType.CauseRadioNetworkPresentUnknownPDUSessionID)
	case ngapType.CauseRadioNetworkPresentUnkownQosFlowID:
		return fmt.Sprintf("UnkownQosFlowID (%d)", ngapType.CauseRadioNetworkPresentUnkownQosFlowID)
	case ngapType.CauseRadioNetworkPresentMultiplePDUSessionIDInstances:
		return fmt.Sprintf("MultiplePDUSessionIDInstances (%d)", ngapType.CauseRadioNetworkPresentMultiplePDUSessionIDInstances)
	case ngapType.CauseRadioNetworkPresentMultipleQosFlowIDInstances:
		return fmt.Sprintf("MultipleQosFlowIDInstances (%d)", ngapType.CauseRadioNetworkPresentMultipleQosFlowIDInstances)
	case ngapType.CauseRadioNetworkPresentEncryptionAndOrIntegrityProtectionAlgorithmsNotSupported:
		return fmt.Sprintf("EncryptionAndOrIntegrityProtectionAlgorithmsNotSupported (%d)", ngapType.CauseRadioNetworkPresentEncryptionAndOrIntegrityProtectionAlgorithmsNotSupported)
	case ngapType.CauseRadioNetworkPresentNgIntraSystemHandoverTriggered:
		return fmt.Sprintf("NgIntraSystemHandoverTriggered (%d)", ngapType.CauseRadioNetworkPresentNgIntraSystemHandoverTriggered)
	case ngapType.CauseRadioNetworkPresentNgInterSystemHandoverTriggered:
		return fmt.Sprintf("NgInterSystemHandoverTriggered (%d)", ngapType.CauseRadioNetworkPresentNgInterSystemHandoverTriggered)
	case ngapType.CauseRadioNetworkPresentXnHandoverTriggered:
		return fmt.Sprintf("XnHandoverTriggered (%d)", ngapType.CauseRadioNetworkPresentXnHandoverTriggered)
	case ngapType.CauseRadioNetworkPresentNotSupported5QIValue:
		return fmt.Sprintf("NotSupported5QIValue (%d)", ngapType.CauseRadioNetworkPresentNotSupported5QIValue)
	case ngapType.CauseRadioNetworkPresentUeContextTransfer:
		return fmt.Sprintf("UeContextTransfer (%d)", ngapType.CauseRadioNetworkPresentUeContextTransfer)
	case ngapType.CauseRadioNetworkPresentImsVoiceEpsFallbackOrRatFallbackTriggered:
		return fmt.Sprintf("ImsVoiceEpsFallbackOrRatFallbackTriggered (%d)", ngapType.CauseRadioNetworkPresentImsVoiceEpsFallbackOrRatFallbackTriggered)
	case ngapType.CauseRadioNetworkPresentUpIntegrityProtectionNotPossible:
		return fmt.Sprintf("UpIntegrityProtectionNotPossible (%d)", ngapType.CauseRadioNetworkPresentUpIntegrityProtectionNotPossible)
	case ngapType.CauseRadioNetworkPresentUpConfidentialityProtectionNotPossible:
		return fmt.Sprintf("UpConfidentialityProtectionNotPossible (%d)", ngapType.CauseRadioNetworkPresentUpConfidentialityProtectionNotPossible)
	case ngapType.CauseRadioNetworkPresentSliceNotSupported:
		return fmt.Sprintf("SliceNotSupported (%d)", ngapType.CauseRadioNetworkPresentSliceNotSupported)
	case ngapType.CauseRadioNetworkPresentUeInRrcInactiveStateNotReachable:
		return fmt.Sprintf("UeInRrcInactiveStateNotReachable (%d)", ngapType.CauseRadioNetworkPresentUeInRrcInactiveStateNotReachable)
	case ngapType.CauseRadioNetworkPresentRedirection:
		return fmt.Sprintf("Redirection (%d)", ngapType.CauseRadioNetworkPresentRedirection)
	case ngapType.CauseRadioNetworkPresentResourcesNotAvailableForTheSlice:
		return fmt.Sprintf("ResourcesNotAvailableForTheSlice (%d)", ngapType.CauseRadioNetworkPresentResourcesNotAvailableForTheSlice)
	case ngapType.CauseRadioNetworkPresentUeMaxIntegrityProtectedDataRateReason:
		return fmt.Sprintf("UeMaxIntegrityProtectedDataRateReason (%d)", ngapType.CauseRadioNetworkPresentUeMaxIntegrityProtectedDataRateReason)
	case ngapType.CauseRadioNetworkPresentReleaseDueToCnDetectedMobility:
		return fmt.Sprintf("ReleaseDueToCnDetectedMobility (%d)", ngapType.CauseRadioNetworkPresentReleaseDueToCnDetectedMobility)
	case ngapType.CauseRadioNetworkPresentN26InterfaceNotAvailable:
		return fmt.Sprintf("N26InterfaceNotAvailable (%d)", ngapType.CauseRadioNetworkPresentN26InterfaceNotAvailable)
	case ngapType.CauseRadioNetworkPresentReleaseDueToPreEmption:
		return fmt.Sprintf("ReleaseDueToPreEmption (%d)", ngapType.CauseRadioNetworkPresentReleaseDueToPreEmption)
	default:
		return fmt.Sprintf("Unknown Radio Network Cause: %d", cause.Value)
	}
}

func transportCauseToString(cause ngapType.CauseTransport) string {
	switch cause.Value {
	case ngapType.CauseTransportPresentTransportResourceUnavailable:
		return fmt.Sprintf("TransportResourceUnavailable (%d)", ngapType.CauseTransportPresentTransportResourceUnavailable)
	case ngapType.CauseTransportPresentUnspecified:
		return fmt.Sprintf("Unspecified (%d)", ngapType.CauseTransportPresentUnspecified)
	default:
		return fmt.Sprintf("Unknown Transport Cause: %d", cause.Value)
	}
}

func nasCauseToString(cause ngapType.CauseNas) string {
	switch cause.Value {
	case ngapType.CauseNasPresentNormalRelease:
		return fmt.Sprintf("NormalRelease (%d)", ngapType.CauseNasPresentNormalRelease)
	case ngapType.CauseNasPresentAuthenticationFailure:
		return fmt.Sprintf("AuthenticationFailure (%d)", ngapType.CauseNasPresentAuthenticationFailure)
	case ngapType.CauseNasPresentDeregister:
		return fmt.Sprintf("Deregister (%d)", ngapType.CauseNasPresentDeregister)
	case ngapType.CauseNasPresentUnspecified:
		return fmt.Sprintf("Unspecified (%d)", ngapType.CauseNasPresentUnspecified)
	default:
		return fmt.Sprintf("Unknown NAS Cause: %d", cause.Value)
	}
}

func protocolCauseToString(cause ngapType.CauseProtocol) string {
	switch cause.Value {
	case ngapType.CauseProtocolPresentTransferSyntaxError:
		return fmt.Sprintf("TransferSyntaxError (%d)", ngapType.CauseProtocolPresentTransferSyntaxError)
	case ngapType.CauseProtocolPresentAbstractSyntaxErrorReject:
		return fmt.Sprintf("AbstractSyntaxErrorReject (%d)", ngapType.CauseProtocolPresentAbstractSyntaxErrorReject)
	case ngapType.CauseProtocolPresentAbstractSyntaxErrorIgnoreAndNotify:
		return fmt.Sprintf("AbstractSyntaxErrorIgnoreAndNotify (%d)", ngapType.CauseProtocolPresentAbstractSyntaxErrorIgnoreAndNotify)
	case ngapType.CauseProtocolPresentMessageNotCompatibleWithReceiverState:
		return fmt.Sprintf("MessageNotCompatibleWithReceiverState (%d)", ngapType.CauseProtocolPresentMessageNotCompatibleWithReceiverState)
	case ngapType.CauseProtocolPresentSemanticError:
		return fmt.Sprintf("SemanticError (%d)", ngapType.CauseProtocolPresentSemanticError)
	case ngapType.CauseProtocolPresentAbstractSyntaxErrorFalselyConstructedMessage:
		return fmt.Sprintf("AbstractSyntaxErrorFalselyConstructedMessage (%d)", ngapType.CauseProtocolPresentAbstractSyntaxErrorFalselyConstructedMessage)
	case ngapType.CauseProtocolPresentUnspecified:
		return fmt.Sprintf("Unspecified (%d)", ngapType.CauseProtocolPresentUnspecified)
	default:
		return fmt.Sprintf("Unknown Protocol Cause: %d", cause.Value)
	}
}

func miscCauseToString(cause ngapType.CauseMisc) string {
	switch cause.Value {
	case ngapType.CauseMiscPresentControlProcessingOverload:
		return fmt.Sprintf("ControlProcessingOverload (%d)", ngapType.CauseMiscPresentControlProcessingOverload)
	case ngapType.CauseMiscPresentNotEnoughUserPlaneProcessingResources:
		return fmt.Sprintf("NotEnoughUserPlaneProcessingResources (%d)", ngapType.CauseMiscPresentNotEnoughUserPlaneProcessingResources)
	case ngapType.CauseMiscPresentHardwareFailure:
		return fmt.Sprintf("HardwareFailure (%d)", ngapType.CauseMiscPresentHardwareFailure)
	case ngapType.CauseMiscPresentOmIntervention:
		return fmt.Sprintf("OmIntervention (%d)", ngapType.CauseMiscPresentOmIntervention)
	case ngapType.CauseMiscPresentUnknownPLMN:
		return fmt.Sprintf("UnknownPLMN (%d)", ngapType.CauseMiscPresentUnknownPLMN)
	case ngapType.CauseMiscPresentUnspecified:
		return fmt.Sprintf("Unspecified (%d)", ngapType.CauseMiscPresentUnspecified)
	default:
		return fmt.Sprintf("Unknown Misc Cause: %d", cause.Value)
	}
}
