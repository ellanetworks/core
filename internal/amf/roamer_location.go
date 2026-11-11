// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package amf

import "github.com/ellanetworks/core/etsi"

func (amf *AMF) ServesRoamer(supi etsi.SUPI) bool {
	ue, ok := amf.LookupUeBySupi(supi)

	return ok && ue.IsVisitor()
}
