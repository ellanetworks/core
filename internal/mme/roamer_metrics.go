// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"context"

	"github.com/ellanetworks/core/internal/roaming"
)

func (m *MME) IsRoamer(ctx context.Context, ue *UeContext) bool {
	if ue == nil {
		return false
	}

	if ue.IsVisitor() {
		return true
	}

	imsi := ue.imsiOrEmpty()
	if imsi == "" || m.Roaming == nil {
		return false
	}

	partners, err := m.Roaming.ListRoamingPartners(ctx)
	if err != nil {
		return false
	}

	_, _, ok := roaming.PartnerOfIMSI(partners, imsi)

	return ok
}
