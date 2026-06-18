// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"fmt"

	"github.com/ellanetworks/core/s1ap/aper"
)

// CauseGroup selects a Cause CHOICE alternative (TS 36.413 §9.2.1.3).
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
// CauseXxx type, which sets the index width (TS 36.413 §9.2.1.3).
var causeGroupRootCount = [causeRootGroups]int{
	CauseGroupRadioNetwork: 36,
	CauseGroupTransport:    2,
	CauseGroupNAS:          4,
	CauseGroupProtocol:     7,
	CauseGroupMisc:         6,
}

// Cause ::= CHOICE of five extensible ENUMERATED groups. Value is the chosen
// group's enumeration index; Extended is set when Value names an extension
// addition of that enumeration rather than a root value.
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
