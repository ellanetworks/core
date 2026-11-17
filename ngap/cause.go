// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import "fmt"

// Cause ::= CHOICE of five extensible ENUMERATED groups plus a
// choice-Extensions alternative.
type Cause struct {
	RadioNetwork    *CauseRadioNetwork `per:",choice:0"`
	Transport       *CauseTransport    `per:",choice:1"`
	NAS             *CauseNas          `per:",choice:2"`
	Protocol        *CauseProtocol     `per:",choice:3"`
	Misc            *CauseMisc         `per:",choice:4"`
	ChoiceExtension *SingleContainer   `per:",choice:5,extie"`
}

type CauseRadioNetwork uint8

const (
	CauseRadioNetworkUnspecified CauseRadioNetwork = iota // unspecified
	CauseRadioNetworkTXnRelocOverallExpiry
	CauseRadioNetworkSuccessfulHandover               // successful-handover
	CauseRadioNetworkReleaseDueToNGRANGeneratedReason // release-due-to-ngran-generated-reason
	CauseRadioNetworkReleaseDueTo5GCGeneratedReason   // release-due-to-5gc-generated-reason
	CauseRadioNetworkHandoverCancelled                // handover-cancelled
	CauseRadioNetworkPartialHandover
	CauseRadioNetworkHOFailureInTarget     // ho-failure-in-target-5GC-ngran-node-or-target-system
	CauseRadioNetworkHoTargetNotAllowed    // ho-target-not-allowed
	CauseRadioNetworkTNGRelocOverallExpiry // tngrelocoverall-expiry
	CauseRadioNetworkTNGRelocPrepExpiry
	CauseRadioNetworkCellNotAvailable
	CauseRadioNetworkUnknownTargetID              // unknown-targetID
	CauseRadioNetworkNoRadioResourcesInTargetCell // no-radio-resources-available-in-target-cell
	CauseRadioNetworkUnknownLocalUENGAPID         // unknown-local-UE-NGAP-ID
	CauseRadioNetworkInconsistentRemoteUEID       // inconsistent-remote-UE-NGAP-ID
	CauseRadioNetworkHandoverDesirableForRadio    // handover-desirable-for-radio-reason
	CauseRadioNetworkTimeCriticalHandover         // time-critical-handover
	CauseRadioNetworkResourceOptimisationHandover // resource-optimisation-handover
	CauseRadioNetworkReduceLoadInServingCell      // reduce-load-in-serving-cell
	CauseRadioNetworkUserInactivity               // user-inactivity
	CauseRadioNetworkRadioConnectionWithUELost    // radio-connection-with-ue-lost
	CauseRadioNetworkRadioResourcesNotAvailable   // radio-resources-not-available
	CauseRadioNetworkInvalidQoSCombination
	CauseRadioNetworkFailureInRadioInterfaceProcedure
	CauseRadioNetworkInteractionWithOtherProcedure
	CauseRadioNetworkUnknownPDUSessionID // unknown-PDU-session-ID
	CauseRadioNetworkUnknownQoSFlowID
	CauseRadioNetworkMultiplePDUSessionIDs // multiple-PDU-session-ID-instances
	CauseRadioNetworkMultipleQoSFlowIDInstances
	// encryption-and-or-integrity-protection-algorithms-not-supported
	CauseRadioNetworkEncryptionAlgorithmsNotSupported
	CauseRadioNetworkNGIntraSystemHandoverTriggered // ng-intra-system-handover-triggered
	CauseRadioNetworkNGInterSystemHandoverTriggered // ng-inter-system-handover-triggered
	CauseRadioNetworkXnHandoverTriggered            // xn-handover-triggered
	CauseRadioNetworkNotSupported5QIValue
	CauseRadioNetworkUEContextTransfer
	// ims-voice-eps-fallback-or-rat-fallback-triggered
	CauseRadioNetworkIMSVoiceEPSFallbackTriggered // encryption-and-or-integrity-protection-algorithms-not-supported
	CauseRadioNetworkUPIntegrityProtectionNotPossible
	CauseRadioNetworkUPConfidentialityProtectionNotPossible
	CauseRadioNetworkSliceNotSupported // slice-not-supported
	CauseRadioNetworkUEInRRCInactiveStateNotReachable
	CauseRadioNetworkRedirection // redirection
	CauseRadioNetworkResourcesNotAvailableForTheSlice
	CauseRadioNetworkUEMaxIntegrityProtectedDataRateReason
	CauseRadioNetworkReleaseDueToCNDetectedMobility // release-due-to-cn-detected-mobility
	CauseRadioNetworkN26InterfaceNotAvailable
	CauseRadioNetworkReleaseDueToPreEmption
	CauseRadioNetworkMultipleLocationReportingReferenceIDInstances
	CauseRadioNetworkRSNNotAvailableForTheUP
	CauseRadioNetworkNPNAccessDenied
	CauseRadioNetworkCAGOnlyAccessDenied
	CauseRadioNetworkInsufficientUECapabilities
	CauseRadioNetworkRedCapUENotSupported
	CauseRadioNetworkUnknownMBSSessionID
	CauseRadioNetworkIndicatedMBSSessionAreaInformationNotServedByTheGNB
	CauseRadioNetworkInconsistentSliceInfoForTheSession
	CauseRadioNetworkMisalignedAssociationForMulticastUnicast
	CauseRadioNetworkERedCapUENotSupported
	CauseRadioNetworkTwoRxXRUENotSupported
	CauseRadioNetworkAerialUEFlightInformationReportingInitiationFailure
	CauseRadioNetworkUnknownRANAIoTDeviceNGAPID
	CauseRadioNetworkRequestedAIoTServiceAreaInformationNotServedByTheGNB
	CauseRadioNetworkUnknownAIoTSession
	CauseRadioNetworkAIoTDeviceNotReachable
	CauseRadioNetworkMultipleAIoTSessionIDInstances
)

