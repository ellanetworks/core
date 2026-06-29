// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

// Package ngapcause renders an NGAP Cause as a human-readable "<group>: <name>
// (<value>)" string for logging (TS 38.413 §9.3.1.2). It mirrors the 4G
// internal/s1apcause package.
package ngapcause

import (
	"fmt"

	"github.com/free5gc/ngap/ngapType"
)

// CauseToString renders an NGAP Cause as "<group>: <name> (<value>)".
func CauseToString(cause ngapType.Cause) string {
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
		return fmt.Sprintf("CellNotAvailable (%d)", cause.Value)
	case ngapType.CauseRadioNetworkPresentUnknownTargetID:
		return fmt.Sprintf("UnknownTargetID (%d)", cause.Value)
	case ngapType.CauseRadioNetworkPresentNoRadioResourcesAvailableInTargetCell:
		return fmt.Sprintf("NoRadioResourcesAvailableInTargetCell (%d)", cause.Value)
	case ngapType.CauseRadioNetworkPresentUnknownLocalUENGAPID:
		return fmt.Sprintf("UnknownLocalUENGAPID (%d)", cause.Value)
	case ngapType.CauseRadioNetworkPresentInconsistentRemoteUENGAPID:
		return fmt.Sprintf("InconsistentRemoteUENGAPID (%d)", cause.Value)
	case ngapType.CauseRadioNetworkPresentHandoverDesirableForRadioReason:
		return fmt.Sprintf("HandoverDesirableForRadioReason (%d)", cause.Value)
	case ngapType.CauseRadioNetworkPresentTimeCriticalHandover:
		return fmt.Sprintf("TimeCriticalHandover (%d)", cause.Value)
	case ngapType.CauseRadioNetworkPresentResourceOptimisationHandover:
		return fmt.Sprintf("ResourceOptimisationHandover (%d)", cause.Value)
	case ngapType.CauseRadioNetworkPresentReduceLoadInServingCell:
		return fmt.Sprintf("ReduceLoadInServingCell (%d)", cause.Value)
	case ngapType.CauseRadioNetworkPresentUserInactivity:
		return fmt.Sprintf("UserInactivity (%d)", cause.Value)
	case ngapType.CauseRadioNetworkPresentRadioConnectionWithUeLost:
		return fmt.Sprintf("RadioConnectionWithUeLost (%d)", cause.Value)
	case ngapType.CauseRadioNetworkPresentRadioResourcesNotAvailable:
		return fmt.Sprintf("RadioResourcesNotAvailable (%d)", cause.Value)
	case ngapType.CauseRadioNetworkPresentInvalidQosCombination:
		return fmt.Sprintf("InvalidQosCombination (%d)", cause.Value)
	case ngapType.CauseRadioNetworkPresentFailureInRadioInterfaceProcedure:
		return fmt.Sprintf("FailureInRadioInterfaceProcedure (%d)", cause.Value)
	case ngapType.CauseRadioNetworkPresentInteractionWithOtherProcedure:
		return fmt.Sprintf("InteractionWithOtherProcedure (%d)", cause.Value)
	case ngapType.CauseRadioNetworkPresentUnknownPDUSessionID:
		return fmt.Sprintf("UnknownPDUSessionID (%d)", cause.Value)
	case ngapType.CauseRadioNetworkPresentUnkownQosFlowID:
		return fmt.Sprintf("UnkownQosFlowID (%d)", cause.Value)
	case ngapType.CauseRadioNetworkPresentMultiplePDUSessionIDInstances:
		return fmt.Sprintf("MultiplePDUSessionIDInstances (%d)", cause.Value)
	case ngapType.CauseRadioNetworkPresentMultipleQosFlowIDInstances:
		return fmt.Sprintf("MultipleQosFlowIDInstances (%d)", cause.Value)
	case ngapType.CauseRadioNetworkPresentEncryptionAndOrIntegrityProtectionAlgorithmsNotSupported:
		return fmt.Sprintf("EncryptionAndOrIntegrityProtectionAlgorithmsNotSupported (%d)", cause.Value)
	case ngapType.CauseRadioNetworkPresentNgIntraSystemHandoverTriggered:
		return fmt.Sprintf("NgIntraSystemHandoverTriggered (%d)", cause.Value)
	case ngapType.CauseRadioNetworkPresentNgInterSystemHandoverTriggered:
		return fmt.Sprintf("NgInterSystemHandoverTriggered (%d)", cause.Value)
	case ngapType.CauseRadioNetworkPresentXnHandoverTriggered:
		return fmt.Sprintf("XnHandoverTriggered (%d)", cause.Value)
	case ngapType.CauseRadioNetworkPresentNotSupported5QIValue:
		return fmt.Sprintf("NotSupported5QIValue (%d)", cause.Value)
	case ngapType.CauseRadioNetworkPresentUeContextTransfer:
		return fmt.Sprintf("UeContextTransfer (%d)", cause.Value)
	case ngapType.CauseRadioNetworkPresentImsVoiceEpsFallbackOrRatFallbackTriggered:
		return fmt.Sprintf("ImsVoiceEpsFallbackOrRatFallbackTriggered (%d)", cause.Value)
	case ngapType.CauseRadioNetworkPresentUpIntegrityProtectionNotPossible:
		return fmt.Sprintf("UpIntegrityProtectionNotPossible (%d)", cause.Value)
	case ngapType.CauseRadioNetworkPresentUpConfidentialityProtectionNotPossible:
		return fmt.Sprintf("UpConfidentialityProtectionNotPossible (%d)", cause.Value)
	case ngapType.CauseRadioNetworkPresentSliceNotSupported:
		return fmt.Sprintf("SliceNotSupported (%d)", cause.Value)
	case ngapType.CauseRadioNetworkPresentUeInRrcInactiveStateNotReachable:
		return fmt.Sprintf("UeInRrcInactiveStateNotReachable (%d)", cause.Value)
	case ngapType.CauseRadioNetworkPresentRedirection:
		return fmt.Sprintf("Redirection (%d)", cause.Value)
	case ngapType.CauseRadioNetworkPresentResourcesNotAvailableForTheSlice:
		return fmt.Sprintf("ResourcesNotAvailableForTheSlice (%d)", cause.Value)
	case ngapType.CauseRadioNetworkPresentUeMaxIntegrityProtectedDataRateReason:
		return fmt.Sprintf("UeMaxIntegrityProtectedDataRateReason (%d)", cause.Value)
	case ngapType.CauseRadioNetworkPresentReleaseDueToCnDetectedMobility:
		return fmt.Sprintf("ReleaseDueToCnDetectedMobility (%d)", cause.Value)
	case ngapType.CauseRadioNetworkPresentN26InterfaceNotAvailable:
		return fmt.Sprintf("N26InterfaceNotAvailable (%d)", cause.Value)
	case ngapType.CauseRadioNetworkPresentReleaseDueToPreEmption:
		return fmt.Sprintf("ReleaseDueToPreEmption (%d)", cause.Value)
	default:
		return fmt.Sprintf("Unknown Radio Network Cause: %d", cause.Value)
	}
}

