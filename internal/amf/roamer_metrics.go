// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package amf

import (
	"context"

	"github.com/ellanetworks/core/internal/roaming"
)

func (amf *AMF) IsRoamer(ctx context.Context, ue *UeContext) bool {
	if ue.IsVisitor() {
		return true
	}

	if amf.Roaming == nil {
		return false
	}

	prefix, _, ok := hplmnOfUE(ue)
	if !ok {
		return false
	}

	partners, err := amf.Roaming.ListRoamingPartners(ctx)
	if err != nil {
		return false
	}

	_, _, ok = roaming.PartnerOfIMSI(partners, prefix)

	return ok
}
