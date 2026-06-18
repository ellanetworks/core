// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"fmt"

	"github.com/ellanetworks/core/internal/s1apcause"
	"github.com/ellanetworks/core/s1ap"
)

// s1apCauseGroupName is the display name of each S1AP cause group (TS 36.413),
// mirroring the AMF's NGAP cause logging so 4G and 5G control-plane
// causes read alike.
var s1apCauseGroupName = map[s1ap.CauseGroup]string{
	s1ap.CauseGroupRadioNetwork: "Radio Network",
	s1ap.CauseGroupTransport:    "Transport",
	s1ap.CauseGroupNAS:          "NAS",
	s1ap.CauseGroupProtocol:     "Protocol",
	s1ap.CauseGroupMisc:         "Misc",
}

// s1apCauseName renders an S1AP cause as "<group>: <name> (<value>)".
func s1apCauseName(c *s1ap.Cause) string {
	group, ok := s1apCauseGroupName[c.Group]
	if !ok {
		return fmt.Sprintf("group-%d: value-%d", int(c.Group), c.Value)
	}

	name, index := s1apcause.ValueName(c.Group, c.Value, c.Extended)

	return fmt.Sprintf("%s: %s (%d)", group, name, index)
}
