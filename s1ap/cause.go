// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"fmt"

	"github.com/ellanetworks/core/s1ap/aper"
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

// causeGroupRootCount is the number of root ENUMERATED values in each group's
// CauseXxx type, which sets the index width (TS 36.413).
var causeGroupRootCount = [causeRootGroups]int{
	CauseGroupRadioNetwork: 36,
	CauseGroupTransport:    2,
	CauseGroupNAS:          4,
	CauseGroupProtocol:     7,
	CauseGroupMisc:         6,
}

// Named root values of each Cause group's ENUMERATED (TS 36.413) that
// Ella Core emits. Each value is its group's enumeration index, so the same
// number means different things in different groups — the group prefix names it.
const (
	CauseRadioNetworkUnspecified             = 0  // unspecified
	CauseRadioNetworkUnknownMMEUES1APID      = 13 // unknown-mme-ue-s1ap-id
	CauseRadioNetworkUnknownPairUES1APID     = 15 // unknown-pair-ue-s1ap-id
	CauseRadioNetworkMultipleERABIDInstances = 31 // multiple-E-RAB-ID-instances

	CauseTransportResourceUnavailable = 0 // transport-resource-unavailable

	CauseNASNormalRelease         = 0 // normal-release
	CauseNASAuthenticationFailure = 1 // authentication-failure
	CauseNASDetach                = 2 // detach
	CauseNASUnspecified           = 3 // unspecified

	CauseProtocolTransferSyntaxError = 0 // transfer-syntax-error

	CauseMiscUnspecified = 4 // unspecified
	CauseMiscUnknownPLMN = 5 // unknown-PLMN
)

// Cause ::= CHOICE of five extensible ENUMERATED groups. Value is the chosen
// group's enumeration index; Extended is true when Value indexes an extension
// addition of that enumeration and false when it indexes a root value.
type Cause struct {
	Group    CauseGroup
	Value    int
	Extended bool
}

func (c Cause) encode(w *aper.Writer) error {
	if int(c.Group) >= causeRootGroups {
		return fmt.Errorf("s1ap: invalid cause group %d", c.Group)
	}

	if err := w.WriteChoiceIndex(int(c.Group), causeRootGroups, true, false); err != nil {
		return err
	}

	return w.WriteEnum(c.Value, causeGroupRootCount[c.Group], true, c.Extended)
}

func decodeCause(r *aper.Reader) (Cause, error) {
	gi, gExt, err := r.ReadChoiceIndex(causeRootGroups, true)
	if err != nil {
		return Cause{}, fmt.Errorf("s1ap: cause group: %w", err)
	}

	if gExt {
		return Cause{}, fmt.Errorf("s1ap: unsupported cause extension group")
	}

	vi, vExt, err := r.ReadEnum(causeGroupRootCount[gi], true)
	if err != nil {
		return Cause{}, fmt.Errorf("s1ap: cause value: %w", err)
	}

	return Cause{Group: CauseGroup(gi), Value: vi, Extended: vExt}, nil
}
