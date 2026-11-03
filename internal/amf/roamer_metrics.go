// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package amf

import (
	"context"

	"github.com/ellanetworks/core/internal/roaming"
)

func (amf *AMF) RecordRoamerReject(ctx context.Context, ue *UeContext, r *Rejection) {
	cause := r.Cause.String()
	if r.AuthReject {
		cause = "Authentication reject"
	}

	if partner := amf.roamerPartnerName(ctx, ue); partner != "" {
		roaming.RecordReject5G(partner, cause)
	}
}

func (amf *AMF) roamerPartnerName(ctx context.Context, ue *UeContext) string {
	if v, ok := ue.Visitor(); ok {
		return v.PartnerName
	}

	if amf.Roaming == nil {
		return ""
	}

	prefix, _, ok := homeOfUE(ue)
	if !ok {
		return ""
	}

	partners, err := amf.Roaming.ListRoamingPartners(ctx)
	if err != nil {
		return ""
	}

	if p, _, ok := roaming.PartnerOfIMSI(partners, prefix); ok {
		return p.Name
	}

	return ""
}