func transportCauseToString(cause ngapType.CauseTransport) string {
	switch cause.Value {
	case ngapType.CauseTransportPresentTransportResourceUnavailable:
		return fmt.Sprintf("TransportResourceUnavailable (%d)", cause.Value)
	case ngapType.CauseTransportPresentUnspecified:
		return fmt.Sprintf("Unspecified (%d)", cause.Value)
	default:
		return fmt.Sprintf("Unknown Transport Cause: %d", cause.Value)
	}
}

func nasCauseToString(cause ngapType.CauseNas) string {
	switch cause.Value {
	case ngapType.CauseNasPresentNormalRelease:
		return fmt.Sprintf("NormalRelease (%d)", cause.Value)
	case ngapType.CauseNasPresentAuthenticationFailure:
		return fmt.Sprintf("AuthenticationFailure (%d)", cause.Value)
	case ngapType.CauseNasPresentDeregister:
		return fmt.Sprintf("Deregister (%d)", cause.Value)
	case ngapType.CauseNasPresentUnspecified:
		return fmt.Sprintf("Unspecified (%d)", cause.Value)
	default:
		return fmt.Sprintf("Unknown NAS Cause: %d", cause.Value)
	}
}

func protocolCauseToString(cause ngapType.CauseProtocol) string {
	switch cause.Value {
	case ngapType.CauseProtocolPresentTransferSyntaxError:
		return fmt.Sprintf("TransferSyntaxError (%d)", cause.Value)
	case ngapType.CauseProtocolPresentAbstractSyntaxErrorReject:
		return fmt.Sprintf("AbstractSyntaxErrorReject (%d)", cause.Value)
	case ngapType.CauseProtocolPresentAbstractSyntaxErrorIgnoreAndNotify:
		return fmt.Sprintf("AbstractSyntaxErrorIgnoreAndNotify (%d)", cause.Value)
	case ngapType.CauseProtocolPresentMessageNotCompatibleWithReceiverState:
		return fmt.Sprintf("MessageNotCompatibleWithReceiverState (%d)", cause.Value)
	case ngapType.CauseProtocolPresentSemanticError:
		return fmt.Sprintf("SemanticError (%d)", cause.Value)
	case ngapType.CauseProtocolPresentAbstractSyntaxErrorFalselyConstructedMessage:
		return fmt.Sprintf("AbstractSyntaxErrorFalselyConstructedMessage (%d)", cause.Value)
	case ngapType.CauseProtocolPresentUnspecified:
		return fmt.Sprintf("Unspecified (%d)", cause.Value)
	default:
		return fmt.Sprintf("Unknown Protocol Cause: %d", cause.Value)
	}
}

func miscCauseToString(cause ngapType.CauseMisc) string {
	switch cause.Value {
	case ngapType.CauseMiscPresentControlProcessingOverload:
		return fmt.Sprintf("ControlProcessingOverload (%d)", cause.Value)
	case ngapType.CauseMiscPresentNotEnoughUserPlaneProcessingResources:
		return fmt.Sprintf("NotEnoughUserPlaneProcessingResources (%d)", cause.Value)
	case ngapType.CauseMiscPresentHardwareFailure:
		return fmt.Sprintf("HardwareFailure (%d)", cause.Value)
	case ngapType.CauseMiscPresentOmIntervention:
		return fmt.Sprintf("OmIntervention (%d)", cause.Value)
	case ngapType.CauseMiscPresentUnknownPLMN:
		return fmt.Sprintf("UnknownPLMN (%d)", cause.Value)
	case ngapType.CauseMiscPresentUnspecified:
		return fmt.Sprintf("Unspecified (%d)", cause.Value)
	default:
		return fmt.Sprintf("Unknown Misc Cause: %d", cause.Value)
	}
}
