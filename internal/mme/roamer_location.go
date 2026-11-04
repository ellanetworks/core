// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import "github.com/ellanetworks/core/etsi"

func (m *MME) ServesRoamer(supi etsi.SUPI) bool {
	ue, ok := m.LookupUeBySupi(supi)

	return ok && ue.IsVisitor()
}
