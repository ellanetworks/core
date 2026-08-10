// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package models

import "github.com/ellanetworks/core/nas/eps"

type PDNTypeError struct {
	Cause eps.ESMCause
}

func (e *PDNTypeError) Error() string { return "PDN type refused: " + e.Cause.String() }
