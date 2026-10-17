// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package hss

import (
	"context"

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
	if err != nil {
		return h.errorAnswer(req, id, "location info", err)
	}

	reg, err := h.store.Registration(ctx, sub.IMSI)
	if err != nil {
		return h.errorAnswer(req, id, "location info", err)
	}

	if reg != nil && reg.ServerName != "" && (reg.State == Registered || reg.State == Unregistered) {
		return h.locationInfoAnswer(req, id, cx.LocationInfo{Result: tgpp.Result{Code: diameter.ResultSuccess}, ServerName: reg.ServerName})
	}

	if r.Originating {
		return h.locationInfoAnswer(req, id, cx.LocationInfo{Result: tgpp.Experimental(tgpp.ResultUnregisteredService), Capabilities: &cx.ServerCapabilities{}})
	}

	return experimentalAnswer(req, id, tgpp.ResultErrorIdentityNotRegistered)
}

func (h *HSS) locationInfoAnswer(req *diameter.Message, id diameter.Identity, l cx.LocationInfo) *diameter.Message {
	ans, err := cx.NewLocationInfoAnswer(req, id, l)
	if err != nil {
		return h.errorAnswer(req, id, "location info", err)
	}

	return ans
}
