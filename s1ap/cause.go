// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"fmt"
	"strings"

	"github.com/ellanetworks/core/per"
)

// Cause ::= CHOICE { radioNetwork, transport, nas, protocol, misc, ... }
// (TS 36.413 §9.2.1.3).
type Cause struct {
	RadioNetwork *CauseRadioNetwork        `per:",choice:0"`
	Transport    *CauseTransport           `per:",choice:1"`
	NAS          *CauseNas                 `per:",choice:2"`
	Protocol     *CauseProtocol            `per:",choice:3"`
	Misc         *CauseMisc                `per:",choice:4"`
	Ext          *per.ExtensionAlternative `per:",extalt"`
}

type CauseRadioNetwork uint8

const (
	CauseRadioNetworkUnspecified CauseRadioNetwork = iota
	CauseRadioNetworkTX2RelocOverallExpiry
	CauseRadioNetworkSuccessfulHandover
	CauseRadioNetworkReleaseDueToEUTRANGeneratedReason
	CauseRadioNetworkHandoverCancelled
	CauseRadioNetworkPartialHandover
	CauseRadioNetworkHOFailureInTarget
	CauseRadioNetworkHOTargetNotAllowed
	CauseRadioNetworkTS1RelocOverallExpiry
	CauseRadioNetworkTS1RelocPrepExpiry
	CauseRadioNetworkCellNotAvailable
	CauseRadioNetworkUnknownTargetID
	CauseRadioNetworkNoRadioResourcesInTargetCell
	CauseRadioNetworkUnknownMMEUES1APID
	CauseRadioNetworkUnknownENBUES1APID
	CauseRadioNetworkUnknownPairUES1APID
	CauseRadioNetworkHandoverDesirableForRadio
	CauseRadioNetworkTimeCriticalHandover
	CauseRadioNetworkResourceOptimisationHandover
	CauseRadioNetworkReduceLoadInServingCell
	CauseRadioNetworkUserInactivity
	CauseRadioNetworkRadioConnectionWithUELost
	CauseRadioNetworkLoadBalancingTAURequired
	CauseRadioNetworkCSFallbackTriggered
	CauseRadioNetworkUENotAvailableForPSService
	CauseRadioNetworkRadioResourcesNotAvailable
	CauseRadioNetworkFailureInRadioInterfaceProcedure
	CauseRadioNetworkInvalidQoSCombination
	CauseRadioNetworkInterRATRedirection
	CauseRadioNetworkInteractionWithOtherProcedure
	CauseRadioNetworkUnknownERABID
	CauseRadioNetworkMultipleERABIDInstances
	CauseRadioNetworkEncryptionAlgorithmsNotSupported
	CauseRadioNetworkS1IntraSystemHandoverTriggered
	CauseRadioNetworkS1InterSystemHandoverTriggered
	CauseRadioNetworkX2HandoverTriggered
	CauseRadioNetworkRedirectionTowards1xRTT
	CauseRadioNetworkNotSupportedQCIValue
	CauseRadioNetworkInvalidCSGID
	CauseRadioNetworkReleaseDueToPreemption
	CauseRadioNetworkN26InterfaceNotAvailable
	CauseRadioNetworkInsufficientUECapabilities
	CauseRadioNetworkMaximumBearerPreemptionRateExceeded
	CauseRadioNetworkUPIntegrityProtectionNotPossible
	CauseRadioNetworkReleaseDueToDiscontinuousCoverage
)

type CauseTransport uint8

const (
	CauseTransportResourceUnavailable CauseTransport = iota
	CauseTransportUnspecified
)

type CauseNas uint8

const (
	CauseNASNormalRelease CauseNas = iota
	CauseNASAuthenticationFailure
	CauseNASDetach
	CauseNASUnspecified
	CauseNASCSGSubscriptionExpiry
	CauseNASUENotInPLMNServingArea
	CauseNASIABNotAuthorized
)

type CauseProtocol uint8

const (
	CauseProtocolTransferSyntaxError CauseProtocol = iota
	CauseProtocolAbstractSyntaxErrorReject
	CauseProtocolAbstractSyntaxErrorIgnoreAndNotify
	CauseProtocolMessageNotCompatible
	CauseProtocolSemanticError
	CauseProtocolAbstractSyntaxErrorFalselyConstructedMessage
	CauseProtocolUnspecified
)

type CauseMisc uint8

const (
	CauseMiscControlProcessingOverload CauseMisc = iota
	CauseMiscNotEnoughUserPlaneProcessingResources
	CauseMiscHardwareFailure
	CauseMiscOMIntervention
	CauseMiscUnspecified
	CauseMiscUnknownPLMN
)

func (v CauseRadioNetwork) Cause() Cause { return Cause{RadioNetwork: &v} }

func (v CauseTransport) Cause() Cause { return Cause{Transport: &v} }

func (v CauseNas) Cause() Cause { return Cause{NAS: &v} }

func (v CauseProtocol) Cause() Cause { return Cause{Protocol: &v} }

func (v CauseMisc) Cause() Cause { return Cause{Misc: &v} }

func (c Cause) alternative() (index int, value interface{ Name() string }, enumIndex int, ok bool) {
	switch {
	case c.RadioNetwork != nil:
		return 0, *c.RadioNetwork, int(*c.RadioNetwork), true
	case c.Transport != nil:
		return 1, *c.Transport, int(*c.Transport), true
	case c.NAS != nil:
		return 2, *c.NAS, int(*c.NAS), true
	case c.Protocol != nil:
		return 3, *c.Protocol, int(*c.Protocol), true
	case c.Misc != nil:
		return 4, *c.Misc, int(*c.Misc), true
	default:
		return 0, nil, 0, false
	}
}

var causeGroupNames = [...]string{"radioNetwork", "transport", "nas", "protocol", "misc"}

func (c Cause) Group() (index int, ok bool) {
	index, _, _, ok = c.alternative()

	return index, ok
}

func (c Cause) GroupName() string {
	index, _, _, ok := c.alternative()
	if !ok {
		return ""
	}

	return causeGroupNames[index]
}

func (c Cause) EnumeratedValue() int {
	_, v := c.ValueName()

	return v
}

func (c Cause) ValueName() (name string, index int) {
	_, v, i, ok := c.alternative()
	if !ok {
		return "unknown", 0
	}

	name = v.Name()
	if strings.HasPrefix(name, "unknown(") {
		name = "unknown"
	}

	return name, i
}

func (c Cause) String() string {
	group := c.GroupName()
	if group == "" {
		if c.Ext != nil {
			return fmt.Sprintf("extension-%d", c.Ext.Index)
		}

		return ""
	}

	name, index := c.ValueName()

	return fmt.Sprintf("%s: %s (%d)", group, name, index)
}
