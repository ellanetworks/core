// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"github.com/ellanetworks/core/internal/decoder/utils"
	"github.com/ellanetworks/core/nas/fgs"
)

func cause5GMMToEnum(cause fgs.GMMCause) utils.EnumField {
	return utils.NamedEnum(uint8(cause), cause.Name())
}
