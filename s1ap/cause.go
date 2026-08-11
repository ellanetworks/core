// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"fmt"

	"github.com/ellanetworks/core/per"
)

// CauseGroup selects a Cause CHOICE alternative (TS 36.413).
type CauseGroup uint8

const (
	CauseGroupRadioNetwork CauseGroup = iota
	CauseGroupTransport
	CauseGroupNAS
	CauseGroupProtocol
	CauseGroupMisc

	causeRootGroups = 5
)

// Root ENUMERATED values per group, which sets the index width.
var causeGroupRootCount = [causeRootGroups]int{
	CauseGroupRadioNetwork: 36,
	CauseGroupTransport:    2,
	CauseGroupNAS:          4,
	CauseGroupProtocol:     7,
	CauseGroupMisc:         6,
}

// Root ENUMERATED values of each Cause group (TS 36.413). A value is an index
// within its own group, so the same number differs across groups.
const (
	CauseRadioNetworkUnspecified                       = 0  // unspecified
	CauseRadioNetworkSuccessfulHandover                = 2  // successful-handover
	CauseRadioNetworkReleaseDueToEUTRANGeneratedReason = 3  // release-due-to-eutran-generated-reason
	CauseRadioNetworkHandoverCancelled                 = 4  // handover-cancelled
	CauseRadioNetworkPartialHandover                   = 5  // partial-handover
	CauseRadioNetworkHOFailureInTarget                 = 6  // ho-failure-in-target-EPC-eNB-or-target-system
	CauseRadioNetworkHOTargetNotAllowed                = 7  // ho-target-not-allowed
	CauseRadioNetworkTS1RelocOverallExpiry             = 8  // tS1relocoverall-expiry
	CauseRadioNetworkUnknownTargetID                   = 11 // unknown-targetID
	CauseRadioNetworkUnknownMMEUES1APID                = 13 // unknown-mme-ue-s1ap-id
	CauseRadioNetworkUnknownPairUES1APID               = 15 // unknown-pair-ue-s1ap-id
	CauseRadioNetworkHandoverDesirableForRadio         = 16 // handover-desirable-for-radio-reason
	CauseRadioNetworkTimeCriticalHandover              = 17 // time-critical-handover
	CauseRadioNetworkResourceOptimisationHandover      = 18 // resource-optimisation-handover
	CauseRadioNetworkReduceLoadInServingCell           = 19 // reduce-load-in-serving-cell
	CauseRadioNetworkRadioConnectionWithUELost         = 21 // radio-connection-with-ue-lost
	CauseRadioNetworkRadioResourcesNotAvailable        = 25 // radio-resources-not-available
	CauseRadioNetworkMultipleERABIDInstances           = 31 // multiple-E-RAB-ID-instances

	CauseTransportResourceUnavailable = 0 // transport-resource-unavailable

	CauseNASNormalRelease         = 0 // normal-release
	CauseNASAuthenticationFailure = 1 // authentication-failure
	CauseNASDetach                = 2 // detach
	CauseNASUnspecified           = 3 // unspecified

	CauseProtocolTransferSyntaxError                = 0 // transfer-syntax-error
	CauseProtocolAbstractSyntaxErrorReject          = 1 // abstract-syntax-error-reject
	CauseProtocolAbstractSyntaxErrorIgnoreAndNotify = 2 // abstract-syntax-error-ignore-and-notify
	CauseProtocolSemanticError                      = 4 // semantic-error

	// abstract-syntax-error-falsely-constructed-message
	CauseProtocolAbstractSyntaxErrorFalselyConstructedMessage = 5

	CauseMiscUnspecified = 4 // unspecified
	CauseMiscUnknownPLMN = 5 // unknown-PLMN
)

// Cause ::= CHOICE of five extensible ENUMERATED groups. Value indexes the
// chosen group; Extended marks it as indexing an extension addition.
type Cause struct {
	Group    CauseGroup
	Value    int
	Extended bool
}

func (c Cause) MarshalPER(w *per.Writer, enc per.Encoding) error {
	if int(c.Group) >= causeRootGroups {
		return fmt.Errorf("s1ap: invalid cause group %d", c.Group)
	}

	w.WriteBit(false)

	if err := per.EncodeConstrainedWholeNumber(w, enc, 0, causeRootGroups-1, int64(c.Group)); err != nil {
		return err
	}

	n := int64(causeGroupRootCount[c.Group])

	idx := int64(c.Value)
	if c.Extended {
		idx += n
	}

	return per.EncodeEnumerated(w, enc, n, true, idx)
}

func (c *Cause) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	gExt, err := r.ReadBit()
	if err != nil {
		return err
	}

	if gExt {
		return fmt.Errorf("%w: cause extension group", errNotComprehended)
	}

	gi, err := per.DecodeConstrainedWholeNumber(r, enc, 0, causeRootGroups-1)
	if err != nil {
		return fmt.Errorf("s1ap: cause group: %w", err)
	}

	n := int64(causeGroupRootCount[gi])

	idx, err := per.DecodeEnumerated(r, enc, n, true)
	if err != nil {
		return fmt.Errorf("s1ap: cause value: %w", err)
	}

	if idx >= n {
		*c = Cause{Group: CauseGroup(gi), Value: int(idx - n), Extended: true}
	} else {
		*c = Cause{Group: CauseGroup(gi), Value: int(idx)}
	}

	return nil
}
