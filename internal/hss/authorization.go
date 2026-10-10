// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package hss

import (
	"context"

	"github.com/ellanetworks/core/diameter"
	"github.com/ellanetworks/core/diameter/cx"
	"github.com/ellanetworks/core/diameter/tgpp"
)

func (h *HSS) UserAuthorization(ctx context.Context, id diameter.Identity, req *diameter.Message) *diameter.Message {
	ans := h.authorizeUser(ctx, id, req)
	recordAuthorization(req, ans)

	return ans
}

func (h *HSS) authorizeUser(ctx context.Context, id diameter.Identity, req *diameter.Message) *diameter.Message {
	r, err := cx.ParseUserAuthorizationRequest(req)
	if err != nil {
		return cx.NewErrorAnswer(req, id, err, 0)
	}

	if cx.RequiresUnsupportedFeatures(req, 0) {
		return experimentalAnswer(req, id, tgpp.ResultErrorFeatureUnsupported)
	}

	sub, err := h.resolve(ctx, r.PrivateIdentity, r.PublicIdentity)
	if err != nil {
		return h.errorAnswer(req, id, "user authorization", err)
	}

	if !sub.authorized() && !r.EmergencyRegistration {
		return cx.NewAnswer(req, id, tgpp.Result{Code: diameter.ResultAuthorizationRejected}, 0)
	}

	if r.AuthorizationType != cx.AuthorizationDeregistration && !r.EmergencyRegistration && !sub.entitled() {
		return cx.NewAnswer(req, id, tgpp.Result{Code: diameter.ResultAuthorizationRejected}, 0)
	}

	if r.AuthorizationType == cx.AuthorizationRegistrationAndCapabilities {
		return h.userAuthorizationAnswer(req, id, cx.UserAuthorization{Result: tgpp.Result{Code: diameter.ResultSuccess}, Capabilities: &cx.ServerCapabilities{}})
	}

	reg, err := h.store.Registration(ctx, sub.IMSI)
	if err != nil {
		return h.errorAnswer(req, id, "user authorization", err)
	}

	assigned := reg != nil && reg.ServerName != ""

	if r.AuthorizationType == cx.AuthorizationDeregistration {
		if !assigned || (reg.State == NotRegistered && !reg.AuthPending) {
			return experimentalAnswer(req, id, tgpp.ResultErrorIdentityNotRegistered)
		}

		return h.userAuthorizationAnswer(req, id, cx.UserAuthorization{Result: tgpp.Result{Code: diameter.ResultSuccess}, ServerName: reg.ServerName})
	}

	if assigned {
		return h.userAuthorizationAnswer(req, id, cx.UserAuthorization{Result: tgpp.Experimental(tgpp.ResultSubsequentRegistration), ServerName: reg.ServerName})
	}

	return h.userAuthorizationAnswer(req, id, cx.UserAuthorization{Result: tgpp.Experimental(tgpp.ResultFirstRegistration), Capabilities: &cx.ServerCapabilities{}})
}

func (h *HSS) userAuthorizationAnswer(req *diameter.Message, id diameter.Identity, a cx.UserAuthorization) *diameter.Message {
	ans, err := cx.NewUserAuthorizationAnswer(req, id, a)
	if err != nil {
		return h.errorAnswer(req, id, "user authorization", err)
	}

	return ans
}