type CauseTransport uint8

const (
	CauseTransportResourceUnavailable CauseTransport = iota // transport-resource-unavailable
	CauseTransportUnspecified                               // unspecified
)

type CauseNas uint8

const (
	CauseNASNormalRelease         CauseNas = iota // normal-release
	CauseNASAuthenticationFailure                 // authentication-failure
	CauseNASDeregister                            // deregister
	CauseNASUnspecified                           // unspecified
	CauseNASUENotInPLMNServingArea
	CauseNASMobileIABNotAuthorized
	CauseNASIABNotAuthorized
	CauseNASNoAIoTNASResponse
)

type CauseProtocol uint8

const (
	CauseProtocolTransferSyntaxError                CauseProtocol = iota // transfer-syntax-error
	CauseProtocolAbstractSyntaxErrorReject                               // abstract-syntax-error-reject
	CauseProtocolAbstractSyntaxErrorIgnoreAndNotify                      // abstract-syntax-error-ignore-and-notify
	CauseProtocolMessageNotCompatible                                    // message-not-compatible-with-receiver-state
	CauseProtocolSemanticError                                           // semantic-error
	// abstract-syntax-error-falsely-constructed-message
	CauseProtocolAbstractSyntaxErrorFalselyConstructedMessage
	CauseProtocolUnspecified // unspecified
)

type CauseMisc uint8

const (
	CauseMiscControlProcessingOverload CauseMisc = iota // control-processing-overload
	CauseMiscNotEnoughUserPlaneProcessingResources
	CauseMiscHardwareFailure   // hardware-failure
	CauseMiscOMIntervention    // om-intervention
	CauseMiscUnknownPLMNOrSNPN // unknown-PLMN-or-SNPN
	CauseMiscUnspecified       // unspecified
)

func (v CauseRadioNetwork) Cause() Cause { return Cause{RadioNetwork: &v} }

func (v CauseTransport) Cause() Cause { return Cause{Transport: &v} }

func (v CauseNas) Cause() Cause { return Cause{NAS: &v} }

func (v CauseProtocol) Cause() Cause { return Cause{Protocol: &v} }

func (v CauseMisc) Cause() Cause { return Cause{Misc: &v} }

func (c Cause) Alternative() (index int, name string) {
	switch {
	case c.RadioNetwork != nil:
		return 0, "radioNetwork"
	case c.Transport != nil:
		return 1, "transport"
	case c.NAS != nil:
		return 2, "nas"
	case c.Protocol != nil:
		return 3, "protocol"
	case c.Misc != nil:
		return 4, "misc"
	case c.ChoiceExtension != nil:
		return 5, "choice-Extensions"
	default:
		return -1, ""
	}
}

func (c Cause) Value() (index int, name string) {
	switch {
	case c.RadioNetwork != nil:
		return int(*c.RadioNetwork), c.RadioNetwork.Name()
	case c.Transport != nil:
		return int(*c.Transport), c.Transport.Name()
	case c.NAS != nil:
		return int(*c.NAS), c.NAS.Name()
	case c.Protocol != nil:
		return int(*c.Protocol), c.Protocol.Name()
	case c.Misc != nil:
		return int(*c.Misc), c.Misc.Name()
	default:
		return -1, ""
	}
}

func (c Cause) String() string {
	_, alt := c.Alternative()
	if alt == "" {
		return "empty"
	}

	if c.ChoiceExtension != nil {
		return fmt.Sprintf("%s: id %d", alt, c.ChoiceExtension.ID)
	}

	index, name := c.Value()

	return fmt.Sprintf("%s: %s (%d)", alt, name, index)
}
