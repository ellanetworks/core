// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"context"

	"github.com/ellanetworks/core/diameter"
	"github.com/ellanetworks/core/internal/diameternode"
	"github.com/ellanetworks/core/internal/roaming"
	"github.com/ellanetworks/core/nas/eps"
)

func (m *MME) RecordRoamerReject(ctx context.Context, ue *UeContext, cause eps.EMMCause) {
	if v, ok := ue.Visitor(); ok {
		roaming.RecordReject4G(v.PartnerName, cause.String())
		return
	}

	imsi := ue.imsiOrEmpty()
	if imsi == "" || m.Roaming == nil {
		return
	}

	partners, err := m.Roaming.ListRoamingPartners(ctx)
	if err != nil {
		return
	}

	if p, _, ok := roaming.PartnerOfIMSI(partners, imsi); ok {
		roaming.RecordReject4G(p.Name, cause.String())
	}
}

func (m *MME) countedHomeHandler(command uint32, h func(context.Context, diameter.Identity, *diameter.Message) *diameter.Message) diameter.Handler {
	return diameternode.Handler(func(ctx context.Context, id diameter.Identity, req *diameter.Message) *diameter.Message {
		partner := m.partnerOfHomeRequest(req)
		ans := h(ctx, id, req)
		roaming.RecordS6aAnswer(partner, command, ans, nil)

		return ans
	})
}

func (m *MME) partnerOfHomeRequest(req *diameter.Message) string {
	if a, ok := req.Find(diameter.AVPUserName, 0); ok {
		if ue, ok := m.LookupUeByIMSI(a.UTF8String()); ok {
			if v, ok := ue.Visitor(); ok {
				return v.PartnerName
			}
		}
	}

	if a, ok := req.Find(diameter.AVPOriginHost, 0); ok {
		for _, ue := range m.visitorsServedBy(a.UTF8String()) {
			if v, ok := ue.Visitor(); ok {
				return v.PartnerName
			}
		}
	}

	return ""
}
