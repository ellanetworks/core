// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"fmt"

	"github.com/ellanetworks/core/internal/s1apcause"
	"github.com/ellanetworks/core/s1ap"
)

// S1AP causes (TS 36.413) the MME uses when releasing a UE context:
// "nas: detach" after a detach, and "nas: unspecified" after an attach reject.
var (
	CauseNASNormalRelease = s1ap.Cause{Group: s1ap.CauseGroupNAS, Value: s1ap.CauseNASNormalRelease}
	CauseNASDetach        = s1ap.Cause{Group: s1ap.CauseGroupNAS, Value: s1ap.CauseNASDetach}
	CauseNASUnspecified   = s1ap.Cause{Group: s1ap.CauseGroupNAS, Value: s1ap.CauseNASUnspecified}
)

// s1apCauseGroupName is the display name of each S1AP cause group (TS 36.413)
var s1apCauseGroupName = map[s1ap.CauseGroup]string{
	s1ap.CauseGroupRadioNetwork: "Radio Network",
	s1ap.CauseGroupTransport:    "Transport",
	s1ap.CauseGroupNAS:          "NAS",
	s1ap.CauseGroupProtocol:     "Protocol",
	s1ap.CauseGroupMisc:         "Misc",
}

// S1apCauseName renders an S1AP cause as "<group>: <name> (<value>)".
func S1apCauseName(c *s1ap.Cause) string {
	group, ok := s1apCauseGroupName[c.Group]
	if !ok {
		return fmt.Sprintf("group-%d: value-%d", int(c.Group), c.Value)
	}

	name, index := s1apcause.ValueName(c.Group, c.Value, c.Extended)

	return fmt.Sprintf("%s: %s (%d)", group, name, index)
}

// EMM cause values (TS 24.301).
const (
	EmmCauseIMSIUnknownInHSS      uint8 = 2
	EmmCauseEPSServicesNotAllowed uint8 = 7
	EmmCauseUEIdentityUnderivable uint8 = 9
	EmmCauseCSDomainNotAvailable  uint8 = 18
	EmmCauseESMFailure            uint8 = 19
	EmmCauseMACFailure            uint8 = 20
	EmmCauseSynchFailure          uint8 = 21
	EmmCauseUESecCapsMismatch     uint8 = 23
)
