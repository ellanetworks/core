// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"github.com/ellanetworks/core/internal/decoder/utils"
	"github.com/ellanetworks/core/nas/fgs"
)

// cause5GMMToEnum renders a 5GMM cause, marking values TS 24.501 does not assign
// as unknown.
func cause5GMMToEnum(cause fgs.GMMCause) utils.EnumField {
	if name := cause.Name(); name != "" {
		return utils.MakeEnum(uint8(cause), name, false)
	}

	return utils.MakeEnum(uint8(cause), "", true)
}
