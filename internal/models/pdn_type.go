// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package models

import "github.com/ellanetworks/core/nas/eps"

// PDNTypeError carries the ESM cause a refused PDN type draws. The anchor holds
// the data-network pools, so it is what can tell #28 from #50 and #51
// (TS 24.301 §6.5.1.4.1); the MME only relays the answer.
type PDNTypeError struct {
	Cause eps.ESMCause
}

func (e *PDNTypeError) Error() string { return "PDN type refused: " + e.Cause.String() }
