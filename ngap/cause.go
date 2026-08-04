// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"fmt"

	"github.com/ellanetworks/core/per"
)

// CauseGroup selects a Cause CHOICE alternative (TS 38.413 §9.3.1.2).
type CauseGroup uint8

const (
	CauseGroupRadioNetwork CauseGroup = iota
	CauseGroupTransport
	CauseGroupNAS
	CauseGroupProtocol
	CauseGroupMisc

	causeRootGroups = 5

	// An alternative index, not a group, so never a valid CauseGroup.
	causeChoiceExtensions = 5

	// The CHOICE is not extensible, so there is no extension bit and the index
	// spans the choice-Extensions alternative.
	causeAlternatives = 6
)

// Root ENUMERATED values per group, which sets the index width (§9.3.1.2).
var causeGroupRootCount = [causeRootGroups]int{
	CauseGroupRadioNetwork: 45,
	CauseGroupTransport:    2,
	CauseGroupNAS:          4,
	CauseGroupProtocol:     7,
	CauseGroupMisc:         6,
}

// Root ENUMERATED values of each Cause group (TS 38.413 §9.3.1.2). A value is
// an index within its own group, so the same number differs across groups.
const (
	CauseRadioNetworkUnspecified                      = 0  // unspecified
	CauseRadioNetworkSuccessfulHandover               = 2  // successful-handover
	CauseRadioNetworkReleaseDueToNGRANGeneratedReason = 3  // release-due-to-ngran-generated-reason
	CauseRadioNetworkTNGRelocOverallExpiry            = 9  // tngrelocoverall-expiry
	CauseRadioNetworkUnknownLocalUENGAPID             = 14 // unknown-local-UE-NGAP-ID
	CauseRadioNetworkInconsistentRemoteUEID           = 15 // inconsistent-remote-UE-NGAP-ID
	CauseRadioNetworkUserInactivity                   = 20 // user-inactivity
	CauseRadioNetworkRadioConnectionWithUELost        = 21 // radio-connection-with-ue-lost
	CauseRadioNetworkMultiplePDUSessionIDs            = 28 // multiple-PDU-session-ID-instances
	CauseRadioNetworkSliceNotSupported                = 39 // slice-not-supported

	CauseTransportResourceUnavailable = 0 // transport-resource-unavailable
	CauseTransportUnspecified         = 1 // unspecified

	CauseNASNormalRelease         = 0 // normal-release
	CauseNASAuthenticationFailure = 1 // authentication-failure
	CauseNASDeregister            = 2 // deregister
	CauseNASUnspecified           = 3 // unspecified

	CauseProtocolTransferSyntaxError                = 0 // transfer-syntax-error
	CauseProtocolAbstractSyntaxErrorReject          = 1 // abstract-syntax-error-reject
	CauseProtocolAbstractSyntaxErrorIgnoreAndNotify = 2 // abstract-syntax-error-ignore-and-notify
	CauseProtocolMessageNotCompatible               = 3 // message-not-compatible-with-receiver-state
	CauseProtocolSemanticError                      = 4 // semantic-error
	CauseProtocolUnspecified                        = 6 // unspecified

	// abstract-syntax-error-falsely-constructed-message
	CauseProtocolAbstractSyntaxErrorFalselyConstructedMessage = 5

	CauseMiscControlProcessingOverload = 0 // control-processing-overload
	CauseMiscHardwareFailure           = 2 // hardware-failure
	CauseMiscOMIntervention            = 3 // om-intervention
	CauseMiscUnknownPLMNOrSNPN         = 4 // unknown-PLMN-or-SNPN
	CauseMiscUnspecified               = 5 // unspecified
)

// Cause ::= CHOICE of five extensible ENUMERATED groups plus a
// choice-Extensions alternative. Value indexes the chosen group; Extended
// marks it as indexing an extension addition of that group.
type Cause struct {
	Group    CauseGroup
	Value    int
	Extended bool
}

func (c Cause) MarshalPER(w *per.Writer, enc per.Encoding) error {
	if int(c.Group) >= causeRootGroups {
		return fmt.Errorf("ngap: invalid cause group %d", c.Group)
	}

	if err := per.EncodeConstrainedWholeNumber(w, enc, 0, causeAlternatives-1, int64(c.Group)); err != nil {
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
	gi, err := per.DecodeConstrainedWholeNumber(r, enc, 0, causeAlternatives-1)
	if err != nil {
		return fmt.Errorf("ngap: cause group: %w", err)
	}

	if gi == causeChoiceExtensions {
		return decodeChoiceExtension(r, enc, "Cause")
	}

	n := int64(causeGroupRootCount[gi])

	idx, err := per.DecodeEnumerated(r, enc, n, true)
	if err != nil {
		return fmt.Errorf("ngap: cause value: %w", err)
	}

	if idx >= n {
		*c = Cause{Group: CauseGroup(gi), Value: int(idx - n), Extended: true}
	} else {
		*c = Cause{Group: CauseGroup(gi), Value: int(idx)}
	}

	return nil
}
