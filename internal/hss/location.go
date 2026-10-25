// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package hss

import (
	"context"
	"errors"

	"github.com/ellanetworks/core/diameter"
	"github.com/ellanetworks/core/diameter/cx"
	"github.com/ellanetworks/core/diameter/tgpp"
)

func (h *HSS) LocationInfo(ctx context.Context, id diameter.Identity, req *diameter.Message) *diameter.Message {
	r, err := cx.ParseLocationInfoRequest(req)
	if err != nil {
		return cx.NewErrorAnswer(req, id, err, 0)
	}

	if cx.RequiresUnsupportedFeatures(req, 0) {
		return experimentalAnswer(req, id, tgpp.ResultErrorFeatureUnsupported)
	}

	sub, err := h.resolve(ctx, "", r.PublicIdentity)
	if errors.Is(err, ErrSubscriberUnknown) && !r.Originating {
		psi, psiErr := h.store.ServiceIdentity(ctx, r.PublicIdentity)
		if psiErr != nil {
			return h.errorAnswer(req, id, "location info", psiErr)
		}

		if psi != nil {
			return h.locationInfoAnswer(req, id, cx.LocationInfo{
				Result:                   tgpp.Result{Code: diameter.ResultSuccess},
				ServerName:               psi.ServerName,
				WildcardedPublicIdentity: psi.Wildcard,
				PSIDirectRouting:         true,
			})
		}
	}

	if err != nil {
		return h.errorAnswer(req, id, "location info", err)
	}

	reg, err := h.store.Registration(ctx, sub.IMSI)
	if err != nil {
		return h.errorAnswer(req, id, "location info", err)
	}

	assigned := reg != nil && reg.ServerName != ""

	if assigned && (reg.State == Registered || reg.State == Unregistered) {
		return h.locationInfoAnswer(req, id, cx.LocationInfo{Result: tgpp.Result{Code: diameter.ResultSuccess}, ServerName: reg.ServerName})
	}

	if !sub.entitled() {
		return experimentalAnswer(req, id, tgpp.ResultErrorIdentityNotRegistered)
	}

	if !r.Originating {
		ifcs, err := h.store.FilterCriteria(ctx, sub.IMSI)
		if err != nil {
			return h.errorAnswer(req, id, "location info", err)
		}

		if !hasUnregisteredTerminatingServices(ifcs) {
			return experimentalAnswer(req, id, tgpp.ResultErrorIdentityNotRegistered)
		}
	}

	if assigned {
		return h.locationInfoAnswer(req, id, cx.LocationInfo{Result: tgpp.Result{Code: diameter.ResultSuccess}, ServerName: reg.ServerName})
	}

	return h.locationInfoAnswer(req, id, cx.LocationInfo{Result: tgpp.Experimental(tgpp.ResultUnregisteredService)})
}

func (h *HSS) locationInfoAnswer(req *diameter.Message, id diameter.Identity, l cx.LocationInfo) *diameter.Message {
	ans, err := cx.NewLocationInfoAnswer(req, id, l)
	if err != nil {
		return h.errorAnswer(req, id, "location info", err)
	}

	return ans
}
