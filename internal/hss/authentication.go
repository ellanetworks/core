// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package hss

import (
	"context"
	"errors"

	"github.com/ellanetworks/core/diameter"
	"github.com/ellanetworks/core/diameter/cx"
	"github.com/ellanetworks/core/diameter/tgpp"
	"github.com/ellanetworks/core/internal/udm"
	"go.uber.org/zap"
)

var errResyncFromAnotherServer = errors.New("resynchronisation from an S-CSCF other than the assigned one")

func (h *HSS) MultimediaAuth(ctx context.Context, id diameter.Identity, req *diameter.Message) *diameter.Message {
	r, err := cx.ParseMultimediaAuthRequest(req)
	if err != nil {
		return cx.NewErrorAnswer(req, id, err, 0)
	}

	if cx.RequiresUnsupportedFeatures(req, 0) {
		return experimentalAnswer(req, id, tgpp.ResultErrorFeatureUnsupported)
	}

	sub, err := h.resolve(ctx, r.PrivateIdentity, r.PublicIdentity)
	if err != nil {
		return h.errorAnswer(req, id, "multimedia auth", err)
	}

	if !r.Scheme.IsAKA() {
		return experimentalAnswer(req, id, tgpp.ResultErrorAuthSchemeNotSupported)
	}

	af := tgpp.ParseEnvelope(req).Origin

	set := sub.requestedSet()

	err = h.updateRegistration(ctx, sub.IMSI, func(current *Registration) (*Registration, error) {
		if r.Resync != nil && (current == nil || current.ServerName != r.ServerName) {
			return nil, errResyncFromAnotherServer
		}

		previous, known := current.set(set)

		if known && previous.State == Registered && current.ServerName == r.ServerName {
			return current, nil
		}

		pending := SetRegistration{State: NotRegistered, AuthPending: true}

		if current == nil || current.ServerName != r.ServerName {
			if current.registered() {
				h.log.Info("IMS subscriber moved to another S-CSCF; the previous S-CSCF is not notified",
					zap.String("imsi", sub.IMSI), zap.String("previous", current.ServerName), zap.String("serverName", r.ServerName))
			}

			next := &Registration{ServerName: r.ServerName, OriginHost: af.OriginHost, OriginRealm: af.OriginRealm}

			return next.with(set, pending), nil
		}

		if known {
			pending.State, pending.Emergency = previous.State, previous.Emergency
			pending.DefaultIdentity, pending.ProfileDigest = previous.DefaultIdentity, previous.ProfileDigest
		}

		next := current.with(set, pending)
		next.OriginHost, next.OriginRealm = af.OriginHost, af.OriginRealm

		return next, nil
	})
	if errors.Is(err, errResyncFromAnotherServer) {
		h.log.Info("IMS AKA resynchronisation from an S-CSCF other than the assigned one", zap.String("imsi", sub.IMSI), zap.String("serverName", r.ServerName))

		return cx.NewAnswer(req, id, tgpp.Result{Code: diameter.ResultUnableToComply}, 0)
	}

	if err != nil {
		return h.errorAnswer(req, id, "multimedia auth", err)
	}

	var resync *udm.IMSResync
	if r.Resync != nil {
		resync = &udm.IMSResync{RAND: r.Resync.RAND, AUTS: r.Resync.AUTS}
	}

	var av *udm.IMSAV

	err = bounded(ctx, func(ctx context.Context) error {
		v, err := h.credentials.GenerateIMSVector(ctx, sub.IMSI, resync)
		av = v

		return err
	})
	if err != nil {
		return h.errorAnswer(req, id, "multimedia auth", err)
	}

	ans, err := cx.NewMultimediaAuthAnswer(req, id, cx.MultimediaAuth{
		PrivateIdentity: r.PrivateIdentity,
		PublicIdentity:  r.PublicIdentity,
		Items: []cx.AuthItem{{
			ItemNumber: 1,
			Scheme:     r.Scheme,
			AKA:        &cx.AKAVector{RAND: av.RAND, AUTN: av.AUTN, XRES: av.XRES, CK: av.CK, IK: av.IK},
		}},
	})
	if err != nil {
		return h.errorAnswer(req, id, "multimedia auth", err)
	}

	return ans
}
