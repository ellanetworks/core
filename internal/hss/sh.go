// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package hss

import (
	"context"
	"errors"
	"strings"

	"github.com/ellanetworks/core/diameter"
	"github.com/ellanetworks/core/diameter/sh"
	"github.com/ellanetworks/core/diameter/tgpp"
	"go.uber.org/zap"
)

type IPSMGWRegistration struct {
	Number        string
	DiameterHost  string
	DiameterRealm string
}

type SMS interface {
	RegisterIPSMGW(ctx context.Context, imsi string, r IPSMGWRegistration) error
	DeregisterIPSMGW(ctx context.Context, imsi string) error
}

func (h *HSS) ProfileUpdate(ctx context.Context, id diameter.Identity, req *diameter.Message) *diameter.Message {
	pur, err := sh.ParseProfileUpdateRequest(req)
	if err != nil {
		return sh.NewErrorAnswer(req, id, err, 0)
	}

	if sh.RequiresUnsupportedFeatures(req, 0) {
		return shAnswer(req, id, tgpp.ResultErrorFeatureUnsupported)
	}

	for _, ref := range pur.DataReferences {
		if ref != sh.DataSMSRegistrationInfo {
			return shAnswer(req, id, tgpp.ResultErrorUserDataCannotBeModified)
		}
	}

	if pur.WildcardedPublicIdentity != "" {
		return shAnswer(req, id, tgpp.ResultErrorOperationNotAllowed)
	}

	impu := pur.User.PublicIdentity
	if pur.WildcardedIMPU != "" {
		impu = pur.WildcardedIMPU
	}

	var sub *subject

	switch {
	case impu != "":
		var psi *ServiceIdentity
		if psi, err = h.store.ServiceIdentity(ctx, impu); err == nil && psi != nil {
			return shAnswer(req, id, tgpp.ResultErrorOperationNotAllowed)
		}

		if err == nil {
			sub, err = h.resolve(ctx, pur.PrivateIdentity, impu)
		}
	case pur.User.MSISDN != "":
		sub, err = h.resolveMSISDN(ctx, pur.PrivateIdentity, pur.User.MSISDN)
	default:
		err = ErrSubscriberUnknown
	}

	if err != nil {
		return h.shErrorAnswer(req, id, err)
	}

	data, err := sh.ParseShData(pur.UserData)
	if err != nil || data.SMSRegistration == nil {
		return shAnswer(req, id, tgpp.ResultErrorUserDataNotRecognized)
	}

	if h.sms == nil {
		return sh.NewAnswer(req, id, tgpp.Result{Code: diameter.ResultUnableToComply}, 0)
	}

	if number := data.SMSRegistration.IPSMGWNumber; number != "" {
		origin := tgpp.ParseEnvelope(req).Origin
		err = h.sms.RegisterIPSMGW(ctx, sub.IMSI, IPSMGWRegistration{Number: number, DiameterHost: origin.OriginHost, DiameterRealm: origin.OriginRealm})
	} else {
		err = h.sms.DeregisterIPSMGW(ctx, sub.IMSI)
	}

	if err != nil {
		return h.shErrorAnswer(req, id, err)
	}

	ans, err := sh.NewProfileUpdateAnswer(req, id, sh.ProfileUpdate{Result: tgpp.Result{Code: diameter.ResultSuccess}})
	if err != nil {
		return sh.NewAnswer(req, id, tgpp.Result{Code: diameter.ResultUnableToComply}, 0)
	}

	return ans
}

func (h *HSS) resolveMSISDN(ctx context.Context, impi, msisdn string) (*subject, error) {
	owner, err := h.subscriberByMSISDN(ctx, msisdn)
	if err != nil {
		return nil, err
	}

	if impi == "" {
		domain, err := h.store.IMSDomain(ctx)
		if err != nil {
			return nil, err
		}

		return &subject{Subscriber: *owner, domain: strings.ToLower(domain)}, nil
	}

	sub, err := h.resolve(ctx, impi)
	if err != nil {
		return nil, err
	}

	if sub.IMSI != owner.IMSI {
		return nil, &sh.ResultError{Result: tgpp.Experimental(tgpp.ResultErrorIdentitiesDontMatch)}
	}

	return sub, nil
}

func (h *HSS) shErrorAnswer(req *diameter.Message, id diameter.Identity, err error) *diameter.Message {
	if r, ok := tgpp.ResultOf(err); ok {
		return sh.NewAnswer(req, id, r, 0)
	}

	switch {
	case errors.Is(err, ErrSubscriberUnknown):
		return shAnswer(req, id, tgpp.ResultErrorUserUnknown)
	case errors.Is(err, ErrUnavailable):
		return sh.NewAnswer(req, id, tgpp.Result{Code: diameter.UnavailableResult(req)}, 0)
	}

	h.log.Warn("Sh update failed", zap.Error(err))

	return sh.NewAnswer(req, id, tgpp.Result{Code: diameter.ResultUnableToComply}, 0)
}

func shAnswer(req *diameter.Message, id diameter.Identity, code uint32) *diameter.Message {
	return sh.NewAnswer(req, id, tgpp.Experimental(code), 0)
}
