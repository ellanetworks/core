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

	pushed, err := h.pushProfile(ctx, s.privateIdentity(), reg, userData)
	if err != nil {
		span.RecordError(err)
		h.log.Warn("Cx profile push failed", zap.String("imsi", imsi), zap.String("serverHost", reg.OriginHost), zap.Error(err))

		return err
	}

	if pushed {
		h.log.Info("IMS user profile pushed", zap.String("imsi", imsi), zap.String("serverHost", reg.OriginHost))
	} else {
		h.log.Info("the S-CSCF does not support profile push; it gets the new profile at its next server assignment",
			zap.String("imsi", imsi), zap.String("serverHost", reg.OriginHost))
	}

	return nil
}

func (h *HSS) pushProfile(ctx context.Context, privateIdentity string, reg *Registration, userData []byte) (bool, error) {
	var node *diameter.Node
	if h.peers != nil {
		node = h.peers.Node()
	}

	if node == nil {
		return false, ErrDiameterUnavailable
	}

	req, err := cx.NewPushProfileRequest(tgpp.Envelope{
		SessionID:        node.NewSessionID(),
		Origin:           node.Identity(),
		DestinationHost:  reg.OriginHost,
		DestinationRealm: reg.OriginRealm,
	}, cx.PushProfileRequest{PrivateIdentity: privateIdentity, UserData: userData})
	if err != nil {
		return false, err
	}

	ans, err := node.Send(ctx, req)
	if err != nil {
		return false, err
	}

	r, err := tgpp.ParseResult(ans)
	if err != nil {
		return false, err
	}

	if r.Code == diameter.ResultCommandUnsupported {
		return false, nil
	}

	if r.Failure() {
		return false, fmt.Errorf("the S-CSCF answered %s", r)
	}

	return true, nil
}
