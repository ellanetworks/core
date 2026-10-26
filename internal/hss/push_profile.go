// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package hss

import (
	"context"
	"errors"
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

type change int

const (
	changeNone change = iota
	changePush
	changeServerChange
	changePermanentTermination
)

func (c change) String() string {
	switch c {
	case changePush:
		return "push profile"
	case changeServerChange:
		return "server change"
	case changePermanentTermination:
		return "permanent termination"
	default:
		return "none"
	}
}

func decide(reg *Registration, sub *subject, p *profile) change {
	terminated := !sub.entitled() || p.privateIdentity != reg.PrivateIdentity

	switch {
	case terminated && reg.Emergency:
		return changeNone
	case terminated:
		return changePermanentTermination
	case p.defaultIdentity != reg.DefaultIdentity && reg.State == Registered:
		return changeServerChange
	case p.digest != reg.ProfileDigest || p.defaultIdentity != reg.DefaultIdentity:
		return changePush
	default:
		return changeNone
	}
}

func (h *HSS) Reconcile(ctx context.Context, imsi string) error {
	reg, err := h.store.Registration(ctx, imsi)
	if err != nil {
		return err
	}

	if reg == nil || reg.State == NotRegistered || reg.ServerName == "" || reg.OriginHost == "" {
		return nil
	}

	sub, err := h.store.Subscriber(ctx, imsi)
	if errors.Is(err, ErrSubscriberUnknown) {
		return nil
	}

	if err != nil {
		return err
	}

	domain, err := h.store.IMSDomain(ctx)
	if err != nil {
		return err
	}

	s := &subject{Subscriber: *sub, domain: strings.ToLower(domain)}

	p, err := h.profileOf(ctx, s)
	if err != nil {
		return err
	}

	c := decide(reg, s, p)

	switch c {
	case changeNone:
		return nil
	case changePush:
		return h.pushProfile(ctx, imsi, reg, p)
	default:
		reason := cx.ReasonPermanentTermination
		if c == changeServerChange {
			reason = cx.ReasonServerChange
		}

		h.log.Info("IMS subscription changed; de-registering the user", zap.String("imsi", imsi), zap.String("serverHost", reg.OriginHost),
			zap.Stringer("change", c))

		return h.Terminate(ctx, termination(imsi, reg, reason))
	}
}

func (h *HSS) pushProfile(ctx context.Context, imsi string, reg *Registration, p *profile) error {
	pushCtx, cancel := context.WithTimeout(ctx, pushProfileTimeout)
	defer cancel()

	pushCtx, span := tracer.Start(pushCtx, "cx/push-profile",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			attribute.String("network.protocol.name", "diameter"),
			attribute.Int64("diameter.application_id", int64(cx.ApplicationID)),
			attribute.Int64("diameter.command_code", int64(cx.CommandPushProfile)),
		),
	)
	defer span.End()

	r, err := h.send(pushCtx, reg, p)
	if err != nil {
		span.RecordError(err)

		outcome := pushFailed
		if errors.Is(err, diameter.ErrNotConnected) || errors.Is(err, diameter.ErrUnableToDeliver) {
			outcome = pushNotConnected
		}

		pushes.WithLabelValues(outcome).Inc()
		h.log.Warn("Cx profile push failed; the S-CSCF gets the profile at its next server assignment", zap.String("imsi", imsi),
			zap.String("serverHost", reg.OriginHost), zap.Error(err))

		return err
	}

	switch {
	case r.Success():
		pushes.WithLabelValues(pushDelivered).Inc()
		h.log.Info("IMS user profile pushed", zap.String("imsi", imsi), zap.String("serverHost", reg.OriginHost))

		return h.updateRegistration(context.WithoutCancel(ctx), imsi, func(current *Registration) (*Registration, error) {
			if !sameAssignment(current, reg) {
				return current, nil
			}

			next := *current
			next.DefaultIdentity, next.ProfileDigest = p.defaultIdentity, p.digest

			return &next, nil
		})
	case r.Code == diameter.ResultCommandUnsupported:
		pushes.WithLabelValues(pushUnsupported).Inc()
		h.log.Info("the S-CSCF does not support profile push; it gets the new profile at its next server assignment",
			zap.String("imsi", imsi), zap.String("serverHost", reg.OriginHost))

		return nil
	case r.IsExperimental(tgpp.ResultErrorNotSupportedUserData), r.IsExperimental(tgpp.ResultErrorTooMuchData):
		pushes.WithLabelValues(pushRejected).Inc()
		h.log.Warn("the S-CSCF rejected the IMS user profile; de-registering the user so that another S-CSCF is assigned",
			zap.String("imsi", imsi), zap.String("serverHost", reg.OriginHost), zap.Stringer("result", r))

		return h.Terminate(context.WithoutCancel(ctx), termination(imsi, reg, cx.ReasonServerChange))
	case r.IsExperimental(tgpp.ResultErrorUserUnknown):
		pushes.WithLabelValues(pushUserUnknown).Inc()
		h.log.Warn("the S-CSCF does not know the user; marking it not registered",
			zap.String("imsi", imsi), zap.String("serverHost", reg.OriginHost))

		return h.updateRegistration(context.WithoutCancel(ctx), imsi, func(current *Registration) (*Registration, error) {
			if !sameAssignment(current, reg) {
				return current, nil
			}

			return nil, nil
		})
	default:
		pushes.WithLabelValues(pushFailed).Inc()

		err := fmt.Errorf("the S-CSCF answered %s", r)
		span.RecordError(err)
		h.log.Warn("Cx profile push failed; the S-CSCF gets the profile at its next server assignment", zap.String("imsi", imsi),
			zap.String("serverHost", reg.OriginHost), zap.Error(err))

		return err
	}
}

func sameAssignment(current, reg *Registration) bool {
	return current != nil && *current == *reg
}

func (h *HSS) send(ctx context.Context, reg *Registration, p *profile) (tgpp.Result, error) {
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
	}, cx.PushProfileRequest{PrivateIdentity: reg.PrivateIdentity, UserData: p.userData})
	if err != nil {
		return tgpp.Result{}, err
	}

	ans, err := node.Send(ctx, req, diameter.FailFast())
	if err != nil {
		return tgpp.Result{}, err
	}

	return tgpp.ParseResult(ans)
}
