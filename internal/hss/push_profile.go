// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package hss

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/ellanetworks/core/diameter"
	"github.com/ellanetworks/core/diameter/cx"
	"github.com/ellanetworks/core/diameter/tgpp"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

const pushProfileTimeout = 10 * time.Second

func (h *HSS) PushProfile(ctx context.Context, imsi string) error {
	reg, err := h.store.Registration(ctx, imsi)
	if err != nil {
		return err
	}

	if reg == nil || reg.State == NotRegistered || reg.ServerName == "" || reg.OriginHost == "" {
		return nil
	}

	sub, err := h.store.Subscriber(ctx, imsi)
	if err != nil {
		return err
	}

	domain, err := h.store.IMSDomain(ctx)
	if err != nil {
		return err
	}

	s := subject{Subscriber: *sub, domain: strings.ToLower(domain)}

	ifcs, err := h.store.FilterCriteria(ctx, imsi)
	if err != nil {
		return err
	}

	userData, err := s.userData(ifcs)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(ctx, pushProfileTimeout)
	defer cancel()

	ctx, span := tracer.Start(ctx, "cx/push-profile",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			attribute.String("network.protocol.name", "diameter"),
			attribute.Int64("diameter.application_id", int64(cx.ApplicationID)),
			attribute.Int64("diameter.command_code", int64(cx.CommandPushProfile)),
		),
	)
	defer span.End()

	r, err := h.pushProfile(ctx, s.privateIdentity(), reg, userData)
	if err != nil {
		span.RecordError(err)
		h.log.Warn("Cx profile push failed", zap.String("imsi", imsi), zap.String("serverHost", reg.OriginHost), zap.Error(err))

		return err
	}

	switch {
	case r.Code == diameter.ResultCommandUnsupported:
		h.log.Info("the S-CSCF does not support profile push; it gets the new profile at its next server assignment",
			zap.String("imsi", imsi), zap.String("serverHost", reg.OriginHost))

		return nil
	case r.IsExperimental(tgpp.ResultErrorNotSupportedUserData), r.IsExperimental(tgpp.ResultErrorTooMuchData):
		h.log.Warn("the S-CSCF rejected the IMS user profile; deregistering the user so that another S-CSCF is assigned",
			zap.String("imsi", imsi), zap.String("serverHost", reg.OriginHost), zap.Stringer("result", r))

		return h.reassign(ctx, imsi, reg)
	case r.IsExperimental(tgpp.ResultErrorUserUnknown):
		h.log.Warn("the S-CSCF does not know the user; marking it not registered",
			zap.String("imsi", imsi), zap.String("serverHost", reg.OriginHost))

		return h.clearRegistration(ctx, imsi, reg.ServerName)
	case r.Failure():
		err := fmt.Errorf("the S-CSCF answered %s", r)
		span.RecordError(err)
		h.log.Warn("Cx profile push failed", zap.String("imsi", imsi), zap.String("serverHost", reg.OriginHost), zap.Error(err))

		return err
	}

	h.log.Info("IMS user profile pushed", zap.String("imsi", imsi), zap.String("serverHost", reg.OriginHost))

	return nil
}

func (h *HSS) reassign(ctx context.Context, imsi string, reg *Registration) error {
	t, err := h.Termination(ctx, imsi)
	if err != nil || t == nil {
		return err
	}

	t.Reason = cx.ReasonServerChange

	if err := h.Terminate(ctx, t); err != nil {
		return err
	}

	return h.clearRegistration(ctx, imsi, reg.ServerName)
}

func (h *HSS) clearRegistration(ctx context.Context, imsi, serverName string) error {
	return h.updateRegistration(ctx, imsi, func(current *Registration) (*Registration, error) {
		if current == nil || current.State == NotRegistered || current.ServerName != serverName {
			return current, nil
		}

		return nil, nil
	})
}

func (h *HSS) pushProfile(ctx context.Context, privateIdentity string, reg *Registration, userData []byte) (tgpp.Result, error) {
	var node *diameter.Node
	if h.peers != nil {
		node = h.peers.Node()
	}

	if node == nil {
		return tgpp.Result{}, ErrDiameterUnavailable
	}

	req, err := cx.NewPushProfileRequest(tgpp.Envelope{
		SessionID:        node.NewSessionID(),
		Origin:           node.Identity(),
		DestinationHost:  reg.OriginHost,
		DestinationRealm: reg.OriginRealm,
	}, cx.PushProfileRequest{PrivateIdentity: privateIdentity, UserData: userData})
	if err != nil {
		return tgpp.Result{}, err
	}

	ans, err := node.Send(ctx, req)
	if err != nil {
		return tgpp.Result{}, err
	}

	return tgpp.ParseResult(ans)
}

func (h *HSS) IdentitiesChanged(ctx context.Context, imsi, previousMSISDN string) error {
	reg, err := h.store.Registration(ctx, imsi)
	if err != nil {
		return err
	}

	if reg == nil || reg.State != Registered || previousMSISDN == "" {
		return h.PushProfile(ctx, imsi)
	}

	sub, err := h.store.Subscriber(ctx, imsi)
	if err != nil {
		return err
	}

	if sub.MSISDN == previousMSISDN {
		return h.PushProfile(ctx, imsi)
	}

	t, err := h.Termination(ctx, imsi)
	if err != nil || t == nil {
		return err
	}

	t.Reason = cx.ReasonServerChange
	if sub.MSISDN == "" {
		t.Reason = cx.ReasonPermanentTermination
	}

	if err := h.Terminate(ctx, t); err != nil {
		return err
	}

	return h.clearRegistration(ctx, imsi, reg.ServerName)
}
