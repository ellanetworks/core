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

const terminationTimeout = 10 * time.Second

var ErrDiameterUnavailable = errors.New("the Diameter node is not running")

type Peers interface {
	Node() *diameter.Node
}

type Termination struct {
	IMSI            string
	PrivateIdentity string
	ServerHost      string
	ServerRealm     string
}

func (h *HSS) Termination(ctx context.Context, imsi string) (*Termination, error) {
	reg, err := h.store.Registration(ctx, imsi)
	if err != nil || reg == nil || reg.State == NotRegistered || reg.OriginHost == "" {
		return nil, err
	}

	domain, err := h.store.IMSDomain(ctx)
	if err != nil {
		return nil, err
	}

	return &Termination{
		IMSI:            imsi,
		PrivateIdentity: imsi + "@" + strings.ToLower(domain),
		ServerHost:      reg.OriginHost,
		ServerRealm:     reg.OriginRealm,
	}, nil
}

func (h *HSS) Terminate(ctx context.Context, t *Termination) error {
	ctx, cancel := context.WithTimeout(ctx, terminationTimeout)
	defer cancel()

	ctx, span := tracer.Start(ctx, "cx/registration-termination",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			attribute.String("network.protocol.name", "diameter"),
			attribute.Int64("diameter.application_id", int64(cx.ApplicationID)),
			attribute.Int64("diameter.command_code", int64(cx.CommandRegistrationTermination)),
		),
	)
	defer span.End()

	err := h.terminate(ctx, t)
	if err != nil {
		span.RecordError(err)
		h.log.Warn("Cx registration termination failed", zap.String("imsi", t.IMSI), zap.String("serverHost", t.ServerHost), zap.Error(err))

		return err
	}

	h.log.Info("IMS registration terminated", zap.String("imsi", t.IMSI), zap.String("serverHost", t.ServerHost))

	return nil
}

func (h *HSS) terminate(ctx context.Context, t *Termination) error {
	var node *diameter.Node
	if h.peers != nil {
		node = h.peers.Node()
	}

	if node == nil {
		return ErrDiameterUnavailable
	}

	req, err := cx.NewRegistrationTerminationRequest(tgpp.Envelope{
		SessionID:        node.NewSessionID(),
		Origin:           node.Identity(),
		DestinationHost:  t.ServerHost,
		DestinationRealm: t.ServerRealm,
	}, cx.RegistrationTerminationRequest{
		PrivateIdentity: t.PrivateIdentity,
		Reason:          cx.DeregistrationReason{Code: cx.ReasonPermanentTermination},
	})
	if err != nil {
		return err
	}

	ans, err := node.Send(ctx, req)
	if err != nil {
		return err
	}

	r, err := tgpp.ParseResult(ans)
	if err != nil {
		return err
	}

	if r.Failure() {
		return fmt.Errorf("the S-CSCF answered %s", r)
	}

	return nil
}
