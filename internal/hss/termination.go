// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package hss

import (
	"context"
	"errors"
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
	ServerName      string
	ServerHost      string
	ServerRealm     string
	Reason          cx.ReasonCode
	expected        Registration
}

func (h *HSS) Termination(ctx context.Context, imsi string) (*Termination, error) {
	reg, err := h.store.Registration(ctx, imsi)
	if err != nil || reg == nil || reg.State == NotRegistered {
		return nil, err
	}

	return termination(imsi, reg, cx.ReasonPermanentTermination), nil
}

func termination(imsi string, reg *Registration, reason cx.ReasonCode) *Termination {
	return &Termination{
		IMSI:            imsi,
		PrivateIdentity: reg.PrivateIdentity,
		ServerName:      reg.ServerName,
		ServerHost:      reg.OriginHost,
		ServerRealm:     reg.OriginRealm,
		Reason:          reason,
		expected:        *reg,
	}
}

func (h *HSS) Terminate(ctx context.Context, t *Termination) error {
	if t.Reason == cx.ReasonServerChange {
		if err := h.clearRegistration(ctx, t); err != nil {
			return err
		}
	}

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

	emergency, err := h.terminate(ctx, t)
	if err != nil {
		span.RecordError(err)
		h.log.Warn("Cx registration termination failed", zap.String("imsi", t.IMSI), zap.String("serverHost", t.ServerHost), zap.Error(err))

		return err
	}

	h.log.Info("IMS registration terminated", zap.String("imsi", t.IMSI), zap.String("serverHost", t.ServerHost), zap.Bool("emergencyRegistered", emergency))

	if t.Reason == cx.ReasonPermanentTermination {
		return h.permanentlyTerminated(ctx, t, emergency)
	}

	return nil
}

func (h *HSS) permanentlyTerminated(ctx context.Context, t *Termination, emergency bool) error {
	if !emergency {
		return h.clearRegistration(ctx, t)
	}

	return h.updateRegistration(ctx, t.IMSI, func(current *Registration) (*Registration, error) {
		if current == nil || current.State == NotRegistered || !t.holds(current) {
			return current, nil
		}

		next := *current
		next.State, next.Emergency = Unregistered, true

		return &next, nil
	})
}

func (h *HSS) terminate(ctx context.Context, t *Termination) (bool, error) {
	var node *diameter.Node
	if h.peers != nil {
		node = h.peers.Node()
	}

	if node == nil {
		return false, ErrDiameterUnavailable
	}

	req, err := cx.NewRegistrationTerminationRequest(tgpp.Envelope{
		SessionID:        node.NewSessionID(),
		Origin:           node.Identity(),
		DestinationHost:  t.ServerHost,
		DestinationRealm: t.ServerRealm,
	}, cx.RegistrationTerminationRequest{
		PrivateIdentity: t.PrivateIdentity,
		Reason:          cx.DeregistrationReason{Code: t.Reason},
	})
	if err != nil {
		return false, err
	}

	ans, err := node.Send(ctx, req, diameter.FailFast())
	if err != nil {
		return false, err
	}

	a, err := cx.ParseRegistrationTerminationAnswer(ans)

	var re *cx.RegistrationTerminationError
	if errors.As(err, &re) && re.Code == diameter.ResultUnableToComply && len(re.EmergencyIdentities) > 0 {
		return true, nil
	}

	if err != nil {
		return false, err
	}

	return len(a.EmergencyIdentities) > 0, nil
}

func (t *Termination) holds(current *Registration) bool {
	return *current == t.expected
}

func (h *HSS) clearRegistration(ctx context.Context, t *Termination) error {
	return h.updateRegistration(ctx, t.IMSI, func(current *Registration) (*Registration, error) {
		if current == nil || current.State == NotRegistered || !t.holds(current) {
			return current, nil
		}

		return nil, nil
	})
}
