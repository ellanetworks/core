// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"github.com/ellanetworks/core/s1ap"
)

// S1AP NAS-group causes the MME uses when releasing a UE context (TS 36.413).
var (
	CauseNASNormalRelease = s1ap.CauseNASNormalRelease.Cause()
	CauseNASDetach        = s1ap.CauseNASDetach.Cause()
	CauseNASUnspecified   = s1ap.CauseNASUnspecified.Cause()
)

// S1apCauseName renders an S1AP cause as "<group>: <name> (<value>)".
func S1apCauseName(c *s1ap.Cause) string {
	return c.String()
}

// emmCauseNames maps an EMM cause value to its human-readable name (TS 24.301
// §9.9.3.9, table 9.9.3.9.1), for logging.
